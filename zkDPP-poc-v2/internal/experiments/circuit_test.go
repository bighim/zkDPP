package experiments

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"testing"
)

func TestCompileProfiles(t *testing.T) {
	pkg, _, e := v2case.KeyFixture()
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range Profiles() {
		t.Run(p.Name, func(t *testing.T) {
			cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, NewCircuit(p, pkg.PublicKey))
			if e != nil {
				t.Fatal(e)
			}
			if cs.GetNbPublicVariables() != len(p.PublicNames()) {
				t.Fatal("public shape")
			}
		})
	}
}
