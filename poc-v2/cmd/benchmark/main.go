package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bighim/zkDPP/poc-v2/internal/profilecase"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

type proofMarshaler interface{ MarshalSolidity() []byte }

type Result struct {
	Profile             string `json:"profile"`
	Event               string `json:"event"`
	StateLength         int    `json:"stateLength"`
	Arity               int    `json:"arity,omitempty"`
	TreeDepth           int    `json:"treeDepth"`
	Constraints         int    `json:"constraints"`
	PublicInputs        int    `json:"publicInputs"`
	CompileMillis       int64  `json:"compileMillis"`
	SetupMillis         int64  `json:"setupMillis"`
	ProveMillis         int64  `json:"proveMillis"`
	VerifyMillis        int64  `json:"verifyMillis"`
	TotalAllocatedBytes uint64 `json:"totalAllocatedBytes"`
	ProofBytes          int    `json:"proofBytes"`
	CCSBytes            int64  `json:"ccsBytes"`
	SRSCanonicalBytes   int64  `json:"srsCanonicalBytes"`
	SRSLagrangeBytes    int64  `json:"srsLagrangeBytes"`
	ProvingKeyBytes     int64  `json:"provingKeyBytes"`
	VerifyingKeyBytes   int64  `json:"verifyingKeyBytes"`
}

type Raw struct {
	GeneratedAt string   `json:"generatedAt"`
	Machine     string   `json:"machine"`
	Curve       string   `json:"curve"`
	ProofSystem string   `json:"proofSystem"`
	Hash        string   `json:"hash"`
	TreeDepth   int      `json:"treeDepth"`
	Results     []Result `json:"results"`
}

func main() {
	selected := flag.String("profile", "all", "profile name or all")
	flag.Parse()
	if err := run(*selected); err != nil {
		panic(err)
	}
}

func run(selected string) error {
	cases, err := allCases()
	if err != nil {
		return err
	}
	results := make([]Result, 0, len(cases))
	for _, c := range cases {
		if selected != "all" && selected != c.Name {
			continue
		}
		result, err := benchmark(c)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		results = append(results, result)
		fmt.Printf("%-22s constraints=%-6d public=%-2d setup=%4dms prove=%4dms verify=%3dms\n", c.Name, result.Constraints, result.PublicInputs, result.SetupMillis, result.ProveMillis, result.VerifyMillis)
	}
	if selected == "all" && len(results) != 34 {
		return fmt.Errorf("expected 34 configurations, got %d", len(results))
	}
	raw := Raw{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Machine: machine(), Curve: "BLS12-381", ProofSystem: "PLONK-KZG", Hash: "Poseidon2", TreeDepth: protocol.TreeDepth, Results: results}
	if err := writeJSON(filepath.Join("benchmarks", "raw.json"), raw); err != nil {
		return err
	}
	return writeCSV(filepath.Join("benchmarks", "summary.csv"), results)
}

