package assignments

import (
	"fmt"

	"github.com/bighim/zkDPP/poc-v2/circuits/common"
	entrycircuit "github.com/bighim/zkDPP/poc-v2/circuits/entry"
	exitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/exit"
	mergecircuit "github.com/bighim/zkDPP/poc-v2/circuits/merge"
	proceedcircuit "github.com/bighim/zkDPP/poc-v2/circuits/proceed"
	processcircuit "github.com/bighim/zkDPP/poc-v2/circuits/process"
	recallcircuit "github.com/bighim/zkDPP/poc-v2/circuits/recall"
	splitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/split"
	transfercircuit "github.com/bighim/zkDPP/poc-v2/circuits/transfer"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
	"github.com/consensys/gnark/frontend"
)

type Canonical struct {
	Entries   map[string]entrycircuit.Circuit
	Transfers map[string]transfercircuit.Circuit
	Proceeds  map[string]proceedcircuit.Circuit
	Recalls   map[string]recallcircuit.Circuit
	Merges    map[string]mergecircuit.Circuit
	Splits    map[string]splitcircuit.Circuit
	Processes map[string]processcircuit.Circuit
	Exits     map[string]exitcircuit.Circuit
}

func Build(d *scenario.Derived) (*Canonical, error) {
	a := &Canonical{
		Entries: map[string]entrycircuit.Circuit{}, Transfers: map[string]transfercircuit.Circuit{},
		Proceeds: map[string]proceedcircuit.Circuit{}, Recalls: map[string]recallcircuit.Circuit{},
		Merges: map[string]mergecircuit.Circuit{}, Splits: map[string]splitcircuit.Circuit{},
		Processes: map[string]processcircuit.Circuit{}, Exits: map[string]exitcircuit.Circuit{},
	}
	for _, name := range d.EntryOrder {
		n := d.Notes[name]
		a.Entries[name] = entrycircuit.Circuit{Commitment: n.Commitment, Note: output(n), Address: n.Owner.Address}
	}
	for name := range d.Transfers {
		assignment, err := transferAssignment(d, name)
		if err != nil {
			return nil, err
		}
		a.Transfers[name] = assignment
	}
	for name := range d.Proceeds {
		assignment, err := proceedAssignment(d, name)
		if err != nil {
			return nil, err
		}
		a.Proceeds[name] = assignment
	}
	for name := range d.Recalls {
		assignment, err := recallAssignment(d, name)
		if err != nil {
			return nil, err
		}
		a.Recalls[name] = assignment
	}
	for name := range d.Merges {
		a.Merges[name] = mergeAssignment(d, name)
	}
	for name := range d.Splits {
		a.Splits[name] = splitAssignment(d, name)
	}
	for name := range d.Processes {
		assignment, err := processAssignment(d, name)
		if err != nil {
			return nil, err
		}
		a.Processes[name] = assignment
	}
	for name := range d.Exits {
		a.Exits[name] = exitAssignment(d, name)
	}
	return a, nil
}

func transferAssignment(d *scenario.Derived, name string) (transfercircuit.Circuit, error) {
	spec, ok := d.Transfers[name]
	if !ok {
		return transfercircuit.Circuit{}, fmt.Errorf("unknown transfer %s", name)
	}
	in, change, voucher := d.Notes[spec.Input], d.Notes[spec.Change], d.Vouchers[spec.Voucher]
	sender, receiver := d.Owners[spec.Sender], d.Owners[spec.Receiver]
	w := d.MTWitness["transfer:"+name]
	transferOut := common.OutputWitness{DocumentHash: voucher.DocumentHash, Quantity: voucher.Quantity, State: frontState(voucher.State), Opening: 0}
	rem := d.Remainders["transfer:"+name+":change"]
	return transfercircuit.Circuit{
		Root: w.Root, InputCommitment: in.Commitment, Nullifier: protocol.Nullifier(sender, in.Commitment),
		VoucherCommitment: voucher.Commitment, ChangeCommitment: change.Commitment, DeltaEpoch: voucher.DeltaEpoch,
		SenderSecretKey: sender.SecretKey, SenderPublicKey: sender.PublicKey, ReceiverPublicKey: receiver.PublicKey,
		SenderAddress: sender.Address, ReceiverAddress: receiver.Address, Input: note(in, w), Transfer: transferOut,
		Change: output(change), VoucherOpening: voucher.Opening, Remainder: variables(rem),
	}, nil
}

