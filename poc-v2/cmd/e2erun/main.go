package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bighim/zkDPP/poc-v2/internal/assignments"
	"github.com/bighim/zkDPP/poc-v2/internal/benchmarktime"
	"github.com/bighim/zkDPP/poc-v2/internal/proofruntime"
	"github.com/bighim/zkDPP/poc-v2/internal/proofspec"
	"github.com/bighim/zkDPP/poc-v2/internal/scenario"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	chainID        = 31337
	transactionGas = 29_000_000
	receiptTimeout = 30 * time.Second
	receiptPoll    = 10 * time.Millisecond
)

type proofMarshaler interface{ MarshalSolidity() []byte }

type artifact struct {
	ABI json.RawMessage `json:"abi"`
}

type forgeBroadcast struct {
	Transactions []forgeTransaction `json:"transactions"`
	Receipts     []forgeReceipt     `json:"receipts"`
}

type forgeTransaction struct {
	Hash            string `json:"hash"`
	ContractName    string `json:"contractName"`
	ContractAddress string `json:"contractAddress"`
}

type forgeReceipt struct {
	TransactionHash string `json:"transactionHash"`
	ContractAddress string `json:"contractAddress"`
	Status          string `json:"status"`
}

type machineMetadata struct {
	OS        string `json:"os"`
	OSVersion string `json:"osVersion,omitempty"`
	Arch      string `json:"arch"`
	CPU       string `json:"cpu,omitempty"`
	CPUCores  int    `json:"cpuCores"`
	RAMBytes  string `json:"ramBytes,omitempty"`
	GoVersion string `json:"goVersion"`
}

type durationMillis float64

type loadResult struct {
	Relation string         `json:"relation"`
	LoadMS   durationMillis `json:"artifactLoadMs"`
}

type callResult struct {
	Sequence          int            `json:"sequence"`
	Event             string         `json:"event"`
	Case              string         `json:"case"`
	Actor             string         `json:"actor"`
	WitnessMS         durationMillis `json:"witnessMs"`
	ProveMS           durationMillis `json:"proveMs"`
	TransactionPrepMS durationMillis `json:"txPrepareMs"`
	SubmitReceiptMS   durationMillis `json:"submitToReceiptMs"`
	E2EMS             durationMillis `json:"e2eMs"`
	TransactionHash   string         `json:"transactionHash"`
	BlockNumber       uint64         `json:"blockNumber"`
	ReceiptGasUsed    uint64         `json:"receiptGasUsed"`
	CalldataBytes     int            `json:"calldataBytes"`
	ZeroBytes         int            `json:"zeroBytes"`
	NonZeroBytes      int            `json:"nonZeroBytes"`
}

type runResult struct {
	Run           int             `json:"run"`
	GeneratedAt   string          `json:"generatedAt"`
	Measurement   string          `json:"measurementType"`
	Machine       machineMetadata `json:"machine"`
	Contract      string          `json:"contract"`
	ArtifactLoads []loadResult    `json:"artifactLoads"`
	Calls         []callResult    `json:"calls"`
	MTAppendCount int             `json:"mtAppendCount"`
	RVAppendCount int             `json:"rvMTAppendCount"`
	MTLeafCount   uint64          `json:"mtLeafCount"`
	RVMTLeafCount uint64          `json:"rvMTLeafCount"`
}

type sender struct {
	key   *ecdsa.PrivateKey
	nonce uint64
}

func main() {
	root := flag.String("root", ".", "poc-v2 root")
	rpcURL := flag.String("rpc", "http://127.0.0.1:8545", "Anvil JSON-RPC URL")
	broadcastPath := flag.String("broadcast", "", "DeployCanonical broadcast JSON")
	runNumber := flag.Int("run", 1, "run number")
	out := flag.String("out", "", "run JSON output")
	flag.Parse()
	if *broadcastPath == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*root, *rpcURL, *broadcastPath, *runNumber, *out); err != nil {
		panic(err)
	}
}

