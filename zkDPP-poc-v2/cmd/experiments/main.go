package main

import (
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"github.com/consensys/gnark/logger"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	logger.Disable()
	mode := flag.String("mode", "prepare", "prepare operation")
	experiment := flag.Int("experiment", 1, "experiment 1..8")
	smoke := flag.Bool("smoke", false, "development-only single warmup")
	root := flag.String("root", ".", "module root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	r, e := filepath.Abs(*root)
	if e == nil {
		switch *mode {
		case "prepare":
			e = experiments.Prepare(r)
		case "setup":
			e = experiments.Setup(r)
		case "run":
			e = experiments.Attempt(r, "run", *experiment, *smoke, func() error { return experiments.Run(r, *experiment, *smoke) })
		case "all":
			e = experiments.RunAll(r)
		case "check":
			e = experiments.Check(r)
		case "finalize":
			e = experiments.Finalize(r)
		case "report":
			e = experiments.Report(r)
		case "fixtures":
			e = experiments.ExportFoundry(r)
		case "baseline-check":
			e = experiments.CheckBaseline(r)
		default:
			e = fmt.Errorf("operation not implemented: %s", *mode)
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
