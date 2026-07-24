package zkhash

import (
	"encoding/binary"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	poseidon2 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr/poseidon2"
	"golang.org/x/text/unicode/norm"
)

func Element(v uint64) fr.Element {
	var out fr.Element
	out.SetUint64(v)
	return out
}

func Hash(inputs ...fr.Element) fr.Element {
	h := poseidon2.NewMerkleDamgardHasher()
	for i := range inputs {
		bytes := inputs[i].Bytes()
		_, _ = h.Write(bytes[:])
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

func EncodeDocumentInfo(productName, lotID, unit string) ([]byte, error) {
	fields := []string{productName, lotID, unit}
	var size uint64
	for _, value := range fields {
		size += 4 + uint64(len([]byte(norm.NFC.String(value))))
	}
	if size > uint64(^uint32(0)) {
		return nil, fmt.Errorf("encoded DocumentInfo exceeds uint32 length")
	}
	out := make([]byte, 0, size)
	for _, value := range fields {
		encoded := []byte(norm.NFC.String(value))
		if uint64(len(encoded)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("DocumentInfo field exceeds uint32 length")
		}
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(encoded)))
		out = append(out, length[:]...)
		out = append(out, encoded...)
	}
	return out, nil
}

func DocumentHash(productName, lotID, unit string) (fr.Element, error) {
	encoded, err := EncodeDocumentInfo(productName, lotID, unit)
	if err != nil {
		return fr.Element{}, err
	}
	values, err := fr.Hash(encoded, []byte{}, 1)
	if err != nil {
		return fr.Element{}, fmt.Errorf("hash DocumentInfo to field: %w", err)
	}
	return values[0], nil
}
