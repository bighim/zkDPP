package events

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/bighim/zkDPP/sahai-poc/circuits/gadgets"
	"github.com/bighim/zkDPP/sahai-poc/internal/document"
)

const (
	Sender = iota
	Recipient
	ItemCode
	Quantity
)

type Asset struct {
	Type     string
	Doc      document.Document
	DocHash  document.Digest
	Terminal bool
}

func (a Asset) ID() string { return "0x" + hex.EncodeToString(a.DocHash[:]) }

type Fixture struct {
	Case        string
	Name        string
	Profile     document.Profile
	Inputs      []Asset
	Outputs     []Asset
	Assignments []gadgets.NamedAssignment
}

type Proof struct {
	Gadget      string   `json:"gadget"`
	Proof       string   `json:"proof"`
	PublicInput []string `json:"publicInputs"`
}

type AssetJSON struct {
	Type     string `json:"type"`
	DocHash  string `json:"docHash"`
	Terminal bool   `json:"terminal"`
}

type Payload struct {
	Case             string        `json:"case"`
	Event            string        `json:"event"`
	HashProfile      string        `json:"hashProfile"`
	Inputs           []AssetJSON   `json:"inputs"`
	Outputs          []AssetJSON   `json:"outputs"`
	InDocs           []string      `json:"inDocs"`
	Proofs           []Proof       `json:"proofs"`
	LogicalBytes     int           `json:"logicalBytes"`
	ABICalldataBytes int           `json:"abiCalldataBytes,omitempty"`
	Timing           PayloadTiming `json:"timing"`
}

type PayloadTiming struct {
	BuildMS   float64 `json:"buildMs"`
	WitnessMS float64 `json:"witnessMs"`
	ProveMS   float64 `json:"proveMs"`
	EncodeMS  float64 `json:"encodeMs"`
}

func MakePayload(fixture Fixture, proofs []Proof, timing PayloadTiming) (Payload, error) {
	payload := Payload{Case: fixture.Case, Event: fixture.Name, HashProfile: string(fixture.Profile), Proofs: proofs, Timing: timing}
	for _, value := range fixture.Inputs {
		payload.Inputs = append(payload.Inputs, assetJSON(value))
		payload.InDocs = append(payload.InDocs, value.ID())
	}
	for _, value := range fixture.Outputs {
		payload.Outputs = append(payload.Outputs, assetJSON(value))
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Payload{}, err
	}
	payload.LogicalBytes = len(raw)
	return payload, nil
}

func assetJSON(value Asset) AssetJSON {
	return AssetJSON{Type: value.Type, DocHash: value.ID(), Terminal: value.Terminal}
}

func ParseID(value string) ([32]byte, error) {
	var out [32]byte
	raw, err := hex.DecodeString(trim0x(value))
	if err != nil || len(raw) != 32 {
		return out, fmt.Errorf("invalid document ID %q", value)
	}
	copy(out[:], raw)
	return out, nil
}

func trim0x(value string) string {
	if len(value) >= 2 && value[:2] == "0x" {
		return value[2:]
	}
	return value
}
