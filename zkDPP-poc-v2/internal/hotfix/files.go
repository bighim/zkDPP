package hotfix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Dir = "artifacts/development/m1-hotfix"

func Write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func Read(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func Hash(path string) (string, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if info.Mode()&os.ModeSymlink != 0 {
		v, e := os.Readlink(path)
		if e != nil {
			return "", e
		}
		h := sha256.Sum256([]byte(v))
		return hex.EncodeToString(h[:]), nil
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type Inventory struct {
	Files   map[string]string
	Missing []string
}

func Baseline(root string) error {
	path := filepath.Join(root, Dir, "baseline.json")
	if _, e := os.Stat(path); e == nil {
		return VerifyInventory(root, path)
	}
	inv := Inventory{Files: map[string]string{}}
	for _, rel := range []string{"../zkDPP-poc-v1/output", "../zkDPP-poc-v1/artifacts", "../zkDPP-poc-v1/milestones", "../Conversation History", "artifacts/development/m1", "contracts/src/generated/v2-m1", "contracts/test/fixtures/v2-m1-proofs.json", "milestones/M1-master-key-audit-result.md"} {
		e := filepath.WalkDir(filepath.Join(root, rel), func(p string, d os.DirEntry, e error) error {
			if os.IsNotExist(e) {
				inv.Missing = append(inv.Missing, rel)
				return nil
			}
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			r, _ := filepath.Rel(root, p)
			h, e := Hash(p)
			inv.Files[r] = h
			return e
		})
		if e != nil {
			return e
		}
	}
	files, e := filepath.Glob(filepath.Join(root, "output/m1-*.json"))
	if e != nil {
		return e
	}
	for _, p := range files {
		r, _ := filepath.Rel(root, p)
		h, e := Hash(p)
		if e != nil {
			return e
		}
		inv.Files[r] = h
	}
	return Write(path, inv)
}
func VerifyInventory(root, path string) error {
	var inv Inventory
	if e := Read(path, &inv); e != nil {
		return e
	}
	for p, want := range inv.Files {
		got, e := Hash(filepath.Join(root, p))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("checksum mismatch: %s", p)
		}
	}
	return nil
}

type Attempt struct {
	Mode              string
	Started, Finished time.Time
	Error             string
	Success           bool
}

func AttemptRun(root, mode string, fn func() error) (err error) {
	dir := filepath.Join(root, Dir, "attempts")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	a := Attempt{Mode: mode, Started: time.Now().UTC()}
	p := filepath.Join(dir, mode+"-"+a.Started.Format("20060102T150405.000000000")+".json")
	if e := Write(p, a); e != nil {
		return e
	}
	defer func() {
		a.Finished = time.Now().UTC()
		a.Success = err == nil
		if err != nil {
			a.Error = err.Error()
		}
		_ = Write(p, a)
	}()
	return fn()
}
func Finalize(root string) error {
	target := filepath.Join(root, "output/m1-hotfix-generated-checksums.json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("final manifest already exists")
	}
	if e := VerifyInventory(root, filepath.Join(root, Dir, "baseline.json")); e != nil {
		return e
	}
	if e := ValidateReports(root); e != nil {
		return e
	}
	var report map[string]any
	if e := Read(filepath.Join(root, "output/m1-hotfix-circuit.json"), &report); e != nil {
		return e
	}
	if e := Write(filepath.Join(root, Dir, "manifests/public-inputs.json"), report["relations"]); e != nil {
		return e
	}
	inv := Inventory{Files: map[string]string{}}
	for _, rel := range []string{Dir, "features", "internal", "cmd", "testdata/common/actors-v2.json", "contracts/src/ZkDPPV2Ledger.sol", "contracts/src/generated/v2-m1-hotfix", "contracts/hotfix", "contracts/test/fixtures/v2-m1-hotfix-proofs.json", "milestones/M1-HF-spec-conformance-result.md", "milestones/M1-HF-spec-conformance.md", "README.md", "ARCHITECTURE.md", "REUSE.md", "MILESTONES.md", "AGENTS.md", "Makefile", "go.mod", "go.sum", "docker-compose.hotfix.yml"} {
		e := filepath.WalkDir(filepath.Join(root, rel), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			r, _ := filepath.Rel(root, p)
			if d.IsDir() {
				if strings.Contains(r, "/out") || strings.Contains(r, "/cache") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.Contains(r, "/key-package/") && strings.Contains(r, "share") {
				return nil
			}
			h, e := Hash(p)
			inv.Files[r] = h
			return e
		})
		if e != nil {
			return e
		}
	}
	for _, name := range []string{"circuit", "key-recovery", "anvil", "audit"} {
		p := "output/m1-hotfix-" + name + ".json"
		h, e := Hash(filepath.Join(root, p))
		if e != nil {
			return e
		}
		inv.Files[p] = h
	}
	return Write(target, inv)
}
func Check(root string) error {
	if e := VerifyInventory(root, filepath.Join(root, Dir, "baseline.json")); e != nil {
		return e
	}
	path := filepath.Join(root, "output/m1-hotfix-generated-checksums.json")
	if e := VerifyInventory(root, path); e != nil {
		return e
	}
	if e := ValidateReports(root); e != nil {
		return e
	}
	return VerifyInventory(root, path)
}
