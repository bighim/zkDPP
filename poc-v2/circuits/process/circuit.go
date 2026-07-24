package process

import (
	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

const (
	Arity       = 3
	Denominator = 1_000_000_000
)

type Circuit struct {
	M                 frontend.Variable                            `gnark:",public"`
	N                 frontend.Variable                            `gnark:",public"`
	ProcessDelta      [common.StateLength]frontend.Variable        `gnark:",public"`
	Allocation        [Arity][common.StateLength]frontend.Variable `gnark:",public"`
	PublicInputs      [Arity]InputPublic                           `gnark:",public"`
	OutputCommitments [Arity]frontend.Variable                     `gnark:",public"`
	SecretKey         frontend.Variable
	PublicKey         frontend.Variable
	Address           frontend.Variable
	Inputs            [Arity]common.NoteWitness
	Outputs           [Arity]common.OutputWitness
	Remainders        [Arity - 1][common.StateLength]frontend.Variable
}

type InputPublic struct {
	Root       frontend.Variable
	Commitment frontend.Variable
	Nullifier  frontend.Variable
}

func (c *Circuit) Define(api frontend.API) error {
	if err := common.DeriveOwner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	activeIn := activeFlags(api, c.M)
	activeOut := activeFlags(api, c.N)
	var inputAggregate, outputAggregate [common.StateLength]frontend.Variable
	for k := 0; k < common.StateLength; k++ {
		inputAggregate[k] = 0
		outputAggregate[k] = 0
	}
	for i := 0; i < Arity; i++ {
		active := activeIn[i]
		inactive := api.Sub(1, active)
		cm, err := common.Commitment(api, common.AsOutput(c.Inputs[i]), c.Address)
		if err != nil {
			return err
		}
		assertWhen(api, active, api.Sub(c.PublicInputs[i].Commitment, cm))
		membershipRoot, err := membershipRoot(api, c.PublicInputs[i].Commitment, c.Inputs[i].Path)
		if err != nil {
			return err
		}
		assertWhen(api, active, api.Sub(c.PublicInputs[i].Root, membershipRoot))
		nf, err := common.Hash(api, c.SecretKey, c.PublicInputs[i].Commitment)
		if err != nil {
			return err
		}
		assertWhen(api, active, api.Sub(c.PublicInputs[i].Nullifier, nf))
		for _, value := range inputSlotValues(c, i) {
			assertWhen(api, inactive, value)
		}
		for k := 0; k < common.StateLength; k++ {
			inputAggregate[k] = api.Add(inputAggregate[k], api.Mul(active, c.Inputs[i].State[k]))
		}
	}
	for j := 0; j < Arity; j++ {
		active := activeOut[j]
		inactive := api.Sub(1, active)
		common.AssertUint64(api, c.Outputs[j].Quantity)
		assertWhen(api, active, api.IsZero(c.Outputs[j].Quantity))
		for k := 0; k < common.StateLength; k++ {
			common.AssertUint64(api, c.Outputs[j].State[k])
			outputAggregate[k] = api.Add(outputAggregate[k], api.Mul(active, c.Outputs[j].State[k]))
		}
		cm, err := common.Commitment(api, c.Outputs[j], c.Address)
		if err != nil {
			return err
		}
		assertWhen(api, active, api.Sub(c.OutputCommitments[j], cm))
		for _, value := range outputSlotValues(c, j) {
			assertWhen(api, inactive, value)
		}
	}
	for k := 0; k < common.StateLength; k++ {
		common.AssertUint66(api, c.ProcessDelta[k])
	}
	api.AssertIsEqual(inputAggregate[0], api.Add(outputAggregate[0], c.ProcessDelta[0]))
	api.AssertIsEqual(outputAggregate[1], api.Add(inputAggregate[1], c.ProcessDelta[1]))
	api.AssertIsEqual(outputAggregate[2], api.Add(inputAggregate[2], c.ProcessDelta[2]))
	api.AssertIsEqual(outputAggregate[2], inputAggregate[2])
	for k := 0; k < common.StateLength; k++ {
		sum := frontend.Variable(0)
		for j := 0; j < Arity; j++ {
			api.AssertIsLessOrEqual(c.Allocation[j][k], Denominator)
			sum = api.Add(sum, c.Allocation[j][k])
			assertWhen(api, api.Sub(1, activeOut[j]), c.Allocation[j][k])
		}
		api.AssertIsEqual(sum, Denominator)
		for j := 1; j < Arity; j++ {
			r := c.Remainders[j-1][k]
			api.ToBinary(r, 30)
			api.AssertIsLessOrEqual(r, Denominator-1)
			relation := api.Sub(api.Mul(outputAggregate[k], c.Allocation[j][k]), api.Add(api.Mul(Denominator, c.Outputs[j].State[k]), r))
			assertWhen(api, activeOut[j], relation)
			assertWhen(api, api.Sub(1, activeOut[j]), r)
		}
	}
	return nil
}

func activeFlags(api frontend.API, count frontend.Variable) [Arity]frontend.Variable {
	one := api.IsZero(api.Sub(count, 1))
	two := api.IsZero(api.Sub(count, 2))
	three := api.IsZero(api.Sub(count, 3))
	api.AssertIsEqual(api.Add(one, two, three), 1)
	return [Arity]frontend.Variable{1, api.Add(two, three), three}
}

func assertWhen(api frontend.API, flag, value frontend.Variable) {
	api.AssertIsEqual(api.Mul(flag, value), 0)
}

func membershipRoot(api frontend.API, leaf frontend.Variable, path common.MerklePath) (frontend.Variable, error) {
	bits := api.ToBinary(path.Index, common.TreeDepth)
	current := leaf
	for level := 0; level < common.TreeDepth; level++ {
		left := api.Select(bits[level], path.Siblings[level], current)
		right := api.Select(bits[level], current, path.Siblings[level])
		parent, err := common.Compress(api, left, right)
		if err != nil {
			return nil, err
		}
		current = parent
	}
	return current, nil
}

func inputSlotValues(c *Circuit, i int) []frontend.Variable {
	values := []frontend.Variable{c.PublicInputs[i].Root, c.PublicInputs[i].Commitment, c.PublicInputs[i].Nullifier, c.Inputs[i].DocumentHash, c.Inputs[i].Quantity, c.Inputs[i].Opening, c.Inputs[i].Path.Index}
	values = append(values, c.Inputs[i].State[:]...)
	values = append(values, c.Inputs[i].Path.Siblings[:]...)
	return values
}
func outputSlotValues(c *Circuit, j int) []frontend.Variable {
	values := []frontend.Variable{c.OutputCommitments[j], c.Outputs[j].DocumentHash, c.Outputs[j].Quantity, c.Outputs[j].Opening}
	values = append(values, c.Outputs[j].State[:]...)
	return values
}
