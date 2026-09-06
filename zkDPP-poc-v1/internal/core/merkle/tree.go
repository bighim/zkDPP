package merkle

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const Depth = 32

type nodeKey struct {
	level uint
	index uint64
}

type Path struct {
	Index    uint64
	Siblings [Depth]fr.Element
}

type Tree struct {
	count  uint64
	zeroes [Depth + 1]fr.Element
	nodes  map[nodeKey]fr.Element
	root   fr.Element
}

func New() *Tree {
	tree := &Tree{nodes: make(map[nodeKey]fr.Element)}
	for level := 0; level < Depth; level++ {
		tree.zeroes[level+1] = zkhash.Compress(tree.zeroes[level], tree.zeroes[level])
	}
	tree.root = tree.zeroes[Depth]
	return tree
}

func (t *Tree) Count() uint64    { return t.count }
func (t *Tree) Root() fr.Element { return t.root }

func (t *Tree) node(level uint, index uint64) fr.Element {
	if value, ok := t.nodes[nodeKey{level: level, index: index}]; ok {
		return value
	}
	return t.zeroes[level]
}

func (t *Tree) Append(leaf fr.Element) (uint64, fr.Element, error) {
	if t.count >= uint64(1)<<Depth {
		return 0, fr.Element{}, fmt.Errorf("tree is full")
	}
	index := t.count
	position := index
	current := leaf
	t.nodes[nodeKey{level: 0, index: position}] = current
	for level := uint(0); level < Depth; level++ {
		var left, right fr.Element
		if position&1 == 0 {
			left, right = current, t.node(level, position+1)
		} else {
			left, right = t.node(level, position-1), current
		}
		current = zkhash.Compress(left, right)
		position >>= 1
		t.nodes[nodeKey{level: level + 1, index: position}] = current
	}
	t.count++
	t.root = current
	return index, current, nil
}

func (t *Tree) Path(index uint64) (Path, error) {
	if index >= t.count {
		return Path{}, fmt.Errorf("leaf index %d is not present", index)
	}
	var path Path
	path.Index = index
	position := index
	for level := 0; level < Depth; level++ {
		path.Siblings[level] = t.node(uint(level), position^1)
		position >>= 1
	}
	return path, nil
}

func Verify(root, leaf fr.Element, path Path) bool {
	current := leaf
	position := path.Index
	for _, sibling := range path.Siblings {
		if position&1 == 0 {
			current = zkhash.Compress(current, sibling)
		} else {
			current = zkhash.Compress(sibling, current)
		}
		position >>= 1
	}
	return current.Equal(&root)
}
