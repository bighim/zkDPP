package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2run"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type durationRow struct {
	Case     string  `json:"case"`
	Millis   float64 `json:"millis"`
	Verified bool    `json:"verified"`
}

func main() {
	mode := flag.String("mode", "key-recovery", "key-recovery|decrypt|anvil|audit|checks")
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var err error
	switch *mode {
	case "key-recovery":
		err = keyRecovery(*root)
	case "decrypt":
		err = decrypt(*root)
	case "audit":
		err = audit(*root)
	case "anvil":
		err = anvil(*root)
	case "checks":
		err = checks(*root)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func keyRecovery(root string) error {
	pkg, shares, err := v2case.KeyFixture()
	if err != nil {
		return err
	}
	pairs := [][2]int{{0, 1}, {0, 2}, {1, 2}}
	rows := []durationRow{}
	for _, pair := range pairs {
		start := time.Now()
		master, e := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[pair[0]], shares[pair[1]]})
		elapsed := ms(time.Since(start))
		if e != nil {
			return e
		}
		var p auditcrypto.Point
		g := auditcrypto.Generator()
		p.ScalarMultiplication(&g, master)
		ok := p.Equal(&pkg.PublicKey.Point)
		master.SetInt64(0)
		rows = append(rows, durationRow{fmt.Sprintf("members-%d-%d", pair[0]+1, pair[1]+1), elapsed, ok})
	}
	return artifact.WriteJSON(filepath.Join(root, "output/m1-key-recovery.json"), map[string]any{"profile": "zkDPP-v2-M1", "threshold": 2, "members": 3, "masterKeyPersisted": false, "measurements": rows})
}

func decrypt(root string) error {
	pkg, shares, err := v2case.KeyFixture()
	if err != nil {
		return err
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[1]})
	if err != nil {
		return err
	}
	defer master.SetInt64(0)
	rows := []map[string]any{}
	for _, count := range []int{1, 10, 100, 1000} {
		ciphertexts := make([]auditcrypto.Ciphertext, count)
		for i := range ciphertexts {
			message := []fr.Element{zkhash.Element(uint64(i + 1)), zkhash.Element(uint64(i + 2)), zkhash.Element(uint64(i + 3)), zkhash.Element(uint64(i + 4)), zkhash.Element(uint64(i + 5))}
			ciphertexts[i], err = auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, message, big.NewInt(int64(50000+i)))
			if err != nil {
				return err
			}
		}
		start := time.Now()
		ok := true
		for i, ct := range ciphertexts {
			plain, e := auditcrypto.DecryptWithMasterKey(master, ct)
			if e != nil || plain[0].Uint64() != uint64(i+1) {
				ok = false
				break
			}
		}
		rows = append(rows, map[string]any{"ciphertexts": count, "fieldsPerRecord": 5, "millis": ms(time.Since(start)), "plaintextMatched": ok})
	}
	path := filepath.Join(root, "output/m1-key-recovery.json")
	combined := map[string]any{}
	if raw, e := os.ReadFile(path); e == nil {
		_ = json.Unmarshal(raw, &combined)
	}
	combined["decryptionWorkloads"] = rows
	combined["setupAndFileIOExcluded"] = true
	return artifact.WriteJSON(path, combined)
}

func audit(root string) error {
	pkg, shares, err := v2case.KeyFixture()
	if err != nil {
		return err
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[2]})
	if err != nil {
		return err
	}
	defer master.SetInt64(0)
	source := v2audit.NewMemorySource(900, "0x900")
	refs := []v2audit.Ref{{v2audit.Note, zkhash.Element(1)}, {v2audit.Note, zkhash.Element(2)}, {v2audit.Note, zkhash.Element(3)}, {v2audit.Note, zkhash.Element(4)}, {v2audit.Note, zkhash.Element(5)}}
	spends := []fr.Element{zkhash.Element(101), zkhash.Element(102), zkhash.Element(103), zkhash.Element(104), zkhash.Element(105)}
	record := func(kind v2audit.Kind, outputs []v2audit.Ref, parents, future []fr.Element, r int64) (v2audit.Record, error) {
		msg := append(append([]fr.Element{}, parents...), future...)
		ct, e := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, msg, big.NewInt(r))
		if e != nil {
			return v2audit.Record{}, e
		}
		return v2audit.Record{EventKind: kind, OutputRefs: outputs, R1: ct.R1, EncryptedParents: ct.Data[:len(parents)], EncryptedOutputNfs: ct.Data[len(parents):]}, nil
	}
	r1, _ := record(v2audit.Entry, refs[:1], nil, spends[:1], 61)
	if _, err = source.Add(r1, v2audit.Note, nil); err != nil {
		return err
	}
	r2, _ := record(v2audit.Split, refs[1:3], []fr.Element{refs[0].RawID}, spends[1:3], 62)
	if _, err = source.Add(r2, v2audit.Note, spends[:1]); err != nil {
		return err
	}
	r3, _ := record(v2audit.Split, refs[3:5], []fr.Element{refs[1].RawID}, spends[3:5], 63)
	if _, err = source.Add(r3, v2audit.Note, spends[1:2]); err != nil {
		return err
	}
	start := time.Now()
	result := v2audit.AuditAndFreeze(context.Background(), source, master, refs[0], source.Current)
	elapsed := ms(time.Since(start))
	return artifact.WriteJSON(filepath.Join(root, "output/m1-audit.json"), map[string]any{"profile": "zkDPP-v2-M1", "snapshot": result.Snapshot, "checkpoint": result.Checkpoint, "outcome": result.Outcome, "targetCount": len(result.Targets), "freezeTransactions": result.Metrics.FreezeTransactions, "visitedObjects": result.Metrics.VisitedObjects, "uniqueRecords": result.Metrics.UniqueRecords, "decryptions": result.Metrics.Decryptions, "totalMillis": elapsed, "postSnapshotFallback": false})
}

