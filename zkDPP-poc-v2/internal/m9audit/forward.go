package m9audit

import (
	"context"
	"fmt"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Result struct {
	Snapshot                                            audit.Snapshot
	Start                                               audit.Ref
	Objects, Records, Decryptions, Responses, CacheHits int
	Leaves                                              []audit.Leaf
	DPPs                                                []audit.Ref
	Relationships                                       []audit.Relationship
	TotalMillis                                         float64
}
type plain struct{ spends []fr.Element }

func Forward(ctx context.Context, src *audit.RPCSource, committee auditcrypto.Committee, start audit.Ref, snap audit.Snapshot) (out Result, err error) {
	out.Snapshot = snap
	out.Start = start
	begin := time.Now()
	defer func() { out.TotalMillis = float64(time.Since(begin).Nanoseconds()) / 1e6 }()
	if err = src.Check(ctx, snap); err != nil {
		return out, err
	}
	cache := map[uint64]plain{}
	records := map[uint64]bool{}
	queue := []audit.Ref{start}
	visited := map[string]bool{}
	decrypt := func(id uint64, l audit.Loaded) (plain, error) {
		if p, ok := cache[id]; ok {
			out.CacheHits++
			return p, nil
		}
		parts := make([]auditcrypto.Partial, 2)
		for i := 0; i < 2; i++ {
			p, e := auditcrypto.PartialDecrypt(committee.Shares[i], l.Record.R1)
			if e != nil {
				return plain{}, e
			}
			parts[i] = p
			out.Responses++
		}
		m, e := auditcrypto.CombineAndDecrypt(l.Context, l.Record.Ciphertext(), parts)
		if e != nil {
			return plain{}, e
		}
		shape, _ := audit.Shape(l.Record.EventKind)
		p := plain{spends: append([]fr.Element{}, m[shape.ParentCount:]...)}
		cache[id] = p
		out.Decryptions++
		return p, nil
	}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if visited[ref.Key()] {
			continue
		}
		visited[ref.Key()] = true
		out.Objects++
		aid, e := src.Producer(ctx, ref, snap)
		if e != nil || aid == 0 {
			return out, fmt.Errorf("producer: %w", e)
		}
		producer, e := src.Load(ctx, aid, snap)
		if e != nil {
			return out, e
		}
		records[aid] = true
		pos, e := producer.Record.Position(ref)
		if e != nil {
			return out, e
		}
		p, e := decrypt(aid, producer)
		if e != nil {
			return out, e
		}
		if pos >= len(p.spends) {
			return out, fmt.Errorf("missing output spend")
		}
		spend := p.spends[pos]
		cid, e := src.Spent(ctx, ref.ObjectType, spend, snap)
		if e != nil {
			return out, e
		}
		if cid == 0 {
			out.Leaves = append(out.Leaves, audit.Leaf{Ref: ref, SpendValue: spend})
			continue
		}
		consumer, e := src.Load(ctx, cid, snap)
		if e != nil {
			special, base, se := loadExit(ctx, src, snap, cid)
			if se != nil {
				return out, se
			}
			if special.EventKind != 7 || len(base) != 3 || !base[1].Equal(&spend) || len(special.OutputRefs) != 1 || special.OutputRefs[0].ObjectType != DPP {
				return out, fmt.Errorf("invalid DPP terminal")
			}
			records[cid] = true
			dpp := special.OutputRefs[0]
			out.DPPs = append(out.DPPs, dpp)
			out.Relationships = append(out.Relationships, audit.Relationship{Parent: ref, Child: dpp, RecordID: cid})
			continue
		}
		records[cid] = true
		shape, _ := audit.Shape(consumer.Record.EventKind)
		found := false
		for _, i := range shape.SpendPositions {
			if i < len(consumer.Base) && consumer.Base[i].Equal(&spend) {
				found = true
			}
		}
		if !found {
			return out, fmt.Errorf("consumer mismatch")
		}
		for _, child := range consumer.Record.OutputRefs {
			out.Relationships = append(out.Relationships, audit.Relationship{Parent: ref, Child: child, RecordID: cid})
			queue = append(queue, child)
		}
	}
	out.Records = len(records)
	return out, nil
}
