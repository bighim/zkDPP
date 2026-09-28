package experiments

// This deliberately linkable direct-transfer alternative is isolated from the
// owner-secret nullifier ledger. Knowing an opening reveals nf, not ownership.
import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"math/big"
)

type SharedCircuit Circuit

func sharedProfile(kind int) Profile {
	p := Profile{Kind: kind, Audit: true}
	switch kind {
	case Entry:
		p.Name = "shared-entry"
		p.N = 1
	case Direct:
		p.Name = "shared-direct"
		p.M = 1
		p.N = 2
	case Exit:
		p.Name = "shared-exit"
		p.M = 1
	default:
		panic("shared profile")
	}
	return p
}
func newSharedCircuit(p Profile) *SharedCircuit { return (*SharedCircuit)(NewCircuit(p, defaultPK())) }

var sharedResolutionTag = h.MustHashToField("zkDPP:ExperimentSharedResolution:v1")

func sharedNF(n note.Note) fr.Element {
	return h.Hash(h.NullifierTag, n.Commitment, h.Hash(sharedResolutionTag, n.Opening))
}

func (c *SharedCircuit) Define(api frontend.API) error {
	p, v := c.Profile, c.Public
	addr := must(circuitutil.Address(api, c.Secret))
	cm := func(n circuitutil.NoteWitness) frontend.Variable {
		circuitutil.AssertNote(api, n)
		return must(circuitutil.Commitment(api, n))
	}
	nf := func(n circuitutil.NoteWitness, x frontend.Variable) frontend.Variable {
		return must(circuitutil.Hash(api, h.NullifierTag, x, must(circuitutil.Hash(api, sharedResolutionTag, n.Opening))))
	}
	parents, outputs := []frontend.Variable{}, []frontend.Variable{}
	n := c.Notes[0]
	x := cm(n)
	api.AssertIsEqual(n.Address, addr)
	if p.Kind == Entry {
		api.AssertIsEqual(x, v[0])
		outputs = append(outputs, nf(n, x))
	} else {
		if e := circuitutil.AssertMembership(api, v[0], x, c.Paths[0]); e != nil {
			return e
		}
		api.AssertIsEqual(v[1], nf(n, x))
		parents = append(parents, x)
	}
	if p.Kind == Direct {
		recv, change := c.Notes[1], c.Notes[2]
		rc, cc := cm(recv), cm(change)
		api.AssertIsEqual(v[2], rc)
		api.AssertIsEqual(v[3], cc)
		api.AssertIsDifferent(rc, cc)
		api.AssertIsEqual(n.AssetRole, 0)
		api.AssertIsEqual(recv.AssetRole, 0)
		api.AssertIsEqual(change.AssetRole, 0)
		api.AssertIsEqual(recv.DocumentHash, n.DocumentHash)
		api.AssertIsEqual(change.DocumentHash, n.DocumentHash)
		api.AssertIsEqual(change.Address, addr)
		api.AssertIsDifferent(recv.QMass, 0)
		api.ToBinary(c.Aux[2], 64)
		api.AssertIsEqual(n.QMass, api.Add(recv.QMass, change.QMass))
		ratio(api, change.QMass, n.ARec, n.QMass, change.ARec, c.Aux[0])
		ratio(api, change.QMass, n.E, n.QMass, change.E, c.Aux[1])
		api.AssertIsEqual(n.ARec, api.Add(recv.ARec, change.ARec))
		api.AssertIsEqual(recv.E, api.Add(api.Sub(n.E, change.E), c.Aux[2]))
		outputs = append(outputs, nf(recv, rc), nf(change, cc))
	}
	if p.Kind != Entry && p.Kind != Direct && p.Kind != Exit {
		return fmt.Errorf("unsupported shared relation")
	}
	b := len(p.BaseNames())
	msg := append(parents, outputs...)
	if len(msg) != p.M+p.N {
		return fmt.Errorf("shared audit shape")
	}
	return circuitutil.AssertMasterEncrypted(api, c.PK, msg, c.Randomness[0], ed.Point{X: v[b], Y: v[b+1]}, v[b+2:])
}

func sharedBuild(w *World, op Operation) (*Event, *SharedCircuit, error) {
	ev, e := w.Build(op)
	if e != nil {
		return nil, nil, e
	}
	if op.P.M > 0 {
		ev.Public[1] = sharedNF(op.Inputs[0])
	}
	ev.Future = nil
	for _, n := range ev.Outputs {
		ev.Future = append(ev.Future, sharedNF(n))
	}
	r := ev.Assignment.Randomness[0]
	ct, e := auditcrypto.EncryptWithMasterPublicKey(w.PK, append(append([]fr.Element{}, ev.Parents...), ev.Future...), r.(*big.Int))
	if e != nil {
		return nil, nil, e
	}
	ev.Cipher = ct
	b := len(op.P.BaseNames())
	ev.Public = append(ev.Public[:b:b], ct.R1.X, ct.R1.Y)
	ev.Public = append(ev.Public, ct.Data...)
	for i, x := range ev.Public {
		ev.Assignment.Public[i] = x
	}
	return ev, (*SharedCircuit)(ev.Assignment), nil
}
