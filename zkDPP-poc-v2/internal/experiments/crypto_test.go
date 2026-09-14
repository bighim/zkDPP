package experiments

import (
	"math/big"
	"testing"
)

func TestThresholdRecovery(t *testing.T) {
	for _, kn := range [][2]int{{2, 3}, {3, 10}, {5, 10}, {10, 10}, {2, 20}} {
		p, s, e := keyFixture(kn[0], kn[1])
		if e != nil {
			t.Fatal(e)
		}
		a, _, e := recoverKey(p, s[:p.K])
		if e != nil {
			t.Fatal(e)
		}
		b, _, e := recoverKey(p, s[p.N-p.K:])
		if e != nil || a.Cmp(b) != 0 {
			t.Fatal("combination mismatch", e)
		}
		if _, _, e = recoverKey(p, s[:p.K-1]); e == nil {
			t.Fatal("insufficient")
		}
		bad := append([]Released{}, s[:p.K]...)
		bad[1] = bad[0]
		if _, _, e = recoverKey(p, bad); e == nil {
			t.Fatal("duplicate")
		}
		bad = append([]Released{}, s[:p.K]...)
		bad[0].Session = "other"
		if _, _, e = recoverKey(p, bad); e == nil {
			t.Fatal("session")
		}
		bad = append([]Released{}, s[:p.K]...)
		bad[0].Value = new(big.Int).Add(bad[0].Value, big.NewInt(1))
		if _, _, e = recoverKey(p, bad); e == nil {
			t.Fatal("tamper")
		}
		a.SetInt64(0)
		b.SetInt64(0)
		for _, x := range s {
			x.Value.SetInt64(0)
		}
	}
}
