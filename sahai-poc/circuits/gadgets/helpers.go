package gadgets

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/frontend"
	sha2hash "github.com/consensys/gnark/std/hash/sha2"
	stdbits "github.com/consensys/gnark/std/math/bits"
	"github.com/consensys/gnark/std/math/uints"
	sha2perm "github.com/consensys/gnark/std/permutation/sha2"
)

var sha256IV = uints.NewU32Array([]uint32{
	0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A,
	0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19,
})

func valueBytes(api frontend.API, uapi *uints.BinaryField[uints.U32], value frontend.Variable) [16]uints.U8 {
	decomposition := stdbits.ToBinary(api, value, stdbits.WithNbDigits(128))
	var out [16]uints.U8
	for byteIndex := 0; byteIndex < 16; byteIndex++ {
		littleEndianOffset := (15 - byteIndex) * 8
		byteValue := stdbits.FromBinary(api, decomposition[littleEndianOffset:littleEndianOffset+8])
		out[byteIndex] = uapi.ByteValueOf(byteValue)
	}
	return out
}

func digestBytes(api frontend.API, uapi *uints.BinaryField[uints.U32], limbs [2]frontend.Variable) [32]uints.U8 {
	hi := valueBytes(api, uapi, limbs[0])
	lo := valueBytes(api, uapi, limbs[1])
	var out [32]uints.U8
	copy(out[:16], hi[:])
	copy(out[16:], lo[:])
	return out
}

func pack16(api frontend.API, uapi *uints.BinaryField[uints.U32], bytes []uints.U8) frontend.Variable {
	if len(bytes) != 16 {
		panic("pack16 requires 16 bytes")
	}
	terms := make([]frontend.Variable, 16)
	for i := range bytes {
		coefficient := new(big.Int).Lsh(big.NewInt(1), uint(8*(15-i)))
		terms[i] = api.Mul(uapi.Bytes.Value(bytes[i]), coefficient)
	}
	return api.Add(terms[0], terms[1], terms[2:]...)
}

func assertDigest(api frontend.API, uapi *uints.BinaryField[uints.U32], digest []uints.U8, expected [2]frontend.Variable) {
	api.AssertIsEqual(pack16(api, uapi, digest[:16]), expected[0])
	api.AssertIsEqual(pack16(api, uapi, digest[16:]), expected[1])
}

func attributeHash(api frontend.API, uapi *uints.BinaryField[uints.U32], value, salt frontend.Variable) ([]uints.U8, error) {
	valueEncoded := valueBytes(api, uapi, value)
	saltEncoded := valueBytes(api, uapi, salt)
	hasher, err := sha2hash.New(api)
	if err != nil {
		return nil, fmt.Errorf("new SHA-256: %w", err)
	}
	hasher.Write(append(valueEncoded[:], saltEncoded[:]...))
	return hasher.Sum(), nil
}

func compress(api frontend.API, uapi *uints.BinaryField[uints.U32], input [64]uints.U8) []uints.U8 {
	var state [8]uints.U32
	copy(state[:], sha256IV)
	result := sha2perm.Permute(uapi, state, input)
	out := make([]uints.U8, 0, 32)
	for i := range result {
		out = append(out, uapi.UnpackMSB(result[i])...)
	}
	return out
}

func constrainCommitment(
	api frontend.API,
	uapi *uints.BinaryField[uints.U32],
	value, salt frontend.Variable,
	expected [2]frontend.Variable,
) error {
	digest, err := attributeHash(api, uapi, value, salt)
	if err != nil {
		return err
	}
	assertDigest(api, uapi, digest, expected)
	return nil
}
