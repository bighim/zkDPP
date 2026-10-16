package hotfix

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

func objMap(ex v2case.IntegratedExpected) map[string]v2case.IntegratedObject {
	m := map[string]v2case.IntegratedObject{}
	for _, o := range ex.Objects {
		m[o.Label] = o
	}
	return m
}
func expectedTargets(ex v2case.IntegratedExpected) []v2audit.Frontier {
	m := objMap(ex)
	var t []v2audit.Frontier
	for _, l := range ex.Targets {
		o := m[l]
		t = append(t, v2audit.Frontier{Ref: o.Ref, SpendValue: o.Spend})
	}
	return t
}
func frontierKeys(ts []v2audit.Frontier) []string {
	var ks []string
	for _, t := range ts {
		ks = append(ks, t.Ref.Key()+":"+hex.EncodeToString(t.SpendValue[:]))
	}
	sort.Strings(ks)
	return ks
}
func refKeys(rs []v2audit.Ref) []string {
	var ks []string
	for _, r := range rs {
		ks = append(ks, r.Key())
	}
	sort.Strings(ks)
	return ks
}
func edgeKeys(es []v2audit.Edge) []string {
	var ks []string
	for _, e := range es {
		ks = append(ks, fmt.Sprintf("%s>%d>%s", e.Parent.Key(), e.RecordID, e.Child.Key()))
	}
	sort.Strings(ks)
	return ks
}
func expectedGraph(ex v2case.IntegratedExpected) ([]v2audit.Ref, []v2audit.Edge) {
	m := objMap(ex)
	seen := map[string]bool{}
	q := []string{ex.Start}
	var refs []v2audit.Ref
	var edges []v2audit.Edge
	for len(q) > 0 {
		l := q[0]
		q = q[1:]
		if seen[l] {
			continue
		}
		seen[l] = true
		refs = append(refs, m[l].Ref)
		for _, s := range ex.Steps {
			for _, input := range s.Inputs {
				if input == l {
					for _, out := range s.Outputs {
						edges = append(edges, v2audit.Edge{Parent: m[l].Ref, Child: m[out].Ref, RecordID: s.Aid})
						q = append(q, out)
					}
				}
			}
		}
	}
	return refs, edges
}
func (e *EVM) validateIntegrated(f IntegratedProofFile, master *big.Int, h v2audit.Snapshot, out string) error {
	m := objMap(f.Expected)
	var recordRows []map[string]any
	for _, step := range f.Expected.Steps {
		r, err := e.source.Record(e.ctx, step.Aid, h)
		if err != nil {
			return err
		}
		ev := f.Base.Events[step.Aid-1]
		if r.EventKind != ev.Record.EventKind || !r.PolicyRef.Equal(&ev.Record.PolicyRef) || !reflect.DeepEqual(r.OutputRefs, ev.Record.OutputRefs) || !r.R1.X.Equal(&ev.Record.R1.X) || !r.R1.Y.Equal(&ev.Record.R1.Y) || !v2audit.Equal(r.EncryptedParents, ev.Record.EncryptedParents) || !v2audit.Equal(r.EncryptedOutputNfs, ev.Record.EncryptedOutputNfs) {
			return fmt.Errorf("stored record mismatch: %s", step.Name)
		}
		plain, err := auditcrypto.DecryptWithMasterKey(master, r.Ciphertext())
		if err != nil {
			return err
		}
		var want []fr.Element
		for _, l := range step.Inputs {
			want = append(want, m[l].Ref.RawID)
		}
		var outs []v2audit.Ref
		for _, l := range step.Outputs {
			o := m[l]
			outs = append(outs, o.Ref)
			sid, err := v2audit.Field(new(big.Int).SetBytes(o.Spend[:]))
			if err != nil {
				return err
			}
			want = append(want, sid)
		}
		if !v2audit.Equal(plain, want) || !reflect.DeepEqual(r.OutputRefs, outs) {
			return fmt.Errorf("independent parent/future/output mismatch: %s", step.Name)
		}
		recordRows = append(recordRows, map[string]any{"aid": step.Aid, "event": step.Name, "storedRecord": &r, "decryptedParentFutureVector": plain, "independentExpectedVector": want, "matched": true, "snapshot": h})
	}
	if err := Write(filepath.Join(out, "raw/actual-records.json"), recordRows); err != nil {
		return err
	}
	for _, o := range f.Expected.Objects {
		p, err := e.source.Producer(e.ctx, o.Ref, h)
		if err != nil {
			return err
		}
		s, err := e.source.SpentIn(e.ctx, o.Ref.ObjectType, o.Spend, h)
		if err != nil {
			return err
		}
		if p != o.Producer || s != o.Consumer {
			return fmt.Errorf("independent index mismatch: %s", o.Label)
		}
	}
	trace, err := v2audit.TraceForward(e.ctx, e.source, master, f.Base.Start, h)
	if err != nil {
		return err
	}
	refs, edges := expectedGraph(f.Expected)
	if !reflect.DeepEqual(refKeys(trace.Objects), refKeys(refs)) || !reflect.DeepEqual(edgeKeys(trace.Graph), edgeKeys(edges)) || !reflect.DeepEqual(frontierKeys(trace.Frontier), frontierKeys(expectedTargets(f.Expected))) {
		return fmt.Errorf("independent graph/target mismatch")
	}
	if len(trace.Claims) != 0 || len(trace.Exits) != 0 {
		return fmt.Errorf("unexpected terminal")
	}
	return nil
}
func (e *EVM) stateEvidence(ex v2case.IntegratedExpected) (map[string]any, error) {
	h, err := e.source.Checkpoint(e.ctx)
	if err != nil {
		return nil, err
	}
	row := map[string]any{"snapshot": h}
	for _, name := range []string{"currentNoteRoot", "currentVoucherRoot", "noteLeafCount", "voucherLeafCount", "nextAuditRecordId"} {
		v, err := e.view(name)
		if err != nil {
			return nil, err
		}
		row[name] = v[0].(*big.Int).String()
	}
	var objects []map[string]any
	for _, o := range ex.Objects {
		s, err := e.source.SpentIn(e.ctx, o.Ref.ObjectType, o.Spend, h)
		if err != nil {
			return nil, err
		}
		status, spent, err := e.source.Status(e.ctx, o.Ref.ObjectType, o.Spend, h)
		if err != nil {
			return nil, err
		}
		p, err := e.source.Producer(e.ctx, o.Ref, h)
		if err != nil {
			return nil, err
		}
		method := "commitments"
		if o.Ref.ObjectType == v2audit.Voucher {
			method = "voucherCommitments"
		}
		v, err := e.view(method, v2audit.Big(o.Ref.RawID))
		if err != nil {
			return nil, err
		}
		objects = append(objects, map[string]any{"label": o.Label, "ref": &o.Ref, "spend": o.Spend, "producer": p, "spentIn": s, "status": status, "spent": spent, "exists": v[0]})
	}
	row["objects"] = objects
	return row, nil
}
func equivalentState(a, b map[string]any) bool {
	aa := map[string]any{}
	bb := map[string]any{}
	for k, v := range a {
		if k != "snapshot" {
			aa[k] = v
		}
	}
	for k, v := range b {
		if k != "snapshot" {
			bb[k] = v
		}
	}
	return reflect.DeepEqual(aa, bb)
}
func (e *EVM) reset(snapshot string) error {
	ledgerContext := e.source.Context
	var ok bool
	if err := e.c.Client().CallContext(e.ctx, &ok, "evm_revert", snapshot); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("evm revert failed")
	}
	e.source = v2audit.NewRPCSource(e.c, e.a, e.ledger, e.from)
	e.source.Context = ledgerContext
	return nil
}
func (e *EVM) saveTransactions(out, branch string, start int, extra []common.Hash) error {
	rows := []map[string]any{}
	var hashes []common.Hash
	for _, t := range e.report.Transactions[start:] {
		hashes = append(hashes, common.HexToHash(t.Hash))
	}
	hashes = append(hashes, extra...)
	for _, h := range hashes {
		tx, _, err := e.c.TransactionByHash(e.ctx, h)
		if err != nil {
			return err
		}
		receipt, err := e.c.TransactionReceipt(e.ctx, h)
		if err != nil {
			return err
		}
		header, err := e.c.HeaderByNumber(e.ctx, receipt.BlockNumber)
		if err != nil {
			return err
		}
		rows = append(rows, map[string]any{"transaction": tx, "receipt": receipt, "header": header})
	}
	return Write(filepath.Join(out, "raw", branch+"-transactions.json"), rows)
}
func validateResult(ex v2case.IntegratedExpected, r v2audit.Result, want v2audit.Outcome) error {
	var rows []v2audit.Frontier
	for _, row := range r.TargetResults {
		rows = append(rows, row.Target)
		if row.Observation.BlockNumber > r.Checkpoint.BlockNumber {
			return fmt.Errorf("checkpoint predates observation")
		}
		if row.Transaction != nil && row.Transaction.BlockNumber > r.Checkpoint.BlockNumber {
			return fmt.Errorf("checkpoint predates receipt")
		}
	}
	keys := frontierKeys(expectedTargets(ex))
	if !reflect.DeepEqual(keys, frontierKeys(r.Targets)) || !reflect.DeepEqual(keys, frontierKeys(rows)) {
		return fmt.Errorf("original target/result set altered")
	}
	if r.Outcome != want || r.Checkpoint.BlockNumber < r.Snapshot.BlockNumber {
		return fmt.Errorf("outcome/checkpoint mismatch: got %s want %s", r.Outcome, want)
	}
	return nil
}

