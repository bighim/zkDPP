package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bighim/zkDPP/sahai-poc/circuits/gadgets"
	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

type proofMarshaler interface{ MarshalSolidity() []byte }

type fixture struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}

type manifest struct {
	HashProfile    string            `json:"hashProfile"`
	Gadget         string            `json:"gadget"`
	Curve          string            `json:"curve"`
	ProofSystem    string            `json:"proofSystem"`
	Hash           string            `json:"hash"`
	Constraints    int               `json:"constraints"`
	PublicInputs   int               `json:"publicInputs"`
	CompileMS      float64           `json:"compileMs"`
	SetupMS        float64           `json:"setupMs"`
	ProveMS        float64           `json:"proveMs"`
	VerifyMS       float64           `json:"verifyMs"`
	ProofBytes     int               `json:"proofBytes"`
	ArtifactBytes  map[string]int64  `json:"artifactBytes"`
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

func main() {
	selected := flag.String("gadget", "all", "gadget name or all")
	selectedProfile := flag.String("profile", "all", "poseidon2, sha256, or all")
	flag.Parse()
	profiles := []document.Profile{document.Poseidon2, document.SHA256}
	if *selectedProfile != "all" {
		profile, err := document.ParseProfile(*selectedProfile)
		if err != nil {
			fatal(err)
		}
		profiles = []document.Profile{profile}
	}
	for _, profile := range profiles {
		assignments, err := gadgets.CanonicalAssignmentsFor(profile)
		if err != nil {
			fatal(err)
		}
		fixtures := map[string]fixture{}
		manifests := make([]manifest, 0, len(assignments))
		for _, spec := range assignments {
			if *selected != "all" && *selected != spec.Gadget {
				continue
			}
			item, data, err := build(spec)
			if err != nil {
				fatal(fmt.Errorf("%s: %w", spec.Name, err))
			}
			fixtures[spec.Gadget] = item
			manifests = append(manifests, data)
			fmt.Printf("profile=%-9s gadget=%-10s constraints=%d public=%d\n", profile, spec.Gadget, data.Constraints, data.PublicInputs)
		}
		if err := writeJSON(filepath.Join("contracts", "test", "fixtures", string(profile)+"-gadget-proofs.json"), map[string]any{"hashProfile": profile, "gadgets": fixtures}); err != nil {
			fatal(err)
		}
		if err := writeJSON(filepath.Join("benchmarks", "setup-"+string(profile)+".json"), map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339), "hashProfile": profile, "results": manifests}); err != nil {
			fatal(err)
		}
		if err := writeCSV(filepath.Join("benchmarks", "setup-"+string(profile)+".csv"), manifests); err != nil {
			fatal(err)
		}
	}
}

