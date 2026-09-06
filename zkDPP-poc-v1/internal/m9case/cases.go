package m9case

import (
	"fmt"
	"math/big"
	"path/filepath"

	aentry "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	issue "github.com/bighim/zkDPP/zkDPP-poc-v1/features/issue_claim"
	m8exit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/m8_exit_dpp"
	amerge "github.com/bighim/zkDPP/zkDPP-poc-v1/features/merge"
	aproceed "github.com/bighim/zkDPP/zkDPP-poc-v1/features/proceed"
	aprocess "github.com/bighim/zkDPP/zkDPP-poc-v1/features/process_policy_3_2"
	arecall "github.com/bighim/zkDPP/zkDPP-poc-v1/features/recall"
	asplit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/split"
	atransfer "github.com/bighim/zkDPP/zkDPP-poc-v1/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m7case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m8case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Event struct {
	Case  *m7case.Case
	Epoch uint64
}
type Scenario struct {
	Events                             []Event
	ProductExit, WasteExit             m8case.ExitCase
	Standard, Strict                   m8case.IssueCase
	EntryAluminumA, Product1, Product2 note.Note
	ProcessInputNF                     fr.Element
	FirstVoucherNF                     fr.Element
	FinalNoteRoot, FinalVoucherRoot    fr.Element
	FinalNoteCount, FinalVoucherCount  uint64
}

