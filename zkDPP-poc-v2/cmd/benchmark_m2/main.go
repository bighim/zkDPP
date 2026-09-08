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
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m2case"
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

type fixedProof struct {
	Proof        string   `json:"proof"`
	PublicInputs []string `json:"publicInputs"`
}

type fixedEntry struct {
	Name       string `json:"name"`
	Commitment string `json:"commitment"`
	fixedProof
}

type fixedExit struct {
	Root      string `json:"root"`
	Nullifier string `json:"nullifier"`
	fixedProof
}

type fixedFixtures struct {
	Entries []fixedEntry `json:"entries"`
	Exit    fixedExit    `json:"exit"`
}

type TxResult struct {
	Kind                string  `json:"kind"`
	Name                string  `json:"name"`
	Case                string  `json:"case,omitempty"`
	Actor               string  `json:"actor"`
	TransactionHash     string  `json:"transactionHash"`
	ContractAddress     string  `json:"contractAddress,omitempty"`
	BlockNumber         uint64  `json:"blockNumber"`
	ReceiptGasUsed      uint64  `json:"receiptGasUsed"`
	CalldataBytes       int     `json:"calldataBytes"`
	WitnessMillis       float64 `json:"witnessMillis,omitempty"`
	ProveMillis         float64 `json:"proveMillis,omitempty"`
	TxPrepareMillis     float64 `json:"txPrepareMillis,omitempty"`
	SubmitReceiptMillis float64 `json:"submitReceiptMillis,omitempty"`
	E2EMillis           float64 `json:"e2eMillis,omitempty"`
}

type FinalState struct {
	LeafCount      uint64 `json:"leafCount"`
	CurrentRoot    string `json:"currentRoot"`
	ExpectedRoot   string `json:"expectedRoot"`
	ExitNullifier  string `json:"exitNullifier"`
	NullifierSpent bool   `json:"nullifierSpent"`
	PathMatches    bool   `json:"pathMatches"`
}

