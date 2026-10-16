package mergesame

import (
	"fmt"
	mergebase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/merge"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2circuit"
	"github.com/consensys/gnark/frontend"
)

// Transaction retains every current base and v2 Merge condition. No private
// parent commitment, amount, or additional linkage value becomes public.
type Transaction struct{ Base mergebase.Circuit }

func (c *Transaction) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	api.AssertIsEqual(c.Base.Inputs[0].AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Base.Inputs[1].AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Base.Inputs[0].DocumentHash, c.Base.Inputs[1].DocumentHash)
	api.AssertIsEqual(c.Base.Output.DocumentHash, c.Base.Inputs[0].DocumentHash)
	return nil
}

// AuditLink recomputes BOTH input NF bindings. Joint extraction plus existing
// collision resistance binds their hidden (CM,SK) pairs to Transaction's pairs.
// Commitment binding then binds the complete hidden Note fields. It does not
// assert algebraic uniqueness of preimages or duplicate membership/accounting.
type AuditLink struct {
	NF1         frontend.Variable `gnark:",public"`
	NF2         frontend.Variable `gnark:",public"`
	CMOut       frontend.Variable `gnark:",public"`
	Audit       v2circuit.AuditWitness
	Inputs      [2]circuitutil.NoteWitness
	SKOwner     frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func (c *AuditLink) Define(api frontend.API) error {
	parents := make([]frontend.Variable, 2)
	for i := range c.Inputs {
		cm, e := circuitutil.Commitment(api, c.Inputs[i])
		if e != nil {
			return e
		}
		parents[i] = cm
		nf, e := circuitutil.Nullifier(api, c.SKOwner, cm)
		if e != nil {
			return e
		}
		expected := c.NF1
		if i == 1 {
			expected = c.NF2
		}
		api.AssertIsEqual(nf, expected)
	}
	future, e := circuitutil.Nullifier(api, c.SKOwner, c.CMOut)
	if e != nil {
		return e
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, parents, []frontend.Variable{future}, c.Audit)
}
func Definition(name string, pk auditcrypto.PublicKey) frontend.Circuit {
	switch name {
	case "T":
		return &Transaction{}
	case "A":
		return &AuditLink{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(2, 1)}
	case "U":
		return v2events.NewMerge(pk)
	}
	panic(fmt.Sprintf("unknown definition %s", name))
}
func Split(c *v2events.MergeCircuit) [2]frontend.Circuit {
	return [2]frontend.Circuit{&Transaction{Base: c.Base}, &AuditLink{NF1: c.Base.NF1, NF2: c.Base.NF2, CMOut: c.Base.CMOut, Audit: c.Audit, Inputs: c.Base.Inputs, SKOwner: c.Base.SKOwner, CommitteePK: c.CommitteePK}}
}

// Both module public witnesses are derived from the single canonical submission.
func PublicSplit(c *v2events.MergeCircuit) [2]frontend.Circuit {
	t := &Transaction{}
	t.Base.NoteRoot = c.Base.NoteRoot
	t.Base.NF1 = c.Base.NF1
	t.Base.NF2 = c.Base.NF2
	t.Base.CMOut = c.Base.CMOut
	a := &AuditLink{NF1: c.Base.NF1, NF2: c.Base.NF2, CMOut: c.Base.CMOut, Audit: c.Audit}
	return [2]frontend.Circuit{t, a}
}
