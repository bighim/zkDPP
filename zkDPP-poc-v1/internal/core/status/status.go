package status

import (
	"fmt"

	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type ObjectType uint8

const (
	ObjectTypeNote    ObjectType = 1
	ObjectTypeVoucher ObjectType = 2
)

type Status uint8

const (
	Active  Status = 0
	Frozen  Status = 1
	Revoked Status = 2
)

type nodeKey struct {
	level uint
	index uint64
}

type Path struct {
	Siblings [merkle.Depth]fr.Element
}

type Tree struct {
	zeroes [merkle.Depth + 1]fr.Element
	nodes  map[nodeKey]fr.Element
	root   fr.Element
}

func New() *Tree {
	t := &Tree{nodes: make(map[nodeKey]fr.Element)}
	for level := 0; level < merkle.Depth; level++ {
		t.zeroes[level+1] = zkhash.Compress(t.zeroes[level], t.zeroes[level])
	}
	t.root = t.zeroes[merkle.Depth]
	return t
}

func (t *Tree) Root() fr.Element { return t.root }

func (t *Tree) node(level uint, index uint64) fr.Element {
	if value, ok := t.nodes[nodeKey{level: level, index: index}]; ok {
		return value
	}
	return t.zeroes[level]
}

func (t *Tree) Status(index uint64) Status {
	var value fr.Element
	if stored, ok := t.nodes[nodeKey{level: 0, index: index}]; ok {
		value = stored
	}
	if value.IsZero() {
		return Active
	}
	frozen := Element(Frozen)
	if value.Equal(&frozen) {
		return Frozen
	}
	return Revoked
}

func (t *Tree) Path(index uint64) (Path, error) {
	if index >= uint64(1)<<merkle.Depth {
		return Path{}, fmt.Errorf("status index %d exceeds depth %d", index, merkle.Depth)
	}
	var path Path
	position := index
	for level := 0; level < merkle.Depth; level++ {
		path.Siblings[level] = t.node(uint(level), position^1)
		position >>= 1
	}
	return path, nil
}

func (t *Tree) Apply(index uint64, oldStatus, newStatus Status) (fr.Element, error) {
	if !AllowedTransition(oldStatus, newStatus) {
		return fr.Element{}, fmt.Errorf("invalid status transition %d -> %d", oldStatus, newStatus)
	}
	if t.Status(index) != oldStatus {
		return fr.Element{}, fmt.Errorf("status at index %d is not %d", index, oldStatus)
	}
	path, err := t.Path(index)
	if err != nil {
		return fr.Element{}, err
	}
	position := index
	current := Element(newStatus)
	setNode(t, 0, position, current)
	for level := uint(0); level < merkle.Depth; level++ {
		if position&1 == 0 {
			current = zkhash.Compress(current, path.Siblings[level])
		} else {
			current = zkhash.Compress(path.Siblings[level], current)
		}
		position >>= 1
		setNode(t, level+1, position, current)
	}
	t.root = current
	return current, nil
}

func AllowedTransition(oldStatus, newStatus Status) bool {
	return oldStatus == Active && newStatus == Frozen ||
		oldStatus == Frozen && newStatus == Active ||
		oldStatus == Frozen && newStatus == Revoked
}

func ComputeRoot(value Status, index uint64, path Path) fr.Element {
	current := Element(value)
	position := index
	for _, sibling := range path.Siblings {
		if position&1 == 0 {
			current = zkhash.Compress(current, sibling)
		} else {
			current = zkhash.Compress(sibling, current)
		}
		position >>= 1
	}
	return current
}

func Element(value Status) fr.Element {
	var out fr.Element
	out.SetUint64(uint64(value))
	return out
}

func setNode(t *Tree, level uint, index uint64, value fr.Element) {
	key := nodeKey{level: level, index: index}
	if value.Equal(&t.zeroes[level]) {
		delete(t.nodes, key)
		return
	}
	t.nodes[key] = value
}
