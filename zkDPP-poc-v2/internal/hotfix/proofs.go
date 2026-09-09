package hotfix

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"io"
	"os"
	"path/filepath"
	"time"
)

type ProofEvent struct {
	Name, Relation            string
	Kind                      v2audit.Kind
	Proof                     string
	Public                    []string
	Record                    v2audit.Record
	ProveMillis, VerifyMillis float64
	ProofBytes                int
}
type ProofGroup struct {
	Name                    string
	Events, Extra           []ProofEvent
	Start                   v2audit.Ref
	NoteRoot, VoucherRoot   fr.Element
	NoteCount, VoucherCount uint64
	ClaimValues             []claim.DPPClaim
	Document                document.DocumentInfo
}
type ProofFile struct {
	Profile string
	Groups  []ProofGroup
}

func Setup(root string) error {
	if e := Baseline(root); e != nil {
		return e
	}
	if e := Prepare(root); e != nil {
		return e
	}
	_, e := v2run.Setup(root)
	if e != nil {
		return e
	}
	f, e := os.Create(filepath.Join(root, "contracts/src/generated/v2-m1-hotfix/Poseidon2BLS12381.sol"))
	if e != nil {
		return e
	}
	e = solgen.WritePoseidon2BLS12381(f)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func Evaluate(root string) error {
	target := filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("proof fixture exists")
	}
	groups, e := v2case.BuildHotfix(root)
	if e != nil {
		return e
	}
	var result v2run.CircuitResult
	if e = Read(filepath.Join(root, "output/m1-hotfix-circuit.json"), &result); e != nil {
		return e
	}
	out := ProofFile{Profile: "zkDPP-v2-M1-HF"}
	measured := map[string]bool{}
	for _, g := range groups {
		pg := ProofGroup{Name: g.Name, Start: g.Start, NoteRoot: g.NoteRoot, VoucherRoot: g.VoucherRoot, NoteCount: g.NoteCount, VoucherCount: g.VoucherCount, ClaimValues: g.ClaimValues, Document: g.Document}
		for index, ev := range append(append([]v2case.Event{}, g.Events...), g.Extra...) {
			dir := filepath.Join(root, Dir, "circuits", ev.Relation)
			var manifest v2run.RelationResult
			if e = Read(filepath.Join(dir, "manifest.json"), &manifest); e != nil {
				return e
			}
			ccs := plonk.NewCS(ecc.BLS12_381)
			pk := plonk.NewProvingKey(ecc.BLS12_381)
			vk := plonk.NewVerifyingKey(ecc.BLS12_381)
			for n, obj := range map[string]io.ReaderFrom{"ccs.bin": ccs, "proving.key": pk, "verifying.key": vk} {
				p := filepath.Join(dir, n)
				hash, e := Hash(p)
				if e != nil {
					return e
				}
				if hash != manifest.Files[n] {
					return fmt.Errorf("key mismatch %s", n)
				}
				f, e := os.Open(p)
				if e != nil {
					return e
				}
				_, e = obj.ReadFrom(f)
				f.Close()
				if e != nil {
					return e
				}
			}
			w, e := frontend.NewWitness(ev.Assignment, ecc.BLS12_381.ScalarField())
			if e != nil {
				return e
			}
			pub, e := w.Public()
			if e != nil {
				return e
			}
			if !v2audit.Equal([]fr.Element(pub.Vector().(fr.Vector)), ev.Public) {
				return fmt.Errorf("public order mismatch")
			}
			start := time.Now()
			proof, e := plonk.Prove(ccs, pk, w)
			pm := float64(time.Since(start).Nanoseconds()) / 1e6
			if e != nil {
				return fmt.Errorf("%s: %w", ev.Name, e)
			}
			start = time.Now()
			e = plonk.Verify(proof, vk, pub)
			vm := float64(time.Since(start).Nanoseconds()) / 1e6
			if e != nil {
				return e
			}
			var buf bytes.Buffer
			proof.WriteTo(&buf)
			pe := ProofEvent{Name: ev.Name, Relation: ev.Relation, Kind: ev.Kind, Proof: "0x" + hex.EncodeToString(proof.(interface{ MarshalSolidity() []byte }).MarshalSolidity()), Record: ev.Record, ProveMillis: pm, VerifyMillis: vm, ProofBytes: buf.Len()}
			for _, p := range ev.Public {
				pe.Public = append(pe.Public, auditcrypto.EncodeField(p))
			}
			if index < len(g.Events) {
				pg.Events = append(pg.Events, pe)
			} else {
				pg.Extra = append(pg.Extra, pe)
			}
			if !measured[ev.Relation] {
				for i := range result.Relations {
					if result.Relations[i].Name == ev.Relation {
						result.Relations[i].ProveMillis = pm
						result.Relations[i].VerifyMillis = vm
						result.Relations[i].ProofBytes = buf.Len()
					}
				}
				measured[ev.Relation] = true
			}
		}
		out.Groups = append(out.Groups, pg)
	}
	if len(measured) != 10 {
		return fmt.Errorf("only %d relations proved", len(measured))
	}
	if e = artifact.WriteJSON(target, out); e != nil {
		return e
	}
	return Write(filepath.Join(root, "output/m1-hotfix-circuit.json"), result)
}
