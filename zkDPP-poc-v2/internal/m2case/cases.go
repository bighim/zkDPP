package m2case

import (
	"fmt"
	"path/filepath"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	privatespend "github.com/bighim/zkDPP/zkDPP-poc-v2/features/private_spend"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type EntryCase struct {
	Name       string
	ActorID    string
	Note       note.Note
	Assignment *entrycircuit.Circuit
}

type Scenario struct {
	Entries        []EntryCase
	ExitAssignment *privatespend.Circuit
	ExitNote       note.Note
	ExitPath       merkle.Path
	FinalRoot      fr.Element
}

func Build(root string) (*Scenario, error) {
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata", "common", "actors-v1.json"))
	if err != nil {
		return nil, err
	}
	specs := []struct {
		name, actorID, product, lot string
		qMass, aRec, carbon         uint64
		opening                     uint64
	}{
		{"raw-material-a", "actor-1", "Raw Material A", "M2-A-001", 100, 20, 80, 2001},
		{"raw-material-b", "actor-2", "Raw Material B", "M2-B-001", 50, 0, 40, 2002},
		{"raw-material-c", "actor-3", "Raw Material C", "M2-C-001", 30, 10, 25, 2003},
	}
	tree := merkle.New()
	entries := make([]EntryCase, len(specs))
	for i, spec := range specs {
		actor, err := testkit.ByID(actors, spec.actorID)
		if err != nil {
			return nil, err
		}
		documentHash, err := document.Hash(document.DocumentInfo{ProductName: spec.product, LotID: spec.lot})
		if err != nil {
			return nil, err
		}
		value, err := note.New(
			documentHash,
			note.State{QMass: spec.qMass * note.MassScale, ARec: spec.aRec * note.MassScale, E: spec.carbon * note.CarbonScale},
			note.AssetRoleEligible,
			actor.Address,
			zkhash.Element(spec.opening),
		)
		if err != nil {
			return nil, err
		}
		if _, _, err := tree.Append(value.Commitment); err != nil {
			return nil, err
		}
		entries[i] = EntryCase{Name: spec.name, ActorID: spec.actorID, Note: value, Assignment: entrycircuit.Assignment(value)}
	}
	path, err := tree.Path(0)
	if err != nil {
		return nil, err
	}
	actor, err := testkit.ByID(actors, "actor-1")
	if err != nil {
		return nil, err
	}
	exit := privatespend.Assignment(entries[0].Note, actor.SKOwner, tree.Root(), path)
	if exit.NF == nil {
		return nil, fmt.Errorf("exit fixture has no nullifier")
	}
	return &Scenario{Entries: entries, ExitAssignment: exit, ExitNote: entries[0].Note, ExitPath: path, FinalRoot: tree.Root()}, nil
}
