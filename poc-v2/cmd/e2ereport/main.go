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
	"time"
)

type machineMetadata struct {
	OS        string `json:"os"`
	OSVersion string `json:"osVersion,omitempty"`
	Arch      string `json:"arch"`
	CPU       string `json:"cpu,omitempty"`
	CPUCores  int    `json:"cpuCores"`
	RAMBytes  string `json:"ramBytes,omitempty"`
	GoVersion string `json:"goVersion"`
}

type loadResult struct {
	Relation string  `json:"relation"`
	LoadMS   float64 `json:"artifactLoadMs"`
}

type callResult struct {
	Sequence          int     `json:"sequence"`
	Event             string  `json:"event"`
	Case              string  `json:"case"`
	Actor             string  `json:"actor"`
	WitnessMS         float64 `json:"witnessMs"`
	ProveMS           float64 `json:"proveMs"`
	TransactionPrepMS float64 `json:"txPrepareMs"`
	SubmitReceiptMS   float64 `json:"submitToReceiptMs"`
	E2EMS             float64 `json:"e2eMs"`
	TransactionHash   string  `json:"transactionHash"`
	BlockNumber       uint64  `json:"blockNumber"`
	ReceiptGasUsed    uint64  `json:"receiptGasUsed"`
	CalldataBytes     int     `json:"calldataBytes"`
	ZeroBytes         int     `json:"zeroBytes"`
	NonZeroBytes      int     `json:"nonZeroBytes"`
}

type runResult struct {
	Run           int             `json:"run"`
	GeneratedAt   string          `json:"generatedAt"`
	Measurement   string          `json:"measurementType"`
	Backend       string          `json:"backend"`
	RPC           string          `json:"rpc"`
	ChainID       uint64          `json:"chainId"`
	Machine       machineMetadata `json:"machine"`
	Contract      string          `json:"contract"`
	ArtifactLoads []loadResult    `json:"artifactLoads"`
	Calls         []callResult    `json:"calls"`
	MTAppendCount int             `json:"mtAppendCount"`
	RVAppendCount int             `json:"rvMTAppendCount"`
	MTLeafCount   uint64          `json:"mtLeafCount"`
	RVMTLeafCount uint64          `json:"rvMTLeafCount"`
}

