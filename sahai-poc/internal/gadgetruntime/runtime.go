package gadgetruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
)

type Loaded struct {
	Profile  string
	Gadget   string
	CCS      constraint.ConstraintSystem
	PK       plonk.ProvingKey
	VK       plonk.VerifyingKey
	LoadTime time.Duration
}

type metadata struct {
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

func Load(root, profile, gadget string) (*Loaded, error) {
	start := time.Now()
	dir := filepath.Join(root, "artifacts", "gadgets", profile, gadget)
	var manifest metadata
	if err := readJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		return nil, err
	}
	for _, name := range []string{"ccs.bin", "proving.key", "verifying.key"} {
		want := manifest.ArtifactSHA256[name]
		got, err := checksum(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if want == "" || want != got {
			return nil, fmt.Errorf("%s %s checksum mismatch", gadget, name)
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
	return &Loaded{Profile: profile, Gadget: gadget, CCS: ccs, PK: pk, VK: vk, LoadTime: time.Since(start)}, nil
}

func (loaded *Loaded) Verify(proof plonk.Proof, assignment frontend.Circuit) (time.Duration, error) {
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return 0, err
	}
	start := time.Now()
	err = plonk.Verify(proof, loaded.VK, witness)
	return time.Since(start), err
}

func (loaded *Loaded) Prove(assignment frontend.Circuit) (plonk.Proof, time.Duration, time.Duration, error) {
	witnessStart := time.Now()
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	witnessDuration := time.Since(witnessStart)
	if err != nil {
		return nil, witnessDuration, 0, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
	return proof, witnessDuration, time.Since(proveStart), err
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
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

func checksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
