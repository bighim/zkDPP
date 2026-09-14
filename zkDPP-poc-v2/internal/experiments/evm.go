package experiments

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	audit "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	blsplonk "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/consensys/gnark/frontend"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
	"reflect"
	"runtime"
	"strings"
	"time"
)

// The standard, publicly documented Anvil development identity; never production.
const anvilPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

type Meter struct {
	Calls  int
	Millis float64
}

func (m *Meter) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	res, e := http.DefaultTransport.RoundTrip(r)
	if e == nil {
		b, er := io.ReadAll(res.Body)
		res.Body.Close()
		res.Body = io.NopCloser(bytes.NewReader(b))
		if er != nil {
			e = er
		}
	}
	m.Calls++
	m.Millis += ms(time.Since(start))
	return res, e
}

type Transaction struct {
	Name, Hash, BlockHash string
	Block, Gas, Status    uint64
	Calldata              int
	Millis                float64
}
type Measurement struct {
	ProofArtifact                                      string
	Case, Profile, Variant                             string
	Run                                                int
	Warmup                                             bool
	WitnessMS, ProveMS, VerifyMS, TransactionMS, E2EMS float64
	AllocatedBytes                                     uint64
	ProofBytes, Constraints, PublicInputs              int
	Tx                                                 Transaction
	SSTORECount, SSTOREGas                             uint64
	Matched                                            bool
}
type Engine struct {
	Root             string
	Ctx              context.Context
	Client           *ethclient.Client
	Meter            *Meter
	Key              *ecdsa.PrivateKey
	From, Ledger     common.Address
	ABI              abi.ABI
	Audit            bool
	Keys             map[string]*Keys
	Verifiers        map[string]common.Address
	Source           *audit.RPCSource
	Preparations     []Transaction
	Deployments      []map[string]any
	Rows             []Measurement
	Sequence         uint64
	RunTag           string
	LastReceipt      time.Time
	DeferDiagnostics bool
	Pending          []func() error
}

