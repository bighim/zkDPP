package m3case_test

import (
	"path/filepath"
	"testing"

	proceedcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/proceed"
	recallcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/recall"
	transfercircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/transfer"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m3case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

func TestM3ValidCircuitsAndPublicInputs(t *testing.T) {
	s, err := m3case.Build(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	assert := test.NewAssert(t)
	assert.SolvingSucceeded(&transfercircuit.Circuit{}, s.Partial.Assignment, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&transfercircuit.Circuit{}, s.Full.Assignment, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&proceedcircuit.Circuit{}, s.Proceed.Assignment, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&recallcircuit.Circuit{}, s.Recall.Assignment, test.WithCurves(ecc.BLS12_381))
	for _, item := range []struct {
		name    string
		circuit frontend.Circuit
		want    int
	}{{"transfer", &transfercircuit.Circuit{}, 6}, {"proceed", &proceedcircuit.Circuit{}, 3}, {"recall", &recallcircuit.Circuit{}, 4}} {
		ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.circuit)
		if err != nil {
			t.Fatal(err)
		}
		if got := ccs.GetNbPublicVariables(); got != item.want {
			t.Fatalf("%s public=%d want=%d", item.name, got, item.want)
		}
	}
}

func TestM3InvalidWitnesses(t *testing.T) {
	s, err := m3case.Build(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	assert := test.NewAssert(t)
	badTransfer := *s.Partial.Assignment
	badTransfer.RemainderA = uint64(0)
	assert.SolvingFailed(&transfercircuit.Circuit{}, &badTransfer, test.WithCurves(ecc.BLS12_381))
	badPath := *s.Partial.Assignment
	badPath.InputPath.Siblings[0] = uint64(999)
	assert.SolvingFailed(&transfercircuit.Circuit{}, &badPath, test.WithCurves(ecc.BLS12_381))
	badProceed := *s.Proceed.Assignment
	badProceed.Voucher.Opening = uint64(999)
	assert.SolvingFailed(&proceedcircuit.Circuit{}, &badProceed, test.WithCurves(ecc.BLS12_381))
	badRecall := *s.Recall.Assignment
	badRecall.CurrentEpoch = m3case.TransferEpoch + m3case.DeltaEpoch
	assert.SolvingFailed(&recallcircuit.Circuit{}, &badRecall, test.WithCurves(ecc.BLS12_381))
}

func TestWasteTransfer(t *testing.T) {
	actors, err := testkit.LoadActors(filepath.Join("..", "..", "testdata", "common", "actors-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := note.New(zkhash.Element(77), note.State{QMass: 3 * note.MassScale}, note.AssetRoleWaste, actors[0].Address, zkhash.Element(7001))
	if err != nil {
		t.Fatal(err)
	}
	vs, cs, rem, err := voucher.Allocate(in.State, note.MassScale)
	if err != nil {
		t.Fatal(err)
	}
	v, err := voucher.New(in.DocumentHash, in.AssetRole, vs, actors[0].Address, actors[1].Address, 106, zkhash.Element(7002))
	if err != nil {
		t.Fatal(err)
	}
	c, err := note.New(in.DocumentHash, cs, in.AssetRole, actors[0].Address, zkhash.Element(7003))
	if err != nil {
		t.Fatal(err)
	}
	tree := merkle.New()
	_, _, _ = tree.Append(in.Commitment)
	path, _ := tree.Path(0)
	a := transfercircuit.Assignment(in, actors[0].SKOwner, tree.Root(), path, v, c, rem, 100, 6)
	test.NewAssert(t).SolvingSucceeded(&transfercircuit.Circuit{}, a, test.WithCurves(ecc.BLS12_381))
}
