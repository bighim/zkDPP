package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"os"
)

func main() {
	mode := flag.String("mode", "setup", "setup or run")
	flag.Parse()
	var e error
	if *mode == "setup" {
		e = experiments.SharedSetup(".")
	} else if *mode == "run" {
		e = experiments.SharedRun(".")
	} else {
		e = fmt.Errorf("unknown mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
