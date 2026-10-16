package mergecompare

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	cu "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	curve "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Key struct {
	CCS constraint.ConstraintSystem
	PK  plonk.ProvingKey
	VK  plonk.VerifyingKey
}
type SetupRow struct {
	Name                                               string
	Constraints, PublicInputs, Domain, CanonicalPoints int
	CompileMS, SetupMS, LoadMS                         float64
	Reused                                             bool
	Bytes                                              map[string]int64
	SHA256                                             map[string]string
}
type Env struct {
	Go, OS, Arch                             string
	GOMAXPROCS, NumCPU                       int
	Gnark, GnarkCrypto, Curve, Backend, Hash string
}

func Environment() Env {
	return Env{runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), runtime.NumCPU(), "v0.15.0", "v0.20.1", "BLS12-381", "PLONK-KZG", "Poseidon2"}
}
func writeBin(p string, w io.WriterTo) error {
	f, e := os.Create(p)
	if e != nil {
		return e
	}
	_, e = w.WriteTo(f)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func readBin(p string, r io.ReaderFrom) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = r.ReadFrom(f)
	return e
}
func ms(t time.Duration) float64 { return float64(t.Nanoseconds()) / 1e6 }
func public(c frontend.Circuit) (witness.Witness, error) {
	return frontend.NewWitness(c, ecc.BLS12_381.ScalarField(), frontend.PublicOnly())
}
func full(c frontend.Circuit) (witness.Witness, error) {
	return frontend.NewWitness(c, ecc.BLS12_381.ScalarField())
}
func writeJSON(p string, v any) error { return cu.WriteJSON(p, v) }
func CCase(root string) (v2case.Case, error) {
	suite, e := v2case.Build(root)
	if e != nil {
		return v2case.Case{}, e
	}
	if e = suite.Validate(); e != nil {
		return v2case.Case{}, e
	}
	for _, c := range suite.Cases {
		if c.Name == "merge" {
			return c, nil
		}
	}
	return v2case.Case{}, fmt.Errorf("missing Merge")
}
func Prepare(source, oldSetup, out string) error {
	if _, e := os.Stat(filepath.Join(out, "setup.json")); e == nil {
		return fmt.Errorf("setup already exists")
	}
	c, e := CCase(source)
	if e != nil {
		return e
	}
	defs := map[string]frontend.Circuit{"path": Definition("path"), "eq": Definition("eq"), "add": Definition("add"), "B": Definition("B"), "C": c.Definition}
	canonical := new(kzg.SRS)
	if e = readBin(filepath.Join(oldSetup, "artifacts/development/m1-hotfix/srs/universal-canonical.bin"), canonical); e != nil {
		return e
	}
	lagranges := map[int]*kzg.SRS{}
	rows := []SetupRow{}
	for _, name := range []string{"path", "eq", "add", "B", "C"} {
		t := time.Now()
		ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, defs[name])
		compileMS := ms(time.Since(t))
		if e != nil {
			return e
		}
		req, domain := plonk.SRSSize(ccs)
		row := SetupRow{Name: name, Constraints: ccs.GetNbConstraints(), PublicInputs: ccs.GetNbPublicVariables(), Domain: domain, CanonicalPoints: req, CompileMS: compileMS, Bytes: map[string]int64{}, SHA256: map[string]string{}}
		dir := filepath.Join(out, "keys", name)
		os.MkdirAll(dir, 0755)
		var pk plonk.ProvingKey
		var vk plonk.VerifyingKey
		if name == "C" {
			old := filepath.Join(oldSetup, "artifacts/development/m1-hotfix/circuits/merge")
			var b bytes.Buffer
			ccs.WriteTo(&b)
			prior, e := os.ReadFile(filepath.Join(old, "ccs.bin"))
			if e != nil {
				return e
			}
			if !bytes.Equal(b.Bytes(), prior) {
				return fmt.Errorf("C compiled CCS differs from pinned setup")
			}
			t = time.Now()
			pk = plonk.NewProvingKey(ecc.BLS12_381)
			vk = plonk.NewVerifyingKey(ecc.BLS12_381)
			if e = readBin(filepath.Join(old, "proving.key"), pk); e != nil {
				return e
			}
			if e = readBin(filepath.Join(old, "verifying.key"), vk); e != nil {
				return e
			}
			row.Reused = true
			row.LoadMS = ms(time.Since(t))
		} else {
			if lagranges[domain] == nil {
				t = time.Now()
				points := append([]curve.G1Affine(nil), canonical.Pk.G1[:domain]...)
				lg, e := kzg.ToLagrangeG1(points)
				if e != nil {
					return e
				}
				lagranges[domain] = &kzg.SRS{Pk: kzg.ProvingKey{G1: lg}, Vk: canonical.Vk}
				writeJSON(filepath.Join(out, "keys", fmt.Sprintf("lagrange-%d.json", domain)), map[string]any{"domain": domain, "deriveMS": ms(time.Since(t)), "sameCanonicalSRS": true})
			}
			t = time.Now()
			pk, vk, e = plonk.Setup(ccs, canonical, lagranges[domain])
			row.SetupMS = ms(time.Since(t))
			if e != nil {
				return e
			}
		}
		for fname, obj := range map[string]io.WriterTo{"ccs.bin": ccs, "proving.key": pk, "verifying.key": vk} {
			p := filepath.Join(dir, fname)
			if e = writeBin(p, obj); e != nil {
				return e
			}
			inf, _ := os.Stat(p)
			row.Bytes[fname] = inf.Size()
			row.SHA256[fname], e = cu.Checksum(p)
			if e != nil {
				return e
			}
		}
		if e = cu.ExportSolidity(filepath.Join(out, "contracts/src/generated", name, "PlonkVerifier.sol"), vk); e != nil {
			return e
		}
		if e = writeJSON(filepath.Join(dir, "manifest.json"), row); e != nil {
			return e
		}
		rows = append(rows, row)
		fmt.Printf("prepared %s constraints=%d domain=%d PI=%d\n", name, row.Constraints, domain, row.PublicInputs)
	}
	sum, e := cu.Checksum(filepath.Join(oldSetup, "artifacts/development/m1-hotfix/srs/universal-canonical.bin"))
	if e != nil {
		return e
	}
	return writeJSON(filepath.Join(out, "setup.json"), map[string]any{"environment": Environment(), "rows": rows, "canonicalSRSChecksum": sum, "sourceRevision": "483f4a0280d832f95a6842997ca89e35bcbb0879", "sourceRoot": source, "reusedSetup": oldSetup, "developmentOnly": true})
}
func Load(out string) (map[string]*Key, error) {
	keys := map[string]*Key{}
	for _, name := range []string{"path", "eq", "add", "B", "C"} {
		k := &Key{plonk.NewCS(ecc.BLS12_381), plonk.NewProvingKey(ecc.BLS12_381), plonk.NewVerifyingKey(ecc.BLS12_381)}
		var row SetupRow
		b, e := os.ReadFile(filepath.Join(out, "keys", name, "manifest.json"))
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &row); e != nil {
			return nil, e
		}
		for fname, obj := range map[string]io.ReaderFrom{"ccs.bin": k.CCS, "proving.key": k.PK, "verifying.key": k.VK} {
			p := filepath.Join(out, "keys", name, fname)
			sum, e := cu.Checksum(p)
			if e != nil || sum != row.SHA256[fname] {
				return nil, fmt.Errorf("hash mismatch %s/%s", name, fname)
			}
			if e = readBin(p, obj); e != nil {
				return nil, e
			}
		}
		keys[name] = k
	}
	return keys, nil
}

