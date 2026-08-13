package gadgets

import (
	"math/big"

	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/consensys/gnark/frontend"
)

type NamedAssignment struct {
	Name         string
	Profile      document.Profile
	Gadget       string
	Circuit      frontend.Circuit
	Assignment   frontend.Circuit
	PublicInputs []*big.Int
}

func CanonicalAssignments() ([]NamedAssignment, error) {
	return CanonicalAssignmentsFor(document.SHA256)
}

func CanonicalAssignmentsFor(profile document.Profile) ([]NamedAssignment, error) {
	doc := document.CanonicalDocument()
	merkleSpec, err := MerkleAssignmentFor(profile, doc, 2)
	if err != nil {
		return nil, err
	}
	x, sx := document.ValueFromUint64(42), document.ValueFromUint64(101)
	y, sy := document.ValueFromUint64(42), document.ValueFromUint64(202)
	addX, addSX := document.ValueFromUint64(40), document.ValueFromUint64(301)
	addY, addSY := document.ValueFromUint64(2), document.ValueFromUint64(302)
	addZ, addSZ := document.ValueFromUint64(42), document.ValueFromUint64(303)
	andX, andSX := document.ValueFromUint64(1), document.ValueFromUint64(401)
	andY, andSY := document.ValueFromUint64(1), document.ValueFromUint64(402)
	andZ, andSZ := document.ValueFromUint64(1), document.ValueFromUint64(403)
	if profile == document.Poseidon2 {
		eq := PoseidonEqAssignment(x, sx, y, sy)
		add := PoseidonAddAssignment(addX, addSX, addY, addSY, addZ, addSZ)
		and := PoseidonAndAssignment(andX, andSX, andY, andSY, andZ, andSZ)
		return []NamedAssignment{
			merkleSpec,
			named(profile, "eq", &PoseidonEqCircuit{}, eq, []frontend.Variable{eq.HX, eq.HY}),
			named(profile, "add", &PoseidonAddCircuit{}, add, []frontend.Variable{add.HX, add.HY, add.HZ}),
			named(profile, "and", &PoseidonAndCircuit{}, and, []frontend.Variable{and.HX, and.HY, and.HZ}),
		}, nil
	}
	eq := EqAssignment(x, sx, y, sy)
	add := AddAssignment(addX, addSX, addY, addSY, addZ, addSZ)
	and := AndAssignment(andX, andSX, andY, andSY, andZ, andSZ)
	return []NamedAssignment{
		merkleSpec,
		{Name: "sha256-eq", Profile: profile, Gadget: "eq", Circuit: &EqCircuit{}, Assignment: eq, PublicInputs: publicEq(eq)},
		{Name: "sha256-add", Profile: profile, Gadget: "add", Circuit: &AddCircuit{}, Assignment: add, PublicInputs: publicAdd(add)},
		{Name: "sha256-and", Profile: profile, Gadget: "and", Circuit: &AndCircuit{}, Assignment: and, PublicInputs: publicAnd(and)},
	}, nil
}

func MerkleAssignment(doc document.Document, leaf int) (NamedAssignment, error) {
	return MerkleAssignmentFor(document.SHA256, doc, leaf)
}

func MerkleAssignmentFor(profile document.Profile, doc document.Document, leaf int) (NamedAssignment, error) {
	tree := document.Build(profile, doc)
	opening, err := tree.Open(doc, leaf)
	if err != nil {
		return NamedAssignment{}, err
	}
	if profile == document.Poseidon2 {
		merkle := &PoseidonMerklePathCircuit{LeafIndex: opening.LeafIndex}
		merkle.Root = digestScalar(tree.Root())
		public := document.DigestPublic(profile, tree.Root())
		for i := 0; i < 4; i++ {
			merkle.Attributes[i] = valueInt(opening.Attributes[i])
			merkle.Salts[i] = valueInt(opening.Salts[i])
			commitment := document.AttributeCommitment(profile, opening.Attributes[i], opening.Salts[i])
			merkle.Commitments[i] = digestScalar(commitment)
			public = append(public, document.DigestPublic(profile, commitment)...)
		}
		for i := 0; i < 3; i++ {
			merkle.Siblings[i] = digestScalar(opening.Siblings[i])
		}
		return NamedAssignment{Name: "poseidon2-merklepath", Profile: profile, Gadget: "merklepath", Circuit: &PoseidonMerklePathCircuit{}, Assignment: merkle, PublicInputs: public}, nil
	}
	merkle := &MerklePathCircuit{LeafIndex: opening.LeafIndex}
	merkle.Root = variableDigest(tree.Root())
	publicMerkle := document.DigestPublic(profile, tree.Root())
	for i := 0; i < 4; i++ {
		merkle.Attributes[i] = valueInt(opening.Attributes[i])
		merkle.Salts[i] = valueInt(opening.Salts[i])
		commitment := document.AttributeCommitment(profile, opening.Attributes[i], opening.Salts[i])
		merkle.Commitments[i] = variableDigest(commitment)
		publicMerkle = append(publicMerkle, document.DigestPublic(profile, commitment)...)
	}
	for i := 0; i < 3; i++ {
		merkle.Siblings[i] = variableDigest(opening.Siblings[i])
	}
	return NamedAssignment{Name: "sha256-merklepath", Profile: profile, Gadget: "merklepath", Circuit: &MerklePathCircuit{}, Assignment: merkle, PublicInputs: publicMerkle}, nil
}

