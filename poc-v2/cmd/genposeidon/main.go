package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bighim/zkDPP/poc-v2/internal/solgen"
)

func main() {
	path := filepath.Join("contracts", "src", "generated", "Poseidon2BLS12381.sol")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err := solgen.WritePoseidon2BLS12381(file); err != nil {
		_ = file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
	fmt.Println(path)
}