func allCases() ([]profilecase.Case, error) {
	var cases []profilecase.Case
	for _, event := range []string{"entry", "transfer", "proceed", "recall", "merge", "split", "exit"} {
		for l := 0; l <= 3; l++ {
			c, err := profilecase.Build(event, l, 0)
			if err != nil {
				return nil, err
			}
			cases = append(cases, c)
		}
	}
	for l := 0; l <= 3; l++ {
		c, err := profilecase.Build("process", l, 3)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	for _, arity := range []int{1, 2} {
		c, err := profilecase.Build("process", 3, arity)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func benchmark(c profilecase.Case) (Result, error) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	compileStart := time.Now()
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, c.Circuit)
	if err != nil {
		return Result{}, err
	}
	compileMillis := time.Since(compileStart).Milliseconds()
	if got, want := ccs.GetNbPublicVariables(), len(c.Public); got != want {
		return Result{}, fmt.Errorf("public input count %d != fixture count %d", got, want)
	}
	setupStart := time.Now()
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	if err != nil {
		return Result{}, err
	}
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		return Result{}, err
	}
	setupMillis := time.Since(setupStart).Milliseconds()
	witness, err := frontend.NewWitness(c.Assignment, ecc.BLS12_381.ScalarField())
	if err != nil {
		return Result{}, err
	}
	publicWitness, err := witness.Public()
	if err != nil {
		return Result{}, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(ccs, pk, witness)
	if err != nil {
		return Result{}, err
	}
	proveMillis := time.Since(proveStart).Milliseconds()
	verifyStart := time.Now()
	if err := plonk.Verify(proof, vk, publicWitness); err != nil {
		return Result{}, err
	}
	verifyMillis := time.Since(verifyStart).Milliseconds()
	proofBytes := proof.(proofMarshaler).MarshalSolidity()
	runtime.ReadMemStats(&after)
	result := Result{
		Profile: c.Name, Event: c.Event, StateLength: c.StateLength, Arity: c.Arity,
		TreeDepth: protocol.TreeDepth, Constraints: ccs.GetNbConstraints(), PublicInputs: ccs.GetNbPublicVariables(),
		CompileMillis: compileMillis, SetupMillis: setupMillis, ProveMillis: proveMillis, VerifyMillis: verifyMillis,
		TotalAllocatedBytes: after.TotalAlloc - before.TotalAlloc, ProofBytes: len(proofBytes),
		CCSBytes: serializedSize(ccs), SRSCanonicalBytes: serializedSize(srs), SRSLagrangeBytes: serializedSize(srsLagrange),
		ProvingKeyBytes: serializedSize(pk), VerifyingKeyBytes: serializedSize(vk),
	}
	return result, nil
}

func serializedSize(value io.WriterTo) int64 {
	var counter byteCounter
	_, _ = value.WriteTo(&counter)
	return counter.n
}

type byteCounter struct{ n int64 }

func (w *byteCounter) Write(p []byte) (int, error) { w.n += int64(len(p)); return len(p), nil }

func machine() string {
	parts := []string{fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), "Go " + runtime.Version()}
	if runtime.GOOS == "darwin" {
		for _, item := range [][2]string{{"model", "hw.model"}, {"chip", "machdep.cpu.brand_string"}, {"cores", "hw.ncpu"}, {"memory_bytes", "hw.memsize"}} {
			if output, err := exec.Command("sysctl", "-n", item[1]).Output(); err == nil {
				parts = append(parts, item[0]+"="+strings.TrimSpace(string(output)))
			}
		}
		if output, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			parts = append(parts, "macOS="+strings.TrimSpace(string(output)))
		}
	}
	return strings.Join(parts, "; ")
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func writeCSV(path string, results []Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"profile", "event", "state_length", "arity", "constraints", "public_inputs", "compile_ms", "setup_ms", "prove_ms", "verify_ms", "total_allocated_bytes", "proof_bytes", "ccs_bytes", "srs_canonical_bytes", "srs_lagrange_bytes", "proving_key_bytes", "verifying_key_bytes"})
	for _, r := range results {
		_ = w.Write([]string{r.Profile, r.Event, strconv.Itoa(r.StateLength), strconv.Itoa(r.Arity), strconv.Itoa(r.Constraints), strconv.Itoa(r.PublicInputs), strconv.FormatInt(r.CompileMillis, 10), strconv.FormatInt(r.SetupMillis, 10), strconv.FormatInt(r.ProveMillis, 10), strconv.FormatInt(r.VerifyMillis, 10), strconv.FormatUint(r.TotalAllocatedBytes, 10), strconv.Itoa(r.ProofBytes), strconv.FormatInt(r.CCSBytes, 10), strconv.FormatInt(r.SRSCanonicalBytes, 10), strconv.FormatInt(r.SRSLagrangeBytes, 10), strconv.FormatInt(r.ProvingKeyBytes, 10), strconv.FormatInt(r.VerifyingKeyBytes, 10)})
	}
	w.Flush()
	return w.Error()
}
