package profile

import (
	"fmt"

	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/consensys/gnark/frontend"
)

const Denominator = 1_000_000_000

type Note struct {
	DocumentHash frontend.Variable
	Quantity     frontend.Variable
	State        []frontend.Variable
	Opening      frontend.Variable
	Path         common.MerklePath
}

type Output struct {
	DocumentHash frontend.Variable
	Quantity     frontend.Variable
	State        []frontend.Variable
	Opening      frontend.Variable
}

type Voucher struct {
	DocumentHash    frontend.Variable
	Quantity        frontend.Variable
	State           []frontend.Variable
	SenderAddress   frontend.Variable
	ReceiverAddress frontend.Variable
	DeltaEpoch      frontend.Variable
	Opening         frontend.Variable
	Path            common.MerklePath
}

type InputPublic struct {
	Root       frontend.Variable
	Commitment frontend.Variable
	Nullifier  frontend.Variable
}

func newNote(l int) Note       { return Note{State: make([]frontend.Variable, l)} }
func newOutput(l int) Output   { return Output{State: make([]frontend.Variable, l)} }
func newVoucher(l int) Voucher { return Voucher{State: make([]frontend.Variable, l)} }
func asOutput(n Note) Output {
	return Output{DocumentHash: n.DocumentHash, Quantity: n.Quantity, State: n.State, Opening: n.Opening}
}
func commitment(api frontend.API, n Output, address frontend.Variable) (frontend.Variable, error) {
	inputs := []frontend.Variable{n.DocumentHash, n.Quantity}
	inputs = append(inputs, n.State...)
	inputs = append(inputs, address, n.Opening)
	return common.Hash(api, inputs...)
}
func voucherCommitment(api frontend.API, v Voucher) (frontend.Variable, error) {
	inputs := []frontend.Variable{v.DocumentHash, v.Quantity}
	inputs = append(inputs, v.State...)
	inputs = append(inputs, v.SenderAddress, v.ReceiverAddress, v.DeltaEpoch, v.Opening)
	return common.Hash(api, inputs...)
}
func owner(api frontend.API, sk, pk, addr frontend.Variable) error {
	return common.DeriveOwner(api, sk, pk, addr)
}

func NewCircuit(event string, l, arity int) (frontend.Circuit, error) {
	switch event {
	case "entry":
		return &Entry{Note: newOutput(l)}, nil
	case "transfer":
		return &Transfer{Input: newNote(l), Transfer: newOutput(l), Change: newOutput(l), Remainder: make([]frontend.Variable, l)}, nil
	case "proceed":
		return &Proceed{Voucher: newVoucher(l), Output: newOutput(l)}, nil
	case "recall":
		return &Recall{Voucher: newVoucher(l), Output: newOutput(l)}, nil
	case "merge":
		return &Merge{Inputs: []Note{newNote(l), newNote(l)}, Output: newOutput(l)}, nil
	case "split":
		return &Split{Input: newNote(l), Outputs: []Output{newOutput(l), newOutput(l)}, Remainder: make([]frontend.Variable, l)}, nil
	case "process":
		return newProcess(l, arity), nil
	case "exit":
		return &Exit{Note: newNote(l)}, nil
	default:
		return nil, fmt.Errorf("unknown event %q", event)
	}
}

type Entry struct {
	Commitment frontend.Variable `gnark:",public"`
	Note       Output
	Address    frontend.Variable
}

func (c *Entry) Define(api frontend.API) error {
	common.AssertPositiveUint64(api, c.Note.Quantity)
	for _, v := range c.Note.State {
		common.AssertUint64(api, v)
	}
	cm, err := commitment(api, c.Note, c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Commitment, cm)
	return nil
}

type Exit struct {
	Root       frontend.Variable `gnark:",public"`
	Commitment frontend.Variable `gnark:",public"`
	Nullifier  frontend.Variable `gnark:",public"`
	SecretKey  frontend.Variable
	PublicKey  frontend.Variable
	Address    frontend.Variable
	Note       Note
}

func (c *Exit) Define(api frontend.API) error {
	if err := owner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	cm, err := commitment(api, asOutput(c.Note), c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Commitment, cm)
	if err := common.AssertMembership(api, c.Root, c.Commitment, c.Note.Path); err != nil {
		return err
	}
	nf, err := common.Hash(api, c.SecretKey, c.Commitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Nullifier, nf)
	return nil
}

