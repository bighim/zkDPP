package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	roleKeyHex    = "0000000000000000000000000000000000000000000000000000000000000002"
	outsideKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"
)

type artifact struct {
	ABI json.RawMessage `json:"abi"`
}

type deployment struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type runFile struct {
	Deployments []deployment `json:"deployments"`
}

type assetABI struct {
	DocumentType [32]byte
	DocHash      [32]byte
	Terminal     bool
}

type gadgetProofABI struct {
	Proof        []byte
	PublicInputs []*big.Int
}

type result struct {
	GeneratedAt          string `json:"generatedAt"`
	RPC                  string `json:"rpc"`
	Ledger               string `json:"ledger"`
	AllowedUnregistered  string `json:"allowedUnregisteredAccount"`
	ParticipantBefore    bool   `json:"participantBefore"`
	RoleTxHash           string `json:"roleRejectedTransactionHash"`
	RoleReceiptStatus    uint64 `json:"roleRejectedReceiptStatus"`
	OutsideAccount       string `json:"outsideAllowlistAccount"`
	OutsideRejected      bool   `json:"outsideTransactionRejected"`
	OutsideRejectionText string `json:"outsideRejectionText"`
	Passed               bool   `json:"passed"`
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	rpcURL := flag.String("rpc", "http://127.0.0.1:9545", "Besu JSON-RPC URL")
	runPath := flag.String("run", "benchmarks/besu-runs/run-01.json", "evmrun JSON")
	out := flag.String("out", "benchmarks/besu-permission-smoke.json", "output JSON")
	flag.Parse()

	ctx := context.Background()
	client, err := ethclient.Dial(*rpcURL)
	must(err)
	defer client.Close()
	chainID, err := client.ChainID(ctx)
	must(err)
	if chainID.Uint64() != 31337 {
		panic(fmt.Errorf("chain ID %s", chainID))
	}

	ledger := ledgerAddress(filepath.Join(*root, *runPath))
	contractABI := loadABI(filepath.Join(*root, "contracts/out/BenchmarkSahaiLedger.sol/BenchmarkSahaiLedger.json"))
	roleKey, err := crypto.HexToECDSA(roleKeyHex)
	must(err)
	roleAddress := crypto.PubkeyToAddress(roleKey.PublicKey)
	participantData, err := contractABI.Pack("participants", roleAddress)
	must(err)
	returned, err := client.CallContract(ctx, ethereum.CallMsg{To: &ledger, Data: participantData}, nil)
	must(err)
	values, err := contractABI.Unpack("participants", returned)
	must(err)
	participant := values[0].(bool)

	entryData, err := contractABI.Pack("entry",
		assetABI{}, []gadgetProofABI{})
	must(err)
	roleTx, roleReceipt := sendAndWait(ctx, client, chainID, roleKey, ledger, entryData)

	outsideKey, err := crypto.HexToECDSA(outsideKeyHex)
	must(err)
	outsideAddress := crypto.PubkeyToAddress(outsideKey.PublicKey)
	outsideTx := signedTx(ctx, client, chainID, outsideKey, outsideAddress, nil)
	err = client.SendTransaction(ctx, outsideTx)
	outcome := result{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), RPC: *rpcURL, Ledger: ledger.Hex(),
		AllowedUnregistered: roleAddress.Hex(), ParticipantBefore: participant,
		RoleTxHash: roleTx.Hash().Hex(), RoleReceiptStatus: roleReceipt.Status,
		OutsideAccount: outsideAddress.Hex(), OutsideRejected: err != nil,
	}
	if err != nil {
		outcome.OutsideRejectionText = err.Error()
	}
	outcome.Passed = !participant && roleReceipt.Status == types.ReceiptStatusFailed && outcome.OutsideRejected
	write(filepath.Join(*root, *out), outcome)
	if !outcome.Passed {
		panic(fmt.Errorf("permission gate failed: %+v", outcome))
	}
}

func ledgerAddress(path string) common.Address {
	raw, err := os.ReadFile(path)
	must(err)
	var value runFile
	must(json.Unmarshal(raw, &value))
	for _, item := range value.Deployments {
		if item.Name == "SahaiLedger" {
			return common.HexToAddress(item.Address)
		}
	}
	panic("SahaiLedger deployment not found")
}

func loadABI(path string) abi.ABI {
	raw, err := os.ReadFile(path)
	must(err)
	var value artifact
	must(json.Unmarshal(raw, &value))
	parsed, err := abi.JSON(bytes.NewReader(value.ABI))
	must(err)
	return parsed
}

func sendAndWait(ctx context.Context, client *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey, to common.Address, data []byte) (*types.Transaction, *types.Receipt) {
	tx := signedTx(ctx, client, chainID, key, to, data)
	must(client.SendTransaction(ctx, tx))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err == nil {
			return tx, receipt
		}
		if err != ethereum.NotFound {
			panic(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	panic("receipt timeout")
}

func signedTx(ctx context.Context, client *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey, to common.Address, data []byte) *types.Transaction {
	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	must(err)
	tx, err := types.SignNewTx(key, types.LatestSignerForChainID(chainID), &types.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, GasTipCap: big.NewInt(1_000_000_000), GasFeeCap: big.NewInt(100_000_000_000),
		Gas: 500_000, To: &to, Value: new(big.Int), Data: data,
	})
	must(err)
	return tx
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(raw, '\n'), 0o644))
}
