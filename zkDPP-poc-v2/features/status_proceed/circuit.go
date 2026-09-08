package statusproceed

import (
	proceed "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	VoucherRoot       frontend.Variable `gnark:",public"`
	VoucherStatusRoot frontend.Variable `gnark:",public"`
	RVNF              frontend.Variable `gnark:",public"`
	CMReceiver        frontend.Variable `gnark:",public"`
	ReceiverSecret    frontend.Variable
	Voucher           circuitutil.VoucherWitness
	VoucherPath       circuitutil.MerklePath
	Output            circuitutil.NoteWitness
	StatusPath        circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := proceed.Circuit{VoucherRoot: c.VoucherRoot, RVNF: c.RVNF, CMReceiver: c.CMReceiver, ReceiverSecret: c.ReceiverSecret, Voucher: c.Voucher, VoucherPath: c.VoucherPath, Output: c.Output}
	if err := base.Define(api); err != nil {
		return err
	}
	return circuitutil.AssertStatusActive(api, c.VoucherStatusRoot, c.VoucherPath.Index, c.StatusPath)
}

func FromBase(base *proceed.Circuit, root fr.Element, path status.Path) *Circuit {
	c := &Circuit{VoucherRoot: base.VoucherRoot, VoucherStatusRoot: root, RVNF: base.RVNF, CMReceiver: base.CMReceiver, ReceiverSecret: base.ReceiverSecret, Voucher: base.Voucher, VoucherPath: base.VoucherPath, Output: base.Output}
	for i := range path.Siblings {
		c.StatusPath.Siblings[i] = path.Siblings[i]
	}
	return c
}
