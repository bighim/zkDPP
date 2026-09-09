package v2case

import (
	"fmt"
	entry "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	merge "github.com/bighim/zkDPP/zkDPP-poc-v2/features/merge"
	spend "github.com/bighim/zkDPP/zkDPP-poc-v2/features/private_spend"
	proceed "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	process "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	recall "github.com/bighim/zkDPP/zkDPP-poc-v2/features/recall"
	split "github.com/bighim/zkDPP/zkDPP-poc-v2/features/split"
	transfer "github.com/bighim/zkDPP/zkDPP-poc-v2/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2circuit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"math/big"
	"path/filepath"
	"reflect"
)

type Event struct {
	Name, Relation string
	Kind           v2audit.Kind
	Assignment     frontend.Circuit
	Public         []fr.Element
	Record         v2audit.Record
}
type Group struct {
	Name                    string
	Events, Extra           []Event
	Start                   v2audit.Ref
	NoteRoot, VoucherRoot   fr.Element
	NoteCount, VoucherCount uint64
	ClaimValues             []claim.DPPClaim
	Document                document.DocumentInfo
}
type builder struct {
	pk              auditcrypto.PublicKey
	notes, vouchers *merkle.Tree
	ni, vi          map[string]uint64
	serial          uint64
	group           Group
}

