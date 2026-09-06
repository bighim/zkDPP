package auditencryption

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"math/big"
)

type Circuit struct {
	Context     frontend.Variable   `gnark:",public"`
	R1          ed.Point            `gnark:",public"`
	Data        []frontend.Variable `gnark:",public"`
	Message     []frontend.Variable
	Randomness  frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey, n int) *Circuit {
	return &Circuit{CommitteePK: pk, Data: make([]frontend.Variable, n), Message: make([]frontend.Variable, n)}
}
func (c *Circuit) Define(api frontend.API) error {
	return circuitutil.AssertEncrypted(api, c.CommitteePK, c.Context, c.Message, c.Randomness, c.R1, c.Data)
}
func Assignment(pk auditcrypto.PublicKey, context fr.Element, message []fr.Element, r *big.Int, ct auditcrypto.Ciphertext) *Circuit {
	c := New(pk, len(message))
	c.Context = context
	c.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
	c.Randomness = new(big.Int).Set(r)
	for i := range message {
		c.Message[i] = message[i]
		c.Data[i] = ct.Data[i]
	}
	return c
}
