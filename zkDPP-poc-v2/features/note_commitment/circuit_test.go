package notecommitment_test

import (
	"math/big"
	"path/filepath"
	"testing"

	notecommitment "github.com/bighim/zkDPP/zkDPP-poc-v2/features/note_commitment"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func eligibleNote(t *testing.T) note.Note {
	t.Helper()
	actors, err := testkit.LoadActors(filepath.Join("..", "..", "testdata", "common", "actors-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := document.Hash(document.DocumentInfo{ProductName: "Aluminum", LotID: "LOT-M1-001"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := note.New(hash, note.State{QMass: 100 * note.MassScale, ARec: 20 * note.MassScale, E: 30 * note.CarbonScale}, note.AssetRoleEligible, actors[0].Address, zkhash.Element(1001))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestValidEligibleAndWaste(t *testing.T) {
	assert := test.NewAssert(t)
	eligible := eligibleNote(t)
	assert.SolvingSucceeded(&notecommitment.Circuit{}, notecommitment.Assignment(eligible), test.WithCurves(ecc.BLS12_381))
	waste, err := note.New(zkhash.Element(202), note.State{QMass: 5 * note.MassScale}, note.AssetRoleWaste, eligible.Address, zkhash.Element(1002))
	if err != nil {
		t.Fatal(err)
	}
	assert.SolvingSucceeded(&notecommitment.Circuit{}, notecommitment.Assignment(waste), test.WithCurves(ecc.BLS12_381))
}

func TestInvalidNoteWitnesses(t *testing.T) {
	assert := test.NewAssert(t)
	valid := notecommitment.Assignment(eligibleNote(t))
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	cases := map[string]func(*notecommitment.Circuit){
		"document":     func(a *notecommitment.Circuit) { a.Note.DocumentHash = 999 },
		"mass range":   func(a *notecommitment.Circuit) { a.Note.QMass = two64 },
		"credit":       func(a *notecommitment.Circuit) { a.Note.ARec = valid.Note.QMass.(uint64) + 1 },
		"role":         func(a *notecommitment.Circuit) { a.Note.AssetRole = 2 },
		"waste credit": func(a *notecommitment.Circuit) { a.Note.AssetRole, a.Note.ARec = 1, 1 },
		"waste carbon": func(a *notecommitment.Circuit) { a.Note.AssetRole, a.Note.E = 1, 1 },
		"address":      func(a *notecommitment.Circuit) { a.Note.Address = 999 },
		"opening":      func(a *notecommitment.Circuit) { a.Note.Opening = 999 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			assignment := *valid
			mutate(&assignment)
			assert.SolvingFailed(&notecommitment.Circuit{}, &assignment, test.WithCurves(ecc.BLS12_381))
		})
	}
}
