package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	mergecircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/merge"
	splitcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m1case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m4case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"os"
	"path/filepath"
	"time"
)

type solidityMarshaler interface{ MarshalSolidity() []byte }
type proofFixture struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}
type entryFixture struct {
	Name, Commitment string
	proofFixture
}
type mergeFixture struct {
	NoteRoot, NF1, NF2, CMOut string
	proofFixture
}
type splitFixture struct {
	NoteRoot, NF, CMOut1, CMOut2 string
	proofFixture
}
type fixedFixtures struct {
	Entries    []entryFixture
	Merge      mergeFixture
	Split      splitFixture
	FinalRoot  string
	FinalCount uint64
}
type circuitResult struct {
	Feature                                                 string
	Constraints, PublicInputs                               int
	CompileMillis, SRSMillis, PlonkSetupMillis, SetupMillis int64
	WitnessMillis, ProveMillis, VerifyMillis                float64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactFileBytes                                       map[string]int64
}
type report struct {
	GeneratedAt        string
	RunCount           int
	Curve, ProofSystem string
	Results            []circuitResult
}
type checksums struct {
	GeneratedAt, Algorithm string
	Files, PriorOutputs    map[string]string
}

func main() {
	root := flag.String("root", ".", "root")
	flag.Parse()
	if e := run(*root); e != nil {
		panic(e)
	}
}
func run(root string) error {
	s, e := m4case.Build(root)
	if e != nil {
		return e
	}
	entry, e := ensureEntry(root)
	if e != nil {
		return e
	}
	private, e := ensurePrivate(root)
	if e != nil {
		return e
	}
	old := []struct {
		name, milestone string
		loaded          *artifact.Loaded
	}{{"entry", "m2", entry}, {"private-spend", "m1", private}}
	for _, n := range []string{"transfer", "proceed", "recall"} {
		l, err := artifact.LoadAt(root, "m3", n)
		if err != nil {
			return err
		}
		old = append(old, struct {
			name, milestone string
			loaded          *artifact.Loaded
		}{n, "m3", l})
	}
	specs := []artifact.Spec{{Name: "merge", Circuit: &mergecircuit.Circuit{}, ExpectedPublic: 4}, {Name: "split", Circuit: &splitcircuit.Circuit{}, ExpectedPublic: 4}}
	assign := []frontend.Circuit{s.Merge.Assignment, s.Split.Assignment}
	loaded := make([]*artifact.Loaded, 2)
	man := make([]artifact.Manifest, 2)
	for i, v := range specs {
		man[i], e = artifact.SetupAt(root, "m4", v)
		if e != nil {
			return e
		}
		loaded[i], e = artifact.LoadAt(root, "m4", v.Name)
		if e != nil {
			return e
		}
	}
	for _, v := range old {
		if e = artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", v.name, "PlonkVerifier.sol"), v.loaded.VK); e != nil {
			return e
		}
	}
	for i, v := range specs {
		if e = artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", v.Name, "PlonkVerifier.sol"), loaded[i].VK); e != nil {
			return e
		}
	}
	if e = writePoseidon(filepath.Join(root, "contracts", "src", "generated", "Poseidon2BLS12381.sol")); e != nil {
		return e
	}
	f := fixedFixtures{FinalRoot: s.FinalRoot.String(), FinalCount: s.FinalCount}
	for _, v := range s.Entries {
		_, _, sol, _, _, _, err := prove(entry, v.Assignment)
		if err != nil {
			return err
		}
		f.Entries = append(f.Entries, entryFixture{v.Name, v.Note.Commitment.String(), proofFixture{"0x" + hex.EncodeToString(sol), []string{v.Note.Commitment.String()}}})
	}
	results := make([]circuitResult, 0, 2)
	proofs := make([][]byte, 2)
	for i := range specs {
		_, bin, sol, wt, pt, vt, err := prove(loaded[i], assign[i])
		if err != nil {
			return err
		}
		proofs[i] = sol
		m := man[i]
		results = append(results, circuitResult{specs[i].Name, m.Constraints, m.PublicInputs, m.CompileMillis, m.SRSMillis, m.PlonkSetupMillis, m.SetupMillis, ms(wt), ms(pt), ms(vt), bin, len(sol), m.FileBytes})
	}
	rootMerge := field(s.Merge.Assignment.NoteRoot)
	nf1 := field(s.Merge.Assignment.NF1)
	nf2 := field(s.Merge.Assignment.NF2)
	f.Merge = mergeFixture{rootMerge, nf1, nf2, s.Merge.Output.Commitment.String(), proofFixture{"0x" + hex.EncodeToString(proofs[0]), []string{rootMerge, nf1, nf2, s.Merge.Output.Commitment.String()}}}
	rootSplit := field(s.Split.Assignment.NoteRoot)
	nf := field(s.Split.Assignment.NF)
	f.Split = splitFixture{rootSplit, nf, s.Split.Outputs[0].Commitment.String(), s.Split.Outputs[1].Commitment.String(), proofFixture{"0x" + hex.EncodeToString(proofs[1]), []string{rootSplit, nf, s.Split.Outputs[0].Commitment.String(), s.Split.Outputs[1].Commitment.String()}}}
	if e = artifact.WriteJSON(filepath.Join(root, "contracts", "test", "fixtures", "m4-proofs.json"), f); e != nil {
		return e
	}
	if e = artifact.WriteJSON(filepath.Join(root, "output", "m4-circuit.json"), report{time.Now().UTC().Format(time.RFC3339), 1, "BLS12-381", "PLONK-KZG", results}); e != nil {
		return e
	}
	if e = writeChecksums(root); e != nil {
		return e
	}
	for _, v := range results {
		fmt.Printf("%-8s constraints=%d public=%d solidity-proof=%dB\n", v.Feature, v.Constraints, v.PublicInputs, v.SolidityProofBytes)
	}
	return nil
}
func field(v frontend.Variable) string { x := v.(fr.Element); return x.String() }
func ensureEntry(root string) (*artifact.Loaded, error) {
	if l, e := artifact.LoadAt(root, "m2", "entry"); e == nil {
		return l, nil
	}
	if _, e := artifact.SetupAt(root, "m2", artifact.Spec{Name: "entry", Circuit: &entrycircuit.Circuit{}, ExpectedPublic: 1}); e != nil {
		return nil, e
	}
	return artifact.LoadAt(root, "m2", "entry")
}
func ensurePrivate(root string) (*artifact.Loaded, error) {
	if l, e := artifact.Load(root, "private-spend"); e == nil {
		return l, nil
	}
	cs, e := m1case.All(root)
	if e != nil {
		return nil, e
	}
	for _, c := range cs {
		if c.Name == "private-spend" {
			if _, e = artifact.Setup(root, c); e != nil {
				return nil, e
			}
			return artifact.Load(root, "private-spend")
		}
	}
	return nil, fmt.Errorf("private spend missing")
}
func prove(l *artifact.Loaded, a frontend.Circuit) (plonk.Proof, int, []byte, time.Duration, time.Duration, time.Duration, error) {
	ws := time.Now()
	w, e := frontend.NewWitness(a, ecc.BLS12_381.ScalarField())
	wt := time.Since(ws)
	if e != nil {
		return nil, 0, nil, wt, 0, 0, e
	}
	pub, e := w.Public()
	if e != nil {
		return nil, 0, nil, wt, 0, 0, e
	}
	ps := time.Now()
	p, e := plonk.Prove(l.CCS, l.PK, w)
	pt := time.Since(ps)
	if e != nil {
		return nil, 0, nil, wt, pt, 0, e
	}
	vs := time.Now()
	e = plonk.Verify(p, l.VK, pub)
	vt := time.Since(vs)
	if e != nil {
		return nil, 0, nil, wt, pt, vt, e
	}
	var b bytes.Buffer
	_, e = p.WriteTo(&b)
	if e != nil {
		return nil, 0, nil, wt, pt, vt, e
	}
	return p, b.Len(), p.(solidityMarshaler).MarshalSolidity(), wt, pt, vt, nil
}
func ms(v time.Duration) float64 { return float64(v.Microseconds()) / 1000 }
func writePoseidon(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return solgen.WritePoseidon2BLS12381(f)
}
func writeChecksums(root string) error {
	names := []string{"contracts/src/generated/merge/PlonkVerifier.sol", "contracts/src/generated/split/PlonkVerifier.sol", "contracts/test/fixtures/m4-proofs.json"}
	r := checksums{time.Now().UTC().Format(time.RFC3339), "SHA-256", map[string]string{}, map[string]string{}}
	for _, n := range names {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		r.Files[n] = v
	}
	for _, n := range []string{"output/m1-core.json", "output/m2-circuit.json", "output/m2-anvil-gas.json", "output/m2-anvil-e2e.json", "output/m3-circuit.json", "output/m3-anvil-gas.json", "output/m3-anvil-e2e.json"} {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		r.PriorOutputs[n] = v
	}
	return artifact.WriteJSON(filepath.Join(root, "output", "m4-generated-checksums.json"), r)
}
