package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m1case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/solgen"
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
	ActorID    string `json:"actorId"`
	Commitment string `json:"commitment"`
	proofFixture
}

type exitFixture struct {
	Root      string `json:"root"`
	Nullifier string `json:"nullifier"`
	proofFixture
}

type treeFixture struct {
	FinalRoot    string   `json:"finalRoot"`
	Commitments  []string `json:"commitments"`
	ExitIndex    uint64   `json:"exitIndex"`
	ExitSiblings []string `json:"exitSiblings"`
}

type fixedFixtures struct {
	Entries []entryFixture `json:"entries"`
	Exit    exitFixture    `json:"exit"`
	Tree    treeFixture    `json:"tree"`
}

type circuitResult struct {
	Feature            string           `json:"feature"`
	ReusedFrom         string           `json:"reusedFrom,omitempty"`
	Constraints        int              `json:"constraints"`
	PublicInputs       int              `json:"publicInputs"`
	CompileMillis      int64            `json:"compileMillis"`
	SetupMillis        int64            `json:"setupMillis"`
	WitnessMillis      float64          `json:"witnessMillis"`
	ProveMillis        float64          `json:"proveMillis"`
	VerifyMillis       float64          `json:"verifyMillis"`
	BinaryProofBytes   int              `json:"binaryProofBytes"`
	SolidityProofBytes int              `json:"solidityProofBytes"`
	ArtifactFileBytes  map[string]int64 `json:"artifactFileBytes"`
}

type circuitReport struct {
	GeneratedAt string          `json:"generatedAt"`
	RunCount    int             `json:"runCount"`
	Curve       string          `json:"curve"`
	ProofSystem string          `json:"proofSystem"`
	Results     []circuitResult `json:"results"`
}

type checksumReport struct {
	GeneratedAt      string            `json:"generatedAt"`
	Algorithm        string            `json:"algorithm"`
	Files            map[string]string `json:"files"`
	M1OutputChecksum string            `json:"m1OutputChecksum"`
}

func main() {
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if err := run(*root); err != nil {
		panic(err)
	}
}

func run(root string) error {
	scenario, err := m2case.Build(root)
	if err != nil {
		return err
	}
	entryManifest, err := artifact.SetupAt(root, "m2", artifact.Spec{Name: "entry", Circuit: &entrycircuit.Circuit{}, ExpectedPublic: 1})
	if err != nil {
		return err
	}
	entryLoaded, err := artifact.LoadAt(root, "m2", "entry")
	if err != nil {
		return err
	}
	privateLoaded, err := ensurePrivateSpend(root)
	if err != nil {
		return err
	}
	if err := artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", "entry", "PlonkVerifier.sol"), entryLoaded.VK); err != nil {
		return err
	}
	if err := artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", "private-spend", "PlonkVerifier.sol"), privateLoaded.VK); err != nil {
		return err
	}
	if err := writePoseidon(filepath.Join(root, "contracts", "src", "generated", "Poseidon2BLS12381.sol")); err != nil {
		return err
	}
	fixtures := fixedFixtures{Tree: treeFixture{FinalRoot: scenario.FinalRoot.String(), ExitIndex: scenario.ExitPath.Index}}
	fixtures.Tree.Commitments = make([]string, len(scenario.Entries))
	for _, sibling := range scenario.ExitPath.Siblings {
		fixtures.Tree.ExitSiblings = append(fixtures.Tree.ExitSiblings, sibling.String())
	}
	results := make([]circuitResult, 0, 2)
	var entryBinary, entrySolidity int
	var entryWitness, entryProve, entryVerify float64
	for i, item := range scenario.Entries {
		proof, binaryBytes, solidityBytes, witnessTime, proveTime, verifyTime, err := prove(entryLoaded, item.Assignment)
		if err != nil {
			return fmt.Errorf("prove %s: %w", item.Name, err)
		}
		if i == 0 {
			entryBinary, entrySolidity = binaryBytes, len(solidityBytes)
			entryWitness, entryProve, entryVerify = milliseconds(witnessTime), milliseconds(proveTime), milliseconds(verifyTime)
		}
		fixtures.Tree.Commitments[i] = item.Note.Commitment.String()
		fixtures.Entries = append(fixtures.Entries, entryFixture{
			Name: item.Name, ActorID: item.ActorID, Commitment: item.Note.Commitment.String(),
			proofFixture: proofFixture{Proof: "0x" + hex.EncodeToString(solidityBytes), PublicInputs: []string{item.Note.Commitment.String()}},
		})
		_ = proof
	}
	_, exitBinary, exitSolidityBytes, exitWitnessTime, exitProveTime, exitVerifyTime, err := prove(privateLoaded, scenario.ExitAssignment)
	if err != nil {
		return err
	}
	exitRoot := scenario.FinalRoot
	exitNF, ok := scenario.ExitAssignment.NF.(fr.Element)
	if !ok {
		return fmt.Errorf("exit nullifier is %T, want fr.Element", scenario.ExitAssignment.NF)
	}
	fixtures.Exit = exitFixture{
		Root: exitRoot.String(), Nullifier: exitNF.String(),
		proofFixture: proofFixture{Proof: "0x" + hex.EncodeToString(exitSolidityBytes), PublicInputs: []string{exitRoot.String(), exitNF.String()}},
	}
	if err := artifact.WriteJSON(filepath.Join(root, "contracts", "test", "fixtures", "m2-proofs.json"), fixtures); err != nil {
		return err
	}
	if err := writeChecksums(root); err != nil {
		return err
	}
	results = append(results,
		circuitResult{Feature: "entry", Constraints: entryManifest.Constraints, PublicInputs: entryManifest.PublicInputs, CompileMillis: entryManifest.CompileMillis, SetupMillis: entryManifest.SetupMillis, WitnessMillis: entryWitness, ProveMillis: entryProve, VerifyMillis: entryVerify, BinaryProofBytes: entryBinary, SolidityProofBytes: entrySolidity, ArtifactFileBytes: entryManifest.FileBytes},
		circuitResult{Feature: "private-spend", ReusedFrom: "M1", Constraints: privateLoaded.Manifest.Constraints, PublicInputs: privateLoaded.Manifest.PublicInputs, CompileMillis: privateLoaded.Manifest.CompileMillis, SetupMillis: privateLoaded.Manifest.SetupMillis, WitnessMillis: milliseconds(exitWitnessTime), ProveMillis: milliseconds(exitProveTime), VerifyMillis: milliseconds(exitVerifyTime), BinaryProofBytes: exitBinary, SolidityProofBytes: len(exitSolidityBytes), ArtifactFileBytes: privateLoaded.Manifest.FileBytes},
	)
	report := circuitReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339), RunCount: 1, Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Results: results}
	if err := artifact.WriteJSON(filepath.Join(root, "output", "m2-circuit.json"), report); err != nil {
		return err
	}
	for _, result := range results {
		fmt.Printf("%-16s constraints=%d public=%d solidity-proof=%dB\n", result.Feature, result.Constraints, result.PublicInputs, result.SolidityProofBytes)
	}
	return nil
}

