package proceed

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	VoucherRoot       frontend.Variable `gnark:",public"`
	VoucherCommitment frontend.Variable `gnark:",public"`
	VoucherNullifier  frontend.Variable `gnark:",public"`
	OutputCommitment  frontend.Variable `gnark:",public"`
	ReceiverSecretKey frontend.Variable
	ReceiverPublicKey frontend.Variable
	Voucher           common.VoucherWitness
	Output            common.OutputWitness
}

func (c *Circuit) Define(api frontend.API) error {
	receiverAddress, err := common.Hash(api, c.ReceiverPublicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.ReceiverAddress, receiverAddress)
	derivedPK, err := common.Hash(api, c.ReceiverSecretKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ReceiverPublicKey, derivedPK)
	if err := common.AssertMembership(api, c.VoucherRoot, c.VoucherCommitment, c.Voucher.Path); err != nil {
		return err
	}
	rv, err := common.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.VoucherCommitment, rv)
	rvnf, err := common.Hash(api, c.VoucherCommitment, c.Voucher.Opening)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.VoucherNullifier, rvnf)
	api.AssertIsEqual(c.Output.DocumentHash, c.Voucher.DocumentHash)
	api.AssertIsEqual(c.Output.Quantity, c.Voucher.Quantity)
	for k := 0; k < common.StateLength; k++ {
		api.AssertIsEqual(c.Output.State[k], c.Voucher.State[k])
	}
	cm, err := common.Commitment(api, c.Output, c.Voucher.ReceiverAddress)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment, cm)
	return nil
}