func command(root string, args ...string) error {
	cmd := exec.Command("docker", append([]string{"compose", "-f", "docker-compose.experiments.yml", "-p", "zkdpp-experiments"}, args...)...)
	cmd.Dir = root
	out, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("docker %v: %w\n%s", args, e, out)
	}
	return nil
}
func StartNode(root string, auditOnly bool) (func(), error) {
	l, e := net.Listen("tcp", "127.0.0.1:18547")
	if e != nil {
		return nil, fmt.Errorf("port 18547 occupied or unavailable: %w", e)
	}
	l.Close()
	service := "anvil"
	if auditOnly {
		service = "anvil-audit"
	}
	if e = command(root, "up", "-d", service); e != nil {
		return nil, e
	}
	stop := func() {
		if e := command(root, "down"); e != nil {
			fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", e)
		}
	}
	return stop, nil
}
func NewEngine(root string, ctx context.Context, audited bool) (*Engine, error) {
	m := &Meter{}
	r, e := rpc.DialOptions(ctx, "http://127.0.0.1:18547", rpc.WithHTTPClient(&http.Client{Transport: m, Timeout: 60 * time.Second}))
	if e != nil {
		return nil, e
	}
	key, e := crypto.HexToECDSA(anvilPrivateKey)
	if e != nil {
		return nil, e
	}
	en := &Engine{Root: root, Ctx: ctx, Client: ethclient.NewClient(r), Meter: m, Key: key, From: crypto.PubkeyToAddress(key.PublicKey), Audit: audited, Keys: map[string]*Keys{}, Verifiers: map[string]common.Address{}, RunTag: fmt.Sprint(time.Now().UnixNano())}
	for i := 0; i < 100; i++ {
		id, e := en.Client.ChainID(ctx)
		if e == nil && id.Uint64() == 31337 {
			return en, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	en.Client.Close()
	return nil, fmt.Errorf("Anvil not ready")
}
func (e *Engine) keys(name string) (*Keys, error) {
	if k := e.Keys[name]; k != nil {
		return k, nil
	}
	k, er := LoadKeys(e.Root, name)
	if er == nil {
		e.Keys[name] = k
	}
	return k, er
}
func (e *Engine) send(name string, to *common.Address, data []byte) (*types.Receipt, Transaction, error) {
	start := time.Now()
	row := Transaction{Name: name, Calldata: len(data)}
	gas, er := e.Client.EstimateGas(e.Ctx, ethereum.CallMsg{From: e.From, To: to, Data: data})
	if er != nil {
		return nil, row, er
	}
	if gas > 30000000 {
		return nil, row, fmt.Errorf("gas exceeds 30M")
	}
	nonce, er := e.Client.PendingNonceAt(e.Ctx, e.From)
	if er != nil {
		return nil, row, er
	}
	price, er := e.Client.SuggestGasPrice(e.Ctx)
	if er != nil {
		return nil, row, er
	}
	tx := types.NewTx(&types.LegacyTx{Nonce: nonce, GasPrice: price, Gas: gas, To: to, Data: data})
	tx, er = types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(31337)), e.Key)
	if er != nil {
		return nil, row, er
	}
	row.Hash = tx.Hash().Hex()
	if er = e.Client.SendTransaction(e.Ctx, tx); er != nil {
		return nil, row, er
	}
	for {
		receipt, er := e.Client.TransactionReceipt(e.Ctx, tx.Hash())
		if er == nil {
			row.Block = receipt.BlockNumber.Uint64()
			row.BlockHash = receipt.BlockHash.Hex()
			row.Gas = receipt.GasUsed
			row.Status = receipt.Status
			row.Millis = ms(time.Since(start))
			if receipt.Status != 1 {
				return receipt, row, fmt.Errorf("transaction reverted %s", name)
			}
			return receipt, row, nil
		}
		if er != ethereum.NotFound {
			return nil, row, er
		}
		select {
		case <-e.Ctx.Done():
			return nil, row, e.Ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func (e *Engine) contract(source, name string) (abi.ABI, []byte, error) {
	var raw struct {
		ABI      json.RawMessage
		Bytecode struct{ Object string }
	}
	if er := readJSON(filepath.Join(e.Root, "contracts/experiments/out", source, name+".json"), &raw); er != nil {
		return abi.ABI{}, nil, er
	}
	a, er := abi.JSON(bytes.NewReader(raw.ABI))
	if er != nil {
		return a, nil, er
	}
	b, er := hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x"))
	return a, b, er
}
func (e *Engine) deploy(source, name string, args ...any) (common.Address, abi.ABI, error) {
	a, b, er := e.contract(source, name)
	if er != nil {
		return common.Address{}, a, er
	}
	arg, er := a.Constructor.Inputs.Pack(args...)
	if er != nil {
		return common.Address{}, a, er
	}
	r, row, er := e.send("deploy-"+name, nil, append(b, arg...))
	if er != nil {
		return common.Address{}, a, er
	}
	e.Preparations = append(e.Preparations, row)
	code, er := e.Client.CodeAt(e.Ctx, r.ContractAddress, nil)
	if er != nil {
		return common.Address{}, a, er
	}
	if len(code) > 24576 {
		return common.Address{}, a, fmt.Errorf("EIP170 %s %d", name, len(code))
	}
	e.Deployments = append(e.Deployments, map[string]any{"name": name, "address": r.ContractAddress.Hex(), "runtimeBytes": len(code), "codeHash": crypto.Keccak256Hash(code).Hex(), "transaction": row})
	return r.ContractAddress, a, nil
}
func (e *Engine) verifier(p Profile) (common.Address, error) {
	if a, ok := e.Verifiers[p.Name]; ok {
		return a, nil
	}
	index := -1
	for i, v := range Profiles() {
		if v.Name == p.Name {
			index = i
		}
	}
	if index < 0 {
		return common.Address{}, fmt.Errorf("profile")
	}
	a, _, er := e.deploy("Imports.sol", fmt.Sprintf("V%d", index))
	if er == nil {
		e.Verifiers[p.Name] = a
	}
	return a, er
}
func (e *Engine) call(name string, args ...any) error {
	b, er := e.ABI.Pack(name, args...)
	if er != nil {
		return er
	}
	_, row, er := e.send(name, &e.Ledger, b)
	e.Preparations = append(e.Preparations, row)
	return er
}
func (e *Engine) view(name string, args ...any) ([]any, error) {
	return m7run.View(e.Ctx, e.Client, e.ABI, e.Ledger, name, args...)
}
func (e *Engine) Initialize(vector int) error {
	suffix := "-plain"
	if e.Audit {
		suffix = "-audit"
	}
	fixed := [7]common.Address{}
	for i, name := range []string{"entry", "transfer", "proceed", "recall", "merge", "split", "exit"} {
		n := name + suffix
		if vector > 0 && (name == "entry" || name == "merge") {
			n = fmt.Sprintf("vector-%s-%d", name, vector)
		}
		p := must(ProfileByName(n))
		a, er := e.verifier(p)
		if er != nil {
			return er
		}
		fixed[i] = a
	}
	hasher, _, er := e.deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if er != nil {
		return er
	}
	ledgerName := "ExperimentalAuditLedger"
	args := []any{fixed, hasher, e.From}
	if !e.Audit {
		ledgerName = "ExperimentalNoAuditLedger"
		d, er := e.verifier(must(ProfileByName("direct-plain")))
		if er != nil {
			return er
		}
		args = append(args, d)
	}
	e.Ledger, e.ABI, er = e.deploy(ledgerName+".sol", ledgerName, args...)
	if er != nil {
		return er
	}
	dep := e.Preparations[len(e.Preparations)-1]
	code, er := e.Client.CodeAt(e.Ctx, e.Ledger, nil)
	if er != nil {
		return er
	}
	e.Source = audit.NewRPCSource(e.Client, e.ABI, e.Ledger, e.From)
	e.Source.Context = audit.LedgerContext{ChainID: 31337, CodeHash: crypto.Keccak256Hash(code).Hex(), DeploymentBlock: dep.Block}
	if er = e.call("setEntryIssuer", e.From, true); er != nil {
		return er
	}
	if vector != 0 {
		return nil
	}
	if er = e.call("registerPolicyAuthority", e.From); er != nil {
		return er
	}
	// Experiment families 101 and 102 do not alias the historical Policy IDs.
	for id := 1; id <= 102; id++ {
		kind := uint8(6)
		if id == 2 || id == 102 {
			kind = 8
		}
		if er = e.call("reservePolicy", kind); er != nil {
			return er
		}
	}
	profiles := []string{"issue" + suffix}
	for _, s := range [][2]int{{2, 1}, {2, 2}, {3, 2}, {3, 3}, {4, 3}} {
		profiles = append(profiles, fmt.Sprintf("process-%d-%d%s", s[0], s[1], suffix))
	}
	if e.Audit {
		profiles = append(profiles, "issue-recycled-audit", "issue-carbon-audit")
	}
	for _, name := range profiles {
		p := must(ProfileByName(name))
		if name == "process-2-2"+suffix || name == "process-3-2"+suffix || name == "process-3-3"+suffix || name == "process-4-3"+suffix {
			if er = e.call("reservePolicyVersion", uint64(101)); er != nil {
				return er
			}
		}
		if name == "issue-carbon-audit" {
			if er = e.call("reservePolicyVersion", uint64(102)); er != nil {
				return er
			}
		}
		v, er := e.verifier(p)
		if er != nil {
			return er
		}
		k, er := e.keys(name)
		if er != nil {
			return er
		}
		opt, er := solgen.ExtractOptimizedVK(k.VK.(*blsplonk.VerifyingKey))
		if er != nil {
			return er
		}
		hb := must(hex.DecodeString(opt.SHA256))
		var vkHash [32]byte
		copy(vkHash[:], hb)
		n := p.N
		if p.Kind == Issue {
			n = 1
		}
		if er = e.call("registerPolicy", audit.Big(p.PolicyRef), uint8(p.M), uint8(n), vkHash, v); er != nil {
			return er
		}
		if p.Kind == Process {
			for _, sk := range []uint64{11, 22} {
				s := new(fr.Element).SetUint64(sk)
				if er = e.call("setPolicyGrant", audit.Big(p.PolicyRef), audit.Big(policy.ScopeRef(*s, p.PolicyRef)), true); er != nil {
					return er
				}
			}
		}
	}
	return nil
}
func (e *Engine) Snapshot() (string, error) {
	var id string
	er := e.Client.Client().CallContext(e.Ctx, &id, "evm_snapshot")
	return id, er
}
func (e *Engine) Restore(id string) (string, error) {
	var ok bool
	if er := e.Client.Client().CallContext(e.Ctx, &ok, "evm_revert", id); er != nil {
		return "", er
	}
	if !ok {
		return "", fmt.Errorf("snapshot restore rejected")
	}
	return e.Snapshot()
}
func eventData(a abi.ABI, ev *Event, proof []byte) ([]byte, error) {
	p := ev.Profile
	args := []any{proof}
	v := ev.Public
	b := len(p.BaseNames())
	method := []string{"entry", "transfer", "proceed", "recall", "merge", "split", "process", "exit", "issue", "directTransfer", "entry", "merge"}[p.Kind]
	if p.Kind == Process {
		ins, outs := make([]*big.Int, p.M), make([]*big.Int, p.N)
		for i := range ins {
			ins[i] = audit.Big(v[3+i])
		}
		for i := range outs {
			outs[i] = audit.Big(v[3+p.M+i])
		}
		args = append(args, audit.Big(v[0]), audit.Big(v[1]), audit.Big(v[2]), ins, outs)
	} else {
		for _, x := range v[:b] {
			args = append(args, audit.Big(x))
		}
	}
	if p.Audit {
		ct := audit.CipherABI{R1X: audit.Big(v[b]), R1Y: audit.Big(v[b+1]), EncryptedParents: []*big.Int{}, EncryptedOutputNfs: []*big.Int{}}
		for _, x := range v[b+2 : b+2+p.M] {
			ct.EncryptedParents = append(ct.EncryptedParents, audit.Big(x))
		}
		for _, x := range v[b+2+p.M:] {
			ct.EncryptedOutputNfs = append(ct.EncryptedOutputNfs, audit.Big(x))
		}
		args = append(args, ct)
	}
	return a.Pack(method, args...)
}
func (e *Engine) Execute(w *World, op Operation, caseID string, run int, trace bool) (*Event, error) {
	keys, er := e.keys(op.P.Name)
	if er != nil {
		return nil, er
	}
	op = w.Ready(op)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	ev, er := w.Build(op)
	if er != nil {
		return nil, er
	}
	witness, er := frontend.NewWitness(ev.Assignment, ecc.BLS12_381.ScalarField())
	if er != nil {
		return nil, er
	}
	pub, er := witness.Public()
	if er != nil {
		return nil, er
	}
	wp := []fr.Element(pub.Vector().(fr.Vector))
	if !audit.Equal(wp, ev.Public) {
		return nil, fmt.Errorf("witness public order")
	}
	wms := ms(time.Since(start))
	t := time.Now()
	proof, er := plonk.Prove(keys.CS, keys.PK, witness)
	if er != nil {
		return nil, er
	}
	pms := ms(time.Since(t))
	t = time.Now()
	raw := proof.(interface{ MarshalSolidity() []byte }).MarshalSolidity()
	data, er := eventData(e.ABI, ev, raw)
	if er != nil {
		return nil, er
	}
	_, tx, er := e.send(caseID, &e.Ledger, data)
	if er != nil {
		return nil, er
	}
	e.LastReceipt = time.Now()
	txms := ms(time.Since(t))
	end := ms(time.Since(start))
	runtime.ReadMemStats(&after)
	row := Measurement{Case: caseID, Profile: op.P.Name, Variant: map[bool]string{true: "audit", false: "plain"}[op.P.Audit], Run: run, Warmup: run == 0, WitnessMS: wms, ProveMS: pms, TransactionMS: txms, E2EMS: end, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Constraints: keys.Relation.Constraints, PublicInputs: len(ev.Public), Tx: tx}
	if er = w.Commit(ev); er != nil {
		return nil, er
	}
	idx := len(e.Rows)
	e.Rows = append(e.Rows, row)
	e.Sequence++
	path := filepath.Join(e.Root, ArtifactDir, "proofs", e.RunTag, fmt.Sprintf("%s-%s-%06d.json", caseID, e.Ledger.Hex(), e.Sequence))
	e.Rows[idx].ProofArtifact, _ = filepath.Rel(e.Root, path)
	diagnostic := func() error {
		t := time.Now()
		if er := plonk.Verify(proof, keys.VK, pub); er != nil {
			return er
		}
		e.Rows[idx].VerifyMS = ms(time.Since(t))
		var buf bytes.Buffer
		proof.WriteTo(&buf)
		e.Rows[idx].ProofBytes = buf.Len()
		if er := e.MatchWorld(w); er != nil {
			return er
		}
		if er := e.MatchEffects(ev); er != nil {
			return er
		}
		if trace {
			var er error
			e.Rows[idx].SSTORECount, e.Rows[idx].SSTOREGas, er = m7run.StoreTrace(e.Ctx, e.Client, tx.Hash)
			if er != nil {
				return er
			}
		}
		if e.Audit {
			if er := e.MatchRecord(ev); er != nil {
				return er
			}
		}
		e.Rows[idx].Matched = true
		return writeJSON(path, map[string]any{"profile": op.P.Name, "public": ev.Public, "proof": "0x" + hex.EncodeToString(raw), "goProof": "0x" + hex.EncodeToString(buf.Bytes()), "tx": tx})
	}
	if e.DeferDiagnostics {
		e.Pending = append(e.Pending, diagnostic)
	} else if er = diagnostic(); er != nil {
		return nil, er
	}
	return ev, nil
}
func (e *Engine) FlushDiagnostics() error {
	for _, f := range e.Pending {
		if er := f(); er != nil {
			return er
		}
	}
	e.Pending = nil
	return nil
}
func (e *Engine) MatchWorld(w *World) error {
	for _, v := range []struct {
		name string
		want *big.Int
	}{{"currentNoteRoot", audit.Big(w.Notes.Root())}, {"currentVoucherRoot", audit.Big(w.Vouchers.Root())}, {"noteLeafCount", new(big.Int).SetUint64(w.Notes.Count())}, {"voucherLeafCount", new(big.Int).SetUint64(w.Vouchers.Count())}} {
		got, er := e.view(v.name)
		if er != nil {
			return er
		}
		if got[0].(*big.Int).Cmp(v.want) != 0 {
			return fmt.Errorf("%s mismatch", v.name)
		}
	}
	return nil
}
func (e *Engine) MatchRecord(ev *Event) error {
	v, er := e.view("nextAuditRecordId")
	if er != nil {
		return er
	}
	id := new(big.Int).Sub(v[0].(*big.Int), big.NewInt(1))
	v, er = e.view("getAuditRecord", id)
	if er != nil {
		return er
	}
	r := reflect.ValueOf(v[0])
	parents := r.FieldByName("EncryptedParents")
	outs := r.FieldByName("EncryptedOutputNfs")
	if parents.Len() != ev.Profile.M || outs.Len() != ev.Profile.N {
		return fmt.Errorf("stored shape")
	}
	if r.FieldByName("R1X").Interface().(*big.Int).Cmp(audit.Big(ev.Cipher.R1.X)) != 0 || r.FieldByName("R1Y").Interface().(*big.Int).Cmp(audit.Big(ev.Cipher.R1.Y)) != 0 {
		return fmt.Errorf("stored point")
	}
	for i := range ev.Cipher.Data {
		var x *big.Int
		if i < parents.Len() {
			x = parents.Index(i).Interface().(*big.Int)
		} else {
			x = outs.Index(i - parents.Len()).Interface().(*big.Int)
		}
		if x.Cmp(audit.Big(ev.Cipher.Data[i])) != 0 {
			return fmt.Errorf("stored cipher")
		}
	}
	return nil
}

func (e *Engine) MatchEffects(ev *Event) error {
	var record *big.Int
	if e.Audit {
		v, er := e.view("nextAuditRecordId")
		if er != nil {
			return er
		}
		record = new(big.Int).Sub(v[0].(*big.Int), big.NewInt(1))
	}
	for i, n := range ev.Profile.BaseNames() {
		if n != "rvnf" && !strings.HasPrefix(n, "nf") {
			continue
		}
		name := "noteSpent"
		if n == "rvnf" {
			name = "voucherSpent"
		}
		if e.Audit {
			name += "In"
		}
		v, er := e.view(name, audit.Big(ev.Public[i]))
		if er != nil {
			return er
		}
		if e.Audit {
			if v[0].(*big.Int).Cmp(record) != 0 {
				return fmt.Errorf("spent record mismatch")
			}
		} else if !v[0].(bool) {
			return fmt.Errorf("spent flag missing")
		}
	}
	refs := []audit.Ref{}
	for _, n := range ev.Outputs {
		refs = append(refs, audit.Ref{ObjectType: 1, RawID: n.Commitment})
	}
	for _, n := range ev.VectorOutputs {
		refs = append(refs, audit.Ref{ObjectType: 1, RawID: n.CM})
	}
	if ev.Voucher != nil {
		refs = append(refs, audit.Ref{ObjectType: 2, RawID: ev.Voucher.Commitment})
	}
	if ev.Claim != nil {
		refs = append(refs, audit.Ref{ObjectType: 3, RawID: ev.Claim.Handle})
	}
	for _, ref := range refs {
		name := "commitments"
		if ref.ObjectType == 2 {
			name = "voucherCommitments"
		}
		if ref.ObjectType == 3 {
			name = "claimRegistered"
		}
		v, er := e.view(name, audit.Big(ref.RawID))
		if er != nil {
			return er
		}
		if !v[0].(bool) {
			return fmt.Errorf("output registration missing")
		}
		if e.Audit {
			v, er = e.view("producerOf", ref.ObjectType, audit.Big(ref.RawID))
			if er != nil {
				return er
			}
			if v[0].(*big.Int).Cmp(record) != 0 {
				return fmt.Errorf("producer mismatch")
			}
		}
	}
	return nil
}
func defaultPK() auditcrypto.PublicKey {
	pkg, _, er := v2case.KeyFixture()
	return must(pkg.PublicKey, er)
}
