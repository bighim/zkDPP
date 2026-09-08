package audit_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"testing"
)

type memory struct {
	records           map[uint64]audit.Loaded
	producers, spent  map[string]uint64
	failRead, changed bool
}

func (m *memory) Check(context.Context, audit.Snapshot) error {
	if m.changed {
		return errors.New("snapshot changed")
	}
	return nil
}
func (m *memory) Load(_ context.Context, id uint64, _ audit.Snapshot) (audit.Loaded, error) {
	if m.failRead {
		return audit.Loaded{}, errors.New("unavailable")
	}
	v, ok := m.records[id]
	if !ok {
		return v, errors.New("missing record")
	}
	return v, nil
}
func (m *memory) Producer(_ context.Context, r audit.Ref, _ audit.Snapshot) (uint64, error) {
	return m.producers[r.Key()], nil
}
func (m *memory) Spent(_ context.Context, typ uint8, v fr.Element, _ audit.Snapshot) (uint64, error) {
	if m.failRead {
		return 0, errors.New("RPC failure")
	}
	return m.spent[(audit.Ref{ObjectType: typ, RawID: v}).Key()], nil
}
func source(events []*m7case.Case) *memory {
	m := &memory{records: map[uint64]audit.Loaded{}, producers: map[string]uint64{}, spent: map[string]uint64{}}
	for i, c := range events {
		id := uint64(i + 1)
		m.records[id] = audit.Loaded{Record: c.Record, Base: c.Base, Context: c.Context}
		for _, r := range c.Record.OutputRefs {
			m.producers[r.Key()] = id
		}
		l, _ := audit.Shape(c.Kind)
		for _, j := range l.SpendPositions {
			m.spent[(audit.Ref{ObjectType: l.ParentType, RawID: c.Base[j]}).Key()] = id
		}
	}
	return m
}
func TestSnapshotTraversal(t *testing.T) {
	pk, shares, e := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{5}, 1024)))
	if e != nil {
		t.Fatal(e)
	}
	c := auditcrypto.Committee{PK: pk, Shares: shares}
	groups, e := m7case.Build("../..", pk)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	snap := audit.Snapshot{Number: 3, Hash: "fixed"}
	g := groups[0]
	m := source(g.Events[:3])
	a := g.Events[0].Record.OutputRefs[0]
	d := g.Events[2].Record.OutputRefs[0]
	back, e := audit.Trace(ctx, m, c, "backward", d, snap)
	if e != nil {
		t.Fatal(e)
	}
	if back.Metrics.UniqueRecords != 3 || back.Metrics.Decryptions != 2 || back.Metrics.Responses != 4 || len(back.Entries) != 1 {
		t.Fatalf("backward %+v", back.Metrics)
	}
	forward, e := audit.Trace(ctx, m, c, "forward", a, snap)
	if e != nil {
		t.Fatal(e)
	}
	if forward.Metrics.Objects != 5 || forward.Metrics.UniqueRecords != 3 || forward.Metrics.Decryptions != 3 || forward.Metrics.Responses != 6 || len(forward.Leaves) != 3 {
		t.Fatalf("forward %+v", forward.Metrics)
	}
	if len(forward.Relationships) != 4 || len(back.Relationships) != 4 || len(forward.RecordIDs) != 3 || len(forward.Objects) != 5 {
		t.Fatal("missing recovered graph")
	}
	wantEdges := map[string]bool{}
	for _, child := range g.Events[1].Record.OutputRefs {
		wantEdges[a.Key()+"->"+child.Key()] = true
	}
	for _, child := range g.Events[2].Record.OutputRefs {
		wantEdges[g.Events[1].Record.OutputRefs[0].Key()+"->"+child.Key()] = true
	}
	for _, r := range forward.Relationships {
		if !wantEdges[r.Parent.Key()+"->"+r.Child.Key()] {
			t.Fatal("wrong recovered edge")
		}
	}
	want := []audit.Ref{g.Events[1].Record.OutputRefs[1], g.Events[2].Record.OutputRefs[0], g.Events[2].Record.OutputRefs[1]}
	for i, l := range forward.Leaves {
		if l.Ref.Key() != want[i].Key() {
			t.Fatal("leaf order")
		}
	}
	m = source(g.Events)
	after, e := audit.Trace(ctx, m, c, "forward", a, snap)
	if e != nil {
		t.Fatal(e)
	}
	if len(after.Leaves) != 2 || len(after.Terminated) != 1 {
		t.Fatal("Exit is not an unspent leaf")
	}
	for _, g := range groups[1:] {
		m := source(g.Events)
		for _, ref := range g.Events[len(g.Events)-1].Record.OutputRefs {
			if _, e := audit.Trace(ctx, m, c, "backward", ref, snap); e != nil {
				t.Fatal(g.Name, e)
			}
		}
		if _, e := audit.Trace(ctx, m, c, "forward", g.Events[0].Record.OutputRefs[0], snap); e != nil {
			t.Fatal(g.Name, e)
		}
	}
	for _, kind := range []string{"read", "snapshot", "missing", "wrong-consumer", "noncausal", "record-shape"} {
		t.Run(kind, func(t *testing.T) {
			m := source(g.Events[:3])
			switch kind {
			case "read":
				m.failRead = true
			case "snapshot":
				m.changed = true
			case "missing":
				delete(m.producers, a.Key())
			case "wrong-consumer":
				for k := range m.spent {
					m.spent[k] = 3
				}
			case "noncausal":
				for k := range m.spent {
					m.spent[k] = 1
				}
			case "record-shape":
				v := m.records[1]
				v.Record.OutputRefs = nil
				m.records[1] = v
			}
			if _, e := audit.Trace(ctx, m, c, "forward", a, snap); e == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}
