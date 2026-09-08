package note

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	MassScale   = uint64(1_000_000_000)
	CarbonScale = uint64(1_000_000_000)
)

type AssetRole uint8

const (
	AssetRoleEligible AssetRole = iota
	AssetRoleWaste
)

type State struct {
	QMass uint64 `json:"qMass"`
	ARec  uint64 `json:"aRec"`
	E     uint64 `json:"e"`
}

type Note struct {
	DocumentHash fr.Element
	State        State
	AssetRole    AssetRole
	Address      fr.Element
	Opening      fr.Element
	Commitment   fr.Element
}

func ValidateState(state State, role AssetRole) error {
	if state.ARec > state.QMass {
		return fmt.Errorf("a_rec=%d exceeds q_mass=%d", state.ARec, state.QMass)
	}
	switch role {
	case AssetRoleEligible:
		return nil
	case AssetRoleWaste:
		if state.ARec != 0 || state.E != 0 {
			return fmt.Errorf("WASTE requires a_rec=0 and e=0")
		}
		return nil
	default:
		return fmt.Errorf("unknown AssetRole %d", role)
	}
}

func Commitment(documentHash fr.Element, state State, role AssetRole, address, opening fr.Element) fr.Element {
	return zkhash.Hash(
		zkhash.NoteTag,
		documentHash,
		zkhash.Element(uint64(role)),
		zkhash.Element(state.QMass),
		zkhash.Element(state.ARec),
		zkhash.Element(state.E),
		address,
		opening,
	)
}

func New(documentHash fr.Element, state State, role AssetRole, address, opening fr.Element) (Note, error) {
	if err := ValidateState(state, role); err != nil {
		return Note{}, err
	}
	return Note{
		DocumentHash: documentHash,
		State:        state, AssetRole: role, Address: address, Opening: opening,
		Commitment: Commitment(documentHash, state, role, address, opening),
	}, nil
}

func Nullifier(skOwner, commitment fr.Element) fr.Element {
	return zkhash.Hash(zkhash.NullifierTag, commitment, skOwner)
}
