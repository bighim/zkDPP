package note_test

import (
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/owner"
)

func TestNoteAndNullifierDomainsBindInputs(t *testing.T) {
	sk := zkhash.Element(11)
	identity, err := owner.FromSecret(sk)
	if err != nil {
		t.Fatal(err)
	}
	state := note.State{QMass: 100 * note.MassScale, ARec: 20 * note.MassScale, E: 30 * note.CarbonScale}
	value, err := note.New(zkhash.Element(101), state, note.AssetRoleEligible, identity.Address, zkhash.Element(1001))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := note.New(zkhash.Element(101), state, note.AssetRoleEligible, identity.Address, zkhash.Element(1002))
	if err != nil {
		t.Fatal(err)
	}
	if value.Commitment.Equal(&changed.Commitment) {
		t.Fatal("opening change did not change commitment")
	}
	nf := note.Nullifier(sk, value.Commitment)
	if value.Commitment.Equal(&nf) {
		t.Fatal("Note and Nullifier domains produced the same value")
	}
}

func TestStateValidation(t *testing.T) {
	validWaste := note.State{QMass: note.MassScale}
	if err := note.ValidateState(validWaste, note.AssetRoleWaste); err != nil {
		t.Fatalf("valid WASTE rejected: %v", err)
	}
	for name, item := range map[string]struct {
		state note.State
		role  note.AssetRole
	}{
		"credit exceeds mass": {note.State{QMass: 1, ARec: 2}, note.AssetRoleEligible},
		"waste credit":        {note.State{QMass: 2, ARec: 1}, note.AssetRoleWaste},
		"waste carbon":        {note.State{QMass: 2, E: 1}, note.AssetRoleWaste},
		"unknown role":        {note.State{}, note.AssetRole(2)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := note.ValidateState(item.state, item.role); err == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
}
