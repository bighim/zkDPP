package experiments

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/test"
	"testing"
)

func sample(p Profile, pk auditcrypto.PublicKey) (*World, Operation) {
	w := NewWorld(pk, 900000)
	sk := h.Element(11)
	doc := h.Element(42)
	op := Operation{P: p, SK: sk, Document: doc, Receiver: owner.Address(h.Element(22)), State: note.State{QMass: 300e9, ARec: 30e9, E: 261e9}, Transport: 1e9}
	if p.StateCount > 0 {
		op.VectorState = make([]uint64, p.StateCount)
		for i := range op.VectorState {
			op.VectorState[i] = uint64(i+1) * Scale
		}
		for j := 0; j < p.M; j++ {
			n := VectorNote{Document: doc, Address: owner.Address(sk), Opening: w.opening(), State: append([]uint64{}, op.VectorState...)}
			n.CM = vectorCM(n)
			must(0, w.Commit(&Event{VectorOutputs: []VectorNote{n}}))
			op.VectorInputs = append(op.VectorInputs, n)
		}
		return w, w.Ready(op)
	}
	if p.Kind == Proceed || p.Kind == Recall {
		v := must(voucher.New(doc, 0, op.State, owner.Address(sk), op.Receiver, 1000000, w.opening()))
		must(0, w.Commit(&Event{Voucher: &v}))
		op.Voucher = v
		if p.Kind == Proceed {
			op.SK = h.Element(22)
		}
		return w, w.Ready(op)
	}
	for i := 0; i < p.M; i++ {
		s := note.State{QMass: 160e9, ARec: 15e9, E: 115e9}
		if p.Kind == Issue {
			s = op.State
		}
		if p.Kind == Process {
			totals := []uint64{320e9, 30e9, 231e9}
			for j, x := range totals {
				q := x / uint64(p.M)
				if i == 0 {
					q = x - q*uint64(p.M-1)
				}
				switch j {
				case 0:
					s.QMass = q
				case 1:
					s.ARec = q
				case 2:
					s.E = q
				}
			}
		}
		n := w.MakeNote(doc, s, 0, owner.Address(sk))
		must(0, w.Commit(&Event{Outputs: []note.Note{n}}))
		op.Inputs = append(op.Inputs, n)
	}
	if p.Kind == Split {
		op.Q = 80e9
	}
	return w, w.Ready(op)
}
func TestNativeCircuits(t *testing.T) {
	pkg, _, e := v2case.KeyFixture()
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range Profiles() {
		t.Run(p.Name, func(t *testing.T) {
			w, op := sample(p, pkg.PublicKey)
			ev, e := w.Build(op)
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e = ev.Witness(); e != nil {
				t.Fatal(e)
			}
			if e = test.IsSolved(NewCircuit(p, pkg.PublicKey), ev.Assignment, ecc.BLS12_381.ScalarField()); e != nil {
				t.Fatal(e)
			}
			old := ev.Public[len(ev.Public)-1]
			var bad fr.Element
			bad.Add(&old, new(fr.Element).SetOne())
			ev.Assignment.Public[len(ev.Public)-1] = bad
			if e = test.IsSolved(NewCircuit(p, pkg.PublicKey), ev.Assignment, ecc.BLS12_381.ScalarField()); e == nil {
				t.Fatal("tampered public input accepted")
			}
		})
	}
}
func TestProcessConservation(t *testing.T) {
	for _, m := range []int{2, 3, 4} {
		p := Profile{Kind: Process, M: m, N: 3}
		pkg, _, _ := v2case.KeyFixture()
		_, op := sample(p, pkg.PublicKey)
		in := make([]note.State, m)
		for i, n := range op.Inputs {
			in[i] = n.State
		}
		out, _, e := ProcessStates(in, 3)
		if e != nil {
			t.Fatal(e)
		}
		want := []note.State{{QMass: 150e9, ARec: 15e9, E: 130500000000}, {QMass: 100e9, ARec: 10e9, E: 87e9}, {QMass: 50e9, ARec: 5e9, E: 43500000000}}
		for i := range out {
			if out[i] != want[i] {
				t.Fatalf("%v != %v", out, want)
			}
		}
	}
	if _, _, e := ProcessStates([]note.State{{}, {}}, 1); e == nil {
		t.Fatal("zero accepted")
	}
}
