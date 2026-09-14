package experiments

import (
	"crypto/rand"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"math/big"
)

type VectorNote struct {
	Document, Address, Opening, CM fr.Element
	State                          []uint64
}
type Operation struct {
	Prepared     bool
	P            Profile
	Inputs       []note.Note
	Voucher      voucher.Voucher
	SK, Receiver fr.Element
	State        note.State
	Document     fr.Element
	Q, Transport uint64
	Paths        []merkle.Path
	Root         fr.Element
	VectorInputs []VectorNote
	VectorState  []uint64
}
type Event struct {
	Profile         Profile
	Assignment      *Circuit
	Public          []fr.Element
	Outputs         []note.Note
	Voucher         *voucher.Voucher
	VectorOutputs   []VectorNote
	Claim           *claim.DPPClaim
	Parents, Future []fr.Element
	Cipher          auditcrypto.Ciphertext
}
type World struct {
	PK              auditcrypto.PublicKey
	Notes, Vouchers *merkle.Tree
	NI, VI          map[string]uint64
	Serial          uint64
	History         []*Event
}

func NewWorld(pk auditcrypto.PublicKey, seed uint64) *World {
	return &World{PK: pk, Notes: merkle.New(), Vouchers: merkle.New(), NI: map[string]uint64{}, VI: map[string]uint64{}, Serial: seed}
}
func (w *World) opening() fr.Element { w.Serial++; return h.Element(w.Serial) }
func (w *World) MakeNote(doc fr.Element, s note.State, role note.AssetRole, address fr.Element) note.Note {
	return must(note.New(doc, s, role, address, w.opening()))
}
func (w *World) Ready(op Operation) Operation {
	if op.Prepared {
		return op
	}
	op.Root = w.Notes.Root()
	for _, n := range op.Inputs {
		op.Paths = append(op.Paths, must(w.Notes.Path(w.NI[n.Commitment.String()])))
	}
	for _, n := range op.VectorInputs {
		op.Paths = append(op.Paths, must(w.Notes.Path(w.NI[n.CM.String()])))
	}
	if op.P.Kind == Proceed || op.P.Kind == Recall {
		op.Root = w.Vouchers.Root()
		op.Paths = []merkle.Path{must(w.Vouchers.Path(w.VI[op.Voucher.Commitment.String()]))}
	}
	op.Prepared = true
	return op
}
func nw(n note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: n.DocumentHash, AssetRole: uint64(n.AssetRole), QMass: n.State.QMass, ARec: n.State.ARec, E: n.State.E, Address: n.Address, Opening: n.Opening}
}
func vw(v voucher.Voucher) circuitutil.VoucherWitness {
	return circuitutil.VoucherWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, SenderAddress: v.SenderAddress, ReceiverAddress: v.ReceiverAddress, DeadlineBlock: v.DeadlineBlock, Opening: v.Opening}
}
func path(p merkle.Path) circuitutil.MerklePath {
	x := circuitutil.MerklePath{Index: p.Index}
	for i, s := range p.Siblings {
		x.Siblings[i] = s
	}
	return x
}
func divrem(x, y, d uint64) (uint64, uint64) {
	z := new(big.Int).Mul(new(big.Int).SetUint64(x), new(big.Int).SetUint64(y))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(z, new(big.Int).SetUint64(d), r)
	return q.Uint64(), r.Uint64()
}
func ProcessStates(in []note.State, n int) ([]note.State, []uint64, error) {
	if len(in) < 2 || len(in) > 4 || n < 1 || n > 3 {
		return nil, nil, fmt.Errorf("arity")
	}
	var total note.State
	for _, x := range in {
		var e error
		total, e = allocation.Add(total, x)
		if e != nil {
			return nil, nil, e
		}
	}
	loss, rl := divrem(total.QMass, 62500000, Scale)
	add, rc := divrem(total.QMass, 93750000, Scale)
	carbon := total.E + add
	if carbon < total.E {
		return nil, nil, fmt.Errorf("overflow")
	}
	mid := note.State{QMass: total.QMass - loss, ARec: total.ARec, E: carbon}
	if mid.QMass == 0 {
		return nil, nil, fmt.Errorf("zero process output")
	}
	out := make([]note.State, n)
	rem := []uint64{rl, rc}
	out[0] = mid
	for j := 1; j < n; j++ {
		weights := []uint64{1}
		den := uint64(1)
		if n == 2 {
			weights = []uint64{2, 1}
			den = 3
		} else if n == 3 {
			weights = []uint64{3, 2, 1}
			den = 6
		}
		q, _ := divrem(mid.QMass, weights[j], den)
		a, ra := divrem(mid.ARec, q, mid.QMass)
		e, re := divrem(mid.E, q, mid.QMass)
		out[j] = note.State{QMass: q, ARec: a, E: e}
		out[0].QMass -= q
		out[0].ARec -= a
		out[0].E -= e
		rem = append(rem, ra, re)
	}
	for _, o := range out {
		if o.QMass == 0 {
			return nil, nil, fmt.Errorf("zero output")
		}
		if e := note.ValidateState(o, note.AssetRoleEligible); e != nil {
			return nil, nil, e
		}
	}
	return out, rem, nil
}
func vectorCM(v VectorNote) fr.Element {
	fields := []fr.Element{vectorTag, v.Document}
	for _, x := range v.State {
		fields = append(fields, h.Element(x))
	}
	fields = append(fields, v.Address, v.Opening)
	return h.Hash(fields...)
}
func vectorWitness(v VectorNote) VectorWitness {
	n := VectorWitness{DocumentHash: v.Document, Address: v.Address, Opening: v.Opening, State: make([]frontend.Variable, len(v.State))}
	for i, x := range v.State {
		n.State[i] = x
	}
	return n
}
func (w *World) Build(op Operation) (ev *Event, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("native build: %v", x)
		}
	}()
	p := op.P
	c := NewCircuit(p, w.PK)
	c.Secret = op.SK
	address := owner.Address(op.SK)
	e := &Event{Profile: p, Assignment: c}
	base := []fr.Element{}
	for i, x := range op.Paths {
		c.Paths[i] = path(x)
	}
	for _, n := range op.Inputs {
		e.Parents = append(e.Parents, n.Commitment)
	}
	out := func(doc fr.Element, s note.State, addr fr.Element) note.Note {
		n := w.MakeNote(doc, s, note.AssetRoleEligible, addr)
		e.Outputs = append(e.Outputs, n)
		if p.Audit {
			e.Future = append(e.Future, note.Nullifier(op.SK, n.Commitment))
		}
		return n
	}
	switch p.Kind {
	case Entry:
		n := out(op.Document, op.State, address)
		c.Notes[0] = nw(n)
		base = []fr.Element{n.Commitment}
	case Transfer, Direct:
		n := op.Inputs[0]
		q := op.Q
		if q == 0 {
			q = n.State.QMass
		}
		vs, cs, r, er := voucher.AllocateWithTransport(n.State, q, op.Transport)
		if er != nil {
			return nil, er
		}
		c.Notes[0] = nw(n)
		for i, x := range []uint64{r.ARec, r.E, op.Transport} {
			c.Aux[i] = x
		}
		var first fr.Element
		if p.Kind == Transfer {
			v := must(voucher.New(n.DocumentHash, n.AssetRole, vs, address, op.Receiver, 1000000, w.opening()))
			e.Voucher = &v
			c.Vouchers[0] = vw(v)
			first = v.Commitment
			e.Future = []fr.Element{voucher.Nullifier(v.Opening, v.Commitment)}
		} else {
			n2 := out(n.DocumentHash, vs, op.Receiver)
			c.Notes[1] = nw(n2)
			first = n2.Commitment
		}
		change := out(n.DocumentHash, cs, address)
		c.Notes[len(c.Notes)-1] = nw(change)
		base = []fr.Element{op.Root, note.Nullifier(op.SK, n.Commitment), first, change.Commitment}
		if p.Kind == Transfer {
			base = append(base, h.Element(1000000))
		}
	case Proceed, Recall:
		v := op.Voucher
		n := out(v.DocumentHash, v.State, address)
		c.Notes[0] = nw(n)
		c.Vouchers[0] = vw(v)
		e.Parents = []fr.Element{v.Commitment}
		base = []fr.Element{op.Root, voucher.Nullifier(v.Opening, v.Commitment), n.Commitment}
		if p.Kind == Recall {
			base = append(base, h.Element(v.DeadlineBlock))
		}
	case Merge:
		a, b := op.Inputs[0], op.Inputs[1]
		o := out(a.DocumentHash, must(allocation.Add(a.State, b.State)), address)
		c.Notes[0], c.Notes[1], c.Notes[2] = nw(a), nw(b), nw(o)
		base = []fr.Element{op.Root, note.Nullifier(op.SK, a.Commitment), note.Nullifier(op.SK, b.Commitment), o.Commitment}
	case Split:
		a := op.Inputs[0]
		s1, s2, r, er := allocation.ByMass(a.State, op.Q)
		if er != nil {
			return nil, er
		}
		n1, n2 := out(a.DocumentHash, s1, address), out(a.DocumentHash, s2, address)
		c.Notes[0], c.Notes[1], c.Notes[2] = nw(a), nw(n1), nw(n2)
		c.Aux[0], c.Aux[1] = r.ARec, r.E
		base = []fr.Element{op.Root, note.Nullifier(op.SK, a.Commitment), n1.Commitment, n2.Commitment}
	case Exit, Issue:
		n := op.Inputs[0]
		c.Notes[0] = nw(n)
		base = []fr.Element{op.Root, note.Nullifier(op.SK, n.Commitment)}
		if p.Kind == Issue {
			nonce := w.opening()
			handle := claim.Handle(n.DocumentHash, p.PolicyRef, nonce)
			e.Claim = &claim.DPPClaim{IssuePolicyRef: p.PolicyRef, Handle: handle, ClaimNonce: nonce}
			c.Aux[0] = nonce
			base = []fr.Element{p.PolicyRef, op.Root, base[1], handle}
		}
	case Process:
		states := make([]note.State, p.M)
		base = []fr.Element{p.PolicyRef, policy.ScopeRef(op.SK, p.PolicyRef), op.Root}
		for i, n := range op.Inputs {
			states[i] = n.State
			c.Notes[i] = nw(n)
			base = append(base, note.Nullifier(op.SK, n.Commitment))
		}
		os, rem, er := ProcessStates(states, p.N)
		if er != nil {
			return nil, er
		}
		for i, x := range rem {
			c.Aux[i] = x
		}
		for i, s := range os {
			n := out(op.Document, s, address)
			c.Notes[p.M+i] = nw(n)
			base = append(base, n.Commitment)
		}
	case VectorEntry, VectorMerge:
		state := append([]uint64{}, op.VectorState...)
		if p.Kind == VectorMerge {
			state = make([]uint64, p.StateCount)
			base = []fr.Element{op.Root}
			for i, n := range op.VectorInputs {
				c.Vectors[i] = vectorWitness(n)
				e.Parents = append(e.Parents, n.CM)
				base = append(base, note.Nullifier(op.SK, n.CM))
				for j, x := range n.State {
					old := state[j]
					state[j] += x
					if state[j] < old {
						return nil, fmt.Errorf("vector overflow")
					}
				}
			}
		}
		n := VectorNote{Document: op.Document, Address: address, Opening: w.opening(), State: state}
		n.CM = vectorCM(n)
		c.Vectors[p.M] = vectorWitness(n)
		e.VectorOutputs = []VectorNote{n}
		e.Future = []fr.Element{note.Nullifier(op.SK, n.CM)}
		base = append(base, n.CM)
	default:
		return nil, fmt.Errorf("unsupported native")
	}
	if len(base) != len(p.BaseNames()) {
		return nil, fmt.Errorf("base length")
	}
	e.Public = base
	if p.Audit {
		r := must(auditcrypto.RandomScalar(rand.Reader))
		ct := must(auditcrypto.EncryptWithMasterPublicKey(w.PK, append(append([]fr.Element{}, e.Parents...), e.Future...), r))
		e.Cipher = ct
		c.Randomness[0] = r
		e.Public = append(e.Public, ct.R1.X, ct.R1.Y)
		e.Public = append(e.Public, ct.Data...)
	}
	for i, v := range e.Public {
		c.Public[i] = v
	}
	return e, nil
}
func (w *World) Commit(e *Event) error {
	for _, n := range e.Outputs {
		w.NI[n.Commitment.String()] = w.Notes.Count()
		if _, _, err := w.Notes.Append(n.Commitment); err != nil {
			return err
		}
	}
	for _, n := range e.VectorOutputs {
		w.NI[n.CM.String()] = w.Notes.Count()
		if _, _, err := w.Notes.Append(n.CM); err != nil {
			return err
		}
	}
	if e.Voucher != nil {
		v := e.Voucher
		w.VI[v.Commitment.String()] = w.Vouchers.Count()
		if _, _, err := w.Vouchers.Append(v.Commitment); err != nil {
			return err
		}
	}
	w.History = append(w.History, e)
	return nil
}

func (w *World) Clone() *World {
	n := NewWorld(w.PK, w.Serial)
	for _, e := range w.History {
		must(0, n.Commit(e))
	}
	return n
}
func (e *Event) Witness() (frontend.Circuit, []fr.Element, error) {
	w, err := frontend.NewWitness(e.Assignment, ecc.BLS12_381.ScalarField())
	if err != nil {
		return nil, nil, err
	}
	p, err := w.Public()
	if err != nil {
		return nil, nil, err
	}
	actual := []fr.Element(p.Vector().(fr.Vector))
	if len(actual) != len(e.Public) {
		return nil, nil, fmt.Errorf("public count")
	}
	for i := range actual {
		if !actual[i].Equal(&e.Public[i]) {
			return nil, nil, fmt.Errorf("public order")
		}
	}
	return e.Assignment, actual, nil
}
func DefaultDocument() document.DocumentInfo {
	return document.DocumentInfo{ProductName: "Experiment Product", LotID: "EXPERIMENT-1", Unit: "kg"}
}
