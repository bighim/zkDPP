package hotfix

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func KeyBench(root string) error {
	path := filepath.Join(root, "output/m1-hotfix-key-recovery.json")
	if _, e := os.Stat(path); e == nil {
		return fmt.Errorf("key measurement exists")
	}
	pkg, shares, e := v2case.KeyFixture()
	if e != nil {
		return e
	}
	var pairs []map[string]any
	for _, p := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		r := []auditcrypto.ReleasedShare{}
		for _, i := range p {
			r = append(r, auditcrypto.ReleasedShare{Profile: pkg.Profile, SessionID: pkg.SessionID, PackageChecksum: auditcrypto.PackageChecksum(pkg), Share: shares[i]})
		}
		start := time.Now()
		master, e := auditcrypto.RecoverReleased(pkg, r)
		ms := float64(time.Since(start).Nanoseconds()) / 1e6
		if e != nil {
			return e
		}
		master.SetInt64(0)
		pairs = append(pairs, map[string]any{"members": []int{p[0] + 1, p[1] + 1}, "millis": ms, "publicKeyMatched": true})
	}
	master, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if e != nil {
		return e
	}
	defer master.SetInt64(0)
	defer func() {
		for i := range shares {
			shares[i].Value.SetInt64(0)
		}
	}()
	var rows []map[string]any
	for _, count := range []int{1, 10, 100, 1000} {
		cts := make([]auditcrypto.Ciphertext, count)
		messages := make([][]fr.Element, count)
		for i := range cts {
			messages[i] = []fr.Element{h.Element(uint64(i)), h.Element(3), h.Element(5), h.Element(7), h.Element(11)}
			cts[i], e = auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, messages[i], big.NewInt(int64(90000+i)))
			if e != nil {
				return e
			}
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		for i, ct := range cts {
			out, e := auditcrypto.DecryptWithMasterKey(master, ct)
			if e != nil {
				return e
			}
			for j := range out {
				if !out[j].Equal(&messages[i][j]) {
					return fmt.Errorf("plaintext mismatch")
				}
			}
		}
		ms := float64(time.Since(start).Nanoseconds()) / 1e6
		runtime.ReadMemStats(&after)
		rows = append(rows, map[string]any{"records": count, "fields": 5, "millis": ms, "allocatedBytes": after.TotalAlloc - before.TotalAlloc, "allFieldsMatched": true})
	}
	return Write(path, map[string]any{"profile": pkg.Profile, "packageChecksum": auditcrypto.PackageChecksum(pkg), "recovery": pairs, "decrypt": rows, "runCount": 1})
}
