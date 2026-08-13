package document

import "fmt"

const (
	AttributeCount    = 32
	AttributesPerLeaf = 4
	LeafCount         = 8
	PathSiblings      = 3
)

type Document struct {
	Attributes [AttributeCount]Value128
	Salts      [AttributeCount]Value128
}

type Tree struct {
	Profile Profile
	Levels  [4][]Digest
}

type Opening struct {
	LeafIndex  int
	Attributes [AttributesPerLeaf]Value128
	Salts      [AttributesPerLeaf]Value128
	Siblings   [PathSiblings]Digest
}

func Build(profile Profile, doc Document) Tree {
	tree := Tree{Profile: profile}
	tree.Levels[0] = make([]Digest, LeafCount)
	for leaf := 0; leaf < LeafCount; leaf++ {
		var values [4]Value128
		copy(values[:], doc.Attributes[leaf*AttributesPerLeaf:(leaf+1)*AttributesPerLeaf])
		if profile == Poseidon2 {
			tree.Levels[0][leaf] = PoseidonLeaf(values)
		} else {
			var block [64]byte
			for i := range values {
				copy(block[i*16:(i+1)*16], values[i][:])
			}
			tree.Levels[0][leaf] = SHACompress(block)
		}
	}
	for level := 1; level < 4; level++ {
		previous := tree.Levels[level-1]
		tree.Levels[level] = make([]Digest, len(previous)/2)
		for i := range tree.Levels[level] {
			left, right := previous[2*i], previous[2*i+1]
			if profile == Poseidon2 {
				tree.Levels[level][i] = PoseidonCompress(left, right)
			} else {
				var block [64]byte
				copy(block[:32], left[:])
				copy(block[32:], right[:])
				tree.Levels[level][i] = SHACompress(block)
			}
		}
	}
	return tree
}

func (t Tree) Root() Digest { return t.Levels[3][0] }

func (t Tree) Open(doc Document, leaf int) (Opening, error) {
	if leaf < 0 || leaf >= LeafCount {
		return Opening{}, fmt.Errorf("leaf index %d outside [0,%d)", leaf, LeafCount)
	}
	opening := Opening{LeafIndex: leaf}
	for i := 0; i < AttributesPerLeaf; i++ {
		opening.Attributes[i] = doc.Attributes[leaf*AttributesPerLeaf+i]
		opening.Salts[i] = doc.Salts[leaf*AttributesPerLeaf+i]
	}
	position := leaf
	for level := 0; level < PathSiblings; level++ {
		opening.Siblings[level] = t.Levels[level][position^1]
		position /= 2
	}
	return opening, nil
}

func RootFromOpening(profile Profile, opening Opening) Digest {
	var current Digest
	if profile == Poseidon2 {
		current = PoseidonLeaf(opening.Attributes)
	} else {
		var leaf [64]byte
		for i := range opening.Attributes {
			copy(leaf[i*16:(i+1)*16], opening.Attributes[i][:])
		}
		current = SHACompress(leaf)
	}
	position := opening.LeafIndex
	for _, sibling := range opening.Siblings {
		left, right := current, sibling
		if position&1 == 1 {
			left, right = sibling, current
		}
		if profile == Poseidon2 {
			current = PoseidonCompress(left, right)
		} else {
			var block [64]byte
			copy(block[:32], left[:])
			copy(block[32:], right[:])
			current = SHACompress(block)
		}
		position >>= 1
	}
	return current
}

func CanonicalDocument() Document {
	var doc Document
	for i := 0; i < AttributeCount; i++ {
		doc.Attributes[i] = ValueFromUint64(uint64(1000 + i))
		doc.Salts[i] = ValueFromUint64(uint64(9000 + 17*i))
	}
	return doc
}
