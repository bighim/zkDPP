package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type stats struct {
	Median float64 `json:"median"`
}
type gadget struct {
	Gadget       string  `json:"gadget"`
	Constraints  int     `json:"constraints"`
	PublicInputs int     `json:"publicInputs"`
	ProofBytes   int     `json:"proofBytes"`
	ProveMS      float64 `json:"proveMs"`
}
type gadgetReport struct {
	Runs             int      `json:"runs"`
	RequestedThreads int      `json:"requestedThreads"`
	GoMaxProcs       int      `json:"goMaxProcs"`
	ThreadMode       string   `json:"threadMode"`
	HashProfile      string   `json:"hashProfile"`
	Results          []gadget `json:"results"`
}
type payload struct {
	Case         string  `json:"case"`
	Event        string  `json:"event"`
	Proofs       int     `json:"proofs"`
	LogicalBytes int     `json:"logicalBytes"`
	ProveMS      float64 `json:"proveMs"`
}
type payloadReport struct {
	Runs             int       `json:"runs"`
	RequestedThreads int       `json:"requestedThreads"`
	GoMaxProcs       int       `json:"goMaxProcs"`
	ThreadMode       string    `json:"threadMode"`
	HashProfile      string    `json:"hashProfile"`
	Results          []payload `json:"results"`
}
type evm struct {
	Event     string `json:"event"`
	CaseCount int    `json:"caseCount"`
	Gas       stats  `json:"receiptGasUsed"`
	E2E       stats  `json:"e2eMs"`
}
type evmReport struct {
	Runs             int    `json:"runs"`
	RequestedThreads int    `json:"requestedThreads"`
	GoMaxProcs       int    `json:"goMaxProcs"`
	ThreadMode       string `json:"threadMode"`
	HashProfile      string `json:"hashProfile"`
	Results          []evm  `json:"results"`
}
type comparison struct {
	SahaiEvent string  `json:"sahaiEvent"`
	Z          string  `json:"zkdppClosestRelation"`
	SC         int     `json:"sahaiCaseCount"`
	ZC         int     `json:"zkdppCaseCount"`
	SG         float64 `json:"sahaiGasMedian"`
	ZG         float64 `json:"zkdppGasMedianAcrossCases"`
	GR         float64 `json:"sahaiOverZkdppGas"`
	SE         float64 `json:"sahaiE2EMsMedian"`
	ZE         float64 `json:"zkdppE2EMsMedianAcrossCases"`
	ER         float64 `json:"sahaiOverZkdppE2E"`
}
type comparisonReport struct {
	Runs        int          `json:"runsPerCombination"`
	ThreadMode  string       `json:"threadMode"`
	HashProfile string       `json:"hashProfile"`
	Results     []comparison `json:"results"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	flag.Parse()
	for _, profile := range []string{"poseidon2", "sha256"} {
		for _, mode := range []string{"st", "mt"} {
			validateGadget(*root, profile, mode)
		}
		for _, scenario := range []string{"canonical", "independent"} {
			validatePayload(*root, profile, scenario)
		}
		validateEVM(*root, "anvil-"+profile+"-gas")
		validateEVM(*root, "anvil-"+profile+"-mt-e2e")
	}
	validateEVM(*root, "besu-poseidon2-gas")
	validateEVM(*root, "besu-poseidon2-mt-e2e")
	validateComparison(*root)
	validateParity(*root)
	validateActiveArtifacts(*root)
	fmt.Println("M8 RESULT GATE PASS: JSON/CSV/TeX, MT one-run policy, parity, RSA-free active artifacts")
}

func validateGadget(root, profile, mode string) {
	base := "gadgets-" + profile + "-" + mode
	var report gadgetReport
	readJSON(root, "benchmarks/"+base+".json", &report)
	wantThreads := map[string]int{"st": 1, "mt": 8}[mode]
	if report.Runs != 1 || report.HashProfile != profile || strings.ToLower(report.ThreadMode) != mode || report.RequestedThreads != wantThreads || report.GoMaxProcs != wantThreads {
		panic("gadget metadata " + base)
	}
	rows := readCSV(root, "benchmarks/"+base+".csv")
	if len(rows) != len(report.Results) {
		panic("gadget row count " + base)
	}
	for i, v := range report.Results {
		eq(rows[i]["gadget"], v.Gadget)
		eqInt(rows[i]["constraints"], v.Constraints)
		eqInt(rows[i]["public_inputs"], v.PublicInputs)
		eqInt(rows[i]["proof_bytes"], v.ProofBytes)
		near(rows[i]["prove_ms"], v.ProveMS)
	}
}

func validatePayload(root, profile, scenario string) {
	base := "event-payload-" + scenario + "-" + profile + "-mt"
	var report payloadReport
	readJSON(root, "benchmarks/"+base+".json", &report)
	if report.Runs != 1 || strings.ToLower(report.ThreadMode) != "mt" || report.RequestedThreads != 8 || report.GoMaxProcs != 8 || report.HashProfile != profile {
		panic("payload metadata " + base)
	}
	rows := readCSV(root, "benchmarks/"+base+".csv")
	if len(rows) != len(report.Results) {
		panic("payload row count " + base)
	}
	for i, v := range report.Results {
		eq(rows[i]["case"], v.Case)
		eq(rows[i]["event"], v.Event)
		eqInt(rows[i]["proofs"], v.Proofs)
		eqInt(rows[i]["logical_bytes"], v.LogicalBytes)
		near(rows[i]["prove_ms"], v.ProveMS)
	}
}

func validateEVM(root, base string) {
	var report evmReport
	readJSON(root, "benchmarks/"+base+".json", &report)
	if report.Runs != 1 {
		panic("EVM runs " + base)
	}
	if strings.Contains(base, "mt-e2e") && (report.ThreadMode != "MT" || report.RequestedThreads != 8 || report.GoMaxProcs != 8) {
		panic("EVM MT metadata " + base)
	}
	rows := readCSV(root, "benchmarks/"+base+".csv")
	if len(rows) != len(report.Results) {
		panic("EVM row count " + base)
	}
	for i, v := range report.Results {
		eq(rows[i]["event"], v.Event)
		eqInt(rows[i]["case_count"], v.CaseCount)
		near(rows[i]["gas_median"], v.Gas.Median)
		near(rows[i]["e2e_ms_median"], v.E2E.Median)
	}
}

func validateComparison(root string) {
	var raw comparisonReport
	readJSON(root, "benchmarks/zkdpp-poc-comparison.json", &raw)
	if raw.Runs != 1 || raw.ThreadMode != "MT" || raw.HashProfile != "poseidon2" {
		panic("comparison metadata")
	}
	gas := readCSV(root, "benchmarks/zkdpp-poc-gas-comparison.csv")
	e2e := readCSV(root, "benchmarks/zkdpp-poc-mt-e2e-comparison.csv")
	texRaw, err := os.ReadFile(filepath.Join(root, "Sahai POC Evaluation Report.tex"))
	if err != nil {
		panic(err)
	}
	tex := string(texRaw)
	for i, v := range raw.Results {
		eq(gas[i]["sahai_event"], v.SahaiEvent)
		near(gas[i]["sahai_gas_median"], v.SG)
		near(gas[i]["zkdpp_gas_median"], v.ZG)
		near(gas[i]["sahai_over_zkdpp_ratio"], v.GR)
		near(e2e[i]["sahai_mt_e2e_ms_median"], v.SE)
		near(e2e[i]["zkdpp_mt_e2e_ms_median"], v.ZE)
		near(e2e[i]["sahai_over_zkdpp_ratio"], v.ER)
		gasLine := fmt.Sprintf("%s & %s & %d & %d & %.0f & %.0f & %.3f", v.SahaiEvent, v.Z, v.SC, v.ZC, v.SG, v.ZG, v.GR)
		e2eLine := fmt.Sprintf("%s & %s & %d & %d & %.1f & %.1f & %.3f", v.SahaiEvent, v.Z, v.SC, v.ZC, v.SE, v.ZE, v.ER)
		if !strings.Contains(tex, gasLine) || !strings.Contains(tex, e2eLine) {
			panic("TeX comparison mismatch " + v.SahaiEvent)
		}
	}
}

func validateParity(root string) {
	var v struct {
		Passed  bool `json:"passed"`
		Results []struct {
			GasEqual      bool `json:"gasEqual"`
			CalldataEqual bool `json:"calldataEqual"`
		} `json:"results"`
	}
	readJSON(root, "benchmarks/backend-parity.json", &v)
	if !v.Passed {
		panic("backend parity")
	}
	for _, x := range v.Results {
		if !x.GasEqual || !x.CalldataEqual {
			panic("backend parity row")
		}
	}
}

func validateActiveArtifacts(root string) {
	for _, rel := range []string{"contracts/src", "contracts/test/fixtures", "internal/document", "internal/events", "testdata/generated"} {
		err := filepath.Walk(filepath.Join(root, rel), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lower := strings.ToLower(string(raw))
			for _, term := range []string{"docprime", "docaccumulator", "rpoke", "pocklington", "primecertificate", "verifymembership", "verifynonmembership"} {
				if strings.Contains(lower, term) {
					return fmt.Errorf("forbidden %s in %s", term, path)
				}
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
}

func readJSON(root, path string, v any) {
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		panic(err)
	}
}
func readCSV(root, path string) []map[string]string {
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	all, err := csv.NewReader(f).ReadAll()
	if err != nil {
		panic(err)
	}
	out := make([]map[string]string, 0, len(all)-1)
	for _, row := range all[1:] {
		m := map[string]string{}
		for i, k := range all[0] {
			m[k] = row[i]
		}
		out = append(out, m)
	}
	return out
}
func eq(a, b string) {
	if a != b {
		panic(fmt.Sprintf("%q != %q", a, b))
	}
}
func eqInt(a string, b int) {
	v, err := strconv.Atoi(a)
	if err != nil || v != b {
		panic("integer mismatch")
	}
}
func near(a string, b float64) {
	v, err := strconv.ParseFloat(a, 64)
	if err != nil || math.Abs(v-b) > 0.0000015 {
		panic(fmt.Sprintf("float mismatch %s %.9f", a, b))
	}
}
