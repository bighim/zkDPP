package proofruntime

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
	Relation string
	CCS      constraint.ConstraintSystem
	PK       plonk.ProvingKey
	LoadTime time.Duration
}

type manifest struct {
	Files map[string]string `json:"files"`
}

func Load(root, relation string) (*Loaded, error) {
	start := time.Now()
	dir := filepath.Join(root, "artifacts", "state-3", "depth-32", relation)
	var metadata manifest
	if err := readJSON(filepath.Join(dir, "manifest.json"), &metadata); err != nil {
		return nil, fmt.Errorf("read %s manifest: %w", relation, err)
	}
	for _, name := range []string{"ccs.bin", "proving.key"} {
		want, ok := metadata.Files[name]
		if !ok || want == "" {
			return nil, fmt.Errorf("%s manifest has no checksum for %s", relation, name)
		}
		got, err := checksum(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if got != want {
			return nil, fmt.Errorf("%s checksum mismatch for %s: got %s, want %s", relation, name, got, want)
		}
	}
	ccs := plonk.NewCS(ecc.BLS12_381)
	if err := readBinary(filepath.Join(dir, "ccs.bin"), ccs); err != nil {
		return nil, fmt.Errorf("read %s CCS: %w", relation, err)
	}
	pk := plonk.NewProvingKey(ecc.BLS12_381)
	if err := readBinary(filepath.Join(dir, "proving.key"), pk); err != nil {
		return nil, fmt.Errorf("read %s proving key: %w", relation, err)
	}
	return &Loaded{Relation: relation, CCS: ccs, PK: pk, LoadTime: time.Since(start)}, nil
}

func (loaded *Loaded) Prove(assignment frontend.Circuit) (plonk.Proof, time.Duration, time.Duration, error) {
	witnessStart := time.Now()
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	witnessTime := time.Since(witnessStart)
	if err != nil {
		return nil, witnessTime, 0, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
	proveTime := time.Since(proveStart)
	if err != nil {
		return nil, witnessTime, proveTime, err
	}
	return proof, witnessTime, proveTime, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
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
