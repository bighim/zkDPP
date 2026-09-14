package experiments

import (
	"context"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	audit "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments/audit"
	"os"
	"path/filepath"
	"time"
)

type DPPRow struct {
	Run                     int
	Warmup                  bool
	TotalMS, RPCMS, LocalMS float64
	RPCCalls                int
	Matched                 bool
}

func (e *Engine) VerifyClaim(cl claim.DPPClaim, run int) (DPPRow, error) {
	row := DPPRow{Run: run, Warmup: run == 0}
	calls, rpcms := e.Meter.Calls, e.Meter.Millis
	start := time.Now()
	snap, er := e.Source.Checkpoint(e.Ctx)
	if er != nil {
		return row, er
	}
	if er = e.Source.CheckSnapshot(e.Ctx, snap); er != nil {
		return row, er
	}
	doc, er := document.Hash(DefaultDocument())
	if er != nil {
		return row, er
	}
	got := claim.Handle(doc, cl.IssuePolicyRef, cl.ClaimNonce)
	if !got.Equal(&cl.Handle) {
		return row, fmt.Errorf("DPP hash")
	}
	expected := must(ProfileByName("issue-audit")).PolicyRef
	if !expected.Equal(&cl.IssuePolicyRef) {
		return row, fmt.Errorf("unexpected Policy")
	}
	registered, er := e.Source.ClaimRegistered(e.Ctx, got, snap)
	if er != nil || !registered {
		return row, fmt.Errorf("DPP unregistered: %v", er)
	}
	id, er := e.Source.Producer(e.Ctx, audit.Ref{ObjectType: 3, RawID: got}, snap)
	if er != nil || id == 0 {
		return row, fmt.Errorf("DPP producer: %v", er)
	}
	r, er := e.Source.Record(e.Ctx, id, snap)
	if er != nil {
		return row, er
	}
	if r.EventKind != audit.Issue || !r.PolicyRef.Equal(&expected) || len(r.OutputRefs) != 1 || !r.OutputRefs[0].RawID.Equal(&got) {
		return row, fmt.Errorf("DPP Issue record")
	}
	if er = e.Source.CheckIssuePolicy(e.Ctx, expected, snap); er != nil {
		return row, er
	}
	if er = e.Source.CheckSnapshot(e.Ctx, snap); er != nil {
		return row, er
	}
	row.TotalMS = ms(time.Since(start))
	row.RPCMS = e.Meter.Millis - rpcms
	row.LocalMS = row.TotalMS - row.RPCMS
	row.RPCCalls = e.Meter.Calls - calls
	row.Matched = true
	return row, nil
}
func (e *Engine) Lifecycle(exp, run int, trace bool) (claim.DPPClaim, error) {
	w := NewWorld(defaultPK(), 10000000+uint64(run)*10000)
	doc := must(document.Hash(DefaultDocument()))
	sender, factory := h.Element(11), h.Element(22)
	suffix := "-plain"
	if e.Audit {
		suffix = "-audit"
	}
	index := 0
	exec := func(name string, op Operation) *Event {
		op.P = must(ProfileByName(name + suffix))
		index++
		ev, er := e.Execute(w, op, fmt.Sprintf("exp%d-lifecycle-%02d", exp, index), run, trace)
		if er != nil {
			panic(er)
		}
		return ev
	}
	init := note.State{QMass: 160e9, ARec: 15e9, E: 115e9}
	a := exec("entry", Operation{SK: sender, Document: doc, State: init}).Outputs[0]
	b := exec("entry", Operation{SK: sender, Document: doc, State: init}).Outputs[0]
	v := exec("transfer", Operation{SK: sender, Inputs: []note.Note{a}, Receiver: owner.Address(factory), Transport: 1e9}).Voucher
	ret := exec("recall", Operation{SK: sender, Voucher: *v}).Outputs[0]
	v = exec("transfer", Operation{SK: sender, Inputs: []note.Note{ret}, Receiver: owner.Address(factory)}).Voucher
	aa := exec("proceed", Operation{SK: factory, Voucher: *v}).Outputs[0]
	v = exec("transfer", Operation{SK: sender, Inputs: []note.Note{b}, Receiver: owner.Address(factory)}).Voucher
	bb := exec("proceed", Operation{SK: factory, Voucher: *v}).Outputs[0]
	merged := exec("merge", Operation{SK: factory, Inputs: []note.Note{aa, bb}}).Outputs[0]
	if merged.State != (note.State{QMass: 320e9, ARec: 30e9, E: 231e9}) {
		return claim.DPPClaim{}, fmt.Errorf("Merge expected total")
	}
	ps := exec("split", Operation{SK: factory, Inputs: []note.Note{merged}, Q: 160e9}).Outputs
	product := exec("process-2-1", Operation{SK: factory, Inputs: ps, Document: doc}).Outputs[0]
	if product.State != (note.State{QMass: 300e9, ARec: 30e9, E: 261e9}) {
		return claim.DPPClaim{}, fmt.Errorf("Process expected total")
	}
	products := exec("split", Operation{SK: factory, Inputs: []note.Note{product}, Q: 200e9}).Outputs
	c := exec("issue", Operation{SK: factory, Inputs: []note.Note{products[0]}}).Claim
	exec("exit", Operation{SK: factory, Inputs: []note.Note{products[1]}})
	return *c, nil
}
func Run(root string, exp int, smoke bool) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("experiment panic: %v", x)
		}
	}()
	if exp < 1 || exp > 8 {
		return fmt.Errorf("experiment outside 1..8")
	}
	if er := CheckBaseline(root); er != nil {
		return er
	}
	target := filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", exp))
	if _, er := os.Stat(target); er == nil {
		return fmt.Errorf("result exists; validate before reuse")
	}
	if exp == 7 || exp == 8 {
		if smoke {
			return fmt.Errorf("use crypto unit tests for development; smoke is not an official crypto run")
		}
		return RunCrypto(root, exp)
	}
	stop, er := StartNode(root, exp == 4)
	if er != nil {
		return er
	}
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	if exp == 2 {
		return Routes(root, ctx, smoke)
	}
	if exp == 4 {
		return RunGraphs(root, ctx, smoke)
	}
	if exp == 5 || exp == 6 {
		return Variants(root, ctx, exp, smoke)
	}
	if exp == 1 || exp == 3 {
		audited := exp == 1
		en, er := NewEngine(root, ctx, audited)
		if er != nil {
			return er
		}
		defer en.Client.Close()
		if er = en.Initialize(0); er != nil {
			return er
		}
		snapshot, er := en.Snapshot()
		if er != nil {
			return er
		}
		var dpp []DPPRow
		last := Repetitions
		if smoke {
			last = 0
		}
		for run := 0; run <= last; run++ {
			if run > 0 {
				snapshot, er = en.Restore(snapshot)
				if er != nil {
					return er
				}
			}
			cl, er := en.Lifecycle(exp, run, !smoke)
			if er != nil {
				return er
			}
			if audited {
				row, er := en.VerifyClaim(cl, run)
				if er != nil {
					return er
				}
				dpp = append(dpp, row)
			}
			fmt.Printf("experiment %d lifecycle run %d complete\n", exp, run)
		}
		result := map[string]any{"experiment": exp, "status": "complete", "warmups": 1, "repetitions": 5, "rows": en.Rows, "dpp": dpp, "preparations": en.Preparations, "deployments": en.Deployments, "smoke": smoke}
		if smoke {
			result["status"] = "development-smoke"
			result["configuredRepetitions"] = 5
			result["repetitions"] = 0
			target = filepath.Join(root, ArtifactDir, "attempts", fmt.Sprintf("smoke-%d-%d.json", exp, time.Now().UnixNano()))
		}
		return writeJSON(target, result)
	}
	return fmt.Errorf("experiment %d runner pending implementation", exp)
}
