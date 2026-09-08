package m6b1run

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
)

const Milestone = "m6-b1"
const Core = "audit-encryption"
const Process = "audit-process-3-2"
const FoundryImage = "ghcr.io/foundry-rs/foundry:v1.7.1"

func ArtifactDir(root string, parts ...string) string {
	return filepath.Join(append([]string{root, "artifacts", "development", Milestone}, parts...)...)
}
func Millis(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

type Environment struct {
	GoVersion        string
	GOOS             string
	GOARCH           string
	CPU              string
	NumCPU           int
	RequestedThreads int
	GOMAXPROCS       int
	Modules          map[string]string
	ProofSystem      string
	ProofCurve       string
	EncryptionCurve  string
	Hash             string
	Profile          string
}

func Env() Environment {
	cpu := runtime.GOARCH
	if runtime.GOOS == "darwin" {
		if b, e := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); e == nil {
			cpu = strings.TrimSpace(string(b))
		}
	}
	e := Environment{GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPU: cpu, NumCPU: runtime.NumCPU(), RequestedThreads: 8, GOMAXPROCS: runtime.GOMAXPROCS(0), Modules: map[string]string{}, ProofSystem: "PLONK-KZG", ProofCurve: "BLS12-381", EncryptionCurve: "Jubjub", Hash: "Poseidon2-Merkle-Damgard", Profile: auditcrypto.Profile}
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, d := range b.Deps {
			if strings.Contains(d.Path, "gnark") || strings.Contains(d.Path, "go-ethereum") {
				e.Modules[d.Path] = d.Version
			}
		}
	}
	return e
}
func ReadJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func Write(root, path string, v any) error { return artifact.WriteJSON(filepath.Join(root, path), v) }

type Attempt struct {
	Name       string
	Number     int
	StartedAt  string
	FinishedAt string
	Status     string
	Error      string
}

func Begin(root, name, output string) (int, func(error), error) {
	if output != "" {
		if _, e := os.Stat(filepath.Join(root, output)); e == nil {
			return 0, nil, fmt.Errorf("%s already exists; refusing duplicate official measurement", output)
		} else if !os.IsNotExist(e) {
			return 0, nil, e
		}
	}
	dir := ArtifactDir(root, "attempts")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return 0, nil, e
	}
	paths, e := filepath.Glob(filepath.Join(dir, name+"-*.json"))
	if e != nil {
		return 0, nil, e
	}
	a := Attempt{Name: name, Number: len(paths) + 1, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "running"}
	path := filepath.Join(dir, fmt.Sprintf("%s-%02d.json", name, a.Number))
	if e = artifact.WriteJSON(path, a); e != nil {
		return 0, nil, e
	}
	finish := func(err error) {
		a.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		a.Status = "success"
		if err != nil {
			a.Status = "failed"
			a.Error = err.Error()
		}
		_ = artifact.WriteJSON(path, a)
	}
	return a.Number, finish, nil
}

type Binding struct {
	Profile           string
	PublicKeyChecksum string
	Relation          string
	PublicInputs      int
	MembershipPaths   int
	TreeDepth         int
}

func BindingFor(c auditcrypto.PublicConfig, relation string) Binding {
	b := Binding{Profile: auditcrypto.Profile, PublicKeyChecksum: c.Checksum, Relation: relation, PublicInputs: 8}
	if relation == Process {
		b.PublicInputs = 15
		b.MembershipPaths = 3
		b.TreeDepth = 32
	}
	return b
}
func CheckBinding(root string, c auditcrypto.PublicConfig, relation string) error {
	var b Binding
	if e := ReadJSON(ArtifactDir(root, relation, "binding.json"), &b); e != nil {
		return e
	}
	if b != BindingFor(c, relation) {
		return fmt.Errorf("%s public key binding mismatch", relation)
	}
	return nil
}
func FinalChecks(root string) error {
	var base struct {
		Algorithm string            `json:"algorithm"`
		Files     map[string]string `json:"files"`
	}
	if e := ReadJSON(ArtifactDir(root, "baseline.json"), &base); e != nil {
		return e
	}
	for p, want := range base.Files {
		got, e := artifact.Checksum(filepath.Join(root, "..", p))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("protected baseline changed: %s", p)
		}
	}
	files := map[string]string{}
	roots := []string{
		"internal/core/auditcrypto", "internal/m6b1case", "internal/m6b1run",
		"features/audit_encryption", "features/audit_process_3_2",
		"cmd/setup_m6_b1", "cmd/evaluate_m6_b1", "cmd/benchmark_m6_b1",
		"artifacts/development/m6-b1", "contracts/src/generated/audit-process-3-2",
	}
	for _, dir := range roots {
		e := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasPrefix(d.Name(), "member-") && strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			h, e := artifact.Checksum(p)
			if e != nil {
				return e
			}
			files[rel] = h
			return nil
		})
		if e != nil {
			return e
		}
	}
	for _, p := range []string{"internal/circuitutil/audit_encryption.go", "contracts/src/AuditProcessVerifier.sol", "contracts/src/AuditEncryptionStore.sol", "contracts/test/AuditEncryptionStore.t.sol", "contracts/test/fixtures/m6-b1-proofs.json", "docker-compose.m6-b1.yml", "output/m6-b1-circuit.json", "output/m6-b1-anvil-gas.json", "output/m6-b1-decryption.json"} {
		h, e := artifact.Checksum(filepath.Join(root, p))
		if e != nil {
			return e
		}
		files[p] = h
	}
	config, _, e := auditcrypto.LoadPublic(ArtifactDir(root, "committee"))
	if e != nil {
		return e
	}
	attempts := []Attempt{}
	paths, _ := filepath.Glob(ArtifactDir(root, "attempts", "*.json"))
	for _, p := range paths {
		var a Attempt
		if e = ReadJSON(p, &a); e != nil {
			return e
		}
		attempts = append(attempts, a)
	}
	out := struct {
		Algorithm          string
		PublicKeyChecksum  string
		Files              map[string]string
		ProtectedFiles     map[string]string
		ProtectedUnchanged bool
		Attempts           []Attempt
	}{"SHA-256", config.Checksum, files, base.Files, true, attempts}
	return Write(root, "output/m6-b1-generated-checksums.json", out)
}