var slotNames = [6]string{"path", "path", "path", "eq", "eq", "add"}

func AtomicPublic(s Canonical) ([6]witness.Witness, error) {
	var w [6]witness.Witness
	for i, c := range PublicAtomic(s) {
		v, e := public(c)
		if e != nil {
			return w, e
		}
		w[i] = v
	}
	return w, nil
}
func VerifyAtomic(keys map[string]*Key, proofs []plonk.Proof, s Canonical) error {
	if len(proofs) != 6 {
		return fmt.Errorf("proof count")
	}
	ws, e := AtomicPublic(s)
	if e != nil {
		return e
	}
	for i, p := range proofs {
		if e = plonk.Verify(p, keys[slotNames[i]].VK, ws[i]); e != nil {
			return fmt.Errorf("slot %d: %w", i, e)
		}
	}
	return nil
}
func makeProofs(keys map[string]*Key, tx Transaction) ([]plonk.Proof, error) {
	ps := make([]plonk.Proof, 6)
	for i, c := range tx.Bundle() {
		w, e := full(c)
		if e != nil {
			return nil, e
		}
		ps[i], e = plonk.Prove(keys[slotNames[i]].CCS, keys[slotNames[i]].PK, w)
		if e != nil {
			return nil, e
		}
	}
	return ps, nil
}
func scalars(w witness.Witness) []string {
	v := w.Vector().(fr.Vector)
	o := make([]string, len(v))
	for i := range v {
		o[i] = v[i].String()
	}
	return o
}
func solidity(p plonk.Proof) string {
	return "0x" + hex.EncodeToString(p.(interface{ MarshalSolidity() []byte }).MarshalSolidity())
}
func native(p plonk.Proof) []byte { var b bytes.Buffer; p.WriteTo(&b); return b.Bytes() }
func CloneC(c *v2events.MergeCircuit) *v2events.MergeCircuit {
	o := *c
	o.Audit.Parents = append([]frontend.Variable(nil), c.Audit.Parents...)
	o.Audit.OutputNfs = append([]frontend.Variable(nil), c.Audit.OutputNfs...)
	return &o
}
func Check(source, out string) error {
	keys, e := Load(out)
	if e != nil {
		return e
	}
	rows := []map[string]any{}
	check := func(name string, ok bool, detail any) error {
		rows = append(rows, map[string]any{"name": name, "passed": ok, "detail": detail})
		if !ok {
			return fmt.Errorf("failed check %s: %v", name, detail)
		}
		return nil
	}
	tx := Fixture(10000)
	// Solver correspondence is checked against independent gadget assignments;
	// B literally calls exactly these relations, including their private inputs.
	for _, name := range []string{"valid", "wrong-recipient", "wrong-quantity", "item-code-not-constrained", "zero-quantity", "field-wrap"} {
		t := tx
		switch name {
		case "wrong-recipient":
			d := t.Documents[0]
			d.Values[1] = fr.NewElement(301)
			t.Documents[0] = MakeDocument(d.Values, 40000)
		case "wrong-quantity":
			d := t.Documents[2]
			d.Values[3] = fr.NewElement(100e9 + 1)
			t.Documents[2] = MakeDocument(d.Values, 50000)
		case "item-code-not-constrained":
			d := t.Documents[2]
			d.Values[2] = fr.NewElement(9999)
			t.Documents[2] = MakeDocument(d.Values, 60000)
		case "zero-quantity":
			for i := range t.Documents {
				d := t.Documents[i]
				d.Values[3] = fr.Element{}
				t.Documents[i] = MakeDocument(d.Values, 70000+uint64(i)*100)
			}
		case "field-wrap":
			var neg fr.Element
			neg.SetInt64(-1)
			vals := [3]fr.Element{neg, fr.NewElement(2), fr.NewElement(1)}
			for i := range t.Documents {
				d := t.Documents[i]
				d.Values[3] = vals[i]
				t.Documents[i] = MakeDocument(d.Values, 80000+uint64(i)*100)
			}
		}
		aOK := true
		for i, c := range t.Bundle() {
			w, e := full(c)
			if e != nil {
				return e
			}
			if keys[slotNames[i]].CCS.IsSolved(w) != nil {
				aOK = false
			}
		}
		bw, e := full(t.Combined())
		if e != nil {
			return e
		}
		bOK := keys["B"].CCS.IsSolved(bw) == nil
		want := name != "wrong-recipient" && name != "wrong-quantity"
		if e = check("AB-relation-"+name, aOK == bOK && bOK == want, map[string]any{"A": aOK, "B": bOK, "expected": want}); e != nil {
			return e
		}
	}
	pa, e := makeProofs(keys, tx)
	if e != nil {
		return e
	}
	if e = check("A-native-valid", VerifyAtomic(keys, pa, tx.Canonical()) == nil, nil); e != nil {
		return e
	}
	alt := Fixture(90000)
	pb, e := makeProofs(keys, alt)
	if e != nil {
		return e
	}
	if e = check("A-alt-native-valid", VerifyAtomic(keys, pb, alt.Canonical()) == nil, nil); e != nil {
		return e
	}
	for i := range pa {
		mixed := append([]plonk.Proof(nil), pa...)
		mixed[i] = pb[i]
		if e = check(fmt.Sprintf("A-mixed-proof-slot-%d", i), VerifyAtomic(keys, mixed, tx.Canonical()) != nil, nil); e != nil {
			return e
		}
	}
	bw, e := full(tx.Combined())
	if e != nil {
		return e
	}
	bpublic, _ := bw.Public()
	bproof, e := plonk.Prove(keys["B"].CCS, keys["B"].PK, bw)
	if e != nil {
		return e
	}
	if e = check("B-native-valid", plonk.Verify(bproof, keys["B"].VK, bpublic) == nil, nil); e != nil {
		return e
	}
	for _, i := range []int{0, 4, 8, 14} {
		st := tx.Canonical()
		flat := append(append([]frontend.Variable{}, st.Roots[:]...), st.Commitments[0][:]...)
		_ = flat
		if i < 3 {
			st.Roots[i] = fr.NewElement(7)
		} else {
			st.Commitments[(i-3)/4][(i-3)%4] = fr.NewElement(7)
		}
		bad, _ := public(&Conjunction{Statement: st})
		if e = check(fmt.Sprintf("AB-wrong-public-%d", i), VerifyAtomic(keys, pa, st) != nil && plonk.Verify(bproof, keys["B"].VK, bad) != nil, nil); e != nil {
			return e
		}
	}
	cc, e := CCase(source)
	if e != nil {
		return e
	}
	ca := cc.Assignment.(*v2events.MergeCircuit)
	cw, e := full(ca)
	if e != nil {
		return e
	}
	cpub, _ := cw.Public()
	cp, e := plonk.Prove(keys["C"].CCS, keys["C"].PK, cw)
	if e != nil {
		return e
	}
	if e = check("C-native-valid", plonk.Verify(cp, keys["C"].VK, cpub) == nil, nil); e != nil {
		return e
	}
	mutations := []struct {
		name   string
		mutate func(*v2events.MergeCircuit)
	}{
		{"output-mass", func(c *v2events.MergeCircuit) { c.Base.Output.QMass = uint64(100e9 + 1) }},
		{"output-credit", func(c *v2events.MergeCircuit) { c.Base.Output.ARec = uint64(20e9 + 1) }},
		{"output-carbon", func(c *v2events.MergeCircuit) { c.Base.Output.E = uint64(80e9 + 1) }},
		{"different-document", func(c *v2events.MergeCircuit) { c.Base.Output.DocumentHash = fr.NewElement(8) }},
		{"wrong-owner", func(c *v2events.MergeCircuit) { c.Base.SKOwner = fr.NewElement(8) }},
		{"duplicate-input", func(c *v2events.MergeCircuit) {
			c.Base.Inputs[1] = c.Base.Inputs[0]
			c.Base.Paths[1] = c.Base.Paths[0]
			c.Base.NF2 = c.Base.NF1
		}},
		{"wrong-parent-cipher", func(c *v2events.MergeCircuit) { c.Audit.Parents[0] = fr.NewElement(8) }},
		{"wrong-future-nf-cipher", func(c *v2events.MergeCircuit) { c.Audit.OutputNfs[0] = fr.NewElement(8) }},
		{"wrong-R1", func(c *v2events.MergeCircuit) { c.Audit.R1.X = fr.NewElement(8) }},
	}
	for _, m := range mutations {
		bad := CloneC(ca)
		m.mutate(bad)
		// Recommit/re-encrypt invalid output values, so these cases fail the
		// conservation/document rule rather than merely an unchanged CMOut.
		if m.name == "output-mass" || m.name == "output-credit" || m.name == "output-carbon" || m.name == "different-document" {
			o := bad.Base.Output
			cm := h.Hash(h.NoteTag, o.DocumentHash.(fr.Element), h.Element(o.AssetRole.(uint64)), h.Element(o.QMass.(uint64)), h.Element(o.ARec.(uint64)), h.Element(o.E.(uint64)), o.Address.(fr.Element), o.Opening.(fr.Element))
			bad.Base.CMOut = cm
			ct, e := auditcrypto.EncryptWithMasterPublicKey(bad.CommitteePK, []fr.Element{cc.Message[0], cc.Message[1], note.Nullifier(bad.Base.SKOwner.(fr.Element), cm)}, big.NewInt(42))
			if e != nil {
				return e
			}
			bad.Audit.Randomness = big.NewInt(42)
			bad.Audit.R1.X = ct.R1.X
			bad.Audit.R1.Y = ct.R1.Y
			bad.Audit.Parents[0] = ct.Data[0]
			bad.Audit.Parents[1] = ct.Data[1]
			bad.Audit.OutputNfs[0] = ct.Data[2]
		}
		w, e := full(bad)
		if e != nil {
			return e
		}
		if e = check("C-witness-reject-"+m.name, keys["C"].CCS.IsSolved(w) != nil, nil); e != nil {
			return e
		}
		if m.name == "wrong-parent-cipher" {
			_, e = plonk.Prove(keys["C"].CCS, keys["C"].PK, w)
			if e = check("C-Prove-reject-audit", e != nil, nil); e != nil {
				return e
			}
		}
	}
	for i := 0; i < 9; i++ {
		v := append(fr.Vector(nil), cpub.Vector().(fr.Vector)...)
		v[i] = fr.NewElement(8)
		bad, e := witness.New(ecc.BLS12_381.ScalarField())
		if e != nil {
			return e
		}
		ch := make(chan any, len(v))
		for _, x := range v {
			ch <- x
		}
		close(ch)
		if e = bad.Fill(len(v), 0, ch); e != nil {
			return e
		}
		if e = check(fmt.Sprintf("C-proof-reject-public-%d", i), plonk.Verify(cp, keys["C"].VK, bad) != nil, nil); e != nil {
			return e
		}
	}
	badTx := tx
	d := badTx.Documents[2]
	d.Values[3] = fr.NewElement(100e9 + 1)
	badTx.Documents[2] = MakeDocument(d.Values, 50000)
	aw, _ := full(badTx.Bundle()[5])
	_, ae := plonk.Prove(keys["add"].CCS, keys["add"].PK, aw)
	badBW, _ := full(badTx.Combined())
	_, be := plonk.Prove(keys["B"].CCS, keys["B"].PK, badBW)
	if e = check("AB-Prove-reject-quantity", ae != nil && be != nil, nil); e != nil {
		return e
	}
	sa := make([]string, 6)
	sb := make([]string, 6)
	for i := range pa {
		sa[i] = solidity(pa[i])
		sb[i] = solidity(pb[i])
	}
	fixture := map[string]any{"canonical": scalars(bpublic), "A": sa, "altCanonical": func() []string { w, _ := public(alt.Combined()); return scalars(w) }(), "altA": sb, "B": solidity(bproof), "C": solidity(cp), "publicC": scalars(cpub)}
	if e = writeJSON(filepath.Join(out, "contracts/test/fixtures/proofs.json"), fixture); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(out, "raw/C-assignment.json"), ca); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(out, "raw/C-evidence.json"), map[string]any{"publicNames": cc.PublicNames, "public": scalars(cpub), "auditMessage": cc.Message, "ciphertext": cc.Ciphertext, "decryptValidated": true, "assignmentType": fmt.Sprintf("%T", ca)}); e != nil {
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
	tx := Fixture(10000)
	assign := tx.Bundle()
	aws := make([]witness.Witness, 6)
	prep := map[string]float64{}
	t := time.Now()
	for i, c := range assign {
		aws[i], e = full(c)
		if e != nil {
			return e
		}
	}
	prep["A_full_witness_ms"] = ms(time.Since(t))
	t = time.Now()
	apubs, e := AtomicPublic(tx.Canonical())
	if e != nil {
		return e
	}
	prep["A_canonical_public_derivation_ms"] = ms(time.Since(t))
	t = time.Now()
	bw, e := full(tx.Combined())
	if e != nil {
		return e
	}
	bpub, _ := bw.Public()
	prep["B_witness_ms"] = ms(time.Since(t))
	cc, e := CCase(source)
	if e != nil {
		return e
	}
	t = time.Now()
	cw, e := full(cc.Assignment)
	if e != nil {
		return e
	}
	cpub, _ := cw.Public()
	prep["C_witness_ms"] = ms(time.Since(t))
	var rows []BenchRow
	arms := []string{"A-sequential", "A-parallel-2", "B", "C"}
	for round := -1; round < 10; round++ {
		shift := (round + 1 + run) % 4
		for order := 0; order < 4; order++ {
			arm := arms[(order+shift)%4]
			runtime.GC()
			n := 1
			if arm[0] == 'A' {
				n = 6
			}
			ps := make([]plonk.Proof, n)
			times := make([]float64, n)
			errs := make([]error, n)
			prove := func(i int) {
				name := "B"
				w := bw
				if arm[0] == 'A' {
					name = slotNames[i]
					w = aws[i]
				} else if arm == "C" {
					name = "C"
					w = cw
				}
				st := time.Now()
				ps[i], errs[i] = plonk.Prove(keys[name].CCS, keys[name].PK, w)
				times[i] = ms(time.Since(st))
			}
			st := time.Now()
			if arm == "A-parallel-2" {
				var wg sync.WaitGroup
				jobs := make(chan int, n)
				for j := 0; j < 2; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for i := range jobs {
							prove(i)
						}
					}()
				}
				for i := 0; i < n; i++ {
					jobs <- i
				}
				close(jobs)
				wg.Wait()
			} else {
				for i := 0; i < n; i++ {
					prove(i)
				}
			}
			proveMS := ms(time.Since(st))
			for _, e := range errs {
				if e != nil {
					return e
				}
			}
			st = time.Now()
			for i, p := range ps {
				name := "B"
				pwi := bpub
				if arm[0] == 'A' {
					name = slotNames[i]
					pwi = apubs[i]
				} else if arm == "C" {
					name = "C"
					pwi = cpub
				}
				if e = plonk.Verify(p, keys[name].VK, pwi); e != nil {
					return e
				}
			}
			verifyMS := ms(time.Since(st))
			publics := bpub
			if arm == "C" {
				publics = cpub
			}
			canonicalNative := 1 + 4 + len(scalars(publics))*32 + 4
			row := BenchRow{Run: run, Round: round, Order: order, Arm: arm, Warmup: round == -1, ProveMS: proveMS, VerifyMS: verifyMS, IndividualProveMS: times, PublicInputBytes: len(scalars(publics)) * 32}
			for _, p := range ps {
				nb := native(p)
				sp := p.(interface{ MarshalSolidity() []byte }).MarshalSolidity()
				row.NativeProofBytes += len(nb)
				row.SolidityProofBytes += len(sp)
				canonicalNative += 4 + len(nb)
				hash := sha256.Sum256(nb)
				row.NativeProofSHA256 = append(row.NativeProofSHA256, hex.EncodeToString(hash[:]))
				row.SolidityProofs = append(row.SolidityProofs, "0x"+hex.EncodeToString(sp))
			}
			row.CanonicalNativeBytes = canonicalNative
			actualSubmission := NativeSubmission(publics, ps)
			if len(actualSubmission) != canonicalNative {
				return fmt.Errorf("native serialization length mismatch")
			}
			if e = os.WriteFile(filepath.Join(out, "raw", fmt.Sprintf("submission-run-%02d-round-%02d-%s.bin", run, round, arm)), actualSubmission, 0644); e != nil {
				return e
			}
			row.CanonicalNativeBytes = len(actualSubmission)
			rows = append(rows, row)
			fmt.Printf("run=%d round=%d arm=%s prove=%.3f verify=%.3f ms\n", run, round, arm, proveMS, verifyMS)
		}
	}
	return writeJSON(filepath.Join(out, "raw", fmt.Sprintf("native-run-%02d.json", run)), map[string]any{"environment": Environment(), "witnessPreparation": prep, "rows": rows, "canonicalAB": scalars(bpub), "publicC": scalars(cpub)})
}

// Native canonical serialization definition is independent of gnark's witness
// header and sends each shared scalar once. Bench sizes are measured from it.
func NativeSubmission(pub witness.Witness, ps []plonk.Proof) []byte {
	var b bytes.Buffer
	b.WriteByte(1)
	binary.Write(&b, binary.BigEndian, uint32(len(scalars(pub))))
	for _, s := range scalars(pub) {
		x, _ := new(big.Int).SetString(s, 10)
		v := make([]byte, 32)
		x.FillBytes(v)
		b.Write(v)
	}
	binary.Write(&b, binary.BigEndian, uint32(len(ps)))
	for _, p := range ps {
		v := native(p)
		binary.Write(&b, binary.BigEndian, uint32(len(v)))
		b.Write(v)
	}
	return b.Bytes()
}
