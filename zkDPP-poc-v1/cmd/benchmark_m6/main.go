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

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m3case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m4case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m5case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6case"
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

var rpcURL = "http://127.0.0.1:" + env("ANVIL_PORT", "18545")

type marshaler interface{ MarshalSolidity() []byte }
type rawContract struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type Tx struct {
	Kind, Name, Case, Actor, TransactionHash, ContractAddress                   string
	BlockNumber, ReceiptGasUsed                                                 uint64
	CalldataBytes                                                               int
	WitnessMillis, ProveMillis, TxPrepareMillis, SubmitReceiptMillis, E2EMillis float64
}
type Report struct {
	GeneratedAt, Mode, RPC, GoVersion string
	RunCount                          int
	ChainID                           uint64
	Transactions                      []Tx
	FinalStatusRoots                  map[string]string
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
type deployed struct {
	poseidon, entry, privateSpend, transfer, proceed, recall, merge, split, process, update common.Address
	ledgerABI                                                                               abi.ABI
	ledgerCode                                                                              []byte
}

func main() {
	mode := flag.String("mode", "gas", "gas or e2e")
	root := flag.String("root", ".", "root")
	flag.Parse()
	if err := run(*root, *mode); err != nil {
		panic(err)
	}
}

func run(root, mode string) error {
	if mode != "gas" && mode != "e2e" {
		return fmt.Errorf("invalid mode %s", mode)
	}
	if err := cmd(root, "docker", "compose", "down", "-v"); err != nil {
		return err
	}
	if err := cmd(root, "docker", "compose", "up", "-d", "anvil"); err != nil {
		return err
	}
	defer func() { _ = cmd(root, "docker", "compose", "down", "-v") }()
	client, err := wait()
	if err != nil {
		return err
	}
	defer client.Close()
	if err = cmd(root, "docker", "compose", "run", "--rm", "foundry", "forge", "build"); err != nil {
		return err
	}
	ctx := context.Background()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	actors := make([]sender, 7)
	for i := range actors {
		actors[i], err = derive(root, i)
		if err != nil {
			return err
		}
	}
	report := Report{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Mode: mode, RPC: rpcURL, GoVersion: runtime.Version(), RunCount: 1, ChainID: chainID.Uint64(), FinalStatusRoots: map[string]string{}}
	contracts, deployTx, err := deployContracts(ctx, client, actors[0], chainID, root)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, deployTx...)
	m2, err := m2case.Build(root)
	if err != nil {
		return err
	}
	m3, err := m3case.Build(root)
	if err != nil {
		return err
	}
	m4, err := m4case.Build(root)
	if err != nil {
		return err
	}
	m5, err := m5case.Build(root)
	if err != nil {
		return err
	}
	m6, err := m6case.Build(root)
	if err != nil {
		return err
	}
	loaded, err := loadArtifacts(root)
	if err != nil {
		return err
	}

	// Exit
	ledger, ledgerDeployment, err := newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, ledgerDeployment)
	if err = seedM2(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m2); err != nil {
		return err
	}
	tx, err := eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "exit", []any{mustProof(loaded["status-private-spend"], m6.PrivateSpend), bigField(m2.ExitAssignment.NoteRoot.(fr.Element)), bigField(m2.ExitAssignment.NF.(fr.Element))}, "exit", mode, loaded["status-private-spend"], m6.PrivateSpend)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Transfer
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM3(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m3); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "transfer", []any{mustProof(loaded["status-transfer"], m6.Transfer), fieldVar(m6.Transfer.NoteRoot), fieldVar(m6.Transfer.NF), fieldVar(m6.Transfer.RVNew), fieldVar(m6.Transfer.CMChange), u64(m3case.TransferEpoch), u64(m3case.DeltaEpoch)}, "transfer", mode, loaded["status-transfer"], m6.Transfer)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Proceed
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM3(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m3); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[3], chainID, ledger, pack(contracts.ledgerABI, "transfer", mustProof(loaded["status-transfer"], m6.Transfer), fieldVar(m6.Transfer.NoteRoot), fieldVar(m6.Transfer.NF), fieldVar(m6.Transfer.RVNew), fieldVar(m6.Transfer.CMChange), u64(m3case.TransferEpoch), u64(m3case.DeltaEpoch)), "setup", "transfer", "proceed"); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "proceed", []any{mustProof(loaded["status-proceed"], m6.Proceed), fieldVar(m6.Proceed.VoucherRoot), fieldVar(m6.Proceed.RVNF), fieldVar(m6.Proceed.CMReceiver)}, "proceed", mode, loaded["status-proceed"], m6.Proceed)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Recall
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM3(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m3); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[3], chainID, ledger, pack(contracts.ledgerABI, "transfer", mustProof(loaded["status-transfer"], m6.Transfer), fieldVar(m6.Transfer.NoteRoot), fieldVar(m6.Transfer.NF), fieldVar(m6.Transfer.RVNew), fieldVar(m6.Transfer.CMChange), u64(m3case.TransferEpoch), u64(m3case.DeltaEpoch)), "setup", "transfer", "recall-prefix"); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[3], chainID, ledger, pack(contracts.ledgerABI, "proceed", mustProof(loaded["status-proceed"], m6.Proceed), fieldVar(m6.Proceed.VoucherRoot), fieldVar(m6.Proceed.RVNF), fieldVar(m6.Proceed.CMReceiver)), "setup", "proceed", "recall-prefix"); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[3], chainID, ledger, pack(contracts.ledgerABI, "transfer", mustProof(loaded["status-transfer"], m6.FullTransfer), fieldVar(m6.FullTransfer.NoteRoot), fieldVar(m6.FullTransfer.NF), fieldVar(m6.FullTransfer.RVNew), fieldVar(m6.FullTransfer.CMChange), u64(m3case.TransferEpoch), u64(m3case.DeltaEpoch)), "setup", "transfer", "recall"); err != nil {
		return err
	}
	if err = setTimestamp(ctx, client, m3case.RecallEpoch*600); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "recall", []any{mustProof(loaded["status-recall"], m6.Recall), fieldVar(m6.Recall.VoucherRoot), fieldVar(m6.Recall.RVNF), fieldVar(m6.Recall.CMReturn), u64(m3case.RecallEpoch)}, "recall", mode, loaded["status-recall"], m6.Recall)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Merge and Split
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM4(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m4); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "merge", []any{mustProof(loaded["status-merge"], m6.Merge), fieldVar(m6.Merge.NoteRoot), fieldVar(m6.Merge.NF1), fieldVar(m6.Merge.NF2), fieldVar(m6.Merge.CMOut)}, "merge", mode, loaded["status-merge"], m6.Merge)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	// Split needs a fresh ledger because Merge nullifiers from the measured call are already spent.
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM4(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m4); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[3], chainID, ledger, pack(contracts.ledgerABI, "merge", mustProof(loaded["status-merge"], m6.Merge), fieldVar(m6.Merge.NoteRoot), fieldVar(m6.Merge.NF1), fieldVar(m6.Merge.NF2), fieldVar(m6.Merge.CMOut)), "setup", "merge", "split"); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "split", []any{mustProof(loaded["status-split"], m6.Split), fieldVar(m6.Split.NoteRoot), fieldVar(m6.Split.NF), fieldVar(m6.Split.CMOut1), fieldVar(m6.Split.CMOut2)}, "split", mode, loaded["status-split"], m6.Split)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Process
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM5(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m5); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[0], chainID, ledger, pack(contracts.ledgerABI, "registerPolicyAuthority", actors[2].address), "setup", "authority", ""); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[2], chainID, ledger, pack(contracts.ledgerABI, "reservePolicy", uint8(6)), "setup", "reserve", ""); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[2], chainID, ledger, pack(contracts.ledgerABI, "registerPolicy", bigField(m5.PolicyRef), uint8(3), uint8(2), common.HexToHash("0x01"), contracts.process), "setup", "policy", ""); err != nil {
		return err
	}
	if _, err = call(ctx, client, actors[2], chainID, ledger, pack(contracts.ledgerABI, "setPolicyGrant", bigField(m5.PolicyRef), bigField(m5.ScopeRef), true), "setup", "grant", ""); err != nil {
		return err
	}
	nf := [3]*big.Int{fieldVar(m6.Process.NF[0]), fieldVar(m6.Process.NF[1]), fieldVar(m6.Process.NF[2])}
	outs := [2]*big.Int{fieldVar(m6.Process.CMOut[0]), fieldVar(m6.Process.CMOut[1])}
	tx, err = eventCall(ctx, client, actors[3], chainID, ledger, contracts.ledgerABI, "process", []any{mustProof(loaded["status-process"], m6.Process), fieldVar(m6.Process.PolicyRef), fieldVar(m6.Process.ScopeRef), fieldVar(m6.Process.NoteRoot), nf, outs}, "process", mode, loaded["status-process"], m6.Process)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)

	// Status update
	ledger, _, err = newLedger(ctx, client, actors, chainID, contracts)
	if err != nil {
		return err
	}
	if err = seedM2(ctx, client, actors, chainID, contracts.ledgerABI, ledger, loaded["entry"], m2); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[5], chainID, ledger, contracts.ledgerABI, "updateStatus", []any{mustProof(loaded["status-update"], m6.StatusUpdate), uint8(1), bigField(m2.Entries[1].Note.Commitment), uint32(1), uint8(0), uint8(1), fieldVar(m6.StatusUpdate.NewRoot)}, "active-to-frozen", mode, loaded["status-update"], m6.StatusUpdate)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	tx, err = eventCall(ctx, client, actors[5], chainID, ledger, contracts.ledgerABI, "updateStatus", []any{mustProof(loaded["status-update"], m6.StatusUnfreeze), uint8(1), bigField(m2.Entries[1].Note.Commitment), uint32(1), uint8(1), uint8(0), fieldVar(m6.StatusUnfreeze.NewRoot)}, "frozen-to-active", mode, loaded["status-update"], m6.StatusUnfreeze)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	if _, err = call(ctx, client, actors[5], chainID, ledger, pack(contracts.ledgerABI, "updateStatus", mustProof(loaded["status-update"], m6.StatusUpdate), uint8(1), bigField(m2.Entries[1].Note.Commitment), uint32(1), uint8(0), uint8(1), fieldVar(m6.StatusUpdate.NewRoot)), "setup", "active-to-frozen", "revoke-prefix"); err != nil {
		return err
	}
	tx, err = eventCall(ctx, client, actors[5], chainID, ledger, contracts.ledgerABI, "updateStatus", []any{mustProof(loaded["status-update"], m6.StatusRevoke), uint8(1), bigField(m2.Entries[1].Note.Commitment), uint32(1), uint8(1), uint8(2), fieldVar(m6.StatusRevoke.NewRoot)}, "frozen-to-revoked", mode, loaded["status-update"], m6.StatusRevoke)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, tx)
	rootNow, err := viewBig(ctx, client, contracts.ledgerABI, ledger, "noteStatusRoot")
	if err != nil {
		return err
	}
	if rootNow.Cmp(fieldVar(m6.StatusRevoke.NewRoot)) != 0 {
		return fmt.Errorf("status root mismatch")
	}
	report.FinalStatusRoots["note"] = rootNow.String()
	report.FinalStatusRoots["voucher"] = m6.VoucherStatusRoot.String()
	name := "output/m6-anvil-gas.json"
	if mode == "e2e" {
		name = "output/m6-anvil-e2e.json"
	}
	return artifact.WriteJSON(filepath.Join(root, name), report)
}

