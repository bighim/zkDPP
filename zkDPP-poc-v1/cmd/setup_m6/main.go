package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	statusmerge "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_merge"
	statusprivatespend "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_private_spend"
	statusproceed "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_proceed"
	statusprocess "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_process"
	statusrecall "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_recall"
	statussplit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_split"
	statustransfer "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_transfer"
	statusupdate "github.com/bighim/zkDPP/zkDPP-poc-v1/features/status_update"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6case"
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

type fixture struct {
	NoteStatusRoot    string                  `json:"noteStatusRoot"`
	VoucherStatusRoot string                  `json:"voucherStatusRoot"`
	Features          map[string]proofFixture `json:"features"`
}

type circuitResult struct {
	Feature                                                 string `json:"feature"`
	Constraints, PublicInputs                               int
	CompileMillis, SRSMillis, PlonkSetupMillis, SetupMillis int64
	WitnessMillis, ProveMillis, VerifyMillis                float64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactFileBytes                                       map[string]int64
}

type report struct {
	GeneratedAt        string `json:"generatedAt"`
	RunCount           int    `json:"runCount"`
	Curve, ProofSystem string
	Results            []circuitResult
}

type checksums struct {
	GeneratedAt, Algorithm string
	Files, PriorOutputs    map[string]string
}

func main() {
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if err := run(*root); err != nil {
		panic(err)
	}
}

func run(root string) error {
	s, err := m6case.Build(root)
	if err != nil {
		return err
	}
	items := []struct {
		spec       artifact.Spec
		assignment frontend.Circuit
	}{
		{artifact.Spec{Name: "status-update", Circuit: &statusupdate.Circuit{}, ExpectedPublic: 6}, s.StatusUpdate},
		{artifact.Spec{Name: "status-private-spend", Circuit: &statusprivatespend.Circuit{}, ExpectedPublic: 3}, s.PrivateSpend},
		{artifact.Spec{Name: "status-transfer", Circuit: &statustransfer.Circuit{}, ExpectedPublic: 7}, s.Transfer},
		{artifact.Spec{Name: "status-proceed", Circuit: &statusproceed.Circuit{}, ExpectedPublic: 4}, s.Proceed},
		{artifact.Spec{Name: "status-recall", Circuit: &statusrecall.Circuit{}, ExpectedPublic: 5}, s.Recall},
		{artifact.Spec{Name: "status-merge", Circuit: &statusmerge.Circuit{}, ExpectedPublic: 5}, s.Merge},
		{artifact.Spec{Name: "status-split", Circuit: &statussplit.Circuit{}, ExpectedPublic: 5}, s.Split},
		{artifact.Spec{Name: "status-process", Circuit: &statusprocess.Circuit{}, ExpectedPublic: 9}, s.Process},
	}
	results := make([]circuitResult, 0, len(items))
	fixtures := fixture{NoteStatusRoot: s.NoteStatusRoot.String(), VoucherStatusRoot: s.VoucherStatusRoot.String(), Features: map[string]proofFixture{}}
	generated := make([]string, 0, len(items)+1)
	for _, item := range items {
		manifest, setupErr := artifact.SetupAt(root, "m6", item.spec)
		if setupErr != nil {
			return setupErr
		}
		loaded, loadErr := artifact.LoadAt(root, "m6", item.spec.Name)
		if loadErr != nil {
			return loadErr
		}
		path := filepath.Join(root, "contracts", "src", "generated", item.spec.Name, "PlonkVerifier.sol")
		if exportErr := artifact.ExportSolidity(path, loaded.VK); exportErr != nil {
			return exportErr
		}
		generated = append(generated, filepath.ToSlash(filepath.Join("contracts", "src", "generated", item.spec.Name, "PlonkVerifier.sol")))
		binary, solidity, public, witnessTime, proveTime, verifyTime, proofErr := prove(loaded, item.assignment)
		if proofErr != nil {
			return proofErr
		}
		fixtures.Features[item.spec.Name] = proofFixture{Proof: "0x" + hex.EncodeToString(solidity), PublicInputs: public}
		results = append(results, circuitResult{
			Feature: item.spec.Name, Constraints: manifest.Constraints, PublicInputs: manifest.PublicInputs,
			CompileMillis: manifest.CompileMillis, SRSMillis: manifest.SRSMillis, PlonkSetupMillis: manifest.PlonkSetupMillis, SetupMillis: manifest.SetupMillis,
			WitnessMillis: ms(witnessTime), ProveMillis: ms(proveTime), VerifyMillis: ms(verifyTime),
			BinaryProofBytes: binary, SolidityProofBytes: len(solidity), ArtifactFileBytes: manifest.FileBytes,
		})
	}
	fixturePath := filepath.Join(root, "contracts", "test", "fixtures", "m6-proofs.json")
	if err = artifact.WriteJSON(fixturePath, fixtures); err != nil {
		return err
	}
	generated = append(generated, "contracts/test/fixtures/m6-proofs.json")
	if err = artifact.WriteJSON(filepath.Join(root, "output", "m6-circuit.json"), report{GeneratedAt: time.Now().UTC().Format(time.RFC3339), RunCount: 1, Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Results: results}); err != nil {
		return err
	}
	if err = writeChecksums(root, generated); err != nil {
		return err
	}
	for _, value := range results {
		fmt.Printf("%-22s constraints=%d public=%d proof=%dB\n", value.Feature, value.Constraints, value.PublicInputs, value.SolidityProofBytes)
	}
	return nil
}

