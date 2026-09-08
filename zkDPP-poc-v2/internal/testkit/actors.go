package testkit

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	ActorProfile = "zkDPP-fixed-actors-v1"
	ActorPurpose = "local-test-only"
	ActorCurve   = "BLS12-381"
	ActorHash    = "Poseidon2-Merkle-Damgard"
)

type Actor struct {
	ID      string
	SKOwner fr.Element
	Address fr.Element
}

type actorFile struct {
	Profile  string `json:"profile"`
	Purpose  string `json:"purpose"`
	Curve    string `json:"curve"`
	Hash     string `json:"hash"`
	OwnerTag string `json:"ownerTag"`
	Actors   []struct {
		ID      string `json:"id"`
		SKOwner string `json:"skOwner"`
		Address string `json:"address"`
	} `json:"actors"`
}

func LoadActors(path string) ([]Actor, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open actor fixture: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var raw actorFile
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode actor fixture: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return nil, err
	}
	if raw.Profile != ActorProfile || raw.Purpose != ActorPurpose || raw.Curve != ActorCurve || raw.Hash != ActorHash || raw.OwnerTag != zkhash.OwnerTagString {
		return nil, fmt.Errorf("actor fixture profile metadata mismatch")
	}
	if len(raw.Actors) == 0 {
		return nil, fmt.Errorf("actor fixture is empty")
	}
	ids := map[string]bool{}
	secrets := map[[32]byte]bool{}
	addresses := map[[32]byte]bool{}
	out := make([]Actor, len(raw.Actors))
	for i, item := range raw.Actors {
		if item.ID == "" || ids[item.ID] {
			return nil, fmt.Errorf("actor %d has empty or duplicate id %q", i, item.ID)
		}
		ids[item.ID] = true
		sk, err := parseCanonicalField(item.SKOwner)
		if err != nil || sk.IsZero() {
			return nil, fmt.Errorf("actor %q has invalid skOwner", item.ID)
		}
		address, err := parseCanonicalField(item.Address)
		if err != nil {
			return nil, fmt.Errorf("actor %q has invalid address", item.ID)
		}
		skKey, addressKey := sk.Bytes(), address.Bytes()
		if secrets[skKey] || addresses[addressKey] {
			return nil, fmt.Errorf("actor %q duplicates skOwner or address", item.ID)
		}
		secrets[skKey], addresses[addressKey] = true, true
		derived := owner.Address(sk)
		if !derived.Equal(&address) {
			return nil, fmt.Errorf("actor %q address does not match skOwner", item.ID)
		}
		out[i] = Actor{ID: item.ID, SKOwner: sk, Address: address}
	}
	return out, nil
}

func ByID(actors []Actor, id string) (Actor, error) {
	for _, actor := range actors {
		if actor.ID == id {
			return actor, nil
		}
	}
	return Actor{}, fmt.Errorf("unknown actor %q", id)
}

func parseCanonicalField(value string) (fr.Element, error) {
	integer, ok := new(big.Int).SetString(value, 10)
	if !ok || integer.Sign() < 0 || integer.Cmp(fr.Modulus()) >= 0 {
		return fr.Element{}, fmt.Errorf("non-canonical field value %q", value)
	}
	var out fr.Element
	out.SetBigInt(integer)
	return out, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("actor fixture contains multiple JSON values")
		}
		return fmt.Errorf("decode actor fixture trailer: %w", err)
	}
	return nil
}
