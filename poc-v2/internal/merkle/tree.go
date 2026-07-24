package merkle

import (
	"fmt"

	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type nodeKey struct {
	level uint
	index uint64
}

type Path struct {
	Index    uint64
	Siblings []fr.Element
}

type Compressor func(left, right fr.Element) fr.Element

type Tree struct {
	depth    uint
	count    uint64
	zeroes   []fr.Element
	nodes    map[nodeKey]fr.Element
	root     fr.Element
	accepted map[[32]byte]bool
	compress Compressor
}

func New(depth uint) *Tree {
	return NewWithCompressor(depth, zkhash.Compress)
}

func NewWithCompressor(depth uint, compress Compressor) *Tree {
	zeroes := make([]fr.Element, depth+1)
	for level := uint(0); level < depth; level++ {
		zeroes[level+1] = compress(zeroes[level], zeroes[level])
	}
	tree := &Tree{
		depth: depth, zeroes: zeroes, nodes: make(map[nodeKey]fr.Element),
		root: zeroes[depth], accepted: make(map[[32]byte]bool), compress: compress,
	}
	tree.accepted[tree.root.Bytes()] = true
	return tree
}

func (t *Tree) Depth() uint      { return t.depth }
func (t *Tree) Count() uint64    { return t.count }
func (t *Tree) Root() fr.Element { return t.root }

func (t *Tree) Accepted(root fr.Element) bool {
	return t.accepted[root.Bytes()]
}

func (t *Tree) node(level uint, index uint64) fr.Element {
	if value, ok := t.nodes[nodeKey{level: level, index: index}]; ok {
		return value
	}
	return t.zeroes[level]
}

func (t *Tree) Append(leaf fr.Element) (uint64, fr.Element, error) {
	if t.depth < 64 && t.count >= uint64(1)<<t.depth {
		return 0, fr.Element{}, fmt.Errorf("tree is full")
	}
	index := t.count
	position := index
	current := leaf
	t.nodes[nodeKey{level: 0, index: position}] = current
	for level := uint(0); level < t.depth; level++ {
		var left, right fr.Element
		if position&1 == 0 {
			left, right = current, t.node(level, position+1)
		} else {
			left, right = t.node(level, position-1), current
		}
		current = t.compress(left, right)
		position >>= 1
		t.nodes[nodeKey{level: level + 1, index: position}] = current
	}
	t.count++
	t.root = current
	t.accepted[current.Bytes()] = true
	return index, current, nil
}

func (t *Tree) Path(index uint64) (Path, error) {
	if index >= t.count {
		return Path{}, fmt.Errorf("leaf index %d is not present", index)
	}
	siblings := make([]fr.Element, t.depth)
	position := index
	for level := uint(0); level < t.depth; level++ {
		siblings[level] = t.node(level, position^1)
		position >>= 1
	}
	return Path{Index: index, Siblings: siblings}, nil
}

func Verify(root, leaf fr.Element, path Path) bool {
	return VerifyWithCompressor(root, leaf, path, zkhash.Compress)
}

func VerifyWithCompressor(root, leaf fr.Element, path Path, compress Compressor) bool {
	current := leaf
	position := path.Index
	for _, sibling := range path.Siblings {
		if position&1 == 0 {
			current = compress(current, sibling)
		} else {
			current = compress(sibling, current)
		}
		position >>= 1
	}
	return current.Equal(&root)
}
