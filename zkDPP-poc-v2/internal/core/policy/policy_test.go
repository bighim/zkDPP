package policy_test

import (
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"testing"
)

func TestCanonicalPolicy(t *testing.T) {
	r, e := policy.Apply([3]note.State{{QMass: 100e9, ARec: 20e9, E: 80e9}, {QMass: 120e9, ARec: 10e9, E: 90e9}, {QMass: 100e9, E: 60e9}})
	if e != nil {
		t.Fatal(e)
	}
	if r.QLoss != 20e9 || r.CarbonAdd != 30e9 || r.Eligible != (note.State{QMass: 270e9, ARec: 30e9, E: 260e9}) || r.Waste != (note.State{QMass: 30e9}) {
		t.Fatalf("bad result %#v", r)
	}
	a := policy.ScopeRef(zkhash.Element(11), policy.CanonicalRef())
	b := policy.ScopeRef(zkhash.Element(11), policy.Ref(policy.EventProcess, 1, 2, 1))
	if a.Equal(&b) {
		t.Fatal("cross-policy scope linked")
	}
}
