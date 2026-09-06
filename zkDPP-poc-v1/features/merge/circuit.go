package merge

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot frontend.Variable `gnark:",public"`
	NF1      frontend.Variable `gnark:",public"`
	NF2      frontend.Variable `gnark:",public"`
	CMOut    frontend.Variable `gnark:",public"`
	SKOwner  frontend.Variable
	Inputs   [2]circuitutil.NoteWitness
	Paths    [2]circuitutil.MerklePath
	Output   circuitutil.NoteWitness
}

func (c *Circuit) Define(api frontend.API) error {
	address, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	commitments := [2]frontend.Variable{}
	for i := 0; i < 2; i++ {
		circuitutil.AssertNote(api, c.Inputs[i])
		api.AssertIsEqual(c.Inputs[i].Address, address)
		commitments[i], err = circuitutil.Commitment(api, c.Inputs[i])
		if err != nil {
			return err
		}
		if err = circuitutil.AssertMembership(api, c.NoteRoot, commitments[i], c.Paths[i]); err != nil {
			return err
		}
		nf, e := circuitutil.Nullifier(api, c.SKOwner, commitments[i])
		if e != nil {
			return e
		}
		if i == 0 {
			api.AssertIsEqual(c.NF1, nf)
		} else {
			api.AssertIsEqual(c.NF2, nf)
		}
	}
	api.AssertIsDifferent(commitments[0], commitments[1])
	api.AssertIsDifferent(c.NF1, c.NF2)
	circuitutil.AssertNote(api, c.Output)
	api.AssertIsEqual(c.Inputs[0].AssetRole, c.Inputs[1].AssetRole)
	api.AssertIsEqual(c.Output.AssetRole, c.Inputs[0].AssetRole)
	api.AssertIsEqual(c.Output.Address, address)
	api.AssertIsEqual(c.Output.QMass, api.Add(c.Inputs[0].QMass, c.Inputs[1].QMass))
	api.AssertIsEqual(c.Output.ARec, api.Add(c.Inputs[0].ARec, c.Inputs[1].ARec))
	api.AssertIsEqual(c.Output.E, api.Add(c.Inputs[0].E, c.Inputs[1].E))
	cm, err := circuitutil.Commitment(api, c.Output)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMOut, cm)
	return nil
}

func Assignment(a, b note.Note, sk, root fr.Element, pathA, pathB merkle.Path, out note.Note) *Circuit {
	c := &Circuit{NoteRoot: root, NF1: note.Nullifier(sk, a.Commitment), NF2: note.Nullifier(sk, b.Commitment), CMOut: out.Commitment, SKOwner: sk, Inputs: [2]circuitutil.NoteWitness{witness(a), witness(b)}, Output: witness(out)}
	c.Paths[0] = path(pathA)
	c.Paths[1] = path(pathB)
	return c
}
func witness(v note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, Address: v.Address, Opening: v.Opening}
}
func path(v merkle.Path) circuitutil.MerklePath {
	p := circuitutil.MerklePath{Index: v.Index}
	for i := range v.Siblings {
		p.Siblings[i] = v.Siblings[i]
	}
	return p
}
