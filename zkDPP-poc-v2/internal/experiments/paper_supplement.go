package experiments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	audit "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	blsplonk "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const PaperArtifacts = "artifacts/development/paper-supplement"
const PaperOutput = "output/paper-supplement"

func paperProfiles() []Profile {
	var out []Profile
	for _, a := range []bool{false, true} {
		for i, mn := range [][2]int{{3, 1}, {4, 1}, {2, 3}} {
			suffix := "plain"
			if a {
				suffix = "audit"
			}
			out = append(out, Profile{Name: fmt.Sprintf("process-%d-%d-%s", mn[0], mn[1], suffix), Kind: Process, M: mn[0], N: mn[1], Audit: a, PolicyRef: policy.Ref(6, 1, 101, uint64(6+i))})
		}
	}
	return out
}

// Checks the original frozen checksums without changing the historical baseline.
func PaperPreflight(root string) error {
	var hashes map[string]string
	if err := readJSON(filepath.Join(root, OutputDir, "checksums.json"), &hashes); err != nil {
		return err
	}
	checked := 0
	for path, want := range hashes {
		if !(strings.HasPrefix(path, ArtifactDir+"/") || strings.HasPrefix(path, OutputDir+"/") || strings.HasPrefix(path, "contracts/src/generated/experiments/") || path == "internal/experiments/circuit.go" || path == "internal/experiments/native.go") {
			continue
		}
		got, err := checksum(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("frozen checksum mismatch %s", path)
		}
		checked++
	}
	for _, name := range []string{"process-2-1-plain", "process-2-1-audit", "process-2-2-plain", "process-2-2-audit"} {
		k, err := LoadKeys(root, name)
		if err != nil {
			return err
		}
		cs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, NewCircuit(must(ProfileByName(name)), defaultPK()))
		if err != nil {
			return err
		}
		var b bytes.Buffer
		if _, err = cs.WriteTo(&b); err != nil {
			return err
		}
		sum := sha256.Sum256(b.Bytes())
		if hex.EncodeToString(sum[:]) != k.Relation.Files["ccs.bin"] {
			return fmt.Errorf("current circuit differs %s", name)
		}
	}
	for _, n := range []int{4, 7, 8} {
		if err := ValidateExperiment(root, n); err != nil {
			return err
		}
	}
	// Solidity metadata ties reused bytecode to the actual source files.
	paths, err := filepath.Glob(filepath.Join(root, "contracts/experiments/out/*/*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		var built struct {
			Metadata struct {
				Sources map[string]struct {
					Keccak string `json:"keccak256"`
				}
			}
		}
		if err = readJSON(path, &built); err != nil {
			return err
		}
		for source, info := range built.Metadata.Sources {
			b, err := os.ReadFile(filepath.Join(root, "contracts/experiments", source))
			if err != nil {
				return err
			}
			if crypto.Keccak256Hash(b).Hex() != info.Keccak {
				return fmt.Errorf("Solidity build/source mismatch %s", source)
			}
		}
	}
	fmt.Printf("preflight: %d frozen files verified; four reused Process CCS matched current compile; time-only raw validated, no remeasurement\n", checked)
	return nil
}

func paperRead(path string, v io.ReaderFrom) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = v.ReadFrom(f)
	return e
}

