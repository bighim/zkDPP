package m8run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
)

const Milestone = "m8"
const FoundryImage = m7run.FoundryImage

func Dir(root string, parts ...string) string {
	return filepath.Join(append([]string{root, "artifacts", "development", Milestone}, parts...)...)
}
func Load(root, name string) (*artifact.Loaded, error)     { return artifact.LoadAt(root, Milestone, name) }
func Committee(root string) (auditcrypto.Committee, error) { return m7run.Committee(root) }
func Millis(d time.Duration) float64                       { return float64(d.Nanoseconds()) / 1e6 }
func Write(root, name string, value any) error {
	return artifact.WriteJSON(filepath.Join(root, name), value)
}
func Read(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}

type Attempt struct {
	Name                                 string
	Number                               int
	StartedAt, FinishedAt, Status, Error string
}

func Begin(root, name, output string) (int, func(error), error) {
	if output != "" {
		if _, err := os.Stat(filepath.Join(root, output)); err == nil {
			return 0, nil, fmt.Errorf("official output exists: %s", output)
		} else if !os.IsNotExist(err) {
			return 0, nil, err
		}
	}
	paths, _ := filepath.Glob(Dir(root, "attempts", name+"-*.json"))
	a := Attempt{Name: name, Number: len(paths) + 1, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "running"}
	p := Dir(root, "attempts", fmt.Sprintf("%s-%02d.json", name, a.Number))
	if err := artifact.WriteJSON(p, a); err != nil {
		return 0, nil, err
	}
	return a.Number, func(runErr error) {
		a.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		a.Status = "success"
		if runErr != nil {
			a.Status = "failed"
			a.Error = runErr.Error()
		}
		_ = artifact.WriteJSON(p, a)
	}, nil
}

type Baseline struct {
	Algorithm string            `json:"algorithm"`
	Files     map[string]string `json:"files"`
}

func CaptureBaseline(root string) error {
	path := Dir(root, "baseline.json")
	if _, err := os.Stat(path); err == nil {
		return CheckBaseline(root)
	} else if !os.IsNotExist(err) {
		return err
	}
	b := Baseline{Algorithm: "SHA-256", Files: map[string]string{}}
	add := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		h, err := artifact.Checksum(path)
		if err != nil {
			return err
		}
		b.Files[filepath.ToSlash(rel)] = h
		return nil
	}
	for _, dir := range []string{"output", "artifacts/development/m1", "artifacts/development/m2", "artifacts/development/m3", "artifacts/development/m4", "artifacts/development/m5", "artifacts/development/m6", "artifacts/development/m6-b1", "artifacts/development/m7", "internal/audit", "internal/m7case", "internal/m7run", "features/audit_entry", "features/audit_exit", "features/audit_transfer", "features/audit_proceed", "features/audit_recall", "features/audit_merge", "features/audit_split", "features/audit_process_3_2"} {
		base := filepath.Join(root, dir)
		if err := filepath.WalkDir(base, add); err != nil {
			return err
		}
	}
	for _, pattern := range []string{"contracts/src/ZkDPPAuditLedger.sol", "contracts/src/M7*Verifier.sol", "contracts/src/AuditProcessVerifier.sol", "contracts/test/M7*.sol", "contracts/test/fixtures/m7-*.json"} {
		paths, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				return err
			}
			if err = add(p, fileEntry{info}, nil); err != nil {
				return err
			}
		}
	}
	parent := filepath.Join(root, "..", "Conversation History")
	if _, err := os.Stat(parent); err == nil {
		if err = filepath.WalkDir(parent, add); err != nil {
			return err
		}
	}
	return artifact.WriteJSON(path, b)
}

type fileEntry struct{ os.FileInfo }

func (f fileEntry) Type() os.FileMode          { return f.Mode().Type() }
func (f fileEntry) Info() (os.FileInfo, error) { return f.FileInfo, nil }

func CheckBaseline(root string) error {
	var b Baseline
	if err := Read(Dir(root, "baseline.json"), &b); err != nil {
		return err
	}
	if b.Algorithm != "SHA-256" || len(b.Files) == 0 {
		return fmt.Errorf("invalid M8 baseline")
	}
	for p, want := range b.Files {
		got, err := artifact.Checksum(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("protected file changed: %s", p)
		}
	}
	return nil
}

func GeneratedFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	for _, dir := range []string{"internal/core/dpp", "internal/core/issuepolicy", "features/m8_exit_dpp", "features/issue_claim", "internal/m8case", "internal/m8run", "internal/m8audit", "cmd/setup_m8", "cmd/evaluate_m8", "cmd/benchmark_m8", "artifacts/development/m8"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() || strings.HasSuffix(p, "m8-generated-checksums.json") {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			h, e := artifact.Checksum(p)
			if e == nil {
				files[filepath.ToSlash(rel)] = h
			}
			return e
		})
		if err != nil {
			return nil, err
		}
	}
	for _, pattern := range []string{"contracts/src/ZkDPPClaimLedger.sol", "contracts/src/M8*Verifier.sol", "contracts/src/Issue*Verifier.sol", "contracts/src/generated/m8-*/*", "contracts/test/M8*.sol", "contracts/test/fixtures/m8-*.json", "docker-compose.m8.yml", "output/m8-*.json", "milestones/M8-*.md"} {
		paths, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, p := range paths {
			if strings.HasSuffix(p, "m8-generated-checksums.json") {
				continue
			}
			rel, _ := filepath.Rel(root, p)
			h, e := artifact.Checksum(p)
			if e != nil {
				return nil, e
			}
			files[filepath.ToSlash(rel)] = h
		}
	}
	return files, nil
}
