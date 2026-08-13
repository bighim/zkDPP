package document

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"math/bits"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	poseidon2 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr/poseidon2"
)

type Profile string

const (
	Poseidon2 Profile = "poseidon2"
	SHA256    Profile = "sha256"
)

func ParseProfile(value string) (Profile, error) {
	switch Profile(value) {
	case Poseidon2, SHA256:
		return Profile(value), nil
	default:
		return "", fmt.Errorf("unknown hash profile %q", value)
	}
}

type Value128 [16]byte
type Digest [32]byte

var initialState = [8]uint32{
	0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A,
	0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19,
}

var roundConstants = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

func AttributeCommitment(profile Profile, value, salt Value128) Digest {
	if profile == Poseidon2 {
		return PoseidonHash(valueElement(value), valueElement(salt))
	}
	var input [32]byte
	copy(input[:16], value[:])
	copy(input[16:], salt[:])
	return sha256.Sum256(input[:])
}

func PoseidonLeaf(values [4]Value128) Digest {
	elements := make([]fr.Element, len(values))
	for i := range values {
		elements[i] = valueElement(values[i])
	}
	return PoseidonHash(elements...)
}

func PoseidonHash(values ...fr.Element) Digest {
	hasher := poseidon2.NewMerkleDamgardHasher()
	for i := range values {
		encoded := values[i].Bytes()
		_, _ = hasher.Write(encoded[:])
	}
	var out Digest
	copy(out[:], hasher.Sum(nil))
	return out
}

func PoseidonCompress(left, right Digest) Digest {
	permutation := poseidon2.NewDefaultPermutation()
	encoded, err := permutation.Compress(left[:], right[:])
	if err != nil {
		panic(err)
	}
	var out Digest
	copy(out[:], encoded)
	return out
}

// SHACompress applies one SHA-256 compression block from the standard IV without padding.
func SHACompress(block [64]byte) Digest {
	var schedule [64]uint32
	for i := 0; i < 16; i++ {
		schedule[i] = binary.BigEndian.Uint32(block[i*4 : i*4+4])
	}
	for i := 16; i < 64; i++ {
		v1 := schedule[i-2]
		s1 := bits.RotateLeft32(v1, -17) ^ bits.RotateLeft32(v1, -19) ^ (v1 >> 10)
		v2 := schedule[i-15]
		s0 := bits.RotateLeft32(v2, -7) ^ bits.RotateLeft32(v2, -18) ^ (v2 >> 3)
		schedule[i] = s1 + schedule[i-7] + s0 + schedule[i-16]
	}
	a, b, c, d := initialState[0], initialState[1], initialState[2], initialState[3]
	e, f, g, h := initialState[4], initialState[5], initialState[6], initialState[7]
	for i := 0; i < 64; i++ {
		t1 := h + (bits.RotateLeft32(e, -6) ^ bits.RotateLeft32(e, -11) ^ bits.RotateLeft32(e, -25)) +
			((e & f) ^ (^e & g)) + roundConstants[i] + schedule[i]
		t2 := (bits.RotateLeft32(a, -2) ^ bits.RotateLeft32(a, -13) ^ bits.RotateLeft32(a, -22)) +
			((a & b) ^ (a & c) ^ (b & c))
		h, g, f, e, d, c, b, a = g, f, e, d+t1, c, b, a, t1+t2
	}
	state := [8]uint32{
		initialState[0] + a, initialState[1] + b, initialState[2] + c, initialState[3] + d,
		initialState[4] + e, initialState[5] + f, initialState[6] + g, initialState[7] + h,
	}
	var digest Digest
	for i := range state {
		binary.BigEndian.PutUint32(digest[i*4:i*4+4], state[i])
	}
	return digest
}

func DigestPublic(profile Profile, digest Digest) []*big.Int {
	if profile == Poseidon2 {
		return []*big.Int{new(big.Int).SetBytes(digest[:])}
	}
	return []*big.Int{new(big.Int).SetBytes(digest[:16]), new(big.Int).SetBytes(digest[16:])}
}

func ValueFromUint64(value uint64) Value128 {
	var out Value128
	binary.BigEndian.PutUint64(out[8:], value)
	return out
}

func valueElement(value Value128) fr.Element {
	var encoded [32]byte
	copy(encoded[16:], value[:])
	var out fr.Element
	out.SetBytes(encoded[:])
	return out
}
