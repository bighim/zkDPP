package experiments

import (
	"crypto/rand"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	kzg "github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Relation struct {
	Profile             Profile
	PublicNames         []string
	Constraints, Domain int
	CompileMS, SetupMS  float64
	Files               map[string]string
}
type SetupResult struct {
	Relations         []Relation
	CanonicalPoints   int
	CanonicalChecksum string
	SRSMS             float64
	LagrangeMS        map[int]float64
	DevelopmentOnly   bool
}

func writeBinary(path string, v io.WriterTo) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = v.WriteTo(f)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func Setup(root string) error {
	if e := Prepare(root); e != nil {
		return e
	}
	out := filepath.Join(root, OutputDir, "setup.json")
	if _, e := os.Stat(out); e == nil {
		return fmt.Errorf("setup exists; use validated artifacts")
	}
	pkg, _, e := v2case.KeyFixture()
	if e != nil {
		return e
	}
	ps := Profiles()
	css := make([]constraint.ConstraintSystem, len(ps))
	result := SetupResult{DevelopmentOnly: true, LagrangeMS: map[int]float64{}}
	for i, p := range ps {
		start := time.Now()
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, NewCircuit(p, pkg.PublicKey))
		if e != nil {
			return fmt.Errorf("%s: %w", p.Name, e)
		}
		if cs.GetNbPublicVariables() != len(p.PublicNames()) {
			return fmt.Errorf("public order size %s", p.Name)
		}
		canonical, domain := plonk.SRSSize(cs)
		if canonical > result.CanonicalPoints {
			result.CanonicalPoints = canonical
		}
		css[i] = cs
		result.Relations = append(result.Relations, Relation{Profile: p, PublicNames: p.PublicNames(), Constraints: cs.GetNbConstraints(), Domain: domain, CompileMS: ms(time.Since(start))})
		fmt.Printf("compile %s: %d constraints, domain %d\n", p.Name, cs.GetNbConstraints(), domain)
	}
	start := time.Now()
	tau, e := rand.Int(rand.Reader, fr.Modulus())
	if e != nil {
		return e
	}
	for tau.Sign() == 0 {
		tau, e = rand.Int(rand.Reader, fr.Modulus())
		if e != nil {
			return e
		}
	}
	srs, e := kzg.NewSRS(uint64(result.CanonicalPoints), tau)
	tau.SetInt64(0)
	if e != nil {
		return e
	}
	result.SRSMS = ms(time.Since(start))
	srsPath := filepath.Join(root, ArtifactDir, "srs/canonical.bin")
	if e = writeBinary(srsPath, srs); e != nil {
		return e
	}
	result.CanonicalChecksum, e = checksum(srsPath)
	if e != nil {
		return e
	}
	lgs := map[int]*kzg.SRS{}
	for i, r := range result.Relations {
		if lgs[r.Domain] == nil {
			start = time.Now()
			points := append([]bls.G1Affine(nil), srs.Pk.G1[:r.Domain]...)
			lg, e := kzg.ToLagrangeG1(points)
			if e != nil {
				return e
			}
			lgs[r.Domain] = &kzg.SRS{Pk: kzg.ProvingKey{G1: lg}, Vk: srs.Vk}
			result.LagrangeMS[r.Domain] = ms(time.Since(start))
			if e = writeBinary(filepath.Join(root, ArtifactDir, fmt.Sprintf("srs/lagrange-%d.bin", r.Domain)), lgs[r.Domain]); e != nil {
				return e
			}
		}
		start = time.Now()
		pk, vk, e := plonk.Setup(css[i], srs, lgs[r.Domain])
		if e != nil {
			return e
		}
		result.Relations[i].SetupMS = ms(time.Since(start))
		files := map[string]string{}
		dir := filepath.Join(root, ArtifactDir, "circuits", r.Profile.Name)
		for _, item := range []struct {
			name  string
			value io.WriterTo
		}{{"ccs.bin", css[i]}, {"proving.key", pk}, {"verifying.key", vk}} {
			p := filepath.Join(dir, item.name)
			if e = writeBinary(p, item.value); e != nil {
				return e
			}
			files[item.name], e = checksum(p)
			if e != nil {
				return e
			}
		}
		verifier := filepath.Join(root, "contracts/src/generated/experiments", r.Profile.Name, "PlonkVerifier.sol")
		if e = artifact.ExportSolidity(verifier, vk); e != nil {
			return e
		}
		files["verifier"], e = checksum(verifier)
		if e != nil {
			return e
		}
		result.Relations[i].Files = files
		if e = writeJSON(filepath.Join(dir, "manifest.json"), result.Relations[i]); e != nil {
			return e
		}
		fmt.Printf("setup %s: %.1f ms\n", r.Profile.Name, result.Relations[i].SetupMS)
	}
	hasher := filepath.Join(root, "contracts/src/generated/experiments/Poseidon2BLS12381.sol")
	if e = os.MkdirAll(filepath.Dir(hasher), 0755); e != nil {
		return e
	}
	f, e := os.OpenFile(hasher, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	e = solgen.WritePoseidon2BLS12381(f)
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = CheckBaseline(root); e != nil {
		return e
	}
	return writeJSON(out, result)
}

type Keys struct {
	CS       constraint.ConstraintSystem
	PK       plonk.ProvingKey
	VK       plonk.VerifyingKey
	Relation Relation
}

func LoadKeys(root, name string) (*Keys, error) {
	var rel Relation
	dir := filepath.Join(root, ArtifactDir, "circuits", name)
	if e := readJSON(filepath.Join(dir, "manifest.json"), &rel); e != nil {
		return nil, e
	}
	k := &Keys{CS: plonk.NewCS(ecc.BLS12_381), PK: plonk.NewProvingKey(ecc.BLS12_381), VK: plonk.NewVerifyingKey(ecc.BLS12_381), Relation: rel}
	for _, item := range []struct {
		name   string
		target io.ReaderFrom
	}{{"ccs.bin", k.CS}, {"proving.key", k.PK}, {"verifying.key", k.VK}} {
		path := filepath.Join(dir, item.name)
		got, e := checksum(path)
		if e != nil {
			return nil, e
		}
		if got != rel.Files[item.name] {
			return nil, fmt.Errorf("artifact mismatch %s", path)
		}
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		_, e = item.target.ReadFrom(f)
		f.Close()
		if e != nil {
			return nil, e
		}
	}
	return k, nil
}
