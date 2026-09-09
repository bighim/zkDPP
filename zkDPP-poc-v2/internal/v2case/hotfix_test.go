package v2case

import (
	process "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"github.com/consensys/gnark/test"
	"math/big"
	"testing"
)

func TestHotfixProcessZeroDomain(t *testing.T) {
	pkg, _, e := KeyFixture()
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range []uint64{0, 1000} {
		sk := h.Element(11)
		addr := owner.Address(sk)
		tree := merkle.New()
		var inputs [3]note.Note
		var states [3]note.State
		states[0].QMass = q
		for i := range inputs {
			inputs[i] = must(note.New(h.Element(1), states[i], 0, addr, h.Element(uint64(10+i))))
			tree.Append(inputs[i].Commitment)
		}
		var paths [3]merkle.Path
		for i := range paths {
			paths[i] = must(tree.Path(uint64(i)))
		}
		res := policy.Result{}
		if q != 0 {
			res = must(policy.Apply(states))
		}
		outs := [2]note.Note{must(note.New(h.Element(2), res.Eligible, 0, addr, h.Element(20))), must(note.New(h.Element(2), res.Waste, 1, addr, h.Element(21)))}
		c := v2events.NewProcess(pkg.PublicKey)
		c.Base = *process.Assignment(inputs, sk, tree.Root(), paths, outs, res)
		msg := []fr.Element{inputs[0].Commitment, inputs[1].Commitment, inputs[2].Commitment, note.Nullifier(sk, outs[0].Commitment), note.Nullifier(sk, outs[1].Commitment)}
		r := big.NewInt(42)
		ct := must(auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, msg, r))
		c.Audit.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
		c.Audit.Randomness = r
		for i := range c.Audit.Parents {
			c.Audit.Parents[i] = ct.Data[i]
		}
		for i := range c.Audit.OutputNfs {
			c.Audit.OutputNfs[i] = ct.Data[i+3]
		}
		err := test.IsSolved(v2events.NewProcess(pkg.PublicKey), c, ecc.BLS12_381.ScalarField())
		if (err == nil) != (q > 0) {
			t.Fatalf("mass=%d: %v", q, err)
		}
	}
}
func TestHotfixConnectedWitnesses(t *testing.T) {
	groups, e := BuildHotfix("../..")
	if e != nil {
		t.Fatal(e)
	}
	pkg, _, e := KeyFixture()
	if e != nil {
		t.Fatal(e)
	}
	representatives, e := Build("../..")
	if e != nil {
		t.Fatal(e)
	}
	_ = pkg
	defs := map[string]Case{}
	for _, c := range representatives.Cases {
		defs[c.Name] = c
	}
	for _, g := range groups {
		for _, ev := range append(append([]Event{}, g.Events...), g.Extra...) {
			if e = test.IsSolved(defs[ev.Relation].Definition, ev.Assignment, ecc.BLS12_381.ScalarField()); e != nil {
				t.Fatalf("%s/%s: %v", g.Name, ev.Name, e)
			}
		}
	}
}
