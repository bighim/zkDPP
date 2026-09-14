package experiments

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	audit "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"math/big"
	"path/filepath"
	"time"
)

type Graph struct {
	World          *World
	Start, Last    audit.Ref
	Leaves         []note.Note
	Records, Depth int
	PreparationMS  float64
	Preparation    []Measurement
}

func buildGraph(en *Engine, shape string, size int, seed uint64) (g Graph, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("graph: %v", x)
		}
	}()
	start := time.Now()
	w := NewWorld(defaultPK(), seed)
	g.World = w
	sk := h.Element(22)
	doc := must(document.Hash(DefaultDocument()))
	q := uint64(size) * Scale
	if shape == "diamond" {
		q = 2 * Scale
	}
	counter := 0
	exec := func(name string, op Operation) *Event {
		op.P = must(ProfileByName(name + "-audit"))
		counter++
		ev := must(en.Execute(w, op, fmt.Sprintf("exp4-%s-%d-prepare-%04d", shape, size, counter), -1, false))
		if counter%25 == 0 {
			fmt.Printf("graph %s %d: %d records prepared\n", shape, size, counter)
		}
		return ev
	}
	initial := exec("entry", Operation{SK: sk, Document: doc, State: note.State{QMass: q, ARec: q / 10, E: q / 2}}).Outputs[0]
	g.Start = audit.Ref{ObjectType: 1, RawID: initial.Commitment}
	if shape == "diamond" {
		n := initial
		for i := 0; i < size; i++ {
			out := exec("split", Operation{SK: sk, Inputs: []note.Note{n}, Q: Scale}).Outputs
			n = exec("merge", Operation{SK: sk, Inputs: out}).Outputs[0]
		}
		g.Leaves = []note.Note{n}
		g.Last = audit.Ref{ObjectType: 1, RawID: n.Commitment}
		g.Depth = 2 * size
	} else {
		var split func(note.Note, int, int)
		split = func(n note.Note, leaves, depth int) {
			if leaves == 1 {
				g.Leaves = append(g.Leaves, n)
				if depth >= g.Depth {
					g.Last = audit.Ref{ObjectType: 1, RawID: n.Commitment}
					g.Depth = depth
				}
				return
			}
			left := (leaves + 1) / 2
			if shape == "chain" {
				left = leaves - 1
			}
			outs := exec("split", Operation{SK: sk, Inputs: []note.Note{n}, Q: uint64(left) * Scale}).Outputs
			split(outs[0], left, depth+1)
			split(outs[1], leaves-left, depth+1)
		}
		split(initial, size, 0)
	}
	g.Records = counter
	g.PreparationMS = ms(time.Since(start))
	g.Preparation = append([]Measurement{}, en.Rows...)
	en.Rows = nil
	return g, nil
}

type AuditRow struct {
	RecordCacheHits                                                                                    int
	PreparedTraceMetrics                                                                               *audit.Metrics `json:",omitempty"`
	Case, Operation                                                                                    string
	Run                                                                                                int
	Warmup                                                                                             bool
	Records, Depth, VisitedObjects, UniqueRecords, Decryptions, Edges, Targets, Transactions, RPCCalls int
	TotalMS, RPCMS                                                                                     float64
	Gas                                                                                                uint64
	Outcome                                                                                            string
	Snapshot, Checkpoint                                                                               audit.Snapshot
	Matched                                                                                            bool
}

