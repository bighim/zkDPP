package statusmerge

import (
	merge "github.com/bighim/zkDPP/zkDPP-poc-v1/features/merge"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot       frontend.Variable `gnark:",public"`
	NoteStatusRoot frontend.Variable `gnark:",public"`
	NF1            frontend.Variable `gnark:",public"`
	NF2            frontend.Variable `gnark:",public"`
	CMOut          frontend.Variable `gnark:",public"`
	SKOwner        frontend.Variable
	Inputs         [2]circuitutil.NoteWitness
	Paths          [2]circuitutil.MerklePath
	Output         circuitutil.NoteWitness
	StatusPaths    [2]circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := merge.Circuit{NoteRoot: c.NoteRoot, NF1: c.NF1, NF2: c.NF2, CMOut: c.CMOut, SKOwner: c.SKOwner, Inputs: c.Inputs, Paths: c.Paths, Output: c.Output}
	if err := base.Define(api); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		if err := circuitutil.AssertStatusActive(api, c.NoteStatusRoot, c.Paths[i].Index, c.StatusPaths[i]); err != nil {
			return err
		}
	}
	return nil
}

func FromBase(base *merge.Circuit, root fr.Element, paths [2]status.Path) *Circuit {
	c := &Circuit{NoteRoot: base.NoteRoot, NoteStatusRoot: root, NF1: base.NF1, NF2: base.NF2, CMOut: base.CMOut, SKOwner: base.SKOwner, Inputs: base.Inputs, Paths: base.Paths, Output: base.Output}
	for j := range paths {
		for i := range paths[j].Siblings {
			c.StatusPaths[j].Siblings[i] = paths[j].Siblings[i]
		}
	}
	return c
}