func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}
func newBuilder(pk auditcrypto.PublicKey, name string) *builder {
	return &builder{pk: pk, notes: merkle.New(), vouchers: merkle.New(), ni: map[string]uint64{}, vi: map[string]uint64{}, serial: 60000, group: Group{Name: name}}
}
func (b *builder) n(doc fr.Element, s note.State, role note.AssetRole, addr fr.Element) note.Note {
	b.serial++
	return must(note.New(doc, s, role, addr, h.Element(b.serial)))
}
func (b *builder) append(n note.Note) {
	b.ni[n.Commitment.String()] = b.notes.Count()
	_, _, e := b.notes.Append(n.Commitment)
	if e != nil {
		panic(e)
	}
}
func (b *builder) path(n note.Note) merkle.Path {
	return must(b.notes.Path(b.ni[n.Commitment.String()]))
}
func (b *builder) seal(k v2audit.Kind, relation string, c frontend.Circuit, parents, outputs []fr.Element) Event {
	b.serial++
	r := big.NewInt(int64(b.serial))
	ct := must(auditcrypto.EncryptWithMasterPublicKey(b.pk, append(append([]fr.Element{}, parents...), outputs...), r))
	a := v2circuit.NewAuditWitness(len(parents), len(outputs))
	a.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
	a.Randomness = r
	for i := range parents {
		a.Parents[i] = ct.Data[i]
	}
	for i := range outputs {
		a.OutputNfs[i] = ct.Data[len(parents)+i]
	}
	reflect.ValueOf(c).Elem().FieldByName("Audit").Set(reflect.ValueOf(a))
	w := must(frontend.NewWitness(c, ecc.BLS12_381.ScalarField()))
	pub := must(w.Public())
	fields := []fr.Element(pub.Vector().(fr.Vector))
	layout := must(v2audit.Shape(k))
	record := must(v2audit.NewRecord(k, fields[:layout.BaseCount], ct))
	ev := Event{Name: fmt.Sprintf("%s-%02d", relation, len(b.group.Events)), Relation: relation, Kind: k, Assignment: c, Public: fields, Record: record}
	b.group.Events = append(b.group.Events, ev)
	return ev
}
func (b *builder) entry(n note.Note, a testkit.Actor) {
	c := v2events.NewEntry(b.pk)
	c.Base = *entry.Assignment(n)
	c.SKOwner = a.SKOwner
	b.seal(v2audit.Entry, "entry", c, nil, []fr.Element{note.Nullifier(a.SKOwner, n.Commitment)})
	b.append(n)
}
func (b *builder) transfer(n note.Note, a, to testkit.Actor, delta uint64) voucher.Voucher {
	vs, cs, rem, e := voucher.AllocateWithTransport(n.State, n.State.QMass, delta)
	if e != nil {
		panic(e)
	}
	b.serial++
	v := must(voucher.New(n.DocumentHash, n.AssetRole, vs, a.Address, to.Address, 100000, h.Element(b.serial)))
	change := b.n(n.DocumentHash, cs, n.AssetRole, a.Address)
	base := transfer.Assignment(n, a.SKOwner, b.notes.Root(), b.path(n), v, change, rem, 0, v.DeadlineBlock)
	c := v2events.NewTransfer(b.pk)
	c.NoteRoot, c.NF, c.RVNew, c.CMChange, c.D = base.NoteRoot, base.NF, base.RVNew, base.CMChange, v.DeadlineBlock
	c.SenderSecret, c.Input, c.InputPath, c.Voucher, c.Change = base.SenderSecret, base.Input, base.InputPath, base.Voucher, base.Change
	c.RemainderA, c.RemainderE, c.DeltaTransport = rem.ARec, rem.E, delta
	b.seal(v2audit.Transfer, "transfer", c, []fr.Element{n.Commitment}, []fr.Element{voucher.Nullifier(v.Opening, v.Commitment), note.Nullifier(a.SKOwner, change.Commitment)})
	b.append(change)
	b.vi[v.Commitment.String()] = b.vouchers.Count()
	_, _, e = b.vouchers.Append(v.Commitment)
	if e != nil {
		panic(e)
	}
	return v
}
func (b *builder) resolve(v voucher.Voucher, a testkit.Actor, isRecall bool) note.Note {
	n := b.n(v.DocumentHash, v.State, v.AssetRole, a.Address)
	path := must(b.vouchers.Path(b.vi[v.Commitment.String()]))
	var c frontend.Circuit
	k := v2audit.Proceed
	if isRecall {
		base := recall.Assignment(v, a.SKOwner, b.vouchers.Root(), path, n, 0)
		x := v2events.NewRecall(b.pk)
		x.VoucherRoot, x.RVNF, x.CMReturn, x.D = base.VoucherRoot, base.RVNF, base.CMReturn, v.DeadlineBlock
		x.SenderSecret, x.Voucher, x.VoucherPath, x.Output = base.SenderSecret, base.Voucher, base.VoucherPath, base.Output
		c = x
		k = v2audit.Recall
	} else {
		x := v2events.NewProceed(b.pk)
		x.Base = *proceed.Assignment(v, a.SKOwner, b.vouchers.Root(), path, n)
		c = x
	}
	b.seal(k, k.String(), c, []fr.Element{v.Commitment}, []fr.Element{note.Nullifier(a.SKOwner, n.Commitment)})
	b.append(n)
	return n
}
func (b *builder) merge(a, c note.Note, owner testkit.Actor) note.Note {
	n := b.n(a.DocumentHash, must(allocation.Add(a.State, c.State)), note.AssetRoleEligible, owner.Address)
	x := v2events.NewMerge(b.pk)
	x.Base = *merge.Assignment(a, c, owner.SKOwner, b.notes.Root(), b.path(a), b.path(c), n)
	b.seal(v2audit.Merge, "merge", x, []fr.Element{a.Commitment, c.Commitment}, []fr.Element{note.Nullifier(owner.SKOwner, n.Commitment)})
	b.append(n)
	return n
}
func (b *builder) split(n note.Note, owner testkit.Actor, q uint64) (note.Note, note.Note) {
	s1, s2, rem, e := allocation.ByMass(n.State, q)
	if e != nil {
		panic(e)
	}
	a, c := b.n(n.DocumentHash, s1, n.AssetRole, owner.Address), b.n(n.DocumentHash, s2, n.AssetRole, owner.Address)
	x := v2events.NewSplit(b.pk)
	x.Base = *split.Assignment(n, owner.SKOwner, b.notes.Root(), b.path(n), a, c, rem)
	b.seal(v2audit.Split, "split", x, []fr.Element{n.Commitment}, []fr.Element{note.Nullifier(owner.SKOwner, a.Commitment), note.Nullifier(owner.SKOwner, c.Commitment)})
	b.append(a)
	b.append(c)
	return a, c
}
func (b *builder) issue(n note.Note, a testkit.Actor, cfg issuepolicy.Config) {
	x := v2events.NewIssue(b.pk, cfg)
	b.serial++
	nonce := h.Element(b.serial)
	handle := claim.Handle(n.DocumentHash, cfg.PolicyRef, nonce)
	x.PolicyRef, x.NoteRoot, x.NF, x.H = cfg.PolicyRef, b.notes.Root(), note.Nullifier(a.SKOwner, n.Commitment), handle
	x.SKOwner, x.Note, x.Path, x.ClaimNonce = a.SKOwner, noteWitness(n), circuitPath(b.path(n)), nonce
	b.seal(v2audit.Issue, cfg.Name, x, []fr.Element{n.Commitment}, nil)
	b.group.ClaimValues = append(b.group.ClaimValues, claim.DPPClaim{IssuePolicyRef: cfg.PolicyRef, Handle: handle, ClaimNonce: nonce})
}
func (b *builder) exit(n note.Note, a testkit.Actor) {
	x := v2events.NewExit(b.pk)
	x.Base = *spend.Assignment(n, a.SKOwner, b.notes.Root(), b.path(n))
	b.seal(v2audit.Exit, "exit", x, []fr.Element{n.Commitment}, nil)
}
func (b *builder) finish() Group {
	b.group.NoteRoot, b.group.VoucherRoot = b.notes.Root(), b.vouchers.Root()
	b.group.NoteCount, b.group.VoucherCount = b.notes.Count(), b.vouchers.Count()
	return b.group
}

