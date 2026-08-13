package events

import (
	"strings"
	"testing"

	"github.com/bighim/zkDPP/sahai-poc/internal/conformance"
	"github.com/bighim/zkDPP/sahai-poc/internal/document"
)

func TestFixtureProofCountsMatchConformance(t *testing.T) {
	fixtures, err := BuildIndependent(document.Poseidon2)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := conformance.Load("../../config/events.json")
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]conformance.Event)
	for _, event := range manifest.Events {
		byName[event.Name] = event
	}
	for _, fixture := range fixtures {
		counts := map[string]int{}
		for _, assignment := range fixture.Assignments {
			counts[assignment.Gadget]++
		}
		want := byName[fixture.Name].Proofs
		if counts["merklepath"] != want.MerklePath || counts["eq"] != want.Eq || counts["add"] != want.Add || counts["and"] != want.And {
			t.Fatalf("%s counts=%v want=%+v", fixture.Name, counts, want)
		}
		if len(fixture.Outputs) != byName[fixture.Name].Outputs || len(fixture.Inputs) != byName[fixture.Name].Inputs {
			t.Fatalf("%s arity mismatch", fixture.Name)
		}
	}
}

func TestEventProofBindingsAreDistinct(t *testing.T) {
	fixtures, err := BuildIndependent(document.Poseidon2)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, fixture := range fixtures {
		for _, assignment := range fixture.Assignments {
			parts := make([]string, len(assignment.PublicInputs))
			for i, value := range assignment.PublicInputs {
				parts[i] = value.String()
			}
			key := assignment.Name + ":" + strings.Join(parts, ",")
			if previous, ok := seen[key]; ok {
				t.Fatalf("%s proof binding reused by %s and %s", assignment.Name, previous, fixture.Name)
			}
			seen[key] = fixture.Name
		}
	}
}

func TestCanonicalScenarioIsConnectedAndUnique(t *testing.T) {
	fixtures, err := BuildCanonical(document.Poseidon2)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 8 {
		t.Fatalf("got %d transactions", len(fixtures))
	}
	produced := map[string]bool{}
	for _, fixture := range fixtures {
		for _, input := range fixture.Inputs {
			if !produced[input.ID()] {
				t.Fatalf("%s input %s was not produced earlier", fixture.Case, input.ID())
			}
		}
		for _, output := range fixture.Outputs {
			if produced[output.ID()] {
				t.Fatalf("duplicate output %s", output.ID())
			}
			produced[output.ID()] = true
		}
	}
}
