package claim

import (
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type DPPClaim struct {
	IssuePolicyRef fr.Element `json:"issuePolicyRef"`
	Handle         fr.Element `json:"h"`
	ClaimNonce     fr.Element `json:"claimNonce"`
}

func Handle(documentHash, issuePolicyRef, claimNonce fr.Element) fr.Element {
	return zkhash.Hash(zkhash.IssueTag, documentHash, issuePolicyRef, claimNonce)
}
