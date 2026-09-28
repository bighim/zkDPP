package main

import (
	"flag"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"log"
)

func main() {
	plain := flag.Bool("plain", false, "replay no-audit representative transaction")
	flag.Parse()
	dir := "output/paper-supplement/transfer-gas-attribution"
	if *plain {
		dir += "-plain"
	}
	if err := experiments.ReplayGasAttributionVariant(".", dir, !*plain); err != nil {
		log.Fatal(err)
	}
}
