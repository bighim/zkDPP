package privatespend_test

import (
	"path/filepath"
	"testing"

	privatespend "github.com/bighim/zkDPP/zkDPP-poc-v1/features/private_spend"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

func spendFixture(t *testing.T) (*privatespend.Circuit, note.Note, fr.Element) {
	t.Helper()
	actors, err := testkit.LoadActors(filepath.Join("..", "..", "testdata", "common", "actors-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	documentHash, err := document.Hash(document.DocumentInfo{ProductName: "Aluminum", LotID: "LOT-M1-001"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := note.New(documentHash, note.State{QMass: 100 * note.MassScale, ARec: 20 * note.MassScale, E: 30 * note.CarbonScale}, note.AssetRoleEligible, actors[0].Address, zkhash.Element(1001))
	if err != nil {
		t.Fatal(err)
	}
	other, err := note.New(zkhash.Element(202), note.State{QMass: note.MassScale}, note.AssetRoleEligible, actors[1].Address, zkhash.Element(1002))
	if err != nil {
		t.Fatal(err)
	}
	tree := merkle.New()
	if _, _, err := tree.Append(value.Commitment); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tree.Append(other.Commitment); err != nil {
		t.Fatal(err)
	}
	path, err := tree.Path(0)
	if err != nil {
		t.Fatal(err)
	}
	return privatespend.Assignment(value, actors[0].SKOwner, tree.Root(), path), value, actors[1].SKOwner
}

func TestValidPrivateSpend(t *testing.T) {
	assignment, _, _ := spendFixture(t)
	test.NewAssert(t).SolvingSucceeded(&privatespend.Circuit{}, assignment, test.WithCurves(ecc.BLS12_381))
}

func TestPrivateSpendHasTwoPublicInputs(t *testing.T) {
	var ccs constraint.ConstraintSystem
	var err error
	ccs, err = frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, &privatespend.Circuit{})
	if err != nil {
		t.Fatal(err)
	}
	if got := ccs.GetNbPublicVariables(); got != 2 {
		t.Fatalf("public variables=%d, want 2", got)
	}
}

func TestInvalidPrivateSpendWitnesses(t *testing.T) {
	assert := test.NewAssert(t)
	valid, _, wrongSecret := spendFixture(t)
	cases := map[string]func(*privatespend.Circuit){
		"secret":    func(a *privatespend.Circuit) { a.SKOwner = wrongSecret },
		"address":   func(a *privatespend.Circuit) { a.Note.Address = 999 },
		"opening":   func(a *privatespend.Circuit) { a.Note.Opening = 999 },
		"index":     func(a *privatespend.Circuit) { a.Path.Index = 1 },
		"sibling":   func(a *privatespend.Circuit) { a.Path.Siblings[0] = 999 },
		"root":      func(a *privatespend.Circuit) { a.NoteRoot = 999 },
		"nullifier": func(a *privatespend.Circuit) { a.NF = 999 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			assignment := *valid
			mutate(&assignment)
			assert.SolvingFailed(&privatespend.Circuit{}, &assignment, test.WithCurves(ecc.BLS12_381))
		})
	}
}