type Merge struct {
	Root1            frontend.Variable `gnark:",public"`
	Root2            frontend.Variable `gnark:",public"`
	Commitment1      frontend.Variable `gnark:",public"`
	Commitment2      frontend.Variable `gnark:",public"`
	Nullifier1       frontend.Variable `gnark:",public"`
	Nullifier2       frontend.Variable `gnark:",public"`
	OutputCommitment frontend.Variable `gnark:",public"`
	SecretKey        frontend.Variable
	PublicKey        frontend.Variable
	Address          frontend.Variable
	Inputs           []Note
	Output           Output
}

func (c *Merge) Define(api frontend.API) error {
	if err := owner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	roots := []frontend.Variable{c.Root1, c.Root2}
	cms := []frontend.Variable{c.Commitment1, c.Commitment2}
	nfs := []frontend.Variable{c.Nullifier1, c.Nullifier2}
	for i := 0; i < 2; i++ {
		cm, err := commitment(api, asOutput(c.Inputs[i]), c.Address)
		if err != nil {
			return err
		}
		api.AssertIsEqual(cms[i], cm)
		if err := common.AssertMembership(api, roots[i], cms[i], c.Inputs[i].Path); err != nil {
			return err
		}
		nf, err := common.Hash(api, c.SecretKey, cms[i])
		if err != nil {
			return err
		}
		api.AssertIsEqual(nfs[i], nf)
	}
	api.AssertIsEqual(c.Inputs[0].DocumentHash, c.Inputs[1].DocumentHash)
	api.AssertIsEqual(c.Output.DocumentHash, c.Inputs[0].DocumentHash)
	api.AssertIsEqual(c.Output.Quantity, api.Add(c.Inputs[0].Quantity, c.Inputs[1].Quantity))
	common.AssertUint64(api, c.Output.Quantity)
	for k := range c.Output.State {
		api.AssertIsEqual(c.Output.State[k], api.Add(c.Inputs[0].State[k], c.Inputs[1].State[k]))
		common.AssertUint64(api, c.Output.State[k])
	}
	cm, err := commitment(api, c.Output, c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment, cm)
	return nil
}

type Split struct {
	Root              frontend.Variable `gnark:",public"`
	InputCommitment   frontend.Variable `gnark:",public"`
	Nullifier         frontend.Variable `gnark:",public"`
	OutputCommitment1 frontend.Variable `gnark:",public"`
	OutputCommitment2 frontend.Variable `gnark:",public"`
	SecretKey         frontend.Variable
	PublicKey         frontend.Variable
	Address           frontend.Variable
	Input             Note
	Outputs           []Output
	Remainder         []frontend.Variable
}

func (c *Split) Define(api frontend.API) error {
	if err := owner(api, c.SecretKey, c.PublicKey, c.Address); err != nil {
		return err
	}
	cm, err := commitment(api, asOutput(c.Input), c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.InputCommitment, cm)
	if err := common.AssertMembership(api, c.Root, c.InputCommitment, c.Input.Path); err != nil {
		return err
	}
	nf, err := common.Hash(api, c.SecretKey, c.InputCommitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Nullifier, nf)
	api.AssertIsEqual(c.Input.Quantity, api.Add(c.Outputs[0].Quantity, c.Outputs[1].Quantity))
	for j := 0; j < 2; j++ {
		common.AssertUint64(api, c.Outputs[j].Quantity)
		api.AssertIsEqual(c.Outputs[j].DocumentHash, c.Input.DocumentHash)
	}
	for k := range c.Input.State {
		for j := 0; j < 2; j++ {
			common.AssertUint64(api, c.Outputs[j].State[k])
		}
		api.AssertIsEqual(api.Mul(c.Outputs[1].Quantity, c.Input.State[k]), api.Add(api.Mul(c.Input.Quantity, c.Outputs[1].State[k]), c.Remainder[k]))
		api.AssertIsLessOrEqual(api.Add(c.Remainder[k], 1), c.Input.Quantity)
		api.AssertIsEqual(c.Input.State[k], api.Add(c.Outputs[0].State[k], c.Outputs[1].State[k]))
	}
	cm1, err := commitment(api, c.Outputs[0], c.Address)
	if err != nil {
		return err
	}
	cm2, err := commitment(api, c.Outputs[1], c.Address)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.OutputCommitment1, cm1)
	api.AssertIsEqual(c.OutputCommitment2, cm2)
	return nil
}