func graphMatch(g Graph, r audit.TraceResult) error {
	if len(r.Frontier) != len(g.Leaves) || len(r.Claims) != 0 || len(r.Exits) != 0 {
		return fmt.Errorf("frontier count/terminal mismatch")
	}
	want := map[string]bool{}
	for _, n := range g.Leaves {
		want[n.Commitment.String()] = true
	}
	for _, f := range r.Frontier {
		if !want[f.Ref.RawID.String()] {
			return fmt.Errorf("unexpected frontier")
		}
	}
	if r.Metrics.UniqueRecords != g.Records {
		return fmt.Errorf("unique records %d != %d", r.Metrics.UniqueRecords, g.Records)
	}
	return nil
}
func RunGraphs(root string, ctx context.Context, smoke bool) error {
	en, er := NewEngine(root, ctx, true)
	if er != nil {
		return er
	}
	defer en.Client.Close()
	if er = en.Initialize(-1); er != nil {
		return er
	}
	base, er := en.Snapshot()
	if er != nil {
		return er
	}
	pkg, ss, er := v2case.KeyFixture()
	if er != nil {
		return er
	}
	master, er := auditcrypto.RecoverMasterKey(pkg, ss[:2])
	if er != nil {
		return er
	}
	defer master.SetInt64(0)
	defer func() {
		for _, s := range ss {
			s.Value.SetInt64(0)
		}
	}()
	var rows []AuditRow
	var preparations []map[string]any
	var deployments []map[string]any
	var nodePreparations []Transaction
	configs := []struct {
		shape string
		size  int
	}{{"chain", 10}, {"balanced", 10}, {"diamond", 5}, {"chain", 100}, {"balanced", 100}, {"diamond", 50}, {"chain", 1000}, {"balanced", 1000}, {"diamond", 500}}
	last := Repetitions
	if smoke {
		configs = configs[:3]
		last = 0
	}
	for ci, cfg := range configs {
		if ci > 0 {
			deployments = append(deployments, en.Deployments...)
			nodePreparations = append(nodePreparations, en.Preparations...)
			var reset any
			if er = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); er != nil {
				return er
			}
			en.Client.Close()
			en, er = NewEngine(root, ctx, true)
			if er != nil {
				return er
			}
			defer en.Client.Close()
			if er = en.Initialize(-1); er != nil {
				return er
			}
			base, er = en.Snapshot()
			if er != nil {
				return er
			}
		}
		base, er = en.Restore(base)
		if er != nil {
			return er
		}
		g, er := buildGraph(en, cfg.shape, cfg.size, 40000000+uint64(ci)*1000000)
		if er != nil {
			return er
		}
		preparations = append(preparations, map[string]any{"shape": cfg.shape, "records": g.Records, "depth": g.Depth, "millis": g.PreparationMS, "transactions": g.Preparation})
		snapshot, er := en.Snapshot()
		if er != nil {
			return er
		}
		label := fmt.Sprintf("%s-%d", cfg.shape, g.Records)
		for _, operation := range []string{"forward", "backward", "freeze", "audit-and-freeze"} {
			for run := 0; run <= last; run++ {
				snapshot, er = en.Restore(snapshot)
				if er != nil {
					return er
				}
				en.Source.FreezeTransactions = nil
				source := audit.NewCachedSource(en.Source)
				var prepared audit.TraceResult
				snap, er := en.Source.Checkpoint(ctx)
				if er != nil {
					return er
				}
				if operation == "freeze" {
					prepared, er = audit.TraceForward(ctx, source, master, g.Start, snap)
					if er != nil {
						return er
					}
					if er = graphMatch(g, prepared); er != nil {
						return er
					}
				}
				beforeCalls, beforeRPC := en.Meter.Calls, en.Meter.Millis
				start := time.Now()
				row := AuditRow{Case: label, Operation: operation, Run: run, Warmup: run == 0, Records: g.Records, Depth: g.Depth, Snapshot: snap}
				if operation == "forward" || operation == "backward" {
					var r audit.TraceResult
					if operation == "forward" {
						r, er = audit.TraceForward(ctx, source, master, g.Start, snap)
					} else {
						r, er = audit.TraceBackward(ctx, source, master, g.Last, snap)
					}
					row.TotalMS = ms(time.Since(start))
					row.RPCMS = en.Meter.Millis - beforeRPC
					row.RPCCalls = en.Meter.Calls - beforeCalls
					if er != nil {
						return er
					}
					if operation == "forward" {
						if er = graphMatch(g, r); er != nil {
							return er
						}
					} else {
						if len(r.Entries) != 1 || !r.Entries[0].RawID.Equal(&g.Start.RawID) {
							return fmt.Errorf("backward Entry mismatch")
						}
						expected := g.Depth + 1
						if cfg.shape == "diamond" {
							expected = g.Records
						}
						if r.Metrics.UniqueRecords != expected {
							return fmt.Errorf("backward records %d != %d", r.Metrics.UniqueRecords, expected)
						}
					}
					row.VisitedObjects = r.Metrics.VisitedObjects
					row.UniqueRecords = r.Metrics.UniqueRecords
					row.Decryptions = r.Metrics.Decryptions
					row.Edges = len(r.Graph)
					row.Targets = len(r.Frontier)
					row.Outcome = "TRACE_COMPLETE"
				} else {
					var result audit.Result
					if operation == "freeze" {
						result = audit.FreezeTargets(ctx, source, prepared, snap)
					} else {
						x, _, e := recoverCurrent(pkg, ss[:])
						if e != nil {
							return e
						}
						snap, e = en.Source.Checkpoint(ctx)
						if e != nil {
							x.SetInt64(0)
							return e
						}
						row.Snapshot = snap
						result = audit.AuditAndFreeze(ctx, source, x, g.Start, snap)
						x.SetInt64(0)
					}
					row.TotalMS = ms(time.Since(start))
					row.RPCMS = en.Meter.Millis - beforeRPC
					row.RPCCalls = en.Meter.Calls - beforeCalls
					row.Checkpoint = result.Checkpoint
					row.Outcome = string(result.Outcome)
					row.Targets = len(result.Targets)
					row.Transactions = len(en.Source.FreezeTransactions)
					row.VisitedObjects = result.Trace.Metrics.VisitedObjects
					row.UniqueRecords = result.Trace.Metrics.UniqueRecords
					row.Decryptions = result.Trace.Metrics.Decryptions
					row.Edges = len(result.Trace.Graph)
					for _, t := range en.Source.FreezeTransactions {
						row.Gas += t.GasUsed
					}
					if result.Outcome != audit.CompleteAtCheckpoint || len(result.Targets) != len(g.Leaves) || row.Transactions != len(g.Leaves) {
						return fmt.Errorf("Freeze incomplete %s %s", label, result.Reason)
					}
					for _, t := range result.TargetResults {
						if t.FinalSpent || t.FinalStatus != audit.Frozen || !t.Blocked {
							return fmt.Errorf("Freeze target mismatch")
						}
					}
				}
				row.RecordCacheHits = source.Hits
				if operation == "freeze" {
					metrics := prepared.Metrics
					row.PreparedTraceMetrics = &metrics
					row.VisitedObjects = 0
					row.UniqueRecords = 0
					row.Decryptions = 0
					row.Edges = 0
					row.RecordCacheHits = 0
				}
				row.Matched = true
				rows = append(rows, row)
				if er = writeJSON(filepath.Join(root, ArtifactDir, "progress", en.RunTag, fmt.Sprintf("exp4-%s-%s-%d.json", label, operation, run)), row); er != nil {
					return er
				}
				fmt.Printf("audit %s %s run %d: %.2f ms\n", label, operation, run, row.TotalMS)
			}
		}
	}
	base, er = en.Restore(base)
	if er != nil {
		return er
	}
	negative, er := graphFailures(en, pkg, ss[:])
	if er != nil {
		return er
	}
	deployments = append(deployments, en.Deployments...)
	nodePreparations = append(nodePreparations, en.Preparations...)
	return saveExperiment(root, 4, smoke, map[string]any{"rows": rows, "graphPreparations": preparations, "deployments": deployments, "preparations": nodePreparations, "correctness": negative, "nodeStepTracing": false, "cleanNodePerGraph": true})
}

