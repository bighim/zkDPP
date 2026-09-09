package v2audit

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"math/big"
)

func TraceBackward(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot) (TraceResult, error) {
	var out TraceResult
	if e := src.CheckSnapshot(ctx, snap); e != nil {
		return out, e
	}
	queue := []Ref{start}
	seen := map[string]bool{}
	expanded := map[uint64]bool{}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if seen[ref.Key()] {
			continue
		}
		seen[ref.Key()] = true
		out.Metrics.VisitedObjects++
		out.Objects = append(out.Objects, ref)
		id, e := src.Producer(ctx, ref, snap)
		if e != nil {
			return out, e
		}
		if id == 0 {
			return out, fmt.Errorf("missing producer")
		}
		r, e := src.Record(ctx, id, snap)
		if e != nil {
			return out, e
		}
		if e = r.Validate(); e != nil {
			return out, e
		}
		if _, e = r.Position(ref); e != nil {
			return out, e
		}
		if expanded[id] {
			continue
		}
		expanded[id] = true
		out.RecordIDs = append(out.RecordIDs, id)
		out.Metrics.UniqueRecords++
		if r.EventKind == Entry {
			out.Entries = append(out.Entries, ref)
			continue
		}
		plain, e := auditcrypto.DecryptWithMasterKey(master, r.Ciphertext())
		if e != nil {
			return out, e
		}
		out.Metrics.Decryptions++
		shape, _ := Shape(r.EventKind)
		for _, v := range plain[:shape.ParentCount] {
			parent := Ref{shape.ParentType, v}
			pid, e := src.Producer(ctx, parent, snap)
			if e != nil {
				return out, e
			}
			if pid == 0 || pid >= id {
				return out, fmt.Errorf("noncausal parent")
			}
			queue = append(queue, parent)
			for _, child := range r.OutputRefs {
				out.Graph = append(out.Graph, Edge{parent, child, id})
			}
		}
	}
	return out, src.CheckSnapshot(ctx, snap)
}
