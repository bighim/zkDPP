// Package auditcrypto implements the M6-B1 registered-ciphertext-only POC.
// It is not TDH2, authenticated encryption, or a production threshold service.
package auditcrypto

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math/big"

	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	ed "github.com/consensys/gnark-crypto/ecc/bls12-381/twistededwards"
)

const Profile = "zkDPP-audit-dh-field-v1"

var (
	AuditKeyTag     = zkhash.MustHashToField("zkDPP:AuditKey:v1")
	AuditMaskTag    = zkhash.MustHashToField("zkDPP:AuditMask:v1")
	AuditContextTag = zkhash.MustHashToField("zkDPP:AuditContext:v1")
)

type Point = ed.PointAffine
type PublicKey struct{ Point Point }
type Share struct {
	ID    uint8
	Value *big.Int `json:"-"`
}
type Ciphertext struct {
	R1   Point
	Data []fr.Element
}
type Partial struct {
	ID    uint8
	Point Point
}

func Order() *big.Int  { p := ed.GetEdwardsCurve(); return new(big.Int).Set(&p.Order) }
func Generator() Point { return ed.GetEdwardsCurve().Base }

func ValidatePoint(p Point) error {
	if !p.IsOnCurve() || p.IsZero() {
		return fmt.Errorf("invalid or identity Jubjub point")
	}
	var check Point
	check.ScalarMultiplication(&p, Order())
	if !check.IsZero() {
		return fmt.Errorf("point is not in the prime-order subgroup")
	}
	return nil
}

func ValidateScalar(v *big.Int, nonzero bool) error {
	if v == nil || v.Sign() < 0 || v.Cmp(Order()) >= 0 || (nonzero && v.Sign() == 0) {
		return fmt.Errorf("scalar outside permitted range")
	}
	return nil
}

// RandomScalar samples uniformly from [1,q-1]. The caller retains r privately.
func RandomScalar(reader io.Reader) (*big.Int, error) {
	if reader == nil {
		reader = cryptorand.Reader
	}
	for {
		v, err := cryptorand.Int(reader, Order())
		if err != nil {
			return nil, fmt.Errorf("scalar randomness unavailable: %w", err)
		}
		if v.Sign() != 0 {
			return v, nil
		}
	}
}

// TrustedSetup returns only PK and shares; it never returns the master secret.
// Clearing big.Int values is best effort, not a Go memory-erasure guarantee.
func TrustedSetup(reader io.Reader) (PublicKey, [3]Share, error) {
	var shares [3]Share
	g := Generator()
	if err := ValidatePoint(g); err != nil {
		return PublicKey{}, shares, err
	}
	for {
		x, err := RandomScalar(reader)
		if err != nil {
			return PublicKey{}, shares, err
		}
		a, err := RandomScalar(reader)
		if err != nil {
			x.SetInt64(0)
			return PublicKey{}, shares, err
		}
		valid := true
		for i := range shares {
			v := new(big.Int).Mul(a, big.NewInt(int64(i+1)))
			v.Add(v, x).Mod(v, Order())
			shares[i] = Share{ID: uint8(i + 1), Value: v}
			valid = valid && v.Sign() != 0
		}
		var point Point
		if valid {
			point.ScalarMultiplication(&g, x)
		}
		x.SetInt64(0)
		a.SetInt64(0)
		if !valid {
			for i := range shares {
				shares[i].Value.SetInt64(0)
			}
			continue
		}
		if err := ValidatePoint(point); err != nil {
			return PublicKey{}, [3]Share{}, err
		}
		return PublicKey{Point: point}, shares, nil
	}
}

func key(z Point, context fr.Element, n int) fr.Element {
	return zkhash.Hash(AuditKeyTag, z.X, z.Y, context, zkhash.Element(uint64(n)))
}
func mask(k fr.Element, index int) fr.Element {
	return zkhash.Hash(AuditMaskTag, k, zkhash.Element(uint64(index)))
}

