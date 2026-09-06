package solgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	curve "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
)

const OptimizedVKWords = 38

type OptimizedVK struct {
	Words      []string `json:"words"`
	ByteLength int      `json:"byteLength"`
	SHA256     string   `json:"sha256"`
}

func ExtractOptimizedVK(vk *plonkbls.VerifyingKey) (OptimizedVK, error) {
	if len(vk.Qcp) != 0 || len(vk.CommitmentConstraintIndexes) != 0 {
		return OptimizedVK{}, fmt.Errorf("custom gates are not supported")
	}
	words := make([]*big.Int, 0, OptimizedVKWords)
	words = append(words, new(big.Int).SetUint64(vk.NbPublicVariables), new(big.Int).SetUint64(vk.Size), frBig(vk.SizeInv), frBig(vk.Generator))
	for _, p := range []curve.G1Affine{vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk} {
		words = appendPoint(words, p)
	}
	for _, p := range vk.S {
		words = appendPoint(words, p)
	}
	words = append(words, frBig(vk.CosetShift), new(big.Int))
	if len(words) != OptimizedVKWords {
		return OptimizedVK{}, fmt.Errorf("optimized words=%d", len(words))
	}
	encoded := make([]byte, 0, len(words)*32)
	decimal := make([]string, len(words))
	for i, w := range words {
		if w.Sign() < 0 || w.BitLen() > 256 {
			return OptimizedVK{}, fmt.Errorf("word %d overflow", i)
		}
		b := w.Bytes()
		encoded = append(encoded, make([]byte, 32-len(b))...)
		encoded = append(encoded, b...)
		decimal[i] = w.String()
	}
	sum := sha256.Sum256(encoded)
	return OptimizedVK{Words: decimal, ByteLength: len(encoded), SHA256: hex.EncodeToString(sum[:])}, nil
}
func frBig(v fr.Element) *big.Int { return v.BigInt(new(big.Int)) }
func appendPoint(words []*big.Int, p curve.G1Affine) []*big.Int {
	return append(words, fpLow(p.X), fpHigh(p.X), fpLow(p.Y), fpHigh(p.Y))
}
func fpLow(v fp.Element) *big.Int {
	x := v.BigInt(new(big.Int))
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	return new(big.Int).And(x, mask)
}
func fpHigh(v fp.Element) *big.Int { x := v.BigInt(new(big.Int)); return x.Rsh(x, 256) }
