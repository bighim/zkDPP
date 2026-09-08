package audit_test

import (
	"errors"
	"os"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"testing"
)

func TestOriginalABIBoundary(t *testing.T) {
	a, _, _, e := m7run.Contract("../..", "ZkDPPAuditLedger.sol", "ZkDPPAuditLedger")
	if e != nil {
		if errors.Is(e, os.ErrNotExist) {
			t.Skip("v1 generated ABI is intentionally not copied into the v2 module")
		}
		t.Fatal(e)
	}
	f, e := m7run.LoadFixture("../..")
	if e != nil {
		t.Fatal(e)
	}
	for _, g := range f.Groups {
		for _, c := range g.Events {
			t.Run(g.Name+"/"+c.Name, func(t *testing.T) {
				data, e := m7run.CaseData(a, c)
				if e != nil {
					t.Fatal(e)
				}
				_, p, r, e := c.Decode()
				if e != nil {
					t.Fatal(e)
				}
				l, e := audit.DecodeOriginal(a, r, data)
				if e != nil {
					t.Fatal(e)
				}
				if !audit.Equal(l.Base, p) {
					t.Fatal("base ordering")
				}
				changed := append([]byte{}, data...)
				changed[0] ^= 0xff
				if _, e = audit.DecodeOriginal(a, r, changed); e == nil {
					t.Fatal("wrong selector")
				}
				if _, e = audit.DecodeOriginal(a, r, data[:3]); e == nil {
					t.Fatal("short calldata")
				}
				r.R1.X.SetZero()
				if _, e = audit.DecodeOriginal(a, r, data); e == nil {
					t.Fatal("changed point")
				}
			})
		}
	}
}
