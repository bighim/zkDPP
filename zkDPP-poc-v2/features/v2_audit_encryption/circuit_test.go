package v2auditencryption

import (
	"math/big"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/test"
)

func TestOneAndFiveFieldRelations(t *testing.T) {
	pkg, _, err := v2case.KeyFixture()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 5} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			message := make([]fr.Element, n)
			for i := range message {
				message[i] = zkhash.Element(uint64(i + 1))
			}
			r := big.NewInt(int64(700 + n))
			ct, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, message, r)
			if err != nil {
				t.Fatal(err)
			}
			if err = test.IsSolved(New(pkg.PublicKey, n), Assignment(pkg.PublicKey, message, r, ct), ecc.BLS12_381.ScalarField()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPositionMaskTamperFails(t *testing.T) {
	pkg, _, err := v2case.KeyFixture()
	if err != nil {
		t.Fatal(err)
	}
	message := []fr.Element{zkhash.Element(1), zkhash.Element(2)}
	r := big.NewInt(77)
	ct, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, message, r)
	if err != nil {
		t.Fatal(err)
	}
	ct.Data[0] = ct.Data[1]
	if err = test.IsSolved(New(pkg.PublicKey, 2), Assignment(pkg.PublicKey, message, r, ct), ecc.BLS12_381.ScalarField()); err == nil {
		t.Fatal("mask position tamper accepted")
	}
}