func Build(root string, pk auditcrypto.PublicKey) (*Scenario, error) {
	actors, e := testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v1.json"))
	if e != nil {
		return nil, e
	}
	supplier, _ := testkit.ByID(actors, "actor-1")
	factory, _ := testkit.ByID(actors, "actor-2")
	component, _ := testkit.ByID(actors, "actor-3")
	noteTree, voucherTree := merkle.New(), merkle.New()
	noteIndex := map[string]uint64{}
	voucherIndex := map[string]uint64{}
	opening := uint64(10001)
	doc := func(product, lot string) (fr.Element, error) {
		return document.Hash(document.DocumentInfo{ProductName: product, LotID: lot})
	}
	mkNote := func(product, lot string, state note.State, role note.AssetRole, address fr.Element) (note.Note, error) {
		d, e := doc(product, lot)
		if e != nil {
			return note.Note{}, e
		}
		v, e := note.New(d, state, role, address, zkhash.Element(opening))
		opening++
		return v, e
	}
	appendNote := func(v note.Note) {
		noteIndex[v.Commitment.String()] = noteTree.Count()
		_, _, _ = noteTree.Append(v.Commitment)
	}
	appendVoucher := func(v voucher.Voucher) {
		voucherIndex[v.Commitment.String()] = voucherTree.Count()
		_, _, _ = voucherTree.Append(v.Commitment)
	}
	s := &Scenario{}
	add := func(name string, k audit.Kind, base frontend.Circuit, sk fr.Element, parents, spends []fr.Element, epoch uint64) error {
		c, e := m7case.New(name, k, base, sk, parents, spends, pk)
		if e != nil {
			return e
		}
		s.Events = append(s.Events, Event{c, epoch})
		return nil
	}
	entries := []struct {
		name  string
		owner testkit.Actor
		state note.State
	}{
		{"aluminum-a", supplier, note.State{QMass: 60e9, ARec: 10e9, E: 40e9}},
		{"aluminum-b", supplier, note.State{QMass: 60e9, ARec: 10e9, E: 50e9}},
		{"cathode", component, note.State{QMass: 100e9, ARec: 10e9, E: 70e9}},
		{"anode", component, note.State{QMass: 100e9, E: 70e9}},
	}
	notes := map[string]note.Note{}
	for _, x := range entries {
		n, e := mkNote("M9 "+x.name, "M9-ENTRY-"+x.name, x.state, note.AssetRoleEligible, x.owner.Address)
		if e != nil {
			return nil, e
		}
		notes[x.name] = n
		if x.name == "aluminum-a" {
			s.EntryAluminumA = n
		}
		if e = add("entry-"+x.name, audit.Entry, aentry.Assignment(n), x.owner.SKOwner, nil, []fr.Element{note.Nullifier(x.owner.SKOwner, n.Commitment)}, 0); e != nil {
			return nil, e
		}
		appendNote(n)
	}
	transfer := func(name string, input note.Note, sender testkit.Actor, epoch, delta uint64) (voucher.Voucher, note.Note, error) {
		vs, cs, rem, e := voucher.Allocate(input.State, input.State.QMass)
		if e != nil {
			return voucher.Voucher{}, note.Note{}, e
		}
		v, e := voucher.New(input.DocumentHash, input.AssetRole, vs, sender.Address, factory.Address, epoch+delta, zkhash.Element(opening))
		opening++
		if e != nil {
			return voucher.Voucher{}, note.Note{}, e
		}
		change, e := note.New(input.DocumentHash, cs, input.AssetRole, sender.Address, zkhash.Element(opening))
		opening++
		if e != nil {
			return voucher.Voucher{}, note.Note{}, e
		}
		p, _ := noteTree.Path(noteIndex[input.Commitment.String()])
		base := atransfer.Assignment(input, sender.SKOwner, noteTree.Root(), p, v, change, rem, epoch, delta)
		if e = add(name, audit.Transfer, base, sender.SKOwner, []fr.Element{input.Commitment}, []fr.Element{voucher.Nullifier(v.Opening, v.Commitment), note.Nullifier(sender.SKOwner, change.Commitment)}, epoch); e != nil {
			return voucher.Voucher{}, note.Note{}, e
		}
		appendVoucher(v)
		appendNote(change)
		return v, change, nil
	}
	proceed := func(name string, v voucher.Voucher) (note.Note, error) {
		out, e := note.New(v.DocumentHash, v.State, v.AssetRole, factory.Address, zkhash.Element(opening))
		opening++
		if e != nil {
			return note.Note{}, e
		}
		p, _ := voucherTree.Path(voucherIndex[v.Commitment.String()])
		base := aproceed.Assignment(v, factory.SKOwner, voucherTree.Root(), p, out)
		if e = add(name, audit.Proceed, base, factory.SKOwner, []fr.Element{v.Commitment}, []fr.Element{note.Nullifier(factory.SKOwner, out.Commitment)}, 0); e != nil {
			return note.Note{}, e
		}
		appendNote(out)
		return out, nil
	}
	recall := func(name string, v voucher.Voucher, sender testkit.Actor, epoch uint64) (note.Note, error) {
		out, e := note.New(v.DocumentHash, v.State, v.AssetRole, sender.Address, zkhash.Element(opening))
		opening++
		if e != nil {
			return note.Note{}, e
		}
		p, _ := voucherTree.Path(voucherIndex[v.Commitment.String()])
		base := arecall.Assignment(v, sender.SKOwner, voucherTree.Root(), p, out, epoch)
		if e = add(name, audit.Recall, base, sender.SKOwner, []fr.Element{v.Commitment}, []fr.Element{note.Nullifier(sender.SKOwner, out.Commitment)}, epoch); e != nil {
			return note.Note{}, e
		}
		appendNote(out)
		return out, nil
	}
	v1, _, e := transfer("transfer-aluminum-a-first", notes["aluminum-a"], supplier, 100, 10)
	if e != nil {
		return nil, e
	}
	s.FirstVoucherNF = voucher.Nullifier(v1.Opening, v1.Commitment)
	returned, e := recall("recall-aluminum-a", v1, supplier, 105)
	if e != nil {
		return nil, e
	}
	v2, _, e := transfer("transfer-aluminum-a-retry", returned, supplier, 106, 10)
	if e != nil {
		return nil, e
	}
	alA, e := proceed("proceed-aluminum-a", v2)
	if e != nil {
		return nil, e
	}
	vB, _, e := transfer("transfer-aluminum-b", notes["aluminum-b"], supplier, 107, 10)
	if e != nil {
		return nil, e
	}
	alB, e := proceed("proceed-aluminum-b", vB)
	if e != nil {
		return nil, e
	}
	vC, _, e := transfer("transfer-cathode", notes["cathode"], component, 108, 10)
	if e != nil {
		return nil, e
	}
	cathode, e := proceed("proceed-cathode", vC)
	if e != nil {
		return nil, e
	}
	vN, _, e := transfer("transfer-anode", notes["anode"], component, 109, 10)
	if e != nil {
		return nil, e
	}
	anode, e := proceed("proceed-anode", vN)
	if e != nil {
		return nil, e
	}
	mergedState, e := allocation.Add(alA.State, alB.State)
	if e != nil {
		return nil, e
	}
	merged, e := mkNote("M9 merged aluminum", "M9-MERGED", mergedState, note.AssetRoleEligible, factory.Address)
	if e != nil {
		return nil, e
	}
	pa, _ := noteTree.Path(noteIndex[alA.Commitment.String()])
	pb, _ := noteTree.Path(noteIndex[alB.Commitment.String()])
	mergeBase := amerge.Assignment(alA, alB, factory.SKOwner, noteTree.Root(), pa, pb, merged)
	if e = add("merge-aluminum", audit.Merge, mergeBase, factory.SKOwner, []fr.Element{alA.Commitment, alB.Commitment}, []fr.Element{note.Nullifier(factory.SKOwner, merged.Commitment)}, 0); e != nil {
		return nil, e
	}
	appendNote(merged)
	inputs := [3]note.Note{merged, cathode, anode}
	var paths [3]merkle.Path
	for i, n := range inputs {
		paths[i], _ = noteTree.Path(noteIndex[n.Commitment.String()])
	}
	res, e := policy.Apply([3]note.State{merged.State, cathode.State, anode.State})
	if e != nil {
		return nil, e
	}
	eligible, e := mkNote("M9 eligible product", "M9-PROCESS-E", res.Eligible, note.AssetRoleEligible, factory.Address)
	if e != nil {
		return nil, e
	}
	waste, e := mkNote("M9 waste", "M9-PROCESS-W", res.Waste, note.AssetRoleWaste, factory.Address)
	if e != nil {
		return nil, e
	}
	processBase := aprocess.Assignment(inputs, factory.SKOwner, noteTree.Root(), paths, [2]note.Note{eligible, waste}, res)
	if e = add("process-product", audit.Process, processBase, factory.SKOwner, []fr.Element{merged.Commitment, cathode.Commitment, anode.Commitment}, []fr.Element{note.Nullifier(factory.SKOwner, eligible.Commitment), note.Nullifier(factory.SKOwner, waste.Commitment)}, 0); e != nil {
		return nil, e
	}
	s.ProcessInputNF = note.Nullifier(factory.SKOwner, merged.Commitment)
	appendNote(eligible)
	appendNote(waste)
	first, second, rem, e := allocation.ByMass(eligible.State, 200e9)
	if e != nil {
		return nil, e
	}
	product1, e := mkNote("M9 product 1", "M9-PRODUCT-1", first, note.AssetRoleEligible, factory.Address)
	if e != nil {
		return nil, e
	}
	product2, e := mkNote("M9 product 2", "M9-PRODUCT-2", second, note.AssetRoleEligible, factory.Address)
	if e != nil {
		return nil, e
	}
	pp, _ := noteTree.Path(noteIndex[eligible.Commitment.String()])
	splitBase := asplit.Assignment(eligible, factory.SKOwner, noteTree.Root(), pp, product1, product2, rem)
	if e = add("split-product", audit.Split, splitBase, factory.SKOwner, []fr.Element{eligible.Commitment}, []fr.Element{note.Nullifier(factory.SKOwner, product1.Commitment), note.Nullifier(factory.SKOwner, product2.Commitment)}, 0); e != nil {
		return nil, e
	}
	appendNote(product1)
	appendNote(product2)
	s.Product1 = product1
	s.Product2 = product2
	makeExit := func(name string, n note.Note, dppOpening, rnd uint64) (m8case.ExitCase, error) {
		data, e := dpp.New(n.DocumentHash, n.AssetRole, n.State, zkhash.Element(dppOpening))
		if e != nil {
			return m8case.ExitCase{}, e
		}
		nf := note.Nullifier(factory.SKOwner, n.Commitment)
		base := []fr.Element{noteTree.Root(), nf, data.Commitment}
		context := zkhash.Hash(auditcrypto.AuditContextTag, zkhash.Element(m8case.EventExit), base[0], base[1], base[2])
		r := new(big.Int).SetUint64(rnd)
		ct, e := auditcrypto.Encrypt(pk, context, []fr.Element{n.Commitment}, r)
		if e != nil {
			return m8case.ExitCase{}, e
		}
		p, _ := noteTree.Path(noteIndex[n.Commitment.String()])
		assignment := m8exit.Assignment(pk, n, factory.SKOwner, noteTree.Root(), p, data, r, ct)
		public := append(append([]fr.Element{}, base...), ct.R1.X, ct.R1.Y, ct.Data[0])
		return m8case.ExitCase{Name: name, Definition: m8exit.New(pk), Assignment: assignment, Data: data, Context: context, Ciphertext: ct, Randomness: r, PublicInputs: public, ParentCM: n.Commitment}, nil
	}
	s.ProductExit, e = makeExit("exit-product-1", product1, 9901, 19901)
	if e != nil {
		return nil, e
	}
	s.WasteExit, e = makeExit("exit-waste", waste, 9902, 19902)
	if e != nil {
		return nil, e
	}
	std, strict := issuepolicy.Standard(), issuepolicy.Strict()
	s.Standard = m8case.IssueCase{Name: std.Name, Config: std, Definition: issue.New(std), Assignment: issue.Assignment(std, s.ProductExit.Data), Data: s.ProductExit.Data, PublicInputs: []fr.Element{std.PolicyRef, s.ProductExit.Data.Commitment}}
	s.Strict = m8case.IssueCase{Name: strict.Name, Config: strict, Definition: issue.New(strict), Assignment: issue.Assignment(strict, s.ProductExit.Data), Data: s.ProductExit.Data, PublicInputs: []fr.Element{strict.PolicyRef, s.ProductExit.Data.Commitment}}
	s.FinalNoteRoot = noteTree.Root()
	s.FinalVoucherRoot = voucherTree.Root()
	s.FinalNoteCount = noteTree.Count()
	s.FinalVoucherCount = voucherTree.Count()
	return s, nil
}

func (s *Scenario) Check() error {
	want1 := note.State{QMass: 200e9, ARec: 22222222223, E: 192592592593}
	want2 := note.State{QMass: 70e9, ARec: 7777777777, E: 67407407407}
	if s.Product1.State != want1 || s.Product2.State != want2 {
		return fmt.Errorf("split state mismatch: %v %v", s.Product1.State, s.Product2.State)
	}
	return nil
}
