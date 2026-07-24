package exit

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Root       frontend.Variable `gnark:",public"`
	Commitment frontend.Variable `gnark:",public"`
	Nullifier  frontend.Variable `gnark:",public"`
	SecretKey  frontend.Variable
	PublicKey  frontend.Variable
	Address    frontend.Variable
	Note       common.NoteWitness
}

func (c *Circuit) Define(api frontend.API) error {
	if err := common.DeriveOwner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	commitment, err := common.Commitment(api, common.AsOutput(c.Note), c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Commitment, commitment)
	if err := common.AssertMembership(api, c.Root, c.Commitment, c.Note.Path); err != nil {
		return err
	}
	nf, err := common.Hash(api, c.SecretKey, c.Commitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Nullifier, nf)
	return nil
}
