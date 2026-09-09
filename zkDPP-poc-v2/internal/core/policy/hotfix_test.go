package policy

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"testing"
)

func TestPositiveEligibleOutput(t *testing.T) {
	if _, e := Apply([3]note.State{}); e == nil {
		t.Fatal("zero Process accepted")
	}
	r, e := Apply([3]note.State{{QMass: 1000}, {}, {}})
	if e != nil || r.Eligible.QMass == 0 {
		t.Fatalf("partial zero rejected: %+v %v", r, e)
	}
}
