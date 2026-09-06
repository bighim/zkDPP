package document

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"golang.org/x/text/unicode/norm"
)

type DocumentInfo struct {
	ProductName string `json:"productName"`
	LotID       string `json:"lotId"`
}

func Encode(info DocumentInfo) ([]byte, error) {
	fields := []string{info.ProductName, info.LotID}
	var total uint64
	for _, value := range fields {
		total += 4 + uint64(len([]byte(norm.NFC.String(value))))
	}
	if total > math.MaxUint32 {
		return nil, fmt.Errorf("encoded DocumentInfo exceeds uint32 length")
	}
	out := make([]byte, 0, total)
	for _, value := range fields {
		encoded := []byte(norm.NFC.String(value))
		if uint64(len(encoded)) > math.MaxUint32 {
			return nil, fmt.Errorf("DocumentInfo field exceeds uint32 length")
		}
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(encoded)))
		out = append(out, size[:]...)
		out = append(out, encoded...)
	}
	return out, nil
}

func Hash(info DocumentInfo) (fr.Element, error) {
	encoded, err := Encode(info)
	if err != nil {
		return fr.Element{}, err
	}
	values, err := fr.Hash(encoded, []byte{}, 1)
	if err != nil {
		return fr.Element{}, fmt.Errorf("hash DocumentInfo to field: %w", err)
	}
	return values[0], nil
}
