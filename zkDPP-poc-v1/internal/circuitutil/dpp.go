package circuitutil

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	"github.com/consensys/gnark/frontend"
)

type DPPWitness struct {
	DocumentHash frontend.Variable
	AssetRole    frontend.Variable
	QMass        frontend.Variable
	ARec         frontend.Variable
	E            frontend.Variable
	Opening      frontend.Variable
}

func AssertDPP(api frontend.API, value DPPWitness) {
	AssertStateAndRole(api, value.QMass, value.ARec, value.E, value.AssetRole)
}

func DPPCommitment(api frontend.API, value DPPWitness) (frontend.Variable, error) {
	return Hash(api, dpp.Tag, value.DocumentHash, value.AssetRole, value.QMass, value.ARec, value.E, value.Opening)
}
