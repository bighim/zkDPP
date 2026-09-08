package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m3case"
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
)

const mnemonic = "test test test test test test test test test test test junk"

var rpcURL = "http://127.0.0.1:" + envOrDefault("ANVIL_PORT", "18545")

type solidityMarshaler interface{ MarshalSolidity() []byte }
type contractArtifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type proofFixture struct {
	Proof string `json:"proof"`
}
type entryFixture struct {
	Commitment string `json:"commitment"`
	proofFixture
}
type transferFixture struct {
	NoteRoot, Nullifier, VoucherCommitment, ChangeCommitment string
	TransferEpoch, DeltaEpoch                                uint64
	proofFixture
}
type resolutionFixture struct {
	VoucherRoot, VoucherNullifier, OutputCommitment string
	CurrentEpoch                                    uint64
	proofFixture
}
type fixedFixtures struct {
	Entries []entryFixture    `json:"entries"`
	Partial transferFixture   `json:"partialTransfer"`
	Proceed resolutionFixture `json:"proceed"`
	Full    transferFixture   `json:"fullTransfer"`
	Recall  resolutionFixture `json:"recall"`
}
type TxResult struct {
	Kind, Name, Case, Actor, TransactionHash, ContractAddress                   string
	BlockNumber, ReceiptGasUsed                                                 uint64
	CalldataBytes                                                               int
	WitnessMillis, ProveMillis, TxPrepareMillis, SubmitReceiptMillis, E2EMillis float64
}
type FinalState struct {
	NoteLeafCount, VoucherLeafCount                                            uint64
	CurrentNoteRoot, ExpectedNoteRoot, CurrentVoucherRoot, ExpectedVoucherRoot string
	NoteNullifiers, VoucherNullifiers                                          bool
	NotePathMatches, VoucherPathMatches                                        bool
}
type Report struct {
	GeneratedAt, Mode, RPC, GoVersion string
	RunCount                          int
	ChainID                           uint64
	Transactions                      []TxResult
	FinalState                        FinalState
}
type sender struct {
	key     *ecdsa.PrivateKey
	address common.Address
}
type preparedTx struct {
	tx       *types.Transaction
	calldata int
	prepare  time.Duration
}

func main() {
	mode := flag.String("mode", "gas", "gas or e2e")
	root := flag.String("root", ".", "root")
	flag.Parse()
	if *mode != "gas" && *mode != "e2e" {
		panic("mode")
	}
	if err := run(*root, *mode); err != nil {
		panic(err)
	}
}

