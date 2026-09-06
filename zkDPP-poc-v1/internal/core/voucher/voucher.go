package voucher

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/allocation"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Voucher struct {
	DocumentHash    fr.Element
	AssetRole       note.AssetRole
	State           note.State
	SenderAddress   fr.Element
	ReceiverAddress fr.Element
	DeadlineEpoch   uint64
	Opening         fr.Element
	Commitment      fr.Element
}

type Remainder = allocation.Remainder

func Commitment(documentHash fr.Element, role note.AssetRole, state note.State, sender, receiver fr.Element, deadline uint64, opening fr.Element) fr.Element {
	return zkhash.Hash(
		zkhash.VoucherTag,
		documentHash,
		zkhash.Element(uint64(role)),
		zkhash.Element(state.QMass),
		zkhash.Element(state.ARec),
		zkhash.Element(state.E),
		sender,
		receiver,
		zkhash.Element(deadline),
		opening,
	)
}

func New(documentHash fr.Element, role note.AssetRole, state note.State, sender, receiver fr.Element, deadline uint64, opening fr.Element) (Voucher, error) {
	if state.QMass == 0 {
		return Voucher{}, fmt.Errorf("voucher q_mass must be positive")
	}
	if err := note.ValidateState(state, role); err != nil {
		return Voucher{}, err
	}
	return Voucher{
		DocumentHash: documentHash, AssetRole: role, State: state,
		SenderAddress: sender, ReceiverAddress: receiver, DeadlineEpoch: deadline, Opening: opening,
		Commitment: Commitment(documentHash, role, state, sender, receiver, deadline, opening),
	}, nil
}

func Nullifier(opening, commitment fr.Element) fr.Element {
	return zkhash.Hash(zkhash.VoucherNFTag, opening, commitment)
}

func Allocate(input note.State, qVoucher uint64) (voucherState, changeState note.State, remainder Remainder, err error) {
	if qVoucher == 0 {
		return voucherState, changeState, remainder, fmt.Errorf("invalid transfer mass")
	}
	return allocation.ByMass(input, qVoucher)
}
