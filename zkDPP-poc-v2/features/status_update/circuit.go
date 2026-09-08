package statusupdate

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	ObjectType frontend.Variable `gnark:",public"`
	OldRoot    frontend.Variable `gnark:",public"`
	NewRoot    frontend.Variable `gnark:",public"`
	Index      frontend.Variable `gnark:",public"`
	OldStatus  frontend.Variable `gnark:",public"`
	NewStatus  frontend.Variable `gnark:",public"`
	Path       circuitutil.StatusPath
}

func (c *Circuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(api.Sub(c.ObjectType, 1), api.Sub(c.ObjectType, 2)), 0)
	oldActive := api.IsZero(c.OldStatus)
	oldFrozen := api.IsZero(api.Sub(c.OldStatus, 1))
	oldRevoked := api.IsZero(api.Sub(c.OldStatus, 2))
	newActive := api.IsZero(c.NewStatus)
	newFrozen := api.IsZero(api.Sub(c.NewStatus, 1))
	newRevoked := api.IsZero(api.Sub(c.NewStatus, 2))
	api.AssertIsEqual(api.Add(oldActive, oldFrozen, oldRevoked), 1)
	api.AssertIsEqual(api.Add(newActive, newFrozen, newRevoked), 1)
	allowed := api.Add(api.Mul(oldActive, newFrozen), api.Mul(oldFrozen, newActive), api.Mul(oldFrozen, newRevoked))
	api.AssertIsEqual(allowed, 1)
	if err := circuitutil.AssertStatusRoot(api, c.OldRoot, c.OldStatus, c.Index, c.Path); err != nil {
		return err
	}
	return circuitutil.AssertStatusRoot(api, c.NewRoot, c.NewStatus, c.Index, c.Path)
}

func Assignment(objectType status.ObjectType, oldRoot, newRoot fr.Element, index uint64, oldStatus, newStatus status.Status, path status.Path) *Circuit {
	c := &Circuit{ObjectType: uint8(objectType), OldRoot: oldRoot, NewRoot: newRoot, Index: index, OldStatus: uint8(oldStatus), NewStatus: uint8(newStatus)}
	for i := range path.Siblings {
		c.Path.Siblings[i] = path.Siblings[i]
	}
	return c
}
