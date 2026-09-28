package experiments

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Case-level one warm-up batch and one official batch, each containing 1000 recoveries.
func RunPreparedRecovery(root string) error {
	dir := filepath.Join(root, "output/paper-supplement/prepared-recovery")
	if e := os.Mkdir(dir, 0755); e != nil {
		return e
	}
	base, all, e := keyFixture(2, 20)
	if e != nil {
		return e
	}
	defer func() {
		for _, s := range all {
			s.Value.SetInt64(0)
		}
	}()
	rows := []map[string]any{}
	cases := [][2]int{{2, 3}, {2, 5}, {2, 10}, {2, 20}, {3, 10}, {5, 10}, {7, 10}, {10, 10}}
	for _, kn := range cases {
		k, n := kn[0], kn[1]
		var p KeyPackage
		var shares []Released
		if k == 2 {
			p = base
			p.N = n
			p.Public = p.Public[:n]
			p.Session = fmt.Sprintf("prepared-2-of-%d", n)
			p.Digest = keyDigest(p)
			shares = append([]Released(nil), all[:k]...)
			for i := range shares {
				shares[i].Session = p.Session
				shares[i].Digest = p.Digest
			}
		} else {
			p, shares, e = keyFixture(k, n)
			if e != nil {
				return e
			}
		}
		start := time.Now()
		prepared, e := prepareCommittee(p)
		if e != nil {
			return e
		}
		preparation := ms(time.Since(start))
		for run := 0; run < 2; run++ {
			runtime.GC()
			var phases RecoveryTime
			start = time.Now()
			for i := 0; i < 1000; i++ {
				key, t, err := recoverPreparedKey(prepared, shares[:k])
				if err != nil {
					return err
				}
				phases.ValidationMS += t.ValidationMS
				phases.InterpolationMS += t.InterpolationMS
				phases.PublicCheckMS += t.PublicCheckMS
				key.SetInt64(0)
			}
			elapsed := ms(time.Since(start))
			row := map[string]any{"k": k, "n": n, "run": run, "warmup": run == 0, "innerIterations": 1000, "batchMS": elapsed, "recoveryMS": elapsed / 1000, "shareValidationMS": phases.ValidationMS / 1000, "interpolationMS": phases.InterpolationMS / 1000, "publicCheckMS": phases.PublicCheckMS / 1000, "committeePreparationMS": preparation, "checkedSecretSharesPerRecovery": k, "fullCommitteeValidationsInTimer": 0}
			rows = append(rows, row)
			fmt.Printf("%d-of-%d warmup=%v %.6f ms\n", k, n, run == 0, elapsed/1000)
		}
		if k != 2 {
			for _, s := range shares {
				s.Value.SetInt64(0)
			}
		}
	}
	return writeJSON(filepath.Join(dir, "result.json"), map[string]any{"status": "complete", "officialRuns": 1, "warmups": 1, "innerIterations": 1000, "GOMAXPROCS": runtime.GOMAXPROCS(0), "goVersion": runtime.Version(), "sameK2KeyAndSelectedShares": true, "boundary": "validated immutable Committee data prepared outside timer; only k released shares verified then interpolated and result checked; best-effort scalar clearing included; no DKG, network or approval wait", "rows": rows})
}
