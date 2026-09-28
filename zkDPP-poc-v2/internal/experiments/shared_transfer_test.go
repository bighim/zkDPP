package experiments

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/ethereum/go-ethereum/crypto"
	"os"
	"path/filepath"
	"testing"
)

// Recompilation and metadata checks are read-only and create no new Proofs.
func TestSharedTransferArtifacts(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, e := os.Stat(filepath.Join(root, sharedOutput, "result.json")); os.IsNotExist(e) {
		t.Skip("no measured result")
	}
	var setup struct {
		Protected   map[string]string
		SRSChecksum string
	}
	if e := readJSON(filepath.Join(root, sharedArtifacts, "setup.json"), &setup); e != nil {
		t.Fatal(e)
	}
	for p, want := range setup.Protected {
		if got := must(checksum(filepath.Join(root, p))); got != want {
			t.Fatalf("protected differs %s", p)
		}
	}
	var old SetupResult
	if e := readJSON(filepath.Join(root, OutputDir, "setup.json"), &old); e != nil {
		t.Fatal(e)
	}
	if old.CanonicalChecksum != setup.SRSChecksum {
		t.Fatal("SRS provenance mismatch")
	}
	for _, kind := range []int{Entry, Direct, Exit} {
		p := sharedProfile(kind)
		k, e := sharedKeys(root, p)
		if e != nil {
			t.Fatal(e)
		}
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, newSharedCircuit(p))
		if e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if _, e = cs.WriteTo(&buf); e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(buf.Bytes())
		if hex.EncodeToString(sum[:]) != k.Relation.Files["ccs.bin"] {
			t.Fatal("circuit changed")
		}
		if cs.GetNbPublicVariables() != len(p.PublicNames()) {
			t.Fatal("public inputs")
		}
	}
	paths := must(filepath.Glob(filepath.Join(root, "contracts/shared-transfer/out/*/*.json")))
	for _, p := range paths {
		var b struct {
			Metadata struct {
				Sources map[string]struct {
					Keccak string `json:"keccak256"`
				}
			}
		}
		if e := readJSON(p, &b); e != nil {
			t.Fatal(e)
		}
		for source, meta := range b.Metadata.Sources {
			data, e := os.ReadFile(filepath.Join(root, "contracts/shared-transfer", source))
			if e != nil {
				t.Fatal(e)
			}
			if crypto.Keccak256Hash(data).Hex() != meta.Keccak {
				t.Fatalf("compiled source differs %s", source)
			}
		}
	}
	for _, pair := range [][2]string{{"contracts/src/experiments/SharedSecretAuditLedger.sol", "contracts/shared-transfer/shared/experiments/SharedSecretAuditLedger.sol"}, {"artifacts/development/shared-transfer/contracts/S0.sol", "contracts/shared-transfer/shared/S0.sol"}, {"artifacts/development/shared-transfer/contracts/S1.sol", "contracts/shared-transfer/shared/S1.sol"}, {"artifacts/development/shared-transfer/contracts/S2.sol", "contracts/shared-transfer/shared/S2.sol"}} {
		if must(checksum(filepath.Join(root, pair[0]))) != must(checksum(filepath.Join(root, pair[1]))) {
			t.Fatal("staged source differs")
		}
	}
}

func TestSharedTransferOwnership(t *testing.T) {
	w := NewWorld(defaultPK(), 20000000)
	sender, receiver := h.Element(11), h.Element(22)
	solve := func(c *SharedCircuit, want bool) {
		t.Helper()
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, newSharedCircuit(c.Profile))
		if e != nil {
			t.Fatal(e)
		}
		wit, e := frontend.NewWitness(c, ecc.BLS12_381.ScalarField())
		if e != nil {
			t.Fatal(e)
		}
		e = cs.IsSolved(wit)
		if (e == nil) != want {
			t.Fatalf("solved=%v want=%v: %v", e == nil, want, e)
		}
	}
	en, ec, e := sharedBuild(w, Operation{P: sharedProfile(Entry), SK: sender, Document: must(document.Hash(DefaultDocument())), State: note.State{QMass: 160e9, ARec: 15e9, E: 115e9}})
	if e != nil {
		t.Fatal(e)
	}
	solve(ec, true)
	if e = w.Commit(en); e != nil {
		t.Fatal(e)
	}
	op := w.Ready(Operation{P: sharedProfile(Direct), SK: sender, Inputs: en.Outputs, Receiver: owner.Address(receiver), Transport: 1e9})
	ev, c, e := sharedBuild(w, op)
	if e != nil {
		t.Fatal(e)
	}
	solve(c, true)
	if e = w.Commit(ev); e != nil {
		t.Fatal(e)
	}
	pkg, shares, e := v2case.KeyFixture()
	if e != nil {
		t.Fatal(e)
	}
	mk, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if e != nil {
		t.Fatal(e)
	}
	defer mk.SetInt64(0)
	plain, e := auditcrypto.DecryptWithMasterKey(mk, ev.Cipher)
	if e != nil {
		t.Fatal(e)
	}
	if plain[1] != sharedNF(ev.Outputs[0]) || plain[2] != sharedNF(ev.Outputs[1]) {
		t.Fatal("future nf mismatch")
	}
	ex, xc, e := sharedBuild(w, w.Ready(Operation{P: sharedProfile(Exit), SK: receiver, Inputs: ev.Outputs[:1]}))
	if e != nil {
		t.Fatal(e)
	}
	solve(xc, true)
	if ex.Public[1] != plain[1] {
		t.Fatal("decrypted nf differs from spend")
	}
	xc.Secret = sender
	solve(xc, false)
	xc.Secret = receiver
	xc.Public[1] = note.Nullifier(receiver, ev.Outputs[0].Commitment)
	solve(xc, false)
	xc.Public[1] = ex.Public[1]
	xc.Notes[0].Opening = h.Element(987)
	solve(xc, false)
}
