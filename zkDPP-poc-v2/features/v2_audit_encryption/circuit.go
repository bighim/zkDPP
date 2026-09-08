package v2auditencryption

import (
	"math/big"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

type Circuit struct {
	R1          ed.Point            `gnark:",public"`
	Ciphertext  []frontend.Variable `gnark:",public"`
	Plaintext   []frontend.Variable
	Randomness  frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey, fields int) *Circuit {
	if fields < 1 {
		panic("audit plaintext must not be empty")
	}
	return &Circuit{CommitteePK: pk, Ciphertext: make([]frontend.Variable, fields), Plaintext: make([]frontend.Variable, fields)}
}

func (c *Circuit) Define(api frontend.API) error {
	return circuitutil.AssertMasterEncrypted(api, c.CommitteePK, c.Plaintext, c.Randomness, c.R1, c.Ciphertext)
}

func Assignment(pk auditcrypto.PublicKey, plaintext []fr.Element, randomness *big.Int, ciphertext auditcrypto.Ciphertext) *Circuit {
	c := New(pk, len(plaintext))
	c.Randomness = new(big.Int).Set(randomness)
	c.R1 = ed.Point{X: ciphertext.R1.X, Y: ciphertext.R1.Y}
	for i := range plaintext {
		c.Plaintext[i] = plaintext[i]
		c.Ciphertext[i] = ciphertext.Data[i]
	}
	return c
}
