package experiments

import (
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

const (
	Entry       = 0
	Transfer    = 1
	Proceed     = 2
	Recall      = 3
	Merge       = 4
	Split       = 5
	Process     = 6
	Exit        = 7
	Issue       = 8
	Direct      = 9
	VectorEntry = 10
	VectorMerge = 11
)
const Scale = uint64(1_000_000_000)

type Profile struct {
	Name                   string
	Kind, M, N, StateCount int
	Audit                  bool
	Condition              string
	PolicyRef              fr.Element
}

// Accept the limb encoding from the initial setup manifest and emit canonical
// field strings for all new metadata. The stored key files are not rewritten.
func (p Profile) MarshalJSON() ([]byte, error) {
	type wire Profile
	return json.Marshal(struct {
		wire
		PolicyRef string
	}{wire(p), p.PolicyRef.String()})
}
func (p *Profile) UnmarshalJSON(b []byte) error {
	type wire Profile
	x := struct {
		*wire
		PolicyRef json.RawMessage
	}{wire: (*wire)(p)}
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	if len(x.PolicyRef) == 0 {
		return nil
	}
	if x.PolicyRef[0] == '[' {
		var limbs [4]uint64
		if e := json.Unmarshal(x.PolicyRef, &limbs); e != nil {
			return e
		}
		p.PolicyRef = fr.Element(limbs)
		return nil
	}
	return json.Unmarshal(x.PolicyRef, &p.PolicyRef)
}

func Profiles() []Profile {
	var ps []Profile
	names := []string{"entry", "transfer", "proceed", "recall", "merge", "split", "process-2-1", "exit", "issue"}
	shapes := [][2]int{{0, 1}, {1, 2}, {1, 1}, {1, 1}, {2, 1}, {1, 2}, {2, 1}, {1, 0}, {1, 0}}
	for _, a := range []bool{false, true} {
		for k, n := range names {
			p := Profile{Name: n, Kind: k, M: shapes[k][0], N: shapes[k][1], Audit: a, Condition: "both"}
			if a {
				p.Name += "-audit"
			} else {
				p.Name += "-plain"
			}
			if k == Process {
				p.PolicyRef = policy.Ref(6, 1, 101, 1)
			}
			if k == Issue {
				p.PolicyRef = policy.Ref(8, 1, 2, 1)
			}
			ps = append(ps, p)
		}
		for i, s := range [][2]int{{2, 2}, {3, 2}, {3, 3}, {4, 3}} {
			p := Profile{Name: fmt.Sprintf("process-%d-%d", s[0], s[1]), Kind: Process, M: s[0], N: s[1], Audit: a, PolicyRef: policy.Ref(6, 1, 101, uint64(i+2))}
			if a {
				p.Name += "-audit"
			} else {
				p.Name += "-plain"
			}
			ps = append(ps, p)
		}
	}
	ps = append(ps, Profile{Name: "direct-plain", Kind: Direct, M: 1, N: 2})
	for _, s := range []int{1, 2, 3, 4, 8, 16} {
		for _, k := range []int{VectorEntry, VectorMerge} {
			m, n := 0, 1
			name := "vector-entry"
			if k == VectorMerge {
				m = 2
				name = "vector-merge"
			}
			ps = append(ps, Profile{Name: fmt.Sprintf("%s-%d", name, s), Kind: k, M: m, N: n, StateCount: s, Audit: true})
		}
	}
	for i, cond := range []string{"recycled", "carbon"} {
		ps = append(ps, Profile{Name: "issue-" + cond + "-audit", Kind: Issue, M: 1, Audit: true, Condition: cond, PolicyRef: policy.Ref(8, 1, 102, uint64(i+1))})
	}
	return ps
}
func ProfileByName(name string) (Profile, error) {
	for _, p := range Profiles() {
		if p.Name == name {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("unknown profile %s", name)
}
func (p Profile) BaseNames() []string {
	switch p.Kind {
	case Entry, VectorEntry:
		return []string{"cm"}
	case Transfer:
		return []string{"noteRoot", "nf", "rvNew", "cmChange", "D"}
	case Proceed:
		return []string{"voucherRoot", "rvnf", "cmReceiver"}
	case Recall:
		return []string{"voucherRoot", "rvnf", "cmReturn", "D"}
	case Merge, VectorMerge:
		return []string{"noteRoot", "nf1", "nf2", "cmOut"}
	case Split:
		return []string{"noteRoot", "nf", "cmOut1", "cmOut2"}
	case Exit:
		return []string{"noteRoot", "nf"}
	case Issue:
		return []string{"policyRef", "noteRoot", "nf", "h"}
	case Direct:
		return []string{"noteRoot", "nf", "cmReceiver", "cmChange"}
	case Process:
		n := []string{"policyRef", "policyScopeRef", "noteRoot"}
		for i := 0; i < p.M; i++ {
			n = append(n, fmt.Sprintf("nf%d", i))
		}
		for i := 0; i < p.N; i++ {
			n = append(n, fmt.Sprintf("cm%d", i))
		}
		return n
	}
	panic("profile")
}
func (p Profile) PublicNames() []string {
	n := p.BaseNames()
	if p.Audit {
		n = append(n, "R1X", "R1Y")
		for i := 0; i < p.M; i++ {
			n = append(n, fmt.Sprintf("encryptedParent%d", i))
		}
		for i := 0; i < p.N; i++ {
			n = append(n, fmt.Sprintf("encryptedOutputNf%d", i))
		}
	}
	return n
}

type VectorWitness struct {
	DocumentHash, Address, Opening frontend.Variable
	State                          []frontend.Variable
}
type Circuit struct {
	Public     []frontend.Variable `gnark:",public"`
	Secret     frontend.Variable
	Notes      []circuitutil.NoteWitness
	Vouchers   []circuitutil.VoucherWitness
	Paths      []circuitutil.MerklePath
	Aux        []frontend.Variable
	Randomness []frontend.Variable
	Vectors    []VectorWitness
	Profile    Profile               `gnark:"-"`
	PK         auditcrypto.PublicKey `gnark:"-"`
}

func NewCircuit(p Profile, pk auditcrypto.PublicKey) *Circuit {
	c := &Circuit{Profile: p, PK: pk, Public: make([]frontend.Variable, len(p.PublicNames()))}
	if p.Audit {
		c.Randomness = make([]frontend.Variable, 1)
	}
	if p.StateCount > 0 {
		c.Vectors = make([]VectorWitness, p.M+p.N)
		for i := range c.Vectors {
			c.Vectors[i].State = make([]frontend.Variable, p.StateCount)
		}
	} else {
		count := p.M + p.N
		if p.Kind == Transfer {
			count = 2
		}
		if p.Kind == Proceed || p.Kind == Recall {
			count = 1
		}
		c.Notes = make([]circuitutil.NoteWitness, count)
	}
	c.Paths = make([]circuitutil.MerklePath, p.M)
	switch p.Kind {
	case Transfer:
		c.Vouchers = make([]circuitutil.VoucherWitness, 1)
		c.Aux = make([]frontend.Variable, 3)
	case Proceed, Recall:
		c.Vouchers = make([]circuitutil.VoucherWitness, 1)
	case Direct:
		c.Aux = make([]frontend.Variable, 3)
	case Split:
		c.Aux = make([]frontend.Variable, 2)
	case Process:
		c.Aux = make([]frontend.Variable, 2+2*(p.N-1))
	case Issue:
		c.Aux = make([]frontend.Variable, 1)
	}
	return c
}
func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}

var vectorTag = h.MustHashToField("zkDPP:ExperimentVectorNote:v1")

func (c *Circuit) Define(api frontend.API) error {
	p := c.Profile
	v := c.Public
	addr := must(circuitutil.Address(api, c.Secret))
	parents := []frontend.Variable{}
	outspends := []frontend.Variable{}
	nf := func(cm frontend.Variable) frontend.Variable { return must(circuitutil.Nullifier(api, c.Secret, cm)) }
	cm := func(n circuitutil.NoteWitness) frontend.Variable {
		circuitutil.AssertNote(api, n)
		return must(circuitutil.Commitment(api, n))
	}
	eligible := func(n circuitutil.NoteWitness) { api.AssertIsEqual(n.AssetRole, 0) }
	same := func(a, b circuitutil.NoteWitness) {
		api.AssertIsEqual(a.DocumentHash, b.DocumentHash)
		api.AssertIsEqual(a.AssetRole, b.AssetRole)
	}
	input := func(i int, root, spent frontend.Variable) frontend.Variable {
		n := c.Notes[i]
		api.AssertIsEqual(n.Address, addr)
		x := cm(n)
		if e := circuitutil.AssertMembership(api, root, x, c.Paths[i]); e != nil {
			panic(e)
		}
		api.AssertIsEqual(spent, nf(x))
		parents = append(parents, x)
		return x
	}
	output := func(n circuitutil.NoteWitness, want, owner frontend.Variable) {
		api.AssertIsEqual(n.Address, owner)
		x := cm(n)
		api.AssertIsEqual(x, want)
		if p.Audit {
			outspends = append(outspends, nf(x))
		}
	}
	switch p.Kind {
	case Entry:
		output(c.Notes[0], v[0], addr)
	case Exit:
		input(0, v[0], v[1])
	case Merge:
		a, b, o := c.Notes[0], c.Notes[1], c.Notes[2]
		x := input(0, v[0], v[1])
		y := input(1, v[0], v[2])
		api.AssertIsDifferent(x, y)
		api.AssertIsDifferent(v[1], v[2])
		eligible(a)
		eligible(b)
		same(a, b)
		same(a, o)
		api.AssertIsEqual(o.QMass, api.Add(a.QMass, b.QMass))
		api.AssertIsEqual(o.ARec, api.Add(a.ARec, b.ARec))
		api.AssertIsEqual(o.E, api.Add(a.E, b.E))
		output(o, v[3], addr)
	case Split:
		a, x, y := c.Notes[0], c.Notes[1], c.Notes[2]
		input(0, v[0], v[1])
		eligible(a)
		same(a, x)
		same(a, y)
		api.AssertIsDifferent(a.QMass, 0)
		api.AssertIsEqual(a.QMass, api.Add(x.QMass, y.QMass))
		api.AssertIsEqual(a.ARec, api.Add(x.ARec, y.ARec))
		api.AssertIsEqual(a.E, api.Add(x.E, y.E))
		ratio(api, y.QMass, a.ARec, a.QMass, y.ARec, c.Aux[0])
		ratio(api, y.QMass, a.E, a.QMass, y.E, c.Aux[1])
		output(x, v[2], addr)
		output(y, v[3], addr)
		api.AssertIsDifferent(v[2], v[3])
		for _, n := range []circuitutil.NoteWitness{x, y} {
			z := api.IsZero(n.QMass)
			api.AssertIsEqual(api.Mul(z, n.ARec), 0)
			api.AssertIsEqual(api.Mul(z, n.E), 0)
		}
	case Transfer, Direct:
		a := c.Notes[0]
		change := c.Notes[len(c.Notes)-1]
		input(0, v[0], v[1])
		eligible(a)
		same(a, change)
		api.ToBinary(c.Aux[2], 64)
		var q, rec, carbon, doc, role, recipient frontend.Variable
		if p.Kind == Transfer {
			u := c.Vouchers[0]
			circuitutil.AssertVoucher(api, u)
			api.ToBinary(v[4], 64)
			api.AssertIsEqual(u.DeadlineBlock, v[4])
			api.AssertIsEqual(u.SenderAddress, addr)
			rv := must(circuitutil.VoucherCommitment(api, u))
			api.AssertIsEqual(v[2], rv)
			if p.Audit {
				outspends = append(outspends, must(circuitutil.VoucherNullifier(api, u.Opening, rv)))
			}
			q, rec, carbon, doc, role, recipient = u.QMass, u.ARec, u.E, u.DocumentHash, u.AssetRole, u.ReceiverAddress
		} else {
			u := c.Notes[1]
			api.AssertIsEqual(cm(u), v[2])
			q, rec, carbon, doc, role, recipient = u.QMass, u.ARec, u.E, u.DocumentHash, u.AssetRole, u.Address
			_ = recipient
		}
		api.AssertIsDifferent(q, 0)
		api.AssertIsEqual(doc, a.DocumentHash)
		api.AssertIsEqual(role, 0)
		api.AssertIsEqual(a.QMass, api.Add(q, change.QMass))
		ratio(api, change.QMass, a.ARec, a.QMass, change.ARec, c.Aux[0])
		ratio(api, change.QMass, a.E, a.QMass, change.E, c.Aux[1])
		api.AssertIsEqual(a.ARec, api.Add(rec, change.ARec))
		api.AssertIsEqual(carbon, api.Add(api.Sub(a.E, change.E), c.Aux[2]))
		output(change, v[3], addr)
	case Proceed, Recall:
		u := c.Vouchers[0]
		circuitutil.AssertVoucher(api, u)
		api.AssertIsEqual(u.AssetRole, 0)
		owner := u.ReceiverAddress
		if p.Kind == Recall {
			owner = u.SenderAddress
			api.ToBinary(v[3], 64)
			api.AssertIsEqual(u.DeadlineBlock, v[3])
		}
		api.AssertIsEqual(owner, addr)
		rv := must(circuitutil.VoucherCommitment(api, u))
		if e := circuitutil.AssertMembership(api, v[0], rv, c.Paths[0]); e != nil {
			return e
		}
		api.AssertIsEqual(v[1], must(circuitutil.VoucherNullifier(api, u.Opening, rv)))
		parents = append(parents, rv)
		o := c.Notes[0]
		api.AssertIsEqual(o.DocumentHash, u.DocumentHash)
		api.AssertIsEqual(o.AssetRole, 0)
		api.AssertIsEqual(o.QMass, u.QMass)
		api.AssertIsEqual(o.ARec, u.ARec)
		api.AssertIsEqual(o.E, u.E)
		output(o, v[2], addr)
	case Issue:
		api.AssertIsEqual(v[0], p.PolicyRef)
		input(0, v[1], v[2])
		n := c.Notes[0]
		eligible(n)
		api.AssertIsDifferent(n.QMass, 0)
		if p.Condition != "carbon" {
			a, b := api.Mul(n.ARec, Scale), api.Mul(n.QMass, 100000000)
			api.ToBinary(a, 94)
			api.ToBinary(b, 94)
			api.AssertIsLessOrEqual(b, a)
		}
		if p.Condition != "recycled" {
			a, b := api.Mul(n.E, Scale), api.Mul(n.QMass, Scale)
			api.ToBinary(a, 94)
			api.ToBinary(b, 94)
			api.AssertIsLessOrEqual(a, b)
		}
		api.AssertIsEqual(v[3], must(circuitutil.Hash(api, h.IssueTag, n.DocumentHash, v[0], c.Aux[0])))
	case Process:
		api.AssertIsEqual(v[0], p.PolicyRef)
		api.AssertIsEqual(v[1], must(circuitutil.Hash(api, h.ScopeRefTag, c.Secret, v[0])))
		var q, a, e frontend.Variable = 0, 0, 0
		for i := 0; i < p.M; i++ {
			input(i, v[2], v[3+i])
			n := c.Notes[i]
			eligible(n)
			q = api.Add(q, n.QMass)
			a = api.Add(a, n.ARec)
			e = api.Add(e, n.E)
			for j := 0; j < i; j++ {
				api.AssertIsDifferent(v[3+i], v[3+j])
			}
		}
		api.ToBinary(q, 64)
		api.ToBinary(a, 64)
		api.ToBinary(e, 64)
		// Loss and carbon quotients are derived from the actual input sum.
		loss := api.Div(api.Sub(api.Mul(q, 62500000), c.Aux[0]), Scale)
		add := api.Div(api.Sub(api.Mul(q, 93750000), c.Aux[1]), Scale)
		ratio(api, q, 62500000, Scale, loss, c.Aux[0])
		ratio(api, q, 93750000, Scale, add, c.Aux[1])
		oq := api.Sub(q, loss)
		oe := api.Add(e, add)
		api.ToBinary(oq, 64)
		api.ToBinary(oe, 64)
		api.AssertIsDifferent(oq, 0)
		var sumQ, sumA, sumE frontend.Variable = 0, 0, 0
		for j := 0; j < p.N; j++ {
			o := c.Notes[p.M+j]
			eligible(o)
			api.AssertIsDifferent(o.QMass, 0)
			if j > 0 {
				same(c.Notes[p.M], o)
				ratio(api, o.QMass, a, oq, o.ARec, c.Aux[2+2*(j-1)])
				ratio(api, o.QMass, oe, oq, o.E, c.Aux[3+2*(j-1)])
			}
			sumQ = api.Add(sumQ, o.QMass)
			sumA = api.Add(sumA, o.ARec)
			sumE = api.Add(sumE, o.E)
			output(o, v[3+p.M+j], addr)
			for k := 0; k < j; k++ {
				api.AssertIsDifferent(v[3+p.M+j], v[3+p.M+k])
			}
		}
		api.AssertIsEqual(sumQ, oq)
		api.AssertIsEqual(sumA, a)
		api.AssertIsEqual(sumE, oe)
	case VectorEntry, VectorMerge:
		vc := func(n VectorWitness) frontend.Variable {
			api.AssertIsEqual(n.Address, addr)
			vals := []frontend.Variable{vectorTag, n.DocumentHash}
			for _, x := range n.State {
				api.ToBinary(x, 64)
				vals = append(vals, x)
			}
			vals = append(vals, n.Address, n.Opening)
			return must(circuitutil.Hash(api, vals...))
		}
		for i := 0; i < p.M; i++ {
			x := vc(c.Vectors[i])
			if e := circuitutil.AssertMembership(api, v[0], x, c.Paths[i]); e != nil {
				return e
			}
			api.AssertIsEqual(v[1+i], nf(x))
			parents = append(parents, x)
		}
		o := c.Vectors[p.M]
		x := vc(o)
		api.AssertIsDifferent(o.State[0], 0)
		pos := 0
		if p.Kind == VectorMerge {
			pos = 3
			api.AssertIsDifferent(v[1], v[2])
			for i := 0; i < 2; i++ {
				api.AssertIsEqual(c.Vectors[i].DocumentHash, o.DocumentHash)
			}
			for j := range o.State {
				api.AssertIsEqual(o.State[j], api.Add(c.Vectors[0].State[j], c.Vectors[1].State[j]))
			}
		}
		api.AssertIsEqual(v[pos], x)
		outspends = append(outspends, nf(x))
	default:
		return fmt.Errorf("unsupported circuit")
	}
	if p.Audit {
		base := len(p.BaseNames())
		msg := append(parents, outspends...)
		if len(msg) != p.M+p.N {
			return fmt.Errorf("audit layout")
		}
		return circuitutil.AssertMasterEncrypted(api, c.PK, msg, c.Randomness[0], ed.Point{X: v[base], Y: v[base+1]}, v[base+2:])
	}
	return nil
}
func ratio(api frontend.API, x, y, den, out, rem frontend.Variable) {
	api.ToBinary(rem, 64)
	api.ToBinary(out, 64)
	api.AssertIsEqual(api.Mul(x, y), api.Add(api.Mul(den, out), rem))
	api.AssertIsLessOrEqual(api.Add(rem, 1), den)
}
