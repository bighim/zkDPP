package experiments

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/big"
	"path/filepath"
	"runtime"
	"time"
)

type KeyPackage struct {
	K, N    int
	Session string
	PK      auditcrypto.Point
	Public  []auditcrypto.Point
	Digest  [32]byte
}
type Released struct {
	ID      int
	Session string
	Digest  [32]byte
	Value   *big.Int
}

func keyDigest(p KeyPackage) [32]byte {
	b := []byte(fmt.Sprintf("experiment-recovery-v1:%d:%d:%s", p.K, p.N, p.Session))
	for _, x := range append([]auditcrypto.Point{p.PK}, p.Public...) {
		xb, yb := x.X.Bytes(), x.Y.Bytes()
		b = append(b, xb[:]...)
		b = append(b, yb[:]...)
	}
	return sha256.Sum256(b)
}
func keyFixture(k, n int) (KeyPackage, []Released, error) {
	if k < 2 || k > n || n > 255 {
		return KeyPackage{}, nil, fmt.Errorf("threshold")
	}
	p := KeyPackage{K: k, N: n, Session: fmt.Sprintf("local-experiment-%d-%d", k, n), Public: make([]auditcrypto.Point, n)}
	q := auditcrypto.Order()
	g := auditcrypto.Generator()
	coeff := make([]*big.Int, k)
	for i := range coeff {
		coeff[i] = must(auditcrypto.RandomScalar(rand.Reader))
	}
	defer func() {
		for _, a := range coeff {
			a.SetInt64(0)
		}
	}()
	p.PK.ScalarMultiplication(&g, coeff[0])
	shares := make([]Released, n)
	for i := 0; i < n; i++ {
		v := new(big.Int)
		x := big.NewInt(int64(i + 1))
		for j := k - 1; j >= 0; j-- {
			v.Mul(v, x)
			v.Add(v, coeff[j])
			v.Mod(v, q)
		}
		if v.Sign() == 0 {
			return KeyPackage{}, nil, fmt.Errorf("zero fixture share")
		}
		p.Public[i].ScalarMultiplication(&g, v)
		shares[i] = Released{ID: i + 1, Session: p.Session, Value: v}
	}
	p.Digest = keyDigest(p)
	for i := range shares {
		shares[i].Digest = p.Digest
	}
	return p, shares, nil
}

type RecoveryTime struct{ ValidationMS, InterpolationMS, PublicCheckMS float64 }

func recoverKey(p KeyPackage, shares []Released) (*big.Int, RecoveryTime, error) {
	var t RecoveryTime
	start := time.Now()
	if p.K < 2 || p.K > p.N || p.N != len(p.Public) || len(shares) != p.K || p.Session == "" || keyDigest(p) != p.Digest {
		return nil, t, fmt.Errorf("invalid key package")
	}
	if e := auditcrypto.ValidatePoint(p.PK); e != nil {
		return nil, t, e
	}
	for _, v := range p.Public {
		if e := auditcrypto.ValidatePoint(v); e != nil {
			return nil, t, e
		}
	}
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

type CryptoRow struct {
	Experiment                                 int
	Case                                       string
	Run                                        int
	Warmup                                     bool
	Ciphertexts, Fields, K, N, InnerIterations int
	TotalMS                                    float64
	AllocatedBytes                             uint64
	Recovery                                   RecoveryTime
	Matched                                    bool
}

func RunCrypto(root string, exp int) error {
	if e := CheckBaseline(root); e != nil {
		return e
	}
	if exp != 7 && exp != 8 {
		return fmt.Errorf("crypto experiment")
	}
	var rows []CryptoRow
	if exp == 7 {
		pkg, shares, e := v2case.KeyFixture()
		if e != nil {
			return e
		}
		master, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
		if e != nil {
			return e
		}
		defer master.SetInt64(0)
		defer func() {
			for _, s := range shares {
				s.Value.SetInt64(0)
			}
		}()
		cases := [][2]int{{1, 3}, {10, 3}, {100, 3}, {1000, 3}, {1, 1}, {1, 2}, {1, 5}, {1, 10}, {1, 20}}
		for _, cfg := range cases {
			count, fields := cfg[0], cfg[1]
			cts := make([]auditcrypto.Ciphertext, count)
			msgs := make([][]fr.Element, count)
			for i := range cts {
				msgs[i] = make([]fr.Element, fields)
				for j := range msgs[i] {
					msgs[i][j] = h.Element(uint64(i*fields + j + 1))
				}
				cts[i], e = auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, msgs[i], must(auditcrypto.RandomScalar(rand.Reader)))
				if e != nil {
					return e
				}
			}
			for run := 0; run <= Repetitions; run++ {
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				last := make([][]fr.Element, count)
				start := time.Now()
				for iter := 0; iter < 1000; iter++ {
					for i, ct := range cts {
						last[i], e = auditcrypto.DecryptWithMasterKey(master, ct)
						if e != nil {
							return e
						}
					}
				}
				elapsed := ms(time.Since(start))
				runtime.ReadMemStats(&after)
				for i := range last {
					for j := range last[i] {
						if !last[i][j].Equal(&msgs[i][j]) {
							return fmt.Errorf("decryption mismatch")
						}
					}
				}
				rows = append(rows, CryptoRow{Experiment: 7, Case: fmt.Sprintf("ciphertexts-%d-fields-%d", count, fields), Run: run, Warmup: run == 0, Ciphertexts: count, Fields: fields, InnerIterations: 1000, TotalMS: elapsed, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Matched: true})
				fmt.Printf("decrypt %d x %d run %d: %.3f ms\n", count, fields, run, elapsed)
			}
		}
	} else {
		cases := [][2]int{{2, 10}, {3, 10}, {5, 10}, {7, 10}, {10, 10}, {2, 3}, {2, 5}, {2, 20}}
		for _, cfg := range cases {
			p, ss, e := keyFixture(cfg[0], cfg[1])
			if e != nil {
				return e
			}
			for run := 0; run <= Repetitions; run++ {
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				start := time.Now()
				var phases RecoveryTime
				for i := 0; i < 1000; i++ {
					x, t, e := recoverKey(p, ss[:p.K])
					if e != nil {
						return e
					}
					phases.ValidationMS += t.ValidationMS
					phases.InterpolationMS += t.InterpolationMS
					phases.PublicCheckMS += t.PublicCheckMS
					x.SetInt64(0)
				}
				elapsed := ms(time.Since(start))
				runtime.ReadMemStats(&after)
				rows = append(rows, CryptoRow{Experiment: 8, Case: fmt.Sprintf("%d-of-%d", p.K, p.N), Run: run, Warmup: run == 0, K: p.K, N: p.N, InnerIterations: 1000, TotalMS: elapsed, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Recovery: phases, Matched: true})
				fmt.Printf("recovery %d-of-%d run %d: %.3f ms\n", p.K, p.N, run, elapsed)
			}
			for _, s := range ss {
				s.Value.SetInt64(0)
			}
		}
	}
	return writeJSON(filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", exp)), map[string]any{"experiment": exp, "rows": rows, "warmups": 1, "repetitions": 5, "innerIterations": 1000, "boundary": "local validated cryptographic operation; fixture generation and final plaintext comparison excluded", "status": "complete"})
}
