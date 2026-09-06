package statustransfer

import (
	transfer "github.com/bighim/zkDPP/zkDPP-poc-v1/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot       frontend.Variable `gnark:",public"`
	NoteStatusRoot frontend.Variable `gnark:",public"`
	NF             frontend.Variable `gnark:",public"`
	RVNew          frontend.Variable `gnark:",public"`
	CMChange       frontend.Variable `gnark:",public"`
	TransferEpoch  frontend.Variable `gnark:",public"`
	DeltaEpoch     frontend.Variable `gnark:",public"`
	SenderSecret   frontend.Variable
	Input          circuitutil.NoteWitness
	InputPath      circuitutil.MerklePath
	Voucher        circuitutil.VoucherWitness
	Change         circuitutil.NoteWitness
	RemainderA     frontend.Variable
	RemainderE     frontend.Variable
	StatusPath     circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := transfer.Circuit{NoteRoot: c.NoteRoot, NF: c.NF, RVNew: c.RVNew, CMChange: c.CMChange, TransferEpoch: c.TransferEpoch, DeltaEpoch: c.DeltaEpoch, SenderSecret: c.SenderSecret, Input: c.Input, InputPath: c.InputPath, Voucher: c.Voucher, Change: c.Change, RemainderA: c.RemainderA, RemainderE: c.RemainderE}
	if err := base.Define(api); err != nil {
		return err
	}
	return circuitutil.AssertStatusActive(api, c.NoteStatusRoot, c.InputPath.Index, c.StatusPath)
}

func FromBase(base *transfer.Circuit, root fr.Element, path status.Path) *Circuit {
	c := &Circuit{NoteRoot: base.NoteRoot, NoteStatusRoot: root, NF: base.NF, RVNew: base.RVNew, CMChange: base.CMChange, TransferEpoch: base.TransferEpoch, DeltaEpoch: base.DeltaEpoch, SenderSecret: base.SenderSecret, Input: base.Input, InputPath: base.InputPath, Voucher: base.Voucher, Change: base.Change, RemainderA: base.RemainderA, RemainderE: base.RemainderE}
	for i := range path.Siblings {
		c.StatusPath.Siblings[i] = path.Siblings[i]
	}
	return c
}
