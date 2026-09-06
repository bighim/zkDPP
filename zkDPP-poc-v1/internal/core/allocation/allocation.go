package allocation

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
)

type Remainder struct{ ARec, E uint64 }

func ByMass(input note.State, qFirst uint64) (first, second note.State, remainder Remainder, err error) {
	if input.QMass == 0 || qFirst > input.QMass {
		return first, second, remainder, fmt.Errorf("invalid allocation mass")
	}
	first.QMass = qFirst
	second.QMass = input.QMass - qFirst
	second.ARec, remainder.ARec = mulDivRem(input.ARec, second.QMass, input.QMass)
	second.E, remainder.E = mulDivRem(input.E, second.QMass, input.QMass)
	first.ARec = input.ARec - second.ARec
	first.E = input.E - second.E
	return first, second, remainder, nil
}

func Add(a, b note.State) (note.State, error) {
	q, cq := bits.Add64(a.QMass, b.QMass, 0)
	r, cr := bits.Add64(a.ARec, b.ARec, 0)
	e, ce := bits.Add64(a.E, b.E, 0)
	if cq != 0 || cr != 0 || ce != 0 {
		return note.State{}, fmt.Errorf("state uint64 overflow")
	}
	return note.State{QMass: q, ARec: r, E: e}, nil
}

func mulDivRem(value, numerator, denominator uint64) (uint64, uint64) {
	product := new(big.Int).Mul(new(big.Int).SetUint64(value), new(big.Int).SetUint64(numerator))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(product, new(big.Int).SetUint64(denominator), r)
	return q.Uint64(), r.Uint64()
}