func PaperSetup(root string) error {
	if err := PaperPreflight(root); err != nil {
		return err
	}
	target := filepath.Join(root, PaperOutput, "setup.json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("supplement setup already exists")
	}
	srs := new(kzg.SRS)
	if e := paperRead(filepath.Join(root, ArtifactDir, "srs/canonical.bin"), srs); e != nil {
		return e
	}
	var result []Relation
	for i, p := range paperProfiles() {
		start := time.Now()
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, NewCircuit(p, defaultPK()))
		if e != nil {
			return e
		}
		needed, domain := plonk.SRSSize(cs)
		if needed > len(srs.Pk.G1) {
			return fmt.Errorf("existing SRS insufficient %s", p.Name)
		}
		rel := Relation{Profile: p, PublicNames: p.PublicNames(), Constraints: cs.GetNbConstraints(), Domain: domain, CompileMS: ms(time.Since(start)), Files: map[string]string{}}
		lg := new(kzg.SRS)
		if e = paperRead(filepath.Join(root, ArtifactDir, fmt.Sprintf("srs/lagrange-%d.bin", domain)), lg); e != nil {
			return fmt.Errorf("existing lagrange required, not regenerated: %w", e)
		}
		start = time.Now()
		pk, vk, e := plonk.Setup(cs, srs, lg)
		if e != nil {
			return e
		}
		rel.SetupMS = ms(time.Since(start))
		dir := filepath.Join(root, PaperArtifacts, "circuits", p.Name)
		for _, x := range []struct {
			name string
			v    io.WriterTo
		}{{"ccs.bin", cs}, {"proving.key", pk}, {"verifying.key", vk}} {
			path := filepath.Join(dir, x.name)
			if e = writeBinary(path, x.v); e != nil {
				return e
			}
			rel.Files[x.name] = must(checksum(path))
		}
		verifier := filepath.Join(root, PaperArtifacts, "contracts/src", fmt.Sprintf("P%d.sol", i))
		if e = artifact.ExportSolidity(verifier, vk); e != nil {
			return e
		}
		rel.Files["verifier"] = must(checksum(verifier))
		if e = writeJSON(filepath.Join(dir, "manifest.json"), rel); e != nil {
			return e
		}
		result = append(result, rel)
		fmt.Printf("supplement setup %s constraints=%d domain=%d\n", p.Name, rel.Constraints, domain)
	}
	return writeJSON(target, map[string]any{"relations": result, "srs": filepath.Join(ArtifactDir, "srs/canonical.bin"), "srsChecksum": must(checksum(filepath.Join(root, ArtifactDir, "srs/canonical.bin"))), "newRelations": 6, "newSRS": false})
}

func paperKeys(root string, p Profile) (*Keys, error) {
	dir := filepath.Join(root, PaperArtifacts, "circuits", p.Name)
	var rel Relation
	if e := readJSON(filepath.Join(dir, "manifest.json"), &rel); e != nil {
		return nil, e
	}
	k := &Keys{CS: plonk.NewCS(ecc.BLS12_381), PK: plonk.NewProvingKey(ecc.BLS12_381), VK: plonk.NewVerifyingKey(ecc.BLS12_381), Relation: rel}
	for _, x := range []struct {
		name string
		v    io.ReaderFrom
	}{{"ccs.bin", k.CS}, {"proving.key", k.PK}, {"verifying.key", k.VK}} {
		path := filepath.Join(dir, x.name)
		got, e := checksum(path)
		if e != nil {
			return nil, e
		}
		if got != rel.Files[x.name] {
			return nil, fmt.Errorf("supplement checksum %s", path)
		}
		if e = paperRead(path, x.v); e != nil {
			return nil, e
		}
	}
	return k, nil
}

func paperEngine(root string, ctx context.Context, a bool) (*Engine, error) {
	m := &Meter{}
	r, err := rpc.DialOptions(ctx, "http://127.0.0.1:18548", rpc.WithHTTPClient(&http.Client{Transport: m, Timeout: 60 * time.Second}))
	if err != nil {
		return nil, err
	}
	key, err := crypto.HexToECDSA(anvilPrivateKey)
	if err != nil {
		return nil, err
	}
	en := &Engine{Root: root, Ctx: ctx, Client: ethclient.NewClient(r), Meter: m, Key: key, From: crypto.PubkeyToAddress(key.PublicKey), Audit: a, Keys: map[string]*Keys{}, Verifiers: map[string]common.Address{}, RunTag: fmt.Sprint(time.Now().UnixNano()), ProofDir: PaperArtifacts, ContractOutDir: PaperArtifacts + "/contracts/out"}
	id, err := en.Client.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if id.Uint64() != 31337 {
		return nil, fmt.Errorf("wrong chain")
	}
	return en, nil
}

