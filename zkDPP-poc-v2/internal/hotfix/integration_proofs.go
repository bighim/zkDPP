package hotfix

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2run"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"io"
	"os"
	"path/filepath"
	"time"
)

type IntegratedProofFile struct {
	Profile        string
	Base           ProofGroup
	Probes         []ProofEvent
	Expected       v2case.IntegratedExpected
	ArtifactHashes map[string]string
}

func IntegrationSetup(root, out string) error {
	setup := filepath.Join(out, "setup")
	if err := os.MkdirAll(filepath.Join(setup, "testdata/common"), 0755); err != nil {
		return err
	}
	for _, name := range []string{"actors-v1.json", "actors-v2.json"} {
		b, err := os.ReadFile(filepath.Join(root, "testdata/common", name))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(setup, "testdata/common", name), b, 0644); err != nil {
			return err
		}
	}
	if _, err := v2run.Setup(setup); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(setup, "contracts/src/generated/v2-m1-hotfix/Poseidon2BLS12381.sol"))
	if err != nil {
		return err
	}
	err = solgen.WritePoseidon2BLS12381(f)
	f.Close()
	if err != nil {
		return err
	}
	src := filepath.Join(setup, "contracts/hotfix/src")
	if err = os.MkdirAll(src, 0755); err != nil {
		return err
	}
	for _, name := range append(append([]string{}, wrappers...), "ZkDPPV2Ledger", "IFieldHasher", "IZkVerifier") {
		b, err := os.ReadFile(filepath.Join(root, "contracts/hotfix/src", name+".sol"))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(src, name+".sol"), b, 0644); err != nil {
			return err
		}
	}
	if err = os.Symlink("../../src/generated/v2-m1-hotfix", filepath.Join(src, "generated")); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(root, "contracts/hotfix/foundry.toml"))
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(setup, "contracts/hotfix/foundry.toml"), b, 0644); err != nil {
		return err
	}
	return Write(filepath.Join(out, "raw/setup-profile.json"), map[string]any{"profile": "supplementary-mixed-containment-development-setup", "sourceCommit": "483f4a0280d832f95a6842997ca89e35bcbb0879", "sameCircuits": true, "developmentOnly": true, "setupRoot": setup, "originalSetupBinariesAvailable": false, "setupBuilder": "unchanged internal/v2run.Setup"})
}
func IntegrationProofs(root, out string) error {
	path := filepath.Join(out, "raw/proofs.json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("proof output exists")
	}
	fixture, err := v2case.BuildIntegrated(root)
	if err != nil {
		return err
	}
	setup := filepath.Join(out, "setup")
	result := IntegratedProofFile{Profile: "supplementary-mixed-containment", Expected: fixture.Expected, ArtifactHashes: map[string]string{}}
	result.Base = ProofGroup{Name: fixture.Base.Name, Start: fixture.Base.Start, NoteRoot: fixture.Base.NoteRoot, VoucherRoot: fixture.Base.VoucherRoot, NoteCount: fixture.Base.NoteCount, VoucherCount: fixture.Base.VoucherCount}
	// Verify source-equivalent constraint systems and every artifact hash before
	// using the newly prepared keys; cache only the loaded relation artifacts.
	suite, err := v2case.Build(root)
	if err != nil {
		return err
	}
	definitions := map[string]frontend.Circuit{}
	for _, c := range suite.Cases {
		definitions[c.Name] = c.Definition
	}
	loaded := map[string]struct {
		cs constraint.ConstraintSystem
		pk plonk.ProvingKey
		vk plonk.VerifyingKey
	}{}
	for idx, ev := range append(append([]v2case.Event{}, fixture.Base.Events...), fixture.Probes...) {
		dir := filepath.Join(setup, Dir, "circuits", ev.Relation)
		pair, ok := loaded[ev.Relation]
		if !ok {
			var manifest v2run.RelationResult
			if err = Read(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
				return err
			}
			cs := plonk.NewCS(ecc.BLS12_381)
			pk := plonk.NewProvingKey(ecc.BLS12_381)
			vk := plonk.NewVerifyingKey(ecc.BLS12_381)
			for n, obj := range map[string]io.ReaderFrom{"ccs.bin": cs, "proving.key": pk, "verifying.key": vk} {
				p := filepath.Join(dir, n)
				hash, err := Hash(p)
				if err != nil {
					return err
				}
				if hash != manifest.Files[n] {
					return fmt.Errorf("artifact hash mismatch: %s", p)
				}
				result.ArtifactHashes[ev.Relation+"/"+n] = hash
				f, err := os.Open(p)
				if err != nil {
					return err
				}
				_, err = obj.ReadFrom(f)
				f.Close()
				if err != nil {
					return err
				}
			}
			compiled, err := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, definitions[ev.Relation])
			if err != nil {
				return err
			}
			var a, b bytes.Buffer
			compiled.WriteTo(&a)
			cs.WriteTo(&b)
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				return fmt.Errorf("compiled CCS mismatch: %s", ev.Relation)
			}
			pair = struct {
				cs constraint.ConstraintSystem
				pk plonk.ProvingKey
				vk plonk.VerifyingKey
			}{cs, pk, vk}
			loaded[ev.Relation] = pair
		}
		w, err := frontend.NewWitness(ev.Assignment, ecc.BLS12_381.ScalarField())
		if err != nil {
			return err
		}
		pub, err := w.Public()
		if err != nil {
			return err
		}
		if !v2audit.Equal([]fr.Element(pub.Vector().(fr.Vector)), ev.Public) {
			return fmt.Errorf("public order mismatch: %s", ev.Name)
		}
		wp := filepath.Join(out, "raw/witnesses", ev.Name+".bin")
		if err = os.MkdirAll(filepath.Dir(wp), 0755); err != nil {
			return err
		}
		wf, err := os.Create(wp)
		if err != nil {
			return err
		}
		_, err = w.WriteTo(wf)
		wf.Close()
		if err != nil {
			return err
		}
		start := time.Now()
		proof, err := plonk.Prove(pair.cs, pair.pk, w)
		if err != nil {
			return fmt.Errorf("%s: %w", ev.Name, err)
		}
		pm := float64(time.Since(start).Nanoseconds()) / 1e6
		start = time.Now()
		if err = plonk.Verify(proof, pair.vk, pub); err != nil {
			return err
		}
		var buf bytes.Buffer
		proof.WriteTo(&buf)
		row := ProofEvent{Name: ev.Name, Relation: ev.Relation, Kind: ev.Kind, Proof: "0x" + hex.EncodeToString(proof.(interface{ MarshalSolidity() []byte }).MarshalSolidity()), Record: ev.Record, ProveMillis: pm, VerifyMillis: float64(time.Since(start).Nanoseconds()) / 1e6, ProofBytes: buf.Len()}
		for _, p := range ev.Public {
			row.Public = append(row.Public, auditcrypto.EncodeField(p))
		}
		if idx < len(fixture.Base.Events) {
			result.Base.Events = append(result.Base.Events, row)
		} else {
			result.Probes = append(result.Probes, row)
		}
		fmt.Println("proved and verified", row.Name)
	}
	result.ArtifactHashes["universal-canonical.bin"], err = Hash(filepath.Join(setup, Dir, "srs/universal-canonical.bin"))
	if err != nil {
		return err
	}
	if err = Write(filepath.Join(out, "raw/expected.json"), &fixture.Expected); err != nil {
		return err
	}
	return Write(path, &result)
}

// Reconstruct each private assignment and require byte-for-byte correspondence
// with the full witness actually supplied to PLONK in the proof run.
func IntegrationWitnessEvidence(root, out string) error {
	f, err := v2case.BuildIntegrated(root)
	if err != nil {
		return err
	}
	var rows []map[string]any
	for _, ev := range append(append([]v2case.Event{}, f.Base.Events...), f.Probes...) {
		w, err := frontend.NewWitness(ev.Assignment, ecc.BLS12_381.ScalarField())
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if _, err = w.WriteTo(&buf); err != nil {
			return err
		}
		p := filepath.Join(out, "raw/witnesses", ev.Name+".bin")
		actual, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, buf.Bytes()) {
			return fmt.Errorf("witness bytes mismatch: %s", ev.Name)
		}
		hash, err := Hash(p)
		if err != nil {
			return err
		}
		if err = Write(filepath.Join(out, "raw/assignments", ev.Name+".json"), ev.Assignment); err != nil {
			return err
		}
		rows = append(rows, map[string]any{"event": ev.Name, "witnessSHA256": hash, "bytesMatched": true})
	}
	return Write(filepath.Join(out, "raw/witness-evidence.json"), rows)
}
