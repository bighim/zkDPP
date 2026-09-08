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
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m4case"
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

type solidityMarshaler interface{ MarshalSolidity() []byte }
type rawContract struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type pf struct {
	Proof string `json:"proof"`
}
type ef struct {
	Commitment string
	pf
}
type mf struct {
	NoteRoot, NF1, NF2, CMOut string
	pf
}
type sf struct {
	NoteRoot, NF, CMOut1, CMOut2 string
	pf
}
type fixtures struct {
	Entries []ef
	Merge   mf
	Split   sf
}
type Tx struct {
	Kind, Name, Case, Actor, TransactionHash, ContractAddress                   string
	BlockNumber, ReceiptGasUsed                                                 uint64
	CalldataBytes                                                               int
	WitnessMillis, ProveMillis, TxPrepareMillis, SubmitReceiptMillis, E2EMillis float64
}
type Final struct {
	LeafCount                 uint64
	CurrentRoot, ExpectedRoot string
	Nullifiers, PathMatches   bool
}
type Report struct {
	GeneratedAt, Mode, RPC, GoVersion string
	RunCount                          int
	ChainID                           uint64
	Transactions                      []Tx
	FinalState                        Final
}
type sender struct {
	key     *ecdsa.PrivateKey
	address common.Address
}
type prepared struct {
	tx       *types.Transaction
	calldata int
	duration time.Duration
}

