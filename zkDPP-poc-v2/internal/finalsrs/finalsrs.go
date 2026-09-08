package finalsrs

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m9run"
	"github.com/consensys/gnark-crypto/ecc"
	curve "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	kzg "github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
)

const MaxDomain = 1 << 17
const CanonicalPoints = MaxDomain + 3

type RelationResult struct {
	Name, Class                                                          string
	EventKind                                                            uint8
	PolicyRef                                                            string
	PublicNames                                                          []string
	Constraints, PublicInputs, SystemSize, DomainSize, CanonicalRequired int
	CompileMillis, SetupMillis, ProveMillis, VerifyMillis                float64
	AllocatedBytes                                                       uint64
	ProofBytes                                                           int
	UniversalSRSChecksum, LagrangeSRSChecksum                            string
	Files                                                                map[string]string `json:",omitempty"`
	FileBytes                                                            map[string]int64  `json:",omitempty"`
}
type RunResult struct {
	Run                     int
	Persisted               bool
	CanonicalMillis         float64
	CanonicalAllocatedBytes uint64
	CanonicalBytes          int64
	CanonicalChecksum       string
	LagrangeMillis          map[int]float64
	LagrangeBytes           map[int]int64
	LagrangeChecksums       map[int]string
	Relations               []RelationResult
}
type SRSManifest struct {
	Version                             int
	DevelopmentOnly, ProductionCeremony bool
	Source, Curve, ProofSystem          string
	MaxDomain, CanonicalPoints          int
	CanonicalChecksum                   string
	CanonicalBytes                      int64
	Lagrange                            map[int]FileInfo
	SelectedRun                         int
}
type FileInfo struct {
	Checksum string
	Bytes    int64
}

type compiled struct {
	relation          m9run.Relation
	ccs               constraint.ConstraintSystem
	compileMS         float64
	domain, canonical int
}

func Run(root string, run int, persist bool) (RunResult, error) {
	committee, err := m9run.Committee(root)
	if err != nil {
		return RunResult{}, err
	}
	relations, err := m9run.Relations(root, committee.PK)
	if err != nil {
		return RunResult{}, err
	}
	if len(relations) != 10 {
		return RunResult{}, fmt.Errorf("final relations=%d", len(relations))
	}
	compiledRelations := make([]compiled, len(relations))
	maxDomain := 0
	for i, r := range relations {
		start := time.Now()
		ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, r.Definition)
		ms := m9run.Millis(time.Since(start))
		if e != nil {
			return RunResult{}, e
		}
		canonical, domain := plonk.SRSSize(ccs)
		if ccs.GetNbPublicVariables() != len(r.Public) {
			return RunResult{}, fmt.Errorf("%s public=%d", r.Name, ccs.GetNbPublicVariables())
		}
		if domain > maxDomain {
			maxDomain = domain
		}
		compiledRelations[i] = compiled{r, ccs, ms, domain, canonical}
	}
	if maxDomain != MaxDomain {
		return RunResult{}, fmt.Errorf("maximum domain=%d want=%d", maxDomain, MaxDomain)
	}
	tau, err := rand.Int(rand.Reader, fr.Modulus())
	if err != nil {
		return RunResult{}, err
	}
	if tau.Sign() == 0 {
		tau = big.NewInt(1)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	canonical, err := kzg.NewSRS(CanonicalPoints, tau)
	canonicalMS := m9run.Millis(time.Since(start))
	runtime.ReadMemStats(&after)
	tau.SetInt64(0)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{Run: run, Persisted: persist, CanonicalMillis: canonicalMS, CanonicalAllocatedBytes: after.TotalAlloc - before.TotalAlloc, LagrangeMillis: map[int]float64{}, LagrangeBytes: map[int]int64{}, LagrangeChecksums: map[int]string{}}
	lagranges := map[int]*kzg.SRS{}
	for _, domain := range []int{1 << 14, 1 << 15, 1 << 16, 1 << 17} {
		points := append([]curve.G1Affine(nil), canonical.Pk.G1[:domain]...)
		start = time.Now()
		lagrangePoints, e := kzg.ToLagrangeG1(points)
		result.LagrangeMillis[domain] = m9run.Millis(time.Since(start))
		if e != nil {
			return result, e
		}
		lagranges[domain] = &kzg.SRS{Pk: kzg.ProvingKey{G1: lagrangePoints}, Vk: canonical.Vk}
	}
	if persist {
		if err = writeSRS(root, canonical, lagranges, &result); err != nil {
			return result, err
		}
	}
	for _, c := range compiledRelations {
		start = time.Now()
		pk, vk, e := plonk.Setup(c.ccs, canonical, lagranges[c.domain])
		setupMS := m9run.Millis(time.Since(start))
		if e != nil {
			return result, e
		}
		w, e := frontend.NewWitness(c.relation.Assignment, ecc.BLS12_381.ScalarField())
		if e != nil {
			return result, e
		}
		public, e := w.Public()
		if e != nil {
			return result, e
		}
		runtime.GC()
		runtime.ReadMemStats(&before)
		start = time.Now()
		proof, e := plonk.Prove(c.ccs, pk, w)
		proveMS := m9run.Millis(time.Since(start))
		runtime.ReadMemStats(&after)
		if e != nil {
			return result, e
		}
		start = time.Now()
		e = plonk.Verify(proof, vk, public)
		verifyMS := m9run.Millis(time.Since(start))
		if e != nil {
			return result, e
		}
		var proofBuf bytes.Buffer
		_, _ = proof.WriteTo(&proofBuf)
		row := RelationResult{Name: c.relation.Name, Class: c.relation.Class, EventKind: c.relation.EventKind, PolicyRef: c.relation.PolicyRef, PublicNames: c.relation.Public, Constraints: c.ccs.GetNbConstraints(), PublicInputs: c.ccs.GetNbPublicVariables(), SystemSize: c.ccs.GetNbConstraints() + c.ccs.GetNbPublicVariables(), DomainSize: c.domain, CanonicalRequired: c.canonical, CompileMillis: c.compileMS, SetupMillis: setupMS, ProveMillis: proveMS, VerifyMillis: verifyMS, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, ProofBytes: proofBuf.Len(), UniversalSRSChecksum: result.CanonicalChecksum, LagrangeSRSChecksum: result.LagrangeChecksums[c.domain]}
		if persist {
			files, sizes, e := writeCircuit(root, c.relation.Name, c.ccs, pk, vk)
			if e != nil {
				return result, e
			}
			row.Files = files
			row.FileBytes = sizes
			if e = artifact.ExportSolidity(filepath.Join(root, "contracts/src/generated/m9", c.relation.Name, "PlonkVerifier.sol"), vk); e != nil {
				return result, e
			}
		}
		result.Relations = append(result.Relations, row)
	}
	return result, nil
}

