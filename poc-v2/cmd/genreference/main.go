package main

import (
	"fmt"
	"path/filepath"

	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
)

func main() {
	d, err := scenario.LoadCanonical(".")
	if err != nil {
		panic(err)
	}
	path := filepath.Join("testdata", "reference", "canonical-derived.json")
	if err := scenario.WriteReference(path, d.Reference()); err != nil {
		panic(err)
	}
	fmt.Println(path)
}
