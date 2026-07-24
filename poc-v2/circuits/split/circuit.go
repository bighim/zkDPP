package split

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Root              frontend.Variable `gnark:",public"`
	InputCommitment   frontend.Variable `gnark:",public"`
	Nullifier         frontend.Variable `gnark:",public"`
	OutputCommitment1 frontend.Variable `gnark:",public"`
	OutputCommitment2 frontend.Variable `gnark:",public"`
	SecretKey         frontend.Variable
	PublicKey         frontend.Variable
	Address           frontend.Variable
	Input             common.NoteWitness
	Outputs           [2]common.OutputWitness
	Remainder         [common.StateLength]frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	if err := common.DeriveOwner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	cm, err := common.Commitment(api, common.AsOutput(c.Input), c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.InputCommitment, cm)
	if err := common.AssertMembership(api, c.Root, c.InputCommitment, c.Input.Path); err != nil {
		return err
	}
	nf, err := common.Hash(api, c.SecretKey, c.InputCommitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Nullifier, nf)
	api.AssertIsEqual(c.Input.Quantity, api.Add(c.Outputs[0].Quantity, c.Outputs[1].Quantity))
	for j := 0; j < 2; j++ {
		common.AssertUint64(api, c.Outputs[j].Quantity)
		api.AssertIsEqual(c.Outputs[j].DocumentHash, c.Input.DocumentHash)
	}
	for k := 0; k < common.StateLength; k++ {
		for j := 0; j < 2; j++ {
			common.AssertUint64(api, c.Outputs[j].State[k])
		}
		api.AssertIsEqual(api.Mul(c.Outputs[1].Quantity, c.Input.State[k]), api.Add(api.Mul(c.Input.Quantity, c.Outputs[1].State[k]), c.Remainder[k]))
		api.AssertIsLessOrEqual(api.Add(c.Remainder[k], 1), c.Input.Quantity)
		api.AssertIsEqual(c.Input.State[k], api.Add(c.Outputs[0].State[k], c.Outputs[1].State[k]))
	}
	cm1, err := common.Commitment(api, c.Outputs[0], c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment1, cm1)
	cm2, err := common.Commitment(api, c.Outputs[1], c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment2, cm2)
	return nil
}
