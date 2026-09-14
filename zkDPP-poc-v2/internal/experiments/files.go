// Package experiments owns the isolated paper-evaluation workspace.
package experiments

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

const ArtifactDir = "artifacts/development/experiments"
const OutputDir = "output/experiments"
const Warmups = 1
const Repetitions = 5

func checksum(path string) (string, error) {
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
func writeJSON(path string, value any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func readJSON(path string, value any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, value)
}
func ms(t time.Duration) float64 { return float64(t.Nanoseconds()) / 1e6 }

type Baseline struct {
	Files       map[string]string `json:"files"`
	PrefixFiles map[string]string `json:"prefixFiles"`
	Version     string            `json:"version"`
}

func Prepare(root string) error {
	path := filepath.Join(root, ArtifactDir, "baseline.json")
	if _, e := os.Stat(path); e == nil {
		return CheckBaseline(root)
	}
	b := Baseline{Files: map[string]string{}, PrefixFiles: map[string]string{}, Version: "experiments-v1"}
	parent := filepath.Dir(root)
	for _, dir := range []string{"zkDPP-poc-v1", "zkDPP-poc-v2", "Conversation History"} {
		e := filepath.WalkDir(filepath.Join(parent, dir), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(parent, p)
			s := filepath.ToSlash(rel)
			if d.IsDir() {
				if strings.Contains(s, "/.git") || strings.Contains(s, "/.cache") || strings.Contains(s, "/.codex-build") || strings.Contains(s, "/contracts/out") || strings.Contains(s, "/contracts/cache") || strings.Contains(s, "/contracts/broadcast") || strings.Contains(s, "/experiments") || strings.Contains(s, "/docs/visual") {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.Name() == ".DS_Store" {
				return nil
			}
			if strings.HasSuffix(s, "zkDPP-poc-v2/Makefile") || strings.HasSuffix(s, "zkDPP-poc-v2/.gitignore") {
				raw, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				b.PrefixFiles[s] = string(raw)
				return nil
			}
			h, e := checksum(p)
			if e != nil {
				return e
			}
			b.Files[s] = h
			return nil
		})
		if e != nil {
			return e
		}
	}
	if e := writeJSON(path, b); e != nil {
		return e
	}
	return writeJSON(filepath.Join(root, ArtifactDir, "config.json"), map[string]any{"warmups": Warmups, "repetitions": Repetitions, "statistic": "median", "range": "min-max", "port": 18547, "chainId": 31337, "gasLimit": 30000000, "developmentOnly": true, "processArities": [][2]int{{2, 1}, {2, 2}, {3, 2}, {3, 3}, {4, 3}}, "stateCounts": []int{1, 2, 3, 4, 8, 16}, "traceRecords": []int{10, 100, 1000}, "decryptCounts": []int{1, 10, 100, 1000}, "fieldCounts": []int{1, 2, 3, 5, 10, 20}})
}
func CheckBaseline(root string) error {
	var b Baseline
	if e := readJSON(filepath.Join(root, ArtifactDir, "baseline.json"), &b); e != nil {
		return e
	}
	for p, want := range b.Files {
		got, e := checksum(filepath.Join(filepath.Dir(root), p))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("protected file changed: %s", p)
		}
	}
	for p, want := range b.PrefixFiles {
		got, e := os.ReadFile(filepath.Join(filepath.Dir(root), p))
		if e != nil {
			return e
		}
		if !strings.HasPrefix(string(got), want) {
			return fmt.Errorf("protected prefix changed: %s", p)
		}
	}
	return nil
}
