package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bighim/zkDPP/sahai-poc/internal/conformance"
)

func main() {
	manifest, err := conformance.Load("config/events.json")
	if err != nil {
		fatal(err)
	}
	if err := conformance.Validate(manifest); err != nil {
		fatal(err)
	}
	doc, err := os.ReadFile("docs/conformance.md")
	if err != nil {
		fatal(err)
	}
	for _, tag := range []string{"[Paper]", "[Adaptation]", "[Experiment]"} {
		if !strings.Contains(string(doc), tag) {
			fatal(fmt.Errorf("missing source-separation tag %s", tag))
		}
	}
	fmt.Printf("M0 PASS: %d events, %d attributes, %d leaves, %d siblings\n",
		len(manifest.Events), manifest.Merkle.Attributes, manifest.Merkle.Leaves, manifest.Merkle.Siblings)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "M0 FAIL:", err)
	os.Exit(1)
}
