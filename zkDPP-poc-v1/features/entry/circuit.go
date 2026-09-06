package entry

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	corenote "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Commitment frontend.Variable `gnark:",public"`
	Note       circuitutil.NoteWitness
}

func (c *Circuit) Define(api frontend.API) error {
	circuitutil.AssertNote(api, c.Note)
	api.AssertIsDifferent(c.Note.QMass, 0)
	api.AssertIsEqual(c.Note.AssetRole, uint64(corenote.AssetRoleEligible))
	commitment, err := circuitutil.Commitment(api, c.Note)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Commitment, commitment)
	return nil
}

func Assignment(value corenote.Note) *Circuit {
	return &Circuit{
		Commitment: value.Commitment,
		Note: circuitutil.NoteWitness{
			DocumentHash: value.DocumentHash,
			AssetRole:    uint64(value.AssetRole),
			QMass:        value.State.QMass,
			ARec:         value.State.ARec,
			E:            value.State.E,
			Address:      value.Address,
			Opening:      value.Opening,
		},
	}
}
