package v2run

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	curve "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	kzg "github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
)

type RelationResult struct {
	Name            string            `json:"name"`
	PublicInputs    int               `json:"publicInputs"`
	Constraints     int               `json:"constraints"`
	DomainSize      int               `json:"domainSize"`
	CanonicalPoints int               `json:"canonicalPoints"`
	PublicNames     []string          `json:"publicNames"`
	CompileMillis   float64           `json:"compileMillis"`
	SetupMillis     float64           `json:"setupMillis"`
	ProveMillis     float64           `json:"proveMillis"`
	VerifyMillis    float64           `json:"verifyMillis"`
	ProofBytes      int               `json:"proofBytes"`
	Files           map[string]string `json:"files,omitempty"`
}

type CircuitResult struct {
	Profile              string           `json:"profile"`
	Curve                string           `json:"curve"`
	ProofSystem          string           `json:"proofSystem"`
	UniversalSRSChecksum string           `json:"universalSRSChecksum"`
	UniversalPoints      int              `json:"universalPoints"`
	MaxDomain            int              `json:"maxDomain"`
	Relations            []RelationResult `json:"relations"`
	Attempts             []map[string]any `json:"attempts,omitempty"`
}

type compiled struct {
	item              v2case.Case
	ccs               constraint.ConstraintSystem
	domain, canonical int
	compileMS         float64
}

func Setup(root string) (CircuitResult, error) {
	if _, e := os.Stat(filepath.Join(root, "artifacts/development/m1-hotfix/srs/universal-canonical.bin")); e == nil {
		return CircuitResult{}, fmt.Errorf("Hotfix SRS exists; refusing replacement")
	}
	if _, e := os.Stat(filepath.Join(root, "output/m1-hotfix-circuit.json")); e == nil {
		return CircuitResult{}, fmt.Errorf("Hotfix setup already exists")
	}
	suite, err := v2case.Build(root)
	if err != nil {
		return CircuitResult{}, err
	}
	if err = suite.Validate(); err != nil {
		return CircuitResult{}, err
	}
	keyDir := filepath.Join(root, "artifacts/development/m1-hotfix/key-package")
	if err = os.MkdirAll(keyDir, 0700); err != nil {
		return CircuitResult{}, err
	}
	public := map[string]any{"profile": suite.Package.Profile, "sessionId": suite.Package.SessionID, "threshold": suite.Package.Threshold, "memberCount": suite.Package.MemberCount, "publicKeyX": auditcrypto.EncodeField(suite.Package.PublicKey.Point.X), "publicKeyY": auditcrypto.EncodeField(suite.Package.PublicKey.Point.Y)}
	if err = artifact.WriteJSON(filepath.Join(keyDir, "public.json"), public); err != nil {
		return CircuitResult{}, err
	}
	for _, share := range suite.Shares {
		path := filepath.Join(keyDir, fmt.Sprintf("member-%d.share.json", share.ID))
		raw := []byte(fmt.Sprintf("{\n  \"sessionId\": %q,\n  \"memberId\": %d,\n  \"share\": %q\n}\n", suite.Package.SessionID, share.ID, share.Value.String()))
		if err = os.WriteFile(path, raw, 0600); err != nil {
			return CircuitResult{}, err
		}
		if err = os.Chmod(path, 0600); err != nil {
			return CircuitResult{}, err
		}
	}
	cs := make([]compiled, len(suite.Cases))
	maxCanonical, maxDomain := 0, 0
	for i, item := range suite.Cases {
		start := time.Now()
		ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.Definition)
		if e != nil {
			return CircuitResult{}, e
		}
		if ccs.GetNbPublicVariables() != len(item.PublicNames) {
			return CircuitResult{}, fmt.Errorf("%s public inputs", item.Name)
		}
		canonical, domain := plonk.SRSSize(ccs)
		if canonical > maxCanonical {
			maxCanonical = canonical
		}
		if domain > maxDomain {
			maxDomain = domain
		}
		cs[i] = compiled{item, ccs, domain, canonical, millis(time.Since(start))}
	}
	tau, err := rand.Int(rand.Reader, fr.Modulus())
	if err != nil {
		return CircuitResult{}, err
	}
	if tau.Sign() == 0 {
		tau = big.NewInt(1)
	}
	canonicalSRS, err := kzg.NewSRS(uint64(maxCanonical), tau)
	tau.SetInt64(0)
	if err != nil {
		return CircuitResult{}, err
	}
	srsDir := filepath.Join(root, "artifacts/development/m1-hotfix/srs")
	if err = os.MkdirAll(srsDir, 0755); err != nil {
		return CircuitResult{}, err
	}
	canonicalPath := filepath.Join(srsDir, "universal-canonical.bin")
	if err = write(canonicalPath, canonicalSRS); err != nil {
		return CircuitResult{}, err
	}
	canonicalHash, err := artifact.Checksum(canonicalPath)
	if err != nil {
		return CircuitResult{}, err
	}
	lagranges := map[int]*kzg.SRS{}
	for _, c := range cs {
		if lagranges[c.domain] != nil {
			continue
		}
		points := append([]curve.G1Affine(nil), canonicalSRS.Pk.G1[:c.domain]...)
		lg, e := kzg.ToLagrangeG1(points)
		if e != nil {
			return CircuitResult{}, e
		}
		lagranges[c.domain] = &kzg.SRS{Pk: kzg.ProvingKey{G1: lg}, Vk: canonicalSRS.Vk}
		if e = write(filepath.Join(srsDir, fmt.Sprintf("lagrange-%d.bin", c.domain)), lagranges[c.domain]); e != nil {
			return CircuitResult{}, e
		}
	}
	result := CircuitResult{Profile: "zkDPP-v2-M1-HF", Curve: "BLS12-381/Jubjub", ProofSystem: "PLONK-KZG", UniversalSRSChecksum: canonicalHash, UniversalPoints: maxCanonical, MaxDomain: maxDomain}
	for _, c := range cs {
		start := time.Now()
		pk, vk, e := plonk.Setup(c.ccs, canonicalSRS, lagranges[c.domain])
		if e != nil {
			return result, e
		}
		setupMS := millis(time.Since(start))
		dir := filepath.Join(root, "artifacts/development/m1-hotfix/circuits", c.item.Name)
		if e = os.MkdirAll(dir, 0755); e != nil {
			return result, e
		}
		files := map[string]string{}
		for name, value := range map[string]io.WriterTo{"ccs.bin": c.ccs, "proving.key": pk, "verifying.key": vk} {
			path := filepath.Join(dir, name)
			if e = write(path, value); e != nil {
				return result, e
			}
			files[name], e = artifact.Checksum(path)
			if e != nil {
				return result, e
			}
		}
		if e = artifact.ExportSolidity(filepath.Join(root, "contracts/src/generated/v2-m1-hotfix", c.item.Name, "PlonkVerifier.sol"), vk); e != nil {
			return result, e
		}
		result.Relations = append(result.Relations, RelationResult{Name: c.item.Name, PublicInputs: len(c.item.PublicNames), Constraints: c.ccs.GetNbConstraints(), DomainSize: c.domain, CanonicalPoints: c.canonical, PublicNames: c.item.PublicNames, CompileMillis: c.compileMS, SetupMillis: setupMS, Files: files})
		if e = artifact.WriteJSON(filepath.Join(dir, "manifest.json"), result.Relations[len(result.Relations)-1]); e != nil {
			return result, e
		}
	}
	if err = artifact.WriteJSON(filepath.Join(srsDir, "manifest.json"), map[string]any{"profile": result.Profile, "developmentOnly": true, "externalDKGAssumed": true, "points": maxCanonical, "maxDomain": maxDomain, "checksum": canonicalHash}); err != nil {
		return result, err
	}
	return result, artifact.WriteJSON(filepath.Join(root, "output/m1-hotfix-circuit.json"), result)
}