func prove(loaded *artifact.Loaded, assignment frontend.Circuit) (int, []byte, []string, time.Duration, time.Duration, time.Duration, error) {
	start := time.Now()
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	witnessTime := time.Since(start)
	if err != nil {
		return 0, nil, nil, witnessTime, 0, 0, err
	}
	publicWitness, err := witness.Public()
	if err != nil {
		return 0, nil, nil, witnessTime, 0, 0, err
	}
	start = time.Now()
	proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
	proveTime := time.Since(start)
	if err != nil {
		return 0, nil, nil, witnessTime, proveTime, 0, err
	}
	start = time.Now()
	err = plonk.Verify(proof, loaded.VK, publicWitness)
	verifyTime := time.Since(start)
	if err != nil {
		return 0, nil, nil, witnessTime, proveTime, verifyTime, err
	}
	var buffer bytes.Buffer
	if _, err = proof.WriteTo(&buffer); err != nil {
		return 0, nil, nil, witnessTime, proveTime, verifyTime, err
	}
	vector, ok := publicWitness.Vector().(fr.Vector)
	if !ok {
		return 0, nil, nil, witnessTime, proveTime, verifyTime, fmt.Errorf("unexpected public vector %T", publicWitness.Vector())
	}
	public := make([]string, len(vector))
	for i := range vector {
		public[i] = vector[i].String()
	}
	return buffer.Len(), proof.(solidityMarshaler).MarshalSolidity(), public, witnessTime, proveTime, verifyTime, nil
}

func ms(value time.Duration) float64 { return float64(value.Microseconds()) / 1000 }

func writeChecksums(root string, files []string) error {
	result := checksums{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Algorithm: "SHA-256", Files: map[string]string{}, PriorOutputs: map[string]string{}}
	for _, name := range files {
		value, err := artifact.Checksum(filepath.Join(root, name))
		if err != nil {
			return err
		}
		result.Files[name] = value
	}
	prior := []string{"output/m1-core.json", "output/m2-circuit.json", "output/m2-anvil-gas.json", "output/m2-anvil-e2e.json", "output/m3-circuit.json", "output/m3-anvil-gas.json", "output/m3-anvil-e2e.json", "output/m4-circuit.json", "output/m4-anvil-gas.json", "output/m4-anvil-e2e.json", "output/m5-circuit.json", "output/m5-anvil-gas.json", "output/m5-anvil-e2e.json", "output/m5-verifier-ablation.json"}
	for _, name := range prior {
		value, err := artifact.Checksum(filepath.Join(root, name))
		if err != nil {
			return err
		}
		result.PriorOutputs[name] = value
	}
	return artifact.WriteJSON(filepath.Join(root, "output", "m6-generated-checksums.json"), result)
}