// Encrypt requires a fresh r for each distinct encryption outside fixed tests.
func Encrypt(pk PublicKey, context fr.Element, message []fr.Element, r *big.Int) (Ciphertext, error) {
	if len(message) == 0 {
		return Ciphertext{}, fmt.Errorf("empty message")
	}
	if err := ValidatePoint(pk.Point); err != nil {
		return Ciphertext{}, err
	}
	if err := ValidateScalar(r, true); err != nil {
		return Ciphertext{}, err
	}
	g := Generator()
	var z, r1 Point
	r1.ScalarMultiplication(&g, r)
	z.ScalarMultiplication(&pk.Point, r)
	k := key(z, context, len(message))
	out := Ciphertext{R1: r1, Data: make([]fr.Element, len(message))}
	for j := range message {
		pad := mask(k, j)
		out.Data[j].Add(&message[j], &pad)
	}
	return out, nil
}

func validateShare(s Share) error {
	if s.ID < 1 || s.ID > 3 {
		return fmt.Errorf("committee ID outside 1..3")
	}
	return ValidateScalar(s.Value, true)
}

// PartialDecrypt is math only. Approval and canonical record lookup belong
// to the caller, not to this reusable core.
func PartialDecrypt(share Share, r1 Point) (Partial, error) {
	if err := validateShare(share); err != nil {
		return Partial{}, err
	}
	if err := ValidatePoint(r1); err != nil {
		return Partial{}, err
	}
	var d Point
	d.ScalarMultiplication(&r1, share.Value)
	return Partial{ID: share.ID, Point: d}, nil
}

func combinePoints(parts []Partial) (Point, error) {
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("exactly two partial decryptions required")
	}
	if parts[0].ID == parts[1].ID {
		return Point{}, fmt.Errorf("duplicate committee ID")
	}
	q := Order()
	sum := Point{}
	sum.Y.SetOne()
	for i, part := range parts {
		if part.ID < 1 || part.ID > 3 {
			return Point{}, fmt.Errorf("committee ID outside 1..3")
		}
		if err := ValidatePoint(part.Point); err != nil {
			return Point{}, err
		}
		other := parts[1-i].ID
		den := big.NewInt(int64(part.ID) - int64(other))
		den.Mod(den, q)
		inverse := new(big.Int).ModInverse(den, q)
		if inverse == nil {
			return Point{}, fmt.Errorf("invalid interpolation denominator")
		}
		lambda := new(big.Int).Mul(big.NewInt(-int64(other)), inverse)
		lambda.Mod(lambda, q)
		var term Point
		term.ScalarMultiplication(&part.Point, lambda)
		sum.Add(&sum, &term)
	}
	if err := ValidatePoint(sum); err != nil {
		return Point{}, err
	}
	return sum, nil
}

// CombineAndDecrypt does not authenticate a well-formed malicious ciphertext
// or a dishonest share. This POC relies on validated records and honest parties.
func CombineAndDecrypt(context fr.Element, ct Ciphertext, parts []Partial) ([]fr.Element, error) {
	if len(ct.Data) == 0 {
		return nil, fmt.Errorf("empty ciphertext")
	}
	if err := ValidatePoint(ct.R1); err != nil {
		return nil, err
	}
	z, err := combinePoints(parts)
	if err != nil {
		return nil, err
	}
	k := key(z, context, len(ct.Data))
	out := make([]fr.Element, len(ct.Data))
	for j := range out {
		pad := mask(k, j)
		out[j].Sub(&ct.Data[j], &pad)
	}
	return out, nil
}

// ValidateCommittee checks configuration coherence without reconstructing x.
func ValidateCommittee(pk PublicKey, shares [3]Share) error {
	if err := ValidatePoint(pk.Point); err != nil {
		return err
	}
	g := Generator()
	var parts [3]Partial
	for i, s := range shares {
		if s.ID != uint8(i+1) {
			return fmt.Errorf("committee files are not ordered by ID")
		}
		p, err := PartialDecrypt(s, g)
		if err != nil {
			return err
		}
		parts[i] = p
	}
	for _, pair := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		p, err := combinePoints([]Partial{parts[pair[0]], parts[pair[1]]})
		if err != nil {
			return err
		}
		if !p.Equal(&pk.Point) {
			return fmt.Errorf("shares do not match public configuration")
		}
	}
	return nil
}