type Report struct {
	GeneratedAt  string     `json:"generatedAt"`
	Mode         string     `json:"mode"`
	RunCount     int        `json:"runCount"`
	ChainID      uint64     `json:"chainId"`
	RPC          string     `json:"rpc"`
	GoVersion    string     `json:"goVersion"`
	Transactions []TxResult `json:"transactions"`
	FinalState   FinalState `json:"finalState"`
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
	root := flag.String("root", ".", "zkDPP-poc-v1 root")
	flag.Parse()
	if *mode != "gas" && *mode != "e2e" {
		panic("mode must be gas or e2e")
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
	if err := command(root, "docker", "compose", "run", "--rm", "foundry", "forge", "build"); err != nil {
		return err
	}
	senders := make([]sender, 5)
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
	poseidonABI, poseidonCode, err := loadContract(root, "Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if err != nil {
		return err
	}
	entryVerifierABI, entryVerifierCode, err := loadContract(root, "EntryVerifier.sol", "EntryVerifier")
	if err != nil {
		return err
	}
	privateVerifierABI, privateVerifierCode, err := loadContract(root, "PrivateSpendVerifier.sol", "PrivateSpendVerifier")
	if err != nil {
		return err
	}
	ledgerABI, ledgerCode, err := loadContract(root, "EntryExitLedger.sol", "EntryExitLedger")
	if err != nil {
		return err
	}
	_ = poseidonABI
	_ = entryVerifierABI
	_ = privateVerifierABI
	poseidonAddress, item, err := deploy(ctx, client, senders[0], chainID, "poseidon2", poseidonCode)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, item)
	entryVerifierAddress, item, err := deploy(ctx, client, senders[0], chainID, "entry-verifier", entryVerifierCode)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, item)
	privateVerifierAddress, item, err := deploy(ctx, client, senders[0], chainID, "private-spend-verifier", privateVerifierCode)
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, item)
	constructor, err := ledgerABI.Pack("", entryVerifierAddress, privateVerifierAddress, poseidonAddress)
	if err != nil {
		return err
	}
	ledgerAddress, item, err := deploy(ctx, client, senders[0], chainID, "entry-exit-ledger", append(ledgerCode, constructor...))
	if err != nil {
		return err
	}
	report.Transactions = append(report.Transactions, item)
	for i := 1; i <= 3; i++ {
		data, err := ledgerABI.Pack("setEntryIssuer", senders[i].address, true)
		if err != nil {
			return err
		}
		result, err := sendCall(ctx, client, senders[0], chainID, ledgerAddress, data, "authorization", "set-entry-issuer", fmt.Sprintf("account-%d", i))
		if err != nil {
			return err
		}
		report.Transactions = append(report.Transactions, result)
	}
	scenario, err := m2case.Build(root)
	if err != nil {
		return err
	}
	fixtures, err := loadFixtures(filepath.Join(root, "contracts", "test", "fixtures", "m2-proofs.json"))
	if err != nil {
		return err
	}
	entryLoaded, err := artifact.LoadAt(root, "m2", "entry")
	if err != nil {
		return err
	}
	privateLoaded, err := artifact.Load(root, "private-spend")
	if err != nil {
		return err
	}
	for i, entry := range scenario.Entries {
		var proof []byte
		var witnessTime, proveTime time.Duration
		eventStart := time.Now()
		if mode == "gas" {
			proof, err = decodeHex(fixtures.Entries[i].Proof)
		} else {
			proof, witnessTime, proveTime, err = liveProof(entryLoaded, entry.Assignment)
		}
		if err != nil {
			return err
		}
		data, err := ledgerABI.Pack("entry", proof, entry.Note.Commitment.BigInt(new(big.Int)))
		if err != nil {
			return err
		}
		result, err := sendCall(ctx, client, senders[i+1], chainID, ledgerAddress, data, "event", "entry", entry.Name)
		if err != nil {
			return err
		}
		if mode == "e2e" {
			result.WitnessMillis = milliseconds(witnessTime)
			result.ProveMillis = milliseconds(proveTime)
			result.E2EMillis = milliseconds(time.Since(eventStart))
		}
		report.Transactions = append(report.Transactions, result)
	}
	var exitProof []byte
	var witnessTime, proveTime time.Duration
	exitStart := time.Now()
	if mode == "gas" {
		exitProof, err = decodeHex(fixtures.Exit.Proof)
	} else {
		exitProof, witnessTime, proveTime, err = liveProof(privateLoaded, scenario.ExitAssignment)
	}
	if err != nil {
		return err
	}
	exitNFElement, ok := scenario.ExitAssignment.NF.(fr.Element)
	if !ok {
		return fmt.Errorf("exit nullifier is %T, want fr.Element", scenario.ExitAssignment.NF)
	}
	exitNF := exitNFElement.BigInt(new(big.Int))
	exitData, err := ledgerABI.Pack("exit", exitProof, scenario.FinalRoot.BigInt(new(big.Int)), exitNF)
	if err != nil {
		return err
	}
	exitResult, err := sendCall(ctx, client, senders[1], chainID, ledgerAddress, exitData, "event", "exit", "raw-material-a")
	if err != nil {
		return err
	}
	if mode == "e2e" {
		exitResult.WitnessMillis = milliseconds(witnessTime)
		exitResult.ProveMillis = milliseconds(proveTime)
		exitResult.E2EMillis = milliseconds(time.Since(exitStart))
	}
	report.Transactions = append(report.Transactions, exitResult)
	state, err := validateState(ctx, client, ledgerABI, ledgerAddress, scenario, exitNF)
	if err != nil {
		return err
	}
	report.FinalState = state
	path := filepath.Join(root, "output", "m2-anvil-"+mode+".json")
	if err := artifact.WriteJSON(path, report); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}

