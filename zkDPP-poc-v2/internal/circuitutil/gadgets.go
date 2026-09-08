package circuitutil

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/consensys/gnark/frontend"
	stdhash "github.com/consensys/gnark/std/hash/poseidon2"
	stdposeidon "github.com/consensys/gnark/std/permutation/poseidon2"
)

type NoteWitness struct {
	DocumentHash frontend.Variable
	AssetRole    frontend.Variable
	QMass        frontend.Variable
	ARec         frontend.Variable
	E            frontend.Variable
	Address      frontend.Variable
	Opening      frontend.Variable
}

type VoucherWitness struct {
	DocumentHash    frontend.Variable
	AssetRole       frontend.Variable
	QMass           frontend.Variable
	ARec            frontend.Variable
	E               frontend.Variable
	SenderAddress   frontend.Variable
	ReceiverAddress frontend.Variable
	DeadlineBlock   frontend.Variable
	Opening         frontend.Variable
}

type MerklePath struct {
	Index    frontend.Variable
	Siblings [merkle.Depth]frontend.Variable
}

type StatusPath struct {
	Siblings [merkle.Depth]frontend.Variable
}

func Hash(api frontend.API, inputs ...frontend.Variable) (frontend.Variable, error) {
	hasher, err := stdhash.New(api)
	if err != nil {
		return nil, fmt.Errorf("construct Poseidon2 hasher: %w", err)
	}
	hasher.Write(inputs...)
	return hasher.Sum(), nil
}

func Compress(api frontend.API, left, right frontend.Variable) (frontend.Variable, error) {
	permutation, err := stdposeidon.NewPoseidon2(api)
	if err != nil {
		return nil, fmt.Errorf("construct Poseidon2 permutation: %w", err)
	}
	return permutation.Compress(left, right), nil
}

func AssertStateAndRole(api frontend.API, qMass, aRec, carbon, role frontend.Variable) {
	api.ToBinary(qMass, 64)
	api.ToBinary(aRec, 64)
	api.ToBinary(carbon, 64)
	api.AssertIsLessOrEqual(aRec, qMass)
	api.AssertIsEqual(api.Mul(role, api.Sub(role, 1)), 0)
	api.AssertIsEqual(api.Mul(role, aRec), 0)
	api.AssertIsEqual(api.Mul(role, carbon), 0)
}

func AssertNote(api frontend.API, note NoteWitness) {
	AssertStateAndRole(api, note.QMass, note.ARec, note.E, note.AssetRole)
}

func AssertVoucher(api frontend.API, voucher VoucherWitness) {
	AssertStateAndRole(api, voucher.QMass, voucher.ARec, voucher.E, voucher.AssetRole)
	api.ToBinary(voucher.DeadlineBlock, 64)
}

func Address(api frontend.API, skOwner frontend.Variable) (frontend.Variable, error) {
	return Hash(api, zkhash.OwnerTag, skOwner)
}

func Commitment(api frontend.API, note NoteWitness) (frontend.Variable, error) {
	return Hash(
		api,
		zkhash.NoteTag,
		note.DocumentHash,
		note.AssetRole,
		note.QMass,
		note.ARec,
		note.E,
		note.Address,
		note.Opening,
	)
}

func Nullifier(api frontend.API, skOwner, commitment frontend.Variable) (frontend.Variable, error) {
	return Hash(api, zkhash.NullifierTag, commitment, skOwner)
}

func VoucherCommitment(api frontend.API, voucher VoucherWitness) (frontend.Variable, error) {
	return Hash(
		api,
		zkhash.VoucherTag,
		voucher.DocumentHash,
		voucher.AssetRole,
		voucher.QMass,
		voucher.ARec,
		voucher.E,
		voucher.SenderAddress,
		voucher.ReceiverAddress,
		voucher.DeadlineBlock,
		voucher.Opening,
	)
}

func VoucherNullifier(api frontend.API, opening, commitment frontend.Variable) (frontend.Variable, error) {
	resolutionSecret, err := Hash(api, zkhash.VoucherResolutionSecretTag, opening)
	if err != nil {
		return nil, err
	}
	return Hash(api, zkhash.VoucherNFTag, commitment, resolutionSecret)
}

func AssertMembership(api frontend.API, root, leaf frontend.Variable, path MerklePath) error {
	bits := api.ToBinary(path.Index, merkle.Depth)
	return assertMerkleRoot(api, root, leaf, bits, path.Siblings)
}

func AssertStatusActive(api frontend.API, root, index frontend.Variable, path StatusPath) error {
	bits := api.ToBinary(index, merkle.Depth)
	return assertMerkleRoot(api, root, 0, bits, path.Siblings)
}

func AssertStatusRoot(api frontend.API, root, leaf frontend.Variable, index frontend.Variable, path StatusPath) error {
	bits := api.ToBinary(index, merkle.Depth)
	return assertMerkleRoot(api, root, leaf, bits, path.Siblings)
}

func assertMerkleRoot(api frontend.API, root, leaf frontend.Variable, bits []frontend.Variable, siblings [merkle.Depth]frontend.Variable) error {
	current := leaf
	for level := 0; level < merkle.Depth; level++ {
		left := api.Select(bits[level], siblings[level], current)
		right := api.Select(bits[level], current, siblings[level])
		parent, err := Compress(api, left, right)
		if err != nil {
			return err
		}
		current = parent
	}
	api.AssertIsEqual(current, root)
	return nil
}
