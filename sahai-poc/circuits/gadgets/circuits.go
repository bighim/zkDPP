package gadgets

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	stdhash "github.com/consensys/gnark/std/hash/poseidon2"
	stdbits "github.com/consensys/gnark/std/math/bits"
	"github.com/consensys/gnark/std/math/uints"
	stdposeidon "github.com/consensys/gnark/std/permutation/poseidon2"
)

type PoseidonMerklePathCircuit struct {
	Root        frontend.Variable    `gnark:",public"`
	Commitments [4]frontend.Variable `gnark:",public"`
	Attributes  [4]frontend.Variable
	Salts       [4]frontend.Variable
	Siblings    [3]frontend.Variable
	LeafIndex   frontend.Variable
}

func poseidonHash(api frontend.API, values ...frontend.Variable) (frontend.Variable, error) {
	hasher, err := stdhash.New(api)
	if err != nil {
		return nil, err
	}
	hasher.Write(values...)
	return hasher.Sum(), nil
}

func poseidonCompress(api frontend.API, left, right frontend.Variable) (frontend.Variable, error) {
	permutation, err := stdposeidon.NewPoseidon2(api)
	if err != nil {
		return nil, err
	}
	return permutation.Compress(left, right), nil
}

func (c *PoseidonMerklePathCircuit) Define(api frontend.API) error {
	for i := 0; i < 4; i++ {
		api.ToBinary(c.Attributes[i], 128)
		api.ToBinary(c.Salts[i], 128)
		commitment, err := poseidonHash(api, c.Attributes[i], c.Salts[i])
		if err != nil {
			return err
		}
		api.AssertIsEqual(commitment, c.Commitments[i])
	}
	current, err := poseidonHash(api, c.Attributes[:]...)
	if err != nil {
		return err
	}
	indexBits := stdbits.ToBinary(api, c.LeafIndex, stdbits.WithNbDigits(3))
	for level := 0; level < 3; level++ {
		left := api.Select(indexBits[level], c.Siblings[level], current)
		right := api.Select(indexBits[level], current, c.Siblings[level])
		current, err = poseidonCompress(api, left, right)
		if err != nil {
			return err
		}
	}
	api.AssertIsEqual(current, c.Root)
	return nil
}

type PoseidonEqCircuit struct {
	HX, HY       frontend.Variable `gnark:",public"`
	X, Y         frontend.Variable
	SaltX, SaltY frontend.Variable
}

func (c *PoseidonEqCircuit) Define(api frontend.API) error {
	for _, item := range []struct{ value, salt, expected frontend.Variable }{{c.X, c.SaltX, c.HX}, {c.Y, c.SaltY, c.HY}} {
		api.ToBinary(item.value, 128)
		api.ToBinary(item.salt, 128)
		commitment, err := poseidonHash(api, item.value, item.salt)
		if err != nil {
			return err
		}
		api.AssertIsEqual(commitment, item.expected)
	}
	api.AssertIsEqual(c.X, c.Y)
	return nil
}

type PoseidonAddCircuit struct {
	HX, HY, HZ          frontend.Variable `gnark:",public"`
	X, Y, Z             frontend.Variable
	SaltX, SaltY, SaltZ frontend.Variable
}

func (c *PoseidonAddCircuit) Define(api frontend.API) error {
	for _, item := range []struct{ value, salt, expected frontend.Variable }{{c.X, c.SaltX, c.HX}, {c.Y, c.SaltY, c.HY}, {c.Z, c.SaltZ, c.HZ}} {
		api.ToBinary(item.value, 128)
		api.ToBinary(item.salt, 128)
		commitment, err := poseidonHash(api, item.value, item.salt)
		if err != nil {
			return err
		}
		api.AssertIsEqual(commitment, item.expected)
	}
	api.AssertIsEqual(api.Add(c.X, c.Y), c.Z)
	return nil
}

type PoseidonAndCircuit PoseidonAddCircuit

func (c *PoseidonAndCircuit) Define(api frontend.API) error {
	base := (*PoseidonAddCircuit)(c)
	for _, item := range []struct{ value, salt, expected frontend.Variable }{{base.X, base.SaltX, base.HX}, {base.Y, base.SaltY, base.HY}, {base.Z, base.SaltZ, base.HZ}} {
		api.AssertIsBoolean(item.value)
		api.ToBinary(item.salt, 128)
		commitment, err := poseidonHash(api, item.value, item.salt)
		if err != nil {
			return err
		}
		api.AssertIsEqual(commitment, item.expected)
	}
	api.AssertIsEqual(api.Mul(base.X, base.Y), base.Z)
	return nil
}

