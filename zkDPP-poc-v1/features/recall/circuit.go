package recall

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/voucher"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	VoucherRoot  frontend.Variable `gnark:",public"`
	RVNF         frontend.Variable `gnark:",public"`
	CMReturn     frontend.Variable `gnark:",public"`
	CurrentEpoch frontend.Variable `gnark:",public"`
	SenderSecret frontend.Variable
	Voucher      circuitutil.VoucherWitness
	VoucherPath  circuitutil.MerklePath
	Output       circuitutil.NoteWitness
}

func (c *Circuit) Define(api frontend.API) error {
	api.ToBinary(c.CurrentEpoch, 64)
	circuitutil.AssertVoucher(api, c.Voucher)
	rv, err := circuitutil.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	if err := circuitutil.AssertMembership(api, c.VoucherRoot, rv, c.VoucherPath); err != nil {
		return err
	}
	sender, err := circuitutil.Address(api, c.SenderSecret)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.SenderAddress, sender)
	rvnf, err := circuitutil.VoucherNullifier(api, c.Voucher.Opening, rv)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.RVNF, rvnf)
	api.AssertIsLessOrEqual(api.Add(c.CurrentEpoch, 1), c.Voucher.DeadlineEpoch)
	circuitutil.AssertNote(api, c.Output)
	api.AssertIsEqual(c.Output.DocumentHash, c.Voucher.DocumentHash)
	api.AssertIsEqual(c.Output.AssetRole, c.Voucher.AssetRole)
	api.AssertIsEqual(c.Output.QMass, c.Voucher.QMass)
	api.AssertIsEqual(c.Output.ARec, c.Voucher.ARec)
	api.AssertIsEqual(c.Output.E, c.Voucher.E)
	api.AssertIsEqual(c.Output.Address, sender)
	cm, err := circuitutil.Commitment(api, c.Output)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMReturn, cm)
	return nil
}

func Assignment(value voucher.Voucher, senderSecret, root fr.Element, path merkle.Path, output note.Note, currentEpoch uint64) *Circuit {
	c := &Circuit{VoucherRoot: root, RVNF: voucher.Nullifier(value.Opening, value.Commitment), CMReturn: output.Commitment, CurrentEpoch: currentEpoch, SenderSecret: senderSecret, Voucher: voucherWitness(value), VoucherPath: circuitutil.MerklePath{Index: path.Index}, Output: noteWitness(output)}
	for i := range path.Siblings {
		c.VoucherPath.Siblings[i] = path.Siblings[i]
	}
	return c
}

func noteWitness(v note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, Address: v.Address, Opening: v.Opening}
}
func voucherWitness(v voucher.Voucher) circuitutil.VoucherWitness {
	return circuitutil.VoucherWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, SenderAddress: v.SenderAddress, ReceiverAddress: v.ReceiverAddress, DeadlineEpoch: v.DeadlineEpoch, Opening: v.Opening}
}
