package statusprocess

import (
	process "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	PolicyRef      frontend.Variable    `gnark:",public"`
	ScopeRef       frontend.Variable    `gnark:",public"`
	NoteRoot       frontend.Variable    `gnark:",public"`
	NoteStatusRoot frontend.Variable    `gnark:",public"`
	NF             [3]frontend.Variable `gnark:",public"`
	CMOut          [2]frontend.Variable `gnark:",public"`
	SKOwner        frontend.Variable
	Inputs         [3]circuitutil.NoteWitness
	Paths          [3]circuitutil.MerklePath
	Outputs        [2]circuitutil.NoteWitness
	QLoss          frontend.Variable
	CarbonAdd      frontend.Variable
	RemLoss        frontend.Variable
	RemCarbon      frontend.Variable
	RemWaste       frontend.Variable
	StatusPaths    [3]circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := process.Circuit{PolicyRef: c.PolicyRef, ScopeRef: c.ScopeRef, NoteRoot: c.NoteRoot, NF: c.NF, CMOut: c.CMOut, SKOwner: c.SKOwner, Inputs: c.Inputs, Paths: c.Paths, Outputs: c.Outputs, QLoss: c.QLoss, CarbonAdd: c.CarbonAdd, RemLoss: c.RemLoss, RemCarbon: c.RemCarbon, RemWaste: c.RemWaste}
	if err := base.Define(api); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err := circuitutil.AssertStatusActive(api, c.NoteStatusRoot, c.Paths[i].Index, c.StatusPaths[i]); err != nil {
			return err
		}
	}
	return nil
}

func FromBase(base *process.Circuit, root fr.Element, paths [3]status.Path) *Circuit {
	c := &Circuit{PolicyRef: base.PolicyRef, ScopeRef: base.ScopeRef, NoteRoot: base.NoteRoot, NoteStatusRoot: root, NF: base.NF, CMOut: base.CMOut, SKOwner: base.SKOwner, Inputs: base.Inputs, Paths: base.Paths, Outputs: base.Outputs, QLoss: base.QLoss, CarbonAdd: base.CarbonAdd, RemLoss: base.RemLoss, RemCarbon: base.RemCarbon, RemWaste: base.RemWaste}
	for j := range paths {
		for i := range paths[j].Siblings {
			c.StatusPaths[j].Siblings[i] = paths[j].Siblings[i]
		}
	}
	return c
}