func liveProof(loaded *artifact.Loaded, assignment frontend.Circuit) ([]byte, time.Duration, time.Duration, error) {
	witnessStart := time.Now()
	witness, err := frontend.NewWitness(assignment, ecc.BLS12_381.ScalarField())
	witnessTime := time.Since(witnessStart)
	if err != nil {
		return nil, witnessTime, 0, err
	}
	proveStart := time.Now()
	proof, err := plonk.Prove(loaded.CCS, loaded.PK, witness)
	proveTime := time.Since(proveStart)
	if err != nil {
		return nil, witnessTime, proveTime, err
	}
	return proof.(solidityMarshaler).MarshalSolidity(), witnessTime, proveTime, nil
}

func deploy(ctx context.Context, client *ethclient.Client, signer sender, chainID *big.Int, name string, data []byte) (common.Address, TxResult, error) {
	prepared, err := prepare(ctx, client, signer, chainID, nil, data)
	if err != nil {
		return common.Address{}, TxResult{}, err
	}
	receipt, submit, err := submit(ctx, client, prepared.tx)
	if err != nil {
		return common.Address{}, TxResult{}, err
	}
	return receipt.ContractAddress, TxResult{Kind: "deployment", Name: name, Actor: signer.address.Hex(), TransactionHash: receipt.TxHash.Hex(), ContractAddress: receipt.ContractAddress.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), ReceiptGasUsed: receipt.GasUsed, CalldataBytes: prepared.calldata, TxPrepareMillis: milliseconds(prepared.prepare), SubmitReceiptMillis: milliseconds(submit)}, nil
}

func sendCall(ctx context.Context, client *ethclient.Client, signer sender, chainID *big.Int, to common.Address, data []byte, kind, name, caseName string) (TxResult, error) {
	prepared, err := prepare(ctx, client, signer, chainID, &to, data)
	if err != nil {
		return TxResult{}, err
	}
	receipt, submitTime, err := submit(ctx, client, prepared.tx)
	if err != nil {
		return TxResult{}, err
	}
	return TxResult{Kind: kind, Name: name, Case: caseName, Actor: signer.address.Hex(), TransactionHash: receipt.TxHash.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), ReceiptGasUsed: receipt.GasUsed, CalldataBytes: prepared.calldata, TxPrepareMillis: milliseconds(prepared.prepare), SubmitReceiptMillis: milliseconds(submitTime)}, nil
}

func prepare(ctx context.Context, client *ethclient.Client, signer sender, chainID *big.Int, to *common.Address, data []byte) (preparedTx, error) {
	start := time.Now()
	nonce, err := client.PendingNonceAt(ctx, signer.address)
	if err != nil {
		return preparedTx{}, err
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return preparedTx{}, err
	}
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{From: signer.address, To: to, Data: data})
	if err != nil {
		return preparedTx{}, err
	}
	var tx *types.Transaction
	if to == nil {
		tx = types.NewContractCreation(nonce, big.NewInt(0), gas, gasPrice, data)
	} else {
		tx = types.NewTransaction(nonce, *to, big.NewInt(0), gas, gasPrice, data)
	}
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), signer.key)
	if err != nil {
		return preparedTx{}, err
	}
	return preparedTx{tx: signed, calldata: len(data), prepare: time.Since(start)}, nil
}

func submit(ctx context.Context, client *ethclient.Client, tx *types.Transaction) (*types.Receipt, time.Duration, error) {
	start := time.Now()
	if err := client.SendTransaction(ctx, tx); err != nil {
		return nil, 0, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err == nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				return nil, 0, fmt.Errorf("transaction %s reverted", tx.Hash())
			}
			return receipt, time.Since(start), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("receipt timeout for %s", tx.Hash())
}

