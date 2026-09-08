package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m5case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const mnemonic = "test test test test test test test test test test test junk"

var rpcURL = "http://127.0.0.1:" + env("ANVIL_PORT", "18545")

type marshaler interface{ MarshalSolidity() []byte }
type rawContract struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type fixture struct {
	PolicyRef, ScopeRef, NoteRoot string
	NF                            [3]string
	CMOut                         [2]string
	Proof                         string `json:"proof"`
	FinalRoot                     string
	FinalCount                    uint64
}
type optimized struct {
	Words      []string `json:"words"`
	ByteLength int      `json:"byteLength"`
	SHA256     string   `json:"sha256"`
}
type Tx struct {
	Kind, Name, Case, Actor, TransactionHash, ContractAddress                   string
	BlockNumber, ReceiptGasUsed                                                 uint64
	CalldataBytes                                                               int
	WitnessMillis, ProveMillis, TxPrepareMillis, SubmitReceiptMillis, E2EMillis float64
}
type Report struct {
	GeneratedAt, Mode, RPC, GoVersion, VKHash string
	RunCount                                  int
	ChainID                                   uint64
	OptimizedVKWords, OptimizedVKBytes        int
	Transactions                              []Tx
	FinalRoot, ExpectedRoot                   string
	FinalLeafCount                            uint64
	Nullifiers                                bool
}
type sender struct {
	key     *ecdsa.PrivateKey
	address common.Address
}
type prepared struct {
	tx       *types.Transaction
	bytes    int
	duration time.Duration
}

