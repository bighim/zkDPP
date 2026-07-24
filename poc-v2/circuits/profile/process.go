package profile

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

type Process struct {
	M                 frontend.Variable     `gnark:",public"`
	N                 frontend.Variable     `gnark:",public"`
	ProcessDelta      []frontend.Variable   `gnark:",public"`
	Allocation        [][]frontend.Variable `gnark:",public"`
	PublicInputs      []InputPublic         `gnark:",public"`
	OutputCommitments []frontend.Variable   `gnark:",public"`
	SecretKey         frontend.Variable
	PublicKey         frontend.Variable
	Address           frontend.Variable
	Inputs            []Note
	Outputs           []Output
	Remainders        [][]frontend.Variable
}

func newProcess(l, a int) *Process {
	c := &Process{ProcessDelta: make([]frontend.Variable, l), Allocation: make([][]frontend.Variable, a), PublicInputs: make([]InputPublic, a), OutputCommitments: make([]frontend.Variable, a), Inputs: make([]Note, a), Outputs: make([]Output, a), Remainders: make([][]frontend.Variable, max(0, a-1))}
	for i := 0; i < a; i++ {
		c.Allocation[i] = make([]frontend.Variable, l)
		c.Inputs[i] = newNote(l)
		c.Outputs[i] = newOutput(l)
	}
	for i := range c.Remainders {
		c.Remainders[i] = make([]frontend.Variable, l)
	}
	return c
}

func (c *Process) Define(api frontend.API) error {
	if err := owner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	a := len(c.Inputs)
	l := len(c.ProcessDelta)
	api.AssertIsEqual(c.M, a)
	api.AssertIsEqual(c.N, a)
	inputAggregate := make([]frontend.Variable, l)
	outputAggregate := make([]frontend.Variable, l)
	for k := 0; k < l; k++ {
		inputAggregate[k] = 0
		outputAggregate[k] = 0
	}
	for i := 0; i < a; i++ {
		cm, err := commitment(api, asOutput(c.Inputs[i]), c.Address)
		if err != nil {
			return err
		}
		api.AssertIsEqual(c.PublicInputs[i].Commitment, cm)
		if err := common.AssertMembership(api, c.PublicInputs[i].Root, c.PublicInputs[i].Commitment, c.Inputs[i].Path); err != nil {
			return err
		}
		nf, err := common.Hash(api, c.SecretKey, c.PublicInputs[i].Commitment)
		if err != nil {
			return err
		}
		api.AssertIsEqual(c.PublicInputs[i].Nullifier, nf)
		for k := 0; k < l; k++ {
			inputAggregate[k] = api.Add(inputAggregate[k], c.Inputs[i].State[k])
		}
	}
	for j := 0; j < a; j++ {
		common.AssertPositiveUint64(api, c.Outputs[j].Quantity)
		for k := 0; k < l; k++ {
			common.AssertUint64(api, c.Outputs[j].State[k])
			outputAggregate[k] = api.Add(outputAggregate[k], c.Outputs[j].State[k])
		}
		cm, err := commitment(api, c.Outputs[j], c.Address)
		if err != nil {
			return err
		}
		api.AssertIsEqual(c.OutputCommitments[j], cm)
	}
	for k := 0; k < l; k++ {
		common.AssertUint66(api, c.ProcessDelta[k])
	}
	if l > 0 {
		api.AssertIsEqual(inputAggregate[0], api.Add(outputAggregate[0], c.ProcessDelta[0]))
	}
	if l > 1 {
		api.AssertIsEqual(outputAggregate[1], api.Add(inputAggregate[1], c.ProcessDelta[1]))
	}
	if l > 2 {
		api.AssertIsEqual(outputAggregate[2], api.Add(inputAggregate[2], c.ProcessDelta[2]))
		api.AssertIsEqual(outputAggregate[2], inputAggregate[2])
	}
	for k := 0; k < l; k++ {
		sum := frontend.Variable(0)
		for j := 0; j < a; j++ {
			api.AssertIsLessOrEqual(c.Allocation[j][k], Denominator)
			sum = api.Add(sum, c.Allocation[j][k])
		}
		api.AssertIsEqual(sum, Denominator)
		for j := 1; j < a; j++ {
			r := c.Remainders[j-1][k]
			api.ToBinary(r, 30)
			api.AssertIsLessOrEqual(r, Denominator-1)
			api.AssertIsEqual(api.Mul(outputAggregate[k], c.Allocation[j][k]), api.Add(api.Mul(Denominator, c.Outputs[j].State[k]), r))
		}
	}
	return nil
}
