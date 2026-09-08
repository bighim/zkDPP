package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m1case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

var binaryNames = []string{
	"ccs.bin",
	"srs-canonical.bin",
	"srs-lagrange.bin",
	"proving.key",
	"verifying.key",
}

type Manifest struct {
	Version          int               `json:"version"`
	Milestone        string            `json:"milestone,omitempty"`
	Relation         string            `json:"relation"`
	DevelopmentOnly  bool              `json:"developmentOnly"`
	Curve            string            `json:"curve"`
	ProofSystem      string            `json:"proofSystem"`
	Hash             string            `json:"hash"`
	TreeDepth        int               `json:"treeDepth"`
	Constraints      int               `json:"constraints"`
	PublicInputs     int               `json:"publicInputs"`
	CompileMillis    int64             `json:"compileMillis"`
	SRSMillis        int64             `json:"srsMillis"`
	PlonkSetupMillis int64             `json:"plonkSetupMillis"`
	SetupMillis      int64             `json:"setupMillis"`
	Files            map[string]string `json:"files"`
	FileBytes        map[string]int64  `json:"fileBytes"`
	GeneratedAt      string            `json:"generatedAt"`
}

type Loaded struct {
	Manifest Manifest
	CCS      constraint.ConstraintSystem
	PK       plonk.ProvingKey
	VK       plonk.VerifyingKey
}

type Spec struct {
	Name           string
	Circuit        frontend.Circuit
	ExpectedPublic int
}

func Setup(root string, item m1case.Case) (Manifest, error) {
	return SetupAt(root, "m1", Spec{Name: item.Name, Circuit: item.Circuit, ExpectedPublic: item.ExpectedPublic})
}

func SetupAt(root, milestone string, item Spec) (Manifest, error) {
	compileStart := time.Now()
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.Circuit)
	if err != nil {
		return Manifest{}, fmt.Errorf("compile %s: %w", item.Name, err)
	}
	compileMillis := time.Since(compileStart).Milliseconds()
	if got := ccs.GetNbPublicVariables(); got != item.ExpectedPublic {
		return Manifest{}, fmt.Errorf("%s public variables=%d, want %d", item.Name, got, item.ExpectedPublic)
	}
	srsStart := time.Now()
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	if err != nil {
		return Manifest{}, fmt.Errorf("create %s development SRS: %w", item.Name, err)
	}
	srsMillis := time.Since(srsStart).Milliseconds()
	plonkStart := time.Now()
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		return Manifest{}, fmt.Errorf("setup %s PLONK: %w", item.Name, err)
	}
	plonkMillis := time.Since(plonkStart).Milliseconds()
	dir := relationDir(root, milestone, item.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Manifest{}, err
	}
	files := map[string]io.WriterTo{
		"ccs.bin": ccs, "srs-canonical.bin": srs, "srs-lagrange.bin": srsLagrange,
		"proving.key": pk, "verifying.key": vk,
	}
	manifest := Manifest{
		Version: 1, Milestone: milestone, Relation: item.Name, DevelopmentOnly: true,
		Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Hash: "Poseidon2-Merkle-Damgard",
		TreeDepth: merkle.Depth, Constraints: ccs.GetNbConstraints(), PublicInputs: ccs.GetNbPublicVariables(),
		CompileMillis: compileMillis, SRSMillis: srsMillis, PlonkSetupMillis: plonkMillis,
		SetupMillis: srsMillis + plonkMillis, Files: map[string]string{}, FileBytes: map[string]int64{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, name := range binaryNames {
		path := filepath.Join(dir, name)
		if err := writeBinary(path, files[name]); err != nil {
			return Manifest{}, err
		}
		manifest.Files[name], err = Checksum(path)
		if err != nil {
			return Manifest{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return Manifest{}, err
		}
		manifest.FileBytes[name] = info.Size()
	}
	if err := WriteJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		return Manifest{}, err
	}
	if _, err := LoadAt(root, milestone, item.Name); err != nil {
		return Manifest{}, fmt.Errorf("reload %s artifact: %w", item.Name, err)
	}
	return manifest, nil
}

func Load(root, relation string) (*Loaded, error) {
	return LoadAt(root, "m1", relation)
}

func LoadAt(root, milestone, relation string) (*Loaded, error) {
	dir := relationDir(root, milestone, relation)
	var manifest Manifest
	if err := readJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != 1 || (manifest.Milestone != "" && manifest.Milestone != milestone) || manifest.Relation != relation || !manifest.DevelopmentOnly || manifest.Curve != "BLS12-381" || manifest.ProofSystem != "PLONK-KZG" {
		return nil, fmt.Errorf("%s manifest metadata mismatch", relation)
	}
	for _, name := range binaryNames {
		want := manifest.Files[name]
		if want == "" {
			return nil, fmt.Errorf("%s manifest has no checksum for %s", relation, name)
		}
		got, err := Checksum(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if got != want {
			return nil, fmt.Errorf("%s checksum mismatch for %s", relation, name)
		}
	}
	ccs := plonk.NewCS(ecc.BLS12_381)
	if err := readBinary(filepath.Join(dir, "ccs.bin"), ccs); err != nil {
		return nil, err
	}
	pk := plonk.NewProvingKey(ecc.BLS12_381)
	if err := readBinary(filepath.Join(dir, "proving.key"), pk); err != nil {
		return nil, err
	}
	vk := plonk.NewVerifyingKey(ecc.BLS12_381)
	if err := readBinary(filepath.Join(dir, "verifying.key"), vk); err != nil {
		return nil, err
	}
	return &Loaded{Manifest: manifest, CCS: ccs, PK: pk, VK: vk}, nil
}

func ExportSolidity(path string, vk plonk.VerifyingKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := vk.ExportSolidity(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func Checksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func WriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func relationDir(root, milestone, relation string) string {
	return filepath.Join(root, "artifacts", "development", milestone, relation)
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

func readBinary(path string, target io.ReaderFrom) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = target.ReadFrom(file)
	return err
}

func readJSON(path string, target any) error {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}
