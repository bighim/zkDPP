package main

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/bighim/zkDPP/sahai-poc/internal/events"
	"github.com/bighim/zkDPP/sahai-poc/internal/gadgetruntime"
)

type proofMarshaler interface{ MarshalSolidity() []byte }

type result struct {
	Case         string  `json:"case"`
	Event        string  `json:"event"`
	Proofs       int     `json:"proofs"`
	LogicalBytes int     `json:"logicalBytes"`
	BuildMS      float64 `json:"buildMs"`
	WitnessMS    float64 `json:"witnessMs"`
	ProveMS      float64 `json:"proveMs"`
	EncodeMS     float64 `json:"encodeMs"`
	TotalMS      float64 `json:"totalMs"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	profileFlag := flag.String("profile", "poseidon2", "poseidon2 or sha256")
	scenario := flag.String("scenario", "independent", "independent or canonical")
	threads := flag.Int("threads", 1, "GOMAXPROCS: 1 (ST) or 8 (MT)")
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
	runtimes := map[string]*gadgetruntime.Loaded{}
	for _, gadget := range []string{"merklepath", "eq", "add", "and"} {
		loaded, err := gadgetruntime.Load(*root, string(profile), gadget)
		if err != nil {
			fatal(fmt.Errorf("load %s/%s: %w", profile, gadget, err))
		}
		runtimes[gadget] = loaded
	}
	buildStart := time.Now()
	var fixtures []events.Fixture
	if *scenario == "canonical" {
		fixtures, err = events.BuildCanonical(profile)
	} else if *scenario == "independent" {
		fixtures, err = events.BuildIndependent(profile)
	} else {
		fatal(fmt.Errorf("invalid scenario %q", *scenario))
	}
	if err != nil {
		fatal(err)
	}
	buildEach := time.Since(buildStart) / time.Duration(len(fixtures))
	order := make([]string, 0, len(fixtures))
	payloads := map[string]events.Payload{}
	results := make([]result, 0, len(fixtures))
	for _, fixture := range fixtures {
		proofs := make([]events.Proof, 0, len(fixture.Assignments))
		var witnessTime, proveTime time.Duration
		for _, assignment := range fixture.Assignments { // Deliberately sequential: only gnark internals may use MT.
			proof, witness, prove, err := runtimes[assignment.Gadget].Prove(assignment.Assignment)
			if err != nil {
				fatal(fmt.Errorf("%s/%s: %w", fixture.Case, assignment.Gadget, err))
			}
			witnessTime += witness
			proveTime += prove
			public := make([]string, len(assignment.PublicInputs))
			for i, value := range assignment.PublicInputs {
				public[i] = value.String()
			}
			proofs = append(proofs, events.Proof{Gadget: assignment.Gadget, Proof: "0x" + hex.EncodeToString(proof.(proofMarshaler).MarshalSolidity()), PublicInput: public})
		}
		encodeStart := time.Now()
		payload, err := events.MakePayload(fixture, proofs, events.PayloadTiming{BuildMS: ms(buildEach), WitnessMS: ms(witnessTime), ProveMS: ms(proveTime)})
		if err != nil {
			fatal(err)
		}
		_, err = json.Marshal(payload)
		if err != nil {
			fatal(err)
		}
		encodeTime := time.Since(encodeStart)
		payload.Timing.EncodeMS = ms(encodeTime)
		order = append(order, fixture.Case)
		payloads[fixture.Case] = payload
		results = append(results, result{Case: fixture.Case, Event: fixture.Name, Proofs: len(proofs), LogicalBytes: payload.LogicalBytes, BuildMS: ms(buildEach), WitnessMS: ms(witnessTime), ProveMS: ms(proveTime), EncodeMS: ms(encodeTime), TotalMS: ms(buildEach + witnessTime + proveTime + encodeTime)})
		fmt.Printf("profile=%s threads=%d case=%-16s event=%-7s proofs=%d total=%.3fs\n", profile, *threads, fixture.Case, fixture.Name, len(proofs), ms(buildEach+witnessTime+proveTime+encodeTime)/1000)
	}
	threadName := map[int]string{1: "st", 8: "mt"}[*threads]
	base := fmt.Sprintf("%s-%s-%s", *scenario, profile, threadName)
	meta := map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339), "runs": 1, "hashProfile": profile, "scenario": *scenario, "threadMode": threadName, "requestedThreads": *threads, "goMaxProcs": runtime.GOMAXPROCS(0), "numCPU": runtime.NumCPU(), "goOS": runtime.GOOS, "goArch": runtime.GOARCH, "proofSystem": "PLONK-KZG", "curve": "BLS12-381", "proofCalls": "sequential; gnark internal parallelism only"}
	fixtureDoc := map[string]any{"metadata": meta, "order": order, "cases": payloads}
	if err := writeJSON(filepath.Join(*root, "testdata", "generated", base+"-payloads.json"), fixtureDoc); err != nil {
		fatal(err)
	}
	meta["results"] = results
	if err := writeJSON(filepath.Join(*root, "benchmarks", "event-payload-"+base+".json"), meta); err != nil {
		fatal(err)
	}
	if err := writeCSV(filepath.Join(*root, "benchmarks", "event-payload-"+base+".csv"), results); err != nil {
		fatal(err)
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}
func writeCSV(path string, values []result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"case", "event", "proofs", "logical_bytes", "build_ms", "witness_ms", "prove_ms", "encode_ms", "total_ms"})
	for _, v := range values {
		_ = w.Write([]string{v.Case, v.Event, strconv.Itoa(v.Proofs), strconv.Itoa(v.LogicalBytes), f64(v.BuildMS), f64(v.WitnessMS), f64(v.ProveMS), f64(v.EncodeMS), f64(v.TotalMS)})
	}
	return w.Error()
}
func f64(v float64) string       { return fmt.Sprintf("%.6f", v) }
func ms(v time.Duration) float64 { return float64(v) / float64(time.Millisecond) }
func fatal(err error)            { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
