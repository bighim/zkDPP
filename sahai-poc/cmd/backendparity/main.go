package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type stats struct {
	Median float64 `json:"median"`
}
type item struct {
	Event     string `json:"event"`
	CaseCount int    `json:"caseCount"`
	Gas       stats  `json:"receiptGasUsed"`
	Calldata  stats  `json:"calldataBytes"`
}
type report struct {
	Backend     string `json:"backend"`
	HashProfile string `json:"hashProfile"`
	Scenario    string `json:"scenario"`
	Results     []item `json:"results"`
}
type parity struct {
	Event         string  `json:"event"`
	CaseCount     int     `json:"caseCount"`
	AnvilGas      float64 `json:"anvilGas"`
	BesuGas       float64 `json:"besuGas"`
	GasEqual      bool    `json:"gasEqual"`
	CalldataEqual bool    `json:"calldataEqual"`
}

func main() {
	anvil := flag.String("anvil", "benchmarks/anvil-poseidon2-gas.json", "Anvil JSON")
	besu := flag.String("besu", "benchmarks/besu-poseidon2-gas.json", "Besu JSON")
	out := flag.String("out", "benchmarks/backend-parity.json", "output")
	flag.Parse()
	var a, b report
	read(*anvil, &a)
	read(*besu, &b)
	if a.HashProfile != b.HashProfile || a.Scenario != b.Scenario {
		panic("profile/scenario mismatch")
	}
	passed := true
	values := []parity{}
	for _, left := range a.Results {
		right, ok := find(b.Results, left.Event)
		if !ok {
			panic("missing " + left.Event)
		}
		v := parity{left.Event, left.CaseCount, left.Gas.Median, right.Gas.Median, left.Gas.Median == right.Gas.Median, left.Calldata.Median == right.Calldata.Median}
		passed = passed && v.GasEqual && v.CalldataEqual
		values = append(values, v)
	}
	write(*out, map[string]any{"hashProfile": a.HashProfile, "scenario": a.Scenario, "definition": "identical fixed proof, connected state, calldata, chain ID and Prague revision", "passed": passed, "results": values})
	if !passed {
		panic("Anvil/Besu parity failed")
	}
}
func find(v []item, name string) (item, bool) {
	for _, x := range v {
		if x.Event == name {
			return x, true
		}
	}
	return item{}, false
}
func read(p string, v any) {
	r, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	if e = json.Unmarshal(r, v); e != nil {
		panic(e)
	}
}
func write(p string, v any) {
	r, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(p, append(r, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Println(p)
}
