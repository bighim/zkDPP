package statussplit

import (
	split "github.com/bighim/zkDPP/zkDPP-poc-v1/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot       frontend.Variable `gnark:",public"`
	NoteStatusRoot frontend.Variable `gnark:",public"`
	NF             frontend.Variable `gnark:",public"`
	CMOut1         frontend.Variable `gnark:",public"`
	CMOut2         frontend.Variable `gnark:",public"`
	SKOwner        frontend.Variable
	Input          circuitutil.NoteWitness
	Path           circuitutil.MerklePath
	Outputs        [2]circuitutil.NoteWitness
	RemainderA     frontend.Variable
	RemainderE     frontend.Variable
	StatusPath     circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := split.Circuit{NoteRoot: c.NoteRoot, NF: c.NF, CMOut1: c.CMOut1, CMOut2: c.CMOut2, SKOwner: c.SKOwner, Input: c.Input, Path: c.Path, Outputs: c.Outputs, RemainderA: c.RemainderA, RemainderE: c.RemainderE}
	if err := base.Define(api); err != nil {
		return err
	}
	return circuitutil.AssertStatusActive(api, c.NoteStatusRoot, c.Path.Index, c.StatusPath)
}

func FromBase(base *split.Circuit, root fr.Element, path status.Path) *Circuit {
	c := &Circuit{NoteRoot: base.NoteRoot, NoteStatusRoot: root, NF: base.NF, CMOut1: base.CMOut1, CMOut2: base.CMOut2, SKOwner: base.SKOwner, Input: base.Input, Path: base.Path, Outputs: base.Outputs, RemainderA: base.RemainderA, RemainderE: base.RemainderE}
	for i := range path.Siblings {
		c.StatusPath.Siblings[i] = path.Siblings[i]
	}
	return c
}
