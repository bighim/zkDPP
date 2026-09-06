package entry_test

import (
	"math/big"
	"testing"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func validEntry(t *testing.T, carbon uint64) *entrycircuit.Circuit {
	t.Helper()
	value, err := note.New(
		zkhash.Element(101),
		note.State{QMass: 100 * note.MassScale, ARec: 20 * note.MassScale, E: carbon},
		note.AssetRoleEligible,
		zkhash.Element(303),
		zkhash.Element(404),
	)
	if err != nil {
		t.Fatal(err)
	}
	return entrycircuit.Assignment(value)
}

func TestEntryAllowsZeroAndPositiveInitialCarbon(t *testing.T) {
	assert := test.NewAssert(t)
	assert.SolvingSucceeded(&entrycircuit.Circuit{}, validEntry(t, 0), test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&entrycircuit.Circuit{}, validEntry(t, 80*note.CarbonScale), test.WithCurves(ecc.BLS12_381))
}

func TestInvalidEntryWitnesses(t *testing.T) {
	assert := test.NewAssert(t)
	valid := validEntry(t, 80*note.CarbonScale)
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	cases := map[string]func(*entrycircuit.Circuit){
		"zero mass":     func(a *entrycircuit.Circuit) { a.Note.QMass = 0 },
		"credit":        func(a *entrycircuit.Circuit) { a.Note.ARec = valid.Note.QMass.(uint64) + 1 },
		"waste":         func(a *entrycircuit.Circuit) { a.Note.AssetRole, a.Note.ARec, a.Note.E = 1, 0, 0 },
		"carbon range":  func(a *entrycircuit.Circuit) { a.Note.E = two64 },
		"commitment":    func(a *entrycircuit.Circuit) { a.Commitment = 999 },
		"document hash": func(a *entrycircuit.Circuit) { a.Note.DocumentHash = 999 },
		"owner address": func(a *entrycircuit.Circuit) { a.Note.Address = 999 },
		"opening":       func(a *entrycircuit.Circuit) { a.Note.Opening = 999 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			assignment := *valid
			mutate(&assignment)
			assert.SolvingFailed(&entrycircuit.Circuit{}, &assignment, test.WithCurves(ecc.BLS12_381))
		})
	}
}
