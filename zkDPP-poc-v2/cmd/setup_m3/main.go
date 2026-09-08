package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	proceedcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	recallcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/recall"
	transfercircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m1case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m3case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
)

type solidityMarshaler interface{ MarshalSolidity() []byte }
type proofFixture struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}
type entryFixture struct {
	Name       string `json:"name"`
	Commitment string `json:"commitment"`
	proofFixture
}
type transferFixture struct {
	Name              string `json:"name"`
	NoteRoot          string `json:"noteRoot"`
	Nullifier         string `json:"nullifier"`
	VoucherCommitment string `json:"voucherCommitment"`
	ChangeCommitment  string `json:"changeCommitment"`
	TransferEpoch     uint64 `json:"transferEpoch"`
	DeltaEpoch        uint64 `json:"deltaEpoch"`
	proofFixture
}
type resolutionFixture struct {
	Name             string `json:"name"`
	VoucherRoot      string `json:"voucherRoot"`
	VoucherNullifier string `json:"voucherNullifier"`
	OutputCommitment string `json:"outputCommitment"`
	CurrentEpoch     uint64 `json:"currentEpoch,omitempty"`
	proofFixture
}
type treeFixture struct {
	FinalNoteRoot     string `json:"finalNoteRoot"`
	FinalVoucherRoot  string `json:"finalVoucherRoot"`
	FinalNoteCount    uint64 `json:"finalNoteCount"`
	FinalVoucherCount uint64 `json:"finalVoucherCount"`
}
type fixedFixtures struct {
	Entries []entryFixture    `json:"entries"`
	Partial transferFixture   `json:"partialTransfer"`
	Proceed resolutionFixture `json:"proceed"`
	Full    transferFixture   `json:"fullTransfer"`
	Recall  resolutionFixture `json:"recall"`
	Tree    treeFixture       `json:"tree"`
}
type circuitResult struct {
	Feature                                                 string `json:"feature"`
	Constraints, PublicInputs                               int
	CompileMillis, SRSMillis, PlonkSetupMillis, SetupMillis int64
	WitnessMillis, ProveMillis, VerifyMillis                float64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactFileBytes                                       map[string]int64
}
type circuitReport struct {
	GeneratedAt        string `json:"generatedAt"`
	RunCount           int    `json:"runCount"`
	Curve, ProofSystem string
	Results            []circuitResult
}
type checksumReport struct {
	GeneratedAt, Algorithm string
	Files                  map[string]string
	PriorOutputs           map[string]string `json:"priorOutputs"`
}

func main() {
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if err := run(*root); err != nil {
		panic(err)
	}
}