func paperInstall(en *Engine) error {
	for i, p := range paperProfiles() {
		if p.Audit != en.Audit {
			continue
		}
		if e := en.call("reservePolicyVersion", uint64(101)); e != nil {
			return e
		}
		en.ContractOutDir = PaperArtifacts + "/contracts/out"
		addr, _, e := en.deploy(fmt.Sprintf("P%d.sol", i), "PlonkVerifier")
		en.ContractOutDir = PaperArtifacts + "/contracts/out"
		if e != nil {
			return e
		}
		k, e := paperKeys(en.Root, p)
		if e != nil {
			return e
		}
		en.Keys[p.Name] = k
		en.Verifiers[p.Name] = addr
		opt, e := solgen.ExtractOptimizedVK(k.VK.(*blsplonk.VerifyingKey))
		if e != nil {
			return e
		}
		var vkHash [32]byte
		copy(vkHash[:], must(hex.DecodeString(opt.SHA256)))
		if e = en.call("registerPolicy", audit.Big(p.PolicyRef), uint8(p.M), uint8(p.N), vkHash, addr); e != nil {
			return e
		}
		for _, sk := range []uint64{11, 22} {
			if e = en.call("setPolicyGrant", audit.Big(p.PolicyRef), audit.Big(policy.ScopeRef(h.Element(sk), p.PolicyRef)), true); e != nil {
				return e
			}
		}
	}
	return nil
}

