package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"os"
	"path/filepath"
)

func main() {
	mode := flag.String("mode", "preflight", "preflight, setup, run")
	root := flag.String("root", ".", "v2 root")
	flag.Parse()
	abs, pathErr := filepath.Abs(*root)
	if pathErr != nil {
		fmt.Fprintln(os.Stderr, pathErr)
		os.Exit(1)
	}
	*root = abs
	var err error
	switch *mode {
	case "preflight":
		err = experiments.PaperPreflight(*root)
	case "setup":
		err = experiments.PaperSetup(*root)
	case "run":
		err = experiments.PaperRun(*root)
	default:
		err = fmt.Errorf("unknown mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
