package proofruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.bin")
	if err := os.WriteFile(path, []byte("zkDPP"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := checksum(path)
	if err != nil {
		t.Fatal(err)
	}
	const want = "7828a2b920f771040138a73622e04c131ce91113ea0daacb2457c5c8a2632d85"
	if got != want {
		t.Fatalf("checksum=%s, want %s", got, want)
	}
}

func TestLoadRejectsMissingManifest(t *testing.T) {
	if _, err := Load(t.TempDir(), "entry"); err == nil {
		t.Fatal("expected missing manifest error")
	}
}
