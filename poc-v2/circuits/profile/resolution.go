package profile

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Transfer struct {
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
	Input             Note
	Transfer          Output
	Change            Output
	VoucherOpening    frontend.Variable
	Remainder         []frontend.Variable
}

func (c *Transfer) Define(api frontend.API) error {
	if err := owner(api, c.SenderSecretKey, c.SenderPublicKey, c.SenderAddress); err != nil {
		return err
	}
	receiver, err := common.Hash(api, c.ReceiverPublicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ReceiverAddress, receiver)
	cm, err := commitment(api, asOutput(c.Input), c.SenderAddress)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.InputCommitment, cm)
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
	for k := range c.Input.State {
		common.AssertUint64(api, c.Transfer.State[k])
		common.AssertUint64(api, c.Change.State[k])
		api.AssertIsEqual(api.Mul(c.Change.Quantity, c.Input.State[k]), api.Add(api.Mul(c.Input.Quantity, c.Change.State[k]), c.Remainder[k]))
		api.AssertIsLessOrEqual(api.Add(c.Remainder[k], 1), c.Input.Quantity)
		api.AssertIsEqual(c.Input.State[k], api.Add(c.Transfer.State[k], c.Change.State[k]))
	}
	v := Voucher{DocumentHash: c.Transfer.DocumentHash, Quantity: c.Transfer.Quantity, State: c.Transfer.State, SenderAddress: c.SenderAddress, ReceiverAddress: c.ReceiverAddress, DeltaEpoch: c.DeltaEpoch, Opening: c.VoucherOpening}
	rv, err := voucherCommitment(api, v)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.VoucherCommitment, rv)
	change, err := commitment(api, c.Change, c.SenderAddress)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ChangeCommitment, change)
	return nil
}

type Proceed struct {
	VoucherRoot       frontend.Variable `gnark:",public"`
	VoucherCommitment frontend.Variable `gnark:",public"`
	VoucherNullifier  frontend.Variable `gnark:",public"`
	OutputCommitment  frontend.Variable `gnark:",public"`
	ReceiverSecretKey frontend.Variable
	ReceiverPublicKey frontend.Variable
	Voucher           Voucher
	Output            Output
}

func (c *Proceed) Define(api frontend.API) error {
	pk, err := common.Hash(api, c.ReceiverSecretKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ReceiverPublicKey, pk)
	addr, err := common.Hash(api, c.ReceiverPublicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.ReceiverAddress, addr)
	return resolution(api, c.VoucherRoot, c.VoucherCommitment, c.VoucherNullifier, c.OutputCommitment, c.Voucher, c.Output, c.Voucher.ReceiverAddress)
}

type Recall struct {
	VoucherRoot       frontend.Variable `gnark:",public"`
	VoucherCommitment frontend.Variable `gnark:",public"`
	VoucherNullifier  frontend.Variable `gnark:",public"`
	OutputCommitment  frontend.Variable `gnark:",public"`
	SenderSecretKey   frontend.Variable
	SenderPublicKey   frontend.Variable
	Voucher           Voucher
	Output            Output
}

func (c *Recall) Define(api frontend.API) error {
	pk, err := common.Hash(api, c.SenderSecretKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.SenderPublicKey, pk)
	addr, err := common.Hash(api, c.SenderPublicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.SenderAddress, addr)
	return resolution(api, c.VoucherRoot, c.VoucherCommitment, c.VoucherNullifier, c.OutputCommitment, c.Voucher, c.Output, c.Voucher.SenderAddress)
}

func resolution(api frontend.API, root, rv, rvnf, cm frontend.Variable, v Voucher, out Output, address frontend.Variable) error {
	if err := common.AssertMembership(api, root, rv, v.Path); err != nil {
		return err
	}
	derived, err := voucherCommitment(api, v)
	if err != nil {
		return err
	}
	api.AssertIsEqual(rv, derived)
	derivedNF, err := common.Hash(api, rv, v.Opening)
	if err != nil {
		return err
	}
	api.AssertIsEqual(rvnf, derivedNF)
	api.AssertIsEqual(out.DocumentHash, v.DocumentHash)
	api.AssertIsEqual(out.Quantity, v.Quantity)
	for k := range out.State {
		api.AssertIsEqual(out.State[k], v.State[k])
	}
	derivedCM, err := commitment(api, out, address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(cm, derivedCM)
	return nil
}
