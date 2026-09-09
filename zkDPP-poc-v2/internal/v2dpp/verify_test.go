package v2dpp

import (
	"context"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type source struct {
	h, policy fr.Element
	record    v2audit.Record
}

func (s source) CheckSnapshot(context.Context, v2audit.Snapshot) error                { return nil }
func (s source) CheckIssuePolicy(context.Context, fr.Element, v2audit.Snapshot) error { return nil }

func (s source) ClaimRegistered(context.Context, fr.Element, v2audit.Snapshot) (bool, error) {
	return true, nil
}
func (s source) Producer(context.Context, v2audit.Ref, v2audit.Snapshot) (uint64, error) {
	return 1, nil
}
func (s source) Record(context.Context, uint64, v2audit.Snapshot) (v2audit.Record, error) {
	return s.record, nil
}

func TestVerifyExternalDPPClaim(t *testing.T) {
	policy := zkhash.Element(81)
	nonce := zkhash.Element(91)
	doc, err := document.Hash(document.DocumentInfo{ProductName: "Battery", LotID: "LOT-1", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	h := claim.Handle(doc, policy, nonce)
	record := v2audit.Record{EventKind: v2audit.Issue, PolicyRef: policy, OutputRefs: []v2audit.Ref{{ObjectType: v2audit.Claim, RawID: h}}}
	dpp := DPP{ProductName: "Battery", LotID: "LOT-1", Unit: "kg", Claim: claim.DPPClaim{IssuePolicyRef: policy, Handle: h, ClaimNonce: nonce}}
	if err = Verify(context.Background(), source{h, policy, record}, v2audit.Snapshot{BlockNumber: 1, BlockHash: "0x1"}, dpp); err != nil {
		t.Fatal(err)
	}
	dpp.Unit = "t"
	if err = Verify(context.Background(), source{h, policy, record}, v2audit.Snapshot{}, dpp); err == nil {
		t.Fatal("wrong Unit accepted")
	}
}