type MerklePathCircuit struct {
	Root        [2]frontend.Variable    `gnark:",public"`
	Commitments [4][2]frontend.Variable `gnark:",public"`
	Attributes  [4]frontend.Variable
	Salts       [4]frontend.Variable
	Siblings    [3][2]frontend.Variable
	LeafIndex   frontend.Variable
}

func (c *MerklePathCircuit) Define(api frontend.API) error {
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	var leaf [64]uints.U8
	for i := 0; i < 4; i++ {
		encoded := valueBytes(api, uapi, c.Attributes[i])
		copy(leaf[i*16:(i+1)*16], encoded[:])
		if err := constrainCommitment(api, uapi, c.Attributes[i], c.Salts[i], c.Commitments[i]); err != nil {
			return err
		}
	}
	current := compress(api, uapi, leaf)
	indexBits := stdbits.ToBinary(api, c.LeafIndex, stdbits.WithNbDigits(3))
	for level := 0; level < 3; level++ {
		sibling := digestBytes(api, uapi, c.Siblings[level])
		var block [64]uints.U8
		for i := 0; i < 32; i++ {
			block[i] = uapi.Bytes.Select(indexBits[level], sibling[i], current[i])
			block[32+i] = uapi.Bytes.Select(indexBits[level], current[i], sibling[i])
		}
		current = compress(api, uapi, block)
	}
	assertDigest(api, uapi, current, c.Root)
	return nil
}

type EqCircuit struct {
	HX    [2]frontend.Variable `gnark:",public"`
	HY    [2]frontend.Variable `gnark:",public"`
	X     frontend.Variable
	Y     frontend.Variable
	SaltX frontend.Variable
	SaltY frontend.Variable
}

func (c *EqCircuit) Define(api frontend.API) error {
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	if err := constrainCommitment(api, uapi, c.X, c.SaltX, c.HX); err != nil {
		return err
	}
	if err := constrainCommitment(api, uapi, c.Y, c.SaltY, c.HY); err != nil {
		return err
	}
	api.AssertIsEqual(c.X, c.Y)
	return nil
}

type AddCircuit struct {
	HX    [2]frontend.Variable `gnark:",public"`
	HY    [2]frontend.Variable `gnark:",public"`
	HZ    [2]frontend.Variable `gnark:",public"`
	X     frontend.Variable
	Y     frontend.Variable
	Z     frontend.Variable
	SaltX frontend.Variable
	SaltY frontend.Variable
	SaltZ frontend.Variable
}

func (c *AddCircuit) Define(api frontend.API) error {
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		value frontend.Variable
		salt  frontend.Variable
		hash  [2]frontend.Variable
	}{{c.X, c.SaltX, c.HX}, {c.Y, c.SaltY, c.HY}, {c.Z, c.SaltZ, c.HZ}} {
		if err := constrainCommitment(api, uapi, item.value, item.salt, item.hash); err != nil {
			return fmt.Errorf("commitment: %w", err)
		}
	}
	// The commitment helper range-constrains X, Y, and Z to 128 bits. Equality
	// in the larger scalar field therefore also rejects 128-bit overflow.
	api.AssertIsEqual(api.Add(c.X, c.Y), c.Z)
	return nil
}

type AndCircuit struct {
	HX    [2]frontend.Variable `gnark:",public"`
	HY    [2]frontend.Variable `gnark:",public"`
	HZ    [2]frontend.Variable `gnark:",public"`
	X     frontend.Variable
	Y     frontend.Variable
	Z     frontend.Variable
	SaltX frontend.Variable
	SaltY frontend.Variable
	SaltZ frontend.Variable
}

func (c *AndCircuit) Define(api frontend.API) error {
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		value frontend.Variable
		salt  frontend.Variable
		hash  [2]frontend.Variable
	}{{c.X, c.SaltX, c.HX}, {c.Y, c.SaltY, c.HY}, {c.Z, c.SaltZ, c.HZ}} {
		if err := constrainCommitment(api, uapi, item.value, item.salt, item.hash); err != nil {
			return err
		}
	}
	api.AssertIsBoolean(c.X)
	api.AssertIsBoolean(c.Y)
	api.AssertIsBoolean(c.Z)
	api.AssertIsEqual(api.Mul(c.X, c.Y), c.Z)
	return nil
}
