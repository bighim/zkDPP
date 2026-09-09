package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/hotfix"
	"os"
	"runtime"
)

func main() {
	mode := flag.String("mode", "baseline", "operation")
	root := flag.String("root", ".", "root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var e error
	switch *mode {
	case "baseline":
		e = hotfix.Baseline(*root)
	case "prepare":
		e = hotfix.Prepare(*root)
	case "setup":
		e = hotfix.AttemptRun(*root, *mode, func() error { return hotfix.Setup(*root) })
	case "evaluate":
		e = hotfix.AttemptRun(*root, *mode, func() error { return hotfix.Evaluate(*root) })
	case "anvil", "audit":
		e = hotfix.AttemptRun(*root, *mode, func() error { return hotfix.RunEVM(*root, *mode) })
	case "check":
		e = hotfix.Check(*root)
	case "validate":
		e = hotfix.ValidateReports(*root)
	case "finalize":
		e = hotfix.Finalize(*root)
	case "key-recovery":
		e = hotfix.AttemptRun(*root, *mode, func() error { return hotfix.KeyBench(*root) })
	default:
		e = fmt.Errorf("unknown mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
