package v2case

import (
	"fmt"
	process "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	transfer "github.com/bighim/zkDPP/zkDPP-poc-v2/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"path/filepath"
)

// Expectations are annotated from explicit fixture operations, never from audit
// decryption, TraceForward, or AuditAndFreeze return values.
type IntegratedObject struct {
	Label              string
	Ref                v2audit.Ref
	Spend              [32]byte
	State              note.State
	Role               note.AssetRole
	Producer, Consumer uint64
}
type IntegratedStep struct {
	Name            string
	Inputs, Outputs []string
	Aid             uint64
}
type IntegratedExpected struct {
	Objects []IntegratedObject
	Steps   []IntegratedStep
	Targets []string
	Start   string
}
type IntegratedFixture struct {
	Base     Group
	Probes   []Event
	Expected IntegratedExpected
}
type integratedBase struct {
	b                 *builder
	factory, receiver testkit.Actor
	notes             map[string]note.Note
	vouchers          map[string]voucher.Voucher
	expected          IntegratedExpected
}

func (x *integratedBase) addNote(label string, n note.Note, a testkit.Actor) {
	x.notes[label] = n
	spend := note.Nullifier(a.SKOwner, n.Commitment)
	x.expected.Objects = append(x.expected.Objects, IntegratedObject{Label: label, Ref: v2audit.Ref{ObjectType: v2audit.Note, RawID: n.Commitment}, Spend: spend.Bytes(), State: n.State, Role: n.AssetRole})
}
func (x *integratedBase) addVoucher(label string, v voucher.Voucher) {
	x.vouchers[label] = v
	spend := voucher.Nullifier(v.Opening, v.Commitment)
	x.expected.Objects = append(x.expected.Objects, IntegratedObject{Label: label, Ref: v2audit.Ref{ObjectType: v2audit.Voucher, RawID: v.Commitment}, Spend: spend.Bytes(), State: v.State, Role: v.AssetRole})
}
func (x *integratedBase) step(name string, inputs, outputs []string) {
	aid := uint64(len(x.b.group.Events))
	x.b.group.Events[aid-1].Name = name
	x.expected.Steps = append(x.expected.Steps, IntegratedStep{name, inputs, outputs, aid})
	for i := range x.expected.Objects {
		for _, l := range inputs {
			if x.expected.Objects[i].Label == l {
				x.expected.Objects[i].Consumer = aid
			}
		}
		for _, l := range outputs {
			if x.expected.Objects[i].Label == l {
				x.expected.Objects[i].Producer = aid
			}
		}
	}
}
func integratedProcess(b *builder, ns [3]note.Note, a testkit.Actor) (note.Note, note.Note) {
	result := must(policy.Apply([3]note.State{ns[0].State, ns[1].State, ns[2].State}))
	doc := must(document.Hash(document.DocumentInfo{ProductName: "Integrated containment product", LotID: "SUPPLEMENT-2026-10-12", Unit: "kg"}))
	eligible, waste := b.n(doc, result.Eligible, note.AssetRoleEligible, a.Address), b.n(doc, result.Waste, note.AssetRoleWaste, a.Address)
	c := v2events.NewProcess(b.pk)
	c.Base = *process.Assignment(ns, a.SKOwner, b.notes.Root(), [3]merkle.Path{b.path(ns[0]), b.path(ns[1]), b.path(ns[2])}, [2]note.Note{eligible, waste}, result)
	b.seal(v2audit.Process, "process", c, []fr.Element{ns[0].Commitment, ns[1].Commitment, ns[2].Commitment}, []fr.Element{note.Nullifier(a.SKOwner, eligible.Commitment), note.Nullifier(a.SKOwner, waste.Commitment)})
	b.append(eligible)
	b.append(waste)
	return eligible, waste
}
func integratedTransfer(b *builder, n note.Note, a, to testkit.Actor, delta uint64) (voucher.Voucher, note.Note) {
	vs, cs, rem, err := voucher.AllocateWithTransport(n.State, n.State.QMass, delta)
	if err != nil {
		panic(err)
	}
	if rem.ARec != 0 || rem.E != 0 {
		panic("nonzero allocation remainder")
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
	_, _, err = b.vouchers.Append(v.Commitment)
	if err != nil {
		panic(err)
	}
	return v, change
}
func buildIntegratedBase(root string) *integratedBase {
	pkg, _, err := KeyFixture()
	if err != nil {
		panic(err)
	}
	actors := must(testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v2.json")))
	x := &integratedBase{b: newBuilder(pkg.PublicKey, "mixed-containment"), factory: actors[1], receiver: actors[2], notes: map[string]note.Note{}, vouchers: map[string]voucher.Voucher{}}
	x.b.serial = 800000
	doc := must(document.Hash(document.DocumentInfo{ProductName: "Supplement input", LotID: "SUPPLEMENT-2026-10-12", Unit: "kg"}))
	for _, row := range []struct {
		l string
		s note.State
	}{{"A1", note.State{QMass: 60e9, ARec: 10e9, E: 50e9}}, {"A2", note.State{QMass: 60e9, ARec: 10e9, E: 50e9}}, {"C", note.State{QMass: 100e9, ARec: 10e9, E: 70e9}}, {"D", note.State{QMass: 100e9, E: 70e9}}, {"U1", note.State{QMass: 115e9, E: 115e9}}, {"U2", note.State{QMass: 115e9, E: 115e9}}} {
		n := x.b.n(doc, row.s, note.AssetRoleEligible, x.factory.Address)
		x.addNote(row.l, n, x.factory)
		x.b.entry(n, x.factory)
		x.step("entry-"+row.l, nil, []string{row.l})
	}
	m := x.b.merge(x.notes["A1"], x.notes["A2"], x.factory)
	x.addNote("M", m, x.factory)
	x.step("merge", []string{"A1", "A2"}, []string{"M"})
	el, w := integratedProcess(x.b, [3]note.Note{m, x.notes["C"], x.notes["D"]}, x.factory)
	if el.State != (note.State{QMass: 270e9, ARec: 30e9, E: 270e9}) || w.State != (note.State{QMass: 30e9}) {
		panic("planned Process arithmetic mismatch")
	}
	x.addNote("EL", el, x.factory)
	x.addNote("W", w, x.factory)
	x.step("process", []string{"M", "C", "D"}, []string{"EL", "W"})
	r, rest := x.b.split(el, x.factory, 90e9)
	x.addNote("R", r, x.factory)
	x.addNote("REST", rest, x.factory)
	x.step("split-1", []string{"EL"}, []string{"R", "REST"})
	p, q := x.b.split(rest, x.factory, 90e9)
	x.addNote("P", p, x.factory)
	x.addNote("V", q, x.factory)
	x.step("split-2", []string{"REST"}, []string{"P", "V"})
	for i, l := range []string{"R", "P", "Q"} {
		nl := l
		if l == "Q" {
			nl = "V"
		}
		v, c := integratedTransfer(x.b, x.notes[nl], x.factory, x.receiver, uint64(i+1)*1e9)
		x.addVoucher("V_"+l, v)
		x.addNote("C_"+l, c, x.factory)
		x.step("transfer-"+l, []string{nl}, []string{"V_" + l, "C_" + l})
		if l == "R" {
			n := x.b.resolve(v, x.factory, true)
			x.addNote("N_R", n, x.factory)
			x.step("recall-R", []string{"V_R"}, []string{"N_R"})
		}
		if l == "P" {
			n := x.b.resolve(v, x.receiver, false)
			x.addNote("N_P", n, x.receiver)
			x.step("proceed-P", []string{"V_P"}, []string{"N_P"})
		}
	}
	x.b.group.Start = v2audit.Ref{ObjectType: v2audit.Note, RawID: x.notes["A1"].Commitment}
	x.expected.Start = "A1"
	x.expected.Targets = []string{"N_R", "N_P", "W", "C_R", "C_P", "C_Q", "V_Q"}
	return x
}
func BuildIntegrated(root string) (out IntegratedFixture, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("integrated fixture: %v", v)
		}
	}()
	base := buildIntegratedBase(root)
	out.Base = base.b.finish()
	out.Expected = base.expected
	// Rebuild the identical base for every exclusive probe. No probe alters the
	// committed base history or another probe's membership state.
	for i, name := range []string{"process-N_R", "transfer-N_P", "proceed-V_Q", "recall-V_Q", "exit-W", "transfer-U1"} {
		x := buildIntegratedBase(root)
		x.b.serial = 900000 + uint64(i)*1000
		switch name {
		case "process-N_R":
			integratedProcess(x.b, [3]note.Note{x.notes["N_R"], x.notes["U1"], x.notes["U2"]}, x.factory)
		case "transfer-N_P":
			integratedTransfer(x.b, x.notes["N_P"], x.receiver, x.factory, 0)
		case "proceed-V_Q":
			x.b.resolve(x.vouchers["V_Q"], x.receiver, false)
		case "recall-V_Q":
			x.b.resolve(x.vouchers["V_Q"], x.factory, true)
		case "exit-W":
			x.b.exit(x.notes["W"], x.factory)
		case "transfer-U1":
			integratedTransfer(x.b, x.notes["U1"], x.factory, x.receiver, 0)
		}
		ev := x.b.group.Events[len(out.Base.Events)]
		ev.Name = name
		out.Probes = append(out.Probes, ev)
	}
	seen := map[string]bool{}
	for _, ev := range append(append([]Event{}, out.Base.Events...), out.Probes...) {
		k := ev.Record.R1.X.String() + ":" + ev.Record.R1.Y.String()
		if seen[k] {
			return out, fmt.Errorf("reused event encryption randomness")
		}
		seen[k] = true
	}
	return out, nil
}
