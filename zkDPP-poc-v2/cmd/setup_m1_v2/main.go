package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2run"
)

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	result, err := v2run.Setup(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("M1 setup: %d relations, max domain %d, universal points %d\n", len(result.Relations), result.MaxDomain, result.UniversalPoints)
}
