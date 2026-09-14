package experiments

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"path/filepath"
	"time"
)

type RouteRow struct {
	Route        string
	Run          int
	Warmup       bool
	TotalMS      float64
	Transactions int
	Gas          uint64
	Matched      bool
}

func Routes(root string, ctx context.Context, smoke bool) error {
	e, er := NewEngine(root, ctx, false)
	if er != nil {
		return er
	}
	defer e.Client.Close()
	if er = e.Initialize(0); er != nil {
		return er
	}
	w := NewWorld(defaultPK(), 20000000)
	sk := h.Element(11)
	recv := h.Element(22)
	doc := must(document.Hash(DefaultDocument()))
	entry, er := e.Execute(w, Operation{P: must(ProfileByName("entry-plain")), SK: sk, Document: doc, State: note.State{QMass: 160e9, ARec: 15e9, E: 115e9}}, "exp2-prepare-entry", -1, false)
	if er != nil {
		return er
	}
	prep := append([]Measurement{}, e.Rows...)
	e.Rows = nil
	snapshot, er := e.Snapshot()
	if er != nil {
		return er
	}
	var routes []RouteRow
	e.DeferDiagnostics = true
	last := Repetitions
	if smoke {
		last = 0
	}
	for _, route := range []string{"direct", "proceed", "recall"} {
		for run := 0; run <= last; run++ {
			snapshot, er = e.Restore(snapshot)
			if er != nil {
				return er
			}
			world := w.Clone()
			world.Serial += uint64(run) * 10000
			offset := len(e.Rows)
			kind := "transfer-plain"
			if route == "direct" {
				kind = "direct-plain"
			}
			if _, er = e.keys(kind); er != nil {
				return er
			}
			if route != "direct" {
				if _, er = e.keys(route + "-plain"); er != nil {
					return er
				}
			}
			op := world.Ready(Operation{P: must(ProfileByName(kind)), SK: sk, Inputs: entry.Outputs, Receiver: owner.Address(recv), Transport: 1e9})
			start := time.Now()
			ev, er := e.Execute(world, op, fmt.Sprintf("exp2-%s-send", route), run, false)
			if er != nil {
				return er
			}
			var final *Event = ev
			if route != "direct" {
				owner := recv
				if route == "recall" {
					owner = sk
				}
				final, er = e.Execute(world, Operation{P: must(ProfileByName(route + "-plain")), SK: owner, Voucher: *ev.Voucher}, "exp2-"+route+"-resolve", run, false)
				if er != nil {
					return er
				}
			}
			elapsed := ms(e.LastReceipt.Sub(start))
			if er = e.FlushDiagnostics(); er != nil {
				return er
			}
			if final.Outputs[0].State != (note.State{QMass: 160e9, ARec: 15e9, E: 116e9}) {
				return fmt.Errorf("route state")
			}
			var gas uint64
			for _, row := range e.Rows[offset:] {
				gas += row.Tx.Gas
			}
			routes = append(routes, RouteRow{Route: route, Run: run, Warmup: run == 0, TotalMS: elapsed, Transactions: len(e.Rows) - offset, Gas: gas, Matched: true})
			fmt.Printf("route %s run %d complete\n", route, run)
		}
	}
	return saveExperiment(root, 2, smoke, map[string]any{"rows": e.Rows, "routes": routes, "preparationRows": prep, "preparations": e.Preparations, "deployments": e.Deployments, "routeBoundary": "live proof chain including local intermediate handoff and path acquisition; local Verify, diagnostics and artifact persistence after final receipt"})
}
func Variants(root string, ctx context.Context, exp int, smoke bool) error {
	var rows, prep []Measurement
	var deployments []map[string]any
	var preparations []Transaction
	last := Repetitions
	if smoke {
		last = 0
	}
	if exp == 6 {
		for _, a := range []bool{false, true} {
			en, er := NewEngine(root, ctx, a)
			if er != nil {
				return er
			}
			defer en.Client.Close()
			var reset any
			if er = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); er != nil {
				return er
			}
			if er = en.Initialize(0); er != nil {
				return er
			}
			suffix := "-plain"
			if a {
				suffix = "-audit"
			}
			baseSnap, er := en.Snapshot()
			if er != nil {
				return er
			}
			for idx, mn := range [][2]int{{2, 1}, {2, 2}, {3, 2}, {3, 3}, {4, 3}} {
				if idx > 0 {
					preparations = append(preparations, en.Preparations...)
					deployments = append(deployments, en.Deployments...)
					var reset any
					if er = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); er != nil {
						return er
					}
					en.Client.Close()
					en, er = NewEngine(root, ctx, a)
					if er != nil {
						return er
					}
					defer en.Client.Close()
					if er = en.Initialize(0); er != nil {
						return er
					}
					baseSnap, er = en.Snapshot()
					if er != nil {
						return er
					}
				}
				baseSnap, er = en.Restore(baseSnap)
				if er != nil {
					return er
				}
				w := NewWorld(defaultPK(), 60000000+uint64(idx)*100000)
				sk := h.Element(22)
				doc := must(document.Hash(DefaultDocument()))
				var inputs []note.Note
				for i := 0; i < mn[0]; i++ {
					s := equalState(mn[0], i)
					ev, er := en.Execute(w, Operation{P: must(ProfileByName("entry" + suffix)), SK: sk, Document: doc, State: s}, fmt.Sprintf("exp6-%d-%d-prepare-%d", mn[0], mn[1], i), -1, false)
					if er != nil {
						return er
					}
					inputs = append(inputs, ev.Outputs[0])
				}
				snap, er := en.Snapshot()
				if er != nil {
					return er
				}
				prep = append(prep, en.Rows...)
				en.Rows = nil
				p := must(ProfileByName(fmt.Sprintf("process-%d-%d%s", mn[0], mn[1], suffix)))
				for run := 0; run <= last; run++ {
					snap, er = en.Restore(snap)
					if er != nil {
						return er
					}
					world := w.Clone()
					world.Serial += uint64(run) * 10000
					ev, er := en.Execute(world, Operation{P: p, SK: sk, Document: doc, Inputs: inputs}, fmt.Sprintf("exp6-%d-%d", mn[0], mn[1]), run, !smoke)
					if er != nil {
						return er
					}
					var q, a, carbon uint64
					for _, o := range ev.Outputs {
						q += o.State.QMass
						a += o.State.ARec
						carbon += o.State.E
					}
					if q != 300e9 || a != 30e9 || carbon != 261e9 {
						return fmt.Errorf("Process aggregate")
					}
					fmt.Printf("Process %s run %d complete\n", p.Name, run)
				}
				rows = append(rows, en.Rows...)
				en.Rows = nil
			}
			deployments = append(deployments, en.Deployments...)
			preparations = append(preparations, en.Preparations...)
		}
	} else if exp == 5 {
		for _, count := range []int{1, 2, 3, 4, 8, 16} {
			en, er := NewEngine(root, ctx, true)
			if er != nil {
				return er
			}
			defer en.Client.Close()
			var reset any
			if er = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); er != nil {
				return er
			}
			if er = en.Initialize(count); er != nil {
				return er
			}
			w := NewWorld(defaultPK(), 50000000+uint64(count)*100000)
			sk := h.Element(22)
			doc := must(document.Hash(DefaultDocument()))
			values := make([]uint64, count)
			for i := range values {
				values[i] = uint64(i+1) * Scale
			}
			var inputs []VectorNote
			for i := 0; i < 2; i++ {
				ev, er := en.Execute(w, Operation{P: must(ProfileByName(fmt.Sprintf("vector-entry-%d", count))), SK: sk, Document: doc, VectorState: values}, fmt.Sprintf("exp5-vector-%d-prepare-%d", count, i), -1, false)
				if er != nil {
					return er
				}
				inputs = append(inputs, ev.VectorOutputs[0])
			}
			prep = append(prep, en.Rows...)
			en.Rows = nil
			snap, er := en.Snapshot()
			if er != nil {
				return er
			}
			for run := 0; run <= last; run++ {
				snap, er = en.Restore(snap)
				if er != nil {
					return er
				}
				world := w.Clone()
				world.Serial += uint64(run) * 10000
				_, er = en.Execute(world, Operation{P: must(ProfileByName(fmt.Sprintf("vector-merge-%d", count))), SK: sk, Document: doc, VectorInputs: inputs}, fmt.Sprintf("exp5-vector-%d", count), run, !smoke)
				if er != nil {
					return er
				}
				fmt.Printf("State count %d run %d complete\n", count, run)
			}
			rows = append(rows, en.Rows...)
			preparations = append(preparations, en.Preparations...)
			deployments = append(deployments, en.Deployments...)
		}
		en, er := NewEngine(root, ctx, true)
		if er != nil {
			return er
		}
		defer en.Client.Close()
		var reset any
		if er = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); er != nil {
			return er
		}
		if er = en.Initialize(0); er != nil {
			return er
		}
		w := NewWorld(defaultPK(), 59000000)
		sk := h.Element(22)
		doc := must(document.Hash(DefaultDocument()))
		input, er := en.Execute(w, Operation{P: must(ProfileByName("entry-audit")), SK: sk, Document: doc, State: note.State{QMass: 300e9, ARec: 30e9, E: 261e9}}, "exp5-claim-prepare", -1, false)
		if er != nil {
			return er
		}
		prep = append(prep, en.Rows...)
		en.Rows = nil
		snap, er := en.Snapshot()
		if er != nil {
			return er
		}
		for _, name := range []string{"issue-recycled-audit", "issue-carbon-audit", "issue-audit"} {
			for run := 0; run <= last; run++ {
				snap, er = en.Restore(snap)
				if er != nil {
					return er
				}
				world := w.Clone()
				world.Serial += uint64(run) * 10000
				_, er = en.Execute(world, Operation{P: must(ProfileByName(name)), SK: sk, Inputs: input.Outputs}, "exp5-"+name, run, !smoke)
				if er != nil {
					return er
				}
			}
		}
		rows = append(rows, en.Rows...)
		preparations = append(preparations, en.Preparations...)
		deployments = append(deployments, en.Deployments...)
	}
	return saveExperiment(root, exp, smoke, map[string]any{"rows": rows, "preparationRows": prep, "preparations": preparations, "deployments": deployments})
}
func equalState(m, i int) note.State {
	v := []uint64{320e9, 30e9, 231e9}
	for j, x := range v {
		q := x / uint64(m)
		if i == 0 {
			q = x - q*uint64(m-1)
		}
		v[j] = q
	}
	return note.State{QMass: v[0], ARec: v[1], E: v[2]}
}
func saveExperiment(root string, exp int, smoke bool, value map[string]any) error {
	value["experiment"] = exp
	value["status"] = "complete"
	value["warmups"] = 1
	value["repetitions"] = 5
	value["smoke"] = smoke
	p := filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", exp))
	if smoke {
		value["status"] = "development-smoke"
		value["configuredRepetitions"] = 5
		value["repetitions"] = 0
		p = filepath.Join(root, ArtifactDir, "attempts", fmt.Sprintf("smoke-%d-%d.json", exp, time.Now().UnixNano()))
	}
	return writeJSON(p, value)
}
