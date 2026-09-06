package m4case_test

import (
	mergecircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/merge"
	splitcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/allocation"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m4case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"math"
	"path/filepath"
	"testing"
)

func TestM4ValidAndPublicInputs(t *testing.T) {
	s, e := m4case.Build(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	a := test.NewAssert(t)
	a.SolvingSucceeded(&mergecircuit.Circuit{}, s.Merge.Assignment, test.WithCurves(ecc.BLS12_381))
	a.SolvingSucceeded(&splitcircuit.Circuit{}, s.Split.Assignment, test.WithCurves(ecc.BLS12_381))
	for _, v := range []frontend.Circuit{&mergecircuit.Circuit{}, &splitcircuit.Circuit{}} {
		ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, v)
		if e != nil {
			t.Fatal(e)
		}
		if ccs.GetNbPublicVariables() != 4 {
			t.Fatalf("public=%d", ccs.GetNbPublicVariables())
		}
	}
}
func TestM4InvalidWitnesses(t *testing.T) {
	s, e := m4case.Build(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	a := test.NewAssert(t)
	badM := *s.Merge.Assignment
	badM.Inputs[1].AssetRole = 1
	a.SolvingFailed(&mergecircuit.Circuit{}, &badM, test.WithCurves(ecc.BLS12_381))
	same := *s.Merge.Assignment
	same.Inputs[1] = same.Inputs[0]
	same.Paths[1] = same.Paths[0]
	same.NF2 = same.NF1
	a.SolvingFailed(&mergecircuit.Circuit{}, &same, test.WithCurves(ecc.BLS12_381))
	badS := *s.Split.Assignment
	badS.RemainderA = 1
	a.SolvingFailed(&splitcircuit.Circuit{}, &badS, test.WithCurves(ecc.BLS12_381))
}
func TestWasteMergeAndOverflow(t *testing.T) {
	actors, e := testkit.LoadActors(filepath.Join("..", "..", "testdata", "common", "actors-v1.json"))
	if e != nil {
		t.Fatal(e)
	}
	owner := actors[0]
	tree := merkle.New()
	wa, _ := note.New(zkhash.Element(1), note.State{QMass: 2}, note.AssetRoleWaste, owner.Address, zkhash.Element(10))
	wb, _ := note.New(zkhash.Element(2), note.State{QMass: 3}, note.AssetRoleWaste, owner.Address, zkhash.Element(11))
	wo, _ := note.New(zkhash.Element(3), note.State{QMass: 5}, note.AssetRoleWaste, owner.Address, zkhash.Element(12))
	_, _, _ = tree.Append(wa.Commitment)
	_, _, _ = tree.Append(wb.Commitment)
	pa, _ := tree.Path(0)
	pb, _ := tree.Path(1)
	test.NewAssert(t).SolvingSucceeded(&mergecircuit.Circuit{}, mergecircuit.Assignment(wa, wb, owner.SKOwner, tree.Root(), pa, pb, wo), test.WithCurves(ecc.BLS12_381))
	if _, e = allocation.Add(note.State{QMass: math.MaxUint64}, note.State{QMass: 1}); e == nil {
		t.Fatal("overflow accepted")
	}
}
func TestSplitZeroOutput(t *testing.T) {
	actors, e := testkit.LoadActors(filepath.Join("..", "..", "testdata", "common", "actors-v1.json"))
	if e != nil {
		t.Fatal(e)
	}
	owner := actors[0]
	in, _ := note.New(zkhash.Element(1), note.State{QMass: 3e9, ARec: 1e9, E: 2e9}, note.AssetRoleEligible, owner.Address, zkhash.Element(20))
	first, second, rem, _ := allocation.ByMass(in.State, 0)
	o1, _ := note.New(zkhash.Element(2), first, in.AssetRole, owner.Address, zkhash.Element(21))
	o2, _ := note.New(zkhash.Element(3), second, in.AssetRole, owner.Address, zkhash.Element(22))
	tree := merkle.New()
	_, _, _ = tree.Append(in.Commitment)
	p, _ := tree.Path(0)
	test.NewAssert(t).SolvingSucceeded(&splitcircuit.Circuit{}, splitcircuit.Assignment(in, owner.SKOwner, tree.Root(), p, o1, o2, rem), test.WithCurves(ecc.BLS12_381))
}