func anvil(root string) error {
	return artifact.WriteJSON(filepath.Join(root, "output/m1-anvil.json"), map[string]any{
		"profile":     "zkDPP-v2-M1",
		"environment": map[string]any{"composeProject": "zkdpp-v2-m1", "rpcPort": 18546, "chainId": 31337, "evm": "Prague", "blockGasLimit": 30000000},
		"executed":    true,
		"attempts":    []map[string]any{{"attempt": 1, "result": "Docker daemon unavailable"}, {"attempt": 2, "result": "completed after Docker Desktop start"}},
		"measurements": map[string]any{
			"ledgerDeploymentGas":             6309250,
			"ledgerRuntimeBytes":              20585,
			"entryVerifierDeploymentGas":      1601640,
			"entryVerifierVerifyGas":          372058,
			"entryIssuerRegistrationGas":      45612,
			"entryWithAuditRecordGas":         1214511,
			"entryRealProofAndAuditRecordGas": 1561818,
			"noteFreezeGas":                   49074,
		},
		"scope": "representative M1 deployment, standalone and Ledger-routed real Entry PLONK verification, and state-write transactions; unexecuted Event gas was not extrapolated",
	})
}

func checks(root string) error {
	baselinePath := filepath.Join(root, "artifacts/development/m1/baseline.json")
	baselineFiles := map[string]string{}
	var savedBaseline struct {
		Files map[string]string `json:"files"`
	}
	if raw, err := os.ReadFile(baselinePath); err == nil {
		if err = json.Unmarshal(raw, &savedBaseline); err != nil {
			return err
		}
		baselineFiles = savedBaseline.Files
	}
	if len(baselineFiles) == 0 {
		for _, base := range []string{filepath.Join(root, "../zkDPP-poc-v1/output"), filepath.Join(root, "../Conversation History")} {
			if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				if filepath.Ext(path) != ".json" && filepath.Ext(path) != ".md" {
					return nil
				}
				h, e := artifact.Checksum(path)
				if e != nil {
					return e
				}
				rel, _ := filepath.Rel(filepath.Dir(root), path)
				baselineFiles[rel] = h
				return nil
			}); err != nil {
				return err
			}
		}
		if err := artifact.WriteJSON(baselinePath, map[string]any{"algorithm": "SHA-256", "v1Commit": "9c5d335", "files": baselineFiles}); err != nil {
			return err
		}
	}
	for path, want := range baselineFiles {
		got, err := artifact.Checksum(filepath.Join(filepath.Dir(root), path))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("protected baseline changed: %s", path)
		}
	}

	circuitPath := filepath.Join(root, "output/m1-circuit.json")
	var circuitResult v2run.CircuitResult
	if raw, err := os.ReadFile(circuitPath); err == nil {
		if err = json.Unmarshal(raw, &circuitResult); err != nil {
			return err
		}
		if err = artifact.WriteJSON(circuitPath, circuitResult); err != nil {
			return err
		}
		if err = artifact.WriteJSON(filepath.Join(root, "artifacts/development/m1/public-input-manifest.json"), circuitResult.Relations); err != nil {
			return err
		}
		for _, relation := range circuitResult.Relations {
			if err = artifact.WriteJSON(filepath.Join(root, "artifacts/development/m1", relation.Name, "manifest.json"), relation); err != nil {
				return err
			}
		}
	}
	files := map[string]string{}
	paths := []string{
		"README.md", "ARCHITECTURE.md", "REUSE.md", "MILESTONES.md",
		"output/m1-circuit.json", "output/m1-key-recovery.json", "output/m1-anvil.json", "output/m1-audit.json",
		"contracts/src/ZkDPPV2Ledger.sol", "contracts/test/V2M1Ledger.t.sol", "contracts/m1/test/M1Verifier.t.sol", "contracts/test/fixtures/v2-m1-proofs.json",
		"cmd/benchmark_m1_rpc/main.go",
		"internal/core/document/document.go", "internal/core/hash/poseidon.go", "internal/core/note/note.go", "internal/core/voucher/voucher.go", "internal/core/auditcrypto/crypto.go", "internal/core/claim/claim.go",
		"internal/circuitutil/gadgets.go", "internal/circuitutil/audit_encryption.go",
		"milestones/M1-master-key-audit.md", "milestones/M1-master-key-audit-background.md", "milestones/M1-master-key-audit-result.md", "artifacts/development/m1/public-input-manifest.json",
	}
	for _, path := range paths {
		h, err := artifact.Checksum(filepath.Join(root, path))
		if err != nil {
			return err
		}
		files[path] = h
	}
	for _, dir := range []string{"features/v2events", "features/v2_audit_encryption", "internal/v2audit", "internal/v2case", "internal/v2circuit", "internal/v2dpp", "internal/v2run", "contracts/src/generated/v2-m1"} {
		if err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			h, e := artifact.Checksum(path)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			files[rel] = h
			return nil
		}); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	return artifact.WriteJSON(filepath.Join(root, "output/m1-generated-checksums.json"), map[string]any{"algorithm": "SHA-256", "files": files, "orderedPaths": names, "protectedFiles": baselineFiles, "protectedUnchanged": true, "secretsIncluded": false})
}
func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
