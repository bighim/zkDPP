package m5case_test

import (
	processcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m5case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"path/filepath"
	"testing"
)

func TestProcessPolicyCircuit(t *testing.T) {
	s, e := m5case.Build(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	test.NewAssert(t).SolvingSucceeded(&processcircuit.Circuit{}, s.Assignment, test.WithCurves(ecc.BLS12_381))
	ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, &processcircuit.Circuit{})
	if e != nil {
		t.Fatal(e)
	}
	if ccs.GetNbPublicVariables() != 8 {
		t.Fatalf("public=%d", ccs.GetNbPublicVariables())
	}
}
func TestInvalidProcessPolicyWitnesses(t *testing.T) {
	s, e := m5case.Build(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	assert := test.NewAssert(t)
	badScope := *s.Assignment
	badScope.ScopeRef = 999
	assert.SolvingFailed(&processcircuit.Circuit{}, &badScope, test.WithCurves(ecc.BLS12_381))
	badLoss := *s.Assignment
	badLoss.QLoss = 1
	assert.SolvingFailed(&processcircuit.Circuit{}, &badLoss, test.WithCurves(ecc.BLS12_381))
	badRole := *s.Assignment
	badRole.Outputs[1].AssetRole = 0
	assert.SolvingFailed(&processcircuit.Circuit{}, &badRole, test.WithCurves(ecc.BLS12_381))
	badPath := *s.Assignment
	badPath.Paths[0].Siblings[0] = 999
	assert.SolvingFailed(&processcircuit.Circuit{}, &badPath, test.WithCurves(ecc.BLS12_381))
}
