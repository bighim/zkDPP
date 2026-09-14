package experiments

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Envelope struct {
	Experiment           int    `json:"experiment"`
	Status               string `json:"status"`
	Smoke                bool   `json:"smoke"`
	Warmups, Repetitions int
	Rows                 json.RawMessage `json:"rows"`
	DPP                  []DPPRow        `json:"dpp"`
	Routes               []RouteRow      `json:"routes"`
}

func loadEnvelope(root string, exp int) (Envelope, error) {
	var v Envelope
	e := readJSON(filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", exp)), &v)
	return v, e
}
func checkRuns(groups map[string][]int) error {
	for key, runs := range groups {
		if len(runs) != 6 {
			return fmt.Errorf("%s has %d runs, expected warmup+5", key, len(runs))
		}
		seen := map[int]bool{}
		for _, r := range runs {
			if r < 0 || r > 5 || seen[r] {
				return fmt.Errorf("invalid/duplicate run %s %d", key, r)
			}
			seen[r] = true
		}
	}
	return nil
}
func ValidateExperiment(root string, exp int) error {
	env, e := loadEnvelope(root, exp)
	if e != nil {
		return e
	}
	if env.Experiment != exp || env.Status != "complete" || env.Smoke || env.Warmups != 1 || env.Repetitions != 5 {
		return fmt.Errorf("incomplete experiment %d", exp)
	}
	groups := map[string][]int{}
	if exp == 4 {
		var rows []AuditRow
		if e = json.Unmarshal(env.Rows, &rows); e != nil {
			return e
		}
		if len(rows) != 216 {
			return fmt.Errorf("audit rows %d", len(rows))
		}
		for _, r := range rows {
			if !r.Matched || r.TotalMS <= 0 || r.RPCMS < 0 || r.RPCMS > r.TotalMS || r.Warmup != (r.Run == 0) {
				return fmt.Errorf("audit measurement invalid")
			}
			groups[r.Case+"/"+r.Operation] = append(groups[r.Case+"/"+r.Operation], r.Run)
		}
	} else if exp >= 7 {
		var rows []CryptoRow
		if e = json.Unmarshal(env.Rows, &rows); e != nil {
			return e
		}
		expected := 54
		if exp == 8 {
			expected = 48
		}
		if len(rows) != expected {
			return fmt.Errorf("crypto rows %d", len(rows))
		}
		for _, r := range rows {
			if !r.Matched || r.TotalMS <= 0 || r.InnerIterations != 1000 || r.Warmup != (r.Run == 0) {
				return fmt.Errorf("crypto measurement invalid")
			}
			groups[r.Case] = append(groups[r.Case], r.Run)
		}
	} else {
		var rows []Measurement
		if e = json.Unmarshal(env.Rows, &rows); e != nil {
			return e
		}
		expected := map[int]int{1: 84, 2: 30, 3: 84, 5: 54, 6: 60}[exp]
		if len(rows) != expected {
			return fmt.Errorf("experiment %d rows %d expected %d", exp, len(rows), expected)
		}
		keys := map[string]*Keys{}
		for _, r := range rows {
			if !r.Matched || r.Tx.Status != 1 || r.Tx.Gas > 30000000 || r.E2EMS <= 0 || r.WitnessMS < 0 || r.ProveMS <= 0 || r.TransactionMS <= 0 || math.Abs(r.E2EMS-(r.WitnessMS+r.ProveMS+r.TransactionMS)) > 5 || r.Warmup != (r.Run == 0) {
				return fmt.Errorf("invalid E2E %s run %d", r.Case, r.Run)
			}
			key := r.Case + "/" + r.Profile
			groups[key] = append(groups[key], r.Run)
			k := keys[r.Profile]
			if k == nil {
				k, e = LoadKeys(root, r.Profile)
				if e != nil {
					return e
				}
				keys[r.Profile] = k
			}
			var f struct {
				Profile        string
				Public         []fr.Element
				Proof, GoProof string
				Tx             Transaction
			}
			proofPath := r.ProofArtifact
			if !filepath.IsAbs(proofPath) {
				proofPath = filepath.Join(root, proofPath)
			}
			if e = readJSON(proofPath, &f); e != nil {
				return e
			}
			if f.Profile != r.Profile || f.Tx.Hash != r.Tx.Hash || len(f.Public) != r.PublicInputs || r.Constraints != k.Relation.Constraints {
				return fmt.Errorf("proof metadata mismatch")
			}
			proof := plonk.NewProof(ecc.BLS12_381)
			raw, e := hex.DecodeString(strings.TrimPrefix(f.GoProof, "0x"))
			if e != nil {
				return e
			}
			if _, e = proof.ReadFrom(bytes.NewReader(raw)); e != nil {
				return e
			}
			c := &Circuit{Public: make([]frontend.Variable, len(f.Public))}
			for i, x := range f.Public {
				c.Public[i] = x
			}
			w, e := frontend.NewWitness(c, ecc.BLS12_381.ScalarField(), frontend.PublicOnly())
			if e != nil {
				return e
			}
			if e = plonk.Verify(proof, k.VK, w); e != nil {
				return fmt.Errorf("stored proof invalid %s: %w", r.Case, e)
			}
			solidity, e := hex.DecodeString(strings.TrimPrefix(f.Proof, "0x"))
			if e != nil {
				return e
			}
			if !bytes.Equal(solidity, proof.(interface{ MarshalSolidity() []byte }).MarshalSolidity()) {
				return fmt.Errorf("Solidity proof mismatch")
			}
		}
		if exp == 1 {
			if len(env.DPP) != 6 {
				return fmt.Errorf("missing DPP runs")
			}
			for _, r := range env.DPP {
				if !r.Matched || r.RPCCalls == 0 || r.TotalMS <= 0 || r.LocalMS < 0 {
					return fmt.Errorf("invalid DPP measurement")
				}
				groups["dpp"] = append(groups["dpp"], r.Run)
			}
		}
		if exp == 2 {
			if len(env.Routes) != 18 {
				return fmt.Errorf("missing route runs")
			}
			for _, r := range env.Routes {
				if !r.Matched || r.TotalMS <= 0 {
					return fmt.Errorf("invalid route")
				}
				groups["route-"+r.Route] = append(groups["route-"+r.Route], r.Run)
			}
		}
	}
	return checkRuns(groups)
}
func currentHashes(root string) (map[string]string, error) {
	out := map[string]string{}
	for _, dir := range []string{"internal/experiments", "cmd/experiments", "contracts/src/experiments", "contracts/experiments/src", "contracts/experiments/test", "contracts/experiments/out", ArtifactDir + "/circuits", ArtifactDir + "/srs", ArtifactDir + "/proofs", "contracts/src/generated/experiments", OutputDir} {
		if e := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(p, "checksums.json") {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			h, e := checksum(p)
			out[rel] = h
			return e
		}); e != nil {
			return nil, e
		}
	}
	for _, p := range []string{"Makefile", "docker-compose.experiments.yml", "contracts/experiments/foundry.toml", ArtifactDir + "/config.json", ArtifactDir + "/baseline.json"} {
		h, e := checksum(filepath.Join(root, p))
		if e != nil {
			return nil, e
		}
		out[p] = h
	}
	return out, nil
}
func Finalize(root string) error {
	if e := CheckBaseline(root); e != nil {
		return e
	}
	for i := 1; i <= 8; i++ {
		if e := ValidateExperiment(root, i); e != nil {
			return e
		}
	}
	hashes, e := currentHashes(root)
	if e != nil {
		return e
	}
	return writeJSON(filepath.Join(root, OutputDir, "checksums.json"), hashes)
}
func Check(root string) error {
	if e := CheckBaseline(root); e != nil {
		return e
	}
	var want map[string]string
	if e := readJSON(filepath.Join(root, OutputDir, "checksums.json"), &want); e != nil {
		return e
	}
	for p, h := range want {
		got, e := checksum(filepath.Join(root, p))
		if e != nil {
			return e
		}
		if got != h {
			return fmt.Errorf("checksum mismatch %s", p)
		}
	}
	for i := 1; i <= 8; i++ {
		if e := ValidateExperiment(root, i); e != nil {
			return e
		}
	}
	return nil
}
func Attempt(root, mode string, exp int, smoke bool, f func() error) (err error) {
	id := fmt.Sprintf("%s-%d-%d", mode, exp, time.Now().UnixNano())
	p := filepath.Join(root, ArtifactDir, "attempts", id)
	start := time.Now()
	if e := writeJSON(p+"-start.json", map[string]any{"mode": mode, "experiment": exp, "smoke": smoke, "started": start.UTC(), "pid": os.Getpid()}); e != nil {
		return e
	}
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("panic: %v", x)
		}
		row := map[string]any{"mode": mode, "experiment": exp, "smoke": smoke, "elapsedMS": ms(time.Since(start)), "success": err == nil}
		if err != nil {
			row["error"] = err.Error()
		}
		if e := writeJSON(p+"-finish.json", row); e != nil && err == nil {
			err = e
		}
	}()
	return f()
}
func RunAll(root string) error {
	for i := 1; i <= 8; i++ {
		if _, e := os.Stat(filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", i))); e == nil {
			if e = ValidateExperiment(root, i); e != nil {
				return e
			}
			fmt.Printf("reuse verified experiment %d\n", i)
			continue
		}
		if e := Attempt(root, "run", i, false, func() error { return Run(root, i, false) }); e != nil {
			return e
		}
	}
	return Finalize(root)
}
