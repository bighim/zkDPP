package experiments

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileRoundTrip(t *testing.T) {
	for _, p := range Profiles() {
		b, e := json.Marshal(p)
		if e != nil {
			t.Fatal(e)
		}
		var q Profile
		if e = json.Unmarshal(b, &q); e != nil {
			t.Fatal(e)
		}
		if p.Name != q.Name || !p.PolicyRef.Equal(&q.PolicyRef) {
			t.Fatal("profile mismatch")
		}
	}
}
func TestRejectIncompleteMeasurement(t *testing.T) {
	root := t.TempDir()
	if e := writeJSON(filepath.Join(root, OutputDir, "experiment-1.json"), map[string]any{"experiment": 1, "status": "complete", "warmups": 1, "repetitions": 5, "rows": []Measurement{}}); e != nil {
		t.Fatal(e)
	}
	if ValidateExperiment(root, 1) == nil {
		t.Fatal("empty results accepted")
	}
}
func TestCheckIsReadOnlyOnTamper(t *testing.T) {
	root := t.TempDir()
	if e := writeJSON(filepath.Join(root, ArtifactDir, "baseline.json"), Baseline{Files: map[string]string{}, PrefixFiles: map[string]string{}}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "sentinel")
	if e := os.WriteFile(path, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	sum, e := checksum(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = writeJSON(filepath.Join(root, OutputDir, "checksums.json"), map[string]string{"sentinel": sum}); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(root, OutputDir, "checksums.json"))
	if e != nil {
		t.Fatal(e)
	}
	if Check(root) == nil {
		t.Fatal("tamper accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, OutputDir, "checksums.json"))
	content, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || string(content) != "tampered" {
		t.Fatal("checker changed files")
	}
}