func run(root string) error {
	scenario, err := m3case.Build(root)
	if err != nil {
		return err
	}
	entryLoaded, err := ensureEntry(root)
	if err != nil {
		return err
	}
	privateLoaded, err := ensurePrivateSpend(root)
	if err != nil {
		return err
	}
	specs := []artifact.Spec{{Name: "transfer", Circuit: &transfercircuit.Circuit{}, ExpectedPublic: 6}, {Name: "proceed", Circuit: &proceedcircuit.Circuit{}, ExpectedPublic: 3}, {Name: "recall", Circuit: &recallcircuit.Circuit{}, ExpectedPublic: 4}}
	assignments := []frontend.Circuit{scenario.Partial.Assignment, scenario.Proceed.Assignment, scenario.Recall.Assignment}
	loaded := make([]*artifact.Loaded, len(specs))
	manifests := make([]artifact.Manifest, len(specs))
	for i, spec := range specs {
		manifests[i], err = artifact.SetupAt(root, "m3", spec)
		if err != nil {
			return err
		}
		loaded[i], err = artifact.LoadAt(root, "m3", spec.Name)
		if err != nil {
			return err
		}
	}
	for _, item := range []struct {
		name string
		vk   plonk.VerifyingKey
	}{{"entry", entryLoaded.VK}, {"private-spend", privateLoaded.VK}, {"transfer", loaded[0].VK}, {"proceed", loaded[1].VK}, {"recall", loaded[2].VK}} {
		if err := artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", item.name, "PlonkVerifier.sol"), item.vk); err != nil {
			return err
		}
	}
	if err := writePoseidon(filepath.Join(root, "contracts", "src", "generated", "Poseidon2BLS12381.sol")); err != nil {
		return err
	}

	fixtures := fixedFixtures{Tree: treeFixture{scenario.FinalNoteRoot.String(), scenario.FinalVoucherRoot.String(), scenario.FinalNoteCount, scenario.FinalVoucherCount}}
	for _, e := range scenario.Entries {
		p, _, sol, _, _, _, err := prove(entryLoaded, e.Assignment)
		if err != nil {
			return err
		}
		fixtures.Entries = append(fixtures.Entries, entryFixture{Name: e.Name, Commitment: e.Note.Commitment.String(), proofFixture: proofFixture{"0x" + hex.EncodeToString(sol), []string{e.Note.Commitment.String()}}})
		_ = p
	}
	results := make([]circuitResult, 0, 3)
	proofs := make([][]byte, 3)
	for i := range specs {
		_, binary, sol, wt, pt, vt, err := prove(loaded[i], assignments[i])
		if err != nil {
			return err
		}
		proofs[i] = sol
		m := manifests[i]
		results = append(results, circuitResult{Feature: specs[i].Name, Constraints: m.Constraints, PublicInputs: m.PublicInputs, CompileMillis: m.CompileMillis, SRSMillis: m.SRSMillis, PlonkSetupMillis: m.PlonkSetupMillis, SetupMillis: m.SetupMillis, WitnessMillis: ms(wt), ProveMillis: ms(pt), VerifyMillis: ms(vt), BinaryProofBytes: binary, SolidityProofBytes: len(sol), ArtifactFileBytes: m.FileBytes})
	}
	partialNF := noteNF(scenario.Partial)
	proceedNF := voucher.Nullifier(scenario.Proceed.Voucher.Opening, scenario.Proceed.Voucher.Commitment)
	fullNF := noteNF(scenario.Full)
	recallNF := voucher.Nullifier(scenario.Recall.Voucher.Opening, scenario.Recall.Voucher.Commitment)
	fixtures.Partial = transferFixture{Name: "partial", NoteRoot: field(scenario.Partial.Assignment.NoteRoot), Nullifier: partialNF.String(), VoucherCommitment: scenario.Partial.Voucher.Commitment.String(), ChangeCommitment: scenario.Partial.Change.Commitment.String(), TransferEpoch: m3case.TransferEpoch, DeltaEpoch: m3case.DeltaEpoch, proofFixture: proofFixture{"0x" + hex.EncodeToString(proofs[0]), []string{field(scenario.Partial.Assignment.NoteRoot), partialNF.String(), scenario.Partial.Voucher.Commitment.String(), scenario.Partial.Change.Commitment.String(), fmt.Sprint(m3case.TransferEpoch), fmt.Sprint(m3case.DeltaEpoch)}}}
	fixtures.Proceed = resolutionFixture{Name: "partial", VoucherRoot: field(scenario.Proceed.Assignment.VoucherRoot), VoucherNullifier: proceedNF.String(), OutputCommitment: scenario.Proceed.Output.Commitment.String(), proofFixture: proofFixture{"0x" + hex.EncodeToString(proofs[1]), []string{field(scenario.Proceed.Assignment.VoucherRoot), proceedNF.String(), scenario.Proceed.Output.Commitment.String()}}}
	_, _, fullSol, _, _, _, err := prove(loaded[0], scenario.Full.Assignment)
	if err != nil {
		return err
	}
	fixtures.Full = transferFixture{Name: "full", NoteRoot: field(scenario.Full.Assignment.NoteRoot), Nullifier: fullNF.String(), VoucherCommitment: scenario.Full.Voucher.Commitment.String(), ChangeCommitment: scenario.Full.Change.Commitment.String(), TransferEpoch: m3case.TransferEpoch, DeltaEpoch: m3case.DeltaEpoch, proofFixture: proofFixture{"0x" + hex.EncodeToString(fullSol), []string{field(scenario.Full.Assignment.NoteRoot), fullNF.String(), scenario.Full.Voucher.Commitment.String(), scenario.Full.Change.Commitment.String(), fmt.Sprint(m3case.TransferEpoch), fmt.Sprint(m3case.DeltaEpoch)}}}
	fixtures.Recall = resolutionFixture{Name: "full", VoucherRoot: field(scenario.Recall.Assignment.VoucherRoot), VoucherNullifier: recallNF.String(), OutputCommitment: scenario.Recall.Output.Commitment.String(), CurrentEpoch: m3case.RecallEpoch, proofFixture: proofFixture{"0x" + hex.EncodeToString(proofs[2]), []string{field(scenario.Recall.Assignment.VoucherRoot), recallNF.String(), scenario.Recall.Output.Commitment.String(), fmt.Sprint(m3case.RecallEpoch)}}}
	if err := artifact.WriteJSON(filepath.Join(root, "contracts", "test", "fixtures", "m3-proofs.json"), fixtures); err != nil {
		return err
	}
	report := circuitReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339), RunCount: 1, Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Results: results}
	if err := artifact.WriteJSON(filepath.Join(root, "output", "m3-circuit.json"), report); err != nil {
		return err
	}
	if err := writeChecksums(root); err != nil {
		return err
	}
	for _, v := range results {
		fmt.Printf("%-10s constraints=%d public=%d solidity-proof=%dB\n", v.Feature, v.Constraints, v.PublicInputs, v.SolidityProofBytes)
	}
	return nil
}

