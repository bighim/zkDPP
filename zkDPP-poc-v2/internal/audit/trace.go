package audit

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"time"
)

type Snapshot struct {
	Number uint64
	Hash   string
}
type Loaded struct {
	Record  Record
	Base    []fr.Element
	Context fr.Element
}
type Source interface {
	Check(context.Context, Snapshot) error
	Load(context.Context, uint64, Snapshot) (Loaded, error)
	Producer(context.Context, Ref, Snapshot) (uint64, error)
	Spent(context.Context, uint8, fr.Element, Snapshot) (uint64, error)
}
type Leaf struct {
	Ref        Ref
	SpendValue fr.Element
}
type Metrics struct {
	Objects, UniqueRecords, Decryptions, Responses, CacheHits                               int
	LookupMillis, Member1Millis, Member2Millis, CombineMillis, TotalMillis, TraversalMillis float64
}
type TraceResult struct {
	Direction     string
	Snapshot      Snapshot
	Leaves        []Leaf
	Entries       []Ref
	Terminated    []Ref
	Metrics       Metrics
	Objects       []Ref
	RecordIDs     []uint64
	Relationships []Relationship
}
type Relationship struct {
	Parent, Child Ref
	RecordID      uint64
}
type plaintext struct {
	Parents []Ref
	Spends  []fr.Element
}

func Trace(ctx context.Context, src Source, committee auditcrypto.Committee, direction string, start Ref, s Snapshot) (out TraceResult, err error) {
	if direction != "forward" && direction != "backward" {
		return out, fmt.Errorf("unknown direction")
	}
	if start.ObjectType != Note && start.ObjectType != Voucher {
		return out, fmt.Errorf("invalid start type")
	}
	out.Direction = direction
	out.Snapshot = s
	begin := time.Now()
	defer func() {
		out.Metrics.TotalMillis = ms(time.Since(begin))
		m := &out.Metrics
		m.TraversalMillis = m.TotalMillis - m.LookupMillis - m.Member1Millis - m.Member2Millis - m.CombineMillis
	}()
	check := func() error {
		t := time.Now()
		e := src.Check(ctx, s)
		out.Metrics.LookupMillis += ms(time.Since(t))
		return e
	}
	if e := check(); e != nil {
		return out, e
	}
	records := map[uint64]bool{}
	cache := map[uint64]plaintext{}
	load := func(id uint64) (Loaded, error) {
		t := time.Now()
		l, e := src.Load(ctx, id, s)
		out.Metrics.LookupMillis += ms(time.Since(t))
		if e == nil {
			if e = l.Record.Validate(); e != nil {
				return l, e
			}
			if !records[id] {
				out.RecordIDs = append(out.RecordIDs, id)
			}
			records[id] = true
			out.Metrics.UniqueRecords = len(records)
		}
		return l, e
	}
	producer := func(r Ref) (uint64, error) {
		t := time.Now()
		id, e := src.Producer(ctx, r, s)
		out.Metrics.LookupMillis += ms(time.Since(t))
		if e == nil && id == 0 {
			e = fmt.Errorf("missing producer")
		}
		return id, e
	}
	decrypt := func(id uint64, l Loaded) (plaintext, error) {
		if p, ok := cache[id]; ok {
			out.Metrics.CacheHits++
			return p, nil
		}
		parts := make([]auditcrypto.Partial, 2)
		for i := 0; i < 2; i++ {
			// Each member independently checks the same approved on-chain original.
			member, e := load(id)
			if e != nil {
				return plaintext{}, e
			}
			if !Equal(member.Record.Ciphertext().Data, l.Record.Ciphertext().Data) || !member.Context.Equal(&l.Context) || !member.Record.R1.Equal(&l.Record.R1) {
				return plaintext{}, fmt.Errorf("member original mismatch")
			}
			t := time.Now()
			p, e := auditcrypto.PartialDecrypt(committee.Shares[i], member.Record.R1)
			elapsed := ms(time.Since(t))
			if i == 0 {
				out.Metrics.Member1Millis += elapsed
			} else {
				out.Metrics.Member2Millis += elapsed
			}
			if e != nil {
				return plaintext{}, e
			}
			parts[i] = p
			out.Metrics.Responses++
		}
		t := time.Now()
		m, e := auditcrypto.CombineAndDecrypt(l.Context, l.Record.Ciphertext(), parts)
		out.Metrics.CombineMillis += ms(time.Since(t))
		if e != nil {
			return plaintext{}, e
		}
		shape, _ := Shape(l.Record.EventKind)
		p := plaintext{Spends: append([]fr.Element{}, m[shape.ParentCount:]...)}
		for _, v := range m[:shape.ParentCount] {
			p.Parents = append(p.Parents, Ref{shape.ParentType, v})
		}
		out.Metrics.Decryptions++
		cache[id] = p
		return p, nil
	}
	queue := []Ref{start}
	visited := map[string]bool{}
	expanded := map[uint64]bool{}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if visited[ref.Key()] {
			continue
		}
		visited[ref.Key()] = true
		out.Objects = append(out.Objects, ref)
		out.Metrics.Objects++
		aid, e := producer(ref)
		if e != nil {
			return out, e
		}
		l, e := load(aid)
		if e != nil {
			return out, e
		}
		pos, e := l.Record.Position(ref)
		if e != nil {
			return out, e
		}
		if direction == "backward" {
			if expanded[aid] {
				continue
			}
			expanded[aid] = true
			if l.Record.EventKind == Entry {
				out.Entries = append(out.Entries, ref)
				continue
			}
			p, e := decrypt(aid, l)
			if e != nil {
				return out, e
			}
			for _, parent := range p.Parents {
				id, e := producer(parent)
				if e != nil {
					return out, e
				}
				if id >= aid {
					return out, fmt.Errorf("noncausal parent")
				}
				queue = append(queue, parent)
				for _, child := range l.Record.OutputRefs {
					out.Relationships = append(out.Relationships, Relationship{parent, child, aid})
				}
			}
		} else {
			p, e := decrypt(aid, l)
			if e != nil {
				return out, e
			}
			spend := p.Spends[pos]
			t := time.Now()
			cid, e := src.Spent(ctx, ref.ObjectType, spend, s)
			out.Metrics.LookupMillis += ms(time.Since(t))
			if e != nil {
				return out, e
			}
			if cid == 0 {
				out.Leaves = append(out.Leaves, Leaf{ref, spend})
				continue
			}
			if cid <= aid {
				return out, fmt.Errorf("noncausal consumer")
			}
			c, e := load(cid)
			if e != nil {
				return out, e
			}
			shape, _ := Shape(c.Record.EventKind)
			found := false
			if shape.ParentType == ref.ObjectType {
				for _, i := range shape.SpendPositions {
					if c.Base[i].Equal(&spend) {
						found = true
					}
				}
			}
			if !found {
				return out, fmt.Errorf("consumer does not spend nullifier")
			}
			if len(c.Record.OutputRefs) == 0 {
				if c.Record.EventKind != Exit {
					return out, fmt.Errorf("unexpected terminal record")
				}
				out.Terminated = append(out.Terminated, ref)
			} else {
				for _, child := range c.Record.OutputRefs {
					out.Relationships = append(out.Relationships, Relationship{ref, child, cid})
				}
				queue = append(queue, c.Record.OutputRefs...)
			}
		}
	}
	if e := check(); e != nil {
		return out, e
	}
	return out, nil
}
func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