func run(root, mode string) error {
	if err := command(root, "docker", "compose", "down", "-v"); err != nil {
		return err
	}
	if err := command(root, "docker", "compose", "up", "-d", "anvil"); err != nil {
		return err
	}
	defer func() { _ = command(root, "docker", "compose", "down", "-v") }()
	client, err := waitClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err = command(root, "docker", "compose", "run", "--rm", "foundry", "forge", "build"); err != nil {
		return err
	}
	senders := make([]sender, 4)
	for i := range senders {
		senders[i], err = deriveSender(root, i)
		if err != nil {
			return err
		}
	}
	ctx := context.Background()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	report := Report{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: mode, RunCount: 1, ChainID: chainID.Uint64(), RPC: rpcURL, GoVersion: runtime.Version()}
	typesToDeploy := []struct{ source, name, label string }{{"Poseidon2BLS12381.sol", "Poseidon2BLS12381", "poseidon2"}, {"EntryVerifier.sol", "EntryVerifier", "entry-verifier"}, {"PrivateSpendVerifier.sol", "PrivateSpendVerifier", "private-spend-verifier"}, {"TransferVerifier.sol", "TransferVerifier", "transfer-verifier"}, {"ProceedVerifier.sol", "ProceedVerifier", "proceed-verifier"}, {"RecallVerifier.sol", "RecallVerifier", "recall-verifier"}}
	addresses := make([]common.Address, len(typesToDeploy))
	var ledgerABI abi.ABI
	var ledgerCode []byte
	var tx TxResult
	for i, item := range typesToDeploy {
		_, code, e := loadContract(root, item.source, item.name)
		if e != nil {
			return e
		}
		addresses[i], tx, e = deploy(ctx, client, senders[0], chainID, item.label, code)
		if e != nil {
			return e
		}
		report.Transactions = append(report.Transactions, tx)
	}
	ledgerABI, ledgerCode, err = loadContract(root, "EntryExitLedger.sol", "EntryExitLedger")
	if err != nil {
		return err
	}
	constructor, err := ledgerABI.Pack("", addresses[1], addresses[2], addresses[3], addresses[4], addresses[5], addresses[2], addresses[2], addresses[0])
	if err != nil {
		return err
	}
	ledgerAddr, tx, err := deploy(ctx, client, senders[0], chainID, "entry-exit-ledger", append(ledgerCode, constructor...))
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	data, err := ledgerABI.Pack("setEntryIssuer", senders[1].address, true)
	if err != nil {
		return err
	}
	tx, err = sendCall(ctx, client, senders[0], chainID, ledgerAddr, data, "authorization", "set-entry-issuer", "")
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	scenario, err := m3case.Build(root)
	if err != nil {
		return err
	}
	fixtures, err := loadFixtures(filepath.Join(root, "contracts", "test", "fixtures", "m3-proofs.json"))
	if err != nil {
		return err
	}
	entryLoaded, err := artifact.LoadAt(root, "m2", "entry")
	if err != nil {
		return err
	}
	transferLoaded, err := artifact.LoadAt(root, "m3", "transfer")
	if err != nil {
		return err
	}
	proceedLoaded, err := artifact.LoadAt(root, "m3", "proceed")
	if err != nil {
		return err
	}
	recallLoaded, err := artifact.LoadAt(root, "m3", "recall")
	if err != nil {
		return err
	}
	for i, e := range scenario.Entries {
		proof, wt, pt, err := selectProof(mode, fixtures.Entries[i].Proof, entryLoaded, e.Assignment)
		if err != nil {
			return err
		}
		start := time.Now()
		data, err := ledgerABI.Pack("entry", proof, bigOf(e.Note.Commitment))
		if err != nil {
			return err
		}
		result, err := sendCall(ctx, client, senders[1], chainID, ledgerAddr, data, "event", "entry", e.Name)
		if err != nil {
			return err
		}
		live(&result, mode, wt, pt, start)
		report.Transactions = append(report.Transactions, result)
	}
	proof, wt, pt, err := selectProof(mode, fixtures.Partial.Proof, transferLoaded, scenario.Partial.Assignment)
	if err != nil {
		return err
	}
	start := time.Now()
	data, err = ledgerABI.Pack("transfer", proof, bigOfVar(scenario.Partial.Assignment.NoteRoot), bigOfVar(scenario.Partial.Assignment.NF), bigOf(scenario.Partial.Voucher.Commitment), bigOf(scenario.Partial.Change.Commitment), new(big.Int).SetUint64(m3case.TransferEpoch), new(big.Int).SetUint64(m3case.DeltaEpoch))
	if err != nil {
		return err
	}
	result, err := sendCall(ctx, client, senders[1], chainID, ledgerAddr, data, "event", "transfer", "partial")
	if err != nil {
		return err
	}
	live(&result, mode, wt, pt, start)
	report.Transactions = append(report.Transactions, result)
	proof, wt, pt, err = selectProof(mode, fixtures.Proceed.Proof, proceedLoaded, scenario.Proceed.Assignment)
	if err != nil {
		return err
	}
	start = time.Now()
	data, err = ledgerABI.Pack("proceed", proof, bigOfVar(scenario.Proceed.Assignment.VoucherRoot), bigOfVar(scenario.Proceed.Assignment.RVNF), bigOf(scenario.Proceed.Output.Commitment))
	if err != nil {
		return err
	}
	result, err = sendCall(ctx, client, senders[2], chainID, ledgerAddr, data, "event", "proceed", "partial")
	if err != nil {
		return err
	}
	live(&result, mode, wt, pt, start)
	report.Transactions = append(report.Transactions, result)
	proof, wt, pt, err = selectProof(mode, fixtures.Full.Proof, transferLoaded, scenario.Full.Assignment)
	if err != nil {
		return err
	}
	start = time.Now()
	data, err = ledgerABI.Pack("transfer", proof, bigOfVar(scenario.Full.Assignment.NoteRoot), bigOfVar(scenario.Full.Assignment.NF), bigOf(scenario.Full.Voucher.Commitment), bigOf(scenario.Full.Change.Commitment), new(big.Int).SetUint64(m3case.TransferEpoch), new(big.Int).SetUint64(m3case.DeltaEpoch))
	if err != nil {
		return err
	}
	result, err = sendCall(ctx, client, senders[1], chainID, ledgerAddr, data, "event", "transfer", "full")
	if err != nil {
		return err
	}
	live(&result, mode, wt, pt, start)
	report.Transactions = append(report.Transactions, result)
	if err := warp(ctx, client, m3case.RecallEpoch*600); err != nil {
		return err
	}
	proof, wt, pt, err = selectProof(mode, fixtures.Recall.Proof, recallLoaded, scenario.Recall.Assignment)
	if err != nil {
		return err
	}
	start = time.Now()
	data, err = ledgerABI.Pack("recall", proof, bigOfVar(scenario.Recall.Assignment.VoucherRoot), bigOfVar(scenario.Recall.Assignment.RVNF), bigOf(scenario.Recall.Output.Commitment), new(big.Int).SetUint64(m3case.RecallEpoch))
	if err != nil {
		return err
	}
	result, err = sendCall(ctx, client, senders[1], chainID, ledgerAddr, data, "event", "recall", "full")
	if err != nil {
		return err
	}
	live(&result, mode, wt, pt, start)
	report.Transactions = append(report.Transactions, result)
	state, err := validateState(ctx, client, ledgerABI, ledgerAddr, scenario)
	if err != nil {
		return err
	}
	report.FinalState = state
	path := filepath.Join(root, "output", "m3-anvil-"+mode+".json")
	if err := artifact.WriteJSON(path, report); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

func selectProof(mode, fixed string, l *artifact.Loaded, a frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
	if mode == "gas" {
		p, e := decodeHex(fixed)
		return p, 0, 0, e
	}
	return liveProof(l, a)
}
func live(r *TxResult, mode string, wt, pt time.Duration, start time.Time) {
	if mode == "e2e" {
		r.WitnessMillis = ms(wt)
		r.ProveMillis = ms(pt)
		r.E2EMillis = ms(time.Since(start)) + r.WitnessMillis + r.ProveMillis
	}
}
func liveProof(l *artifact.Loaded, a frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
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
func deploy(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, name string, data []byte) (common.Address, TxResult, error) {
	p, e := prepare(ctx, c, s, id, nil, data)
	if e != nil {
		return common.Address{}, TxResult{}, e
	}
	r, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return common.Address{}, TxResult{}, e
	}
	return r.ContractAddress, TxResult{Kind: "deployment", Name: name, Actor: s.address.Hex(), TransactionHash: r.TxHash.Hex(), ContractAddress: r.ContractAddress.Hex(), BlockNumber: r.BlockNumber.Uint64(), ReceiptGasUsed: r.GasUsed, CalldataBytes: p.calldata, TxPrepareMillis: ms(p.prepare), SubmitReceiptMillis: ms(d)}, nil
}
func sendCall(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to common.Address, data []byte, kind, name, caseName string) (TxResult, error) {
	p, e := prepare(ctx, c, s, id, &to, data)
	if e != nil {
		return TxResult{}, e
	}
	r, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return TxResult{}, e
	}
	return TxResult{Kind: kind, Name: name, Case: caseName, Actor: s.address.Hex(), TransactionHash: r.TxHash.Hex(), BlockNumber: r.BlockNumber.Uint64(), ReceiptGasUsed: r.GasUsed, CalldataBytes: p.calldata, TxPrepareMillis: ms(p.prepare), SubmitReceiptMillis: ms(d)}, nil
}
func prepare(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to *common.Address, data []byte) (preparedTx, error) {
	start := time.Now()
	n, e := c.PendingNonceAt(ctx, s.address)
	if e != nil {
		return preparedTx{}, e
	}
	price, e := c.SuggestGasPrice(ctx)
	if e != nil {
		return preparedTx{}, e
	}
	gas, e := c.EstimateGas(ctx, ethereum.CallMsg{From: s.address, To: to, Data: data})
	if e != nil {
		return preparedTx{}, e
	}
	var tx *types.Transaction
	if to == nil {
		tx = types.NewContractCreation(n, big.NewInt(0), gas, price, data)
	} else {
		tx = types.NewTransaction(n, *to, big.NewInt(0), gas, price, data)
	}
	signed, e := types.SignTx(tx, types.LatestSignerForChainID(id), s.key)
	return preparedTx{signed, len(data), time.Since(start)}, e
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
				return nil, 0, fmt.Errorf("reverted %s", tx.Hash())
			}
			return r, time.Since(start), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("receipt timeout")
}
func validateState(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, s *m3case.Scenario) (FinalState, error) {
	nc, e := callUint(ctx, c, a, addr, "noteLeafCount")
	if e != nil {
		return FinalState{}, e
	}
	vc, e := callUint(ctx, c, a, addr, "voucherLeafCount")
	if e != nil {
		return FinalState{}, e
	}
	nr, e := callBig(ctx, c, a, addr, "currentNoteRoot")
	if e != nil {
		return FinalState{}, e
	}
	vr, e := callBig(ctx, c, a, addr, "currentVoucherRoot")
	if e != nil {
		return FinalState{}, e
	}
	n1, e := callBool(ctx, c, a, addr, "noteNullifiers", bigOfVar(s.Partial.Assignment.NF))
	if e != nil {
		return FinalState{}, e
	}
	n2, e := callBool(ctx, c, a, addr, "noteNullifiers", bigOfVar(s.Full.Assignment.NF))
	if e != nil {
		return FinalState{}, e
	}
	v1, e := callBool(ctx, c, a, addr, "voucherNullifiers", bigOfVar(s.Proceed.Assignment.RVNF))
	if e != nil {
		return FinalState{}, e
	}
	v2, e := callBool(ctx, c, a, addr, "voucherNullifiers", bigOfVar(s.Recall.Assignment.RVNF))
	if e != nil {
		return FinalState{}, e
	}
	np, e := pathMatches(ctx, c, a, addr, "getNotePath", 1, s.FinalNotePaths[1])
	if e != nil {
		return FinalState{}, e
	}
	vp, e := pathMatches(ctx, c, a, addr, "getVoucherPath", 1, s.FinalVoucherPaths[1])
	if e != nil {
		return FinalState{}, e
	}
	en, ev := bigOf(s.FinalNoteRoot), bigOf(s.FinalVoucherRoot)
	if nc != s.FinalNoteCount || vc != s.FinalVoucherCount || nr.Cmp(en) != 0 || vr.Cmp(ev) != 0 || !n1 || !n2 || !v1 || !v2 || !np || !vp {
		return FinalState{}, fmt.Errorf("unexpected final state")
	}
	return FinalState{nc, vc, nr.String(), en.String(), vr.String(), ev.String(), n1 && n2, v1 && v2, np, vp}, nil
}
func pathMatches(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, method string, index uint64, want interface{}) (bool, error) {
	data, _ := a.Pack(method, new(big.Int).SetUint64(index))
	out, e := c.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if e != nil {
		return false, e
	}
	values, e := a.Unpack(method, out)
	if e != nil {
		return false, e
	}
	got := values[1].([]*big.Int)
	path := want.(merkle.Path)
	if len(got) != len(path.Siblings) {
		return false, nil
	}
	for i := range got {
		if got[i].Cmp(bigOf(path.Siblings[i])) != 0 {
			return false, nil
		}
	}
	return true, nil
}
func callUint(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string) (uint64, error) {
	v, e := callBig(ctx, c, a, addr, m)
	if e != nil {
		return 0, e
	}
	return v.Uint64(), nil
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
func warp(ctx context.Context, c *ethclient.Client, t uint64) error {
	var out any
	if e := c.Client().CallContext(ctx, &out, "evm_setNextBlockTimestamp", t); e != nil {
		return e
	}
	return c.Client().CallContext(ctx, &out, "evm_mine")
}
func loadContract(root, source, name string) (abi.ABI, []byte, error) {
	encoded, e := os.ReadFile(filepath.Join(root, "contracts", "out", source, name+".json"))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	var raw contractArtifact
	if e = json.Unmarshal(encoded, &raw); e != nil {
		return abi.ABI{}, nil, e
	}
	parsed, e := abi.JSON(bytes.NewReader(raw.ABI))
	if e != nil {
		return abi.ABI{}, nil, e
	}
	code, e := decodeHex(raw.Bytecode.Object)
	return parsed, code, e
}
func loadFixtures(path string) (fixedFixtures, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return fixedFixtures{}, e
	}
	var v fixedFixtures
	e = json.Unmarshal(b, &v)
	return v, e
}
func decodeHex(v string) ([]byte, error) { return hex.DecodeString(strings.TrimPrefix(v, "0x")) }
func deriveSender(root string, index int) (sender, error) {
	out, e := outputCommand(root, "docker", "compose", "run", "--rm", "foundry", "cast", "wallet", "private-key", "--mnemonic", mnemonic, "--mnemonic-index", fmt.Sprint(index))
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
	return nil, fmt.Errorf("anvil not ready")
}
func command(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
func outputCommand(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, e := cmd.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("%s: %w: %s", name, e, b)
	}
	return string(b), nil
}
func bigOf(v fr.Element) *big.Int           { return v.BigInt(new(big.Int)) }
func bigOfVar(v frontend.Variable) *big.Int { return bigOf(v.(fr.Element)) }
func ms(v time.Duration) float64            { return float64(v.Microseconds()) / 1000 }
func envOrDefault(n, f string) string {
	if v := os.Getenv(n); v != "" {
		return v
	}
	return f
}