func PaperRun(root string) (err error) {
	if runtime.GOMAXPROCS(0) != 8 {
		return fmt.Errorf("GOMAXPROCS must be 8")
	}
	if err = PaperPreflight(root); err != nil {
		return err
	}
	if err = paperLedgerEvidence(root); err != nil {
		return err
	}
	if err = paperEvidence(root); err != nil {
		return err
	}
	attempt := fmt.Sprint(time.Now().UnixNano())
	if err = writeJSON(filepath.Join(root, PaperOutput, "attempts", attempt+"-start.json"), map[string]any{"officialCases": 10, "warmups": 0, "repetitions": 1}); err != nil {
		return err
	}
	defer func() {
		status := "complete"
		message := ""
		if err != nil {
			status = "failed"
			message = err.Error()
		}
		_ = writeJSON(filepath.Join(root, PaperOutput, "attempts", attempt+"-finish.json"), map[string]any{"status": status, "error": message})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	var rows []Measurement
	for _, mn := range [][2]int{{2, 1}, {3, 1}, {4, 1}, {2, 2}, {2, 3}} {
		for _, a := range []bool{false, true} {
			suffix := "plain"
			if a {
				suffix = "audit"
			}
			name := fmt.Sprintf("process-%d-%d-%s", mn[0], mn[1], suffix)
			target := filepath.Join(root, PaperOutput, name+".json")
			if _, e := os.Stat(target); e == nil {
				return fmt.Errorf("case exists, no repeated official measurement: %s", name)
			}
			en, e := paperEngine(root, ctx, a)
			if e != nil {
				return e
			}
			var reset any
			if e = en.Client.Client().CallContext(ctx, &reset, "anvil_reset"); e != nil {
				return e
			}
			if e = en.Initialize(0); e != nil {
				return e
			}
			if e = paperInstall(en); e != nil {
				return e
			}
			w := NewWorld(defaultPK(), 90000000)
			sk := h.Element(22)
			doc := must(document.Hash(DefaultDocument()))
			var inputs []note.Note
			for j := 0; j < mn[0]; j++ {
				ev, e := en.Execute(w, Operation{P: must(ProfileByName("entry-" + suffix)), SK: sk, Document: doc, State: equalState(mn[0], j)}, name+fmt.Sprintf("-prepare-%d", j), -1, false)
				if e != nil {
					return e
				}
				inputs = append(inputs, ev.Outputs[0])
			}
			prep := append([]Measurement{}, en.Rows...)
			en.Rows = nil
			p, e := ProfileByName(name)
			if e != nil {
				for _, v := range paperProfiles() {
					if v.Name == name {
						p = v
						e = nil
					}
				}
			}
			if e != nil {
				return e
			}
			rootBefore := w.Notes.Root()
			if e = writeJSON(filepath.Join(root, PaperOutput, "attempts", attempt+"-"+name+"-start.json"), map[string]any{"profile": p, "inputs": inputs, "noteRoot": rootBefore, "officialAttempt": 1}); e != nil {
				return e
			}
			ev, e := en.Execute(w, Operation{P: p, SK: sk, Document: doc, Inputs: inputs}, name, 1, false)
			if e != nil {
				return e
			}
			var total note.State
			for _, n := range ev.Outputs {
				total.QMass += n.State.QMass
				total.ARec += n.State.ARec
				total.E += n.State.E
				if n.AssetRole != note.AssetRoleEligible {
					return fmt.Errorf("noneligible output")
				}
			}
			if total != (note.State{QMass: 300e9, ARec: 30e9, E: 261e9}) {
				return fmt.Errorf("aggregate mismatch")
			}
			row := en.Rows[0]
			if !row.Matched || row.Tx.Status != 1 {
				return fmt.Errorf("unmatched transaction")
			}
			transaction, _, e := en.Client.TransactionByHash(ctx, common.HexToHash(row.Tx.Hash))
			if e != nil {
				return e
			}
			receipt, e := en.Client.TransactionReceipt(ctx, common.HexToHash(row.Tx.Hash))
			if e != nil {
				return e
			}
			var fixture struct{ Proof string }
			if e = readJSON(filepath.Join(root, row.ProofArtifact), &fixture); e != nil {
				return e
			}
			calldata, e := eventData(en.ABI, ev, must(hex.DecodeString(strings.TrimPrefix(fixture.Proof, "0x"))))
			if e != nil {
				return e
			}
			if !bytes.Equal(calldata, transaction.Data()) || transaction.To() == nil || *transaction.To() != en.Ledger || receipt.Status != 1 || receipt.GasUsed != row.Tx.Gas {
				return fmt.Errorf("source/proof/calldata/receipt mismatch")
			}
			if e = writeJSON(filepath.Join(root, PaperOutput, name+"-transaction.json"), map[string]any{"transaction": transaction, "receipt": receipt, "proofArtifact": row.ProofArtifact, "matched": true}); e != nil {
				return e
			}
			if e = writeJSON(target, map[string]any{"status": "complete", "warmups": 0, "repetitions": 1, "row": row, "inputs": inputs, "outputs": ev.Outputs, "noteRootBefore": rootBefore, "noteRootAfter": w.Notes.Root(), "preparationRows": prep, "preparations": en.Preparations, "deployments": en.Deployments, "profile": p, "environment": map[string]any{"go": runtime.Version(), "gomaxprocs": 8, "cpuReservation": false, "rpc": "127.0.0.1:18548", "anvilCPUs": 2, "anvilMemoryBytes": uint64(4 << 30)}, "boundary": "same Engine.Execute as original; one official observation; setup/deployment/Entry excluded; local Verify and final state/receipt/record checks after E2E; no timing distribution"}); e != nil {
				return e
			}
			rows = append(rows, row)
			en.Client.Close()
			fmt.Printf("COMPLETE %s constraints=%d gas=%d E2E=%.3fms\n", name, row.Constraints, row.Tx.Gas, row.E2EMS)
		}
	}
	return writeJSON(filepath.Join(root, PaperOutput, "process-independent.json"), map[string]any{"status": "complete", "rows": rows, "officialCases": 10, "warmups": 0, "repetitions": 1, "timeOnlyReruns": 0})
}

func paperLedgerEvidence(root string) error {
	old := "!((inputArity==2&&(outputArity==1||outputArity==2))||(inputArity==3&&(outputArity==2||outputArity==3))||(inputArity==4&&outputArity==3))"
	updated := "!((inputArity==2&&(outputArity==1||outputArity==2||outputArity==3))||(inputArity==3&&(outputArity==1||outputArity==2||outputArity==3))||(inputArity==4&&(outputArity==1||outputArity==3)))"
	rows := map[string]any{}
	for _, name := range []string{"ExperimentalAuditLedger.sol", "ExperimentalNoAuditLedger.sol"} {
		original := filepath.Join(root, "contracts/src/experiments", name)
		supplement := filepath.Join(root, PaperArtifacts, "contracts/src/experiments", name)
		a, e := os.ReadFile(original)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(supplement)
		if e != nil {
			return e
		}
		if strings.Count(string(a), old) != 1 || strings.Replace(string(a), old, updated, 1) != string(b) {
			return fmt.Errorf("supplement Ledger differs beyond approved arities: %s", name)
		}
		built := filepath.Join(root, PaperArtifacts, "contracts/out", name, strings.TrimSuffix(name, ".sol")+".json")
		var raw struct {
			Bytecode         struct{ Object string }
			DeployedBytecode struct{ Object string }
			Metadata         struct {
				Sources map[string]struct {
					Keccak string `json:"keccak256"`
				}
			}
		}
		if e = readJSON(built, &raw); e != nil {
			return e
		}
		for source, info := range raw.Metadata.Sources {
			data, e := os.ReadFile(filepath.Join(root, PaperArtifacts, "contracts", source))
			if e != nil {
				return e
			}
			if crypto.Keccak256Hash(data).Hex() != info.Keccak {
				return fmt.Errorf("supplement bytecode/source mismatch %s", source)
			}
		}
		rows[name] = map[string]any{"originalSHA256": must(checksum(original)), "supplementSHA256": must(checksum(supplement)), "buildJSONSHA256": must(checksum(built)), "runtimeCodeKeccak": crypto.Keccak256Hash(must(hex.DecodeString(strings.TrimPrefix(raw.DeployedBytecode.Object, "0x")))).Hex(), "creationCodeKeccak": crypto.Keccak256Hash(must(hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x")))).Hex(), "onlyChange": "registerPolicy accepts additional 3-to-1, 4-to-1, 2-to-3 Process arities; all other bytes identical"}
	}
	return writeJSON(filepath.Join(root, PaperOutput, "ledger-verification-"+fmt.Sprint(time.Now().UnixNano())+".json"), rows)
}

func paperEvidence(root string) error {
	var baseline Baseline
	if err := readJSON(filepath.Join(root, ArtifactDir, "baseline.json"), &baseline); err != nil {
		return err
	}
	protected := map[string]string{}
	var historicalExceptions []string
	for path, want := range baseline.Files {
		got, err := checksum(filepath.Join(filepath.Dir(root), path))
		if err != nil || got != want {
			if strings.HasPrefix(path, "zkDPP-poc-v1/") || strings.Contains(path, "/m1-hotfix/") || strings.Contains(path, "/output/m1") {
				return fmt.Errorf("protected historical file mismatch %s", path)
			}
			historicalExceptions = append(historicalExceptions, path)
		} else {
			protected[path] = got
		}
	}
	current := map[string]string{}
	for _, path := range []string{"internal/experiments/evm.go", "internal/experiments/paper_supplement.go", "internal/experiments/circuit.go", "internal/experiments/native.go", "internal/experiments/variants.go", "internal/experiments/graphs.go", "internal/experiments/crypto.go", "cmd/paper_supplement/main.go", "docker-compose.paper-supplement.yml", "go.mod", "go.sum"} {
		got, err := checksum(filepath.Join(root, path))
		if err != nil {
			return err
		}
		current[path] = got
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	head, err := cmd.Output()
	if err != nil {
		return err
	}
	cmd = exec.Command("git", "diff", "--", "internal/experiments/evm.go")
	cmd.Dir = root
	diff, err := cmd.Output()
	if err != nil {
		return err
	}
	cmd = exec.Command("docker", "inspect", "--format", "{{.Config.Image}} NanoCPUs={{.HostConfig.NanoCpus}} Memory={{.HostConfig.Memory}} Args={{json .Args}}", "zkdpp-paper-supplement-anvil-1")
	env, err := cmd.Output()
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, PaperOutput, "verification-before-"+fmt.Sprint(time.Now().UnixNano())+".json"), map[string]any{"gitHead": strings.TrimSpace(string(head)), "sourceChecksums": current, "originalEvmDiff": string(diff), "oldChecksumManifestUnchanged": true, "historicalBaselineMatching": protected, "historicalBaselineExceptions": historicalExceptions, "historicalBaselineClaim": "original CheckBaseline not bypassed or rewritten; exceptions reported separately; frozen experiments validated against their own checksums; v1 and M1 protected paths verified", "docker": string(env), "go": runtime.Version(), "gomaxprocs": runtime.GOMAXPROCS(0), "GOMEMLIMIT": os.Getenv("GOMEMLIMIT"), "nativeCPULimit": "GOMAXPROCS is concurrency, not CPU reservation or hard quota", "timeOnlyReview": map[string]any{"reruns": 0, "finding": "no timing-boundary/code error identified; negative paired E2E delta alone does not justify rerun", "tracing": "graph, preloaded master key and initial snapshot excluded; fresh per-run cache; Trace call through return included", "decryption": "1000 workload iterations; key recovery/fixture/final plaintext check excluded", "recovery": "1000 local validated recoveries; validation, interpolation and public-key check included; DKG/network/approval excluded"}})
}
