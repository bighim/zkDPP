package dpp

import (
	"testing"

	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
)

func TestCommitmentBindsEveryField(t *testing.T) {
	base, err := New(zkhash.Element(1), note.AssetRoleEligible, note.State{QMass: 10, ARec: 2, E: 3}, zkhash.Element(4))
	if err != nil {
		t.Fatal(err)
	}
	variants := []struct {
		doc, opening uint64
		state        note.State
	}{
		{2, 4, base.State},
		{1, 5, base.State},
		{1, 4, note.State{QMass: 11, ARec: 2, E: 3}},
		{1, 4, note.State{QMass: 10, ARec: 1, E: 3}},
		{1, 4, note.State{QMass: 10, ARec: 2, E: 4}},
	}
	for _, v := range variants {
		got := Commitment(zkhash.Element(v.doc), note.AssetRoleEligible, v.state, zkhash.Element(v.opening))
		if got.Equal(&base.Commitment) {
			t.Fatal("changed DPP field did not change commitment")
		}
	}
	if _, err = New(zkhash.Element(1), note.AssetRoleWaste, note.State{QMass: 1, E: 1}, zkhash.Element(2)); err == nil {
		t.Fatal("invalid WASTE accepted")
	}
}
