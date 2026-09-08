package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m1case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
)

type Environment struct {
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	GoVersion   string `json:"goVersion"`
	Gnark       string `json:"gnark"`
	GnarkCrypto string `json:"gnarkCrypto"`
	NumCPU      int    `json:"numCPU"`
	GOMAXPROCS  int    `json:"gomaxprocs"`
}

type Result struct {
	Feature           string           `json:"feature"`
	Constraints       int              `json:"constraints"`
	PublicInputs      int              `json:"publicInputs"`
	CompileMillis     int64            `json:"compileMillis"`
	SetupMillis       int64            `json:"setupMillis"`
	WitnessMillis     int64            `json:"witnessMillis"`
	ProveMillis       int64            `json:"proveMillis"`
	VerifyMillis      int64            `json:"verifyMillis"`
	ProofBytes        int              `json:"proofBytes"`
	ArtifactFileBytes map[string]int64 `json:"artifactFileBytes"`
}

type Report struct {
	GeneratedAt string      `json:"generatedAt"`
	RunCount    int         `json:"runCount"`
	Curve       string      `json:"curve"`
	ProofSystem string      `json:"proofSystem"`
	Hash        string      `json:"hash"`
	TreeDepth   int         `json:"treeDepth"`
	Environment Environment `json:"environment"`
	Results     []Result    `json:"results"`
}

func main() {
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if err := run(*root); err != nil {
		panic(err)
	}
}

func run(root string) error {
	cases, err := m1case.All(root)
	if err != nil {
		return err
	}
	results := make([]Result, 0, len(cases))
	for _, item := range cases {
		loaded, err := artifact.Load(root, item.Name)
		if err != nil {
			return fmt.Errorf("load %s: %w", item.Name, err)
		}
		witnessStart := time.Now()
		witness, err := frontend.NewWitness(item.Assignment, ecc.BLS12_381.ScalarField())
		witnessMillis := time.Since(witnessStart).Milliseconds()
		if err != nil {
			return err
		}
		publicWitness, err := witness.Public()
		if err != nil {
			return err
		}
		proveStart := time.Now()
		proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
		proveMillis := time.Since(proveStart).Milliseconds()
		if err != nil {
			return err
		}
		verifyStart := time.Now()
		if err := plonk.Verify(proof, loaded.VK, publicWitness); err != nil {
			return err
		}
		verifyMillis := time.Since(verifyStart).Milliseconds()
		var encoded bytes.Buffer
		if _, err := proof.WriteTo(&encoded); err != nil {
			return err
		}
		result := Result{
			Feature: item.Name, Constraints: loaded.Manifest.Constraints, PublicInputs: loaded.Manifest.PublicInputs,
			CompileMillis: loaded.Manifest.CompileMillis, SetupMillis: loaded.Manifest.SetupMillis,
			WitnessMillis: witnessMillis, ProveMillis: proveMillis, VerifyMillis: verifyMillis,
			ProofBytes: encoded.Len(), ArtifactFileBytes: loaded.Manifest.FileBytes,
		}
		results = append(results, result)
		fmt.Printf("%-16s constraints=%d public=%d prove=%dms verify=%dms proof=%dB\n", result.Feature, result.Constraints, result.PublicInputs, result.ProveMillis, result.VerifyMillis, result.ProofBytes)
	}
	report := Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), RunCount: 1,
		Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Hash: "Poseidon2-Merkle-Damgard", TreeDepth: merkle.Depth,
		Environment: environment(), Results: results,
	}
	return artifact.WriteJSON(root+string(os.PathSeparator)+"output"+string(os.PathSeparator)+"m1-core.json", report)
}

func environment() Environment {
	return Environment{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(),
		Gnark:       moduleVersion("github.com/consensys/gnark"),
		GnarkCrypto: moduleVersion("github.com/consensys/gnark-crypto"),
		NumCPU:      runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0),
	}
}

func moduleVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path == path {
			return dependency.Version
		}
	}
	return "unknown"
}
