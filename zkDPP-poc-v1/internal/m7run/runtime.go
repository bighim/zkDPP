package m7run

import (
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	b1 "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6b1run"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Milestone = "m7"
const FoundryImage = b1.FoundryImage

func Dir(root string, p ...string) string {
	return filepath.Join(append([]string{root, "artifacts", "development", Milestone}, p...)...)
}
func Name(k audit.Kind) string {
	if k == audit.Process {
		return "audit-process-3-2"
	}
	return "audit-" + k.String()
}
func Load(root string, k audit.Kind) (*artifact.Loaded, error) {
	m := Milestone
	if k == audit.Process {
		m = "m6-b1"
	}
	return artifact.LoadAt(root, m, Name(k))
}
func Committee(root string) (auditcrypto.Committee, error) {
	return auditcrypto.LoadCommittee(filepath.Join(root, "artifacts/development/m6-b1/committee"))
}
func Millis(d time.Duration) float64    { return float64(d.Nanoseconds()) / 1e6 }
func Write(root, p string, v any) error { return artifact.WriteJSON(filepath.Join(root, p), v) }
func Read(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}

type Attempt struct {
	Name                                 string
	Number                               int
	StartedAt, FinishedAt, Status, Error string
}

func Begin(root, name, output string) (int, func(error), error) {
	if output != "" {
		if _, e := os.Stat(filepath.Join(root, output)); e == nil {
			return 0, nil, fmt.Errorf("official output exists: %s", output)
		} else if !os.IsNotExist(e) {
			return 0, nil, e
		}
	}
	paths, e := filepath.Glob(Dir(root, "attempts", name+"-*.json"))
	if e != nil {
		return 0, nil, e
	}
	a := Attempt{Name: name, Number: len(paths) + 1, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "running"}
	path := Dir(root, "attempts", fmt.Sprintf("%s-%02d.json", name, a.Number))
	if e = artifact.WriteJSON(path, a); e != nil {
		return 0, nil, e
	}
	return a.Number, func(err error) {
		a.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		a.Status = "success"
		if err != nil {
			a.Status = "failed"
			a.Error = err.Error()
		}
		_ = artifact.WriteJSON(path, a)
	}, nil
}

type Binding struct {
	Profile, PublicKeyChecksum    string
	Kind                          audit.Kind
	PublicInputs, MembershipPaths int
}

func Bind(c auditcrypto.PublicConfig, k audit.Kind) Binding {
	l, _ := audit.Shape(k)
	return Binding{auditcrypto.Profile, c.Checksum, k, l.BaseCount + 2 + l.ParentCount + len(l.OutputTypes), l.ParentCount}
}
func CheckBinding(root string, c auditcrypto.PublicConfig, k audit.Kind) error {
	if k == audit.Process {
		return b1.CheckBinding(root, c, b1.Process)
	}
	var got Binding
	if e := Read(Dir(root, Name(k), "binding.json"), &got); e != nil {
		return e
	}
	if got != Bind(c, k) {
		return fmt.Errorf("committee/shape mismatch: %s", k)
	}
	return nil
}
func FinalChecks(root string) error {
	if e := ValidateReports(root); e != nil {
		return e
	}
	var base struct {
		Files map[string]string `json:"files"`
	}
	if e := Read(Dir(root, "baseline.json"), &base); e != nil {
		return e
	}
	for p, want := range base.Files {
		got, e := artifact.Checksum(filepath.Join(root, p))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("protected file changed: %s", p)
		}
	}
	files := map[string]string{}
	for _, dir := range []string{"internal/audit", "internal/m7case", "internal/m7run", "artifacts/development/m7", "cmd/setup_m7", "cmd/evaluate_m7", "cmd/benchmark_m7", "cmd/audit_m7"} {
		e := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			h, e := artifact.Checksum(p)
			if e == nil {
				files[rel] = h
			}
			return e
		})
		if e != nil {
			return e
		}
	}
	for _, pattern := range []string{"features/audit_*/circuit.go", "features/audit_*/README.md", "internal/circuitutil/audit_event.go", "contracts/src/ZkDPPAuditLedger.sol", "contracts/src/M7*Verifier.sol", "contracts/src/generated/m7-*/*", "contracts/test/M7*.sol", "contracts/test/fixtures/m7-*.json", "docker-compose.m7.yml", "output/m7-*.json"} {
		paths, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, p := range paths {
			if strings.HasSuffix(p, "m7-generated-checksums.json") {
				continue
			}
			h, e := artifact.Checksum(p)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, p)
			files[rel] = h
		}
	}
	c, e := Committee(root)
	if e != nil {
		return e
	}
	attempts := []Attempt{}
	paths, _ := filepath.Glob(Dir(root, "attempts", "*.json"))
	for _, p := range paths {
		var a Attempt
		if e = Read(p, &a); e != nil {
			return e
		}
		attempts = append(attempts, a)
	}
	return Write(root, "output/m7-generated-checksums.json", struct {
		Algorithm, PublicKeyChecksum string
		ProtectedUnchanged           bool
		ProtectedFiles, Files        map[string]string
		Attempts                     []Attempt
	}{"SHA-256", c.Public.Checksum, true, base.Files, files, attempts})
}