func deployContracts(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, root string) (deployed, []Tx, error) {
	var d deployed
	var txs []Tx
	items := []struct {
		source, name string
		target       *common.Address
	}{{"Poseidon2BLS12381.sol", "Poseidon2BLS12381", &d.poseidon}, {"EntryVerifier.sol", "EntryVerifier", &d.entry}, {"StatusPrivateSpendVerifier.sol", "StatusPrivateSpendVerifier", &d.privateSpend}, {"StatusTransferVerifier.sol", "StatusTransferVerifier", &d.transfer}, {"StatusProceedVerifier.sol", "StatusProceedVerifier", &d.proceed}, {"StatusRecallVerifier.sol", "StatusRecallVerifier", &d.recall}, {"StatusMergeVerifier.sol", "StatusMergeVerifier", &d.merge}, {"StatusSplitVerifier.sol", "StatusSplitVerifier", &d.split}, {"StatusProcessVerifier.sol", "StatusProcessVerifier", &d.process}, {"StatusUpdateVerifier.sol", "StatusUpdateVerifier", &d.update}}
	for _, item := range items {
		_, code, err := loadContract(root, item.source, item.name)
		if err != nil {
			return d, nil, err
		}
		addr, tx, err := deploy(ctx, c, s, id, item.name, code)
		if err != nil {
			return d, nil, err
		}
		*item.target = addr
		txs = append(txs, tx)
	}
	d.ledgerABI, d.ledgerCode, _ = loadContract(root, "ZkDPPStatusLedger.sol", "ZkDPPStatusLedger")
	return d, txs, nil
}
func newLedger(ctx context.Context, c *ethclient.Client, a []sender, id *big.Int, d deployed) (common.Address, Tx, error) {
	ctor, _ := d.ledgerABI.Pack("", a[5].address, d.entry, d.privateSpend, d.transfer, d.proceed, d.recall, d.merge, d.split, d.process, d.update, d.poseidon)
	ledger, tx, err := deploy(ctx, c, a[0], id, "ZkDPPStatusLedger", append(d.ledgerCode, ctor...))
	if err != nil {
		return ledger, tx, err
	}
	_, err = call(ctx, c, a[0], id, ledger, pack(d.ledgerABI, "setEntryIssuer", a[1].address, true), "setup", "issuer", "")
	return ledger, tx, err
}
func seedM2(ctx context.Context, c *ethclient.Client, a []sender, id *big.Int, abi abi.ABI, l common.Address, e *artifact.Loaded, s *m2case.Scenario) error {
	for _, v := range s.Entries {
		if _, err := call(ctx, c, a[1], id, l, pack(abi, "entry", mustProof(e, v.Assignment), bigField(v.Note.Commitment)), "setup", "entry", v.Name); err != nil {
			return err
		}
	}
	return nil
}
func seedM3(ctx context.Context, c *ethclient.Client, a []sender, id *big.Int, abi abi.ABI, l common.Address, e *artifact.Loaded, s *m3case.Scenario) error {
	for _, v := range s.Entries {
		if _, err := call(ctx, c, a[1], id, l, pack(abi, "entry", mustProof(e, v.Assignment), bigField(v.Note.Commitment)), "setup", "entry", v.Name); err != nil {
			return err
		}
	}
	return nil
}
func seedM4(ctx context.Context, c *ethclient.Client, a []sender, id *big.Int, abi abi.ABI, l common.Address, e *artifact.Loaded, s *m4case.Scenario) error {
	for _, v := range s.Entries {
		if _, err := call(ctx, c, a[1], id, l, pack(abi, "entry", mustProof(e, v.Assignment), bigField(v.Note.Commitment)), "setup", "entry", v.Name); err != nil {
			return err
		}
	}
	return nil
}
func seedM5(ctx context.Context, c *ethclient.Client, a []sender, id *big.Int, abi abi.ABI, l common.Address, e *artifact.Loaded, s *m5case.Scenario) error {
	for _, v := range s.Entries {
		if _, err := call(ctx, c, a[1], id, l, pack(abi, "entry", mustProof(e, v.Assignment), bigField(v.Note.Commitment)), "setup", "entry", v.Name); err != nil {
			return err
		}
	}
	return nil
}
func loadArtifacts(root string) (map[string]*artifact.Loaded, error) {
	out := map[string]*artifact.Loaded{}
	entry, err := artifact.LoadAt(root, "m2", "entry")
	if err != nil {
		return nil, err
	}
	out["entry"] = entry
	for _, n := range []string{"status-update", "status-private-spend", "status-transfer", "status-proceed", "status-recall", "status-merge", "status-split", "status-process"} {
		v, e := artifact.LoadAt(root, "m6", n)
		if e != nil {
			return nil, e
		}
		out[n] = v
	}
	return out, nil
}
func proof(l *artifact.Loaded, a frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
	st := time.Now()
	w, e := frontend.NewWitness(a, ecc.BLS12_381.ScalarField())
	wt := time.Since(st)
	if e != nil {
		return nil, wt, 0, e
	}
	st = time.Now()
	p, e := plonk.Prove(l.CCS, l.PK, w)
	pt := time.Since(st)
	if e != nil {
		return nil, wt, pt, e
	}
	return p.(marshaler).MarshalSolidity(), wt, pt, nil
}
func mustProof(l *artifact.Loaded, a frontend.Circuit) []byte {
	p, _, _, e := proof(l, a)
	if e != nil {
		panic(e)
	}
	return p
}
func eventCall(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to common.Address, a abi.ABI, method string, args []any, name, mode string, l *artifact.Loaded, assignment frontend.Circuit) (Tx, error) {
	p, wt, pt, e := proof(l, assignment)
	if e != nil {
		return Tx{}, e
	}
	args[0] = p
	start := time.Now()
	tx, e := call(ctx, c, s, id, to, pack(a, method, args...), "event", name, "")
	if e != nil {
		return Tx{}, e
	}
	if mode == "e2e" {
		tx.WitnessMillis = ms(wt)
		tx.ProveMillis = ms(pt)
		tx.E2EMillis = tx.WitnessMillis + tx.ProveMillis + ms(time.Since(start))
	}
	return tx, nil
}
func fieldVar(v frontend.Variable) *big.Int { return bigField(v.(fr.Element)) }
func bigField(v fr.Element) *big.Int        { return v.BigInt(new(big.Int)) }
func u64(v uint64) *big.Int                 { return new(big.Int).SetUint64(v) }
func setTimestamp(ctx context.Context, c *ethclient.Client, t uint64) error {
	var out any
	return c.Client().CallContext(ctx, &out, "evm_setNextBlockTimestamp", t)
}
func deploy(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, name string, data []byte) (common.Address, Tx, error) {
	p, e := prepare(ctx, c, s, id, nil, data)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	r, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	return r.ContractAddress, Tx{Kind: "deployment", Name: name, Actor: s.address.Hex(), TransactionHash: r.TxHash.Hex(), ContractAddress: r.ContractAddress.Hex(), BlockNumber: r.BlockNumber.Uint64(), ReceiptGasUsed: r.GasUsed, CalldataBytes: p.bytes, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
}
func call(ctx context.Context, c *ethclient.Client, s sender, id *big.Int, to common.Address, data []byte, kind, name, caseName string) (Tx, error) {
	p, e := prepare(ctx, c, s, id, &to, data)
	if e != nil {
		return Tx{}, e
	}
	r, d, e := submit(ctx, c, p.tx)
	if e != nil {
		return Tx{}, e
	}
	return Tx{Kind: kind, Name: name, Case: caseName, Actor: s.address.Hex(), TransactionHash: r.TxHash.Hex(), BlockNumber: r.BlockNumber.Uint64(), ReceiptGasUsed: r.GasUsed, CalldataBytes: p.bytes, TxPrepareMillis: ms(p.duration), SubmitReceiptMillis: ms(d)}, nil
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
				return nil, 0, fmt.Errorf("revert %s", tx.Hash())
			}
			return r, time.Since(st), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("receipt timeout")
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
func decode(v string) ([]byte, error) { return hex.DecodeString(strings.TrimPrefix(v, "0x")) }
func pack(a abi.ABI, m string, args ...any) []byte {
	b, e := a.Pack(m, args...)
	if e != nil {
		panic(e)
	}
	return b
}
func viewBig(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, m string) (*big.Int, error) {
	data, _ := a.Pack(m)
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
