package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type fileDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type metadata struct {
	GeneratedAt  string       `json:"generatedAt"`
	GitCommit    string       `json:"gitCommit"`
	GitDirty     bool         `json:"gitDirty"`
	GoVersion    string       `json:"goVersion"`
	GOOS         string       `json:"goos"`
	GOARCH       string       `json:"goarch"`
	OSVersion    string       `json:"osVersion"`
	CPU          string       `json:"cpu"`
	CPUCores     int          `json:"cpuCores"`
	RAMBytes     string       `json:"ramBytes"`
	ProofSystem  string       `json:"proofSystem"`
	Gnark        string       `json:"gnark"`
	GnarkCrypto  string       `json:"gnarkCrypto"`
	Foundry      string       `json:"foundry"`
	Solidity     string       `json:"solidity"`
	EVMRevision  string       `json:"evmRevision"`
	Besu         string       `json:"besu"`
	BesuImage    string       `json:"besuImage"`
	HashProfiles []string     `json:"hashProfiles"`
	Files        []fileDigest `json:"files"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	out := flag.String("out", "benchmarks/environment.json", "metadata JSON")
	flag.Parse()

	commit := command("git", "-C", *root, "rev-parse", "HEAD")
	dirty := command("git", "-C", *root, "status", "--porcelain", "--", ".") != ""
	cpu := command("sysctl", "-n", "machdep.cpu.brand_string")
	ram := command("sysctl", "-n", "hw.memsize")
	if cpu == "unavailable" || ram == "unavailable" {
		fallbackCPU, fallbackRAM := previousMachine(filepath.Join(*root, "../poc-v2/benchmarks/anvil-e2e-time.json"))
		if cpu == "unavailable" {
			cpu = fallbackCPU
		}
		if ram == "unavailable" {
			ram = fallbackRAM
		}
	}
	value := metadata{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		GitCommit:   commit, GitDirty: dirty, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		OSVersion: command("sw_vers", "-productVersion"), CPU: cpu,
		CPUCores: runtime.NumCPU(), RAMBytes: ram,
		ProofSystem: "PLONK-KZG/BLS12-381", Gnark: goModVersion(filepath.Join(*root, "go.mod"), "github.com/consensys/gnark"),
		GnarkCrypto: goModVersion(filepath.Join(*root, "go.mod"), "github.com/consensys/gnark-crypto"),
		Foundry:     "1.7.1", Solidity: "0.8.30", EVMRevision: "Prague",
		Besu:         "26.7.1",
		BesuImage:    "hyperledger/besu@sha256:5c319f8f5f3449438c03ea7fa2c9bf24b866dc55ac98d802bb41ad793e740587",
		HashProfiles: []string{"poseidon2", "sha256"},
	}
	for _, path := range []string{
		"go.sum", "config/events.json", "config/paper-benchmarks.json", "testdata/hash-golden-vectors.json",
		"circuits/gadgets/circuits.go", "internal/events/fixtures.go", "internal/document/hash.go",
		"contracts/src/SahaiLedger.sol", "contracts/src/BenchmarkSahaiLedger.sol",
		"besu/qbftConfigFile.json", "docker-compose.yml",
	} {
		value.Files = append(value.Files, fileDigest{Path: path, SHA256: digest(filepath.Join(*root, path))})
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(*root, *out)), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*root, *out), append(raw, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func command(name string, args ...string) string {
	raw, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(raw))
}

func goModVersion(goMod, module string) string {
	raw, err := os.ReadFile(goMod)
	if err != nil {
		return "unavailable"
	}
	fields := strings.Fields(string(raw))
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == module {
			return fields[i+1]
		}
	}
	return "unavailable"
}

func digest(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "unavailable"
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func previousMachine(path string) (string, string) {
	var value struct {
		Machine struct {
			CPU      string `json:"cpu"`
			RAMBytes string `json:"ramBytes"`
		} `json:"machine"`
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &value) != nil {
		return "unavailable", "unavailable"
	}
	return value.Machine.CPU, value.Machine.RAMBytes
}