func run(root, rpcURL, broadcastPath string, runNumber int, out string) error {
	derived, err := scenario.LoadCanonical(root)
	if err != nil {
		return err
	}
	canonical, err := assignments.Build(derived)
	if err != nil {
		return err
	}
	relations := proofspec.Relations(derived, canonical)
	cases := proofspec.CaseIndex(relations)

	loaded := make(map[string]*proofruntime.Loaded, len(relations))
	loads := make([]loadResult, 0, len(relations))
	for _, relation := range relations {
		item, err := proofruntime.Load(root, relation.Name)
		if err != nil {
			return err
		}
		loaded[relation.Name] = item
		loads = append(loads, loadResult{Relation: relation.Name, LoadMS: millis(item.LoadTime)})
	}

	contractAddress, err := deployedZkDPP(broadcastPath)
	if err != nil {
		return err
	}
	contractABI, err := loadABI(filepath.Join(root, "contracts", "out", "ZkDPP.sol", "ZkDPP.json"))
	if err != nil {
		return err
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	ctx := context.Background()
	actualChainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	if actualChainID.Uint64() != chainID {
		return fmt.Errorf("unexpected chain ID %s", actualChainID)
	}
	senders := make(map[string]*sender, 5)
	for actor, variable := range map[string]string{
		"AluminumSupplier": "ANVIL_ALUMINUM_SUPPLIER_PK",
		"CathodeSupplier":  "ANVIL_CATHODE_SUPPLIER_PK",
		"AnodeSupplier":    "ANVIL_ANODE_SUPPLIER_PK",
		"FoilManufacturer": "ANVIL_FOIL_MANUFACTURER_PK",
		"CellManufacturer": "ANVIL_CELL_MANUFACTURER_PK",
	} {
		senders[actor], err = newSender(ctx, client, os.Getenv(variable))
		if err != nil {
			return fmt.Errorf("%s: %w", actor, err)
		}
	}

	result := runResult{
		Run: runNumber, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Measurement: "live-proof-anvil-e2e", Machine: machine(), Contract: contractAddress.Hex(),
		ArtifactLoads: loads, Calls: make([]callResult, 0, len(proofspec.CanonicalEvents)),
	}
	for sequence, event := range proofspec.CanonicalEvents {
		item, ok := cases[event.Relation+"/"+event.Case]
		if !ok {
			return fmt.Errorf("missing canonical case %s/%s", event.Relation, event.Case)
		}
		e2e := benchmarktime.Start(benchmarktime.SystemClock{})
		proof, witnessTime, proveTime, err := loaded[event.Relation].Prove(item.Assignment)
		if err != nil {
			return fmt.Errorf("prove %s/%s: %w", event.Relation, event.Case, err)
		}
		prepareStart := time.Now()
		encodedProof := proof.(proofMarshaler).MarshalSolidity()
		calldata, err := packCall(contractABI, event.Relation, encodedProof, item.PublicInputs)
		if err != nil {
			return fmt.Errorf("pack %s/%s: %w", event.Relation, event.Case, err)
		}
		selected, ok := senders[event.Actor]
		if !ok {
			return fmt.Errorf("missing sender for actor %s", event.Actor)
		}
		tx, err := signTransaction(selected, contractAddress, calldata)
		if err != nil {
			return err
		}
		selected.nonce++
		prepareTime := time.Since(prepareStart)
		submitStart := time.Now()
		if err := client.SendTransaction(ctx, tx); err != nil {
			return fmt.Errorf("submit %s/%s: %w", event.Relation, event.Case, err)
		}
		receipt, err := waitReceipt(ctx, client, tx.Hash())
		submitTime := time.Since(submitStart)
		if err != nil {
			return fmt.Errorf("receipt %s/%s: %w", event.Relation, event.Case, err)
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("transaction %s/%s reverted: %s", event.Relation, event.Case, tx.Hash())
		}
		zeroBytes, nonZeroBytes := byteCounts(calldata)
		for _, itemLog := range receipt.Logs {
			if itemLog.Address != contractAddress || len(itemLog.Topics) == 0 {
				continue
			}
			switch itemLog.Topics[0] {
			case crypto.Keccak256Hash([]byte("MTAppend(uint256,uint256,uint256)")):
				result.MTAppendCount++
			case crypto.Keccak256Hash([]byte("RVMTAppend(uint256,uint256,uint256)")):
				result.RVAppendCount++
			}
		}
		result.Calls = append(result.Calls, callResult{
			Sequence: sequence + 1, Event: event.Relation, Case: event.Case, Actor: event.Actor,
			WitnessMS: millis(witnessTime), ProveMS: millis(proveTime), TransactionPrepMS: millis(prepareTime),
			SubmitReceiptMS: millis(submitTime), E2EMS: millis(e2e.Elapsed()),
			TransactionHash: tx.Hash().Hex(), BlockNumber: receipt.BlockNumber.Uint64(), ReceiptGasUsed: receipt.GasUsed,
			CalldataBytes: len(calldata), ZeroBytes: zeroBytes, NonZeroBytes: nonZeroBytes,
		})
		fmt.Printf("run=%d %02d %-8s %-20s prove=%8.3fms receipt=%7.3fms e2e=%8.3fms gas=%d\n",
			runNumber, sequence+1, event.Relation, event.Case, float64(millis(proveTime)), float64(millis(submitTime)), float64(result.Calls[len(result.Calls)-1].E2EMS), receipt.GasUsed)
	}

	result.MTLeafCount, err = callUint(ctx, client, contractABI, contractAddress, "mtLeafCount")
	if err != nil {
		return err
	}
	result.RVMTLeafCount, err = callUint(ctx, client, contractABI, contractAddress, "rvLeafCount")
	if err != nil {
		return err
	}
	if result.MTAppendCount != 28 || result.RVAppendCount != 7 || result.MTLeafCount != 28 || result.RVMTLeafCount != 7 {
		return fmt.Errorf("unexpected final state: MT logs=%d leaves=%d, rvMT logs=%d leaves=%d", result.MTAppendCount, result.MTLeafCount, result.RVAppendCount, result.RVMTLeafCount)
	}
	return writeJSON(out, result)
}

func packCall(contractABI abi.ABI, event string, proof []byte, inputs []fr.Element) ([]byte, error) {
	values := bigInts(inputs)
	switch event {
	case "entry":
		return contractABI.Pack("entry", proof, values[0])
	case "transfer":
		return contractABI.Pack("transfer", proof, values[0], values[1], values[2], values[3], values[4], values[5])
	case "proceed":
		return contractABI.Pack("proceed", proof, values[0], values[1], values[2], values[3])
	case "recall":
		return contractABI.Pack("recall", proof, values[0], values[1], values[2], values[3])
	case "merge":
		return contractABI.Pack("merge", proof, values[0], values[1], values[2], values[3], values[4], values[5], values[6])
	case "split":
		return contractABI.Pack("split", proof, values[0], values[1], values[2], values[3], values[4])
	case "process":
		m, n := int(values[0].Uint64()), int(values[1].Uint64())
		if m < 1 || m > 3 || n < 1 || n > 3 {
			return nil, fmt.Errorf("invalid process arity m=%d n=%d", m, n)
		}
		delta := append([]*big.Int(nil), values[2:5]...)
		allocation := append([]*big.Int(nil), values[5:5+n*3]...)
		roots, commitments, nullifiers := make([]*big.Int, m), make([]*big.Int, m), make([]*big.Int, m)
		for i := 0; i < m; i++ {
			roots[i], commitments[i], nullifiers[i] = values[14+i*3], values[15+i*3], values[16+i*3]
		}
		outputs := append([]*big.Int(nil), values[23:23+n]...)
		return contractABI.Pack("process", proof, delta, allocation, roots, commitments, nullifiers, outputs)
	case "exit":
		return contractABI.Pack("exit", proof, values[0], values[1], values[2])
	default:
		return nil, fmt.Errorf("unsupported event %s", event)
	}
}

func signTransaction(selected *sender, to common.Address, calldata []byte) (*types.Transaction, error) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: big.NewInt(chainID), Nonce: selected.nonce, GasTipCap: big.NewInt(1_000_000_000),
		GasFeeCap: big.NewInt(100_000_000_000), Gas: transactionGas, To: &to, Value: new(big.Int), Data: calldata,
	})
	return types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), selected.key)
}