func proceedAssignment(d *scenario.Derived, name string) (proceedcircuit.Circuit, error) {
	spec, ok := d.Proceeds[name]
	if !ok {
		return proceedcircuit.Circuit{}, fmt.Errorf("unknown proceed %s", name)
	}
	voucher, out, receiver := d.Vouchers[spec.Voucher], d.Notes[spec.Output], d.Owners[spec.Owner]
	w := d.RVWitness["proceed:"+name]
	return proceedcircuit.Circuit{VoucherRoot: w.Root, VoucherCommitment: voucher.Commitment, VoucherNullifier: protocol.VoucherNullifier(voucher), OutputCommitment: out.Commitment, ReceiverSecretKey: receiver.SecretKey, ReceiverPublicKey: receiver.PublicKey, Voucher: voucherWitness(voucher, w), Output: output(out)}, nil
}

func recallAssignment(d *scenario.Derived, name string) (recallcircuit.Circuit, error) {
	spec, ok := d.Recalls[name]
	if !ok {
		return recallcircuit.Circuit{}, fmt.Errorf("unknown recall %s", name)
	}
	voucher, out, sender := d.Vouchers[spec.Voucher], d.Notes[spec.Output], d.Owners[spec.Owner]
	w := d.RVWitness["recall:"+name]
	return recallcircuit.Circuit{VoucherRoot: w.Root, VoucherCommitment: voucher.Commitment, VoucherNullifier: protocol.VoucherNullifier(voucher), OutputCommitment: out.Commitment, SenderSecretKey: sender.SecretKey, SenderPublicKey: sender.PublicKey, Voucher: voucherWitness(voucher, w), Output: output(out)}, nil
}

func mergeAssignment(d *scenario.Derived, name string) mergecircuit.Circuit {
	spec := d.Merges[name]
	in1, in2, out, owner := d.Notes[spec.Inputs[0]], d.Notes[spec.Inputs[1]], d.Notes[spec.Output], d.Owners[spec.Owner]
	w1, w2 := d.MTWitness["merge:"+name+":1"], d.MTWitness["merge:"+name+":2"]
	return mergecircuit.Circuit{Root1: w1.Root, Root2: w2.Root, Commitment1: in1.Commitment, Commitment2: in2.Commitment, Nullifier1: protocol.Nullifier(owner, in1.Commitment), Nullifier2: protocol.Nullifier(owner, in2.Commitment), OutputCommitment: out.Commitment, SecretKey: owner.SecretKey, PublicKey: owner.PublicKey, Address: owner.Address, Inputs: [2]common.NoteWitness{note(in1, w1), note(in2, w2)}, Output: output(out)}
}

func splitAssignment(d *scenario.Derived, name string) splitcircuit.Circuit {
	spec := d.Splits[name]
	in, out1, out2, owner := d.Notes[spec.Input], d.Notes[spec.Output1], d.Notes[spec.Output2], d.Owners[spec.Owner]
	w := d.MTWitness["split:"+name]
	rem := d.Remainders["split:"+name+":output2"]
	return splitcircuit.Circuit{Root: w.Root, InputCommitment: in.Commitment, Nullifier: protocol.Nullifier(owner, in.Commitment), OutputCommitment1: out1.Commitment, OutputCommitment2: out2.Commitment, SecretKey: owner.SecretKey, PublicKey: owner.PublicKey, Address: owner.Address, Input: note(in, w), Outputs: [2]common.OutputWitness{output(out1), output(out2)}, Remainder: variables(rem)}
}

