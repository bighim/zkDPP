package mergesame

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var modules = [2]string{"T", "A"}

func splitPublic(c *v2events.MergeCircuit) ([2]witness.Witness, error) {
	var ws [2]witness.Witness
	for i, a := range PublicSplit(c) {
		w, e := public(a)
		if e != nil {
			return ws, e
		}
		ws[i] = w
	}
	return ws, nil
}
func VerifyD(keys map[string]*Key, ps []plonk.Proof, c *v2events.MergeCircuit) error {
	if len(ps) != 2 {
		return fmt.Errorf("D proof count")
	}
	ws, e := splitPublic(c)
	if e != nil {
		return e
	}
	for i, p := range ps {
		if e = plonk.Verify(p, keys[modules[i]].VK, ws[i]); e != nil {
			return fmt.Errorf("%s: %w", modules[i], e)
		}
	}
	return nil
}
func prove(keys map[string]*Key, c *v2events.MergeCircuit, split bool) ([]plonk.Proof, error) {
	assignments := []frontend.Circuit{c}
	names := []string{"U"}
	if split {
		v := Split(c)
		assignments = v[:]
		names = modules[:]
	}
	ps := make([]plonk.Proof, len(names))
	for i, a := range assignments {
		w, e := full(a)
		if e != nil {
			return nil, e
		}
		ps[i], e = plonk.Prove(keys[names[i]].CCS, keys[names[i]].PK, w)
		if e != nil {
			return nil, e
		}
	}
	return ps, nil
}
func element(v frontend.Variable) fr.Element {
	switch x := v.(type) {
	case fr.Element:
		return x
	case uint64:
		return h.Element(x)
	case int:
		return h.Element(uint64(x))
	case *big.Int:
		var f fr.Element
		f.SetBigInt(x)
		return f
	default:
		panic(fmt.Sprintf("unsupported witness %T", v))
	}
}
func cm(n circuitutil.NoteWitness) fr.Element {
	return h.Hash(h.NoteTag, element(n.DocumentHash), element(n.AssetRole), element(n.QMass), element(n.ARec), element(n.E), element(n.Address), element(n.Opening))
}

