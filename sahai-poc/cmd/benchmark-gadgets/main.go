package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/bighim/zkDPP/sahai-poc/circuits/gadgets"
	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/bighim/zkDPP/sahai-poc/internal/gadgetruntime"
)

type proofMarshaler interface{ MarshalSolidity() []byte }
type setupManifest struct {
	Constraints   int              `json:"constraints"`
	PublicInputs  int              `json:"publicInputs"`
	ArtifactBytes map[string]int64 `json:"artifactBytes"`
}
type result struct {
	HashProfile       string  `json:"hashProfile"`
	ThreadMode        string  `json:"threadMode"`
	Gadget            string  `json:"gadget"`
	Constraints       int     `json:"constraints"`
	PublicInputs      int     `json:"publicInputs"`
	ProofBytes        int     `json:"proofBytes"`
	CCSBytes          int64   `json:"ccsBytes"`
	ProvingKeyBytes   int64   `json:"provingKeyBytes"`
	VerifyingKeyBytes int64   `json:"verifyingKeyBytes"`
	SRSBytes          int64   `json:"srsBytes"`
	WitnessMS         float64 `json:"witnessMs"`
	ProveMS           float64 `json:"proveMs"`
	NativeVerifyMS    float64 `json:"nativeVerifyMs"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	profileFlag := flag.String("profile", "poseidon2", "poseidon2 or sha256")
	threads := flag.Int("threads", 1, "1 (ST) or 8 (MT)")
	runs := flag.Int("runs", 1, "must be one")
	flag.Parse()
	if *runs != 1 || (*threads != 1 && *threads != 8) {
		fatal(fmt.Errorf("runs must be 1 and threads must be 1 or 8"))
	}
	profile, err := document.ParseProfile(*profileFlag)
	if err != nil {
		fatal(err)
	}
	runtime.GOMAXPROCS(*threads)
	mode := map[int]string{1: "ST", 8: "MT"}[*threads]
	assignments, err := gadgets.CanonicalAssignmentsFor(profile)
	if err != nil {
		fatal(err)
	}
	results := make([]result, 0, len(assignments))
	for _, assignment := range assignments {
		loaded, err := gadgetruntime.Load(*root, string(profile), assignment.Gadget)
		if err != nil {
			fatal(err)
		}
		proof, witness, prove, err := loaded.Prove(assignment.Assignment)
		if err != nil {
			fatal(err)
		}
		verify, err := loaded.Verify(proof, assignment.Assignment)
		if err != nil {
			fatal(err)
		}
		var setup setupManifest
		readJSON(filepath.Join(*root, "artifacts", "gadgets", string(profile), assignment.Gadget, "manifest.json"), &setup)
		encoded := proof.(proofMarshaler).MarshalSolidity()
		v := result{HashProfile: string(profile), ThreadMode: mode, Gadget: assignment.Gadget, Constraints: setup.Constraints, PublicInputs: setup.PublicInputs, ProofBytes: len(encoded), CCSBytes: setup.ArtifactBytes["ccs.bin"], ProvingKeyBytes: setup.ArtifactBytes["proving.key"], VerifyingKeyBytes: setup.ArtifactBytes["verifying.key"], SRSBytes: setup.ArtifactBytes["srs-canonical.bin"] + setup.ArtifactBytes["srs-lagrange.bin"], WitnessMS: ms(witness), ProveMS: ms(prove), NativeVerifyMS: ms(verify)}
		results = append(results, v)
		fmt.Printf("profile=%s mode=%s gadget=%-10s prove=%.3fms verify=%.3fms\n", profile, mode, assignment.Gadget, v.ProveMS, v.NativeVerifyMS)
	}
	base := fmt.Sprintf("gadgets-%s-%s", profile, stringsLower(mode))
	meta := map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339Nano), "runs": 1, "hashProfile": profile, "threadMode": mode, "requestedThreads": *threads, "goMaxProcs": runtime.GOMAXPROCS(0), "numCPU": runtime.NumCPU(), "proofCalls": "one gadget at a time; gnark internal parallelism only", "proofSystem": "PLONK-KZG", "curve": "BLS12-381", "results": results}
	writeJSON(filepath.Join(*root, "benchmarks", base+".json"), meta)
	writeCSV(filepath.Join(*root, "benchmarks", base+".csv"), results)
}
func readJSON(path string, target any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	if err = json.Unmarshal(raw, target); err != nil {
		fatal(err)
	}
}
func writeJSON(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
		fatal(err)
	}
}
func writeCSV(path string, values []result) {
	f, err := os.Create(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"hash_profile", "thread_mode", "gadget", "constraints", "public_inputs", "proof_bytes", "ccs_bytes", "proving_key_bytes", "verifying_key_bytes", "srs_bytes", "witness_ms", "prove_ms", "native_verify_ms"})
	for _, v := range values {
		_ = w.Write([]string{v.HashProfile, v.ThreadMode, v.Gadget, strconv.Itoa(v.Constraints), strconv.Itoa(v.PublicInputs), strconv.Itoa(v.ProofBytes), strconv.FormatInt(v.CCSBytes, 10), strconv.FormatInt(v.ProvingKeyBytes, 10), strconv.FormatInt(v.VerifyingKeyBytes, 10), strconv.FormatInt(v.SRSBytes, 10), f64(v.WitnessMS), f64(v.ProveMS), f64(v.NativeVerifyMS)})
	}
	if err = w.Error(); err != nil {
		fatal(err)
	}
}
func stringsLower(v string) string {
	if v == "ST" {
		return "st"
	}
	return "mt"
}
func ms(v time.Duration) float64 { return float64(v) / float64(time.Millisecond) }
func f64(v float64) string       { return strconv.FormatFloat(v, 'f', 6, 64) }
func fatal(err error)            { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
