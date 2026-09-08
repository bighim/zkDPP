package v2dpp

import (
	"context"
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type DPP struct {
	ProductName string
	LotID       string
	Unit        string
	Claim       claim.DPPClaim
}

type Source interface {
	ClaimRegistered(context.Context, fr.Element, v2audit.Snapshot) (bool, error)
	Producer(context.Context, v2audit.Ref, v2audit.Snapshot) (uint64, error)
	Record(context.Context, uint64, v2audit.Snapshot) (v2audit.Record, error)
}

// Verify proves only that the disclosed DPP fields recompute a registered
// terminal Claim. Product data remains outside Contract storage.
func Verify(ctx context.Context, source Source, snapshot v2audit.Snapshot, dpp DPP) error {
	documentHash, err := document.Hash(document.DocumentInfo{ProductName: dpp.ProductName, LotID: dpp.LotID, Unit: dpp.Unit})
	if err != nil {
		return err
	}
	h := claim.Handle(documentHash, dpp.Claim.IssuePolicyRef, dpp.Claim.ClaimNonce)
	if !h.Equal(&dpp.Claim.Handle) {
		return fmt.Errorf("DPP Claim handle mismatch")
	}
	registered, err := source.ClaimRegistered(ctx, h, snapshot)
	if err != nil {
		return err
	}
	if !registered {
		return fmt.Errorf("Claim is not registered")
	}
	id, err := source.Producer(ctx, v2audit.Ref{ObjectType: v2audit.Claim, RawID: h}, snapshot)
	if err != nil {
		return err
	}
	if id == 0 {
		return fmt.Errorf("Claim producer missing")
	}
	record, err := source.Record(ctx, id, snapshot)
	if err != nil {
		return err
	}
	if record.EventKind != v2audit.Issue || !record.PolicyRef.Equal(&dpp.Claim.IssuePolicyRef) || len(record.OutputRefs) != 1 || record.OutputRefs[0].ObjectType != v2audit.Claim || !record.OutputRefs[0].RawID.Equal(&h) {
		return fmt.Errorf("Issue AuditRecord mismatch")
	}
	return nil
}
