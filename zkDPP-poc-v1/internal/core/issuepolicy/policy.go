package issuepolicy

import (
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/policy"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	AuthorityID = uint64(1)
	PolicyID    = uint64(2)
	Denominator = uint64(1_000_000_000)
)

type Config struct {
	Name               string
	Version            uint64
	MinRecycledRate    uint64
	MaxCarbonIntensity uint64
	PolicyRef          fr.Element
}

func NewConfig(name string, version, minRecycledRate, maxCarbonIntensity uint64) Config {
	return Config{
		Name: name, Version: version,
		MinRecycledRate: minRecycledRate, MaxCarbonIntensity: maxCarbonIntensity,
		PolicyRef: policy.Ref(policy.EventIssue, AuthorityID, PolicyID, version),
	}
}

func Standard() Config { return NewConfig("issue-standard-v1", 1, 100_000_000, 1_000_000_000) }
func Strict() Config   { return NewConfig("issue-strict-v2", 2, 110_000_000, 970_000_000) }

func Validate(cfg Config, value dpp.PrivateData) error {
	if value.AssetRole != note.AssetRoleEligible || value.State.QMass == 0 {
		return fmt.Errorf("Issue requires nonzero ELIGIBLE DPP")
	}
	if err := note.ValidateState(value.State, value.AssetRole); err != nil {
		return err
	}
	q := new(big.Int).SetUint64(value.State.QMass)
	a := new(big.Int).SetUint64(value.State.ARec)
	e := new(big.Int).SetUint64(value.State.E)
	d := new(big.Int).SetUint64(Denominator)
	if new(big.Int).Mul(a, d).Cmp(new(big.Int).Mul(q, new(big.Int).SetUint64(cfg.MinRecycledRate))) < 0 {
		return fmt.Errorf("recycled rate below Policy")
	}
	if new(big.Int).Mul(e, d).Cmp(new(big.Int).Mul(q, new(big.Int).SetUint64(cfg.MaxCarbonIntensity))) > 0 {
		return fmt.Errorf("carbon intensity above Policy")
	}
	return nil
}
