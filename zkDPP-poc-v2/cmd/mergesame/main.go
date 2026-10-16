package main

import (
	"flag"
	"fmt"
	m "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/mergesame"
	"os"
	"runtime"
)

func main() {
	mode := flag.String("mode", "check", "setup/check/bench")
	out := flag.String("out", "", "output")
	source := flag.String("source", ".", "source root")
	old := flag.String("old-setup", "", "pinned setup")
	run := flag.Int("run", 1, "independent process number")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var e error
	switch *mode {
	case "setup":
		e = m.Prepare(*source, *old, *out)
	case "check":
		e = m.Check(*source, *out)
	case "bench":
		e = m.Bench(*source, *out, *run)
	default:
		e = fmt.Errorf("invalid mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
