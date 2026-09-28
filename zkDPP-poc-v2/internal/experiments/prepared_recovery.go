package experiments

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"math/big"
	"time"
)

// A validated, privately owned copy. The caller must obtain it from an authenticated
// Committee source; point validation and the digest do not authenticate that source.
type preparedCommittee struct{ p KeyPackage }

func prepareCommittee(p KeyPackage) (*preparedCommittee, error) {
	p.Public = append([]auditcrypto.Point(nil), p.Public...)
	if p.K < 2 || p.K > p.N || p.N != len(p.Public) || p.Session == "" || keyDigest(p) != p.Digest {
		return nil, fmt.Errorf("invalid committee data")
	}
	if err := auditcrypto.ValidatePoint(p.PK); err != nil {
		return nil, err
	}
	for _, v := range p.Public {
		if err := auditcrypto.ValidatePoint(v); err != nil {
			return nil, err
		}
	}
	return &preparedCommittee{p: p}, nil
}
func recoverPreparedKey(checked *preparedCommittee, shares []Released) (*big.Int, RecoveryTime, error) {
	var t RecoveryTime
	start := time.Now()
	if checked == nil || len(shares) != checked.p.K {
		return nil, t, fmt.Errorf("wrong share count")
	}
	p := checked.p
	g := auditcrypto.Generator()
	seen := map[int]bool{}
	for _, s := range shares {
		if s.ID < 1 || s.ID > p.N || seen[s.ID] || s.Session != p.Session || s.Digest != p.Digest || s.Value == nil {
			return nil, t, fmt.Errorf("invalid release")
		}
		seen[s.ID] = true
		if e := auditcrypto.ValidateScalar(s.Value, true); e != nil {
			return nil, t, e
		}
		var pub auditcrypto.Point
		pub.ScalarMultiplication(&g, s.Value)
		if !pub.Equal(&p.Public[s.ID-1]) {
			return nil, t, fmt.Errorf("public share mismatch")
		}
	}
	t.ValidationMS = ms(time.Since(start))
	start = time.Now()
	q := auditcrypto.Order()
	out := new(big.Int)
	for i, s := range shares {
		num, den := big.NewInt(1), big.NewInt(1)
		for j, o := range shares {
			if i == j {
				continue
			}
			num.Mul(num, big.NewInt(int64(-o.ID))).Mod(num, q)
			den.Mul(den, big.NewInt(int64(s.ID-o.ID))).Mod(den, q)
		}
		inv := new(big.Int).ModInverse(den, q)
		if inv == nil {
			return nil, t, fmt.Errorf("singular IDs")
		}
		term := new(big.Int).Mul(num, inv)
		term.Mul(term, s.Value)
		out.Add(out, term).Mod(out, q)
	}
	t.InterpolationMS = ms(time.Since(start))
	start = time.Now()
	var pub auditcrypto.Point
	pub.ScalarMultiplication(&g, out)
	if !pub.Equal(&p.PK) {
		out.SetInt64(0)
		return nil, t, fmt.Errorf("master key mismatch")
	}
	t.PublicCheckMS = ms(time.Since(start))
	return out, t, nil
}
