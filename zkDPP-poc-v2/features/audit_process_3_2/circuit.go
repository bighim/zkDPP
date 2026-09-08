package auditprocess

import (
	process "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"math/big"
)

type Circuit struct {
	Process     process.Circuit
	R1          ed.Point             `gnark:",public"`
	Data        [5]frontend.Variable `gnark:",public"`
	Randomness  frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey) *Circuit { return &Circuit{CommitteePK: pk} }
func (c *Circuit) Define(api frontend.API) error {
	if err := c.Process.Define(api); err != nil {
		return err
	}
	message := make([]frontend.Variable, 5)
	for i := 0; i < 3; i++ {
		cm, err := circuitutil.Commitment(api, c.Process.Inputs[i])
		if err != nil {
			return err
		}
		message[i] = cm
	}
	for i := 0; i < 2; i++ {
		nf, err := circuitutil.Nullifier(api, c.Process.SKOwner, c.Process.CMOut[i])
		if err != nil {
			return err
		}
		message[3+i] = nf
	}
	l, err := circuitutil.Hash(api, auditcrypto.AuditContextTag, policy.EventProcess, c.Process.PolicyRef, c.Process.ScopeRef, c.Process.NoteRoot, c.Process.NF[0], c.Process.NF[1], c.Process.NF[2], c.Process.CMOut[0], c.Process.CMOut[1])
	if err != nil {
		return err
	}
	return circuitutil.AssertEncrypted(api, c.CommitteePK, l, message, c.Randomness, c.R1, c.Data[:])
}
func Assignment(pk auditcrypto.PublicKey, base *process.Circuit, r *big.Int, ct auditcrypto.Ciphertext) *Circuit {
	c := New(pk)
	c.Process = *base
	c.Randomness = new(big.Int).Set(r)
	c.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
	for i := range c.Data {
		c.Data[i] = ct.Data[i]
	}
	return c
}
