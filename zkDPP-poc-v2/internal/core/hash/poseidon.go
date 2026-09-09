package zkhash

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	poseidon2 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr/poseidon2"
)

const (
	NoteTagString                    = "zkDPP:Note:v2"
	OwnerTagString                   = "zkDPP:Owner:v2"
	NullifierTagString               = "zkDPP:Nullifier:v2"
	VoucherTagString                 = "zkDPP:Voucher:v2"
	VoucherNFTagString               = "zkDPP:VoucherNullifier:v2"
	PolicyRefTagString               = "zkDPP:PolicyRef:v2"
	ScopeRefTagString                = "zkDPP:ScopeRef:v2"
	VoucherResolutionSecretTagString = "zkDPP:VoucherResolutionSecret:v2"
	IssueTagString                   = "zkDPP:Issue:v2"
)

func Element(value uint64) fr.Element {
	var out fr.Element
	out.SetUint64(value)
	return out
}

func Hash(inputs ...fr.Element) fr.Element {
	h := poseidon2.NewMerkleDamgardHasher()
	for i := range inputs {
		encoded := inputs[i].Bytes()
		_, _ = h.Write(encoded[:])
	}
	digest := h.Sum(nil)
	var out fr.Element
	if err := out.SetBytesCanonical(digest); err != nil {
		panic(fmt.Sprintf("poseidon2 digest is not canonical: %v", err))
	}
	return out
}

func Compress(left, right fr.Element) fr.Element {
	permutation := poseidon2.NewDefaultPermutation()
	leftBytes := left.Bytes()
	rightBytes := right.Bytes()
	digest, err := permutation.Compress(leftBytes[:], rightBytes[:])
	if err != nil {
		panic(fmt.Sprintf("poseidon2 compression failed: %v", err))
	}
	var out fr.Element
	if err := out.SetBytesCanonical(digest); err != nil {
		panic(fmt.Sprintf("poseidon2 compressed value is not canonical: %v", err))
	}
	return out
}

func HashToField(value string) (fr.Element, error) {
	fields, err := fr.Hash([]byte(value), []byte{}, 1)
	if err != nil {
		return fr.Element{}, fmt.Errorf("hash %q to field: %w", value, err)
	}
	return fields[0], nil
}

func MustHashToField(value string) fr.Element {
	out, err := HashToField(value)
	if err != nil {
		panic(err)
	}
	return out
}

var (
	NoteTag                    = MustHashToField(NoteTagString)
	OwnerTag                   = MustHashToField(OwnerTagString)
	NullifierTag               = MustHashToField(NullifierTagString)
	VoucherTag                 = MustHashToField(VoucherTagString)
	VoucherNFTag               = MustHashToField(VoucherNFTagString)
	PolicyRefTag               = MustHashToField(PolicyRefTagString)
	ScopeRefTag                = MustHashToField(ScopeRefTagString)
	VoucherResolutionSecretTag = MustHashToField(VoucherResolutionSecretTagString)
	IssueTag                   = MustHashToField(IssueTagString)
)