type statistics struct {
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type loadSummary struct {
	Relation string     `json:"relation"`
	LoadMS   statistics `json:"artifactLoadMs"`
}

type callSummary struct {
	Sequence          int        `json:"sequence"`
	Event             string     `json:"event"`
	Case              string     `json:"case"`
	Actor             string     `json:"actor"`
	WitnessMS         statistics `json:"witnessMs"`
	ProveMS           statistics `json:"proveMs"`
	TransactionPrepMS statistics `json:"txPrepareMs"`
	SubmitReceiptMS   statistics `json:"submitToReceiptMs"`
	E2EMS             statistics `json:"e2eMs"`
	ReceiptGasUsed    statistics `json:"receiptGasUsed"`
	CalldataBytes     statistics `json:"calldataBytes"`
}

type softwareMetadata struct {
	Gnark       string `json:"gnark"`
	GnarkCrypto string `json:"gnarkCrypto"`
	GoEthereum  string `json:"goEthereum"`
	Foundry     string `json:"foundry"`
	Anvil       string `json:"anvil,omitempty"`
	Besu        string `json:"besu,omitempty"`
	Solidity    string `json:"solidity"`
	ProofSystem string `json:"proofSystem"`
	Curve       string `json:"curve"`
}

type chainMetadata struct {
	ChainID             uint64 `json:"chainId"`
	Hardfork            string `json:"hardfork"`
	GenesisTimestamp    uint64 `json:"genesisTimestamp"`
	BlockGasLimit       uint64 `json:"blockGasLimit"`
	TransactionGasLimit uint64 `json:"transactionGasLimit"`
}

type report struct {
	GeneratedAt         string           `json:"generatedAt"`
	Backend             string           `json:"backend"`
	MeasurementType     string           `json:"measurementType"`
	MeasurementBoundary string           `json:"measurementBoundary"`
	Runs                int              `json:"runs"`
	Representative      string           `json:"representative"`
	Machine             machineMetadata  `json:"machine"`
	Software            softwareMetadata `json:"software"`
	Chain               chainMetadata    `json:"chain"`
	ArtifactLoadSummary []loadSummary    `json:"artifactLoadSummary"`
	EventSummary        []callSummary    `json:"eventSummary"`
	RawRuns             []runResult      `json:"rawRuns"`
}

func main() {
	input := flag.String("input", "benchmarks/e2e-runs/run-*.json", "input glob")
	expectedRuns := flag.Int("runs", 1, "expected run count")
	outJSON := flag.String("out-json", "benchmarks/anvil-e2e-time.json", "output JSON")
	outCSV := flag.String("out-csv", "benchmarks/anvil-e2e-time.csv", "output CSV")
	flag.Parse()
	if err := run(*input, *expectedRuns, *outJSON, *outCSV); err != nil {
		panic(err)
	}
}

func run(input string, expectedRuns int, outJSON, outCSV string) error {
	paths, err := filepath.Glob(input)
	if err != nil {
		return err
	}
	if len(paths) != expectedRuns {
		return fmt.Errorf("expected %d E2E runs, found %d", expectedRuns, len(paths))
	}
	runs := make([]runResult, 0, len(paths))
	seenRuns := map[int]bool{}
	for _, path := range paths {
		var item runResult
		if err := readJSON(path, &item); err != nil {
			return err
		}
		if seenRuns[item.Run] {
			return fmt.Errorf("duplicate run number %d", item.Run)
		}
		seenRuns[item.Run] = true
		if len(item.Calls) != 25 || item.MTAppendCount != 28 || item.RVAppendCount != 7 || item.MTLeafCount != 28 || item.RVMTLeafCount != 7 {
			return fmt.Errorf("run %d has invalid canonical final state", item.Run)
		}
		runs = append(runs, item)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Run < runs[j].Run })
	backend := runs[0].Backend
	if backend == "" {
		backend = "anvil"
	}
	chainID := runs[0].ChainID
	if chainID == 0 {
		chainID = 31337
	}
	for _, item := range runs[1:] {
		itemBackend := item.Backend
		if itemBackend == "" {
			itemBackend = "anvil"
		}
		itemChainID := item.ChainID
		if itemChainID == 0 {
			itemChainID = 31337
		}
		if itemBackend != backend || itemChainID != chainID {
			return fmt.Errorf("run backend or chain ID mismatch")
		}
	}
	if err := validateShape(runs); err != nil {
		return err
	}

	result := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Backend: backend, MeasurementType: "live-proof-evm-e2e",
		MeasurementBoundary: "witness construction start through successful EVM transaction receipt; compile, setup, deployment, and artifact loading excluded",
		Runs:                len(runs), Representative: "median", Machine: runs[0].Machine,
		Software: softwareMetadata{
			Gnark: "v0.15.0", GnarkCrypto: "v0.20.1", GoEthereum: "v1.17.4",
			Foundry: "v1.7.1", Anvil: "v1.7.1", Solidity: "0.8.30",
			ProofSystem: "PLONK-KZG", Curve: "BLS12-381",
		},
		Chain: chainMetadata{
			ChainID: chainID, Hardfork: "prague", GenesisTimestamp: 60000,
			BlockGasLimit: 30000000, TransactionGasLimit: 29000000,
		},
		RawRuns: runs,
	}
	if backend == "besu" {
		result.Software.Anvil = ""
		result.Software.Besu = "v26.7.1"
	}
	for loadIndex, load := range runs[0].ArtifactLoads {
		values := make([]float64, len(runs))
		for runIndex := range runs {
			values[runIndex] = runs[runIndex].ArtifactLoads[loadIndex].LoadMS
		}
		result.ArtifactLoadSummary = append(result.ArtifactLoadSummary, loadSummary{Relation: load.Relation, LoadMS: stats(values)})
	}
	for callIndex, call := range runs[0].Calls {
		witness, prove, prepare, receipt, e2e, gas, calldata := make([]float64, len(runs)), make([]float64, len(runs)), make([]float64, len(runs)), make([]float64, len(runs)), make([]float64, len(runs)), make([]float64, len(runs)), make([]float64, len(runs))
		for runIndex := range runs {
			item := runs[runIndex].Calls[callIndex]
			witness[runIndex], prove[runIndex], prepare[runIndex] = item.WitnessMS, item.ProveMS, item.TransactionPrepMS
			receipt[runIndex], e2e[runIndex] = item.SubmitReceiptMS, item.E2EMS
			gas[runIndex], calldata[runIndex] = float64(item.ReceiptGasUsed), float64(item.CalldataBytes)
		}
		result.EventSummary = append(result.EventSummary, callSummary{
			Sequence: call.Sequence, Event: call.Event, Case: call.Case, Actor: call.Actor,
			WitnessMS: stats(witness), ProveMS: stats(prove), TransactionPrepMS: stats(prepare),
			SubmitReceiptMS: stats(receipt), E2EMS: stats(e2e), ReceiptGasUsed: stats(gas), CalldataBytes: stats(calldata),
		})
	}
	if err := writeJSON(outJSON, result); err != nil {
		return err
	}
	return writeCSV(outCSV, result)
}

