package voucher_test

import (
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"math"
	"testing"
)

func TestAllocationResidualAndFullTransfer(t *testing.T) {
	v, c, r, err := voucher.Allocate(note.State{QMass: 3e9, ARec: 1e9, E: 2e9}, 1e9)
	if err != nil {
		t.Fatal(err)
	}
	if v != (note.State{QMass: 1e9, ARec: 333333334, E: 666666667}) || c != (note.State{QMass: 2e9, ARec: 666666666, E: 1333333333}) || r.ARec != 2e9 || r.E != 1e9 {
		t.Fatalf("unexpected allocation: %#v %#v %#v", v, c, r)
	}
	v, c, _, err = voucher.Allocate(note.State{QMass: 5e9, ARec: 2e9, E: 4e9}, 5e9)
	if err != nil || c != (note.State{}) || v.QMass != 5e9 || v.ARec != 2e9 || v.E != 4e9 {
		t.Fatalf("full transfer failed: %#v %#v %v", v, c, err)
	}
}

func TestTransportCarbonAndOverflow(t *testing.T) {
	v, change, _, err := voucher.AllocateWithTransport(note.State{QMass: 100, ARec: 20, E: 80}, 60, 7)
	if err != nil || v != (note.State{QMass: 60, ARec: 12, E: 55}) || change != (note.State{QMass: 40, ARec: 8, E: 32}) {
		t.Fatalf("transport allocation: %v %v %v", v, change, err)
	}
	if _, _, _, err = voucher.AllocateWithTransport(note.State{QMass: 2, ARec: 0, E: math.MaxUint64}, 2, 1); err == nil {
		t.Fatal("transport carbon overflow accepted")
	}
}

func TestVoucherDomainsBindOpening(t *testing.T) {
	v, err := voucher.New(zkhash.Element(1), note.AssetRoleEligible, note.State{QMass: 3}, zkhash.Element(2), zkhash.Element(3), 10, zkhash.Element(4))
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := voucher.New(v.DocumentHash, v.AssetRole, v.State, v.SenderAddress, v.ReceiverAddress, v.DeadlineBlock, zkhash.Element(5))
	nf := voucher.Nullifier(v.Opening, v.Commitment)
	if v.Commitment.Equal(&changed.Commitment) || v.Commitment.Equal(&nf) {
		t.Fatal("domain or opening not bound")
	}
}
