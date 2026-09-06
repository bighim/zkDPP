package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m8case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m8run"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/consensys/gnark/frontend"
)

type solidityProof interface{ MarshalSolidity() []byte }
type measurement struct {
	Relation                                                string
	Constraints, PublicInputs                               int
	CompileMillis, SRSMillis, PlonkSetupMillis              int64
	EncryptMillis, WitnessMillis, ProveMillis, VerifyMillis float64
	ProveAllocatedBytes, HeapBefore, HeapAfter              uint64
	BinaryProofBytes, SolidityProofBytes                    int
	ArtifactBytes                                           map[string]int64
	OptimizedVKBytes                                        int    `json:",omitempty"`
	VKHash                                                  string `json:",omitempty"`
}

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string) (runErr error) {
	attempt, finish, err := m8run.Begin(root, "evaluate", "output/m8-circuit.json")
	if err != nil {
		return err
	}
	defer func() { finish(runErr) }()
	if err = m8run.CheckBaseline(root); err != nil {
		return err
	}
	committee, err := m8run.Committee(root)
	if err != nil {
		return err
	}
	s, err := m8case.Build(root, committee.PK)
	if err != nil {
		return err
	}
	m7sum, err := artifact.Checksum(root + "/contracts/test/fixtures/m7-proofs.json")
	if err != nil {
		return err
	}
	fixture := m8run.Fixture{Profile: auditcrypto.Profile, PublicKeyChecksum: committee.Public.Checksum, M7FixtureChecksum: m7sum}
	type row struct {
		name, artifact         string
		definition, assignment frontend.Circuit
		inputs                 []fr.Element
		encrypt                float64
		target                 *m8run.FixedProof
	}
	items := []row{
		{s.EligibleExit.Name, "exit-dpp", s.EligibleExit.Definition, s.EligibleExit.Assignment, s.EligibleExit.PublicInputs, s.EligibleExit.EncryptMillis, &fixture.EligibleExit},
		{s.WasteExit.Name, "exit-dpp", s.WasteExit.Definition, s.WasteExit.Assignment, s.WasteExit.PublicInputs, s.WasteExit.EncryptMillis, &fixture.WasteExit},
		{s.Standard.Name, s.Standard.Name, s.Standard.Definition, s.Standard.Assignment, s.Standard.PublicInputs, 0, &fixture.Standard},
		{s.Strict.Name, s.Strict.Name, s.Strict.Definition, s.Strict.Assignment, s.Strict.PublicInputs, 0, &fixture.Strict},
	}
	seen := map[string]bool{}
	results := []measurement{}
	calls := map[string]int{}
	for _, item := range items {
		loaded, err := m8run.Load(root, item.artifact)
		if err != nil {
			return err
		}
		start := time.Now()
		w, err := frontend.NewWitness(item.assignment, ecc.BLS12_381.ScalarField())
		wt := m8run.Millis(time.Since(start))
		if err != nil {
			return err
		}
		public, err := w.Public()
		if err != nil {
			return err
		}
		got := []fr.Element(public.Vector().(fr.Vector))
		if !equal(got, item.inputs) {
			return fmt.Errorf("public input order: %s", item.name)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		start = time.Now()
		proof, err := plonk.Prove(loaded.CCS, loaded.PK, w)
		pt := m8run.Millis(time.Since(start))
		runtime.ReadMemStats(&after)
		if err != nil {
			return err
		}
		start = time.Now()
		err = plonk.Verify(proof, loaded.VK, public)
		vt := m8run.Millis(time.Since(start))
		if err != nil {
			return err
		}
		var binary bytes.Buffer
		if _, err = proof.WriteTo(&binary); err != nil {
			return err
		}
		sol := proof.(solidityProof).MarshalSolidity()
		reload := plonk.NewProof(ecc.BLS12_381)
		if _, err = reload.ReadFrom(bytes.NewReader(binary.Bytes())); err != nil {
			return err
		}
		if err = plonk.Verify(reload, loaded.VK, public); err != nil {
			return err
		}
		fixed := m8run.FixedProof{Name: item.name, Artifact: item.artifact, Proof: "0x" + hex.EncodeToString(sol)}
		for _, v := range item.inputs {
			fixed.PublicInputs = append(fixed.PublicInputs, auditcrypto.EncodeField(v))
		}
		*item.target = fixed
		calls[item.artifact]++
		if !seen[item.artifact] {
			m := loaded.Manifest
			result := measurement{Relation: item.artifact, Constraints: m.Constraints, PublicInputs: m.PublicInputs, CompileMillis: m.CompileMillis, SRSMillis: m.SRSMillis, PlonkSetupMillis: m.PlonkSetupMillis, EncryptMillis: item.encrypt, WitnessMillis: wt, ProveMillis: pt, VerifyMillis: vt, ProveAllocatedBytes: after.TotalAlloc - before.TotalAlloc, HeapBefore: before.HeapAlloc, HeapAfter: after.HeapAlloc, BinaryProofBytes: binary.Len(), SolidityProofBytes: len(sol), ArtifactBytes: m.FileBytes}
			if item.artifact != "exit-dpp" {
				vk := loaded.VK.(*plonkbls.VerifyingKey)
				optimized, e := solgen.ExtractOptimizedVK(vk)
				if e != nil {
					return e
				}
				result.OptimizedVKBytes = optimized.ByteLength
				result.VKHash = optimized.SHA256
				if item.artifact == s.Standard.Name {
					fixture.StandardVKHash = optimized.SHA256
				} else {
					fixture.StrictVKHash = optimized.SHA256
				}
			}
			results = append(results, result)
			seen[item.artifact] = true
		}
	}
	if err = m8run.Write(root, "contracts/test/fixtures/m8-proofs.json", fixture); err != nil {
		return err
	}
	return m8run.Write(root, "output/m8-circuit.json", struct {
		RunCount, Attempt   int
		PublicKeyChecksum   string
		Results             []measurement
		TotalProofCalls     map[string]int
		ProofReloadVerified bool
	}{1, attempt, committee.Public.Checksum, results, calls, true})
}

func equal(a, b []fr.Element) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(&b[i]) {
			return false
		}
	}
	return true
}
