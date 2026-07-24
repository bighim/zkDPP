package common_test

import (
	"testing"

	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/bighim/zkDPP/poc-v2/internal/merkle"
	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

type hashReferenceCircuit struct {
	Hash12     frontend.Variable `gnark:",public"`
	Compress00 frontend.Variable `gnark:",public"`
	Compress12 frontend.Variable `gnark:",public"`
}

func (c *hashReferenceCircuit) Define(api frontend.API) error {
	one, two := frontend.Variable(1), frontend.Variable(2)
	hash12, err := common.Hash(api, one, two)
	if err != nil {
		return err
	}
	compress00, err := common.Compress(api, 0, 0)
	if err != nil {
		return err
	}
	compress12, err := common.Compress(api, one, two)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Hash12, hash12)
	api.AssertIsEqual(c.Compress00, compress00)
	api.AssertIsEqual(c.Compress12, compress12)
	return nil
}

func TestNativeAndCircuitPoseidon2Match(t *testing.T) {
	assignment := &hashReferenceCircuit{
		Hash12:     zkhash.Hash(zkhash.Element(1), zkhash.Element(2)),
		Compress00: zkhash.Compress(zkhash.Element(0), zkhash.Element(0)),
		Compress12: zkhash.Compress(zkhash.Element(1), zkhash.Element(2)),
	}
	if err := test.IsSolved(&hashReferenceCircuit{}, assignment, ecc.BLS12_381.ScalarField()); err != nil {
		t.Fatal(err)
	}
}

type membershipReferenceCircuit struct {
	Root frontend.Variable `gnark:",public"`
	Leaf frontend.Variable `gnark:",public"`
	Path common.MerklePath
}

func (c *membershipReferenceCircuit) Define(api frontend.API) error {
	return common.AssertMembership(api, c.Root, c.Leaf, c.Path)
}

func TestNativeAndCircuitDepth32MembershipMatch(t *testing.T) {
	tree := merkle.New(32)
	for _, value := range []uint64{11, 22, 33} {
		if _, _, err := tree.Append(zkhash.Element(value)); err != nil {
			t.Fatal(err)
		}
	}
	path, err := tree.Path(1)
	if err != nil {
		t.Fatal(err)
	}
	assignment := &membershipReferenceCircuit{Root: tree.Root(), Leaf: zkhash.Element(22)}
	assignment.Path.Index = path.Index
	for i := range path.Siblings {
		assignment.Path.Siblings[i] = path.Siblings[i]
	}
	if err := test.IsSolved(&membershipReferenceCircuit{}, assignment, ecc.BLS12_381.ScalarField()); err != nil {
		t.Fatal(err)
	}
}
