package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/hotfix"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	mode := flag.String("mode", "proofs", "setup, proofs, evm")
	root := flag.String("root", ".", "source checkout root")
	out := flag.String("out", "", "unique execution directory")
	url := flag.String("rpc", "http://127.0.0.1:18547", "local Anvil RPC")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	r, e := filepath.Abs(*root)
	if e != nil {
		panic(e)
	}
	o, e := filepath.Abs(*out)
	if e != nil {
		panic(e)
	}
	if *out == "" {
		panic("-out required")
	}
	switch *mode {
	case "setup":
		e = hotfix.IntegrationSetup(r, o)
	case "proofs":
		e = hotfix.IntegrationProofs(r, o)
	case "evm":
		e = hotfix.IntegrationEVM(r, o, *url)
	case "witness-check":
		e = hotfix.IntegrationWitnessEvidence(r, o)
	default:
		e = fmt.Errorf("unknown mode: %s", *mode)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
