package m5case

import (
	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	processcircuit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/process_policy_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"path/filepath"
)

type EntryCase struct {
	Name, ActorID string
	Note          note.Note
	Assignment    *entrycircuit.Circuit
}
type Scenario struct {
	Entries              []EntryCase
	Inputs               [3]note.Note
	Outputs              [2]note.Note
	Result               policy.Result
	Assignment           *processcircuit.Circuit
	InputRoot, FinalRoot fr.Element
	FinalCount           uint64
	FinalPaths           []merkle.Path
	PolicyRef, ScopeRef  fr.Element
}

func Build(root string) (*Scenario, error) {
	actors, e := testkit.LoadActors(filepath.Join(root, "testdata", "common", "actors-v1.json"))
	if e != nil {
		return nil, e
	}
	owner, _ := testkit.ByID(actors, "actor-1")
	states := [3]note.State{{QMass: 100e9, ARec: 20e9, E: 80e9}, {QMass: 120e9, ARec: 10e9, E: 90e9}, {QMass: 100e9, E: 60e9}}
	names := []string{"A", "B", "C"}
	tree := merkle.New()
	var inputs [3]note.Note
	entries := make([]EntryCase, 3)
	for i := 0; i < 3; i++ {
		d, err := document.Hash(document.DocumentInfo{ProductName: "M5 Input " + names[i], LotID: "M5-" + names[i] + "-001"})
		if err != nil {
			return nil, err
		}
		v, err := note.New(d, states[i], note.AssetRoleEligible, owner.Address, zkhash.Element(uint64(9001+i)))
		if err != nil {
			return nil, err
		}
		inputs[i] = v
		entries[i] = EntryCase{"input-" + names[i], "actor-1", v, entrycircuit.Assignment(v)}
		if _, _, err = tree.Append(v.Commitment); err != nil {
			return nil, err
		}
	}
	rootIn := tree.Root()
	var paths [3]merkle.Path
	for i := 0; i < 3; i++ {
		paths[i], e = tree.Path(uint64(i))
		if e != nil {
			return nil, e
		}
	}
	res, e := policy.Apply(states)
	if e != nil {
		return nil, e
	}
	d0, e := document.Hash(document.DocumentInfo{ProductName: "M5 Eligible Output", LotID: "M5-OUT-E"})
	if e != nil {
		return nil, e
	}
	d1, e := document.Hash(document.DocumentInfo{ProductName: "M5 Waste Output", LotID: "M5-OUT-W"})
	if e != nil {
		return nil, e
	}
	o0, e := note.New(d0, res.Eligible, note.AssetRoleEligible, owner.Address, zkhash.Element(9101))
	if e != nil {
		return nil, e
	}
	o1, e := note.New(d1, res.Waste, note.AssetRoleWaste, owner.Address, zkhash.Element(9102))
	if e != nil {
		return nil, e
	}
	outs := [2]note.Note{o0, o1}
	assignment := processcircuit.Assignment(inputs, owner.SKOwner, rootIn, paths, outs, res)
	_, _, _ = tree.Append(o0.Commitment)
	_, _, _ = tree.Append(o1.Commitment)
	s := &Scenario{entries, inputs, outs, res, assignment, rootIn, tree.Root(), tree.Count(), nil, policy.CanonicalRef(), policy.ScopeRef(owner.SKOwner, policy.CanonicalRef())}
	for i := uint64(0); i < tree.Count(); i++ {
		p, _ := tree.Path(i)
		s.FinalPaths = append(s.FinalPaths, p)
	}
	return s, nil
}
