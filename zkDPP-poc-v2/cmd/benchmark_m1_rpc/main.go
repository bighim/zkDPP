package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type contractArtifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type auditCipherABI struct {
	R1X, R1Y                             *big.Int
	EncryptedParents, EncryptedOutputNfs []*big.Int
}

func load(path string) (abi.ABI, []byte, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, nil, err
	}
	var raw contractArtifact
	if err = json.Unmarshal(rawBytes, &raw); err != nil {
		return abi.ABI{}, nil, err
	}
	a, err := abi.JSON(bytes.NewReader(raw.ABI))
	if err != nil {
		return a, nil, err
	}
	code, err := hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x"))
	return a, code, err
}
func deploy(ctx context.Context, client *ethclient.Client, from common.Address, a abi.ABI, code []byte, name string, args ...any) (common.Address, m7run.Transaction, error) {
	encoded, err := a.Constructor.Inputs.Pack(args...)
	if err != nil {
		return common.Address{}, m7run.Transaction{}, err
	}
	receipt, row, err := m7run.Send(ctx, client, from, nil, append(append([]byte{}, code...), encoded...), name)
	if err != nil {
		return common.Address{}, row, err
	}
	return receipt.ContractAddress, row, nil
}
func send(ctx context.Context, client *ethclient.Client, from, to common.Address, a abi.ABI, method string, args ...any) (m7run.Transaction, error) {
	data, err := a.Pack(method, args...)
	if err != nil {
		return m7run.Transaction{}, err
	}
	_, row, err := m7run.Send(ctx, client, from, &to, data, method)
	return row, err
}
func viewUint(ctx context.Context, client *ethclient.Client, to common.Address, a abi.ABI, method string) (*big.Int, error) {
	data, err := a.Pack(method)
	if err != nil {
		return nil, err
	}
	raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := a.Unpack(method, raw)
	if err != nil || len(values) != 1 {
		return nil, fmt.Errorf("%s view: %w", method, err)
	}
	return values[0].(*big.Int), nil
}
func cipher(ct auditcrypto.Ciphertext, parents int) auditCipherABI {
	out := auditCipherABI{R1X: ct.R1.X.BigInt(new(big.Int)), R1Y: ct.R1.Y.BigInt(new(big.Int))}
	for i, v := range ct.Data {
		p := v.BigInt(new(big.Int))
		if i < parents {
			out.EncryptedParents = append(out.EncryptedParents, p)
		} else {
			out.EncryptedOutputNfs = append(out.EncryptedOutputNfs, p)
		}
	}
	return out
}

func main() {
	fmt.Fprintln(os.Stderr,"M1 historical audit results are immutable; use cmd/m1_hotfix -mode audit")
	os.Exit(1)
	if err := run("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := ethclient.DialContext(ctx, "http://127.0.0.1:18546")
	if err != nil {
		return err
	}
	defer client.Close()
	var accounts []common.Address
	if err = client.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil || len(accounts) == 0 {
		return fmt.Errorf("accounts: %w", err)
	}
	from := accounts[0]
	base := filepath.Join(root, "contracts/m1/out")
	verifierABI, verifierCode, err := load(filepath.Join(base, "V2M1Ledger.t.sol/V2MockVerifier.json"))
	if err != nil {
		return err
	}
	hasherABI, hasherCode, err := load(filepath.Join(base, "V2M1Ledger.t.sol/V2MockHasher.json"))
	if err != nil {
		return err
	}
	ledgerABI, ledgerCode, err := load(filepath.Join(base, "ZkDPPV2Ledger.sol/ZkDPPV2Ledger.json"))
	if err != nil {
		return err
	}
	transactions := []m7run.Transaction{}
	verifier, tx, err := deploy(ctx, client, from, verifierABI, verifierCode, "mock-verifier")
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	hasher, tx, err := deploy(ctx, client, from, hasherABI, hasherCode, "field-hasher")
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	fixed := [7]common.Address{verifier, verifier, verifier, verifier, verifier, verifier, verifier}
	ledger, tx, err := deploy(ctx, client, from, ledgerABI, ledgerCode, "v2-ledger", fixed, hasher, from)
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	tx, err = send(ctx, client, from, ledger, ledgerABI, "setEntryIssuer", from, true)
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	pkg, shares, err := v2case.KeyFixture()
	if err != nil {
		return err
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[1]})
	if err != nil {
		return err
	}
	defer master.SetInt64(0)
	refs := []fr.Element{zkhash.Element(8001), zkhash.Element(8002), zkhash.Element(8003), zkhash.Element(8004), zkhash.Element(8005)}
	spends := []fr.Element{zkhash.Element(8101), zkhash.Element(8102), zkhash.Element(8103), zkhash.Element(8104), zkhash.Element(8105)}
	ct1, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{spends[0]}, big.NewInt(8201))
	if err != nil {
		return err
	}
	tx, err = send(ctx, client, from, ledger, ledgerABI, "entry", []byte{1}, refs[0].BigInt(new(big.Int)), cipher(ct1, 0))
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	noteRoot, err := viewUint(ctx, client, ledger, ledgerABI, "currentNoteRoot")
	if err != nil {
		return err
	}
	ct2, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{refs[0], spends[1], spends[2]}, big.NewInt(8202))
	if err != nil {
		return err
	}
	tx, err = send(ctx, client, from, ledger, ledgerABI, "split", []byte{2}, noteRoot, spends[0].BigInt(new(big.Int)), refs[1].BigInt(new(big.Int)), refs[2].BigInt(new(big.Int)), cipher(ct2, 1))
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	noteRoot, err = viewUint(ctx, client, ledger, ledgerABI, "currentNoteRoot")
	if err != nil {
		return err
	}
	ct3, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{refs[1], spends[3], spends[4]}, big.NewInt(8203))
	if err != nil {
		return err
	}
	tx, err = send(ctx, client, from, ledger, ledgerABI, "split", []byte{3}, noteRoot, spends[1].BigInt(new(big.Int)), refs[3].BigInt(new(big.Int)), refs[4].BigInt(new(big.Int)), cipher(ct3, 1))
	if err != nil {
		return err
	}
	transactions = append(transactions, tx)
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	snapshot := v2audit.Snapshot{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash().Hex()}
	source := v2audit.NewRPCSource(client, ledgerABI, ledger, from)
	start := time.Now()
	result := v2audit.AuditAndFreeze(ctx, source, master, v2audit.Ref{ObjectType: v2audit.Note, RawID: refs[0]}, snapshot)
	elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
	if result.Outcome != v2audit.CompleteAtCheckpoint {
		return fmt.Errorf("AuditAndFreeze=%s: %s", result.Outcome, result.Reason)
	}
	rawPath := filepath.Join(root, "output/m1-audit.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		return err
	}
	report := map[string]any{}
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	report["rpcEndToEnd"] = map[string]any{"backend": "Anvil JSON-RPC", "ledger": ledger.Hex(), "snapshot": result.Snapshot, "checkpoint": result.Checkpoint, "outcome": result.Outcome, "targetCount": len(result.Targets), "freezeTransactions": source.FreezeTransactions, "visitedObjects": result.Metrics.VisitedObjects, "uniqueRecords": result.Metrics.UniqueRecords, "decryptions": result.Metrics.Decryptions, "totalMillis": elapsed, "setupTransactions": transactions}
	return artifact.WriteJSON(rawPath, report)
}