func noteNF(v m3case.TransferCase) fr.Element     { return fieldElement(v.Assignment.NF) }
func field(v frontend.Variable) string            { value := fieldElement(v); return value.String() }
func fieldElement(v frontend.Variable) fr.Element { return v.(fr.Element) }

func ensureEntry(root string) (*artifact.Loaded, error) {
	if l, e := artifact.LoadAt(root, "m2", "entry"); e == nil {
		return l, nil
	}
	if _, e := artifact.SetupAt(root, "m2", artifact.Spec{Name: "entry", Circuit: &entrycircuit.Circuit{}, ExpectedPublic: 1}); e != nil {
		return nil, e
	}
	return artifact.LoadAt(root, "m2", "entry")
}
func ensurePrivateSpend(root string) (*artifact.Loaded, error) {
	if l, e := artifact.Load(root, "private-spend"); e == nil {
		return l, nil
	}
	cases, e := m1case.All(root)
	if e != nil {
		return nil, e
	}
	for _, c := range cases {
		if c.Name == "private-spend" {
			if _, e := artifact.Setup(root, c); e != nil {
				return nil, e
			}
			return artifact.Load(root, "private-spend")
		}
	}
	return nil, fmt.Errorf("private-spend case missing")
}
func prove(l *artifact.Loaded, a frontend.Circuit) (plonk.Proof, int, []byte, time.Duration, time.Duration, time.Duration, error) {
	ws := time.Now()
	w, e := frontend.NewWitness(a, ecc.BLS12_381.ScalarField())
	wt := time.Since(ws)
	if e != nil {
		return nil, 0, nil, wt, 0, 0, e
	}
	pub, e := w.Public()
	if e != nil {
		return nil, 0, nil, wt, 0, 0, e
	}
	ps := time.Now()
	p, e := plonk.Prove(l.CCS, l.PK, w)
	pt := time.Since(ps)
	if e != nil {
		return nil, 0, nil, wt, pt, 0, e
	}
	vs := time.Now()
	e = plonk.Verify(p, l.VK, pub)
	vt := time.Since(vs)
	if e != nil {
		return nil, 0, nil, wt, pt, vt, e
	}
	var b bytes.Buffer
	_, e = p.WriteTo(&b)
	if e != nil {
		return nil, 0, nil, wt, pt, vt, e
	}
	return p, b.Len(), p.(solidityMarshaler).MarshalSolidity(), wt, pt, vt, nil
}
func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
func writePoseidon(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return solgen.WritePoseidon2BLS12381(f)
}
func writeChecksums(root string) error {
	files := []string{"contracts/src/generated/Poseidon2BLS12381.sol", "contracts/src/generated/entry/PlonkVerifier.sol", "contracts/src/generated/private-spend/PlonkVerifier.sol", "contracts/src/generated/transfer/PlonkVerifier.sol", "contracts/src/generated/proceed/PlonkVerifier.sol", "contracts/src/generated/recall/PlonkVerifier.sol", "contracts/test/fixtures/m3-proofs.json"}
	r := checksumReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Algorithm: "SHA-256", Files: map[string]string{}, PriorOutputs: map[string]string{}}
	for _, n := range files {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		r.Files[n] = v
	}
	for _, n := range []string{"output/m1-core.json", "output/m2-circuit.json", "output/m2-anvil-gas.json", "output/m2-anvil-e2e.json"} {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		r.PriorOutputs[n] = v
	}
	return artifact.WriteJSON(filepath.Join(root, "output", "m3-generated-checksums.json"), r)
}
