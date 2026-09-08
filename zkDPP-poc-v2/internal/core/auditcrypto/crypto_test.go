package auditcrypto

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

func testKeys(t *testing.T) (PublicKey, [3]Share) {
	t.Helper()
	pk, s, err := TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{1}, 1024)))
	if err != nil {
		t.Fatal(err)
	}
	return pk, s
}
func same(a, b []fr.Element) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(&b[i]) {
			return false
		}
	}
	return true
}
func TestRoundTripEveryQuorum(t *testing.T) {
	pk, shares := testKeys(t)
	if err := ValidateCommittee(pk, shares); err != nil {
		t.Fatal(err)
	}
	edge := fr.Element{}
	edge.SetBigInt(new(big.Int).Sub(fr.Modulus(), big.NewInt(1)))
	for _, n := range []int{1, 5} {
		t.Run(fmt.Sprintf("fields-%d", n), func(t *testing.T) {
			m := make([]fr.Element, n)
			for i := range m {
				m[i] = zkhash.Element(uint64(i))
			}
			m[n-1] = edge
			ct, err := Encrypt(pk, zkhash.Element(77), m, big.NewInt(7))
			if err != nil {
				t.Fatal(err)
			}
			encoded := EncodeCiphertext(ct)
			ct, err = DecodeCiphertext(encoded, n)
			if err != nil {
				t.Fatal(err)
			}
			parts := make([]Partial, 3)
			for i := range parts {
				parts[i], err = PartialDecrypt(shares[i], ct.R1)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, pair := range [][2]int{{0, 1}, {0, 2}, {1, 2}, {2, 0}} {
				out, err := CombineAndDecrypt(zkhash.Element(77), ct, []Partial{parts[pair[0]], parts[pair[1]]})
				if err != nil || !same(out, m) {
					t.Fatalf("pair %v did not restore message: %v", pair, err)
				}
			}
			if _, err = CombineAndDecrypt(zkhash.Element(77), ct, parts[:1]); err == nil {
				t.Fatal("one response accepted")
			}
			if _, err = CombineAndDecrypt(zkhash.Element(77), ct, []Partial{parts[0], parts[0]}); err == nil {
				t.Fatal("duplicate response accepted")
			}
			wrong := parts[0]
			wrong.ID = 4
			if _, err = CombineAndDecrypt(zkhash.Element(77), ct, []Partial{wrong, parts[1]}); err == nil {
				t.Fatal("unknown member accepted")
			}
			// Authentication is intentionally outside this low-level core.
			ct.Data[0].Add(&ct.Data[0], new(fr.Element).SetOne())
			out, err := CombineAndDecrypt(zkhash.Element(77), ct, parts[:2])
			if err != nil || same(out, m) {
				t.Fatal("well-formed ciphertext mutation should alter plaintext, not promise authentication")
			}
		})
	}
}
func TestValidationAndMaskSeparation(t *testing.T) {
	pk, shares := testKeys(t)
	l := zkhash.Element(1)
	m := []fr.Element{zkhash.Element(0), zkhash.Element(0)}
	for _, r := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1), Order()} {
		if _, err := Encrypt(pk, l, m, r); err == nil {
			t.Fatal("invalid nonce accepted")
		}
	}
	if _, err := Encrypt(pk, l, nil, big.NewInt(1)); err == nil {
		t.Fatal("empty message accepted")
	}
	identity := Point{}
	identity.Y.SetOne()
	torsion := Point{}
	torsion.Y.Neg(new(fr.Element).SetOne())
	for _, p := range []Point{{}, identity, torsion} {
		if ValidatePoint(p) == nil {
			t.Fatal("invalid point accepted")
		}
		if _, err := Encrypt(PublicKey{Point: p}, l, m, big.NewInt(1)); err == nil {
			t.Fatal("invalid public key accepted")
		}
		if _, err := PartialDecrypt(shares[0], p); err == nil {
			t.Fatal("invalid R1 accepted")
		}
	}
	ct, err := Encrypt(pk, l, m, big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if ct.Data[0].Equal(&ct.Data[1]) {
		t.Fatal("index masks collided in fixture")
	}
	ct2, err := Encrypt(pk, zkhash.Element(2), m, big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if same(ct.Data, ct2.Data) {
		t.Fatal("context not bound")
	}
	ct3, err := Encrypt(pk, l, m, big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	if ct.R1.Equal(&ct3.R1) || same(ct.Data, ct3.Data) {
		t.Fatal("fresh nonce ineffective")
	}
	var bad = shares
	bad[2].Value = new(big.Int).Add(bad[2].Value, big.NewInt(1))
	if ValidateCommittee(pk, bad) == nil {
		t.Fatal("incoherent committee accepted")
	}
	if _, _, err := TrustedSetup(bytes.NewReader(nil)); err == nil {
		t.Fatal("failed entropy source accepted")
	}
}
func TestCanonicalEncodingAndPersistence(t *testing.T) {
	pk, _ := testKeys(t)
	one := EncodeField(zkhash.Element(1))
	for _, s := range []string{"1", one + "00", one[:64], "0X" + one[2:], "0x" + strings.Repeat("G", 64), "0x" + fmt.Sprintf("%064x", fr.Modulus())} {
		if _, err := DecodeField(s); err == nil {
			t.Fatal("bad field encoding accepted")
		}
	}
	if _, err := DecodeScalar("0x" + fmt.Sprintf("%064x", Order())); err == nil {
		t.Fatal("scalar modulus accepted")
	}
	if _, err := DecodeCiphertext([]string{one}, 1); err == nil {
		t.Fatal("short ciphertext accepted")
	}
	if _, err := DecodePoint([2]string{one, one}); err == nil {
		t.Fatal("off-curve point accepted")
	}
	if _, err := DecodePoint(EncodePoint(pk.Point)); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "committee")
	first, created, err := EnsureCommittee(dir)
	if err != nil || !created {
		t.Fatal(err)
	}
	second, created, err := EnsureCommittee(dir)
	if err != nil || created || first.Public.Checksum != second.Public.Checksum {
		t.Fatal("setup rotated on reload", err)
	}
	for i := 1; i <= 3; i++ {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("member-%d.json", i)))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("share file permissions", err)
		}
	}
	if err := os.Chmod(filepath.Join(dir, "member-1.json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCommittee(dir); err == nil {
		t.Fatal("insecure share permissions accepted")
	}
}

