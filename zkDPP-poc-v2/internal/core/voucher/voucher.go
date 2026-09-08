package voucher

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/bits"
)

type Voucher struct {
	DocumentHash    fr.Element
	AssetRole       note.AssetRole
	State           note.State
	SenderAddress   fr.Element
	ReceiverAddress fr.Element
	DeadlineBlock   uint64
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
		SenderAddress: sender, ReceiverAddress: receiver, DeadlineBlock: deadline, Opening: opening,
		Commitment: Commitment(documentHash, role, state, sender, receiver, deadline, opening),
	}, nil
}

func Nullifier(opening, commitment fr.Element) fr.Element {
	resolutionSecret := zkhash.Hash(zkhash.VoucherResolutionSecretTag, opening)
	return zkhash.Hash(zkhash.VoucherNFTag, commitment, resolutionSecret)
}

func ResolutionSecret(opening fr.Element) fr.Element {
	return zkhash.Hash(zkhash.VoucherResolutionSecretTag, opening)
}

func Allocate(input note.State, qVoucher uint64) (voucherState, changeState note.State, remainder Remainder, err error) {
	if qVoucher == 0 {
		return voucherState, changeState, remainder, fmt.Errorf("invalid transfer mass")
	}
	return allocation.ByMass(input, qVoucher)
}

func AllocateWithTransport(input note.State, qVoucher, deltaETransport uint64) (voucherState, changeState note.State, remainder Remainder, err error) {
	voucherState, changeState, remainder, err = Allocate(input, qVoucher)
	if err != nil {
		return voucherState, changeState, remainder, err
	}
	transported, carry := bits.Add64(voucherState.E, deltaETransport, 0)
	if carry != 0 {
		return note.State{}, note.State{}, Remainder{}, fmt.Errorf("transport carbon overflow")
	}
	voucherState.E = transported
	return voucherState, changeState, remainder, nil
}
