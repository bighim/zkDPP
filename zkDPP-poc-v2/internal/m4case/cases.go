package m4case

import (
	"path/filepath"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	mergecircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/merge"
	splitcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type EntryCase struct {
	Name, ActorID string
	Note          note.Note
	Assignment    *entrycircuit.Circuit
}
type MergeCase struct {
	Inputs     [2]note.Note
	Output     note.Note
	Assignment *mergecircuit.Circuit
}
type SplitCase struct {
	Input      note.Note
	Outputs    [2]note.Note
	Remainder  allocation.Remainder
	Assignment *splitcircuit.Circuit
}
type Scenario struct {
	Entries    []EntryCase
	Merge      MergeCase
	Split      SplitCase
	FinalRoot  fr.Element
	FinalCount uint64
	FinalPaths []merkle.Path
}

func Build(root string) (*Scenario, error) {
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata", "common", "actors-v1.json"))
	if err != nil {
		return nil, err
	}
	owner, _ := testkit.ByID(actors, "actor-1")
	makeNote := func(product, lot string, state note.State, opening uint64) (note.Note, error) {
		d, e := document.Hash(document.DocumentInfo{ProductName: product, LotID: lot})
		if e != nil {
			return note.Note{}, e
		}
		return note.New(d, state, note.AssetRoleEligible, owner.Address, zkhash.Element(opening))
	}
	a, err := makeNote("M4 Material A", "M4-A-001", note.State{QMass: 3e9, ARec: 1e9, E: 2e9}, 7001)
	if err != nil {
		return nil, err
	}
	b, err := makeNote("M4 Material B", "M4-B-001", note.State{QMass: 2e9, ARec: 500e6, E: 1e9}, 7002)
	if err != nil {
		return nil, err
	}
	entries := []EntryCase{{"merge-input-a", "actor-1", a, entrycircuit.Assignment(a)}, {"merge-input-b", "actor-1", b, entrycircuit.Assignment(b)}}
	tree := merkle.New()
	for _, v := range entries {
		if _, _, err = tree.Append(v.Note.Commitment); err != nil {
			return nil, err
		}
	}
	pa, _ := tree.Path(0)
	pb, _ := tree.Path(1)
	mergedState, err := allocation.Add(a.State, b.State)
	if err != nil {
		return nil, err
	}
	merged, err := makeNote("M4 Merged Material", "M4-M-001", mergedState, 8001)
	if err != nil {
		return nil, err
	}
	m := MergeCase{Inputs: [2]note.Note{a, b}, Output: merged, Assignment: mergecircuit.Assignment(a, b, owner.SKOwner, tree.Root(), pa, pb, merged)}
	if _, _, err = tree.Append(merged.Commitment); err != nil {
		return nil, err
	}
	ps, _ := tree.Path(2)
	first, second, rem, err := allocation.ByMass(merged.State, 2e9)
	if err != nil {
		return nil, err
	}
	out1, err := makeNote("M4 Split Material 1", "M4-S-001", first, 8002)
	if err != nil {
		return nil, err
	}
	out2, err := makeNote("M4 Split Material 2", "M4-S-002", second, 8003)
	if err != nil {
		return nil, err
	}
	split := SplitCase{Input: merged, Outputs: [2]note.Note{out1, out2}, Remainder: rem, Assignment: splitcircuit.Assignment(merged, owner.SKOwner, tree.Root(), ps, out1, out2, rem)}
	_, _, _ = tree.Append(out1.Commitment)
	_, _, _ = tree.Append(out2.Commitment)
	s := &Scenario{Entries: entries, Merge: m, Split: split, FinalRoot: tree.Root(), FinalCount: tree.Count()}
	for i := uint64(0); i < tree.Count(); i++ {
		p, _ := tree.Path(i)
		s.FinalPaths = append(s.FinalPaths, p)
	}
	return s, nil
}
