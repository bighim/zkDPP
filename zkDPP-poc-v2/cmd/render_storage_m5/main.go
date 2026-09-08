package main

import (
	"flag"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "root")
	flag.Parse()
	src, err := os.ReadFile(filepath.Join(*root, "contracts", "src", "generated", "process-policy-3-2", "PlonkVerifier.sol"))
	if err != nil {
		panic(err)
	}
	path := filepath.Join(*root, "contracts", "src", "generated", "storage-process", "StorageProcessVerifier.sol")
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err = solgen.WriteStorageProcessVerifier(f, string(src)); err != nil {
		_ = f.Close()
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
}
