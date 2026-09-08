package m9run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m8run"
)

const FoundryImage = m8run.FoundryImage

func Committee(root string) (auditcrypto.Committee, error) { return m8run.Committee(root) }
func Write(root, name string, v any) error                 { return artifact.WriteJSON(filepath.Join(root, name), v) }
func Read(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func Millis(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

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
	dir := filepath.Join(root, "artifacts/development/m9/attempts")
	paths, _ := filepath.Glob(filepath.Join(dir, name+"-*.json"))
	a := Attempt{Name: name, Number: len(paths) + 1, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "running"}
	p := filepath.Join(dir, fmt.Sprintf("%s-%02d.json", name, a.Number))
	if e := artifact.WriteJSON(p, a); e != nil {
		return 0, nil, e
	}
	return a.Number, func(err error) {
		a.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		a.Status = "success"
		if err != nil {
			a.Status = "failed"
			a.Error = err.Error()
		}
		_ = artifact.WriteJSON(p, a)
	}, nil
}
