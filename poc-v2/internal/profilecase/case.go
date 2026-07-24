package profilecase

import (
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	"github.com/bighim/zkDPP/poc-v2/circuits/profile"
	"github.com/bighim/zkDPP/poc-v2/internal/merkle"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Case struct {
	Name        string
	Event       string
	StateLength int
	Arity       int
	Circuit     frontend.Circuit
	Assignment  frontend.Circuit
	Public      []fr.Element
}

type note struct {
	document fr.Element
	quantity uint64
	state    []uint64
	opening  uint64
	owner    protocol.Owner
	cm       fr.Element
	path     merkle.Path
}

func Build(event string, stateLength, arity int) (Case, error) {
	if stateLength < 0 || stateLength > 3 {
		return Case{}, fmt.Errorf("state length must be between 0 and 3")
	}
	if event != "process" {
		arity = 0
	} else if arity < 1 || arity > 3 {
		return Case{}, fmt.Errorf("process arity must be between 1 and 3")
	}
	circuit, err := profile.NewCircuit(event, stateLength, arity)
	if err != nil {
		return Case{}, err
	}
	assignment, err := profile.NewCircuit(event, stateLength, arity)
	if err != nil {
		return Case{}, err
	}
	result := Case{Name: profileName(event, stateLength, arity), Event: event, StateLength: stateLength, Arity: arity, Circuit: circuit, Assignment: assignment}
	if err := fill(&result); err != nil {
		return Case{}, err
	}
	return result, nil
}

func profileName(event string, l, arity int) string {
	if event == "process" {
		return fmt.Sprintf("process-s%d-%dto%d", l, arity, arity)
	}
	return fmt.Sprintf("%s-s%d", event, l)
}

func fill(c *Case) error {
	owner := protocol.NewOwner(11)
	receiver := protocol.NewOwner(22)
	doc := protocol.Element(777)
	base := []uint64{250_000_000_003, 400_000_000_007, 80_000_000_009}[:c.StateLength]
	switch a := c.Assignment.(type) {
	case *profile.Entry:
		n := makeNote(doc, 10, base, owner, 1001)
		a.Commitment, a.Address = n.cm, owner.Address
		fillOutput(&a.Note, n)
		c.Public = []fr.Element{n.cm}
	case *profile.Exit:
		n, root := noteInTree(doc, 10, base, owner, 1001)
		nf := protocol.Nullifier(owner, n.cm)
		a.Root, a.Commitment, a.Nullifier = root, n.cm, nf
		a.SecretKey, a.PublicKey, a.Address = owner.SecretKey, owner.PublicKey, owner.Address
		fillNote(&a.Note, n)
		c.Public = []fr.Element{root, n.cm, nf}
	case *profile.Merge:
		n1 := makeNote(doc, 10, base, owner, 1001)
		second := scaleState(base, 1, 2)
		n2 := makeNote(doc, 5, second, owner, 1002)
		tree := merkle.New(protocol.TreeDepth)
		_, _, _ = tree.Append(n1.cm)
		_, root, _ := tree.Append(n2.cm)
		n1.path, _ = tree.Path(0)
		n2.path, _ = tree.Path(1)
		out := makeNote(doc, 15, addState(n1.state, n2.state), owner, 1003)
		nf1, nf2 := protocol.Nullifier(owner, n1.cm), protocol.Nullifier(owner, n2.cm)
		a.Root1, a.Root2, a.Commitment1, a.Commitment2 = root, root, n1.cm, n2.cm
		a.Nullifier1, a.Nullifier2, a.OutputCommitment = nf1, nf2, out.cm
		a.SecretKey, a.PublicKey, a.Address = owner.SecretKey, owner.PublicKey, owner.Address
		fillNote(&a.Inputs[0], n1)
		fillNote(&a.Inputs[1], n2)
		fillOutput(&a.Output, out)
		c.Public = []fr.Element{root, root, n1.cm, n2.cm, nf1, nf2, out.cm}
	case *profile.Split:
		in, root := noteInTree(doc, 15, base, owner, 1001)
		s2, rem := floorState(base, 7, 15)
		s1 := subState(base, s2)
		o1, o2 := makeNote(doc, 8, s1, owner, 1002), makeNote(doc, 7, s2, owner, 1003)
		nf := protocol.Nullifier(owner, in.cm)
		a.Root, a.InputCommitment, a.Nullifier = root, in.cm, nf
		a.OutputCommitment1, a.OutputCommitment2 = o1.cm, o2.cm
		a.SecretKey, a.PublicKey, a.Address = owner.SecretKey, owner.PublicKey, owner.Address
		fillNote(&a.Input, in)
		fillOutput(&a.Outputs[0], o1)
		fillOutput(&a.Outputs[1], o2)
		fillVars(a.Remainder, rem)
		c.Public = []fr.Element{root, in.cm, nf, o1.cm, o2.cm}
	case *profile.Transfer:
		in, root := noteInTree(doc, 10, base, owner, 1001)
		changeState, rem := floorState(base, 4, 10)
		transferState := subState(base, changeState)
		transfer := makeNote(doc, 6, transferState, receiver, 0)
		change := makeNote(doc, 4, changeState, owner, 1002)
		voucherOpening := uint64(1003)
		rv := voucherHash(doc, 6, transferState, owner.Address, receiver.Address, 6, voucherOpening)
		nf := protocol.Nullifier(owner, in.cm)
		a.Root, a.InputCommitment, a.Nullifier = root, in.cm, nf
		a.VoucherCommitment, a.ChangeCommitment, a.DeltaEpoch = rv, change.cm, 6
		a.SenderSecretKey, a.SenderPublicKey, a.ReceiverPublicKey = owner.SecretKey, owner.PublicKey, receiver.PublicKey
		a.SenderAddress, a.ReceiverAddress = owner.Address, receiver.Address
		fillNote(&a.Input, in)
		fillOutput(&a.Transfer, transfer)
		fillOutput(&a.Change, change)
		a.VoucherOpening = voucherOpening
		fillVars(a.Remainder, rem)
		c.Public = []fr.Element{root, in.cm, nf, rv, change.cm, protocol.Element(6)}
	case *profile.Proceed:
		return fillResolution(c, a, nil, doc, base, owner, receiver, true)
	case *profile.Recall:
		return fillResolution(c, nil, a, doc, base, owner, receiver, false)
	case *profile.Process:
		return fillProcess(c, a, owner, doc)
	default:
		return fmt.Errorf("unsupported assignment %T", c.Assignment)
	}
	return nil
}

func fillResolution(c *Case, proceed *profile.Proceed, recall *profile.Recall, doc fr.Element, state []uint64, sender, receiver protocol.Owner, isProceed bool) error {
	const quantity, delta, voucherOpening, outputOpening = uint64(6), uint64(6), uint64(1003), uint64(1004)
	rv := voucherHash(doc, quantity, state, sender.Address, receiver.Address, delta, voucherOpening)
	tree := merkle.New(protocol.TreeDepth)
	_, root, _ := tree.Append(rv)
	path, _ := tree.Path(0)
	rvnf := zkhash.Hash(rv, protocol.Element(voucherOpening))
	outputOwner := sender
	if isProceed {
		outputOwner = receiver
	}
	out := makeNote(doc, quantity, state, outputOwner, outputOpening)
	if isProceed {
		proceed.VoucherRoot, proceed.VoucherCommitment, proceed.VoucherNullifier, proceed.OutputCommitment = root, rv, rvnf, out.cm
		proceed.ReceiverSecretKey, proceed.ReceiverPublicKey = receiver.SecretKey, receiver.PublicKey
		fillVoucher(&proceed.Voucher, doc, quantity, state, sender.Address, receiver.Address, delta, voucherOpening, path)
		fillOutput(&proceed.Output, out)
	} else {
		recall.VoucherRoot, recall.VoucherCommitment, recall.VoucherNullifier, recall.OutputCommitment = root, rv, rvnf, out.cm
		recall.SenderSecretKey, recall.SenderPublicKey = sender.SecretKey, sender.PublicKey
		fillVoucher(&recall.Voucher, doc, quantity, state, sender.Address, receiver.Address, delta, voucherOpening, path)
		fillOutput(&recall.Output, out)
	}
	c.Public = []fr.Element{root, rv, rvnf, out.cm}
	return nil
}

func fillProcess(c *Case, a *profile.Process, owner protocol.Owner, doc fr.Element) error {
	l, arity := c.StateLength, c.Arity
	a.M, a.N = arity, arity
	a.SecretKey, a.PublicKey, a.Address = owner.SecretKey, owner.PublicKey, owner.Address
	tree := merkle.New(protocol.TreeDepth)
	inputs := make([]note, arity)
	aggregate := make([]uint64, l)
	for i := 0; i < arity; i++ {
		state := []uint64{100_000_000_000 + uint64(i)*10_000_000_000, 200_000_000_000 + uint64(i)*10_000_000_000, 20_000_000_000 + uint64(i)*1_000_000_000}[:l]
		inputs[i] = makeNote(doc, uint64(10+i), state, owner, uint64(1100+i))
		_, _, _ = tree.Append(inputs[i].cm)
		aggregate = addState(aggregate, state)
	}
	root := tree.Root()
	for i := range inputs {
		inputs[i].path, _ = tree.Path(uint64(i))
		fillNote(&a.Inputs[i], inputs[i])
		nf := protocol.Nullifier(owner, inputs[i].cm)
		a.PublicInputs[i] = profile.InputPublic{Root: root, Commitment: inputs[i].cm, Nullifier: nf}
	}
	delta := make([]uint64, l)
	intermediate := append([]uint64(nil), aggregate...)
	if l > 0 {
		delta[0] = 5_000_000_000
		intermediate[0] -= delta[0]
	}
	if l > 1 {
		delta[1] = 20_000_000_000
		intermediate[1] += delta[1]
	}
	fillVars(a.ProcessDelta, delta)
	weights := allocations(arity)
	outputs := make([][]uint64, arity)
	for j := range outputs {
		outputs[j] = make([]uint64, l)
		for k := 0; k < l; k++ {
			a.Allocation[j][k] = weights[j]
		}
	}
	for j := 1; j < arity; j++ {
		for k := 0; k < l; k++ {
			quotient, remainder := mulDivRem(intermediate[k], weights[j], profile.Denominator)
			outputs[j][k] = quotient
			a.Remainders[j-1][k] = remainder
		}
	}
	for k := 0; k < l; k++ {
		outputs[0][k] = intermediate[k]
		for j := 1; j < arity; j++ {
			outputs[0][k] -= outputs[j][k]
		}
	}
	public := []fr.Element{protocol.Element(uint64(arity)), protocol.Element(uint64(arity))}
	for _, value := range delta {
		public = append(public, protocol.Element(value))
	}
	for j := 0; j < arity; j++ {
		for k := 0; k < l; k++ {
			public = append(public, protocol.Element(weights[j]))
		}
	}
	for i := 0; i < arity; i++ {
		public = append(public, root, inputs[i].cm, protocol.Nullifier(owner, inputs[i].cm))
	}
	for j := 0; j < arity; j++ {
		out := makeNote(protocol.Element(uint64(900+j)), uint64(1000+j), outputs[j], owner, uint64(1200+j))
		fillOutput(&a.Outputs[j], out)
		a.OutputCommitments[j] = out.cm
		public = append(public, out.cm)
	}
	c.Public = public
	return nil
}

func allocations(arity int) []uint64 {
	switch arity {
	case 1:
		return []uint64{profile.Denominator}
	case 2:
		return []uint64{600_000_000, 400_000_000}
	default:
		return []uint64{500_000_000, 300_000_000, 200_000_000}
	}
}

func makeNote(doc fr.Element, quantity uint64, state []uint64, owner protocol.Owner, opening uint64) note {
	return note{document: doc, quantity: quantity, state: append([]uint64(nil), state...), opening: opening, owner: owner, cm: commitment(doc, quantity, state, owner.Address, opening)}
}

func noteInTree(doc fr.Element, quantity uint64, state []uint64, owner protocol.Owner, opening uint64) (note, fr.Element) {
	n := makeNote(doc, quantity, state, owner, opening)
	tree := merkle.New(protocol.TreeDepth)
	_, root, _ := tree.Append(n.cm)
	n.path, _ = tree.Path(0)
	return n, root
}

func commitment(doc fr.Element, quantity uint64, state []uint64, address fr.Element, opening uint64) fr.Element {
	inputs := []fr.Element{doc, protocol.Element(quantity)}
	for _, value := range state {
		inputs = append(inputs, protocol.Element(value))
	}
	inputs = append(inputs, address, protocol.Element(opening))
	return zkhash.Hash(inputs...)
}

func voucherHash(doc fr.Element, quantity uint64, state []uint64, sender, receiver fr.Element, delta, opening uint64) fr.Element {
	inputs := []fr.Element{doc, protocol.Element(quantity)}
	for _, value := range state {
		inputs = append(inputs, protocol.Element(value))
	}
	inputs = append(inputs, sender, receiver, protocol.Element(delta), protocol.Element(opening))
	return zkhash.Hash(inputs...)
}

func fillNote(dst *profile.Note, n note) {
	dst.DocumentHash, dst.Quantity, dst.Opening = n.document, n.quantity, n.opening
	fillVars(dst.State, n.state)
	dst.Path = circuitPath(n.path)
}

func fillOutput(dst *profile.Output, n note) {
	dst.DocumentHash, dst.Quantity, dst.Opening = n.document, n.quantity, n.opening
	fillVars(dst.State, n.state)
}

func fillVoucher(dst *profile.Voucher, doc fr.Element, quantity uint64, state []uint64, sender, receiver fr.Element, delta, opening uint64, path merkle.Path) {
	dst.DocumentHash, dst.Quantity = doc, quantity
	fillVars(dst.State, state)
	dst.SenderAddress, dst.ReceiverAddress = sender, receiver
	dst.DeltaEpoch, dst.Opening, dst.Path = delta, opening, circuitPath(path)
}

func circuitPath(path merkle.Path) common.MerklePath {
	var out common.MerklePath
	out.Index = path.Index
	for i := range path.Siblings {
		out.Siblings[i] = path.Siblings[i]
	}
	return out
}

func fillVars(dst []frontend.Variable, values []uint64) {
	for i := range dst {
		dst[i] = values[i]
	}
}

func addState(a, b []uint64) []uint64 {
	out := make([]uint64, len(a))
	for i := range out {
		out[i] = a[i] + b[i]
	}
	return out
}

func subState(a, b []uint64) []uint64 {
	out := make([]uint64, len(a))
	for i := range out {
		out[i] = a[i] - b[i]
	}
	return out
}

func scaleState(values []uint64, numerator, denominator uint64) []uint64 {
	out := make([]uint64, len(values))
	for i := range out {
		out[i] = values[i] * numerator / denominator
	}
	return out
}

func floorState(values []uint64, numerator, denominator uint64) ([]uint64, []uint64) {
	out, remainder := make([]uint64, len(values)), make([]uint64, len(values))
	for i := range values {
		out[i], remainder[i] = mulDivRem(values[i], numerator, denominator)
	}
	return out, remainder
}

func mulDivRem(value, multiplier, denominator uint64) (uint64, uint64) {
	product := new(big.Int).Mul(new(big.Int).SetUint64(value), new(big.Int).SetUint64(multiplier))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, new(big.Int).SetUint64(denominator), remainder)
	return quotient.Uint64(), remainder.Uint64()
}
