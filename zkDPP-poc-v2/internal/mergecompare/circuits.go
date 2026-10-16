package mergecompare

// A/B reconstruct only the paper's three MerklePath, two GadgetEq and one
// GadgetAdd relations, using BLS12-381/Poseidon2. No application range,
// ItemCode equality, ownership, accumulator, or spent-state rules are added.
import (
	"fmt"
	cu "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

const Depth = 3 // 8 leaves in the paper's concrete document layout; first leaf, zero-based 0.
type Canonical struct {
	Roots       [3]frontend.Variable    `gnark:",public"`
	Commitments [3][4]frontend.Variable `gnark:",public"`
}
type PathOpening struct {
	Values   [4]frontend.Variable
	Salts    [4]frontend.Variable
	Siblings [Depth]frontend.Variable
}
type EqOpening struct{ Value, SaltLeft, SaltRight frontend.Variable }
type AddOpening struct{ Left, Right, SaltLeft, SaltRight, SaltSum frontend.Variable }
type PathCircuit struct {
	Root        frontend.Variable    `gnark:",public"`
	Commitments [4]frontend.Variable `gnark:",public"`
	Opening     PathOpening
}

func (c *PathCircuit) Define(api frontend.API) error {
	leaf, err := cu.Hash(api, c.Opening.Values[:]...)
	if err != nil {
		return err
	}
	for i := range c.Commitments {
		v, e := cu.Hash(api, c.Opening.Values[i], c.Opening.Salts[i])
		if e != nil {
			return e
		}
		api.AssertIsEqual(v, c.Commitments[i])
	}
	for _, sibling := range c.Opening.Siblings {
		leaf, err = cu.Compress(api, leaf, sibling)
		if err != nil {
			return err
		}
	}
	api.AssertIsEqual(leaf, c.Root)
	return nil
}

type EqCircuit struct {
	Left, Right frontend.Variable `gnark:",public"`
	Opening     EqOpening
}

func (c *EqCircuit) Define(api frontend.API) error {
	l, e := cu.Hash(api, c.Opening.Value, c.Opening.SaltLeft)
	if e != nil {
		return e
	}
	r, e := cu.Hash(api, c.Opening.Value, c.Opening.SaltRight)
	if e != nil {
		return e
	}
	api.AssertIsEqual(l, c.Left)
	api.AssertIsEqual(r, c.Right)
	return nil
}

type AddCircuit struct {
	Left, Right, Sum frontend.Variable `gnark:",public"`
	Opening          AddOpening
}

func (c *AddCircuit) Define(api frontend.API) error {
	l, e := cu.Hash(api, c.Opening.Left, c.Opening.SaltLeft)
	if e != nil {
		return e
	}
	r, e := cu.Hash(api, c.Opening.Right, c.Opening.SaltRight)
	if e != nil {
		return e
	}
	z, e := cu.Hash(api, api.Add(c.Opening.Left, c.Opening.Right), c.Opening.SaltSum)
	if e != nil {
		return e
	}
	api.AssertIsEqual(l, c.Left)
	api.AssertIsEqual(r, c.Right)
	api.AssertIsEqual(z, c.Sum)
	return nil
}

// Independent gadget witnesses are retained in B, so it is literally A's
// conjunction. It does not silently replace repeated openings with a stronger
// shared-preimage relation. Both arms use the same canonical external inputs.
type Conjunction struct {
	Statement  Canonical
	Paths      [3]PathOpening
	Equalities [2]EqOpening
	Addition   AddOpening
}

func (c *Conjunction) Define(api frontend.API) error {
	for i := 0; i < 3; i++ {
		p := PathCircuit{Root: c.Statement.Roots[i], Commitments: c.Statement.Commitments[i], Opening: c.Paths[i]}
		if e := p.Define(api); e != nil {
			return e
		}
	}
	for i := 0; i < 2; i++ {
		q := EqCircuit{Left: c.Statement.Commitments[i][1], Right: c.Statement.Commitments[2][0], Opening: c.Equalities[i]}
		if e := q.Define(api); e != nil {
			return e
		}
	}
	a := AddCircuit{Left: c.Statement.Commitments[0][3], Right: c.Statement.Commitments[1][3], Sum: c.Statement.Commitments[2][3], Opening: c.Addition}
	return a.Define(api)
}

type Document struct {
	Values, Salts, Commitments [4]fr.Element
	Siblings                   [Depth]fr.Element
	Root                       fr.Element
}
type Transaction struct{ Documents [3]Document }

func MakeDocument(values [4]fr.Element, saltBase uint64) Document {
	d := Document{Values: values}
	for j := range d.Salts {
		d.Salts[j] = h.Element(saltBase + uint64(j+1))
		d.Commitments[j] = h.Hash(d.Values[j], d.Salts[j])
	}
	zero := fr.Element{}
	empty := h.Hash(zero, zero, zero, zero)
	nodes := make([]fr.Element, 1<<Depth)
	for i := range nodes {
		nodes[i] = empty
	}
	nodes[0] = h.Hash(d.Values[:]...)
	for level := 0; level < Depth; level++ {
		d.Siblings[level] = nodes[1]
		parents := make([]fr.Element, len(nodes)/2)
		for i := range parents {
			parents[i] = h.Compress(nodes[2*i], nodes[2*i+1])
		}
		nodes = parents
	}
	d.Root = nodes[0]
	return d
}
func Fixture(seed uint64) Transaction {
	vals := [3][4]uint64{{100, 300, 1001, 40e9}, {200, 300, 1001, 60e9}, {300, 400, 1001, 100e9}}
	var t Transaction
	for i := range vals {
		var v [4]fr.Element
		for j := range v {
			v[j] = h.Element(vals[i][j])
		}
		t.Documents[i] = MakeDocument(v, seed+uint64(i)*100)
	}
	return t
}
func (t Transaction) Canonical() Canonical {
	var s Canonical
	for i, d := range t.Documents {
		s.Roots[i] = d.Root
		for j := range d.Commitments {
			s.Commitments[i][j] = d.Commitments[j]
		}
	}
	return s
}
func (t Transaction) Bundle() [6]frontend.Circuit {
	var out [6]frontend.Circuit
	for i, d := range t.Documents {
		var w PathOpening
		for j := range w.Values {
			w.Values[j] = d.Values[j]
			w.Salts[j] = d.Salts[j]
		}
		for j := range w.Siblings {
			w.Siblings[j] = d.Siblings[j]
		}
		out[i] = &PathCircuit{Root: d.Root, Commitments: t.Canonical().Commitments[i], Opening: w}
	}
	for i := 0; i < 2; i++ {
		out[3+i] = &EqCircuit{Left: t.Documents[i].Commitments[1], Right: t.Documents[2].Commitments[0], Opening: EqOpening{t.Documents[i].Values[1], t.Documents[i].Salts[1], t.Documents[2].Salts[0]}}
	}
	out[5] = &AddCircuit{Left: t.Documents[0].Commitments[3], Right: t.Documents[1].Commitments[3], Sum: t.Documents[2].Commitments[3], Opening: AddOpening{t.Documents[0].Values[3], t.Documents[1].Values[3], t.Documents[0].Salts[3], t.Documents[1].Salts[3], t.Documents[2].Salts[3]}}
	return out
}
func (t Transaction) Combined() *Conjunction {
	b := t.Bundle()
	c := &Conjunction{Statement: t.Canonical()}
	for i := range c.Paths {
		c.Paths[i] = b[i].(*PathCircuit).Opening
	}
	for i := range c.Equalities {
		c.Equalities[i] = b[3+i].(*EqCircuit).Opening
	}
	c.Addition = b[5].(*AddCircuit).Opening
	return c
}

// PublicAtomic derives all verifier inputs from ONE canonical transaction.
// Callers cannot supply a separately chosen input vector for any gadget.
func PublicAtomic(s Canonical) [6]frontend.Circuit {
	var out [6]frontend.Circuit
	for i := 0; i < 3; i++ {
		out[i] = &PathCircuit{Root: s.Roots[i], Commitments: s.Commitments[i]}
	}
	for i := 0; i < 2; i++ {
		out[3+i] = &EqCircuit{Left: s.Commitments[i][1], Right: s.Commitments[2][0]}
	}
	out[5] = &AddCircuit{Left: s.Commitments[0][3], Right: s.Commitments[1][3], Sum: s.Commitments[2][3]}
	return out
}
func Definition(name string) frontend.Circuit {
	switch name {
	case "path":
		return &PathCircuit{}
	case "eq":
		return &EqCircuit{}
	case "add":
		return &AddCircuit{}
	case "B":
		return &Conjunction{}
	}
	panic(fmt.Sprintf("unknown definition %s", name))
}
