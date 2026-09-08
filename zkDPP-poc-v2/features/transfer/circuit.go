package transfer

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot      frontend.Variable `gnark:",public"`
	NF            frontend.Variable `gnark:",public"`
	RVNew         frontend.Variable `gnark:",public"`
	CMChange      frontend.Variable `gnark:",public"`
	TransferEpoch frontend.Variable `gnark:",public"`
	DeltaEpoch    frontend.Variable `gnark:",public"`

	SenderSecret frontend.Variable
	Input        circuitutil.NoteWitness
	InputPath    circuitutil.MerklePath
	Voucher      circuitutil.VoucherWitness
	Change       circuitutil.NoteWitness
	RemainderA   frontend.Variable
	RemainderE   frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	api.ToBinary(c.TransferEpoch, 64)
	api.ToBinary(c.DeltaEpoch, 64)
	circuitutil.AssertNote(api, c.Input)
	sender, err := circuitutil.Address(api, c.SenderSecret)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Input.Address, sender)
	inputCM, err := circuitutil.Commitment(api, c.Input)
	if err != nil {
		return err
	}
	if err := circuitutil.AssertMembership(api, c.NoteRoot, inputCM, c.InputPath); err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.SenderSecret, inputCM)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.NF, nf)

	circuitutil.AssertVoucher(api, c.Voucher)
	circuitutil.AssertNote(api, c.Change)
	api.AssertIsDifferent(c.Voucher.QMass, 0)
	api.AssertIsEqual(c.Input.QMass, api.Add(c.Voucher.QMass, c.Change.QMass))
	api.AssertIsEqual(c.Voucher.DocumentHash, c.Input.DocumentHash)
	api.AssertIsEqual(c.Change.DocumentHash, c.Input.DocumentHash)
	api.AssertIsEqual(c.Voucher.AssetRole, c.Input.AssetRole)
	api.AssertIsEqual(c.Change.AssetRole, c.Input.AssetRole)
	api.AssertIsEqual(c.Voucher.SenderAddress, sender)
	api.AssertIsEqual(c.Change.Address, sender)

	api.ToBinary(c.RemainderA, 64)
	api.ToBinary(c.RemainderE, 64)
	api.AssertIsEqual(api.Mul(c.Change.QMass, c.Input.ARec), api.Add(api.Mul(c.Input.QMass, c.Change.ARec), c.RemainderA))
	api.AssertIsEqual(api.Mul(c.Change.QMass, c.Input.E), api.Add(api.Mul(c.Input.QMass, c.Change.E), c.RemainderE))
	api.AssertIsLessOrEqual(api.Add(c.RemainderA, 1), c.Input.QMass)
	api.AssertIsLessOrEqual(api.Add(c.RemainderE, 1), c.Input.QMass)
	api.AssertIsEqual(c.Input.ARec, api.Add(c.Voucher.ARec, c.Change.ARec))
	api.AssertIsEqual(c.Input.E, api.Add(c.Voucher.E, c.Change.E))

	api.AssertIsEqual(c.Voucher.DeadlineBlock, api.Add(c.TransferEpoch, c.DeltaEpoch))
	rv, err := circuitutil.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.RVNew, rv)
	changeCM, err := circuitutil.Commitment(api, c.Change)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMChange, changeCM)
	return nil
}

func Assignment(input note.Note, senderSecret fr.Element, inputRoot fr.Element, path merkle.Path, value voucher.Voucher, change note.Note, rem voucher.Remainder, transferEpoch, deltaEpoch uint64) *Circuit {
	c := &Circuit{
		NoteRoot: inputRoot, NF: note.Nullifier(senderSecret, input.Commitment), RVNew: value.Commitment, CMChange: change.Commitment,
		TransferEpoch: transferEpoch, DeltaEpoch: deltaEpoch, SenderSecret: senderSecret,
		Input: noteWitness(input), Voucher: voucherWitness(value), Change: noteWitness(change),
		InputPath: circuitutil.MerklePath{Index: path.Index}, RemainderA: rem.ARec, RemainderE: rem.E,
	}
	for i := range path.Siblings {
		c.InputPath.Siblings[i] = path.Siblings[i]
	}
	return c
}

func noteWitness(v note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, Address: v.Address, Opening: v.Opening}
}

func voucherWitness(v voucher.Voucher) circuitutil.VoucherWitness {
	return circuitutil.VoucherWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, SenderAddress: v.SenderAddress, ReceiverAddress: v.ReceiverAddress, DeadlineBlock: v.DeadlineBlock, Opening: v.Opening}
}