func main() {
	mode := flag.String("mode", "gas", "gas or e2e")
	root := flag.String("root", ".", "root")
	flag.Parse()
	if *mode != "gas" && *mode != "e2e" {
		panic("mode")
	}
	if e := run(*root, *mode); e != nil {
		panic(e)
	}
}
func run(root, mode string) error {
	if e := command(root, "docker", "compose", "down", "-v"); e != nil {
		return e
	}
	if e := command(root, "docker", "compose", "up", "-d", "anvil"); e != nil {
		return e
	}
	defer func() { _ = command(root, "docker", "compose", "down", "-v") }()
	client, e := waitClient()
	if e != nil {
		return e
	}
	defer client.Close()
	if e = command(root, "docker", "compose", "run", "--rm", "foundry", "forge", "build"); e != nil {
		return e
	}
	senders := make([]sender, 3)
	for i := range senders {
		senders[i], e = derive(root, i)
		if e != nil {
			return e
		}
	}
	ctx := context.Background()
	id, e := client.ChainID(ctx)
	if e != nil {
		return e
	}
	r := Report{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: mode, RunCount: 1, ChainID: id.Uint64(), RPC: rpcURL, GoVersion: runtime.Version()}
	deploys := []struct{ source, name, label string }{{"Poseidon2BLS12381.sol", "Poseidon2BLS12381", "poseidon2"}, {"EntryVerifier.sol", "EntryVerifier", "entry-verifier"}, {"PrivateSpendVerifier.sol", "PrivateSpendVerifier", "private-spend-verifier"}, {"TransferVerifier.sol", "TransferVerifier", "transfer-verifier"}, {"ProceedVerifier.sol", "ProceedVerifier", "proceed-verifier"}, {"RecallVerifier.sol", "RecallVerifier", "recall-verifier"}, {"MergeVerifier.sol", "MergeVerifier", "merge-verifier"}, {"SplitVerifier.sol", "SplitVerifier", "split-verifier"}}
	addresses := make([]common.Address, len(deploys))
	var tx Tx
	for i, v := range deploys {
		_, code, err := loadContract(root, v.source, v.name)
		if err != nil {
			return err
		}
		addresses[i], tx, err = deploy(ctx, client, senders[0], id, v.label, code)
		if err != nil {
			return err
		}
		r.Transactions = append(r.Transactions, tx)
	}
	ledgerABI, ledgerCode, e := loadContract(root, "EntryExitLedger.sol", "EntryExitLedger")
	if e != nil {
		return e
	}
	ctor, e := ledgerABI.Pack("", addresses[1], addresses[2], addresses[3], addresses[4], addresses[5], addresses[6], addresses[7], addresses[0])
	if e != nil {
		return e
	}
	ledger, tx, e := deploy(ctx, client, senders[0], id, "entry-exit-ledger", append(ledgerCode, ctor...))
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, tx)
	data, _ := ledgerABI.Pack("setEntryIssuer", senders[1].address, true)
	tx, e = call(ctx, client, senders[0], id, ledger, data, "authorization", "set-entry-issuer", "")
	if e != nil {
		return e
	}
	r.Transactions = append(r.Transactions, tx)
	s, e := m4case.Build(root)
	if e != nil {
		return e
	}
	f, e := loadFixtures(filepath.Join(root, "contracts", "test", "fixtures", "m4-proofs.json"))
	if e != nil {
		return e
	}
	entry, e := artifact.LoadAt(root, "m2", "entry")
	if e != nil {
		return e
	}
	merge, e := artifact.LoadAt(root, "m4", "merge")
	if e != nil {
		return e
	}
	split, e := artifact.LoadAt(root, "m4", "split")
	if e != nil {
		return e
	}
	for i, v := range s.Entries {
		p, wt, pt, err := selectProof(mode, f.Entries[i].Proof, entry, v.Assignment)
		if err != nil {
			return err
		}
		start := time.Now()
		data, _ = ledgerABI.Pack("entry", p, fieldBig(v.Note.Commitment))
		result, err := call(ctx, client, senders[1], id, ledger, data, "event", "entry", v.Name)
		if err != nil {
			return err
		}
		addLive(&result, mode, wt, pt, start)
		r.Transactions = append(r.Transactions, result)
	}
	p, wt, pt, e := selectProof(mode, f.Merge.Proof, merge, s.Merge.Assignment)
	if e != nil {
		return e
	}
	start := time.Now()
	data, _ = ledgerABI.Pack("merge", p, bigVar(s.Merge.Assignment.NoteRoot), bigVar(s.Merge.Assignment.NF1), bigVar(s.Merge.Assignment.NF2), fieldBig(s.Merge.Output.Commitment))
	result, e := call(ctx, client, senders[1], id, ledger, data, "event", "merge", "")
	if e != nil {
		return e
	}
	addLive(&result, mode, wt, pt, start)
	r.Transactions = append(r.Transactions, result)
	p, wt, pt, e = selectProof(mode, f.Split.Proof, split, s.Split.Assignment)
	if e != nil {
		return e
	}
	start = time.Now()
	data, _ = ledgerABI.Pack("split", p, bigVar(s.Split.Assignment.NoteRoot), bigVar(s.Split.Assignment.NF), fieldBig(s.Split.Outputs[0].Commitment), fieldBig(s.Split.Outputs[1].Commitment))
	result, e = call(ctx, client, senders[1], id, ledger, data, "event", "split", "")
	if e != nil {
		return e
	}
	addLive(&result, mode, wt, pt, start)
	r.Transactions = append(r.Transactions, result)
	final, e := validate(ctx, client, ledgerABI, ledger, s)
	if e != nil {
		return e
	}
	r.FinalState = final
	path := filepath.Join(root, "output", "m4-anvil-"+mode+".json")
	if e = artifact.WriteJSON(path, r); e != nil {
		return e
	}
	fmt.Println("wrote", path)
	return nil
}
func selectProof(mode, fixed string, l *artifact.Loaded, a frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
	if mode == "gas" {
		p, e := decode(fixed)
		return p, 0, 0, e
	}
	ws := time.Now()
	w, e := frontend.NewWitness(a, ecc.BLS12_381.ScalarField())
	wt := time.Since(ws)
	if e != nil {
		return nil, wt, 0, e
	}
	ps := time.Now()
	p, e := plonk.Prove(l.CCS, l.PK, w)
	pt := time.Since(ps)
	if e != nil {
		return nil, wt, pt, e
	}
	return p.(solidityMarshaler).MarshalSolidity(), wt, pt, nil
}
func addLive(r *Tx, mode string, wt, pt time.Duration, start time.Time) {
	if mode == "e2e" {
		r.WitnessMillis = ms(wt)
		r.ProveMillis = ms(pt)
		r.E2EMillis = r.WitnessMillis + r.ProveMillis + ms(time.Since(start))
	}
}
func deploy(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, name string, data []byte) (common.Address, Tx, error) {
	p, e := prepare(ctx, c, s, id, nil, data)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	receipt, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	return receipt.ContractAddress, Tx{Kind: "deployment", Name: name, Actor: s.address.Hex(), TransactionHash: receipt.TxHash.Hex(), ContractAddress: receipt.ContractAddress.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), ReceiptGasUsed: receipt.GasUsed, CalldataBytes: p.calldata, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
}
func call(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to common.Address, data []byte, kind, name, caseName string) (Tx, error) {
	p, e := prepare(ctx, c, s, id, &to, data)
	if e != nil {
		return Tx{}, e
	}
	receipt, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return Tx{}, e
	}
	return Tx{Kind: kind, Name: name, Case: caseName, Actor: s.address.Hex(), TransactionHash: receipt.TxHash.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), ReceiptGasUsed: receipt.GasUsed, CalldataBytes: p.calldata, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
}
func prepare(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to *common.Address, data []byte) (prepared, error) {
	start := time.Now()
	nonce, e := c.PendingNonceAt(ctx, s.address)
	if e != nil {
		return prepared{}, e
	}
	price, e := c.SuggestGasPrice(ctx)
	if e != nil {
		return prepared{}, e
	}
	gas, e := c.EstimateGas(ctx, ethereum.CallMsg{From: s.address, To: to, Data: data})
	if e != nil {
		return prepared{}, e
	}
	var tx *types.Transaction
	if to == nil {
		tx = types.NewContractCreation(nonce, big.NewInt(0), gas, price, data)
	} else {
		tx = types.NewTransaction(nonce, *to, big.NewInt(0), gas, price, data)
	}
	signed, e := types.SignTx(tx, types.LatestSignerForChainID(id), s.key)
	return prepared{signed, len(data), time.Since(start)}, e
}
func submit(ctx context.Context, c *ethclient.Client, tx *types.Transaction) (*types.Receipt, time.Duration, error) {
	start := time.Now()
	if e := c.SendTransaction(ctx, tx); e != nil {
		return nil, 0, e
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		r, e := c.TransactionReceipt(ctx, tx.Hash())
		if e == nil {
			if r.Status != types.ReceiptStatusSuccessful {
				return nil, 0, fmt.Errorf("revert")
			}
			return r, time.Since(start), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("timeout")
}
func validate(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, s *m4case.Scenario) (Final, error) {
	count, e := callBig(ctx, c, a, addr, "noteLeafCount")
	if e != nil {
		return Final{}, e
	}
	root, e := callBig(ctx, c, a, addr, "currentNoteRoot")
	if e != nil {
		return Final{}, e
	}
	n1, _ := callBool(ctx, c, a, addr, "noteNullifiers", bigVar(s.Merge.Assignment.NF1))
	n2, _ := callBool(ctx, c, a, addr, "noteNullifiers", bigVar(s.Merge.Assignment.NF2))
	n3, _ := callBool(ctx, c, a, addr, "noteNullifiers", bigVar(s.Split.Assignment.NF))
	match, e := pathMatches(ctx, c, a, addr, 4, s.FinalPaths[4])
	if e != nil {
		return Final{}, e
	}
	want := fieldBig(s.FinalRoot)
	if count.Uint64() != s.FinalCount || root.Cmp(want) != 0 || !n1 || !n2 || !n3 || !match {
		return Final{}, fmt.Errorf("final state")
	}
	return Final{count.Uint64(), root.String(), want.String(), true, match}, nil
}
func pathMatches(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, index uint64, path merkle.Path) (bool, error) {
	data, _ := a.Pack("getNotePath", new(big.Int).SetUint64(index))
	out, e := c.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if e != nil {
		return false, e
	}
	v, e := a.Unpack("getNotePath", out)
	if e != nil {
		return false, e
	}
	got := v[1].([]*big.Int)
	for i := range got {
		if got[i].Cmp(fieldBig(path.Siblings[i])) != 0 {
			return false, nil
		}
	}
	return true, nil
}
func callBig(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string, args ...any) (*big.Int, error) {
	data, e := a.Pack(m, args...)
	if e != nil {
		return nil, e
	}
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
func callBool(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string, args ...any) (bool, error) {
	data, e := a.Pack(m, args...)
	if e != nil {
		return false, e
	}
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
func loadContract(root, source, name string) (abi.ABI, []byte, error) {
	b, e := os.ReadFile(filepath.Join(root, "contracts", "out", source, name+".json"))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	var raw rawContract
	if e = json.Unmarshal(b, &raw); e != nil {
		return abi.ABI{}, nil, e
	}
	a, e := abi.JSON(bytes.NewReader(raw.ABI))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	code, e := decode(raw.Bytecode.Object)
	return a, code, e
}
func loadFixtures(path string) (fixtures, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return fixtures{}, e
	}
	var f fixtures
	e = json.Unmarshal(b, &f)
	return f, e
}
func decode(s string) ([]byte, error) { return hex.DecodeString(strings.TrimPrefix(s, "0x")) }
func derive(root string, index int) (sender, error) {
	out, e := output(root, "docker", "compose", "run", "--rm", "foundry", "cast", "wallet", "private-key", "--mnemonic", mnemonic, "--mnemonic-index", fmt.Sprint(index))
	if e != nil {
		return sender{}, e
	}
	key, e := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(out), "0x"))
	if e != nil {
		return sender{}, e
	}
	return sender{key, crypto.PubkeyToAddress(key.PublicKey)}, nil
}
func waitClient() (*ethclient.Client, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
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
func command(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}
func output(dir, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	b, e := c.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("%w: %s", e, b)
	}
	return string(b), nil
}
func fieldBig(v fr.Element) *big.Int      { return v.BigInt(new(big.Int)) }
func bigVar(v frontend.Variable) *big.Int { return fieldBig(v.(fr.Element)) }
func ms(v time.Duration) float64          { return float64(v.Microseconds()) / 1000 }
func env(n, f string) string {
	if v := os.Getenv(n); v != "" {
		return v
	}
	return f
}