func writeSRS(root string, canonical *kzg.SRS, lagranges map[int]*kzg.SRS, result *RunResult) error {
	dir := filepath.Join(root, "artifacts/final/srs")
	if err := os.MkdirAll(filepath.Join(dir, "lagrange"), 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "universal-canonical.bin")
	if err := write(path, canonical); err != nil {
		return err
	}
	h, e := artifact.Checksum(path)
	if e != nil {
		return e
	}
	info, _ := os.Stat(path)
	result.CanonicalChecksum = h
	result.CanonicalBytes = info.Size()
	manifest := SRSManifest{1, true, false, "local-unsafe-benchmark", "BLS12-381", "PLONK-KZG", MaxDomain, CanonicalPoints, h, info.Size(), map[int]FileInfo{}, 1}
	for domain, srs := range lagranges {
		p := filepath.Join(dir, "lagrange", fmt.Sprintf("domain-2^%d.bin", log2(domain)))
		if e = write(p, srs); e != nil {
			return e
		}
		h, e = artifact.Checksum(p)
		if e != nil {
			return e
		}
		info, _ = os.Stat(p)
		result.LagrangeChecksums[domain] = h
		result.LagrangeBytes[domain] = info.Size()
		manifest.Lagrange[domain] = FileInfo{h, info.Size()}
	}
	return artifact.WriteJSON(filepath.Join(dir, "manifest.json"), manifest)
}
func writeCircuit(root, name string, ccs constraint.ConstraintSystem, pk plonk.ProvingKey, vk plonk.VerifyingKey) (map[string]string, map[string]int64, error) {
	dir := filepath.Join(root, "artifacts/final/circuits", name)
	if e := os.MkdirAll(dir, 0755); e != nil {
		return nil, nil, e
	}
	values := map[string]io.WriterTo{"ccs.bin": ccs, "proving.key": pk, "verifying.key": vk}
	hashes := map[string]string{}
	sizes := map[string]int64{}
	for n, v := range values {
		p := filepath.Join(dir, n)
		if e := write(p, v); e != nil {
			return nil, nil, e
		}
		h, e := artifact.Checksum(p)
		if e != nil {
			return nil, nil, e
		}
		info, _ := os.Stat(p)
		hashes[n] = h
		sizes[n] = info.Size()
	}
	return hashes, sizes, nil
}
func write(path string, v io.WriterTo) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	if _, e = v.WriteTo(f); e != nil {
		f.Close()
		return e
	}
	return f.Close()
}
func log2(v int) int {
	n := 0
	for v > 1 {
		v >>= 1
		n++
	}
	return n
}

type Loaded struct {
	CCS      constraint.ConstraintSystem
	PK       plonk.ProvingKey
	VK       plonk.VerifyingKey
	Manifest RelationResult
}

func Load(root, name string) (*Loaded, error) {
	dir := filepath.Join(root, "artifacts/final/circuits", name)
	var manifest RelationResult
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &manifest); err != nil {
		return nil, err
	}
	ccs := plonk.NewCS(ecc.BLS12_381)
	pk := plonk.NewProvingKey(ecc.BLS12_381)
	vk := plonk.NewVerifyingKey(ecc.BLS12_381)
	for n, v := range map[string]io.ReaderFrom{"ccs.bin": ccs, "proving.key": pk, "verifying.key": vk} {
		path := filepath.Join(dir, n)
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		_, err = v.ReadFrom(f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		h, err := artifact.Checksum(path)
		if err != nil || h != manifest.Files[n] {
			return nil, fmt.Errorf("final artifact checksum %s", n)
		}
	}
	return &Loaded{CCS: ccs, PK: pk, VK: vk, Manifest: manifest}, nil
}