func BuildHotfix(root string) (groups []Group, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("scenario: %v", v)
		}
	}()
	pkg, _, e := KeyFixture()
	if e != nil {
		return nil, e
	}
	actors := must(testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v2.json")))
	supplier, factory, component := actors[0], actors[1], actors[2]
	b := newBuilder(pkg.PublicKey, "lifecycle")
	alDoc := must(document.Hash(document.DocumentInfo{ProductName: "Aluminum", LotID: "AL", Unit: "kg"}))
	catDoc := must(document.Hash(document.DocumentInfo{ProductName: "Cathode", LotID: "CAT", Unit: "kg"}))
	anDoc := must(document.Hash(document.DocumentInfo{ProductName: "Anode", LotID: "AN", Unit: "kg"}))
	inputs := []note.Note{b.n(alDoc, note.State{QMass: 60e9, ARec: 10e9, E: 40e9}, 0, supplier.Address), b.n(alDoc, note.State{QMass: 60e9, ARec: 10e9, E: 50e9}, 0, supplier.Address), b.n(catDoc, note.State{QMass: 100e9, ARec: 10e9, E: 70e9}, 0, component.Address), b.n(anDoc, note.State{QMass: 100e9, E: 70e9}, 0, component.Address)}
	for i, n := range inputs {
		a := supplier
		if i > 1 {
			a = component
		}
		b.entry(n, a)
	}
	b.group.Start = v2audit.Ref{ObjectType: v2audit.Note, RawID: inputs[0].Commitment}
	first := b.transfer(inputs[0], supplier, factory, 1e9)
	returned := b.resolve(first, supplier, true)
	alA := b.resolve(b.transfer(returned, supplier, factory, 0), factory, false)
	alB := b.resolve(b.transfer(inputs[1], supplier, factory, 0), factory, false)
	cath := b.resolve(b.transfer(inputs[2], component, factory, 0), factory, false)
	an := b.resolve(b.transfer(inputs[3], component, factory, 0), factory, false)
	merged := b.merge(alA, alB, factory)
	ps := [3]note.Note{merged, cath, an}
	result := must(policy.Apply([3]note.State{merged.State, cath.State, an.State}))
	if result.Eligible != (note.State{QMass: 270e9, ARec: 30e9, E: 261e9}) {
		return nil, fmt.Errorf("Process totals")
	}
	b.group.Document = document.DocumentInfo{ProductName: "Battery", LotID: "HF-PRODUCT", Unit: "kg"}
	doc := must(document.Hash(b.group.Document))
	eligible, waste := b.n(doc, result.Eligible, 0, factory.Address), b.n(doc, result.Waste, 1, factory.Address)
	x := v2events.NewProcess(pkg.PublicKey)
	x.Base = *process.Assignment(ps, factory.SKOwner, b.notes.Root(), [3]merkle.Path{b.path(merged), b.path(cath), b.path(an)}, [2]note.Note{eligible, waste}, result)
	b.seal(v2audit.Process, "process", x, []fr.Element{merged.Commitment, cath.Commitment, an.Commitment}, []fr.Element{note.Nullifier(factory.SKOwner, eligible.Commitment), note.Nullifier(factory.SKOwner, waste.Commitment)})
	b.append(eligible)
	b.append(waste)
	p1, p2 := b.split(eligible, factory, 200e9)
	b.issue(p1, factory, issuepolicy.Standard())
	b.issue(p2, factory, issuepolicy.Strict())
	b.exit(waste, factory)
	groups = append(groups, b.finish())
	b = newBuilder(pkg.PublicKey, "audit")
	b.serial = 160000
	a := b.n(doc, note.State{QMass: 100e9, ARec: 20e9, E: 50e9}, 0, factory.Address)
	b.entry(a, factory)
	bb, c := b.split(a, factory, 60e9)
	d, ee := b.split(bb, factory, 30e9)
	b.group.Start = v2audit.Ref{ObjectType: 1, RawID: a.Commitment}
	g := b.finish()
	b.exit(c, factory)
	b.issue(d, factory, issuepolicy.Standard())
	b.exit(ee, factory)
	g.Extra = append([]Event{}, b.group.Events[len(g.Events):]...)
	groups = append(groups, g)
	seenPoints := map[string]bool{}
	for _, g := range groups {
		for _, ev := range append(append([]Event{}, g.Events...), g.Extra...) {
			point := ev.Record.R1.X.String() + ":" + ev.Record.R1.Y.String()
			if seenPoints[point] {
				return nil, fmt.Errorf("audit randomness reused across scenarios")
			}
			seenPoints[point] = true
		}
	}
	return groups, nil
}
