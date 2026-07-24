package merkle_test

import (
	"testing"

	"github.com/bighim/zkDPP/poc-v2/internal/merkle"
	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
)

func TestAppendAndPathsShareLatestRoot(t *testing.T) {
	tree := merkle.New(32)
	leaves := []uint64{11, 22, 33}
	for _, value := range leaves {
		if _, _, err := tree.Append(zkhash.Element(value)); err != nil {
			t.Fatal(err)
		}
	}
	root := tree.Root()
	for index, value := range leaves {
		path, err := tree.Path(uint64(index))
		if err != nil {
			t.Fatal(err)
		}
		if !merkle.Verify(root, zkhash.Element(value), path) {
			t.Fatalf("path %d did not verify", index)
		}
	}
	if !tree.Accepted(root) {
		t.Fatal("latest root was not recorded")
	}
}
