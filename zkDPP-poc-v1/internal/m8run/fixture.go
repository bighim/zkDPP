package m8run

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type FixedProof struct {
	Name         string
	Artifact     string
	Proof        string
	PublicInputs []string
}

type Fixture struct {
	Profile, PublicKeyChecksum, M7FixtureChecksum string
	EligibleExit, WasteExit, Standard, Strict     FixedProof
	StandardVKHash, StrictVKHash                  string
}

func LoadFixture(root string) (Fixture, error) {
	var f Fixture
	if err := Read(filepath.Join(root, "contracts/test/fixtures/m8-proofs.json"), &f); err != nil {
		return f, err
	}
	c, err := Committee(root)
	if err != nil {
		return f, err
	}
	if f.Profile != auditcrypto.Profile || f.PublicKeyChecksum != c.Public.Checksum {
		return f, fmt.Errorf("M8 fixture committee mismatch")
	}
	return f, nil
}

func (p FixedProof) Decode() ([]byte, []fr.Element, error) {
	proof, err := hex.DecodeString(strings.TrimPrefix(p.Proof, "0x"))
	if err != nil {
		return nil, nil, err
	}
	inputs := make([]fr.Element, len(p.PublicInputs))
	for i, s := range p.PublicInputs {
		inputs[i], err = auditcrypto.DecodeField(s)
		if err != nil {
			return nil, nil, err
		}
	}
	return proof, inputs, nil
}