func build(spec gadgets.NamedAssignment) (fixture, manifest, error) {
	compileStart := time.Now()
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, spec.Circuit)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	compileDuration := time.Since(compileStart)
	if ccs.GetNbPublicVariables() != len(spec.PublicInputs) {
		return fixture{}, manifest{}, fmt.Errorf("public inputs: ccs=%d assignment=%d", ccs.GetNbPublicVariables(), len(spec.PublicInputs))
	}
	setupStart := time.Now()
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	setupDuration := time.Since(setupStart)
	witness, err := frontend.NewWitness(spec.Assignment, ecc.BLS12_381.ScalarField())
	if err != nil {
		return fixture{}, manifest{}, err
	}
	publicWitness, err := witness.Public()
	if err != nil {
		return fixture{}, manifest{}, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(ccs, pk, witness)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	proveDuration := time.Since(proveStart)
	verifyStart := time.Now()
	if err := plonk.Verify(proof, vk, publicWitness); err != nil {
		return fixture{}, manifest{}, err
	}
	verifyDuration := time.Since(verifyStart)
	dir := filepath.Join("artifacts", "gadgets", string(spec.Profile), spec.Gadget)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fixture{}, manifest{}, err
	}
	files := map[string]io.WriterTo{
		"ccs.bin": ccs, "srs-canonical.bin": srs, "srs-lagrange.bin": srsLagrange,
		"proving.key": pk, "verifying.key": vk,
	}
	sizes := map[string]int64{}
	checksums := map[string]string{}
	for name, value := range files {
		path := filepath.Join(dir, name)
		if err := writeBinary(path, value); err != nil {
			return fixture{}, manifest{}, err
		}
		sizes[name], checksums[name], err = fileMetadata(path)
		if err != nil {
			return fixture{}, manifest{}, err
		}
	}
	verifierPath := filepath.Join("contracts", "src", "generated", string(spec.Profile), spec.Gadget, "PlonkVerifier.sol")
	if err := os.MkdirAll(filepath.Dir(verifierPath), 0o755); err != nil {
		return fixture{}, manifest{}, err
	}
	verifier, err := os.Create(verifierPath)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	if err := vk.ExportSolidity(verifier); err != nil {
		_ = verifier.Close()
		return fixture{}, manifest{}, err
	}
	if err := verifier.Close(); err != nil {
		return fixture{}, manifest{}, err
	}
	rawVerifier, err := os.ReadFile(verifierPath)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	uniqueName := verifierContractName(spec.Profile, spec.Gadget)
	rawVerifier = []byte(strings.Replace(string(rawVerifier), "contract PlonkVerifier", "contract "+uniqueName, 1))
	if err := os.WriteFile(verifierPath, rawVerifier, 0o644); err != nil {
		return fixture{}, manifest{}, err
	}
	sizes["PlonkVerifier.sol"], checksums["PlonkVerifier.sol"], err = fileMetadata(verifierPath)
	if err != nil {
		return fixture{}, manifest{}, err
	}
	encoded := proof.(proofMarshaler).MarshalSolidity()
	public := make([]string, len(spec.PublicInputs))
	for i, value := range spec.PublicInputs {
		public[i] = value.String()
	}
	data := manifest{
		HashProfile: string(spec.Profile), Gadget: spec.Gadget, Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Hash: string(spec.Profile),
		Constraints: ccs.GetNbConstraints(), PublicInputs: ccs.GetNbPublicVariables(),
		CompileMS: millis(compileDuration), SetupMS: millis(setupDuration), ProveMS: millis(proveDuration),
		VerifyMS: millis(verifyDuration), ProofBytes: len(encoded), ArtifactBytes: sizes, ArtifactSHA256: checksums,
	}
	if err := writeJSON(filepath.Join(dir, "manifest.json"), data); err != nil {
		return fixture{}, manifest{}, err
	}
	return fixture{Proof: "0x" + hex.EncodeToString(encoded), PublicInputs: public}, data, nil
}

func writeBinary(path string, value io.WriterTo) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := value.WriteTo(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func writeCSV(path string, values []manifest) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	_ = writer.Write([]string{"hash_profile", "gadget", "constraints", "public_inputs", "compile_ms", "setup_ms", "prove_ms", "verify_ms", "proof_bytes"})
	for _, value := range values {
		_ = writer.Write([]string{value.HashProfile, value.Gadget, strconv.Itoa(value.Constraints), strconv.Itoa(value.PublicInputs),
			fmt.Sprintf("%.6f", value.CompileMS), fmt.Sprintf("%.6f", value.SetupMS), fmt.Sprintf("%.6f", value.ProveMS),
			fmt.Sprintf("%.6f", value.VerifyMS), strconv.Itoa(value.ProofBytes)})
	}
	return writer.Error()
}

func fileMetadata(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, "", err
	}
	return info.Size(), hex.EncodeToString(hash.Sum(nil)), nil
}

func millis(value time.Duration) float64 { return float64(value) / float64(time.Millisecond) }

func verifierContractName(profile document.Profile, gadget string) string {
	prefix := "Poseidon2"
	if profile == document.SHA256 {
		prefix = "SHA256"
	}
	name := map[string]string{"merklepath": "MerklePath", "eq": "Eq", "add": "Add", "and": "And"}[gadget]
	return prefix + name + "Verifier"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
