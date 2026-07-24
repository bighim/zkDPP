package merge

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Root1            frontend.Variable `gnark:",public"`
	Root2            frontend.Variable `gnark:",public"`
	Commitment1      frontend.Variable `gnark:",public"`
	Commitment2      frontend.Variable `gnark:",public"`
	Nullifier1       frontend.Variable `gnark:",public"`
	Nullifier2       frontend.Variable `gnark:",public"`
	OutputCommitment frontend.Variable `gnark:",public"`
	SecretKey        frontend.Variable
	PublicKey        frontend.Variable
	Address          frontend.Variable
	Inputs           [2]common.NoteWitness
	Output           common.OutputWitness
}

func (c *Circuit) Define(api frontend.API) error {
	if err := common.DeriveOwner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	roots := [2]frontend.Variable{c.Root1, c.Root2}
	cms := [2]frontend.Variable{c.Commitment1, c.Commitment2}
	nfs := [2]frontend.Variable{c.Nullifier1, c.Nullifier2}
	for i := 0; i < 2; i++ {
		cm, err := common.Commitment(api, common.AsOutput(c.Inputs[i]), c.Address)
		if err != nil {
			return err
		}
		api.AssertIsEqual(cms[i], cm)
		if err := common.AssertMembership(api, roots[i], cms[i], c.Inputs[i].Path); err != nil {
			return err
		}
		nf, err := common.Hash(api, c.SecretKey, cms[i])
		if err != nil {
			return err
		}
		api.AssertIsEqual(nfs[i], nf)
	}
	api.AssertIsEqual(c.Inputs[0].DocumentHash, c.Inputs[1].DocumentHash)
	api.AssertIsEqual(c.Output.DocumentHash, c.Inputs[0].DocumentHash)
	api.AssertIsEqual(c.Output.Quantity, api.Add(c.Inputs[0].Quantity, c.Inputs[1].Quantity))
	common.AssertUint64(api, c.Output.Quantity)
	for k := 0; k < common.StateLength; k++ {
		api.AssertIsEqual(c.Output.State[k], api.Add(c.Inputs[0].State[k], c.Inputs[1].State[k]))
		common.AssertUint64(api, c.Output.State[k])
	}
	cm, err := common.Commitment(api, c.Output, c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment, cm)
	return nil
}
