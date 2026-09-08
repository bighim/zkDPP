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
	result, err := v2run.Evaluate(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("M1 evaluate: %d proofs verified\n", len(result.Relations))
}
