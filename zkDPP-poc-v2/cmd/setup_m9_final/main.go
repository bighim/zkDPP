package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/finalsrs"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m9run"
)

type prior struct{ ProtectedFiles, Files map[string]string }

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if e := run(*root); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(root string) (runErr error) {
	attempt, finish, e := m9run.Begin(root, "setup-final", "")
	if e != nil {
		return e
	}
	defer func() { finish(runErr) }()
	if _, e = os.Stat(filepath.Join(root, "artifacts/final/srs/universal-canonical.bin")); e == nil {
		return fmt.Errorf("final SRS already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	var p prior
	if e = read(filepath.Join(root, "output/m8-generated-checksums.json"), &p); e != nil {
		return e
	}
	baseline := map[string]string{}
	for k, v := range p.ProtectedFiles {
		baseline[k] = v
	}
	for k, v := range p.Files {
		if filepath.Ext(k) != ".md" {
			baseline[k] = v
		}
	}
	for path, want := range baseline {
		got, e := artifact.Checksum(filepath.Join(root, path))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("M1-M8 protected file changed: %s", path)
		}
	}
	if e = artifact.WriteJSON(filepath.Join(root, "artifacts/development/m9/baseline.json"), struct {
		Algorithm string
		Files     map[string]string
	}{"SHA-256", baseline}); e != nil {
		return e
	}
	result, e := finalsrs.Run(root, 1, true)
	if e != nil {
		return e
	}
	for _, row := range result.Relations {
		if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/circuits", row.Name, "manifest.json"), row); e != nil {
			return e
		}
	}
	if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/verifiers/public-input-manifest.json"), result.Relations); e != nil {
		return e
	}
	return artifact.WriteJSON(filepath.Join(root, "artifacts/development/m9/run-1.json"), struct {
		Attempt int
		Result  finalsrs.RunResult
	}{attempt, result})
}
func read(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
