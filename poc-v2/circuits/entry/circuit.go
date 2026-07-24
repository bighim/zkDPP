package entry

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Commitment frontend.Variable `gnark:",public"`
	Note       common.OutputWitness
	Address    frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	common.AssertPositiveUint64(api, c.Note.Quantity)
	for k := 0; k < common.StateLength; k++ {
		common.AssertUint64(api, c.Note.State[k])
	}
	commitment, err := common.Commitment(api, c.Note, c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Commitment, commitment)
	return nil
}
