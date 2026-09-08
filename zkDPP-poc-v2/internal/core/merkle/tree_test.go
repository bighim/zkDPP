package merkle_test

import (
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
)

func TestDepth32Path(t *testing.T) {
	tree := merkle.New()
	for _, leaf := range []uint64{11, 22, 33} {
		if _, _, err := tree.Append(zkhash.Element(leaf)); err != nil {
			t.Fatal(err)
		}
	}
	path, err := tree.Path(1)
	if err != nil {
		t.Fatal(err)
	}
	if !merkle.Verify(tree.Root(), zkhash.Element(22), path) {
		t.Fatal("valid membership rejected")
	}
	path.Siblings[0] = zkhash.Element(999)
	if merkle.Verify(tree.Root(), zkhash.Element(22), path) {
		t.Fatal("mutated path accepted")
	}
}
