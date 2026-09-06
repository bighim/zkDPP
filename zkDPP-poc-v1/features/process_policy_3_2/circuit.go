package processpolicy

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/policy"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	PolicyRef frontend.Variable    `gnark:",public"`
	ScopeRef  frontend.Variable    `gnark:",public"`
	NoteRoot  frontend.Variable    `gnark:",public"`
	NF        [3]frontend.Variable `gnark:",public"`
	CMOut     [2]frontend.Variable `gnark:",public"`
	SKOwner   frontend.Variable
	Inputs    [3]circuitutil.NoteWitness
	Paths     [3]circuitutil.MerklePath
	Outputs   [2]circuitutil.NoteWitness
	QLoss     frontend.Variable
	CarbonAdd frontend.Variable
	RemLoss   frontend.Variable
	RemCarbon frontend.Variable
	RemWaste  frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.PolicyRef, policy.CanonicalRef())
	owner, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	scope, err := circuitutil.Hash(api, zkhash.ScopeRefTag, c.SKOwner, c.PolicyRef)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.ScopeRef, scope)
	commitments := [3]frontend.Variable{}
	totQ, totA, totE := frontend.Variable(0), frontend.Variable(0), frontend.Variable(0)
	for i := 0; i < 3; i++ {
		circuitutil.AssertNote(api, c.Inputs[i])
		api.AssertIsEqual(c.Inputs[i].AssetRole, uint64(note.AssetRoleEligible))
		api.AssertIsEqual(c.Inputs[i].Address, owner)
		commitments[i], err = circuitutil.Commitment(api, c.Inputs[i])
		if err != nil {
			return err
		}
		if err = circuitutil.AssertMembership(api, c.NoteRoot, commitments[i], c.Paths[i]); err != nil {
			return err
		}
		nf, e := circuitutil.Nullifier(api, c.SKOwner, commitments[i])
		if e != nil {
			return e
		}
		api.AssertIsEqual(c.NF[i], nf)
		totQ = checkedAdd(api, totQ, c.Inputs[i].QMass)
		totA = checkedAdd(api, totA, c.Inputs[i].ARec)
		totE = checkedAdd(api, totE, c.Inputs[i].E)
	}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			api.AssertIsDifferent(commitments[i], commitments[j])
			api.AssertIsDifferent(c.NF[i], c.NF[j])
		}
	}
	assertFloor(api, totQ, policy.LossRate, c.QLoss, c.RemLoss)
	assertFloor(api, totQ, policy.CarbonIntensity, c.CarbonAdd, c.RemCarbon)
	api.AssertIsLessOrEqual(c.QLoss, totQ)
	intermediateQ := api.Sub(totQ, c.QLoss)
	api.ToBinary(intermediateQ, 64)
	intermediateE := checkedAdd(api, totE, c.CarbonAdd)
	for i := 0; i < 2; i++ {
		circuitutil.AssertNote(api, c.Outputs[i])
		api.AssertIsEqual(c.Outputs[i].Address, owner)
	}
	api.AssertIsEqual(c.Outputs[0].AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Outputs[1].AssetRole, uint64(note.AssetRoleWaste))
	assertFloor(api, intermediateQ, policy.WasteMassRate, c.Outputs[1].QMass, c.RemWaste)
	api.AssertIsEqual(c.Outputs[1].ARec, 0)
	api.AssertIsEqual(c.Outputs[1].E, 0)
	api.AssertIsEqual(c.Outputs[0].QMass, api.Sub(intermediateQ, c.Outputs[1].QMass))
	api.AssertIsEqual(c.Outputs[0].ARec, totA)
	api.AssertIsEqual(c.Outputs[0].E, intermediateE)
	for i := 0; i < 2; i++ {
		cm, e := circuitutil.Commitment(api, c.Outputs[i])
		if e != nil {
			return e
		}
		api.AssertIsEqual(c.CMOut[i], cm)
	}
	api.AssertIsDifferent(c.CMOut[0], c.CMOut[1])
	return nil
}
func checkedAdd(api frontend.API, a, b frontend.Variable) frontend.Variable {
	v := api.Add(a, b)
	api.ToBinary(v, 64)
	return v
}
func assertFloor(api frontend.API, value, rate, quotient, remainder frontend.Variable) {
	api.ToBinary(quotient, 64)
	api.ToBinary(remainder, 64)
	api.AssertIsEqual(api.Mul(value, rate), api.Add(api.Mul(policy.Denominator, quotient), remainder))
	api.AssertIsLessOrEqual(api.Add(remainder, 1), policy.Denominator)
}

func Assignment(inputs [3]note.Note, sk, root fr.Element, paths [3]merkle.Path, outputs [2]note.Note, result policy.Result) *Circuit {
	c := &Circuit{PolicyRef: policy.CanonicalRef(), ScopeRef: policy.ScopeRef(sk, policy.CanonicalRef()), NoteRoot: root, SKOwner: sk, QLoss: result.QLoss, CarbonAdd: result.CarbonAdd, RemLoss: result.Remainders.Loss, RemCarbon: result.Remainders.Carbon, RemWaste: result.Remainders.WasteMass}
	for i := 0; i < 3; i++ {
		c.NF[i] = note.Nullifier(sk, inputs[i].Commitment)
		c.Inputs[i] = witness(inputs[i])
		c.Paths[i] = path(paths[i])
	}
	for i := 0; i < 2; i++ {
		c.CMOut[i] = outputs[i].Commitment
		c.Outputs[i] = witness(outputs[i])
	}
	return c
}
func witness(v note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, Address: v.Address, Opening: v.Opening}
}
func path(v merkle.Path) circuitutil.MerklePath {
	p := circuitutil.MerklePath{Index: v.Index}
	for i := range v.Siblings {
		p.Siblings[i] = v.Siblings[i]
	}
	return p
}
