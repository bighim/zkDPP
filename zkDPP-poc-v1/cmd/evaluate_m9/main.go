package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/finalsrs"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m9case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m9run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
)

type solidityProof interface{ MarshalSolidity() []byte }

func main() {
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	if e := run(*root); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func relation(kind uint8) string {
	names := []string{"audit-entry", "audit-transfer", "audit-proceed", "audit-recall", "audit-merge", "audit-split", "audit-process-3-2"}
	return names[kind]
}
func prove(root, name string, definition, assignment frontend.Circuit, inputs []fr.Element) (m9run.FixedProof, error) {
	loaded, e := finalsrs.Load(root, name)
	if e != nil {
		return m9run.FixedProof{}, e
	}
	w, e := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	if e != nil {
		return m9run.FixedProof{}, e
	}
	pub, e := w.Public()
	if e != nil {
		return m9run.FixedProof{}, e
	}
	got := []fr.Element(pub.Vector().(fr.Vector))
	if len(got) != len(inputs) {
		return m9run.FixedProof{}, fmt.Errorf("public count %s", name)
	}
	for i := range got {
		if !got[i].Equal(&inputs[i]) {
			return m9run.FixedProof{}, fmt.Errorf("public order %s", name)
		}
	}
	proof, e := plonk.Prove(loaded.CCS, loaded.PK, w)
	if e != nil {
		return m9run.FixedProof{}, e
	}
	if e = plonk.Verify(proof, loaded.VK, pub); e != nil {
		return m9run.FixedProof{}, e
	}
	fixed := m9run.FixedProof{Relation: name, Proof: "0x" + hex.EncodeToString(proof.(solidityProof).MarshalSolidity())}
	for _, v := range inputs {
		fixed.PublicInputs = append(fixed.PublicInputs, auditcrypto.EncodeField(v))
	}
	return fixed, nil
}
func run(root string) (runErr error) {
	attempt, finish, e := m9run.Begin(root, "evaluate", "contracts/test/fixtures/m9-proofs.json")
	if e != nil {
		return e
	}
	defer func() { finish(runErr) }()
	committee, e := m9run.Committee(root)
	if e != nil {
		return e
	}
	s, e := m9case.Build(root, committee.PK)
	if e != nil {
		return e
	}
	if e = s.Check(); e != nil {
		return e
	}
	f := m9run.Fixture{Profile: auditcrypto.Profile, PublicKeyChecksum: committee.Public.Checksum, EntryAluminumA: auditcrypto.EncodeField(s.EntryAluminumA.Commitment), Product1: auditcrypto.EncodeField(s.Product1.Commitment), Product2: auditcrypto.EncodeField(s.Product2.Commitment), FirstVoucherNF: auditcrypto.EncodeField(s.FirstVoucherNF), ProcessInputNF: auditcrypto.EncodeField(s.ProcessInputNF), FinalNoteRoot: auditcrypto.EncodeField(s.FinalNoteRoot), FinalVoucherRoot: auditcrypto.EncodeField(s.FinalVoucherRoot), FinalNoteCount: s.FinalNoteCount, FinalVoucherCount: s.FinalVoucherCount}
	for _, ev := range s.Events {
		fixed, e := prove(root, relation(uint8(ev.Case.Kind)), ev.Case.Definition, ev.Case.Assignment, ev.Case.Inputs)
		if e != nil {
			return e
		}
		fixed.Name = ev.Case.Name
		fixed.EventKind = uint8(ev.Case.Kind)
		fixed.Epoch = ev.Epoch
		f.Events = append(f.Events, fixed)
	}
	f.ProductExit, e = prove(root, "exit-dpp", s.ProductExit.Definition, s.ProductExit.Assignment, s.ProductExit.PublicInputs)
	if e != nil {
		return e
	}
	f.ProductExit.Name = s.ProductExit.Name
	f.ProductExit.EventKind = 7
	f.WasteExit, e = prove(root, "exit-dpp", s.WasteExit.Definition, s.WasteExit.Assignment, s.WasteExit.PublicInputs)
	if e != nil {
		return e
	}
	f.WasteExit.Name = s.WasteExit.Name
	f.WasteExit.EventKind = 7
	f.Standard, e = prove(root, s.Standard.Name, s.Standard.Definition, s.Standard.Assignment, s.Standard.PublicInputs)
	if e != nil {
		return e
	}
	f.Standard.Name = s.Standard.Name
	f.Standard.EventKind = 8
	f.Strict, e = prove(root, s.Strict.Name, s.Strict.Definition, s.Strict.Assignment, s.Strict.PublicInputs)
	if e != nil {
		return e
	}
	f.Strict.Name = s.Strict.Name
	f.Strict.EventKind = 8
	_ = attempt
	return m9run.Write(root, "contracts/test/fixtures/m9-proofs.json", f)
}