func validateState(ctx context.Context, client *ethclient.Client, contractABI abi.ABI, address common.Address, scenario *m2case.Scenario, nf *big.Int) (FinalState, error) {
	count, err := callUint(ctx, client, contractABI, address, "noteLeafCount")
	if err != nil {
		return FinalState{}, err
	}
	root, err := callBig(ctx, client, contractABI, address, "currentNoteRoot")
	if err != nil {
		return FinalState{}, err
	}
	spent, err := callBool(ctx, client, contractABI, address, "noteNullifiers", nf)
	if err != nil {
		return FinalState{}, err
	}
	data, _ := contractABI.Pack("getNotePath", big.NewInt(0))
	output, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return FinalState{}, err
	}
	values, err := contractABI.Unpack("getNotePath", output)
	if err != nil {
		return FinalState{}, err
	}
	siblings := values[1].([]*big.Int)
	pathMatches := len(siblings) == len(scenario.ExitPath.Siblings)
	if pathMatches {
		for i := range siblings {
			want := scenario.ExitPath.Siblings[i].BigInt(new(big.Int))
			if siblings[i].Cmp(want) != 0 {
				pathMatches = false
				break
			}
		}
	}
	expected := scenario.FinalRoot.BigInt(new(big.Int))
	if count != 3 || root.Cmp(expected) != 0 || !spent || !pathMatches {
		return FinalState{}, fmt.Errorf("unexpected final ledger state")
	}
	return FinalState{LeafCount: count, CurrentRoot: root.String(), ExpectedRoot: expected.String(), ExitNullifier: nf.String(), NullifierSpent: spent, PathMatches: pathMatches}, nil
}

func callUint(ctx context.Context, client *ethclient.Client, contractABI abi.ABI, address common.Address, method string) (uint64, error) {
	value, err := callBig(ctx, client, contractABI, address, method)
	if err != nil {
		return 0, err
	}
	return value.Uint64(), nil
}

func callBig(ctx context.Context, client *ethclient.Client, contractABI abi.ABI, address common.Address, method string, args ...any) (*big.Int, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	output, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := contractABI.Unpack(method, output)
	if err != nil {
		return nil, err
	}
	return values[0].(*big.Int), nil
}

func callBool(ctx context.Context, client *ethclient.Client, contractABI abi.ABI, address common.Address, method string, args ...any) (bool, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return false, err
	}
	output, err := client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return false, err
	}
	values, err := contractABI.Unpack(method, output)
	if err != nil {
		return false, err
	}
	return values[0].(bool), nil
}

func loadContract(root, source, name string) (abi.ABI, []byte, error) {
	path := filepath.Join(root, "contracts", "out", source, name+".json")
	encoded, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, nil, err
	}
	var raw contractArtifact
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return abi.ABI{}, nil, err
	}
	parsed, err := abi.JSON(bytes.NewReader(raw.ABI))
	if err != nil {
		return abi.ABI{}, nil, err
	}
	code, err := decodeHex(raw.Bytecode.Object)
	return parsed, code, err
}

func loadFixtures(path string) (fixedFixtures, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return fixedFixtures{}, err
	}
	var result fixedFixtures
	err = json.Unmarshal(encoded, &result)
	return result, err
}

func decodeHex(value string) ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(value, "0x"))
}

func deriveSender(root string, index int) (sender, error) {
	output, err := outputCommand(root, "docker", "compose", "run", "--rm", "foundry", "cast", "wallet", "private-key", "--mnemonic", mnemonic, "--mnemonic-index", fmt.Sprintf("%d", index))
	if err != nil {
		return sender{}, err
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(output), "0x"))
	if err != nil {
		return sender{}, err
	}
	return sender{key: key, address: crypto.PubkeyToAddress(key.PublicKey)}, nil
}

func waitClient() (*ethclient.Client, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		client, err := ethclient.Dial(rpcURL)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, pingErr := client.BlockNumber(ctx)
			cancel()
			if pingErr == nil {
				return client, nil
			}
			client.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("anvil did not become ready")
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, output)
	}
	return string(output), nil
}

func milliseconds(value time.Duration) float64 { return float64(value.Microseconds()) / 1000 }

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
