package experiments

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/owner"
	audit "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const sharedArtifacts = "artifacts/development/shared-transfer"
const sharedOutput = "output/paper-supplement/shared-transfer"

func sharedProtect(root string) map[string]string {
	paths := []string{"internal/experiments/circuit.go", "internal/experiments/native.go", "contracts/src/experiments/ExperimentalAuditLedger.sol", "contracts/src/experiments/ExperimentalNoAuditLedger.sol", "output/experiments/experiment-1.json", "output/experiments/experiment-2.json", "output/experiments/experiment-3.json", "output/experiments/setup.json"}
	for _, kind := range []int{Entry, Direct, Exit} {
		_ = kind
	}
	paths = append(paths, ArtifactDir+"/srs/canonical.bin")
	result := map[string]string{}
	for _, p := range paths {
		result[p] = must(checksum(filepath.Join(root, p)))
	}
	return result
}
func SharedSetup(root string) error {
	target := filepath.Join(root, sharedArtifacts, "setup.json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("setup exists")
	}
	before := sharedProtect(root)
	srs := new(kzg.SRS)
	var original SetupResult
	if e := readJSON(filepath.Join(root, OutputDir, "setup.json"), &original); e != nil {
		return e
	}
	if before[ArtifactDir+"/srs/canonical.bin"] != original.CanonicalChecksum {
		return fmt.Errorf("original canonical SRS checksum mismatch")
	}
	if e := paperRead(filepath.Join(root, ArtifactDir, "srs/canonical.bin"), srs); e != nil {
		return e
	}
	var rels []Relation
	for i, kind := range []int{Entry, Direct, Exit} {
		p := sharedProfile(kind)
		start := time.Now()
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, newSharedCircuit(p))
		if e != nil {
			return e
		}
		needed, domain := plonk.SRSSize(cs)
		if needed > len(srs.Pk.G1) {
			return fmt.Errorf("SRS too small")
		}
		lg := new(kzg.SRS)
		if e = paperRead(filepath.Join(root, ArtifactDir, fmt.Sprintf("srs/lagrange-%d.bin", domain)), lg); e != nil {
			return e
		}
		rel := Relation{Profile: p, PublicNames: p.PublicNames(), Constraints: cs.GetNbConstraints(), Domain: domain, CompileMS: ms(time.Since(start)), Files: map[string]string{}}
		start = time.Now()
		pk, vk, e := plonk.Setup(cs, srs, lg)
		if e != nil {
			return e
		}
		rel.SetupMS = ms(time.Since(start))
		dir := filepath.Join(root, sharedArtifacts, p.Name)
		for _, f := range []struct {
			name string
			v    io.WriterTo
		}{{"ccs.bin", cs}, {"proving.key", pk}, {"verifying.key", vk}} {
			if e = writeBinary(filepath.Join(dir, f.name), f.v); e != nil {
				return e
			}
			rel.Files[f.name] = must(checksum(filepath.Join(dir, f.name)))
		}
		vp := filepath.Join(root, sharedArtifacts, "contracts", fmt.Sprintf("S%d.sol", i))
		if e = artifact.ExportSolidity(vp, vk); e != nil {
			return e
		}
		rel.Files["verifier"] = must(checksum(vp))
		if e = writeJSON(filepath.Join(dir, "manifest.json"), rel); e != nil {
			return e
		}
		rels = append(rels, rel)
		fmt.Printf("shared setup %s constraints=%d domain=%d\n", p.Name, rel.Constraints, domain)
	}
	for p, w := range before {
		if must(checksum(filepath.Join(root, p))) != w {
			return fmt.Errorf("protected changed %s", p)
		}
	}
	return writeJSON(target, map[string]any{"relations": rels, "protected": before, "developmentOnly": true, "newSRS": false, "srsChecksum": before[ArtifactDir+"/srs/canonical.bin"], "GOMAXPROCS": runtime.GOMAXPROCS(0)})
}
func sharedKeys(root string, p Profile) (*Keys, error) {
	dir := filepath.Join(root, sharedArtifacts, p.Name)
	var rel Relation
	if e := readJSON(filepath.Join(dir, "manifest.json"), &rel); e != nil {
		return nil, e
	}
	k := &Keys{CS: plonk.NewCS(ecc.BLS12_381), PK: plonk.NewProvingKey(ecc.BLS12_381), VK: plonk.NewVerifyingKey(ecc.BLS12_381), Relation: rel}
	for _, x := range []struct {
		name string
		v    io.ReaderFrom
	}{{"ccs.bin", k.CS}, {"proving.key", k.PK}, {"verifying.key", k.VK}} {
		p := filepath.Join(dir, x.name)
		if must(checksum(p)) != rel.Files[x.name] {
			return nil, fmt.Errorf("key checksum")
		}
		if e := paperRead(p, x.v); e != nil {
			return nil, e
		}
	}
	return k, nil
}
func SharedRun(root string) (err error) {
	out := filepath.Join(root, sharedOutput)
	if _, e := os.Stat(filepath.Join(out, "result.json")); e == nil {
		return fmt.Errorf("official result exists")
	}
	if e := os.MkdirAll(out, 0755); e != nil {
		return e
	}
	attempt := fmt.Sprint(time.Now().UnixNano())
	start := time.Now()
	defer func() {
		_ = writeJSON(filepath.Join(out, "attempt-"+attempt+".json"), map[string]any{"started": start, "ended": time.Now(), "error": fmt.Sprint(err), "officialRuns": 1, "warmups": 0})
	}()
	before := sharedProtect(root)
	l, e := net.Listen("tcp", "127.0.0.1:18549")
	if e != nil {
		return fmt.Errorf("port occupied: %w", e)
	}
	l.Close()
	docker := func(args ...string) error {
		cmd := exec.Command("docker", append([]string{"compose", "-f", "docker-compose.shared-transfer.yml", "-p", "zkdpp-shared-transfer"}, args...)...)
		cmd.Dir = root
		b, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("docker: %w %s", e, b)
		}
		return nil
	}
	if e = docker("up", "-d", "anvil"); e != nil {
		return e
	}
	defer func() {
		if e := docker("down"); e != nil {
			fmt.Fprintln(os.Stderr, e)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	m := &Meter{}
	r, e := rpc.DialOptions(ctx, "http://127.0.0.1:18549", rpc.WithHTTPClient(&http.Client{Transport: m, Timeout: time.Minute}))
	if e != nil {
		return e
	}
	key := must(crypto.HexToECDSA(anvilPrivateKey))
	en := &Engine{Root: root, Ctx: ctx, Client: ethclient.NewClient(r), Meter: m, Key: key, From: crypto.PubkeyToAddress(key.PublicKey), Audit: true, ContractOutDir: "contracts/shared-transfer/out"}
	defer en.Client.Close()
	for i := 0; i < 100; i++ {
		id, e := en.Client.ChainID(ctx)
		if e == nil && id.Uint64() == 31337 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var fixed [7]common.Address
	for i := range fixed {
		fixed[i] = common.Address{}
	}
	va := []common.Address{}
	for i := 0; i < 3; i++ {
		a, _, e := en.deploy("Imports.sol", fmt.Sprintf("S%d", i))
		if e != nil {
			return e
		}
		va = append(va, a)
	}
	for i := range fixed {
		fixed[i] = va[0]
	}
	fixed[1] = va[1]
	fixed[6] = va[2]
	hasher, _, e := en.deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if e != nil {
		return e
	}
	en.Ledger, en.ABI, e = en.deploy("SharedSecretAuditLedger.sol", "SharedSecretAuditLedger", fixed, hasher, en.From)
	if e != nil {
		return e
	}
	if e = en.call("setEntryIssuer", en.From, true); e != nil {
		return e
	}
	w := NewWorld(defaultPK(), 20000000)
	sender, receiver := h.Element(11), h.Element(22)
	var frames []map[string]any
	var events []*Event
	execute := func(op Operation, label string) (*Event, error) {
		op = w.Ready(op)
		ev, c, e := sharedBuild(w, op)
		if e != nil {
			return nil, e
		}
		k, e := sharedKeys(root, op.P)
		if e != nil {
			return nil, e
		}
		wit, e := frontend.NewWitness(c, ecc.BLS12_381.ScalarField())
		if e != nil {
			return nil, e
		}
		pub, e := wit.Public()
		if e != nil {
			return nil, e
		}
		if !audit.Equal(ev.Public, []fr.Element(pub.Vector().(fr.Vector))) {
			return nil, fmt.Errorf("public mismatch")
		}
		t := time.Now()
		proof, e := plonk.Prove(k.CS, k.PK, wit)
		if e != nil {
			return nil, e
		}
		proveMS := ms(time.Since(t))
		if e = plonk.Verify(proof, k.VK, pub); e != nil {
			return nil, e
		}
		raw := proof.(interface{ MarshalSolidity() []byte }).MarshalSolidity()
		data, e := eventData(en.ABI, ev, raw)
		if e != nil {
			return nil, e
		}
		receipt, tx, e := en.send(label, &en.Ledger, data)
		if e != nil {
			return nil, e
		}
		if e = w.Commit(ev); e != nil {
			return nil, e
		}
		if e = en.MatchRecord(ev); e != nil {
			return nil, e
		}
		if e = en.MatchEffects(ev); e != nil {
			return nil, e
		}
		rootGot, e := en.view("currentNoteRoot")
		if e != nil {
			return nil, e
		}
		if rootGot[0].(*big.Int).Cmp(audit.Big(w.Notes.Root())) != 0 {
			return nil, fmt.Errorf("tree root mismatch")
		}
		frame := map[string]any{"case": label, "profile": op.P.Name, "public": ev.Public, "proof": "0x" + hex.EncodeToString(raw), "calldata": "0x" + hex.EncodeToString(data), "receipt": receipt, "transaction": tx, "constraints": k.Relation.Constraints, "proveMS": proveMS, "official": op.P.Kind == Direct}
		if e = writeJSON(filepath.Join(root, sharedArtifacts, label+"-proof.json"), frame); e != nil {
			return nil, e
		}
		frames = append(frames, frame)
		events = append(events, ev)
		fmt.Printf("%s gas=%d\n", label, tx.Gas)
		return ev, nil
	}
	entry, e := execute(Operation{P: sharedProfile(Entry), SK: sender, Document: must(document.Hash(DefaultDocument())), State: note.State{QMass: 160e9, ARec: 15e9, E: 115e9}}, "preparation-entry")
	if e != nil {
		return e
	}
	// Match the opening offset of old direct-route official run 1; all scaled State values unchanged.
	w.Serial += 10000
	direct, e := execute(Operation{P: sharedProfile(Direct), SK: sender, Inputs: entry.Outputs, Receiver: owner.Address(receiver), Transport: 1e9}, "official-shared-direct")
	if e != nil {
		return e
	}
	pkg, shares, e := v2case.KeyFixture()
	if e != nil {
		return e
	}
	master, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if e != nil {
		return e
	}
	defer master.SetInt64(0)
	plain, e := auditcrypto.DecryptWithMasterKey(master, direct.Cipher)
	if e != nil {
		return e
	}
	if plain[0] != entry.Outputs[0].Commitment || plain[1] != sharedNF(direct.Outputs[0]) || plain[2] != sharedNF(direct.Outputs[1]) {
		return fmt.Errorf("audit plaintext mismatch")
	}
	ex, e := execute(Operation{P: sharedProfile(Exit), SK: receiver, Inputs: direct.Outputs[:1]}, "correctness-receiver-exit")
	if e != nil {
		return e
	}
	if ex.Public[1] != plain[1] {
		return fmt.Errorf("audit consumed nf mismatch")
	}
	for _, ev := range events {
		for _, n := range ev.Outputs {
			p, e := en.view("producerOf", uint8(1), audit.Big(n.Commitment))
			if e != nil || p[0].(*big.Int).Sign() == 0 {
				return fmt.Errorf("producer missing")
			}
		}
	}
	last := frames[len(frames)-1]
	calldata := must(hex.DecodeString(last["calldata"].(string)[2:]))
	if _, e = en.Client.CallContract(ctx, ethereum.CallMsg{From: en.From, To: &en.Ledger, Data: calldata}, nil); e == nil {
		return fmt.Errorf("double spend accepted")
	}
	for p, want := range before {
		if must(checksum(filepath.Join(root, p))) != want {
			return fmt.Errorf("protected changed %s", p)
		}
	}
	return writeJSON(filepath.Join(out, "result.json"), map[string]any{"frames": frames, "deployments": en.Deployments, "preparations": en.Preparations, "ledger": en.Ledger, "protected": before, "officialRuns": 1, "warmups": 0, "proofs": 3, "setup": "artifacts/development/shared-transfer/setup.json", "ownershipNegativeTests": "TestSharedTransferOwnership", "auditPlaintextMatched": true, "decryptedFutureNfConsumed": true, "doubleSpendRejected": true, "GOMAXPROCS": runtime.GOMAXPROCS(0), "scope": "isolated shared-opening nullifier family; all Notes use same opening-based nf rule; ownership secret checked independently; Sender can identify receiver next spend; no Voucher; audited direct Transfer; only official-shared-direct gas is comparison value"})
}
