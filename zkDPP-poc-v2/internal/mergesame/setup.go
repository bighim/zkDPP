package mergesame

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	cu "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
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
	defs := map[string]frontend.Circuit{"T": Definition("T", c.Definition.(*v2events.MergeCircuit).CommitteePK), "A": Definition("A", c.Definition.(*v2events.MergeCircuit).CommitteePK), "U": c.Definition}
	canonical := new(kzg.SRS)
	if e = readBin(filepath.Join(oldSetup, "artifacts/development/m1-hotfix/srs/universal-canonical.bin"), canonical); e != nil {
		return e
	}
	lagranges := map[int]*kzg.SRS{}
	rows := []SetupRow{}
	for _, name := range []string{"T", "A", "U"} {
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
		if name == "U" {
			old := filepath.Join(oldSetup, "artifacts/development/m1-hotfix/circuits/merge")
			var b bytes.Buffer
			ccs.WriteTo(&b)
			prior, e := os.ReadFile(filepath.Join(old, "ccs.bin"))
			if e != nil {
				return e
			}
			if !bytes.Equal(b.Bytes(), prior) {
				return fmt.Errorf("U compiled CCS differs from pinned setup")
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
	for _, name := range []string{"T", "A", "U"} {
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