func waitIntegratedReceipt(ctx context.Context, c *ethclient.Client, h common.Hash) (*types.Receipt, error) {
	for i := 0; i < 100; i++ {
		r, err := c.TransactionReceipt(ctx, h)
		if err == nil {
			return r, nil
		}
		if err != ethereum.NotFound {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("receipt confirmation deadline: %s", h.Hex())
}
func IntegrationEVM(root, out, url string) (err error) {
	target := filepath.Join(out, "raw/evm-report.json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("EVM report exists")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	c, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return err
	}
	defer c.Close()
	var accounts []common.Address
	if err = c.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil {
		return err
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no local unlocked account")
	}
	var f IntegratedProofFile
	if err = Read(filepath.Join(out, "raw/proofs.json"), &f); err != nil {
		return err
	}
	report := EVMReport{Mode: "integrated-containment", ChainID: 31337}
	report.FixtureHash, _ = Hash(filepath.Join(out, "raw/proofs.json"))
	e := &EVM{root: filepath.Join(out, "setup"), ctx: ctx, c: c, from: accounts[0], report: &report}
	defer func() {
		_ = Write(target, report)
		if err != nil {
			_ = Write(filepath.Join(out, "raw/evm-failure.json"), map[string]any{"error": err.Error()})
			_ = e.saveTransactions(out, "failed-attempt", 0, nil)
		}
	}()
	if err = e.initialize(); err != nil {
		return err
	}
	if err = e.group(f.Base); err != nil {
		return err
	}
	pkg, shares, err := v2case.KeyFixture()
	if err != nil {
		return err
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[1]})
	if err != nil {
		return err
	}
	defer master.SetInt64(0)
	ht, err := e.source.Checkpoint(ctx)
	if err != nil {
		return err
	}
	if err = e.validateIntegrated(f, master, ht, out); err != nil {
		return err
	}
	if err = e.saveTransactions(out, "base", 0, nil); err != nil {
		return err
	}
	baseState, err := e.stateEvidence(f.Expected)
	if err != nil {
		return err
	}
	if err = Write(filepath.Join(out, "raw/base-state.json"), baseState); err != nil {
		return err
	}
	trace, err := v2audit.TraceForward(ctx, e.source, master, f.Base.Start, ht)
	if err != nil {
		return err
	}
	if err = Write(filepath.Join(out, "raw/trace-comparison.json"), map[string]any{"expected": &f.Expected, "actual": &trace, "passed": true, "edgeRepresentation": "Each reached object-to-consumer-to-output path is compared as a folded (parent, consumerAid, child) edge; unreached co-input ancestors are not expanded."}); err != nil {
		return err
	}
	var baseSnapshot string
	if err = c.Client().CallContext(ctx, &baseSnapshot, "evm_snapshot"); err != nil {
		return err
	}
	fresh := func() error {
		if err := e.reset(baseSnapshot); err != nil {
			return err
		}
		return c.Client().CallContext(ctx, &baseSnapshot, "evm_snapshot")
	}
	// Freeze once for the normal case, then restore that exact frozen state for
	// every blocked probe. Active controls independently restore the base.
	start := len(report.Transactions)
	normal := v2audit.AuditAndFreeze(ctx, e.source, master, f.Base.Start, ht)
	if err = validateResult(f.Expected, normal, v2audit.CompleteAtCheckpoint); err != nil {
		return err
	}
	if err = Write(filepath.Join(out, "raw/normal-client.json"), &normal); err != nil {
		return err
	}
	var hashes []common.Hash
	for _, tx := range e.source.FreezeTransactions {
		hashes = append(hashes, common.HexToHash(tx.Hash))
	}
	if err = e.saveTransactions(out, "normal", start, hashes); err != nil {
		return err
	}
	ground, err := e.stateEvidence(f.Expected)
	if err != nil {
		return err
	}
	if err = Write(filepath.Join(out, "raw/normal-ground-truth.json"), ground); err != nil {
		return err
	}
	var frozenSnapshot string
	if err = c.Client().CallContext(ctx, &frozenSnapshot, "evm_snapshot"); err != nil {
		return err
	}
	for _, probe := range f.Probes {
		before, err := e.stateEvidence(f.Expected)
		if err != nil {
			return err
		}
		data, err := eventData(e.a, probe)
		if err != nil {
			return err
		}
		var txhash common.Hash
		// Force a real mined transaction even though estimation would revert.
		if err = c.Client().CallContext(ctx, &txhash, "eth_sendTransaction", map[string]any{"from": e.from, "to": e.ledger, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(30000000)}); err != nil {
			return err
		}
		receipt, err := waitIntegratedReceipt(ctx, c, txhash)
		if err != nil {
			return err
		}
		var tr struct {
			Failed      bool
			ReturnValue string
		}
		if err = c.Client().CallContext(ctx, &tr, "debug_traceTransaction", txhash, map[string]any{"disableStorage": true, "disableStack": true, "enableMemory": false}); err != nil {
			return err
		}
		after, err := e.stateEvidence(f.Expected)
		if err != nil {
			return err
		}
		expected := "InvalidState"
		if probe.Name == "transfer-U1" {
			expected = "accepted"
		}
		pass := receipt.Status == 0 && tr.Failed && bytes.Equal(common.FromHex(tr.ReturnValue), e.a.Errors["InvalidState"].ID.Bytes()[:4]) && equivalentState(before, after)
		if expected == "accepted" {
			pass = receipt.Status == 1 && !tr.Failed
		}
		row := map[string]any{"probe": probe.Name, "expected": expected, "receipt": receipt, "traceFailed": tr.Failed, "returnValue": tr.ReturnValue, "before": before, "after": after, "passed": pass}
		if err = Write(filepath.Join(out, "raw/frozen-"+probe.Name+".json"), row); err != nil {
			return err
		}
		if err = e.saveTransactions(out, "frozen-"+probe.Name, len(report.Transactions), []common.Hash{txhash}); err != nil {
			return err
		}
		if !pass {
			return fmt.Errorf("frozen probe mismatch: %s return=%s status=%d", probe.Name, tr.ReturnValue, receipt.Status)
		}
		fmt.Println("frozen probe passed", probe.Name, expected)
		if err = e.reset(frozenSnapshot); err != nil {
			return err
		}
		if err = c.Client().CallContext(ctx, &frozenSnapshot, "evm_snapshot"); err != nil {
			return err
		}
	}
	for _, probe := range f.Probes {
		if err = fresh(); err != nil {
			return err
		}
		before, err := e.stateEvidence(f.Expected)
		if err != nil {
			return err
		}
		data, err := eventData(e.a, probe)
		if err != nil {
			return err
		}
		start := len(report.Transactions)
		if _, err = e.send(probe.Name, &e.ledger, data); err != nil {
			return fmt.Errorf("valid active control %s: %w", probe.Name, err)
		}
		after, err := e.stateEvidence(f.Expected)
		if err != nil {
			return err
		}
		if err = Write(filepath.Join(out, "raw/active-"+probe.Name+".json"), map[string]any{"probe": probe.Name, "expected": "accepted", "before": before, "after": after, "passed": true}); err != nil {
			return err
		}
		if err = e.saveTransactions(out, "active-"+probe.Name, start, nil); err != nil {
			return err
		}
		fmt.Println("active control accepted", probe.Name)
	}
	for _, mode := range []string{"proceed-first", "recall-first", "final-status-unavailable", "receipt-uncertain"} {
		if err = fresh(); err != nil {
			return err
		}
		start := len(report.Transactions)
		if mode == "proceed-first" || mode == "recall-first" {
			idx := 2
			if mode == "recall-first" {
				idx = 3
			}
			data, err := eventData(e.a, f.Probes[idx])
			if err != nil {
				return err
			}
			if _, err = e.send(mode, &e.ledger, data); err != nil {
				return err
			}
		}
		selected := objMap(f.Expected)["V_Q"]
		fault := &integratedFaultSource{RPCSource: e.source, mode: mode, selected: v2audit.Frontier{Ref: selected.Ref, SpendValue: selected.Spend}, targets: len(f.Expected.Targets)}
		result := v2audit.AuditAndFreeze(ctx, fault, master, f.Base.Start, ht)
		if err = Write(filepath.Join(out, "raw", mode+"-client.json"), map[string]any{"result": &result, "injections": fault.injections, "receiptValidations": fault.receiptValidations, "observations": fault.observations}); err != nil {
			return err
		}
		if err = validateResult(f.Expected, result, v2audit.Incomplete); err != nil {
			return err
		}
		var vr *v2audit.TargetResult
		blocked := 0
		for i := range result.TargetResults {
			r := &result.TargetResults[i]
			if r.Target.Ref.Key() == selected.Ref.Key() {
				vr = r
			}
			if r.Blocked {
				blocked++
			}
		}
		if vr == nil {
			return fmt.Errorf("missing V_Q row")
		}
		switch mode {
		case "proceed-first", "recall-first":
			if !vr.InitialSpent || !vr.FinalSpent || vr.Attempted || vr.Blocked || blocked != 6 {
				return fmt.Errorf("race row/partial success mismatch")
			}
		case "final-status-unavailable":
			if fault.injections != 1 || vr.FinalError == "" || vr.Blocked || blocked != 6 || fault.receiptValidations != 7 {
				return fmt.Errorf("final fault mismatch")
			}
		case "receipt-uncertain":
			if fault.injections != 1 || vr.Transaction == nil || vr.Transaction.State != "UNCERTAIN" || vr.Transaction.Hash == "" || vr.SubmissionError == "" || blocked != 7 || fault.receiptValidations != 6 {
				return fmt.Errorf("uncertain receipt mismatch")
			}
		}
		// These reads bypass the wrapper. Never feed them into client outcome.
		ground, err := e.stateEvidence(f.Expected)
		if err != nil {
			return err
		}
		ground["confirmedFreezeTransactions"] = e.source.FreezeTransactions
		if mode == "proceed-first" || mode == "recall-first" {
			idx := 2
			if mode == "recall-first" {
				idx = 3
			}
			newRef := f.Probes[idx].Record.OutputRefs[0]
			h, err := e.source.Checkpoint(ctx)
			if err != nil {
				return err
			}
			producer, err := e.source.Producer(ctx, newRef, h)
			if err != nil {
				return err
			}
			plain, err := auditcrypto.DecryptWithMasterKey(master, f.Probes[idx].Record.Ciphertext())
			if err != nil {
				return err
			}
			sid := plain[len(plain)-1].Bytes()
			status, spent, err := e.source.Status(ctx, newRef.ObjectType, sid, h)
			if err != nil {
				return err
			}
			ground["replacementNote"] = map[string]any{"ref": &newRef, "producer": producer, "status": status, "spent": spent, "notAddedToOriginalT": true}
			if producer != uint64(len(f.Base.Events)+1) || status != v2audit.Active || spent {
				return fmt.Errorf("replacement Note mismatch")
			}
		}
		if err = Write(filepath.Join(out, "raw", mode+"-ground-truth.json"), ground); err != nil {
			return err
		}
		hashes = nil
		for _, tx := range e.source.FreezeTransactions {
			hashes = append(hashes, common.HexToHash(tx.Hash))
		}
		if err = e.saveTransactions(out, mode, start, hashes); err != nil {
			return err
		}
		report.Audits = append(report.Audits, map[string]any{"case": mode, "outcome": result.Outcome, "targets": len(result.Targets), "blockedClientRows": blocked, "passed": true})
		fmt.Println("race/fault passed", mode, result.Outcome)
	}
	report.Audits = append(report.Audits, map[string]any{"case": "normal", "outcome": normal.Outcome, "targets": len(normal.Targets), "passed": true})
	return Write(filepath.Join(out, "raw/validation.json"), map[string]any{"passed": true, "realPLONKProofs": len(f.Base.Events) + len(f.Probes), "baseEvents": len(f.Base.Events), "validActiveControls": len(f.Probes), "exactInvalidStateRejections": len(f.Probes) - 1, "unrelatedTransferAcceptedWhileFrozen": true, "targetCount": len(f.Expected.Targets), "negativeCompletionBranches": 4, "protocolChanged": false, "circuitChanged": false, "faultLayer": "test-only RPCSource wrapper", "graphAndIndexesIndependentlyMatched": true})
}