func newSender(ctx context.Context, client *ethclient.Client, encodedKey string) (*sender, error) {
	if encodedKey == "" {
		return nil, fmt.Errorf("private key environment variable is empty")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(encodedKey, "0x"))
	if err != nil {
		return nil, err
	}
	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		return nil, err
	}
	return &sender{key: key, nonce: nonce}, nil
}

func waitReceipt(parent context.Context, client *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(parent, receiptTimeout)
	defer cancel()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(receiptPoll):
		}
	}
}

func callUint(ctx context.Context, client *ethclient.Client, contractABI abi.ABI, address common.Address, method string) (uint64, error) {
	data, err := contractABI.Pack(method)
	if err != nil {
		return 0, err
	}
	output, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return 0, err
	}
	values, err := contractABI.Unpack(method, output)
	if err != nil || len(values) != 1 {
		return 0, fmt.Errorf("unpack %s: values=%d, err=%v", method, len(values), err)
	}
	return values[0].(*big.Int).Uint64(), nil
}

func loadABI(path string) (abi.ABI, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, err
	}
	var value artifact
	if err := json.Unmarshal(data, &value); err != nil {
		return abi.ABI{}, err
	}
	return abi.JSON(bytes.NewReader(value.ABI))
}

func deployedZkDPP(path string) (common.Address, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return common.Address{}, err
	}
	var value forgeBroadcast
	if err := json.Unmarshal(data, &value); err != nil {
		return common.Address{}, err
	}
	receipts := make(map[string]forgeReceipt, len(value.Receipts))
	for _, receipt := range value.Receipts {
		if receipt.Status != "0x1" && receipt.Status != "1" {
			return common.Address{}, fmt.Errorf("failed deployment receipt %s", receipt.TransactionHash)
		}
		receipts[strings.ToLower(receipt.TransactionHash)] = receipt
	}
	for _, transaction := range value.Transactions {
		if transaction.ContractName != "ZkDPP" {
			continue
		}
		address := transaction.ContractAddress
		if receipt, ok := receipts[strings.ToLower(transaction.Hash)]; ok && receipt.ContractAddress != "" {
			address = receipt.ContractAddress
		}
		if common.IsHexAddress(address) {
			return common.HexToAddress(address), nil
		}
	}
	return common.Address{}, fmt.Errorf("ZkDPP deployment not found in %s", path)
}

func bigInts(values []fr.Element) []*big.Int {
	result := make([]*big.Int, len(values))
	for i := range values {
		result[i] = new(big.Int)
		values[i].BigInt(result[i])
	}
	return result
}

func byteCounts(data []byte) (zero, nonZero int) {
	for _, value := range data {
		if value == 0 {
			zero++
		} else {
			nonZero++
		}
	}
	return zero, nonZero
}

func machine() machineMetadata {
	return machineMetadata{
		OS: runtime.GOOS, OSVersion: command("sw_vers", "-productVersion"), Arch: runtime.GOARCH,
		CPU: command("sysctl", "-n", "machdep.cpu.brand_string"), CPUCores: runtime.NumCPU(),
		RAMBytes: command("sysctl", "-n", "hw.memsize"), GoVersion: runtime.Version(),
	}
}

func command(name string, args ...string) string {
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func millis(value time.Duration) durationMillis {
	return durationMillis(float64(value) / float64(time.Millisecond))
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