func writeChecksums(root string) error {
	relative := []string{
		"contracts/src/generated/Poseidon2BLS12381.sol",
		"contracts/src/generated/entry/PlonkVerifier.sol",
		"contracts/src/generated/private-spend/PlonkVerifier.sol",
		"contracts/test/fixtures/m2-proofs.json",
	}
	report := checksumReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Algorithm: "SHA-256", Files: map[string]string{}}
	for _, name := range relative {
		checksum, err := artifact.Checksum(filepath.Join(root, name))
		if err != nil {
			return err
		}
		report.Files[name] = checksum
	}
	checksum, err := artifact.Checksum(filepath.Join(root, "output", "m1-core.json"))
	if err != nil {
		return err
	}
	report.M1OutputChecksum = checksum
	return artifact.WriteJSON(filepath.Join(root, "output", "m2-generated-checksums.json"), report)
}

func writePoseidon(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := solgen.WritePoseidon2BLS12381(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func ensurePrivateSpend(root string) (*artifact.Loaded, error) {
	loaded, err := artifact.Load(root, "private-spend")
	if err == nil {
		return loaded, nil
	}
	cases, buildErr := m1case.All(root)
	if buildErr != nil {
		return nil, buildErr
	}
	for _, item := range cases {
		if item.Name != "private-spend" {
			continue
		}
		if _, setupErr := artifact.Setup(root, item); setupErr != nil {
			return nil, setupErr
		}
		return artifact.Load(root, "private-spend")
	}
	return nil, fmt.Errorf("private-spend M1 case not found")
}

func prove(loaded *artifact.Loaded, assignment frontend.Circuit) (plonk.Proof, int, []byte, time.Duration, time.Duration, time.Duration, error) {
	witnessStart := time.Now()
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	witnessTime := time.Since(witnessStart)
	if err != nil {
		return nil, 0, nil, witnessTime, 0, 0, err
	}
	publicWitness, err := witness.Public()
	if err != nil {
		return nil, 0, nil, witnessTime, 0, 0, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
	proveTime := time.Since(proveStart)
	if err != nil {
		return nil, 0, nil, witnessTime, proveTime, 0, err
	}
	verifyStart := time.Now()
	if err := plonk.Verify(proof, loaded.VK, publicWitness); err != nil {
		return nil, 0, nil, witnessTime, proveTime, time.Since(verifyStart), err
	}
	verifyTime := time.Since(verifyStart)
	var binary bytes.Buffer
	if _, err := proof.WriteTo(&binary); err != nil {
		return nil, 0, nil, witnessTime, proveTime, verifyTime, err
	}
	solidityProof := proof.(solidityMarshaler).MarshalSolidity()
	return proof, binary.Len(), solidityProof, witnessTime, proveTime, verifyTime, nil
}

func milliseconds(value time.Duration) float64 { return float64(value.Microseconds()) / 1000 }