func graphFailures(en *Engine, pkg auditcrypto.ExternalKeyPackage, shares []auditcrypto.Share) (map[string]bool, error) {
	g, er := buildGraph(en, "balanced", 3, 49000000)
	if er != nil {
		return nil, er
	}
	save, er := en.Snapshot()
	if er != nil {
		return nil, er
	}
	master, er := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if er != nil {
		return nil, er
	}
	defer master.SetInt64(0)
	snap, er := en.Source.Checkpoint(en.Ctx)
	if er != nil {
		return nil, er
	}
	_, er = en.Execute(g.World, Operation{P: must(ProfileByName("exit-audit")), SK: h.Element(22), Inputs: []note.Note{g.Leaves[0]}}, "exp4-post-snapshot-exit", -1, false)
	if er != nil {
		return nil, er
	}
	en.Source.FreezeTransactions = nil
	r := audit.AuditAndFreeze(en.Ctx, audit.NewCachedSource(en.Source), master, g.Start, snap)
	if r.Outcome != audit.Incomplete || len(r.Targets) != 3 || len(en.Source.FreezeTransactions) != 2 {
		return nil, fmt.Errorf("post-snapshot failure semantics")
	}
	save, er = en.Restore(save)
	if er != nil {
		return nil, er
	}
	for i, n := range g.Leaves {
		nf := note.Nullifier(h.Element(22), n.Commitment)
		if er = en.call("setStatus", uint8(1), audit.Big(nf), uint8(1)); er != nil {
			return nil, er
		}
		if i == 1 {
			if er = en.call("setStatus", uint8(1), audit.Big(nf), uint8(2)); er != nil {
				return nil, er
			}
		}
	}
	en.Source.FreezeTransactions = nil
	snap, er = en.Source.Checkpoint(en.Ctx)
	if er != nil {
		return nil, er
	}
	r = audit.AuditAndFreeze(en.Ctx, audit.NewCachedSource(en.Source), master, g.Start, snap)
	if r.Outcome != audit.CompleteAtCheckpoint || len(en.Source.FreezeTransactions) != 0 {
		return nil, fmt.Errorf("preblocked frontier semantics")
	}
	en.Source.Context.ChainID++
	r = audit.AuditAndFreeze(en.Ctx, audit.NewCachedSource(en.Source), master, g.Start, snap)
	en.Source.Context.ChainID--
	if r.Outcome != audit.TraceFailed || len(en.Source.FreezeTransactions) != 0 {
		return nil, fmt.Errorf("trace failure submitted freeze")
	}
	return map[string]bool{"postSnapshotConsumedIncomplete": true, "preFrozenRevokedCompleteWithoutTransactions": true, "wrongChainTraceFailedWithoutTransactions": true}, nil
}
func recoverCurrent(p auditcrypto.ExternalKeyPackage, s []auditcrypto.Share) (*big.Int, float64, error) {
	start := time.Now()
	released := []auditcrypto.ReleasedShare{}
	for _, v := range s[:2] {
		released = append(released, auditcrypto.ReleasedShare{Profile: p.Profile, SessionID: p.SessionID, PackageChecksum: auditcrypto.PackageChecksum(p), Share: v})
	}
	x, e := auditcrypto.RecoverReleased(p, released)
	return x, ms(time.Since(start)), e
}