func validateShape(runs []runResult) error {
	for runIndex := 1; runIndex < len(runs); runIndex++ {
		if len(runs[runIndex].ArtifactLoads) != len(runs[0].ArtifactLoads) || len(runs[runIndex].Calls) != len(runs[0].Calls) {
			return fmt.Errorf("run %d shape differs from run %d", runs[runIndex].Run, runs[0].Run)
		}
		for i := range runs[0].ArtifactLoads {
			if runs[runIndex].ArtifactLoads[i].Relation != runs[0].ArtifactLoads[i].Relation {
				return fmt.Errorf("run %d artifact order differs", runs[runIndex].Run)
			}
		}
		for i := range runs[0].Calls {
			want, got := runs[0].Calls[i], runs[runIndex].Calls[i]
			if got.Sequence != want.Sequence || got.Event != want.Event || got.Case != want.Case || got.Actor != want.Actor {
				return fmt.Errorf("run %d call %d differs from canonical order", runs[runIndex].Run, i+1)
			}
		}
	}
	return nil
}

func stats(values []float64) statistics {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	median := ordered[len(ordered)/2]
	if len(ordered)%2 == 0 {
		median = (ordered[len(ordered)/2-1] + ordered[len(ordered)/2]) / 2
	}
	return statistics{Median: median, Min: ordered[0], Max: ordered[len(ordered)-1]}
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeCSV(path string, result report) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	header := []string{"row_type", "run", "sequence", "event", "case", "actor", "metric", "value_ms", "median_ms", "min_ms", "max_ms", "receipt_gas", "calldata_bytes", "transaction_hash"}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, run := range result.RawRuns {
		for _, call := range run.Calls {
			if err := writer.Write([]string{"raw", strconv.Itoa(run.Run), strconv.Itoa(call.Sequence), call.Event, call.Case, call.Actor, "e2e", f64(call.E2EMS), "", "", "", strconv.FormatUint(call.ReceiptGasUsed, 10), strconv.Itoa(call.CalldataBytes), call.TransactionHash}); err != nil {
				return err
			}
		}
	}
	for _, call := range result.EventSummary {
		for _, metric := range []struct {
			name string
			data statistics
		}{{"witness", call.WitnessMS}, {"prove", call.ProveMS}, {"tx_prepare", call.TransactionPrepMS}, {"submit_to_receipt", call.SubmitReceiptMS}, {"e2e", call.E2EMS}} {
			if err := writer.Write([]string{"summary", "", strconv.Itoa(call.Sequence), call.Event, call.Case, call.Actor, metric.name, "", f64(metric.data.Median), f64(metric.data.Min), f64(metric.data.Max), "", "", ""}); err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return writer.Error()
}

func f64(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }
