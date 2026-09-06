package circuitutil

import (
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/twistededwards"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

// AssertEncrypted is shared by the standalone relation and the Process adapter.
// pk is a compile-time constant, not an assignable witness/public input.
func AssertEncrypted(api frontend.API, pk auditcrypto.PublicKey, context frontend.Variable, message []frontend.Variable, r frontend.Variable, r1 ed.Point, data []frontend.Variable) error {
	if len(message) == 0 || len(message) != len(data) {
		return fmt.Errorf("audit encryption vector shape mismatch")
	}
	if err := auditcrypto.ValidatePoint(pk.Point); err != nil {
		return err
	}
	g := auditcrypto.Generator()
	if err := auditcrypto.ValidatePoint(g); err != nil {
		return err
	}
	curve, err := ed.NewEdCurve(api, twistededwards.BLS12_381)
	if err != nil {
		return err
	}
	api.ToBinary(r, 252)
	api.AssertIsLessOrEqual(r, new(big.Int).Sub(auditcrypto.Order(), big.NewInt(1)))
	api.AssertIsDifferent(r, 0)
	base := ed.Point{X: g.X, Y: g.Y}
	pub := ed.Point{X: pk.Point.X, Y: pk.Point.Y}
	expected := curve.ScalarMul(base, r)
	api.AssertIsEqual(r1.X, expected.X)
	api.AssertIsEqual(r1.Y, expected.Y)
	z := curve.ScalarMul(pub, r)
	k, err := Hash(api, auditcrypto.AuditKeyTag, z.X, z.Y, context, len(message))
	if err != nil {
		return err
	}
	for j := range message {
		pad, e := Hash(api, auditcrypto.AuditMaskTag, k, j)
		if e != nil {
			return e
		}
		api.AssertIsEqual(data[j], api.Add(message[j], pad))
	}
	return nil
}
