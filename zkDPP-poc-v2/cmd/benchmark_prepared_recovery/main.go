package main

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"log"
)

func main() {
	if e := experiments.RunPreparedRecovery("."); e != nil {
		log.Fatal(e)
	}
}
