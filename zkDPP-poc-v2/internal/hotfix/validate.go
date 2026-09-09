package hotfix

import "encoding/json"
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"path/filepath"
	"reflect"
)

func ValidateReports(root string) error {
	var report v2run.CircuitResult
	if e := Read(filepath.Join(root, "output/m1-hotfix-circuit.json"), &report); e != nil {
		return e
	}
	suite, e := v2case.Build(root)
	if e != nil {
		return e
	}
	if len(report.Relations) != 10 {
		return fmt.Errorf("relation count")
	}
	for i, row := range report.Relations {
		item := suite.Cases[i]
		if row.Name != item.Name || !reflect.DeepEqual(row.PublicNames, item.PublicNames) || row.PublicInputs != len(item.PublicNames) || row.ProveMillis <= 0 || row.ProofBytes == 0 {
			return fmt.Errorf("relation metadata %s", row.Name)
		}
		ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.Definition)
		if e != nil {
			return e
		}
		var buf bytes.Buffer
		if _, e = ccs.WriteTo(&buf); e != nil {
			return e
		}
		sum := sha256.Sum256(buf.Bytes())
		if hex.EncodeToString(sum[:]) != row.Files["ccs.bin"] {
			return fmt.Errorf("Circuit source changed since Setup: %s", row.Name)
		}
		for name, want := range row.Files {
			got, e := Hash(filepath.Join(root, Dir, "circuits", row.Name, name))
			if e != nil {
				return e
			}
			if got != want {
				return fmt.Errorf("Artifact changed %s/%s", row.Name, name)
			}
		}
	}
	hash, e := Hash(filepath.Join(root, Dir, "srs/universal-canonical.bin"))
	if e != nil {
		return e
	}
	if hash != report.UniversalSRSChecksum {
		return fmt.Errorf("SRS mismatch")
	}
	var proof ProofFile
	path := filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json")
	if e = Read(path, &proof); e != nil {
		return e
	}
	groups, e := v2case.BuildHotfix(root)
	if e != nil {
		return e
	}
	if len(proof.Groups) != len(groups) {
		return fmt.Errorf("proof group count")
	}
	for i, g := range proof.Groups {
		events := append(append([]ProofEvent{}, g.Events...), g.Extra...)
		want := append(append([]v2case.Event{}, groups[i].Events...), groups[i].Extra...)
		if len(events) != len(want) {
			return fmt.Errorf("proof event count")
		}
		for j, ev := range events {
			if ev.Name != want[j].Name || ev.Relation != want[j].Relation || len(ev.Public) != len(want[j].Public) {
				return fmt.Errorf("fixture order")
			}
			for k, v := range ev.Public {
				if v != auditcrypto.EncodeField(want[j].Public[k]) {
					return fmt.Errorf("fixture public input mismatch")
				}
			}
		}
	}
	fh, e := Hash(path)
	if e != nil {
		return e
	}
	for _, name := range []string{"anvil", "audit"} {
		var evm EVMReport
		if e = Read(filepath.Join(root, "output/m1-hotfix-"+name+".json"), &evm); e != nil {
			return e
		}
		if evm.MockVerifiers || evm.FixtureHash != fh || len(evm.Groups) != 2 || len(evm.Deployments) != 24 {
			return fmt.Errorf("incomplete real-proof EVM report")
		}
		for _, tx := range evm.Transactions {
			if tx.Hash == "" || tx.BlockHash == "" || tx.Status != 1 || tx.Gas == 0 {
				return fmt.Errorf("missing receipt evidence")
			}
		}
		for _, d := range evm.Deployments {
			if d.RuntimeBytes > 24576 || d.CodeHash == "" {
				return fmt.Errorf("invalid deployment")
			}
		}
		if name == "audit" {
			complete, race, back, term := false, false, false, false
			for _, row := range evm.Audits {
				if direction, _ := row["direction"].(string); direction == "backward" {
					back = true
				}
				if direction, _ := row["direction"].(string); direction == "forward" && row["group"] == "lifecycle" {
					term = true
				}
				if raw, ok := row["result"]; ok {
					data, _ := json.Marshal(raw)
					var r v2audit.Result
					if e = json.Unmarshal(data, &r); e != nil {
						return e
					}
					if r.Outcome == v2audit.CompleteAtCheckpoint && len(r.Targets) == 3 {
						if len(r.TargetResults) != len(r.Targets) || r.Checkpoint.BlockNumber < r.Snapshot.BlockNumber || r.Checkpoint.BlockHash == "" {
							return fmt.Errorf("invalid completion checkpoint")
						}
						for i, t := range r.TargetResults {
							if t.Target.Ref.Key() != r.Targets[i].Ref.Key() || !t.Blocked || t.FinalSpent || (t.FinalStatus != v2audit.Frozen && t.FinalStatus != v2audit.Revoked) || t.Observation.BlockNumber > r.Checkpoint.BlockNumber {
								return fmt.Errorf("invalid target completion")
							}
							if t.Transaction != nil && (t.Transaction.State == "UNCERTAIN" || t.Transaction.BlockNumber > r.Checkpoint.BlockNumber) {
								return fmt.Errorf("unresolved transaction marked complete")
							}
						}
						complete = true
					}
					if row["group"] == "post-snapshot-race" && r.Outcome == v2audit.Incomplete {
						race = true
					}
				}
			}
			if !complete || !race || !back || !term {
				return fmt.Errorf("incomplete audit gates")
			}
		}
	}
	var key struct {
		Recovery []struct {
			Millis           float64
			PublicKeyMatched bool
		}
		Decrypt []struct {
			Records          int
			AllFieldsMatched bool
		}
	}
	if e = Read(filepath.Join(root, "output/m1-hotfix-key-recovery.json"), &key); e != nil {
		return e
	}
	if len(key.Recovery) != 3 || len(key.Decrypt) != 4 {
		return fmt.Errorf("incomplete key measurements")
	}
	for _, r := range key.Recovery {
		if !r.PublicKeyMatched || r.Millis <= 0 {
			return fmt.Errorf("key recovery not verified")
		}
	}
	for i, r := range key.Decrypt {
		if r.Records != []int{1, 10, 100, 1000}[i] || !r.AllFieldsMatched {
			return fmt.Errorf("decrypt workload mismatch")
		}
	}
	return nil
}
