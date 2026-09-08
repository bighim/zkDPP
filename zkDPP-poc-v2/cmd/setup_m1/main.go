package main

import (
	"flag"
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m1case"
)

func main() {
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if err := run(*root); err != nil {
		panic(err)
	}
}

func run(root string) error {
	cases, err := m1case.All(root)
	if err != nil {
		return err
	}
	for _, item := range cases {
		manifest, err := artifact.Setup(root, item)
		if err != nil {
			return err
		}
		fmt.Printf("%-16s constraints=%d public=%d compile=%dms setup=%dms\n", item.Name, manifest.Constraints, manifest.PublicInputs, manifest.CompileMillis, manifest.SetupMillis)
	}
	return nil
}
