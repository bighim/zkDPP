package experiments

import (
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"math/big"
	"testing"
)

func TestPreparedRecovery(t *testing.T) {
	for _, kn := range [][2]int{{2, 3}, {2, 20}, {3, 10}, {5, 10}, {7, 10}, {10, 10}} {
		p, ss, e := keyFixture(kn[0], kn[1])
		if e != nil {
			t.Fatal(e)
		}
		prepared, e := prepareCommittee(p)
		if e != nil {
			t.Fatal(e)
		}
		a, _, e := recoverPreparedKey(prepared, ss[:p.K])
		if e != nil {
			t.Fatal(e)
		}
		b, _, e := recoverPreparedKey(prepared, ss[p.N-p.K:])
		if e != nil || a.Cmp(b) != 0 {
			t.Fatal("combinations", e)
		}
		c, _, e := recoverKey(p, ss[:p.K])
		if e != nil || a.Cmp(c) != 0 {
			t.Fatal("old relation mismatch", e)
		}
		if _, _, e = recoverPreparedKey(prepared, ss[:p.K-1]); e == nil {
			t.Fatal("short")
		}
		for _, kind := range []string{"duplicate", "session", "digest", "value", "range", "id"} {
			bad := append([]Released(nil), ss[:p.K]...)
			switch kind {
			case "duplicate":
				bad[1] = bad[0]
			case "session":
				bad[0].Session = "wrong"
			case "digest":
				bad[0].Digest[0] ^= 1
			case "value":
				bad[0].Value = new(big.Int).Add(bad[0].Value, big.NewInt(1))
			case "range":
				bad[0].Value = auditcrypto.Order()
			case "id":
				bad[0].ID = p.N + 1
			}
			if _, _, e = recoverPreparedKey(prepared, bad); e == nil {
				t.Fatal(kind)
			}
		}
		// Mutating the source does not mutate the validated copy.
		p.Public[0] = auditcrypto.Point{}
		if _, _, e = recoverPreparedKey(prepared, ss[:p.K]); e != nil {
			t.Fatal("copy", e)
		}
		p.Digest = keyDigest(p)
		if _, e = prepareCommittee(p); e == nil {
			t.Fatal("invalid public point")
		}
		a.SetInt64(0)
		b.SetInt64(0)
		c.SetInt64(0)
		for _, x := range ss {
			x.Value.SetInt64(0)
		}
	}
}