func TestMasterKeyRecoveryAndBulkDecrypt(t *testing.T) {
	pk, shares := testKeys(t)
	pkg, err := PackageFromShares("m1-session", pk, shares)
	if err != nil {
		t.Fatal(err)
	}
	message := []fr.Element{zkhash.Element(0), zkhash.Element(1), zkhash.Element(2), zkhash.Element(3), zkhash.Element(4)}
	for _, pair := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		master, err := RecoverMasterKey(pkg, []Share{shares[pair[0]], shares[pair[1]]})
		if err != nil {
			t.Fatalf("pair %v: %v", pair, err)
		}
		ct, err := EncryptWithMasterPublicKey(pk, message, big.NewInt(int64(11+pair[0]+pair[1])))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := DecryptWithMasterKey(master, ct)
		master.SetInt64(0)
		if err != nil || !same(plain, message) {
			t.Fatalf("pair %v failed bulk decrypt: %v", pair, err)
		}
	}
	if _, err := RecoverMasterKey(pkg, []Share{shares[0]}); err == nil {
		t.Fatal("one share recovered master key")
	}
	if _, err := RecoverMasterKey(pkg, []Share{shares[0], shares[0]}); err == nil {
		t.Fatal("duplicate share recovered master key")
	}
	wrong := pkg
	wrong.SessionID = ""
	if _, err := RecoverMasterKey(wrong, []Share{shares[0], shares[1]}); err == nil {
		t.Fatal("invalid package accepted")
	}
}

func TestMasterEncryptionSeparatesRecordsAndPositions(t *testing.T) {
	pk, _ := testKeys(t)
	message := []fr.Element{zkhash.Element(9), zkhash.Element(9)}
	one, err := EncryptWithMasterPublicKey(pk, message, big.NewInt(21))
	if err != nil {
		t.Fatal(err)
	}
	two, err := EncryptWithMasterPublicKey(pk, message, big.NewInt(22))
	if err != nil {
		t.Fatal(err)
	}
	if one.Data[0].Equal(&one.Data[1]) {
		t.Fatal("position masks were reused")
	}
	if one.R1.Equal(&two.R1) || same(one.Data, two.Data) {
		t.Fatal("record randomness did not separate ciphertexts")
	}
}
