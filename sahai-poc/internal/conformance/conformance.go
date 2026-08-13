package conformance

import (
	"encoding/json"
	"fmt"
	"os"
)

type Counts struct {
	MerklePath int `json:"merklePath"`
	Eq         int `json:"eq"`
	Add        int `json:"add"`
	And        int `json:"and"`
}

type Event struct {
	Name           string `json:"name"`
	Inputs         int    `json:"inputs"`
	Outputs        int    `json:"outputs"`
	TerminalOutput bool   `json:"terminalOutput"`
	Proofs         Counts `json:"proofs"`
}

type Merkle struct {
	Attributes          int `json:"attributes"`
	AttributeBits       int `json:"attributeBits"`
	AttributesPerLeaf   int `json:"attributesPerLeaf"`
	Leaves              int `json:"leaves"`
	Siblings            int `json:"siblings"`
	LevelsIncludingLeaf int `json:"levelsIncludingLeaf"`
}

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	Merkle        Merkle  `json:"merkle"`
	Events        []Event `json:"events"`
}

var expected = map[string]Event{
	"Entry":   {Name: "Entry", Inputs: 0, Outputs: 1, Proofs: Counts{MerklePath: 1, Eq: 1}},
	"Ship":    {Name: "Ship", Inputs: 1, Outputs: 1, Proofs: Counts{MerklePath: 2, Eq: 3}},
	"Merge":   {Name: "Merge", Inputs: 2, Outputs: 1, Proofs: Counts{MerklePath: 3, Eq: 2, Add: 1}},
	"Split":   {Name: "Split", Inputs: 1, Outputs: 2, Proofs: Counts{MerklePath: 3, Eq: 2, Add: 1}},
	"Process": {Name: "Process", Inputs: 2, Outputs: 1, Proofs: Counts{MerklePath: 3, Eq: 2, Add: 1}},
	"Exit":    {Name: "Exit", Inputs: 1, Outputs: 1, TerminalOutput: true, Proofs: Counts{MerklePath: 1, Eq: 1}},
}

func Load(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Validate(manifest Manifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("schema version: got %d, want 1", manifest.SchemaVersion)
	}
	if manifest.Merkle.Attributes != 32 || manifest.Merkle.AttributeBits != 128 ||
		manifest.Merkle.AttributesPerLeaf != 4 || manifest.Merkle.Leaves != 8 ||
		manifest.Merkle.Siblings != 3 || manifest.Merkle.LevelsIncludingLeaf != 4 {
		return fmt.Errorf("unexpected Merkle profile: %+v", manifest.Merkle)
	}
	if len(manifest.Events) != len(expected) {
		return fmt.Errorf("event count: got %d, want %d", len(manifest.Events), len(expected))
	}
	seen := make(map[string]bool, len(manifest.Events))
	for _, event := range manifest.Events {
		want, ok := expected[event.Name]
		if !ok {
			return fmt.Errorf("unexpected event %q", event.Name)
		}
		if seen[event.Name] {
			return fmt.Errorf("duplicate event %q", event.Name)
		}
		seen[event.Name] = true
		if event.Inputs != want.Inputs || event.Outputs != want.Outputs ||
			event.TerminalOutput != want.TerminalOutput || event.Proofs != want.Proofs {
			return fmt.Errorf("event %s mismatch: got %+v, want %+v", event.Name, event, want)
		}
	}
	return nil
}
