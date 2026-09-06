package privatespend

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	corenote "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot frontend.Variable `gnark:",public"`
	NF       frontend.Variable `gnark:",public"`

	SKOwner frontend.Variable
	Note    circuitutil.NoteWitness
	Path    circuitutil.MerklePath
}

func (c *Circuit) Define(api frontend.API) error {
	circuitutil.AssertNote(api, c.Note)
	address, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Note.Address, address)
	commitment, err := circuitutil.Commitment(api, c.Note)
	if err != nil {
		return err
	}
	if err := circuitutil.AssertMembership(api, c.NoteRoot, commitment, c.Path); err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.SKOwner, commitment)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.NF, nf)
	return nil
}

func Assignment(value corenote.Note, skOwner, root fr.Element, path merkle.Path) *Circuit {
	assignment := &Circuit{
		NoteRoot: root,
		NF:       corenote.Nullifier(skOwner, value.Commitment),
		SKOwner:  skOwner,
		Note: circuitutil.NoteWitness{
			DocumentHash: value.DocumentHash,
			AssetRole:    uint64(value.AssetRole),
			QMass:        value.State.QMass,
			ARec:         value.State.ARec,
			E:            value.State.E,
			Address:      value.Address,
			Opening:      value.Opening,
		},
		Path: circuitutil.MerklePath{Index: path.Index},
	}
	for i := range path.Siblings {
		assignment.Path.Siblings[i] = path.Siblings[i]
	}
	return assignment
}
