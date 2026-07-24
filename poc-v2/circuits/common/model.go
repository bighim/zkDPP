package common

import "github.com/consensys/gnark/frontend"

const StateLength = 3

type NoteWitness struct {
	DocumentHash frontend.Variable
	Quantity     frontend.Variable
	State        [StateLength]frontend.Variable
	Opening      frontend.Variable
	Path         MerklePath
}

type OutputWitness struct {
	DocumentHash frontend.Variable
	Quantity     frontend.Variable
	State        [StateLength]frontend.Variable
	Opening      frontend.Variable
}

type VoucherWitness struct {
	DocumentHash    frontend.Variable
	Quantity        frontend.Variable
	State           [StateLength]frontend.Variable
	SenderAddress   frontend.Variable
	ReceiverAddress frontend.Variable
	DeltaEpoch      frontend.Variable
	Opening         frontend.Variable
	Path            MerklePath
}

func DeriveOwner(api frontend.API, secretKey, publicKey, address frontend.Variable) error {
	derivedPK, err := Hash(api, secretKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(publicKey, derivedPK)
	derivedAddress, err := Hash(api, publicKey)
	if err != nil {
		return err
	}
	api.AssertIsEqual(address, derivedAddress)
	return nil
}

func Commitment(api frontend.API, note OutputWitness, address frontend.Variable) (frontend.Variable, error) {
	return Hash(api, note.DocumentHash, note.Quantity, note.State[0], note.State[1], note.State[2], address, note.Opening)
}

func VoucherCommitment(api frontend.API, voucher VoucherWitness) (frontend.Variable, error) {
	return Hash(api, voucher.DocumentHash, voucher.Quantity, voucher.State[0], voucher.State[1], voucher.State[2], voucher.SenderAddress, voucher.ReceiverAddress, voucher.DeltaEpoch, voucher.Opening)
}

func AsOutput(note NoteWitness) OutputWitness {
	return OutputWitness{DocumentHash: note.DocumentHash, Quantity: note.Quantity, State: note.State, Opening: note.Opening}
}
