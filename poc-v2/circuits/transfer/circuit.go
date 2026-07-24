package transfer

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Root              frontend.Variable `gnark:",public"`
	InputCommitment   frontend.Variable `gnark:",public"`
	Nullifier         frontend.Variable `gnark:",public"`
	VoucherCommitment frontend.Variable `gnark:",public"`
	ChangeCommitment  frontend.Variable `gnark:",public"`
	DeltaEpoch        frontend.Variable `gnark:",public"`
	SenderSecretKey   frontend.Variable
	SenderPublicKey   frontend.Variable
	ReceiverPublicKey frontend.Variable
	SenderAddress     frontend.Variable
	ReceiverAddress   frontend.Variable
	Input             common.NoteWitness
	Transfer          common.OutputWitness
	Change            common.OutputWitness
	VoucherOpening    frontend.Variable
	Remainder         [common.StateLength]frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	if err := common.DeriveOwner(api, c.SenderSecretKey, c.SenderPublicKey, c.SenderAddress); err != nil {
		return err
	}
	receiverAddress, err := common.Hash(api, c.ReceiverPublicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ReceiverAddress, receiverAddress)
	inputCM, err := common.Commitment(api, common.AsOutput(c.Input), c.SenderAddress)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.InputCommitment, inputCM)
	if err := common.AssertMembership(api, c.Root, c.InputCommitment, c.Input.Path); err != nil {
		return err
	}
	nf, err := common.Hash(api, c.SenderSecretKey, c.InputCommitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Nullifier, nf)
	common.AssertPositiveUint64(api, c.Transfer.Quantity)
	common.AssertUint64(api, c.Change.Quantity)
	api.AssertIsEqual(c.Input.Quantity, api.Add(c.Transfer.Quantity, c.Change.Quantity))
	api.AssertIsEqual(c.Transfer.DocumentHash, c.Input.DocumentHash)
	api.AssertIsEqual(c.Change.DocumentHash, c.Input.DocumentHash)
	for k := 0; k < common.StateLength; k++ {
		common.AssertUint64(api, c.Transfer.State[k])
		common.AssertUint64(api, c.Change.State[k])
		api.AssertIsEqual(api.Mul(c.Change.Quantity, c.Input.State[k]), api.Add(api.Mul(c.Input.Quantity, c.Change.State[k]), c.Remainder[k]))
		api.AssertIsLessOrEqual(api.Add(c.Remainder[k], 1), c.Input.Quantity)
		api.AssertIsEqual(c.Input.State[k], api.Add(c.Transfer.State[k], c.Change.State[k]))
	}
	voucher := common.VoucherWitness{DocumentHash: c.Transfer.DocumentHash, Quantity: c.Transfer.Quantity, State: c.Transfer.State, SenderAddress: c.SenderAddress, ReceiverAddress: c.ReceiverAddress, DeltaEpoch: c.DeltaEpoch, Opening: c.VoucherOpening}
	rv, err := common.VoucherCommitment(api, voucher)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.VoucherCommitment, rv)
	changeCM, err := common.Commitment(api, c.Change, c.SenderAddress)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ChangeCommitment, changeCM)
	return nil
}
