package issuepolicy

import (
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/dpp"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
)

func data(q, a, e uint64, role note.AssetRole) dpp.PrivateData {
	v, _ := dpp.New(zkhash.Element(1), role, note.State{QMass: q, ARec: a, E: e}, zkhash.Element(2))
	return v
}

func TestPolicyBoundaries(t *testing.T) {
	if err := Validate(Standard(), data(100, 10, 100, note.AssetRoleEligible)); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Strict(), data(100, 11, 97, note.AssetRoleEligible)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cfg Config
		v   dpp.PrivateData
	}{
		{Standard(), data(100, 9, 100, note.AssetRoleEligible)},
		{Standard(), data(100, 10, 101, note.AssetRoleEligible)},
		{Strict(), data(100, 11, 98, note.AssetRoleEligible)},
		{Standard(), data(100, 0, 0, note.AssetRoleWaste)},
		{Standard(), data(0, 0, 0, note.AssetRoleEligible)},
	} {
		if err := Validate(tc.cfg, tc.v); err == nil {
			t.Fatal("invalid Policy input accepted")
		}
	}
}
