package common

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	stdhash "github.com/consensys/gnark/std/hash/poseidon2"
	stdposeidon "github.com/consensys/gnark/std/permutation/poseidon2"
)

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

func AssertUint64(api frontend.API, value frontend.Variable) {
	api.ToBinary(value, 64)
}

func AssertUint66(api frontend.API, value frontend.Variable) {
	api.ToBinary(value, 66)
}

func AssertPositiveUint64(api frontend.API, value frontend.Variable) {
	AssertUint64(api, value)
	api.AssertIsDifferent(value, 0)
}
