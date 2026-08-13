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
	"strings"
)

type stats struct {
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type sahaiItem struct {
	Event     string `json:"event"`
	CaseCount int    `json:"caseCount"`
	Gas       stats  `json:"receiptGasUsed"`
	E2E       stats  `json:"e2eMs"`
}

type sahaiReport struct {
	Backend     string      `json:"backend"`
	HashProfile string      `json:"hashProfile"`
	ThreadMode  string      `json:"threadMode"`
	Results     []sahaiItem `json:"results"`
}

type zkdppGasItem struct {
	Event string  `json:"event"`
	Case  string  `json:"case"`
	Gas   float64 `json:"receiptGasUsed"`
}

type zkdppGasReport struct {
	HashProfile string         `json:"hashProfile"`
	ThreadMode  string         `json:"threadMode"`
	Calls       []zkdppGasItem `json:"calls"`
}

type zkdppE2EItem struct {
	Event string `json:"event"`
	Case  string `json:"case"`
	E2E   stats  `json:"e2eMs"`
}

type zkdppE2EReport struct {
	HashProfile  string         `json:"hashProfile"`
	ThreadMode   string         `json:"threadMode"`
	EventSummary []zkdppE2EItem `json:"eventSummary"`
}

type comparison struct {
	SahaiEvent     string  `json:"sahaiEvent"`
	ZkDPPRelation  string  `json:"zkdppClosestRelation"`
	SahaiCaseCount int     `json:"sahaiCaseCount"`
	ZkDPPCaseCount int     `json:"zkdppCaseCount"`
	SahaiGas       float64 `json:"sahaiGasMedian"`
	ZkDPPGas       float64 `json:"zkdppGasMedianAcrossCases"`
	GasRatio       float64 `json:"sahaiOverZkdppGas"`
	SahaiE2EMS     float64 `json:"sahaiE2EMsMedian"`
	ZkDPPE2EMS     float64 `json:"zkdppE2EMsMedianAcrossCases"`
	E2ERatio       float64 `json:"sahaiOverZkdppE2E"`
	Semantics      string  `json:"semanticsBoundary"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	flag.Parse()
	bench := filepath.Join(*root, "benchmarks")

	var sahaiGas, sahaiE2E sahaiReport
	var zkdppGas zkdppGasReport
	var zkdppE2E zkdppE2EReport
	read(filepath.Join(bench, "anvil-poseidon2-gas.json"), &sahaiGas)
	read(filepath.Join(bench, "anvil-poseidon2-mt-e2e.json"), &sahaiE2E)
	read(filepath.Join(bench, "zkdpp-poc", "anvil-mt-gas.json"), &zkdppGas)
	read(filepath.Join(bench, "zkdpp-poc", "anvil-mt-e2e.json"), &zkdppE2E)
	requireProfile("Sahai gas", sahaiGas.HashProfile, sahaiGas.ThreadMode)
	requireProfile("Sahai E2E", sahaiE2E.HashProfile, sahaiE2E.ThreadMode)
	requireProfile("zkDPP gas", zkdppGas.HashProfile, zkdppGas.ThreadMode)
	requireProfile("zkDPP E2E", zkdppE2E.HashProfile, zkdppE2E.ThreadMode)

	mapping := map[string]string{"Entry": "entry", "Ship": "transfer", "Merge": "merge", "Split": "split", "Process": "process", "Exit": "exit"}
	semantics := map[string]string{
		"Entry":   "가장 가까운 기능 대응이며 commitment와 공개 입력은 서로 다름",
		"Ship":    "Sahai Ship을 zkDPP Transfer에 대응하며 소유권 및 hidden-state semantics는 다름",
		"Merge":   "두 input을 한 output으로 결합하지만 predicate와 provenance 표현은 다름",
		"Split":   "한 input을 두 output으로 나누지만 output commitment semantics는 다름",
		"Process": "두 input을 제품 output으로 바꾸지만 relation predicate 구성은 다름",
		"Exit":    "terminal 처리의 가장 가까운 대응이며 Sahai는 terminal output document를 생성함",
	}

	gasGroups := map[string][]float64{}
	for _, item := range zkdppGas.Calls {
		gasGroups[strings.ToLower(item.Event)] = append(gasGroups[strings.ToLower(item.Event)], item.Gas)
	}
	e2eGroups := map[string][]float64{}
	for _, item := range zkdppE2E.EventSummary {
		e2eGroups[strings.ToLower(item.Event)] = append(e2eGroups[strings.ToLower(item.Event)], item.E2E.Median)
	}

	results := make([]comparison, 0, 6)
	for _, event := range []string{"Entry", "Ship", "Merge", "Split", "Process", "Exit"} {
		relation := mapping[event]
		sg := findSahai(sahaiGas.Results, event)
		se := findSahai(sahaiE2E.Results, event)
		zg, ze := gasGroups[relation], e2eGroups[relation]
		if len(zg) == 0 || len(ze) == 0 || len(zg) != len(ze) {
			panic(fmt.Sprintf("invalid zkDPP cases for %s: gas=%d e2e=%d", relation, len(zg), len(ze)))
		}
		zkGas, zkE2E := median(zg), median(ze)
		results = append(results, comparison{
			SahaiEvent: event, ZkDPPRelation: relation,
			SahaiCaseCount: sg.CaseCount, ZkDPPCaseCount: len(zg),
			SahaiGas: sg.Gas.Median, ZkDPPGas: zkGas, GasRatio: ratio(sg.Gas.Median, zkGas),
			SahaiE2EMS: se.E2E.Median, ZkDPPE2EMS: zkE2E, E2ERatio: ratio(se.E2E.Median, zkE2E),
			Semantics: semantics[event],
		})
	}

	report := map[string]any{
		"backend":                 "anvil",
		"hashProfile":             "poseidon2",
		"threadMode":              "MT",
		"requestedThreads":        8,
		"runsPerCombination":      1,
		"comparisonBoundary":      "각 protocol의 연결 시나리오에서 같은 이름 또는 가장 가까운 기능 relation의 case 중앙값을 비교한다. 동일 semantics를 주장하지 않는다.",
		"ratioFormula":            "Sahai-POC actual / zkDPP-POC actual",
		"unmatchedZkDPPRelations": []string{"proceed", "recall"},
		"results":                 results,
	}
	writeJSON(filepath.Join(bench, "zkdpp-poc-comparison.json"), report)
	writeGasCSV(filepath.Join(bench, "zkdpp-poc-gas-comparison.csv"), results)
	writeE2ECSV(filepath.Join(bench, "zkdpp-poc-mt-e2e-comparison.csv"), results)
}

func requireProfile(label, profile, mode string) {
	if profile != "poseidon2" || mode != "MT" {
		panic(fmt.Sprintf("%s must be poseidon2/MT, got %s/%s", label, profile, mode))
	}
}

func findSahai(values []sahaiItem, event string) sahaiItem {
	for _, value := range values {
		if value.Event == event {
			return value
		}
	}
	panic("missing Sahai event " + event)
}

func median(values []float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	if len(copyValues)%2 == 1 {
		return copyValues[len(copyValues)/2]
	}
	return (copyValues[len(copyValues)/2-1] + copyValues[len(copyValues)/2]) / 2
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func read(path string, value any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		panic(err)
	}
}

func writeJSON(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func writeGasCSV(path string, values []comparison) {
	writeCSV(path, []string{"sahai_event", "zkdpp_closest_relation", "sahai_cases", "zkdpp_cases", "sahai_gas_median", "zkdpp_gas_median", "sahai_over_zkdpp_ratio", "semantics_boundary"}, func(v comparison) []string {
		return []string{v.SahaiEvent, v.ZkDPPRelation, strconv.Itoa(v.SahaiCaseCount), strconv.Itoa(v.ZkDPPCaseCount), f(v.SahaiGas), f(v.ZkDPPGas), f(v.GasRatio), v.Semantics}
	}, values)
}

func writeE2ECSV(path string, values []comparison) {
	writeCSV(path, []string{"sahai_event", "zkdpp_closest_relation", "sahai_cases", "zkdpp_cases", "sahai_mt_e2e_ms_median", "zkdpp_mt_e2e_ms_median", "sahai_over_zkdpp_ratio", "semantics_boundary"}, func(v comparison) []string {
		return []string{v.SahaiEvent, v.ZkDPPRelation, strconv.Itoa(v.SahaiCaseCount), strconv.Itoa(v.ZkDPPCaseCount), f(v.SahaiE2EMS), f(v.ZkDPPE2EMS), f(v.E2ERatio), v.Semantics}
	}, values)
}

func writeCSV(path string, header []string, row func(comparison) []string, values []comparison) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	w := csv.NewWriter(file)
	if err := w.Write(header); err != nil {
		panic(err)
	}
	for _, value := range values {
		if err := w.Write(row(value)); err != nil {
			panic(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		panic(err)
	}
}

func f(value float64) string { return strconv.FormatFloat(value, 'f', 6, 64) }
