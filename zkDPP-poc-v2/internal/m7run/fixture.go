package m7run

import (
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"path/filepath"
	"strings"
)

type FixedCase struct {
	Name           string
	Kind           audit.Kind
	Proof          string
	PublicInputs   []string
	Representative bool
}
type FixedGroup struct {
	Name          string
	Events, Extra []FixedCase
}
type Fixture struct {
	Profile, PublicKeyChecksum string
	Groups                     []FixedGroup
}

func LoadFixture(root string) (Fixture, error) {
	var f Fixture
	if e := Read(filepath.Join(root, "contracts/test/fixtures/m7-proofs.json"), &f); e != nil {
		return f, e
	}
	c, e := Committee(root)
	if e != nil {
		return f, e
	}
	if f.Profile != auditcrypto.Profile || f.PublicKeyChecksum != c.Public.Checksum {
		return f, fmt.Errorf("fixture committee mismatch")
	}
	return f, nil
}
func (f FixedCase) Decode() ([]byte, []fr.Element, audit.Record, error) {
	proof, e := hex.DecodeString(strings.TrimPrefix(f.Proof, "0x"))
	if e != nil {
		return nil, nil, audit.Record{}, e
	}
	l, e := audit.Shape(f.Kind)
	if e != nil {
		return nil, nil, audit.Record{}, e
	}
	if len(f.PublicInputs) != l.BaseCount+2+l.ParentCount+len(l.OutputTypes) {
		return nil, nil, audit.Record{}, fmt.Errorf("fixed public shape")
	}
	inputs := make([]fr.Element, len(f.PublicInputs))
	for i, s := range f.PublicInputs {
		v, e := auditcrypto.DecodeField(s)
		if e != nil {
			return nil, nil, audit.Record{}, e
		}
		inputs[i] = v
	}
	b := l.BaseCount
	ct := auditcrypto.Ciphertext{R1: auditcrypto.Point{X: inputs[b], Y: inputs[b+1]}, Data: inputs[b+2:]}
	r, e := audit.NewRecord(f.Kind, inputs[:b], ct)
	return proof, inputs[:b], r, e
}
