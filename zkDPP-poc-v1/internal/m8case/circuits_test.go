package m8case

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/features/issue_claim"
	m8exit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/m8_exit_dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

func TestM8CircuitsAndPublicOrder(t *testing.T) {
	pk, _, err := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{8}, 2048)))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Build("../..", pk)
	if err != nil {
		t.Fatal(err)
	}
	items := []struct {
		name       string
		definition frontend.Circuit
		assignment frontend.Circuit
		public     int
		inputs     []fr.Element
	}{
		{s.EligibleExit.Name, s.EligibleExit.Definition, s.EligibleExit.Assignment, 6, s.EligibleExit.PublicInputs},
		{s.WasteExit.Name, s.WasteExit.Definition, s.WasteExit.Assignment, 6, s.WasteExit.PublicInputs},
		{s.Standard.Name, s.Standard.Definition, s.Standard.Assignment, 2, s.Standard.PublicInputs},
		{s.Strict.Name, s.Strict.Definition, s.Strict.Assignment, 2, s.Strict.PublicInputs},
	}
	for _, item := range items {
		t.Run(item.name, func(t *testing.T) {
			if err := test.IsSolved(item.definition, item.assignment, ecc.BLS12_381.ScalarField()); err != nil {
				t.Fatal(err)
			}
			ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.definition)
			if err != nil {
				t.Fatal(err)
			}
			if ccs.GetNbPublicVariables() != item.public {
				t.Fatalf("public=%d want=%d", ccs.GetNbPublicVariables(), item.public)
			}
			w, err := frontend.NewWitness(item.assignment, ecc.BLS12_381.ScalarField())
			if err != nil {
				t.Fatal(err)
			}
			pub, err := w.Public()
			if err != nil {
				t.Fatal(err)
			}
			if !audit.Equal([]fr.Element(pub.Vector().(fr.Vector)), item.inputs) {
				t.Fatal("public input order mismatch")
			}
		})
	}
}

func TestIssueBoundariesAndFailures(t *testing.T) {
	valid := []struct {
		cfg     issuepolicy.Config
		q, a, e uint64
	}{
		{issuepolicy.Standard(), 100, 10, 100},
		{issuepolicy.Strict(), 100, 11, 97},
		{issuepolicy.Standard(), 270, 30, 260},
		{issuepolicy.Strict(), 270, 30, 260},
		{issuepolicy.Standard(), ^uint64(0), ^uint64(0), ^uint64(0)},
	}
	for i, tc := range valid {
		c, err := Boundary(tc.cfg, tc.q, tc.a, tc.e, uint64(9400+i))
		if err != nil {
			t.Fatal(err)
		}
		if err = test.IsSolved(c.Definition, c.Assignment, ecc.BLS12_381.ScalarField()); err != nil {
			t.Fatal(err)
		}
	}
	invalid := []struct {
		cfg     issuepolicy.Config
		q, a, e uint64
	}{
		{issuepolicy.Standard(), 100, 9, 100},
		{issuepolicy.Standard(), 100, 10, 101},
		{issuepolicy.Strict(), 100, 10, 97},
		{issuepolicy.Strict(), 100, 11, 98},
	}
	for i, tc := range invalid {
		c, err := Boundary(tc.cfg, tc.q, tc.a, tc.e, uint64(9500+i))
		if err != nil {
			t.Fatal(err)
		}
		if test.IsSolved(c.Definition, c.Assignment, ecc.BLS12_381.ScalarField()) == nil {
			t.Fatal("invalid boundary accepted")
		}
	}
}

func TestExitAndIssueSemanticMutationsFail(t *testing.T) {
	pk, _, err := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{9}, 2048)))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Build("../..", pk)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*m8exit.Circuit){
		"DPP state":      func(c *m8exit.Circuit) { c.DPP.QMass = c.DPP.QMass.(uint64) + 1 },
		"ciphertext":     func(c *m8exit.Circuit) { c.EncryptedParentCM = 0 },
		"DPP commitment": func(c *m8exit.Circuit) { c.DPPCommitment = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			copy := *s.EligibleExit.Assignment.(*m8exit.Circuit)
			mutate(&copy)
			if test.IsSolved(s.EligibleExit.Definition, &copy, ecc.BLS12_381.ScalarField()) == nil {
				t.Fatal("mutated Exit accepted")
			}
		})
	}
	zero, err := dpp.New(zkhash.Element(1), note.AssetRoleEligible, note.State{}, zkhash.Element(1))
	if err != nil {
		t.Fatal(err)
	}
	if test.IsSolved(issueclaim.New(issuepolicy.Standard()), issueclaim.Assignment(issuepolicy.Standard(), zero), ecc.BLS12_381.ScalarField()) == nil {
		t.Fatal("zero-mass Issue accepted")
	}
	waste, err := dpp.New(zkhash.Element(1), note.AssetRoleWaste, note.State{QMass: 100}, zkhash.Element(2))
	if err != nil {
		t.Fatal(err)
	}
	if test.IsSolved(issueclaim.New(issuepolicy.Standard()), issueclaim.Assignment(issuepolicy.Standard(), waste), ecc.BLS12_381.ScalarField()) == nil {
		t.Fatal("WASTE Issue accepted")
	}
}

func TestMutatedIssueCommitmentFails(t *testing.T) {
	c, err := Boundary(issuepolicy.Standard(), 100, 10, 100, 9601)
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.New(reflect.TypeOf(c.Assignment).Elem())
	v.Elem().Set(reflect.ValueOf(c.Assignment).Elem())
	v.Elem().FieldByName("DPPCommitment").Set(reflect.ValueOf(frontend.Variable(0)))
	if test.IsSolved(issueclaim.New(c.Config), v.Interface().(frontend.Circuit), ecc.BLS12_381.ScalarField()) == nil {
		t.Fatal("mutated commitment accepted")
	}
}
