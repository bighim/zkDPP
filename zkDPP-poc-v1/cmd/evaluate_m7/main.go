package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	b1 "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6b1run"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m7case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m7run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"os"
	"runtime"
	"time"
)

type solidityProof interface{ MarshalSolidity() []byte }
type Measurement struct {
	Event, Case                                             string
	Constraints, PublicInputs, MembershipPaths              int
	SetupReused                                             bool
	SetupSource                                             string
	CompileMillis, SRSMillis, PlonkSetupMillis              *int64
	EncryptMillis, WitnessMillis, ProveMillis, VerifyMillis float64
	ProveAllocatedBytes, HeapBefore, HeapAfter              uint64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactBytes                                           map[string]int64
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
	attempt, finish, e := m7run.Begin(root, "evaluate", "output/m7-circuit.json")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, e := m7run.Committee(root)
	if e != nil {
		return e
	}
	groups, e := m7case.Build(root, committee.PK)
	if e != nil {
		return e
	}
	fixture := m7run.Fixture{Profile: auditcrypto.Profile, PublicKeyChecksum: committee.Public.Checksum}
	seen := map[audit.Kind]bool{}
	results := []Measurement{}
	counts := map[string]int{}
	for _, g := range groups {
		fixed := m7run.FixedGroup{Name: g.Name}
		for i, c := range append(append([]*m7case.Case{}, g.Events...), g.Extra...) {
			if e = m7run.CheckBinding(root, committee.Public, c.Kind); e != nil {
				return e
			}
			loaded, e := m7run.Load(root, c.Kind)
			if e != nil {
				return e
			}
			start := time.Now()
			w, e := frontend.NewWitness(c.Assignment, ecc.BLS12_381.ScalarField())
			wt := m7run.Millis(time.Since(start))
			if e != nil {
				return e
			}
			public, e := w.Public()
			if e != nil {
				return e
			}
			if !audit.Equal([]fr.Element(public.Vector().(fr.Vector)), c.Inputs) {
				return fmt.Errorf("public ordering %s", c.Name)
			}
			representative := !seen[c.Kind]
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			start = time.Now()
			proof, e := plonk.Prove(loaded.CCS, loaded.PK, w)
			pt := m7run.Millis(time.Since(start))
			runtime.ReadMemStats(&after)
			if e != nil {
				return e
			}
			counts[c.Kind.String()]++
			start = time.Now()
			e = plonk.Verify(proof, loaded.VK, public)
			vt := m7run.Millis(time.Since(start))
			if e != nil {
				return e
			}
			var binary bytes.Buffer
			if _, e = proof.WriteTo(&binary); e != nil {
				return e
			}
			sol, ok := proof.(solidityProof)
			if !ok {
				return fmt.Errorf("Solidity marshal unsupported")
			}
			encoded := sol.MarshalSolidity()
			path := m7run.Dir(root, "proofs", g.Name+"-"+c.Name+".bin")
			if e = os.MkdirAll(m7run.Dir(root, "proofs"), 0700); e != nil {
				return e
			}
			if e = os.WriteFile(path, binary.Bytes(), 0600); e != nil {
				return e
			}
			disk, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			reload := plonk.NewProof(ecc.BLS12_381)
			if _, e = reload.ReadFrom(bytes.NewReader(disk)); e != nil {
				return e
			}
			if e = plonk.Verify(reload, loaded.VK, public); e != nil {
				return e
			}
			fc := m7run.FixedCase{Name: c.Name, Kind: c.Kind, Proof: "0x" + hex.EncodeToString(encoded), Representative: representative}
			for _, v := range c.Inputs {
				fc.PublicInputs = append(fc.PublicInputs, auditcrypto.EncodeField(v))
			}
			if i < len(g.Events) {
				fixed.Events = append(fixed.Events, fc)
			} else {
				fixed.Extra = append(fixed.Extra, fc)
			}
			if representative {
				m := loaded.Manifest
				l, _ := audit.Shape(c.Kind)
				row := Measurement{Event: c.Kind.String(), Case: g.Name + "/" + c.Name, Constraints: m.Constraints, PublicInputs: m.PublicInputs, MembershipPaths: l.ParentCount, SetupReused: c.Kind == audit.Process, SetupSource: "m7", EncryptMillis: c.EncryptMillis, WitnessMillis: wt, ProveMillis: pt, VerifyMillis: vt, ProveAllocatedBytes: after.TotalAlloc - before.TotalAlloc, HeapBefore: before.HeapAlloc, HeapAfter: after.HeapAlloc, BinaryProofBytes: binary.Len(), SolidityProofBytes: len(encoded), ArtifactBytes: m.FileBytes}
				if c.Kind == audit.Process {
					row.SetupSource = "m6-b1/audit-process-3-2"
				} else {
					row.CompileMillis = &m.CompileMillis
					row.SRSMillis = &m.SRSMillis
					row.PlonkSetupMillis = &m.PlonkSetupMillis
				}
				results = append(results, row)
				seen[c.Kind] = true
			}
			fmt.Printf("%s/%s representative=%t prove=%.3fms\n", g.Name, c.Name, representative, pt)
		}
		fixture.Groups = append(fixture.Groups, fixed)
	}
	if len(results) != 8 {
		return fmt.Errorf("missing representative Event")
	}
	if e = m7run.Write(root, "contracts/test/fixtures/m7-proofs.json", fixture); e != nil {
		return e
	}
	return m7run.Write(root, "output/m7-circuit.json", struct {
		RunCount, Attempt   int
		Environment         b1.Environment
		PublicKeyChecksum   string
		Results             []Measurement
		TotalProofCalls     map[string]int
		ProofReloadVerified bool
	}{1, attempt, b1.Env(), committee.Public.Checksum, results, counts, true})
}
