package events

import (
	"math/big"

	"github.com/bighim/zkDPP/sahai-poc/circuits/gadgets"
	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/consensys/gnark/frontend"
)

type fixtureBuilder struct {
	profile document.Profile
	serial  uint64
}

func BuildIndependent(profile document.Profile) ([]Fixture, error) {
	b := &fixtureBuilder{profile: profile}
	constructors := []func() (Fixture, error){b.entry, b.ship, b.merge, b.split, b.process, b.exit}
	out := make([]Fixture, 0, len(constructors))
	for _, constructor := range constructors {
		value, err := constructor()
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

// BuildCanonical creates the connected eight-step benchmark scenario. Entry and Ship each have two cases.
func BuildCanonical(profile document.Profile) ([]Fixture, error) {
	b := &fixtureBuilder{profile: profile}
	entryA := b.asset("Entry", 11, 11, 101, 100, false)
	shipA := b.asset("Ship", 11, 22, 101, 100, false)
	splitA := b.asset("Split", 22, 22, 101, 40, false)
	splitB := b.asset("Split", 22, 22, 101, 60, false)
	merged := b.asset("Merge", 22, 33, 101, 100, false)
	entryB := b.asset("Entry", 44, 44, 202, 50, false)
	shipB := b.asset("Ship", 44, 33, 202, 50, false)
	product := b.asset("Process", 33, 55, 999, 150, false)
	exit := b.asset("Exit", 55, 55, 999, 150, true)
	fixtures := []Fixture{
		b.entryFixture("entry_a", entryA),
		b.shipFixture("ship_a", entryA, shipA),
		b.splitFixture("split_a", shipA, splitA, splitB),
		b.mergeFixture("merge_a", "Merge", splitA, splitB, merged),
		b.entryFixture("entry_b", entryB),
		b.shipFixture("ship_b", entryB, shipB),
		b.mergeFixture("process_product", "Process", merged, shipB, product),
		b.exitFixture("exit_product", product, exit),
	}
	return fixtures, nil
}

func (b *fixtureBuilder) entry() (Fixture, error) {
	output := b.asset("Entry", 11, 11, 100, 40, false)
	return b.entryFixture("entry", output), nil
}
func (b *fixtureBuilder) ship() (Fixture, error) {
	input := b.asset("Input", 10, 20, 101, 50, false)
	output := b.asset("Ship", 20, 30, 101, 50, false)
	return b.shipFixture("ship", input, output), nil
}
func (b *fixtureBuilder) merge() (Fixture, error) {
	a := b.asset("Input", 10, 30, 201, 40, false)
	c := b.asset("Input", 20, 30, 202, 60, false)
	o := b.asset("Merge", 30, 40, 999, 100, false)
	return b.mergeFixture("merge", "Merge", a, c, o), nil
}
func (b *fixtureBuilder) split() (Fixture, error) {
	i := b.asset("Input", 10, 40, 301, 100, false)
	a := b.asset("Split", 40, 50, 301, 35, false)
	c := b.asset("Split", 40, 60, 301, 65, false)
	return b.splitFixture("split", i, a, c), nil
}
func (b *fixtureBuilder) process() (Fixture, error) {
	a := b.asset("Input", 10, 70, 401, 25, false)
	c := b.asset("Input", 20, 70, 402, 75, false)
	o := b.asset("Process", 70, 80, 499, 100, false)
	return b.mergeFixture("process", "Process", a, c, o), nil
}
func (b *fixtureBuilder) exit() (Fixture, error) {
	i := b.asset("Input", 10, 90, 501, 10, false)
	o := b.asset("Exit", 90, 90, 501, 10, true)
	return b.exitFixture("exit", i, o), nil
}

func (b *fixtureBuilder) entryFixture(caseName string, output Asset) Fixture {
	path := b.path(output)
	return Fixture{Case: caseName, Name: "Entry", Profile: b.profile, Outputs: []Asset{output}, Assignments: []gadgets.NamedAssignment{path, b.eq(output, Sender, output, Recipient)}}
}
func (b *fixtureBuilder) shipFixture(caseName string, input, output Asset) Fixture {
	return Fixture{Case: caseName, Name: "Ship", Profile: b.profile, Inputs: []Asset{input}, Outputs: []Asset{output}, Assignments: []gadgets.NamedAssignment{b.path(input), b.path(output), b.eq(input, Recipient, output, Sender), b.eq(input, ItemCode, output, ItemCode), b.eq(input, Quantity, output, Quantity)}}
}
func (b *fixtureBuilder) mergeFixture(caseName, event string, a, c, o Asset) Fixture {
	return Fixture{Case: caseName, Name: event, Profile: b.profile, Inputs: []Asset{a, c}, Outputs: []Asset{o}, Assignments: []gadgets.NamedAssignment{b.path(a), b.path(c), b.path(o), b.eq(a, Recipient, o, Sender), b.eq(c, Recipient, o, Sender), b.add(a, Quantity, c, Quantity, o, Quantity)}}
}
func (b *fixtureBuilder) splitFixture(caseName string, i, a, c Asset) Fixture {
	return Fixture{Case: caseName, Name: "Split", Profile: b.profile, Inputs: []Asset{i}, Outputs: []Asset{a, c}, Assignments: []gadgets.NamedAssignment{b.path(i), b.path(a), b.path(c), b.eq(i, Recipient, a, Sender), b.eq(i, Recipient, c, Sender), b.add(a, Quantity, c, Quantity, i, Quantity)}}
}
func (b *fixtureBuilder) exitFixture(caseName string, i, o Asset) Fixture {
	return Fixture{Case: caseName, Name: "Exit", Profile: b.profile, Inputs: []Asset{i}, Outputs: []Asset{o}, Assignments: []gadgets.NamedAssignment{b.path(o), b.eq(o, Sender, o, Recipient)}}
}

func (b *fixtureBuilder) asset(kind string, sender, recipient, item, quantity uint64, terminal bool) Asset {
	b.serial++
	doc := makeDocument(b.serial, sender, recipient, item, quantity)
	return Asset{Type: kind, Doc: doc, DocHash: document.Build(b.profile, doc).Root(), Terminal: terminal}
}
func makeDocument(serial, sender, recipient, item, quantity uint64) document.Document {
	var doc document.Document
	for i := 0; i < document.AttributeCount; i++ {
		doc.Attributes[i] = document.ValueFromUint64(serial*10000 + uint64(i+1))
		doc.Salts[i] = document.ValueFromUint64(serial*100000 + uint64(31*i+7))
	}
	doc.Attributes[Sender] = document.ValueFromUint64(sender)
	doc.Attributes[Recipient] = document.ValueFromUint64(recipient)
	doc.Attributes[ItemCode] = document.ValueFromUint64(item)
	doc.Attributes[Quantity] = document.ValueFromUint64(quantity)
	return doc
}
func (b *fixtureBuilder) path(asset Asset) gadgets.NamedAssignment {
	value, err := gadgets.MerkleAssignmentFor(b.profile, asset.Doc, 0)
	if err != nil {
		panic(err)
	}
	return value
}
func (b *fixtureBuilder) eq(left Asset, li int, right Asset, ri int) gadgets.NamedAssignment {
	if b.profile == document.Poseidon2 {
		a := gadgets.PoseidonEqAssignment(left.Doc.Attributes[li], left.Doc.Salts[li], right.Doc.Attributes[ri], right.Doc.Salts[ri])
		return namedPoseidon(b.profile, "eq", &gadgets.PoseidonEqCircuit{}, a, []any{a.HX, a.HY})
	}
	a := gadgets.EqAssignment(left.Doc.Attributes[li], left.Doc.Salts[li], right.Doc.Attributes[ri], right.Doc.Salts[ri])
	return gadgets.NamedAssignment{Name: "sha256-eq", Profile: b.profile, Gadget: "eq", Circuit: &gadgets.EqCircuit{}, Assignment: a, PublicInputs: gadgets.PublicEq(a)}
}
func (b *fixtureBuilder) add(x Asset, xi int, y Asset, yi int, z Asset, zi int) gadgets.NamedAssignment {
	if b.profile == document.Poseidon2 {
		a := gadgets.PoseidonAddAssignment(x.Doc.Attributes[xi], x.Doc.Salts[xi], y.Doc.Attributes[yi], y.Doc.Salts[yi], z.Doc.Attributes[zi], z.Doc.Salts[zi])
		return namedPoseidon(b.profile, "add", &gadgets.PoseidonAddCircuit{}, a, []any{a.HX, a.HY, a.HZ})
	}
	a := gadgets.AddAssignment(x.Doc.Attributes[xi], x.Doc.Salts[xi], y.Doc.Attributes[yi], y.Doc.Salts[yi], z.Doc.Attributes[zi], z.Doc.Salts[zi])
	return gadgets.NamedAssignment{Name: "sha256-add", Profile: b.profile, Gadget: "add", Circuit: &gadgets.AddCircuit{}, Assignment: a, PublicInputs: gadgets.PublicAdd(a)}
}
func namedPoseidon(profile document.Profile, gadget string, circuit, assignment any, values []any) gadgets.NamedAssignment {
	public := make([]*big.Int, len(values))
	for i, v := range values {
		public[i] = new(big.Int).Set(v.(*big.Int))
	}
	return gadgets.NamedAssignment{Name: string(profile) + "-" + gadget, Profile: profile, Gadget: gadget, Circuit: circuit.(frontend.Circuit), Assignment: assignment.(frontend.Circuit), PublicInputs: public}
}
