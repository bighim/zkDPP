package assignments_test

import (
	"testing"

	entrycircuit "github.com/bighim/zkDPP/poc-v2/circuits/entry"
	exitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/exit"
	mergecircuit "github.com/bighim/zkDPP/poc-v2/circuits/merge"
	proceedcircuit "github.com/bighim/zkDPP/poc-v2/circuits/proceed"
	processcircuit "github.com/bighim/zkDPP/poc-v2/circuits/process"
	recallcircuit "github.com/bighim/zkDPP/poc-v2/circuits/recall"
	splitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/split"
	transfercircuit "github.com/bighim/zkDPP/poc-v2/circuits/transfer"
	"github.com/bighim/zkDPP/poc-v2/internal/assignments"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

func TestCanonicalAssignmentsSolveAllRelations(t *testing.T) {
	a := load(t)
	cases := []struct {
		name                string
		circuit, assignment frontend.Circuit
	}{
		{"entry", &entrycircuit.Circuit{}, ptr(a.Entries["cathode"])},
		{"transfer", &transfercircuit.Circuit{}, ptr(a.Transfers["aluminum-initial"])},
		{"proceed", &proceedcircuit.Circuit{}, ptr(a.Proceeds["aluminum-retry"])},
		{"recall", &recallcircuit.Circuit{}, ptr(a.Recalls["aluminum-initial"])},
		{"merge", &mergecircuit.Circuit{}, ptr(a.Merges["foil"])},
		{"split", &splitcircuit.Circuit{}, ptr(a.Splits["cell"])},
		{"process-1-to-2", &processcircuit.Circuit{}, ptr(a.Processes["foil-first"])},
		{"process-3-to-3", &processcircuit.Circuit{}, ptr(a.Processes["cell"])},
		{"exit", &exitcircuit.Circuit{}, ptr(a.Exits["waste"])},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := test.IsSolved(tc.circuit, tc.assignment, ecc.BLS12_381.ScalarField()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMutatedPublicStatementsFail(t *testing.T) {
	a := load(t)
	entry := a.Entries["cathode"]
	entry.Commitment = 1
	assertUnsolved(t, "entry", &entrycircuit.Circuit{}, &entry)

	transfer := a.Transfers["aluminum-retry"]
	transfer.DeltaEpoch = 7
	assertUnsolved(t, "transfer", &transfercircuit.Circuit{}, &transfer)

	proceed := a.Proceeds["aluminum-retry"]
	proceed.OutputCommitment = 1
	assertUnsolved(t, "proceed", &proceedcircuit.Circuit{}, &proceed)

	recall := a.Recalls["aluminum-initial"]
	recall.OutputCommitment = 1
	assertUnsolved(t, "recall", &recallcircuit.Circuit{}, &recall)

	merge := a.Merges["foil"]
	merge.OutputCommitment = 1
	assertUnsolved(t, "merge", &mergecircuit.Circuit{}, &merge)

	split := a.Splits["cell"]
	split.Remainder[0] = 9
	assertUnsolved(t, "split", &splitcircuit.Circuit{}, &split)

	process := a.Processes["cell"]
	process.ProcessDelta[1] = 50_000_000_001
	assertUnsolved(t, "process", &processcircuit.Circuit{}, &process)

	exit := a.Exits["waste"]
	exit.Nullifier = 1
	assertUnsolved(t, "exit", &exitcircuit.Circuit{}, &exit)
}

func assertUnsolved(t *testing.T, name string, circuit, assignment frontend.Circuit) {
	t.Helper()
	if err := test.IsSolved(circuit, assignment, ecc.BLS12_381.ScalarField()); err == nil {
		t.Fatalf("mutated %s unexpectedly solved", name)
	}
}

func load(t *testing.T) *assignments.Canonical {
	t.Helper()
	d, err := scenario.LoadCanonical("../..")
	if err != nil {
		t.Fatal(err)
	}
	a, err := assignments.Build(d)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func ptr[T any](value T) *T { return &value }
