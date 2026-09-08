package dpp

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const TagString = "zkDPP:DPP:v1"

var Tag = zkhash.MustHashToField(TagString)

type PrivateData struct {
	DocumentHash fr.Element
	AssetRole    note.AssetRole
	State        note.State
	Opening      fr.Element
	Commitment   fr.Element
}

func Commitment(documentHash fr.Element, role note.AssetRole, state note.State, opening fr.Element) fr.Element {
	return zkhash.Hash(
		Tag,
		documentHash,
		zkhash.Element(uint64(role)),
		zkhash.Element(state.QMass),
		zkhash.Element(state.ARec),
		zkhash.Element(state.E),
		opening,
	)
}

func New(documentHash fr.Element, role note.AssetRole, state note.State, opening fr.Element) (PrivateData, error) {
	if err := note.ValidateState(state, role); err != nil {
		return PrivateData{}, err
	}
	return PrivateData{
		DocumentHash: documentHash,
		AssetRole:    role,
		State:        state,
		Opening:      opening,
		Commitment:   Commitment(documentHash, role, state, opening),
	}, nil
}
