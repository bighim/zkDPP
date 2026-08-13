package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type event struct {
	Case     string  `json:"case"`
	Event    string  `json:"event"`
	Gas      uint64  `json:"receiptGasUsed"`
	Calldata int     `json:"calldataBytes"`
	Submit   float64 `json:"submitToReceiptMs"`
	Build    float64 `json:"buildMs"`
	Witness  float64 `json:"witnessMs"`
	Prove    float64 `json:"proveMs"`
	Encode   float64 `json:"encodeMs"`
	E2E      float64 `json:"e2eMs"`
	Setup    uint64  `json:"excludedSetupGas"`
}
type deployment struct {
	Name       string `json:"name"`
	Gas        uint64 `json:"receiptGasUsed"`
	InputBytes int    `json:"inputBytes"`
}
type run struct {
	Run              int          `json:"run"`
	Backend          string       `json:"backend"`
	ChainID          uint64       `json:"chainId"`
	BlockGasLimit    uint64       `json:"blockGasLimit"`
	HashProfile      string       `json:"hashProfile"`
	Scenario         string       `json:"scenario"`
	ThreadMode       string       `json:"threadMode"`
	RequestedThreads int          `json:"requestedThreads"`
	GoMaxProcs       int          `json:"goMaxProcs"`
	NumCPU           int          `json:"numCPU"`
	Measurement      string       `json:"measurementBoundary"`
	Fixture          string       `json:"fixtureSha256"`
	Live             bool         `json:"liveProofs"`
	Deployments      []deployment `json:"deployments"`
	Events           []event      `json:"events"`
}
type stats struct {
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}
type summary struct {
	Event     string   `json:"event"`
	CaseCount int      `json:"caseCount"`
	Cases     []string `json:"cases"`
	Gas       stats    `json:"receiptGasUsed"`
	Calldata  stats    `json:"calldataBytes"`
	Submit    stats    `json:"submitToReceiptMs"`
	Build     stats    `json:"buildMs"`
	Witness   stats    `json:"witnessMs"`
	Prove     stats    `json:"proveMs"`
	Encode    stats    `json:"encodeMs"`
	E2E       stats    `json:"e2eMs"`
	Setup     stats    `json:"excludedSetupGas"`
}

func main() {
	input := flag.String("input", "benchmarks/anvil-runs/*.json", "run glob")
	expected := flag.Int("runs", 1, "must be one")
	outJSON := flag.String("out-json", "benchmarks/anvil-poseidon2-gas.json", "output JSON")
	outCSV := flag.String("out-csv", "benchmarks/anvil-poseidon2-gas.csv", "output CSV")
	flag.Parse()
	if *expected != 1 {
		panic("runs must be one")
	}
	paths, _ := filepath.Glob(*input)
	if len(paths) != 1 {
		panic(fmt.Sprintf("found %d runs, want 1", len(paths)))
	}
	var value run
	read(paths[0], &value)
	groups := map[string][]event{}
	for _, v := range value.Events {
		groups[v.Event] = append(groups[v.Event], v)
	}
	order := []string{"BaselineNoop", "BaselineStorageWrite", "Entry", "Ship", "Merge", "Split", "Process", "Exit"}
	results := make([]summary, 0, len(order))
	for _, name := range order {
		items := groups[name]
		want := 1
		if value.Scenario == "canonical" && (name == "Entry" || name == "Ship") {
			want = 2
		}
		if len(items) != want {
			panic(fmt.Sprintf("%s has %d cases, want %d", name, len(items), want))
		}
		results = append(results, summarize(name, items))
	}
	report := map[string]any{"backend": value.Backend, "chainId": value.ChainID, "blockGasLimit": value.BlockGasLimit, "hashProfile": value.HashProfile, "scenario": value.Scenario, "threadMode": value.ThreadMode, "requestedThreads": value.RequestedThreads, "goMaxProcs": value.GoMaxProcs, "numCPU": value.NumCPU, "liveProofs": value.Live, "runs": 1, "measurementBoundary": value.Measurement, "results": results}
	write(*outJSON, report)
	writeCSV(*outCSV, results)
}
func summarize(name string, values []event) summary {
	cases := make([]string, len(values))
	gas, calldata, submit, build, witness, prove, encode, e2e, setup := arrays(len(values))
	for i, v := range values {
		cases[i] = v.Case
		gas[i] = float64(v.Gas)
		calldata[i] = float64(v.Calldata)
		submit[i] = v.Submit
		build[i] = v.Build
		witness[i] = v.Witness
		prove[i] = v.Prove
		encode[i] = v.Encode
		e2e[i] = v.E2E
		setup[i] = float64(v.Setup)
	}
	return summary{Event: name, CaseCount: len(values), Cases: cases, Gas: stat(gas), Calldata: stat(calldata), Submit: stat(submit), Build: stat(build), Witness: stat(witness), Prove: stat(prove), Encode: stat(encode), E2E: stat(e2e), Setup: stat(setup)}
}
func arrays(n int) ([]float64, []float64, []float64, []float64, []float64, []float64, []float64, []float64, []float64) {
	return make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
}
func stat(v []float64) stats {
	sort.Float64s(v)
	m := v[len(v)/2]
	if len(v)%2 == 0 {
		m = (v[len(v)/2-1] + v[len(v)/2]) / 2
	}
	return stats{m, v[0], v[len(v)-1]}
}
func read(path string, target any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(raw, target); err != nil {
		panic(err)
	}
}
func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
		panic(err)
	}
}
func writeCSV(path string, values []summary) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"event", "case_count", "gas_median", "gas_min", "gas_max", "calldata_median", "build_ms_median", "witness_ms_median", "prove_ms_median", "encode_ms_median", "submit_ms_median", "e2e_ms_median"})
	for _, v := range values {
		_ = w.Write([]string{v.Event, strconv.Itoa(v.CaseCount), f64(v.Gas.Median), f64(v.Gas.Min), f64(v.Gas.Max), f64(v.Calldata.Median), f64(v.Build.Median), f64(v.Witness.Median), f64(v.Prove.Median), f64(v.Encode.Median), f64(v.Submit.Median), f64(v.E2E.Median)})
	}
	if err = w.Error(); err != nil {
		panic(err)
	}
}
func f64(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