func main() {
	mode := flag.String("mode", "gas", "gas,e2e,ablation")
	root := flag.String("root", ".", "root")
	flag.Parse()
	if e := run(*root, *mode); e != nil {
		panic(e)
	}
}
func run(root, mode string) error {
	if mode != "gas" && mode != "e2e" && mode != "ablation" {
		return fmt.Errorf("mode")
	}
	if e := cmd(root, "docker", "compose", "down", "-v"); e != nil {
		return e
	}
	if e := cmd(root, "docker", "compose", "up", "-d", "anvil"); e != nil {
		return e
	}
	defer func() { _ = cmd(root, "docker", "compose", "down", "-v") }()
	c, e := wait()
	if e != nil {
		return e
	}
	defer c.Close()
	if e = cmd(root, "docker", "compose", "run", "--rm", "foundry", "forge", "build"); e != nil {
		return e
	}
	ctx := context.Background()
	id, e := c.ChainID(ctx)
	if e != nil {
		return e
	}
	ss := make([]sender, 4)
	for i := range ss {
		ss[i], e = derive(root, i)
		if e != nil {
			return e
		}
	}
	f, e := loadFixture(filepath.Join(root, "contracts", "test", "fixtures", "m5-proofs.json"))
	if e != nil {
		return e
	}
	o, e := loadOptimized(filepath.Join(root, "artifacts", "development", "m5", "process-policy-3-2", "optimized-vk.json"))
	if e != nil {
		return e
	}
	r := Report{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: mode, RunCount: 1, ChainID: id.Uint64(), RPC: rpcURL, GoVersion: runtime.Version(), VKHash: o.SHA256, OptimizedVKWords: len(o.Words), OptimizedVKBytes: o.ByteLength}
	constantABI, constantCode, e := loadContract(root, "ProcessPolicyVerifier.sol", "ProcessPolicyVerifier")
	if e != nil {
		return e
	}
	constantAddr, t, e := deploy(ctx, c, ss[0], id, "constant-process-verifier", constantCode)
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	if mode == "ablation" {
		storageABI, storageCode, e := loadContract(root, "StorageProcessVerifier.sol", "StorageProcessVerifier")
		if e != nil {
			return e
		}
		storageAddr, t, e := deploy(ctx, c, ss[0], id, "storage-process-verifier", storageCode)
		if e != nil {
			return e
		}
		r.Transactions = append(r.Transactions, t)
		words, e := wordArray(o.Words)
		if e != nil {
			return e
		}
		hash := common.HexToHash("0x" + o.SHA256)
		data, e := storageABI.Pack("registerVK", mustBig(f.PolicyRef), words, hash)
		if e != nil {
			return e
		}
		t, e = call(ctx, c, ss[0], id, storageAddr, data, "vk", "register-storage-vk", "")
		if e != nil {
			return e
		}
		r.Transactions = append(r.Transactions, t)
		inputs := publicInputs(f)
		proof, _ := decode(f.Proof)
		data, _ = constantABI.Pack("Verify", proof, inputs)
		t, e = call(ctx, c, ss[0], id, constantAddr, data, "verify", "constant", "")
		if e != nil {
			return e
		}
		r.Transactions = append(r.Transactions, t)
		data, _ = storageABI.Pack("verifyForPolicy", mustBig(f.PolicyRef), proof, inputs)
		t, e = call(ctx, c, ss[0], id, storageAddr, data, "verify", "storage", "")
		if e != nil {
			return e
		}
		r.Transactions = append(r.Transactions, t)
		return write(root, "output/m5-verifier-ablation.json", r)
	}
	poseABI, poseCode, e := loadContract(root, "Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	_ = poseABI
	if e != nil {
		return e
	}
	poseAddr, t, e := deploy(ctx, c, ss[0], id, "poseidon2", poseCode)
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	entryABI, entryCode, e := loadContract(root, "EntryVerifier.sol", "EntryVerifier")
	_ = entryABI
	if e != nil {
		return e
	}
	entryAddr, t, e := deploy(ctx, c, ss[0], id, "entry-verifier", entryCode)
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	ledgerABI, ledgerCode, e := loadContract(root, "EntryExitLedger.sol", "EntryExitLedger")
	if e != nil {
		return e
	}
	ctor, _ := ledgerABI.Pack("", entryAddr, entryAddr, entryAddr, entryAddr, entryAddr, entryAddr, entryAddr, poseAddr)
	ledger, t, e := deploy(ctx, c, ss[0], id, "entry-exit-ledger", append(ledgerCode, ctor...))
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	for _, x := range []struct {
		actor int
		name  string
		data  []byte
	}{{0, "set-entry-issuer", pack(ledgerABI, "setEntryIssuer", ss[1].address, true)}, {0, "register-authority", pack(ledgerABI, "registerPolicyAuthority", ss[2].address)}} {
		t, e = call(ctx, c, ss[x.actor], id, ledger, x.data, "registry", x.name, "")
		if e != nil {
			return e
		}
		r.Transactions = append(r.Transactions, t)
	}
	t, e = call(ctx, c, ss[2], id, ledger, pack(ledgerABI, "reservePolicy", uint8(6)), "registry", "reserve-policy", "")
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	hash := common.HexToHash("0x" + o.SHA256)
	t, e = call(ctx, c, ss[2], id, ledger, pack(ledgerABI, "registerPolicy", mustBig(f.PolicyRef), uint8(3), uint8(2), hash, constantAddr), "registry", "register-policy", "")
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	t, e = call(ctx, c, ss[2], id, ledger, pack(ledgerABI, "setPolicyGrant", mustBig(f.PolicyRef), mustBig(f.ScopeRef), true), "registry", "grant", "")
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	s, e := m5case.Build(root)
	if e != nil {
		return e
	}
	entryLoaded, e := artifact.LoadAt(root, "m2", "entry")
	if e != nil {
		return e
	}
	processLoaded, e := artifact.LoadAt(root, "m5", "process-policy-3-2")
	if e != nil {
		return e
	}
	for _, v := range s.Entries {
		proof, wt, pt, e := liveProof(entryLoaded, v.Assignment)
		if e != nil {
			return e
		}
		start := time.Now()
		t, e = call(ctx, c, ss[1], id, ledger, pack(ledgerABI, "entry", proof, bigField(v.Note.Commitment)), "event", "entry", v.Name)
		if e != nil {
			return e
		}
		if mode == "e2e" {
			addLive(&t, wt, pt, start)
		}
		r.Transactions = append(r.Transactions, t)
	}
	var proof []byte
	var wt, pt time.Duration
	if mode == "gas" {
		proof, e = decode(f.Proof)
	} else {
		proof, wt, pt, e = liveProof(processLoaded, s.Assignment)
	}
	if e != nil {
		return e
	}
	start := time.Now()
	nf := [3]*big.Int{mustBig(f.NF[0]), mustBig(f.NF[1]), mustBig(f.NF[2])}
	outs := [2]*big.Int{mustBig(f.CMOut[0]), mustBig(f.CMOut[1])}
	t, e = call(ctx, c, ss[3], id, ledger, pack(ledgerABI, "process", proof, mustBig(f.PolicyRef), mustBig(f.ScopeRef), mustBig(f.NoteRoot), nf, outs), "event", "process", "")
	if e != nil {
		return e
	}
	if mode == "e2e" {
		addLive(&t, wt, pt, start)
	}
	r.Transactions = append(r.Transactions, t)
	t, e = call(ctx, c, ss[2], id, ledger, pack(ledgerABI, "disablePolicy", mustBig(f.PolicyRef)), "registry", "disable-policy", "")
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, t)
	rootNow, _ := viewBig(ctx, c, ledgerABI, ledger, "currentNoteRoot")
	count, _ := viewBig(ctx, c, ledgerABI, ledger, "noteLeafCount")
	spent := true
	for _, n := range nf {
		v, _ := viewBool(ctx, c, ledgerABI, ledger, "noteNullifiers", n)
		spent = spent && v
	}
	r.FinalRoot = rootNow.String()
	r.ExpectedRoot = f.FinalRoot
	r.FinalLeafCount = count.Uint64()
	r.Nullifiers = spent
	if r.FinalRoot != r.ExpectedRoot || r.FinalLeafCount != f.FinalCount || !spent {
		return fmt.Errorf("final state")
	}
	if mode == "gas" {
		if e = write(root, "output/m5-policy-registry-gas.json", r); e != nil {
			return e
		}
		return write(root, "output/m5-anvil-gas.json", r)
	}
	return write(root, "output/m5-anvil-e2e.json", r)
}
func publicInputs(f fixture) []*big.Int {
	return []*big.Int{mustBig(f.PolicyRef), mustBig(f.ScopeRef), mustBig(f.NoteRoot), mustBig(f.NF[0]), mustBig(f.NF[1]), mustBig(f.NF[2]), mustBig(f.CMOut[0]), mustBig(f.CMOut[1])}
}
func wordArray(in []string) ([38]*big.Int, error) {
	var out [38]*big.Int
	if len(in) != 38 {
		return out, fmt.Errorf("words")
	}
	for i, v := range in {
		out[i] = mustBig(v)
	}
	return out, nil
}
func liveProof(l *artifact.Loaded, a frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
	s := time.Now()
	w, e := frontend.NewWitness(a, ecc.BLS12_381.ScalarField())
	wt := time.Since(s)
	if e != nil {
		return nil, wt, 0, e
	}
	s = time.Now()
	p, e := plonk.Prove(l.CCS, l.PK, w)
	pt := time.Since(s)
	if e != nil {
		return nil, wt, pt, e
	}
	return p.(marshaler).MarshalSolidity(), wt, pt, nil
}
func addLive(t *Tx, w, p time.Duration, start time.Time) {
	t.WitnessMillis = ms(w)
	t.ProveMillis = ms(p)
	t.E2EMillis = t.WitnessMillis + t.ProveMillis + ms(time.Since(start))
}
func deploy(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, name string, data []byte) (common.Address, Tx, error) {
	p, e := prepare(ctx, c, s, id, nil, data)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	x, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	return x.ContractAddress, Tx{Kind: "deployment", Name: name, Actor: s.address.Hex(), TransactionHash: x.TxHash.Hex(), ContractAddress: x.ContractAddress.Hex(), BlockNumber: x.BlockNumber.Uint64(), ReceiptGasUsed: x.GasUsed, CalldataBytes: p.bytes, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
}
func call(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to common.Address, data []byte, kind, name, caseName string) (Tx, error) {
	p, e := prepare(ctx, c, s, id, &to, data)
	if e != nil {
		return Tx{}, e
	}
	x, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return Tx{}, e
	}
	return Tx{Kind: kind, Name: name, Case: caseName, Actor: s.address.Hex(), TransactionHash: x.TxHash.Hex(), BlockNumber: x.BlockNumber.Uint64(), ReceiptGasUsed: x.GasUsed, CalldataBytes: p.bytes, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
}
func prepare(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to *common.Address, data []byte) (prepared, error) {
	st := time.Now()
	n, e := c.PendingNonceAt(ctx, s.address)
	if e != nil {
		return prepared{}, e
	}
	price, e := c.SuggestGasPrice(ctx)
	if e != nil {
		return prepared{}, e
	}
	g, e := c.EstimateGas(ctx, ethereum.CallMsg{From: s.address, To: to, Data: data})
	if e != nil {
		return prepared{}, e
	}
	var tx *types.Transaction
	if to == nil {
		tx = types.NewContractCreation(n, big.NewInt(0), g, price, data)
	} else {
		tx = types.NewTransaction(n, *to, big.NewInt(0), g, price, data)
	}
	signed, e := types.SignTx(tx, types.LatestSignerForChainID(id), s.key)
	return prepared{signed, len(data), time.Since(st)}, e
}
func submit(ctx context.Context, c *ethclient.Client, tx *types.Transaction) (*types.Receipt, time.Duration, error) {
	st := time.Now()
	if e := c.SendTransaction(ctx, tx); e != nil {
		return nil, 0, e
	}
	end := time.Now().Add(30 * time.Second)
	for time.Now().Before(end) {
		r, e := c.TransactionReceipt(ctx, tx.Hash())
		if e == nil {
			if r.Status != 1 {
				return nil, 0, fmt.Errorf("revert")
			}
			return r, time.Since(st), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("timeout")
}
func loadContract(root, source, name string) (abi.ABI, []byte, error) {
	b, e := os.ReadFile(filepath.Join(root, "contracts", "out", source, name+".json"))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	var r rawContract
	if e = json.Unmarshal(b, &r); e != nil {
		return abi.ABI{}, nil, e
	}
	a, e := abi.JSON(bytes.NewReader(r.ABI))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	code, e := decode(r.Bytecode.Object)
	return a, code, e
}
func loadFixture(p string) (fixture, error) {
	b, e := os.ReadFile(p)
	var f fixture
	if e == nil {
		e = json.Unmarshal(b, &f)
	}
	return f, e
}
func loadOptimized(p string) (optimized, error) {
	b, e := os.ReadFile(p)
	var o optimized
	if e == nil {
		e = json.Unmarshal(b, &o)
	}
	return o, e
}
func write(root, p string, v any) error { return artifact.WriteJSON(filepath.Join(root, p), v) }
func decode(v string) ([]byte, error)   { return hex.DecodeString(strings.TrimPrefix(v, "0x")) }
func mustBig(v string) *big.Int {
	x, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic(v)
	}
	return x
}
func bigField(v fr.Element) *big.Int { return v.BigInt(new(big.Int)) }
func pack(a abi.ABI, m string, args ...any) []byte {
	b, e := a.Pack(m, args...)
	if e != nil {
		panic(e)
	}
	return b
}
func viewBig(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string, args ...any) (*big.Int, error) {
	data, _ := a.Pack(m, args...)
	out, e := c.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if e != nil {
		return nil, e
	}
	v, e := a.Unpack(m, out)
	if e != nil {
		return nil, e
	}
	return v[0].(*big.Int), nil
}
func viewBool(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string, args ...any) (bool, error) {
	data, _ := a.Pack(m, args...)
	out, e := c.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if e != nil {
		return false, e
	}
	v, e := a.Unpack(m, out)
	if e != nil {
		return false, e
	}
	return v[0].(bool), nil
}
func derive(root string, i int) (sender, error) {
	o, e := out(root, "docker", "compose", "run", "--rm", "foundry", "cast", "wallet", "private-key", "--mnemonic", mnemonic, "--mnemonic-index", fmt.Sprint(i))
	if e != nil {
		return sender{}, e
	}
	k, e := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(o), "0x"))
	if e != nil {
		return sender{}, e
	}
	return sender{k, crypto.PubkeyToAddress(k.PublicKey)}, nil
}
func wait() (*ethclient.Client, error) {
	end := time.Now().Add(30 * time.Second)
	for time.Now().Before(end) {
		c, e := ethclient.Dial(rpcURL)
		if e == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, e = c.BlockNumber(ctx)
			cancel()
			if e == nil {
				return c, nil
			}
			c.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("anvil")
}
func cmd(dir, n string, args ...string) error {
	x := exec.Command(n, args...)
	x.Dir = dir
	x.Stdout, x.Stderr = os.Stdout, os.Stderr
	return x.Run()
}
func out(dir, n string, args ...string) (string, error) {
	x := exec.Command(n, args...)
	x.Dir = dir
	b, e := x.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("%w: %s", e, b)
	}
	return string(b), nil
}
func ms(v time.Duration) float64 { return float64(v.Microseconds()) / 1000 }
func env(n, f string) string {
	if v := os.Getenv(n); v != "" {
		return v
	}
	return f
}
