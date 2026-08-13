package gadgets

import (
	"testing"

	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func TestNativeMerkleOpening(t *testing.T) {
	for _, profile := range []document.Profile{document.Poseidon2, document.SHA256} {
		doc := document.CanonicalDocument()
		tree := document.Build(profile, doc)
		for leaf := 0; leaf < document.LeafCount; leaf++ {
			opening, err := tree.Open(doc, leaf)
			if err != nil {
				t.Fatal(err)
			}
			if document.RootFromOpening(profile, opening) != tree.Root() {
				t.Fatalf("%s leaf %d opening does not recover root", profile, leaf)
			}
		}
	}
}

func TestCanonicalGadgetsSolve(t *testing.T) {
	for _, profile := range []document.Profile{document.Poseidon2, document.SHA256} {
		assignments, err := CanonicalAssignmentsFor(profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, assignment := range assignments {
			t.Run(assignment.Name, func(t *testing.T) {
				if err := test.IsSolved(assignment.Circuit, assignment.Assignment, ecc.BLS12_381.ScalarField()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestInvalidGadgetsFail(t *testing.T) {
	for _, profile := range []document.Profile{document.Poseidon2, document.SHA256} {
		assignments, err := CanonicalAssignmentsFor(profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, assignment := range assignments {
			switch witness := assignment.Assignment.(type) {
			case *MerklePathCircuit:
				witness.Root[0] = 1
			case *EqCircuit:
				witness.Y = 43
			case *AddCircuit:
				witness.Z = 43
			case *AndCircuit:
				witness.Z = 0
			case *PoseidonMerklePathCircuit:
				witness.Root = 1
			case *PoseidonEqCircuit:
				witness.Y = 43
			case *PoseidonAddCircuit:
				witness.Z = 43
			case *PoseidonAndCircuit:
				witness.Z = 0
			}
			if err := test.IsSolved(assignment.Circuit, assignment.Assignment, ecc.BLS12_381.ScalarField()); err == nil {
				t.Fatalf("%s invalid witness solved", assignment.Name)
			}
		}
	}
}
