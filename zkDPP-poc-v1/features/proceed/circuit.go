package proceed

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/voucher"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	VoucherRoot    frontend.Variable `gnark:",public"`
	RVNF           frontend.Variable `gnark:",public"`
	CMReceiver     frontend.Variable `gnark:",public"`
	ReceiverSecret frontend.Variable
	Voucher        circuitutil.VoucherWitness
	VoucherPath    circuitutil.MerklePath
	Output         circuitutil.NoteWitness
}

func (c *Circuit) Define(api frontend.API) error {
	circuitutil.AssertVoucher(api, c.Voucher)
	rv, err := circuitutil.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	if err := circuitutil.AssertMembership(api, c.VoucherRoot, rv, c.VoucherPath); err != nil {
		return err
	}
	receiver, err := circuitutil.Address(api, c.ReceiverSecret)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.ReceiverAddress, receiver)
	rvnf, err := circuitutil.VoucherNullifier(api, c.Voucher.Opening, rv)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.RVNF, rvnf)
	circuitutil.AssertNote(api, c.Output)
	api.AssertIsEqual(c.Output.DocumentHash, c.Voucher.DocumentHash)
	api.AssertIsEqual(c.Output.AssetRole, c.Voucher.AssetRole)
	api.AssertIsEqual(c.Output.QMass, c.Voucher.QMass)
	api.AssertIsEqual(c.Output.ARec, c.Voucher.ARec)
	api.AssertIsEqual(c.Output.E, c.Voucher.E)
	api.AssertIsEqual(c.Output.Address, receiver)
	cm, err := circuitutil.Commitment(api, c.Output)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMReceiver, cm)
	return nil
}

func Assignment(value voucher.Voucher, receiverSecret, root fr.Element, path merkle.Path, output note.Note) *Circuit {
	c := &Circuit{VoucherRoot: root, RVNF: voucher.Nullifier(value.Opening, value.Commitment), CMReceiver: output.Commitment, ReceiverSecret: receiverSecret, Voucher: voucherWitness(value), VoucherPath: circuitutil.MerklePath{Index: path.Index}, Output: noteWitness(output)}
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
