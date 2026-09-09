package hotfix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyChecksumAndTamper(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"verifier.sol", "public-input.json", "raw.json"} {
		p := filepath.Join(dir, name)
		if e := os.WriteFile(p, []byte("original"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	inv := Inventory{Files: map[string]string{}}
	for _, n := range []string{"verifier.sol", "public-input.json", "raw.json"} {
		inv.Files[n], _ = Hash(filepath.Join(dir, n))
	}
	path := filepath.Join(dir, "manifest.json")
	if e := Write(path, inv); e != nil {
		t.Fatal(e)
	}
	before, _ := Hash(path)
	if e := VerifyInventory(dir, path); e != nil {
		t.Fatal(e)
	}
	for n := range inv.Files {
		p := filepath.Join(dir, n)
		if e := os.WriteFile(p, []byte("tampered"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := VerifyInventory(dir, path); e == nil {
			t.Fatalf("accepted mutation %s", n)
		}
		raw, _ := os.ReadFile(p)
		if string(raw) != "tampered" {
			t.Fatal("checker rewrote data")
		}
		os.WriteFile(p, []byte("original"), 0600)
	}
	after, _ := Hash(path)
	if before != after {
		t.Fatal("checker rewrote manifest")
	}
}

func TestUnavailableBackendCannotProduceMeasurement(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if e := os.MkdirAll(bin, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin)
	if e := RunEVM(root, "anvil"); e == nil {
		t.Fatal("unavailable backend accepted")
	}
	if _, e := os.Stat(filepath.Join(root, "output/m1-hotfix-anvil.json")); !os.IsNotExist(e) {
		t.Fatal("measurement written without backend")
	}
}
