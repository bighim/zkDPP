package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m6b1case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m6b1run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type solProof interface{ MarshalSolidity() []byte }
type Result struct {
	Feature                                                 string
	Constraints, PublicInputs, MembershipPaths, TreeDepth   int
	CompileMillis, SRSMillis, PlonkSetupMillis              int64
	EncryptMillis, WitnessMillis, ProveMillis, VerifyMillis float64
	ProveAllocatedBytes, HeapBefore, HeapAfter              uint64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactFileBytes                                       map[string]int64
}

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if e := run(*root); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(root string) (err error) {
	attempt, finish, e := m6b1run.Begin(root, "evaluate", "output/m6-b1-circuit.json")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	config, pk, e := auditcrypto.LoadPublic(m6b1run.ArtifactDir(root, "committee"))
	if e != nil {
		return e
	}
	r, e := auditcrypto.RandomScalar(nil)
	if e != nil {
		return e
	}
	s, e := m6b1case.Build(root, pk, r)
	if e != nil {
		return e
	}
	results := []Result{}
	for _, item := range []struct {
		name       string
		assignment frontend.Circuit
		inputs     []fr.Element
	}{
		{m6b1run.Core, s.Core, append([]fr.Element{s.Context, s.Ciphertext.R1.X, s.Ciphertext.R1.Y}, s.Ciphertext.Data...)},
		{m6b1run.Process, s.Process, s.Inputs},
	} {
		if e = m6b1run.CheckBinding(root, config, item.name); e != nil {
			return e
		}
		loaded, e := artifact.LoadAt(root, m6b1run.Milestone, item.name)
		if e != nil {
			return e
		}
		ws := time.Now()
		w, e := frontend.NewWitness(item.assignment, ecc.BLS12_381.ScalarField())
		wt := time.Since(ws)
		if e != nil {
			return e
		}
		pub, e := w.Public()
		if e != nil {
			return e
		}
		got := []fr.Element(pub.Vector().(fr.Vector))
		if !m6b1case.Equal(got, item.inputs) {
			return fmt.Errorf("%s public input order differs", item.name)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		ps := time.Now()
		proof, e := plonk.Prove(loaded.CCS, loaded.PK, w)
		pt := time.Since(ps)
		runtime.ReadMemStats(&after)
		if e != nil {
			return e
		}
		vs := time.Now()
		e = plonk.Verify(proof, loaded.VK, pub)
		vt := time.Since(vs)
		if e != nil {
			return e
		}
		var binary bytes.Buffer
		if _, e = proof.WriteTo(&binary); e != nil {
			return e
		}
		sol, ok := proof.(solProof)
		if !ok {
			return fmt.Errorf("proof has no Solidity serialization")
		}
		solBytes := sol.MarshalSolidity()
		proofPath := m6b1run.ArtifactDir(root, item.name, "proof.bin")
		if e = os.WriteFile(proofPath, binary.Bytes(), 0600); e != nil {
			return e
		}
		reloaded := plonk.NewProof(ecc.BLS12_381)
		f, e := os.Open(proofPath)
		if e != nil {
			return e
		}
		_, readErr := reloaded.ReadFrom(f)
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if e = plonk.Verify(reloaded, loaded.VK, pub); e != nil {
			return fmt.Errorf("proof reload check failed: %w", e)
		}
		m := loaded.Manifest
		b := m6b1run.BindingFor(config, item.name)
		results = append(results, Result{Feature: item.name, Constraints: m.Constraints, PublicInputs: m.PublicInputs, MembershipPaths: b.MembershipPaths, TreeDepth: b.TreeDepth, CompileMillis: m.CompileMillis, SRSMillis: m.SRSMillis, PlonkSetupMillis: m.PlonkSetupMillis, EncryptMillis: s.EncryptMillis, WitnessMillis: m6b1run.Millis(wt), ProveMillis: m6b1run.Millis(pt), VerifyMillis: m6b1run.Millis(vt), ProveAllocatedBytes: after.TotalAlloc - before.TotalAlloc, HeapBefore: before.HeapAlloc, HeapAfter: after.HeapAlloc, BinaryProofBytes: binary.Len(), SolidityProofBytes: len(solBytes), ArtifactFileBytes: m.FileBytes})
		if item.name == m6b1run.Process {
			encoded := make([]string, len(got))
			for i := range got {
				encoded[i] = auditcrypto.EncodeField(got[i])
			}
			fixture := struct {
				Profile           string   `json:"profile"`
				PublicKeyChecksum string   `json:"publicKeyChecksum"`
				Proof             string   `json:"proof"`
				PublicInputs      []string `json:"publicInputs"`
			}{auditcrypto.Profile, config.Checksum, "0x" + hex.EncodeToString(solBytes), encoded}
			if e = artifact.WriteJSON(filepath.Join(root, "contracts", "test", "fixtures", "m6-b1-proofs.json"), fixture); e != nil {
				return e
			}
		}
		fmt.Printf("%s: prove=%.3fms verify=%.3fms constraints=%d\n", item.name, m6b1run.Millis(pt), m6b1run.Millis(vt), m.Constraints)
	}
	out := struct {
		RunCount             int
		Attempt              int
		Environment          m6b1run.Environment
		PublicKeyChecksum    string
		CommitteeSetupMillis float64
		ProofReloadVerified  bool
		Results              []Result
	}{1, attempt, m6b1run.Env(), config.Checksum, config.SetupMillis, true, results}
	return m6b1run.Write(root, "output/m6-b1-circuit.json", out)
}
