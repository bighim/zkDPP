package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bighim/zkDPP/poc-v2/internal/assignments"
	"github.com/bighim/zkDPP/poc-v2/internal/proofspec"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

type proofMarshaler interface{ MarshalSolidity() []byte }

type proofFixture struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}
type fixture struct {
	Relations map[string][]proofFixture `json:"relations"`
}
type manifest struct {
	Relation      string            `json:"relation"`
	Curve         string            `json:"curve"`
	ProofSystem   string            `json:"proofSystem"`
	StateLength   int               `json:"stateLength"`
	TreeDepth     int               `json:"treeDepth"`
	Constraints   int               `json:"constraints"`
	PublicInputs  int               `json:"publicInputs"`
	CompileMillis int64             `json:"compileMillis"`
	SetupMillis   int64             `json:"setupMillis"`
	ProveMillis   int64             `json:"proveMillis"`
	VerifyMillis  int64             `json:"verifyMillis"`
	ProofBytes    int               `json:"proofBytes"`
	Files         map[string]string `json:"files"`
	FileBytes     map[string]int64  `json:"fileBytes"`
}

func main() {
	relation := flag.String("relation", "all", "relation name or all")
	flag.Parse()
	if err := run(*relation); err != nil {
		panic(err)
	}
}

func run(selected string) error {
	d, err := scenario.LoadCanonical(".")
	if err != nil {
		return err
	}
	a, err := assignments.Build(d)
	if err != nil {
		return err
	}
	specs := proofspec.Relations(d, a)
	fixtures := fixture{Relations: map[string][]proofFixture{}}
	for _, spec := range specs {
		if selected != "all" && selected != spec.Name {
			continue
		}
		cases, m, err := build(spec)
		if err != nil {
			return err
		}
		fixtures.Relations[spec.Name] = cases
		if err := writeJSON(filepath.Join("artifacts", "state-3", "depth-32", spec.Name, "manifest.json"), m); err != nil {
			return err
		}
		fmt.Printf("%-8s constraints=%d public=%d prove=%dms verify=%dms\n", spec.Name, m.Constraints, m.PublicInputs, m.ProveMillis, m.VerifyMillis)
	}
	if selected == "all" {
		if err := writeJSON(filepath.Join("contracts", "test", "fixtures", "canonical-proofs.json"), fixtures); err != nil {
			return err
		}
	}
	return nil
}

func build(spec proofspec.Relation) ([]proofFixture, manifest, error) {
	compileStart := time.Now()
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, spec.Circuit)
	if err != nil {
		return nil, manifest{}, err
	}
	compileDuration := time.Since(compileStart)
	want := len(protocol.CanonicalManifests()[proofspec.EventIndex(spec.Name)].Inputs)
	got := ccs.GetNbPublicVariables()
	if got != want {
		return nil, manifest{}, fmt.Errorf("%s public variables=%d, manifest=%d", spec.Name, got, want)
	}
	setupStart := time.Now()
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	if err != nil {
		return nil, manifest{}, err
	}
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		return nil, manifest{}, err
	}
	setupDuration := time.Since(setupStart)
	dir := filepath.Join("artifacts", "state-3", "depth-32", spec.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, manifest{}, err
	}
	files := map[string]io.WriterTo{"ccs.bin": ccs, "srs-canonical.bin": srs, "srs-lagrange.bin": srsLagrange, "proving.key": pk, "verifying.key": vk}
	checksums := map[string]string{}
	sizes := map[string]int64{}
	for name, value := range files {
		path := filepath.Join(dir, name)
		if err := writeBinary(path, value); err != nil {
			return nil, manifest{}, err
		}
		checksums[name], _ = checksum(path)
		stat, _ := os.Stat(path)
		sizes[name] = stat.Size()
	}
	verifierDir := filepath.Join("contracts", "src", "generated", spec.Name)
	if err := os.MkdirAll(verifierDir, 0o755); err != nil {
		return nil, manifest{}, err
	}
	verifierPath := filepath.Join(verifierDir, "PlonkVerifier.sol")
	f, err := os.Create(verifierPath)
	if err != nil {
		return nil, manifest{}, err
	}
	if err := vk.ExportSolidity(f); err != nil {
		_ = f.Close()
		return nil, manifest{}, err
	}
	if err := f.Close(); err != nil {
		return nil, manifest{}, err
	}
	checksums["PlonkVerifier.sol"], _ = checksum(verifierPath)
	stat, _ := os.Stat(verifierPath)
	sizes["PlonkVerifier.sol"] = stat.Size()
	cases := make([]proofFixture, len(spec.Cases))
	var proveDuration, verifyDuration time.Duration
	proofBytes := 0
	for i, item := range spec.Cases {
		witness, err := frontend.NewWitness(item.Assignment, ecc.BLS12_381.ScalarField())
		if err != nil {
			return nil, manifest{}, err
		}
		publicWitness, _ := witness.Public()
		start := time.Now()
		proof, err := plonk.Prove(ccs, pk, witness)
		proveDuration += time.Since(start)
		if err != nil {
			return nil, manifest{}, err
		}
		start = time.Now()
		if err := plonk.Verify(proof, vk, publicWitness); err != nil {
			return nil, manifest{}, err
		}
		verifyDuration += time.Since(start)
		encoded := proof.(proofMarshaler).MarshalSolidity()
		proofBytes += len(encoded)
		cases[i] = proofFixture{Proof: "0x" + hex.EncodeToString(encoded), PublicInputs: proofspec.FieldStrings(item.PublicInputs)}
	}
	m := manifest{Relation: spec.Name, Curve: "BLS12-381", ProofSystem: "PLONK-KZG", StateLength: 3, TreeDepth: 32, Constraints: ccs.GetNbConstraints(), PublicInputs: got, CompileMillis: compileDuration.Milliseconds(), SetupMillis: setupDuration.Milliseconds(), ProveMillis: (proveDuration / time.Duration(len(cases))).Milliseconds(), VerifyMillis: (verifyDuration / time.Duration(len(cases))).Milliseconds(), ProofBytes: proofBytes / len(cases), Files: checksums, FileBytes: sizes}
	return cases, m, nil
}
func writeBinary(path string, value io.WriterTo) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := value.WriteTo(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
func checksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