// Rebind ciphertext, NF and CM consistently. Rebuilding the real depth-32 tree
// for input mutations prevents stale commitments masking the tested rule.
func Rebind(c *v2events.MergeCircuit, rebuild bool) error {
	sk := element(c.Base.SKOwner)
	parents := []fr.Element{cm(c.Base.Inputs[0]), cm(c.Base.Inputs[1])}
	c.Base.NF1 = note.Nullifier(sk, parents[0])
	c.Base.NF2 = note.Nullifier(sk, parents[1])
	c.Base.CMOut = cm(c.Base.Output)
	if rebuild {
		tree := merkle.New()
		for _, p := range parents {
			if _, _, e := tree.Append(p); e != nil {
				return e
			}
		}
		c.Base.NoteRoot = tree.Root()
		for i := range c.Base.Paths {
			p, e := tree.Path(uint64(i))
			if e != nil {
				return e
			}
			c.Base.Paths[i].Index = p.Index
			for j := range p.Siblings {
				c.Base.Paths[i].Siblings[j] = p.Siblings[j]
			}
		}
	}
	message := append(parents, note.Nullifier(sk, element(c.Base.CMOut)))
	ct, e := auditcrypto.EncryptWithMasterPublicKey(c.CommitteePK, message, big.NewInt(42))
	if e != nil {
		return e
	}
	c.Audit.Randomness = big.NewInt(42)
	c.Audit.R1.X = ct.R1.X
	c.Audit.R1.Y = ct.R1.Y
	for i := range c.Audit.Parents {
		c.Audit.Parents[i] = ct.Data[i]
	}
	c.Audit.OutputNfs[0] = ct.Data[2]
	return nil
}
func Check(source, out string) error {
	keys, e := Load(out)
	if e != nil {
		return e
	}
	cc, e := CCase(source)
	if e != nil {
		return e
	}
	c := cc.Assignment.(*v2events.MergeCircuit)
	rows := []map[string]any{}
	record := func(name string, ok bool, detail any) error {
		rows = append(rows, map[string]any{"name": name, "passed": ok, "detail": detail})
		if !ok {
			return fmt.Errorf("check failed %s: %v", name, detail)
		}
		return nil
	}
	solved := func(c *v2events.MergeCircuit) (bool, [2]bool, error) {
		w, e := full(c)
		if e != nil {
			return false, [2]bool{}, e
		}
		u := keys["U"].CCS.IsSolved(w) == nil
		var d [2]bool
		for i, a := range Split(c) {
			w, e = full(a)
			if e != nil {
				return false, d, e
			}
			d[i] = keys[modules[i]].CCS.IsSolved(w) == nil
		}
		return u, d, nil
	}
	u, d, e := solved(c)
	if e != nil {
		return e
	}
	if e = record("normal-relation", u && d[0] && d[1], nil); e != nil {
		return e
	}
	up, e := prove(keys, c, false)
	if e != nil {
		return e
	}
	dp, e := prove(keys, c, true)
	if e != nil {
		return e
	}
	pub, _ := public(c)
	if e = record("normal-real-proofs", plonk.Verify(up[0], keys["U"].VK, pub) == nil && VerifyD(keys, dp, c) == nil, nil); e != nil {
		return e
	}
	alt := CloneC(c)
	for i := range alt.Base.Inputs {
		alt.Base.Inputs[i].Opening = h.Element(90000 + uint64(i))
	}
	alt.Base.Output.Opening = h.Element(90002)
	if e = Rebind(alt, true); e != nil {
		return e
	}
	altUP, e := prove(keys, alt, false)
	if e != nil {
		return e
	}
	altDP, e := prove(keys, alt, true)
	if e != nil {
		return e
	}
	altPub, _ := public(alt)
	if e = record("alternative-real-proofs", plonk.Verify(altUP[0], keys["U"].VK, altPub) == nil && VerifyD(keys, altDP, alt) == nil, nil); e != nil {
		return e
	}
	for i := range dp {
		mixed := append([]plonk.Proof(nil), dp...)
		mixed[i] = altDP[i]
		if e = record(fmt.Sprintf("mix-other-transaction-slot-%d", i), VerifyD(keys, mixed, c) != nil, nil); e != nil {
			return e
		}
	}
	mutations := []struct {
		name            string
		rebuild, rebind bool
		mutate          func(*v2events.MergeCircuit)
	}{
		{"output-mass", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.QMass = uint64(100e9 + 1) }},
		{"output-credit", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.ARec = uint64(20e9 + 1) }},
		{"output-carbon", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.E = uint64(80e9 + 1) }},
		{"output-document", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.DocumentHash = h.Element(8) }},
		{"input-document", true, true, func(c *v2events.MergeCircuit) { c.Base.Inputs[1].DocumentHash = h.Element(8) }},
		{"owner-secret", false, true, func(c *v2events.MergeCircuit) { c.Base.SKOwner = h.Element(8) }},
		{"output-owner", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.Address = h.Element(8) }},
		{"duplicate-input", true, true, func(c *v2events.MergeCircuit) { c.Base.Inputs[1] = c.Base.Inputs[0] }},
		{"membership-sibling", false, false, func(c *v2events.MergeCircuit) { c.Base.Paths[0].Siblings[0] = h.Element(8) }},
		{"membership-index", false, false, func(c *v2events.MergeCircuit) { c.Base.Paths[0].Index = uint64(7) }},
		{"credit-exceeds-mass", false, true, func(c *v2events.MergeCircuit) { c.Base.Output.ARec = uint64(150e9) }},
		{"mass-range-overflow", true, true, func(c *v2events.MergeCircuit) {
			for i := range c.Base.Inputs {
				c.Base.Inputs[i].QMass = uint64(1<<63) + 1
				c.Base.Inputs[i].ARec = uint64(0)
				c.Base.Inputs[i].E = uint64(0)
			}
			c.Base.Output.QMass = new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(2))
			c.Base.Output.ARec = uint64(0)
			c.Base.Output.E = uint64(0)
		}},
		{"input-WASTE", true, true, func(c *v2events.MergeCircuit) {
			for i := range c.Base.Inputs {
				c.Base.Inputs[i].AssetRole = uint64(1)
				c.Base.Inputs[i].ARec = uint64(0)
				c.Base.Inputs[i].E = uint64(0)
			}
			c.Base.Output.AssetRole = uint64(1)
			c.Base.Output.ARec = uint64(0)
			c.Base.Output.E = uint64(0)
		}},
		{"cipher-parent-1", false, false, func(c *v2events.MergeCircuit) { c.Audit.Parents[0] = h.Element(8) }},
		{"cipher-parent-2", false, false, func(c *v2events.MergeCircuit) { c.Audit.Parents[1] = h.Element(8) }},
		{"cipher-future-NF", false, false, func(c *v2events.MergeCircuit) { c.Audit.OutputNfs[0] = h.Element(8) }},
		{"R1-X", false, false, func(c *v2events.MergeCircuit) { c.Audit.R1.X = h.Element(8) }},
		{"R1-Y", false, false, func(c *v2events.MergeCircuit) { c.Audit.R1.Y = h.Element(8) }},
	}
	for _, m := range mutations {
		bad := CloneC(c)
		m.mutate(bad)
		if m.rebind {
			if e = Rebind(bad, m.rebuild); e != nil {
				return e
			}
		}
		u, d, e = solved(bad)
		if e != nil {
			return e
		}
		if e = record("witness-reject-"+m.name, !u && !(d[0] && d[1]), map[string]any{"U": u, "T": d[0], "A": d[1], "recommittedAndReencrypted": m.rebind, "rebuiltDepth32Membership": m.rebuild}); e != nil {
			return e
		}
		// Real Prove must reject each malformed full relation and its rejecting module.
		_, ue := prove(keys, bad, false)
		reject := 0
		if d[0] {
			reject = 1
		}
		as := Split(bad)
		w, _ := full(as[reject])
		_, de := plonk.Prove(keys[modules[reject]].CCS, keys[modules[reject]].PK, w)
		if e = record("Prove-reject-"+m.name, ue != nil && de != nil, map[string]any{"rejectingDModule": modules[reject]}); e != nil {
			return e
		}
	}
	// Audit-only alternate opening and secret must fail the EXISTING shared NF.
	for _, kind := range []string{"parent-opening", "secret"} {
		linkBad := CloneC(c)
		if kind == "parent-opening" {
			linkBad.Base.Inputs[0].Opening = h.Element(8)
		} else {
			linkBad.Base.SKOwner = h.Element(8)
		}
		// The changed hidden parent/secret has a valid, freshly recomputed audit
		// ciphertext. Only the NF equality to the transaction's fixed NF links it.
		if e = Rebind(linkBad, false); e != nil {
			return e
		}
		linkBad.Base.NF1, linkBad.Base.NF2 = c.Base.NF1, c.Base.NF2
		a := Split(linkBad)[1].(*AuditLink)
		w, _ := full(a)
		_, pe := plonk.Prove(keys["A"].CCS, keys["A"].PK, w)
		if e = record("cross-module-link-reject-"+kind, keys["A"].CCS.IsSolved(w) != nil && pe != nil, map[string]any{"auditCipherRecomputed": true, "sharedNFUnchanged": true}); e != nil {
			return e
		}
	}
	for i := 0; i < 9; i++ {
		bad := CloneC(c)
		switch i {
		case 0:
			bad.Base.NoteRoot = h.Element(8)
		case 1:
			bad.Base.NF1 = h.Element(8)
		case 2:
			bad.Base.NF2 = h.Element(8)
		case 3:
			bad.Base.CMOut = h.Element(8)
		case 4:
			bad.Audit.R1.X = h.Element(8)
		case 5:
			bad.Audit.R1.Y = h.Element(8)
		case 6:
			bad.Audit.Parents[0] = h.Element(8)
		case 7:
			bad.Audit.Parents[1] = h.Element(8)
		case 8:
			bad.Audit.OutputNfs[0] = h.Element(8)
		}
		bp, _ := public(bad)
		if e = record(fmt.Sprintf("proof-reject-public-%d", i), plonk.Verify(up[0], keys["U"].VK, bp) != nil && VerifyD(keys, dp, bad) != nil, nil); e != nil {
			return e
		}
	}
	pkg, shares, e := v2case.KeyFixture()
	if e != nil {
		return e
	}
	sk, e := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[2]})
	if e != nil {
		return e
	}
	defer sk.SetInt64(0)
	recovered, e := auditcrypto.DecryptWithMasterKey(sk, cc.Ciphertext)
	if e != nil {
		return e
	}
	expected := []fr.Element{cm(c.Base.Inputs[0]), cm(c.Base.Inputs[1]), note.Nullifier(element(c.Base.SKOwner), element(c.Base.CMOut))}
	same := len(recovered) == 3
	for i := range expected {
		same = same && recovered[i].Equal(&expected[i])
	}
	if e = record("U-D-same-cipher-decrypt-actual-parent-and-future-NF", same, nil); e != nil {
		return e
	}
	altParent1, altParent2 := cm(alt.Base.Inputs[0]), cm(alt.Base.Inputs[1])
	fixture := map[string]any{"canonical": scalars(pub), "U": solidity(up[0]), "D": []string{solidity(dp[0]), solidity(dp[1])}, "altCanonical": scalars(altPub), "altU": solidity(altUP[0]), "altD": []string{solidity(altDP[0]), solidity(altDP[1])}, "altParents": []string{altParent1.String(), altParent2.String()}}
	if e = writeJSON(filepath.Join(out, "contracts/test/fixtures/proofs.json"), fixture); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(out, "raw/assignment.json"), c); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(out, "raw/evidence.json"), map[string]any{"publicNames": cc.PublicNames, "public": scalars(pub), "DTransactionPublic": scalars(func() witness.Witness { w, _ := public(PublicSplit(c)[0]); return w }()), "DAuditPublic": scalars(func() witness.Witness { w, _ := public(PublicSplit(c)[1]); return w }()), "auditMessage": expected, "ciphertext": cc.Ciphertext, "recovered": recovered, "sameCipherForUAndD": true}); e != nil {
		return e
	}
	return writeJSON(filepath.Join(out, "validation-native.json"), map[string]any{"environment": Environment(), "checks": rows, "allPassed": true})
}

