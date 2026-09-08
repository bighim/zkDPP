package status_test

import (
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
)

func TestStatusTreeTransitions(t *testing.T) {
	tree := status.New()
	initial := tree.Root()
	path, err := tree.Path(7)
	if err != nil {
		t.Fatal(err)
	}
	if got := status.ComputeRoot(status.Active, 7, path); !got.Equal(&initial) {
		t.Fatal("empty Active path does not match initial root")
	}
	frozen, err := tree.Apply(7, status.Active, status.Frozen)
	if err != nil || tree.Status(7) != status.Frozen {
		t.Fatalf("freeze: root=%s err=%v", frozen.String(), err)
	}
	path, _ = tree.Path(7)
	if got := status.ComputeRoot(status.Frozen, 7, path); !got.Equal(&frozen) {
		t.Fatal("Frozen path does not match root")
	}
	active, err := tree.Apply(7, status.Frozen, status.Active)
	if err != nil || !active.Equal(&initial) {
		t.Fatalf("unfreeze did not restore empty root: %v", err)
	}
	_, _ = tree.Apply(7, status.Active, status.Frozen)
	if _, err = tree.Apply(7, status.Frozen, status.Revoked); err != nil {
		t.Fatal(err)
	}
	if _, err = tree.Apply(7, status.Revoked, status.Active); err == nil {
		t.Fatal("terminal Revoked transition accepted")
	}
}

func TestStatusTreesAreIndependent(t *testing.T) {
	noteTree := status.New()
	voucherTree := status.New()
	_, _ = noteTree.Apply(3, status.Active, status.Frozen)
	if voucherTree.Status(3) != status.Active {
		t.Fatal("Note status leaked into Voucher tree")
	}
}