func EqAssignment(x, saltX, y, saltY document.Value128) *EqCircuit {
	return &EqCircuit{
		HX: variableDigest(document.AttributeCommitment(document.SHA256, x, saltX)), HY: variableDigest(document.AttributeCommitment(document.SHA256, y, saltY)),
		X: valueInt(x), Y: valueInt(y), SaltX: valueInt(saltX), SaltY: valueInt(saltY),
	}
}

func AddAssignment(x, saltX, y, saltY, z, saltZ document.Value128) *AddCircuit {
	return &AddCircuit{
		HX: variableDigest(document.AttributeCommitment(document.SHA256, x, saltX)), HY: variableDigest(document.AttributeCommitment(document.SHA256, y, saltY)),
		HZ: variableDigest(document.AttributeCommitment(document.SHA256, z, saltZ)), X: valueInt(x), Y: valueInt(y), Z: valueInt(z),
		SaltX: valueInt(saltX), SaltY: valueInt(saltY), SaltZ: valueInt(saltZ),
	}
}

func AndAssignment(x, saltX, y, saltY, z, saltZ document.Value128) *AndCircuit {
	return &AndCircuit{
		HX: variableDigest(document.AttributeCommitment(document.SHA256, x, saltX)), HY: variableDigest(document.AttributeCommitment(document.SHA256, y, saltY)),
		HZ: variableDigest(document.AttributeCommitment(document.SHA256, z, saltZ)), X: valueInt(x), Y: valueInt(y), Z: valueInt(z),
		SaltX: valueInt(saltX), SaltY: valueInt(saltY), SaltZ: valueInt(saltZ),
	}
}

func PublicEq(c *EqCircuit) []*big.Int   { return publicEq(c) }
func PublicAdd(c *AddCircuit) []*big.Int { return publicAdd(c) }
func PublicAnd(c *AndCircuit) []*big.Int { return publicAnd(c) }

func variableDigest(digest document.Digest) [2]frontend.Variable {
	limbs := document.DigestPublic(document.SHA256, digest)
	return [2]frontend.Variable{limbs[0], limbs[1]}
}

func digestScalar(digest document.Digest) *big.Int { return new(big.Int).SetBytes(digest[:]) }

func PoseidonEqAssignment(x, saltX, y, saltY document.Value128) *PoseidonEqCircuit {
	return &PoseidonEqCircuit{HX: digestScalar(document.AttributeCommitment(document.Poseidon2, x, saltX)), HY: digestScalar(document.AttributeCommitment(document.Poseidon2, y, saltY)), X: valueInt(x), Y: valueInt(y), SaltX: valueInt(saltX), SaltY: valueInt(saltY)}
}

func PoseidonAddAssignment(x, saltX, y, saltY, z, saltZ document.Value128) *PoseidonAddCircuit {
	return &PoseidonAddCircuit{HX: digestScalar(document.AttributeCommitment(document.Poseidon2, x, saltX)), HY: digestScalar(document.AttributeCommitment(document.Poseidon2, y, saltY)), HZ: digestScalar(document.AttributeCommitment(document.Poseidon2, z, saltZ)), X: valueInt(x), Y: valueInt(y), Z: valueInt(z), SaltX: valueInt(saltX), SaltY: valueInt(saltY), SaltZ: valueInt(saltZ)}
}

func PoseidonAndAssignment(x, saltX, y, saltY, z, saltZ document.Value128) *PoseidonAndCircuit {
	add := PoseidonAddAssignment(x, saltX, y, saltY, z, saltZ)
	return (*PoseidonAndCircuit)(add)
}

func named(profile document.Profile, gadget string, circuit, assignment frontend.Circuit, values []frontend.Variable) NamedAssignment {
	public := make([]*big.Int, len(values))
	for i := range values {
		public[i] = new(big.Int).Set(values[i].(*big.Int))
	}
	return NamedAssignment{Name: string(profile) + "-" + gadget, Profile: profile, Gadget: gadget, Circuit: circuit, Assignment: assignment, PublicInputs: public}
}

func valueInt(value document.Value128) *big.Int { return new(big.Int).SetBytes(value[:]) }

func publicEq(c *EqCircuit) []*big.Int { return append(limbs(c.HX), limbs(c.HY)...) }
func publicAdd(c *AddCircuit) []*big.Int {
	return append(append(limbs(c.HX), limbs(c.HY)...), limbs(c.HZ)...)
}
func publicAnd(c *AndCircuit) []*big.Int {
	return append(append(limbs(c.HX), limbs(c.HY)...), limbs(c.HZ)...)
}

func limbs(values [2]frontend.Variable) []*big.Int {
	return []*big.Int{new(big.Int).Set(values[0].(*big.Int)), new(big.Int).Set(values[1].(*big.Int))}
}
