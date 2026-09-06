package m9run

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type FixedProof struct {
	Name, Relation string
	EventKind      uint8
	Epoch          uint64
	Proof          string
	PublicInputs   []string
}
type Fixture struct {
	Profile, PublicKeyChecksum                                                                          string
	Events                                                                                              []FixedProof
	ProductExit, WasteExit, Standard, Strict                                                            FixedProof
	EntryAluminumA, Product1, Product2, FirstVoucherNF, ProcessInputNF, FinalNoteRoot, FinalVoucherRoot string
	FinalNoteCount, FinalVoucherCount                                                                   uint64
}

func LoadFixture(root string) (Fixture, error) {
	var f Fixture
	b, e := os.ReadFile(filepath.Join(root, "contracts/test/fixtures/m9-proofs.json"))
	if e != nil {
		return f, e
	}
	if e = json.Unmarshal(b, &f); e != nil {
		return f, e
	}
	c, e := Committee(root)
	if e != nil {
		return f, e
	}
	if f.Profile != auditcrypto.Profile || f.PublicKeyChecksum != c.Public.Checksum {
		return f, fmt.Errorf("M9 fixture committee mismatch")
	}
	return f, nil
}
func (p FixedProof) Decode() ([]byte, []fr.Element, error) {
	proof, e := hex.DecodeString(strings.TrimPrefix(p.Proof, "0x"))
	if e != nil {
		return nil, nil, e
	}
	inputs := make([]fr.Element, len(p.PublicInputs))
	for i, s := range p.PublicInputs {
		inputs[i], e = auditcrypto.DecodeField(s)
		if e != nil {
			return nil, nil, e
		}
	}
	return proof, inputs, nil
}
