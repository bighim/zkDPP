package owner

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Owner struct {
	SKOwner fr.Element
	Address fr.Element
}

func Address(skOwner fr.Element) fr.Element {
	return zkhash.Hash(zkhash.OwnerTag, skOwner)
}

func FromSecret(skOwner fr.Element) (Owner, error) {
	if skOwner.IsZero() {
		return Owner{}, fmt.Errorf("sk_owner must not be zero")
	}
	return Owner{SKOwner: skOwner, Address: Address(skOwner)}, nil
}