type FixedProof struct {
	Name         string   `json:"name"`
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}
type Fixture struct {
	Profile, PublicKeyX, PublicKeyY string
	Proofs                          []FixedProof
}

func Evaluate(root string) (CircuitResult, error) {
	if _, e := os.Stat(filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json")); e == nil {
		return CircuitResult{}, fmt.Errorf("Hotfix evaluation already exists")
	}
	suite, err := v2case.Build(root)
	if err != nil {
		return CircuitResult{}, err
	}
	var result CircuitResult
	if err = readJSON(filepath.Join(root, "output/m1-hotfix-circuit.json"), &result); err != nil {
		return result, err
	}
	fixture := Fixture{Profile: "zkDPP-v2-M1-HF", PublicKeyX: auditcrypto.EncodeField(suite.Package.PublicKey.Point.X), PublicKeyY: auditcrypto.EncodeField(suite.Package.PublicKey.Point.Y)}
	for i, item := range suite.Cases {
		dir := filepath.Join(root, "artifacts/development/m1-hotfix/circuits", item.Name)
		ccs := plonk.NewCS(ecc.BLS12_381)
		pk := plonk.NewProvingKey(ecc.BLS12_381)
		vk := plonk.NewVerifyingKey(ecc.BLS12_381)
		for name, target := range map[string]io.ReaderFrom{"ccs.bin": ccs, "proving.key": pk, "verifying.key": vk} {
			got, e := artifact.Checksum(filepath.Join(dir, name))
			if e != nil {
				return result, e
			}
			if got != result.Relations[i].Files[name] {
				return result, fmt.Errorf("artifact checksum mismatch %s", name)
			}
			if err = read(filepath.Join(dir, name), target); err != nil {
				return result, err
			}
		}
		w, e := frontend.NewWitness(item.Assignment, ecc.BLS12_381.ScalarField())
		if e != nil {
			return result, e
		}
		public, e := w.Public()
		if e != nil {
			return result, e
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		start := time.Now()
		proof, e := plonk.Prove(ccs, pk, w)
		result.Relations[i].ProveMillis = millis(time.Since(start))
		runtime.ReadMemStats(&after)
		_ = after.TotalAlloc - before.TotalAlloc
		if e != nil {
			return result, e
		}
		start = time.Now()
		e = plonk.Verify(proof, vk, public)
		result.Relations[i].VerifyMillis = millis(time.Since(start))
		if e != nil {
			return result, e
		}
		var proofBuffer bytes.Buffer
		_, _ = proof.WriteTo(&proofBuffer)
		result.Relations[i].ProofBytes = proofBuffer.Len()
		sp, ok := proof.(interface{ MarshalSolidity() []byte })
		if !ok {
			return result, fmt.Errorf("%s proof lacks Solidity encoding", item.Name)
		}
		fixed := FixedProof{Name: item.Name, Proof: "0x" + hex.EncodeToString(sp.MarshalSolidity())}
		for _, v := range []fr.Element(public.Vector().(fr.Vector)) {
			fixed.PublicInputs = append(fixed.PublicInputs, auditcrypto.EncodeField(v))
		}
		fixture.Proofs = append(fixture.Proofs, fixed)
	}
	if err = artifact.WriteJSON(filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json"), fixture); err != nil {
		return result, err
	}
	return result, artifact.WriteJSON(filepath.Join(root, "output/m1-hotfix-circuit.json"), result)
}

func write(path string, v io.WriterTo) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	_, e = v.WriteTo(f)
	closeErr := f.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func read(path string, v io.ReaderFrom) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	_, e = v.ReadFrom(f)
	closeErr := f.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func millis(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
