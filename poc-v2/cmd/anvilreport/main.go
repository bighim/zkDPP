package main

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bighim/zkDPP/poc-v2/internal/proofspec"
	"golang.org/x/crypto/sha3"
)

const (
	expectedMTAppends   = 28
	expectedRVMTAppends = 7
)

type broadcast struct {
	Transactions []broadcastTransaction `json:"transactions"`
	Receipts     []receipt              `json:"receipts"`
}
type broadcastTransaction struct {
	Hash            string          `json:"hash"`
	TransactionType string          `json:"transactionType"`
	ContractName    string          `json:"contractName"`
	ContractAddress string          `json:"contractAddress"`
	Function        string          `json:"function"`
	Transaction     json.RawMessage `json:"transaction"`
}
type transactionEnvelope struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Input string `json:"input"`
	Data  string `json:"data"`
}
type receipt struct {
	Status          string `json:"status"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     string `json:"blockNumber"`
	GasUsed         string `json:"gasUsed"`
	ContractAddress string `json:"contractAddress"`
	Logs            []log  `json:"logs"`
}
type log struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
}

type report struct {
	GeneratedAt         string             `json:"generatedAt"`
	Environment         string             `json:"environment"`
	MeasurementType     string             `json:"measurementType"`
	MeasurementBoundary string             `json:"measurementBoundary"`
	Chain               chainMetadata      `json:"chain"`
	Deployments         []deploymentResult `json:"deployments"`
	Calls               []callResult       `json:"calls"`
	LogValidation       logValidation      `json:"logValidation"`
	Totals              totals             `json:"totals"`
}
type chainMetadata struct {
	ChainID          uint64 `json:"chainId"`
	Hardfork         string `json:"hardfork"`
	GenesisTimestamp uint64 `json:"genesisTimestamp"`
	BlockGasLimit    uint64 `json:"blockGasLimit"`
}
type deploymentResult struct {
	Sequence           int    `json:"sequence"`
	Component          string `json:"component"`
	TransactionHash    string `json:"transactionHash"`
	ContractAddress    string `json:"contractAddress"`
	BlockNumber        uint64 `json:"blockNumber"`
	ReceiptGasUsed     uint64 `json:"receiptGasUsed"`
	CreationInputBytes int    `json:"creationInputBytes"`
	ZeroBytes          int    `json:"zeroBytes"`
	NonZeroBytes       int    `json:"nonZeroBytes"`
}
type callResult struct {
	Sequence        int    `json:"sequence"`
	Event           string `json:"event"`
	Case            string `json:"case"`
	Actor           string `json:"actor"`
	TransactionHash string `json:"transactionHash"`
	BlockNumber     uint64 `json:"blockNumber"`
	ReceiptGasUsed  uint64 `json:"receiptGasUsed"`
	CalldataBytes   int    `json:"calldataBytes"`
	ZeroBytes       int    `json:"zeroBytes"`
	NonZeroBytes    int    `json:"nonZeroBytes"`
}
type logValidation struct {
	MTAppendCount   int `json:"mtAppendCount"`
	RVMTAppendCount int `json:"rvMTAppendCount"`
}
type totals struct {
	DeploymentReceiptGas uint64 `json:"deploymentReceiptGas"`
	EventReceiptGas      uint64 `json:"eventReceiptGas"`
	AllReceiptGas        uint64 `json:"allReceiptGas"`
}
type expectedCall struct{ Event, Case, Actor string }

var deploymentNames = []string{"Poseidon2BLS12381", "EntryVerifier", "TransferVerifier", "ProceedVerifier", "RecallVerifier", "MergeVerifier", "SplitVerifier", "ProcessVerifier", "ExitVerifier", "ZkDPP"}
var expectedCalls = canonicalExpectedCalls()

func canonicalExpectedCalls() []expectedCall {
	result := make([]expectedCall, len(proofspec.CanonicalEvents))
	for i, event := range proofspec.CanonicalEvents {
		result[i] = expectedCall{Event: event.Relation, Case: event.Case, Actor: event.Actor}
	}
	return result
}

func main() {
	broadcastPath := flag.String("broadcast", "", "Forge broadcast run-latest.json")
	outJSON := flag.String("out-json", "", "output JSON path")
	outCSV := flag.String("out-csv", "", "output CSV path")
	flag.Parse()
	if *broadcastPath == "" || *outJSON == "" || *outCSV == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*broadcastPath, *outJSON, *outCSV); err != nil {
		panic(err)
	}
}

func run(broadcastPath, outJSON, outCSV string) error {
	var raw broadcast
	if err := readJSON(broadcastPath, &raw); err != nil {
		return fmt.Errorf("read broadcast: %w", err)
	}
	result, err := buildReport(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outJSON), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outJSON, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	return writeCSV(outCSV, result)
}

func buildReport(raw broadcast) (report, error) {
	expectedTransactions := len(deploymentNames) + len(expectedCalls)
	if len(raw.Transactions) != expectedTransactions || len(raw.Receipts) != expectedTransactions {
		return report{}, fmt.Errorf("expected %d transactions and receipts, got %d and %d", expectedTransactions, len(raw.Transactions), len(raw.Receipts))
	}
	receipts := make(map[string]receipt, len(raw.Receipts))
	for _, item := range raw.Receipts {
		hash := normalize(item.TransactionHash)
		if hash == "" {
			return report{}, fmt.Errorf("receipt without transaction hash")
		}
		if _, exists := receipts[hash]; exists {
			return report{}, fmt.Errorf("duplicate receipt transaction hash %s", item.TransactionHash)
		}
		status, err := parseHexOrDecimal(item.Status)
		if err != nil || status != 1 {
			return report{}, fmt.Errorf("transaction %s failed with status %s", item.TransactionHash, item.Status)
		}
		receipts[hash] = item
	}
	result := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Environment: "Foundry v1.7.1, solc 0.8.30, Prague Anvil",
		MeasurementType:     "anvil-transaction-receipt",
		MeasurementBoundary: "receipt gasUsed for separate top-level transactions; includes intrinsic transaction gas, calldata or creation input gas, execution, storage, and per-transaction cold access",
		Chain:               chainMetadata{ChainID: 31337, Hardfork: "prague", GenesisTimestamp: 60000, BlockGasLimit: 30000000},
	}
	seen := make(map[string]bool, expectedTransactions)
	var previousBlock uint64
	var zkdppAddress string
	for index, tx := range raw.Transactions {
		hash := normalize(tx.Hash)
		if hash == "" || seen[hash] {
			return report{}, fmt.Errorf("missing or duplicate transaction hash at %d", index)
		}
		seen[hash] = true
		item, ok := receipts[hash]
		if !ok {
			return report{}, fmt.Errorf("missing receipt for transaction %s", tx.Hash)
		}
		block, err := parseHexOrDecimal(item.BlockNumber)
		if err != nil {
			return report{}, err
		}
		if index > 0 && block <= previousBlock {
			return report{}, fmt.Errorf("block numbers are not strictly increasing at transaction %d", index)
		}
		previousBlock = block
		gas, err := parseHexOrDecimal(item.GasUsed)
		if err != nil {
			return report{}, err
		}
		input, err := transactionInput(tx.Transaction)
		if err != nil {
			return report{}, err
		}
		inputBytes, zeroBytes, nonZeroBytes, err := byteCounts(input)
		if err != nil {
			return report{}, err
		}
		if index < len(deploymentNames) {
			name := deploymentNames[index]
			if !strings.EqualFold(tx.TransactionType, "CREATE") {
				return report{}, fmt.Errorf("transaction %d (%s) is not CREATE", index, name)
			}
			address := item.ContractAddress
			if address == "" {
				address = tx.ContractAddress
			}
			if address == "" {
				return report{}, fmt.Errorf("deployment %s has no contract address", name)
			}
			if name == "ZkDPP" {
				zkdppAddress = normalize(address)
			}
			result.Deployments = append(result.Deployments, deploymentResult{Sequence: index + 1, Component: name, TransactionHash: tx.Hash, ContractAddress: address, BlockNumber: block, ReceiptGasUsed: gas, CreationInputBytes: inputBytes, ZeroBytes: zeroBytes, NonZeroBytes: nonZeroBytes})
			result.Totals.DeploymentReceiptGas += gas
			continue
		}
		callIndex := index - len(deploymentNames)
		expected := expectedCalls[callIndex]
		if !strings.EqualFold(tx.TransactionType, "CALL") {
			return report{}, fmt.Errorf("transaction %d (%s/%s) is not CALL", index, expected.Event, expected.Case)
		}
		if tx.Function != "" && !strings.HasPrefix(tx.Function, expected.Event+"(") {
			return report{}, fmt.Errorf("transaction %d expected %s, got %s", index, expected.Event, tx.Function)
		}
		result.Calls = append(result.Calls, callResult{Sequence: callIndex + 1, Event: expected.Event, Case: expected.Case, Actor: expected.Actor, TransactionHash: tx.Hash, BlockNumber: block, ReceiptGasUsed: gas, CalldataBytes: inputBytes, ZeroBytes: zeroBytes, NonZeroBytes: nonZeroBytes})
		result.Totals.EventReceiptGas += gas
	}
	if len(seen) != len(receipts) {
		return report{}, fmt.Errorf("receipt set contains transactions absent from transaction list")
	}
	if zkdppAddress == "" {
		return report{}, fmt.Errorf("ZkDPP deployment address not found")
	}
	mtTopic, rvmtTopic := eventTopic("MTAppend(uint256,uint256,uint256)"), eventTopic("RVMTAppend(uint256,uint256,uint256)")
	for _, item := range raw.Receipts {
		for _, itemLog := range item.Logs {
			if normalize(itemLog.Address) != zkdppAddress || len(itemLog.Topics) == 0 {
				continue
			}
			switch normalize(itemLog.Topics[0]) {
			case mtTopic:
				result.LogValidation.MTAppendCount++
			case rvmtTopic:
				result.LogValidation.RVMTAppendCount++
			}
		}
	}
	if result.LogValidation.MTAppendCount != expectedMTAppends || result.LogValidation.RVMTAppendCount != expectedRVMTAppends {
		return report{}, fmt.Errorf("unexpected append logs: MT=%d rvMT=%d", result.LogValidation.MTAppendCount, result.LogValidation.RVMTAppendCount)
	}
	result.Totals.AllReceiptGas = result.Totals.DeploymentReceiptGas + result.Totals.EventReceiptGas
	return result, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func transactionInput(raw json.RawMessage) (string, error) {
	var envelope transactionEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	if envelope.Input != "" {
		return envelope.Input, nil
	}
	return envelope.Data, nil
}
func byteCounts(value string) (total, zero, nonZero int, err error) {
	value = strings.TrimPrefix(value, "0x")
	if len(value)%2 != 0 {
		return 0, 0, 0, fmt.Errorf("odd-length hex input")
	}
	data, err := hex.DecodeString(value)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, item := range data {
		if item == 0 {
			zero++
		} else {
			nonZero++
		}
	}
	return len(data), zero, nonZero, nil
}
func parseHexOrDecimal(value string) (uint64, error) {
	base := 10
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "0x") {
		base = 16
		value = strings.TrimPrefix(value, "0x")
	}
	return strconv.ParseUint(value, base, 64)
}
func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func eventTopic(signature string) string {
	hash := sha3.NewLegacyKeccak256()
	_, _ = hash.Write([]byte(signature))
	return "0x" + hex.EncodeToString(hash.Sum(nil))
}

func writeCSV(path string, value report) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"kind", "sequence", "name", "case", "actor", "transaction_hash", "contract_address", "block_number", "receipt_gas_used", "input_bytes", "zero_bytes", "nonzero_bytes"}); err != nil {
		return err
	}
	for _, item := range value.Deployments {
		if err := writer.Write([]string{"deployment", strconv.Itoa(item.Sequence), item.Component, "", "", item.TransactionHash, item.ContractAddress, u64(item.BlockNumber), u64(item.ReceiptGasUsed), strconv.Itoa(item.CreationInputBytes), strconv.Itoa(item.ZeroBytes), strconv.Itoa(item.NonZeroBytes)}); err != nil {
			return err
		}
	}
	for _, item := range value.Calls {
		if err := writer.Write([]string{"event", strconv.Itoa(item.Sequence), item.Event, item.Case, item.Actor, item.TransactionHash, "", u64(item.BlockNumber), u64(item.ReceiptGasUsed), strconv.Itoa(item.CalldataBytes), strconv.Itoa(item.ZeroBytes), strconv.Itoa(item.NonZeroBytes)}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
func u64(value uint64) string { return strconv.FormatUint(value, 10) }
