package v2case

import (
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

func TestTenV2Relations(t *testing.T) {
	s, err := Build("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range s.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, c.Definition)
			if err != nil {
				t.Fatal(err)
			}
			if got := ccs.GetNbPublicVariables(); got != len(c.PublicNames) {
				t.Fatalf("public=%d want=%d", got, len(c.PublicNames))
			}
			if err = test.IsSolved(c.Definition, c.Assignment, ecc.BLS12_381.ScalarField()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWrongAuditCipherFails(t *testing.T) {
	s, err := Build("../..")
	if err != nil {
		t.Fatal(err)
	}
	c := s.Cases[0]
	a := c.Assignment.(*v2events.EntryCircuit)
	a.Audit.OutputNfs[0] = 123
	if err = test.IsSolved(c.Definition, a, ecc.BLS12_381.ScalarField()); err == nil {
		t.Fatal("wrong ciphertext accepted")
	}
}
