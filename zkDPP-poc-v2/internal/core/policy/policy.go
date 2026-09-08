package policy

import (
	"fmt"
	"math/big"

	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	EventProcess    = uint64(6)
	EventIssue      = uint64(8)
	AuthorityID     = uint64(1)
	PolicyID        = uint64(1)
	Version         = uint64(1)
	InputArity      = 3
	OutputArity     = 2
	Denominator     = uint64(1_000_000_000)
	LossRate        = uint64(62_500_000)
	CarbonIntensity = uint64(93_750_000)
	WasteMassRate   = uint64(100_000_000)
)

type Remainders struct{ Loss, Carbon, WasteMass uint64 }
type Result struct {
	Total, Intermediate note.State
	Eligible, Waste     note.State
	QLoss, CarbonAdd    uint64
	Remainders          Remainders
}

func Ref(eventKind, authorityID, policyID, version uint64) fr.Element {
	return zkhash.Hash(zkhash.PolicyRefTag, zkhash.Element(eventKind), zkhash.Element(authorityID), zkhash.Element(policyID), zkhash.Element(version))
}
func CanonicalRef() fr.Element { return Ref(EventProcess, AuthorityID, PolicyID, Version) }
func ScopeRef(skOwner, policyRef fr.Element) fr.Element {
	return zkhash.Hash(zkhash.ScopeRefTag, skOwner, policyRef)
}

func Apply(inputs [InputArity]note.State) (Result, error) {
	var result Result
	for _, in := range inputs {
		var err error
		result.Total, err = add(result.Total, in)
		if err != nil {
			return Result{}, err
		}
	}
	result.QLoss, result.Remainders.Loss = mulDivRem(result.Total.QMass, LossRate, Denominator)
	result.CarbonAdd, result.Remainders.Carbon = mulDivRem(result.Total.QMass, CarbonIntensity, Denominator)
	if result.QLoss > result.Total.QMass {
		return Result{}, fmt.Errorf("loss exceeds mass")
	}
	carbon, carry := add64(result.Total.E, result.CarbonAdd)
	if carry {
		return Result{}, fmt.Errorf("carbon overflow")
	}
	result.Intermediate = note.State{QMass: result.Total.QMass - result.QLoss, ARec: result.Total.ARec, E: carbon}
	wasteMass, rem := mulDivRem(result.Intermediate.QMass, WasteMassRate, Denominator)
	result.Remainders.WasteMass = rem
	result.Waste = note.State{QMass: wasteMass}
	result.Eligible = note.State{QMass: result.Intermediate.QMass - wasteMass, ARec: result.Intermediate.ARec, E: result.Intermediate.E}
	if err := note.ValidateState(result.Eligible, note.AssetRoleEligible); err != nil {
		return Result{}, err
	}
	if err := note.ValidateState(result.Waste, note.AssetRoleWaste); err != nil {
		return Result{}, err
	}
	return result, nil
}

func add(a, b note.State) (note.State, error) {
	q, cq := add64(a.QMass, b.QMass)
	r, cr := add64(a.ARec, b.ARec)
	e, ce := add64(a.E, b.E)
	if cq || cr || ce {
		return note.State{}, fmt.Errorf("state overflow")
	}
	return note.State{QMass: q, ARec: r, E: e}, nil
}
func add64(a, b uint64) (uint64, bool) { v := a + b; return v, v < a }
func mulDivRem(value, numerator, denominator uint64) (uint64, uint64) {
	p := new(big.Int).Mul(new(big.Int).SetUint64(value), new(big.Int).SetUint64(numerator))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(p, new(big.Int).SetUint64(denominator), r)
	return q.Uint64(), r.Uint64()
}
