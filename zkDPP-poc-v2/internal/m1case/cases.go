package m1case

import (
	"fmt"
	"path/filepath"

	notecommitment "github.com/bighim/zkDPP/zkDPP-poc-v2/features/note_commitment"
	privatespend "github.com/bighim/zkDPP/zkDPP-poc-v2/features/private_spend"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark/frontend"
)

type Case struct {
	Name           string
	Circuit        frontend.Circuit
	Assignment     frontend.Circuit
	ExpectedPublic int
}

func All(root string) ([]Case, error) {
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata", "common", "actors-v1.json"))
	if err != nil {
		return nil, err
	}
	first, err := testkit.ByID(actors, "actor-1")
	if err != nil {
		return nil, err
	}
	second, err := testkit.ByID(actors, "actor-2")
	if err != nil {
		return nil, err
	}
	documentHash, err := document.Hash(document.DocumentInfo{ProductName: "Aluminum", LotID: "LOT-M1-001"})
	if err != nil {
		return nil, err
	}
	value, err := note.New(
		documentHash,
		note.State{QMass: 100 * note.MassScale, ARec: 20 * note.MassScale, E: 30 * note.CarbonScale},
		note.AssetRoleEligible,
		first.Address,
		zkhash.Element(1001),
	)
	if err != nil {
		return nil, err
	}
	other, err := note.New(
		zkhash.Element(202),
		note.State{QMass: note.MassScale},
		note.AssetRoleEligible,
		second.Address,
		zkhash.Element(1002),
	)
	if err != nil {
		return nil, err
	}
	tree := merkle.New()
	if _, _, err := tree.Append(value.Commitment); err != nil {
		return nil, err
	}
	if _, _, err := tree.Append(other.Commitment); err != nil {
		return nil, err
	}
	path, err := tree.Path(0)
	if err != nil {
		return nil, err
	}
	spend := privatespend.Assignment(value, first.SKOwner, tree.Root(), path)
	if spend.NF == nil {
		return nil, fmt.Errorf("private-spend fixture has no nullifier")
	}
	return []Case{
		{Name: "note-commitment", Circuit: &notecommitment.Circuit{}, Assignment: notecommitment.Assignment(value), ExpectedPublic: 1},
		{Name: "private-spend", Circuit: &privatespend.Circuit{}, Assignment: spend, ExpectedPublic: 2},
	}, nil
}
