package m9case

import (
	"bytes"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

func TestScenarioAssignments(t *testing.T) {
	pk, _, e := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{9}, 2048)))
	if e != nil {
		t.Fatal(e)
	}
	s, e := Build("../..", pk)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Check(); e != nil {
		t.Fatal(e)
	}
	for _, event := range s.Events {
		if e = test.IsSolved(event.Case.Definition, event.Case.Assignment, ecc.BLS12_381.ScalarField()); e != nil {
			t.Fatalf("%s: %v", event.Case.Name, e)
		}
	}
	extra := []struct {
		name                   string
		definition, assignment frontend.Circuit
	}{{s.ProductExit.Name, s.ProductExit.Definition, s.ProductExit.Assignment}, {s.WasteExit.Name, s.WasteExit.Definition, s.WasteExit.Assignment}, {s.Standard.Name, s.Standard.Definition, s.Standard.Assignment}, {s.Strict.Name, s.Strict.Definition, s.Strict.Assignment}}
	for _, x := range extra {
		if e = test.IsSolved(x.definition, x.assignment, ecc.BLS12_381.ScalarField()); e != nil {
			t.Fatalf("%s: %v", x.name, e)
		}
	}
	if s.FinalNoteCount != 19 || s.FinalVoucherCount != 5 {
		t.Fatalf("counts note=%d voucher=%d", s.FinalNoteCount, s.FinalVoucherCount)
	}
}
