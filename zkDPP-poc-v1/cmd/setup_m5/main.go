package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	processcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m5case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/consensys/gnark/frontend"
	"os"
	"path/filepath"
	"time"
)

type solidityMarshaler interface{ MarshalSolidity() []byte }
type pf struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}
type ef struct {
	Name, Commitment string
	pf
}
type fixture struct {
	Entries                       []ef
	PolicyRef, ScopeRef, NoteRoot string
	NF                            [3]string
	CMOut                         [2]string
	pf
	FinalRoot  string
	FinalCount uint64
}
type result struct {
	Feature                                                                                     string
	Constraints, PublicInputs                                                                   int
	CompileMillis, SRSMillis, PlonkSetupMillis, SetupMillis                                     int64
	WitnessMillis, ProveMillis, VerifyMillis                                                    float64
	BinaryProofBytes, SolidityProofBytes, SerializedVKBytes, OptimizedVKWords, OptimizedVKBytes int
	VKHash                                                                                      string
	ArtifactFileBytes                                                                           map[string]int64
}
type report struct {
	GeneratedAt        string
	RunCount           int
	Curve, ProofSystem string
	Results            []result
}
type checks struct {
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
	s, e := m5case.Build(root)
	if e != nil {
		return e
	}
	m, e := artifact.SetupAt(root, "m5", artifact.Spec{Name: "process-policy-3-2", Circuit: &processcircuit.Circuit{}, ExpectedPublic: 8})
	if e != nil {
		return e
	}
	l, e := artifact.LoadAt(root, "m5", "process-policy-3-2")
	if e != nil {
		return e
	}
	if e = artifact.ExportSolidity(filepath.Join(root, "contracts", "src", "generated", "process-policy-3-2", "PlonkVerifier.sol"), l.VK); e != nil {
		return e
	}
	constantSource, e := os.ReadFile(filepath.Join(root, "contracts", "src", "generated", "process-policy-3-2", "PlonkVerifier.sol"))
	if e != nil {
		return e
	}
	storagePath := filepath.Join(root, "contracts", "src", "generated", "storage-process", "StorageProcessVerifier.sol")
	if e = os.MkdirAll(filepath.Dir(storagePath), 0755); e != nil {
		return e
	}
	storageFile, e := os.Create(storagePath)
	if e != nil {
		return e
	}
	if e = solgen.WriteStorageProcessVerifier(storageFile, string(constantSource)); e != nil {
		_ = storageFile.Close()
		return e
	}
	if e = storageFile.Close(); e != nil {
		return e
	}
	concrete, ok := l.VK.(*plonkbls.VerifyingKey)
	if !ok {
		return fmt.Errorf("unexpected VK %T", l.VK)
	}
	optimized, e := solgen.ExtractOptimizedVK(concrete)
	if e != nil {
		return e
	}
	if e = artifact.WriteJSON(filepath.Join(root, "artifacts", "development", "m5", "process-policy-3-2", "optimized-vk.json"), optimized); e != nil {
		return e
	}
	_, bin, sol, wt, pt, vt, e := prove(l, s.Assignment)
	if e != nil {
		return e
	}
	f := fixture{PolicyRef: s.PolicyRef.String(), ScopeRef: s.ScopeRef.String(), NoteRoot: s.InputRoot.String(), FinalRoot: s.FinalRoot.String(), FinalCount: s.FinalCount, pf: pf{"0x" + hex.EncodeToString(sol), nil}}
	for _, v := range s.Entries {
		f.Entries = append(f.Entries, ef{v.Name, v.Note.Commitment.String(), pf{}})
	}
	for i := 0; i < 3; i++ {
		x := s.Assignment.NF[i].(fr.Element)
		f.NF[i] = x.String()
	}
	for i := 0; i < 2; i++ {
		f.CMOut[i] = s.Outputs[i].Commitment.String()
	}
	f.PublicInputs = []string{f.PolicyRef, f.ScopeRef, f.NoteRoot, f.NF[0], f.NF[1], f.NF[2], f.CMOut[0], f.CMOut[1]}
	if e = artifact.WriteJSON(filepath.Join(root, "contracts", "test", "fixtures", "m5-proofs.json"), f); e != nil {
		return e
	}
	r := report{time.Now().UTC().Format(time.RFC3339), 1, "BLS12-381", "PLONK-KZG", []result{{"process-policy-3-2", m.Constraints, m.PublicInputs, m.CompileMillis, m.SRSMillis, m.PlonkSetupMillis, m.SetupMillis, ms(wt), ms(pt), ms(vt), bin, len(sol), int(m.FileBytes["verifying.key"]), len(optimized.Words), optimized.ByteLength, optimized.SHA256, m.FileBytes}}}
	if e = artifact.WriteJSON(filepath.Join(root, "output", "m5-circuit.json"), r); e != nil {
		return e
	}
	if e = writeChecks(root); e != nil {
		return e
	}
	fmt.Printf("process-policy-3-2 constraints=%d public=%d optimized-vk=%dB\n", m.Constraints, m.PublicInputs, optimized.ByteLength)
	return nil
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
func writeChecks(root string) error {
	names := []string{"contracts/src/generated/process-policy-3-2/PlonkVerifier.sol", "contracts/test/fixtures/m5-proofs.json", "artifacts/development/m5/process-policy-3-2/optimized-vk.json"}
	c := checks{time.Now().UTC().Format(time.RFC3339), "SHA-256", map[string]string{}, map[string]string{}}
	for _, n := range names {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		c.Files[n] = v
	}
	for _, n := range []string{"output/m1-core.json", "output/m2-circuit.json", "output/m3-circuit.json", "output/m4-circuit.json", "output/m4-anvil-gas.json", "output/m4-anvil-e2e.json"} {
		v, e := artifact.Checksum(filepath.Join(root, n))
		if e != nil {
			return e
		}
		c.PriorOutputs[n] = v
	}
	return artifact.WriteJSON(filepath.Join(root, "output", "m5-generated-checksums.json"), c)
}
