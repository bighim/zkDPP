package issueclaim

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

const productBits = 94

type Circuit struct {
	PolicyRef     frontend.Variable `gnark:",public"`
	DPPCommitment frontend.Variable `gnark:",public"`
	DPP           circuitutil.DPPWitness
	Config        issuepolicy.Config `gnark:"-"`
}

func New(config issuepolicy.Config) *Circuit { return &Circuit{Config: config} }

func (c *Circuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.PolicyRef, c.Config.PolicyRef)
	circuitutil.AssertDPP(api, c.DPP)
	api.AssertIsDifferent(c.DPP.QMass, 0)
	api.AssertIsEqual(c.DPP.AssetRole, uint64(note.AssetRoleEligible))
	commitment, err := circuitutil.DPPCommitment(api, c.DPP)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.DPPCommitment, commitment)
	recycledHave := api.Mul(c.DPP.ARec, issuepolicy.Denominator)
	recycledNeed := api.Mul(c.DPP.QMass, c.Config.MinRecycledRate)
	carbonHave := api.Mul(c.DPP.E, issuepolicy.Denominator)
	carbonLimit := api.Mul(c.DPP.QMass, c.Config.MaxCarbonIntensity)
	api.ToBinary(recycledHave, productBits)
	api.ToBinary(recycledNeed, productBits)
	api.ToBinary(carbonHave, productBits)
	api.ToBinary(carbonLimit, productBits)
	api.AssertIsLessOrEqual(recycledNeed, recycledHave)
	api.AssertIsLessOrEqual(carbonHave, carbonLimit)
	return nil
}

func Assignment(config issuepolicy.Config, value dpp.PrivateData) *Circuit {
	return &Circuit{
		PolicyRef: config.PolicyRef, DPPCommitment: value.Commitment, Config: config,
		DPP: circuitutil.DPPWitness{DocumentHash: value.DocumentHash, AssetRole: uint64(value.AssetRole), QMass: value.State.QMass, ARec: value.State.ARec, E: value.State.E, Opening: value.Opening},
	}
}

func Public(config issuepolicy.Config, value dpp.PrivateData) [2]fr.Element {
	return [2]fr.Element{config.PolicyRef, value.Commitment}
}
