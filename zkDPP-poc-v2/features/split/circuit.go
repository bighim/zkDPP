package split

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot   frontend.Variable `gnark:",public"`
	NF         frontend.Variable `gnark:",public"`
	CMOut1     frontend.Variable `gnark:",public"`
	CMOut2     frontend.Variable `gnark:",public"`
	SKOwner    frontend.Variable
	Input      circuitutil.NoteWitness
	Path       circuitutil.MerklePath
	Outputs    [2]circuitutil.NoteWitness
	RemainderA frontend.Variable
	RemainderE frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	circuitutil.AssertNote(api, c.Input)
	api.AssertIsDifferent(c.Input.QMass, 0)
	address, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Input.Address, address)
	inputCM, err := circuitutil.Commitment(api, c.Input)
	if err != nil {
		return err
	}
	if err = circuitutil.AssertMembership(api, c.NoteRoot, inputCM, c.Path); err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.SKOwner, inputCM)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.NF, nf)
	for i := 0; i < 2; i++ {
		circuitutil.AssertNote(api, c.Outputs[i])
		api.AssertIsEqual(c.Outputs[i].Address, address)
		api.AssertIsEqual(c.Outputs[i].AssetRole, c.Input.AssetRole)
	}
	api.AssertIsEqual(c.Input.QMass, api.Add(c.Outputs[0].QMass, c.Outputs[1].QMass))
	api.ToBinary(c.RemainderA, 64)
	api.ToBinary(c.RemainderE, 64)
	api.AssertIsEqual(api.Mul(c.Outputs[1].QMass, c.Input.ARec), api.Add(api.Mul(c.Input.QMass, c.Outputs[1].ARec), c.RemainderA))
	api.AssertIsEqual(api.Mul(c.Outputs[1].QMass, c.Input.E), api.Add(api.Mul(c.Input.QMass, c.Outputs[1].E), c.RemainderE))
	api.AssertIsLessOrEqual(api.Add(c.RemainderA, 1), c.Input.QMass)
	api.AssertIsLessOrEqual(api.Add(c.RemainderE, 1), c.Input.QMass)
	api.AssertIsEqual(c.Input.ARec, api.Add(c.Outputs[0].ARec, c.Outputs[1].ARec))
	api.AssertIsEqual(c.Input.E, api.Add(c.Outputs[0].E, c.Outputs[1].E))
	cm1, err := circuitutil.Commitment(api, c.Outputs[0])
	if err != nil {
		return err
	}
	cm2, err := circuitutil.Commitment(api, c.Outputs[1])
	if err != nil {
		return err
	}
	api.AssertIsDifferent(cm1, cm2)
	api.AssertIsEqual(c.CMOut1, cm1)
	api.AssertIsEqual(c.CMOut2, cm2)
	return nil
}

func Assignment(input note.Note, sk, root fr.Element, p merkle.Path, out1, out2 note.Note, rem allocation.Remainder) *Circuit {
	c := &Circuit{NoteRoot: root, NF: note.Nullifier(sk, input.Commitment), CMOut1: out1.Commitment, CMOut2: out2.Commitment, SKOwner: sk, Input: witness(input), Path: path(p), Outputs: [2]circuitutil.NoteWitness{witness(out1), witness(out2)}, RemainderA: rem.ARec, RemainderE: rem.E}
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
