package auditcrypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

func EncodeField(v fr.Element) string { b := v.Bytes(); return "0x" + hex.EncodeToString(b[:]) }
func decodeWord(s string) ([]byte, error) {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") || strings.ToLower(s) != s {
		return nil, fmt.Errorf("expected canonical 32-byte lowercase hex")
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil {
		return nil, fmt.Errorf("invalid word encoding")
	}
	return b, nil
}
func DecodeField(s string) (fr.Element, error) {
	var v fr.Element
	b, err := decodeWord(s)
	if err != nil {
		return v, err
	}
	if err = v.SetBytesCanonical(b); err != nil {
		return fr.Element{}, fmt.Errorf("field outside canonical range")
	}
	return v, nil
}
func EncodeScalar(v *big.Int) (string, error) {
	if err := ValidateScalar(v, false); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	v.FillBytes(b)
	return "0x" + hex.EncodeToString(b), nil
}
func DecodeScalar(s string) (*big.Int, error) {
	b, err := decodeWord(s)
	if err != nil {
		return nil, err
	}
	v := new(big.Int).SetBytes(b)
	if err = ValidateScalar(v, false); err != nil {
		return nil, err
	}
	return v, nil
}
func EncodePoint(p Point) [2]string { return [2]string{EncodeField(p.X), EncodeField(p.Y)} }
func DecodePoint(words [2]string) (Point, error) {
	var p Point
	x, err := DecodeField(words[0])
	if err != nil {
		return p, err
	}
	y, err := DecodeField(words[1])
	if err != nil {
		return p, err
	}
	p.X, p.Y = x, y
	if err = ValidatePoint(p); err != nil {
		return Point{}, err
	}
	return p, nil
}
func EncodeCiphertext(ct Ciphertext) []string {
	v := []string{EncodeField(ct.R1.X), EncodeField(ct.R1.Y)}
	for _, c := range ct.Data {
		v = append(v, EncodeField(c))
	}
	return v
}
func DecodeCiphertext(words []string, n int) (Ciphertext, error) {
	if n < 1 || len(words) != n+2 {
		return Ciphertext{}, fmt.Errorf("ciphertext length mismatch")
	}
	p, err := DecodePoint([2]string{words[0], words[1]})
	if err != nil {
		return Ciphertext{}, err
	}
	out := Ciphertext{R1: p, Data: make([]fr.Element, n)}
	for i := range out.Data {
		out.Data[i], err = DecodeField(words[i+2])
		if err != nil {
			return Ciphertext{}, err
		}
	}
	return out, nil
}
func PublicKeyChecksum(pk PublicKey) string {
	x, y := pk.Point.X.Bytes(), pk.Point.Y.Bytes()
	h := sha256.New()
	h.Write([]byte(Profile))
	h.Write(x[:])
	h.Write(y[:])
	return hex.EncodeToString(h.Sum(nil))
}

type PublicConfig struct {
	Profile     string    `json:"profile"`
	Curve       string    `json:"curve"`
	Threshold   int       `json:"threshold"`
	Members     int       `json:"members"`
	PK          [2]string `json:"publicKey"`
	Checksum    string    `json:"publicKeyChecksum"`
	SetupMillis float64   `json:"setupMillis"`
}
type privateFile struct {
	ID       uint8  `json:"id"`
	Scalar   string `json:"scalar"`
	Checksum string `json:"publicKeyChecksum"`
}
type Committee struct {
	Public PublicConfig
	PK     PublicKey
	Shares [3]Share
}

func readJSON(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return fmt.Errorf("invalid configuration JSON")
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing configuration data")
	}
	return nil
}
func writeNewJSON(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(b, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
func LoadPublic(dir string) (PublicConfig, PublicKey, error) {
	var c PublicConfig
	if err := readJSON(filepath.Join(dir, "public.json"), &c); err != nil {
		return c, PublicKey{}, err
	}
	if c.Profile != Profile || c.Curve != "Jubjub" || c.Threshold != 2 || c.Members != 3 {
		return c, PublicKey{}, fmt.Errorf("public configuration profile mismatch")
	}
	p, err := DecodePoint(c.PK)
	if err != nil {
		return c, PublicKey{}, err
	}
	pk := PublicKey{Point: p}
	if PublicKeyChecksum(pk) != c.Checksum {
		return c, PublicKey{}, fmt.Errorf("public key checksum mismatch")
	}
	return c, pk, nil
}
func LoadCommittee(dir string) (Committee, error) {
	var out Committee
	c, pk, err := LoadPublic(dir)
	if err != nil {
		return out, err
	}
	out.Public, out.PK = c, pk
	for i := range out.Shares {
		path := filepath.Join(dir, fmt.Sprintf("member-%d.json", i+1))
		info, err := os.Stat(path)
		if err != nil {
			return Committee{}, err
		}
		if info.Mode().Perm()&0077 != 0 {
			return Committee{}, fmt.Errorf("private share permissions must exclude group/other")
		}
		var disk privateFile
		if err = readJSON(path, &disk); err != nil {
			return Committee{}, err
		}
		if disk.ID != uint8(i+1) || disk.Checksum != c.Checksum {
			return Committee{}, fmt.Errorf("private share configuration mismatch")
		}
		v, err := DecodeScalar(disk.Scalar)
		if err != nil {
			return Committee{}, err
		}
		out.Shares[i] = Share{ID: disk.ID, Value: v}
	}
	if err = ValidateCommittee(pk, out.Shares); err != nil {
		return Committee{}, err
	}
	return out, nil
}

// EnsureCommittee never overwrites an existing setup or silently rotates keys.
func EnsureCommittee(dir string) (Committee, bool, error) {
	if _, err := os.Stat(filepath.Join(dir, "public.json")); err == nil {
		c, e := LoadCommittee(dir)
		return c, false, e
	} else if !errors.Is(err, os.ErrNotExist) {
		return Committee{}, false, err
	}
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		return Committee{}, false, fmt.Errorf("incomplete committee setup; refusing replacement")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Committee{}, false, err
	}
	start := time.Now()
	pk, shares, err := TrustedSetup(nil)
	elapsed := time.Since(start)
	if err != nil {
		return Committee{}, false, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return Committee{}, false, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return Committee{}, false, err
	}
	config := PublicConfig{Profile: Profile, Curve: "Jubjub", Threshold: 2, Members: 3, PK: EncodePoint(pk.Point), Checksum: PublicKeyChecksum(pk), SetupMillis: float64(elapsed.Nanoseconds()) / 1e6}
	for i, s := range shares {
		encoded, e := EncodeScalar(s.Value)
		if e != nil {
			return Committee{}, false, e
		}
		v := privateFile{ID: s.ID, Scalar: encoded, Checksum: config.Checksum}
		if e = writeNewJSON(filepath.Join(dir, fmt.Sprintf("member-%d.json", i+1)), v, 0600); e != nil {
			return Committee{}, false, e
		}
	}
	if err = writeNewJSON(filepath.Join(dir, "public.json"), config, 0644); err != nil {
		return Committee{}, false, err
	}
	out, err := LoadCommittee(dir)
	return out, true, err
}
