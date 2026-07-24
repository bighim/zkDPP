package proofspec

import (
	entrycircuit "github.com/bighim/zkDPP/poc-v2/circuits/entry"
	exitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/exit"
	mergecircuit "github.com/bighim/zkDPP/poc-v2/circuits/merge"
	proceedcircuit "github.com/bighim/zkDPP/poc-v2/circuits/proceed"
	processcircuit "github.com/bighim/zkDPP/poc-v2/circuits/process"
	recallcircuit "github.com/bighim/zkDPP/poc-v2/circuits/recall"
	splitcircuit "github.com/bighim/zkDPP/poc-v2/circuits/split"
	transfercircuit "github.com/bighim/zkDPP/poc-v2/circuits/transfer"
	"github.com/bighim/zkDPP/poc-v2/internal/assignments"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

type Case struct {
	Name         string
	Actor        string
	Assignment   frontend.Circuit
	PublicInputs []fr.Element
}

type Relation struct {
	Name    string
	Circuit frontend.Circuit
	Cases   []Case
}

type Event struct {
	Relation string
	Case     string
	Actor    string
}

var CanonicalEvents = []Event{
	{Relation: "entry", Case: "aluminum-first", Actor: "AluminumSupplier"},
	{Relation: "entry", Case: "aluminum-second", Actor: "AluminumSupplier"},
	{Relation: "entry", Case: "cathode", Actor: "CathodeSupplier"},
	{Relation: "entry", Case: "anode", Actor: "AnodeSupplier"},
	{Relation: "transfer", Case: "aluminum-initial", Actor: "AluminumSupplier"},
	{Relation: "recall", Case: "aluminum-initial", Actor: "AluminumSupplier"},
	{Relation: "transfer", Case: "aluminum-retry", Actor: "AluminumSupplier"},
	{Relation: "proceed", Case: "aluminum-retry", Actor: "FoilManufacturer"},
	{Relation: "transfer", Case: "aluminum-second", Actor: "AluminumSupplier"},
	{Relation: "proceed", Case: "aluminum-second", Actor: "FoilManufacturer"},
	{Relation: "process", Case: "foil-first", Actor: "FoilManufacturer"},
	{Relation: "process", Case: "foil-second", Actor: "FoilManufacturer"},
	{Relation: "transfer", Case: "foil-first", Actor: "FoilManufacturer"},
	{Relation: "proceed", Case: "foil-first", Actor: "CellManufacturer"},
	{Relation: "transfer", Case: "foil-second", Actor: "FoilManufacturer"},
	{Relation: "proceed", Case: "foil-second", Actor: "CellManufacturer"},
	{Relation: "merge", Case: "foil", Actor: "CellManufacturer"},
	{Relation: "transfer", Case: "cathode", Actor: "CathodeSupplier"},
	{Relation: "proceed", Case: "cathode", Actor: "CellManufacturer"},
	{Relation: "transfer", Case: "anode", Actor: "AnodeSupplier"},
	{Relation: "proceed", Case: "anode", Actor: "CellManufacturer"},
	{Relation: "process", Case: "cell", Actor: "CellManufacturer"},
	{Relation: "split", Case: "cell", Actor: "CellManufacturer"},
	{Relation: "exit", Case: "cell", Actor: "CellManufacturer"},
	{Relation: "exit", Case: "waste", Actor: "CellManufacturer"},
}

func Relations(d *scenario.Derived, a *assignments.Canonical) []Relation {
	entries := make([]Case, 0, len(d.EntryOrder))
	for _, name := range d.EntryOrder {
		assignment := a.Entries[name]
		entries = append(entries, Case{Name: name, Actor: actorForOwner(d.Notes[name].Owner, d), Assignment: &assignment, PublicInputs: []fr.Element{d.Notes[name].Commitment}})
	}

	transfers := make([]Case, 0, 7)
	for _, name := range []string{"aluminum-initial", "aluminum-retry", "aluminum-second", "foil-first", "foil-second", "cathode", "anode"} {
		assignment := a.Transfers[name]
		transfers = append(transfers, Case{Name: name, Actor: actorName(d.Transfers[name].Sender), Assignment: &assignment, PublicInputs: Elements(assignment.Root, assignment.InputCommitment, assignment.Nullifier, assignment.VoucherCommitment, assignment.ChangeCommitment, assignment.DeltaEpoch)})
	}

	proceeds := make([]Case, 0, 6)
	for _, name := range []string{"aluminum-retry", "aluminum-second", "foil-first", "foil-second", "cathode", "anode"} {
		assignment := a.Proceeds[name]
		proceeds = append(proceeds, Case{Name: name, Actor: actorName(d.Proceeds[name].Owner), Assignment: &assignment, PublicInputs: Elements(assignment.VoucherRoot, assignment.VoucherCommitment, assignment.VoucherNullifier, assignment.OutputCommitment)})
	}

	processes := make([]Case, 0, 3)
	for _, name := range []string{"foil-first", "foil-second", "cell"} {
		assignment := a.Processes[name]
		processes = append(processes, Case{Name: name, Actor: actorName(d.Processes[name].Owner), Assignment: &assignment, PublicInputs: ProcessPublic(assignment)})
	}

	exits := make([]Case, 0, 2)
	for _, name := range []string{"cell", "waste"} {
		assignment := a.Exits[name]
		exits = append(exits, Case{Name: name, Actor: actorName(d.Exits[name].Owner), Assignment: &assignment, PublicInputs: Elements(assignment.Root, assignment.Commitment, assignment.Nullifier)})
	}

	recall := a.Recalls["aluminum-initial"]
	merge := a.Merges["foil"]
	split := a.Splits["cell"]
	return []Relation{
		{Name: "entry", Circuit: &entrycircuit.Circuit{}, Cases: entries},
		{Name: "transfer", Circuit: &transfercircuit.Circuit{}, Cases: transfers},
		{Name: "proceed", Circuit: &proceedcircuit.Circuit{}, Cases: proceeds},
		{Name: "recall", Circuit: &recallcircuit.Circuit{}, Cases: []Case{{Name: "aluminum-initial", Actor: "AluminumSupplier", Assignment: &recall, PublicInputs: Elements(recall.VoucherRoot, recall.VoucherCommitment, recall.VoucherNullifier, recall.OutputCommitment)}}},
		{Name: "merge", Circuit: &mergecircuit.Circuit{}, Cases: []Case{{Name: "foil", Actor: "CellManufacturer", Assignment: &merge, PublicInputs: Elements(merge.Root1, merge.Root2, merge.Commitment1, merge.Commitment2, merge.Nullifier1, merge.Nullifier2, merge.OutputCommitment)}}},
		{Name: "split", Circuit: &splitcircuit.Circuit{}, Cases: []Case{{Name: "cell", Actor: "CellManufacturer", Assignment: &split, PublicInputs: Elements(split.Root, split.InputCommitment, split.Nullifier, split.OutputCommitment1, split.OutputCommitment2)}}},
		{Name: "process", Circuit: &processcircuit.Circuit{}, Cases: processes},
		{Name: "exit", Circuit: &exitcircuit.Circuit{}, Cases: exits},
	}
}

func CaseIndex(relations []Relation) map[string]Case {
	result := make(map[string]Case, len(CanonicalEvents))
	for _, relation := range relations {
		for _, item := range relation.Cases {
			result[relation.Name+"/"+item.Name] = item
		}
	}
	return result
}

func ProcessPublic(c processcircuit.Circuit) []fr.Element {
	values := Elements(c.M, c.N)
	for _, value := range c.ProcessDelta {
		values = append(values, ToElement(value))
	}
	for j := 0; j < 3; j++ {
		for k := 0; k < 3; k++ {
			values = append(values, ToElement(c.Allocation[j][k]))
		}
	}
	for i := 0; i < 3; i++ {
		values = append(values, ToElement(c.PublicInputs[i].Root), ToElement(c.PublicInputs[i].Commitment), ToElement(c.PublicInputs[i].Nullifier))
	}
	for _, value := range c.OutputCommitments {
		values = append(values, ToElement(value))
	}
	return values
}

func Elements(values ...frontend.Variable) []fr.Element {
	result := make([]fr.Element, len(values))
	for i, value := range values {
		result[i] = ToElement(value)
	}
	return result
}
func ToElement(value frontend.Variable) fr.Element {
	switch value := value.(type) {
	case fr.Element:
		return value
	case uint64:
		return protocol.Element(value)
	case int:
		return protocol.Element(uint64(value))
	default:
		var result fr.Element
		result.SetInterface(value)
		return result
	}
}
func FieldStrings(values []fr.Element) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = protocol.FieldDecimal(value)
	}
	return result
}
func EventIndex(name string) int {
	for i, event := range protocol.EventNames {
		if event == name {
			return i
		}
	}
	panic(name)
}
func actorName(owner string) string {
	switch owner {
	case "aluminum-supplier":
		return "AluminumSupplier"
	case "cathode-supplier":
		return "CathodeSupplier"
	case "anode-supplier":
		return "AnodeSupplier"
	case "foil-manufacturer":
		return "FoilManufacturer"
	case "cell-manufacturer":
		return "CellManufacturer"
	default:
		panic(owner)
	}
}
func actorForOwner(owner protocol.Owner, d *scenario.Derived) string {
	for name, candidate := range d.Owners {
		if candidate.Address.Equal(&owner.Address) {
			return actorName(name)
		}
	}
	panic("owner")
}