func processAssignment(d *scenario.Derived, name string) (processcircuit.Circuit, error) {
	spec := d.Processes[name]
	owner := d.Owners[spec.Owner]
	c := zeroProcess()
	c.M, c.N = spec.M, spec.N
	c.SecretKey, c.PublicKey, c.Address = owner.SecretKey, owner.PublicKey, owner.Address
	for k := 0; k < 3; k++ {
		c.ProcessDelta[k] = spec.Delta[k]
	}
	for j := 0; j < 3; j++ {
		for k := 0; k < 3; k++ {
			c.Allocation[j][k] = spec.Allocation[j][k]
		}
	}
	for i, inputName := range spec.Inputs {
		n := d.Notes[inputName]
		w := d.MTWitness[fmt.Sprintf("process:%s:%d", name, i)]
		c.PublicInputs[i] = processcircuit.InputPublic{Root: w.Root, Commitment: n.Commitment, Nullifier: protocol.Nullifier(owner, n.Commitment)}
		c.Inputs[i] = note(n, w)
	}
	for j, outputName := range spec.Outputs {
		n := d.Notes[outputName]
		c.OutputCommitments[j], c.Outputs[j] = n.Commitment, output(n)
		if j > 0 {
			c.Remainders[j-1] = variables(d.Remainders[fmt.Sprintf("process:%s:output%d", name, j+1)])
		}
	}
	return c, nil
}

func exitAssignment(d *scenario.Derived, name string) exitcircuit.Circuit {
	spec := d.Exits[name]
	n, owner, w := d.Notes[spec.Input], d.Owners[spec.Owner], d.MTWitness["exit:"+name]
	return exitcircuit.Circuit{Root: w.Root, Commitment: n.Commitment, Nullifier: protocol.Nullifier(owner, n.Commitment), SecretKey: owner.SecretKey, PublicKey: owner.PublicKey, Address: owner.Address, Note: note(n, w)}
}

func zeroProcess() processcircuit.Circuit {
	var c processcircuit.Circuit
	c.M, c.N, c.SecretKey, c.PublicKey, c.Address = 0, 0, 0, 0, 0
	for k := 0; k < 3; k++ {
		c.ProcessDelta[k] = 0
	}
	for i := 0; i < 3; i++ {
		c.PublicInputs[i] = processcircuit.InputPublic{Root: 0, Commitment: 0, Nullifier: 0}
		c.Inputs[i] = zeroNote()
		c.OutputCommitments[i], c.Outputs[i] = 0, zeroOutput()
		for k := 0; k < 3; k++ {
			c.Allocation[i][k] = 0
			if i < 2 {
				c.Remainders[i][k] = 0
			}
		}
	}
	return c
}

func zeroNote() common.NoteWitness {
	n := common.NoteWitness{DocumentHash: 0, Quantity: 0, Opening: 0}
	for k := 0; k < 3; k++ {
		n.State[k] = 0
	}
	n.Path.Index = 0
	for i := range n.Path.Siblings {
		n.Path.Siblings[i] = 0
	}
	return n
}
func zeroOutput() common.OutputWitness {
	return common.OutputWitness{DocumentHash: 0, Quantity: 0, State: [3]frontend.Variable{0, 0, 0}, Opening: 0}
}
func note(n protocol.Note, w scenario.Membership) common.NoteWitness {
	return common.NoteWitness{DocumentHash: n.DocumentHash, Quantity: n.Quantity, State: frontState(n.State), Opening: n.Opening, Path: path(w)}
}
func output(n protocol.Note) common.OutputWitness {
	return common.OutputWitness{DocumentHash: n.DocumentHash, Quantity: n.Quantity, State: frontState(n.State), Opening: n.Opening}
}
func voucherWitness(v protocol.Voucher, w scenario.Membership) common.VoucherWitness {
	return common.VoucherWitness{DocumentHash: v.DocumentHash, Quantity: v.Quantity, State: frontState(v.State), SenderAddress: v.SenderAddress, ReceiverAddress: v.ReceiverAddress, DeltaEpoch: v.DeltaEpoch, Opening: v.Opening, Path: path(w)}
}
func path(w scenario.Membership) common.MerklePath {
	var p common.MerklePath
	p.Index = w.Path.Index
	for i := range w.Path.Siblings {
		p.Siblings[i] = w.Path.Siblings[i]
	}
	return p
}
func frontState(s protocol.State) [3]frontend.Variable { return [3]frontend.Variable{s[0], s[1], s[2]} }
func variables(s [3]uint64) [3]frontend.Variable       { return [3]frontend.Variable{s[0], s[1], s[2]} }