type BenchRow struct {
	Run, Round, Order                                                            int
	Arm                                                                          string
	Warmup                                                                       bool
	ProveMS, VerifyMS                                                            float64
	IndividualProveMS                                                            []float64
	NativeProofBytes, SolidityProofBytes, PublicInputBytes, CanonicalNativeBytes int
	NativeProofSHA256                                                            []string
	SolidityProofs                                                               []string
}

func Bench(source, out string, run int) error {
	keys, e := Load(out)
	if e != nil {
		return e
	}
	cc, e := CCase(source)
	if e != nil {
		return e
	}
	c := cc.Assignment.(*v2events.MergeCircuit)
	prep := map[string]float64{}
	t := time.Now()
	uw, e := full(c)
	if e != nil {
		return e
	}
	upub, _ := uw.Public()
	prep["U_witness_ms"] = ms(time.Since(t))
	t = time.Now()
	var dw [2]witness.Witness
	for i, a := range Split(c) {
		dw[i], e = full(a)
		if e != nil {
			return e
		}
	}
	prep["D_witness_ms"] = ms(time.Since(t))
	t = time.Now()
	dpub, e := splitPublic(c)
	if e != nil {
		return e
	}
	prep["D_canonical_public_derivation_ms"] = ms(time.Since(t))
	rows := []BenchRow{}
	arms := []string{"U", "D-sequential", "D-parallel-2"}
	for round := -1; round < 10; round++ {
		shift := (round + 1 + run) % 3
		for order := 0; order < 3; order++ {
			arm := arms[(order+shift)%3]
			runtime.GC()
			n := 1
			if arm != "U" {
				n = 2
			}
			ps := make([]plonk.Proof, n)
			times := make([]float64, n)
			errs := make([]error, n)
			work := func(i int) {
				k := keys["U"]
				w := uw
				if n == 2 {
					k = keys[modules[i]]
					w = dw[i]
				}
				st := time.Now()
				ps[i], errs[i] = plonk.Prove(k.CCS, k.PK, w)
				times[i] = ms(time.Since(st))
			}
			st := time.Now()
			if arm == "D-parallel-2" {
				var wg sync.WaitGroup
				for i := 0; i < 2; i++ {
					wg.Add(1)
					go func(j int) { defer wg.Done(); work(j) }(i)
				}
				wg.Wait()
			} else {
				for i := range ps {
					work(i)
				}
			}
			pm := ms(time.Since(st))
			for _, err := range errs {
				if err != nil {
					return err
				}
			}
			st = time.Now()
			for i, p := range ps {
				k := keys["U"]
				pw := upub
				if n == 2 {
					k = keys[modules[i]]
					pw = dpub[i]
				}
				if e = plonk.Verify(p, k.VK, pw); e != nil {
					return e
				}
			}
			vm := ms(time.Since(st))
			row := BenchRow{Run: run, Round: round, Order: order, Arm: arm, Warmup: round == -1, ProveMS: pm, VerifyMS: vm, IndividualProveMS: times, PublicInputBytes: 288}
			for _, p := range ps {
				nb := native(p)
				sp := p.(interface{ MarshalSolidity() []byte }).MarshalSolidity()
				row.NativeProofBytes += len(nb)
				row.SolidityProofBytes += len(sp)
				digest := sha256.Sum256(nb)
				row.NativeProofSHA256 = append(row.NativeProofSHA256, hex.EncodeToString(digest[:]))
				row.SolidityProofs = append(row.SolidityProofs, "0x"+hex.EncodeToString(sp))
			}
			data := NativeSubmission(upub, ps)
			row.CanonicalNativeBytes = len(data)
			if e = os.WriteFile(filepath.Join(out, "raw", fmt.Sprintf("submission-run-%02d-round-%02d-%s.bin", run, round, arm)), data, 0644); e != nil {
				return e
			}
			rows = append(rows, row)
			fmt.Printf("run=%d round=%d arm=%s prove=%.3f verify=%.3f ms\n", run, round, arm, pm, vm)
		}
	}
	return writeJSON(filepath.Join(out, "raw", fmt.Sprintf("native-run-%02d.json", run)), map[string]any{"environment": Environment(), "witnessPreparation": prep, "rows": rows, "canonical": scalars(upub)})
}
