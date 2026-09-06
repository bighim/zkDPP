package statusprivatespend

import (
	privatespend "github.com/bighim/zkDPP/zkDPP-poc-v1/features/private_spend"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	NoteRoot       frontend.Variable `gnark:",public"`
	NoteStatusRoot frontend.Variable `gnark:",public"`
	NF             frontend.Variable `gnark:",public"`
	SKOwner        frontend.Variable
	Note           circuitutil.NoteWitness
	Path           circuitutil.MerklePath
	StatusPath     circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	base := privatespend.Circuit{NoteRoot: c.NoteRoot, NF: c.NF, SKOwner: c.SKOwner, Note: c.Note, Path: c.Path}
	if err := base.Define(api); err != nil {
		return err
	}
	return circuitutil.AssertStatusActive(api, c.NoteStatusRoot, c.Path.Index, c.StatusPath)
}

func FromBase(base *privatespend.Circuit, root fr.Element, path status.Path) *Circuit {
	c := &Circuit{NoteRoot: base.NoteRoot, NoteStatusRoot: root, NF: base.NF, SKOwner: base.SKOwner, Note: base.Note, Path: base.Path}
	copyStatus(&c.StatusPath, path)
	return c
}

func copyStatus(out *circuitutil.StatusPath, path status.Path) {
	for i := range path.Siblings {
		out.Siblings[i] = path.Siblings[i]
	}
}
