package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bighim/zkDPP/sahai-poc/internal/document"
	"github.com/bighim/zkDPP/sahai-poc/internal/events"
	"github.com/bighim/zkDPP/sahai-poc/internal/gadgetruntime"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const anvilDeployerKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

type artifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}
type forgeArtifact struct {
	ABI      abi.ABI
	Bytecode []byte
}
type deploymentResult struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	TxHash     string `json:"transactionHash"`
	GasUsed    uint64 `json:"receiptGasUsed"`
	InputBytes int    `json:"inputBytes"`
}
type eventResult struct {
	Case         string  `json:"case"`
	Event        string  `json:"event"`
	TxHash       string  `json:"transactionHash"`
	BlockNumber  uint64  `json:"blockNumber"`
	GasUsed      uint64  `json:"receiptGasUsed"`
	Calldata     int     `json:"calldataBytes"`
	ZeroBytes    int     `json:"zeroBytes"`
	NonZeroBytes int     `json:"nonZeroBytes"`
	SubmitMS     float64 `json:"submitToReceiptMs"`
	SetupGas     uint64  `json:"excludedSetupGas"`
	BuildMS      float64 `json:"buildMs"`
	WitnessMS    float64 `json:"witnessMs"`
	ProveMS      float64 `json:"proveMs"`
	EncodeMS     float64 `json:"encodeMs"`
	E2EMS        float64 `json:"e2eMs"`
}
type runResult struct {
	GeneratedAt      string             `json:"generatedAt"`
	Run              int                `json:"run"`
	Backend          string             `json:"backend"`
	RPC              string             `json:"rpc"`
	ChainID          uint64             `json:"chainId"`
	BlockGasLimit    uint64             `json:"blockGasLimit"`
	HashProfile      string             `json:"hashProfile"`
	Scenario         string             `json:"scenario"`
	ThreadMode       string             `json:"threadMode"`
	RequestedThreads int                `json:"requestedThreads"`
	GoMaxProcs       int                `json:"goMaxProcs"`
	NumCPU           int                `json:"numCPU"`
	Measurement      string             `json:"measurementBoundary"`
	ReceiptTimeout   string             `json:"receiptTimeout"`
	PollInterval     string             `json:"pollInterval"`
	FixtureSHA256    string             `json:"fixtureSha256"`
	LiveProofs       bool               `json:"liveProofs"`
	Deployments      []deploymentResult `json:"deployments"`
	Events           []eventResult      `json:"events"`
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
type fixtureFile struct {
	Metadata map[string]any            `json:"metadata"`
	Order    []string                  `json:"order"`
	Cases    map[string]events.Payload `json:"cases"`
}
type proofMarshaler interface{ MarshalSolidity() []byte }

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	backend := flag.String("backend", "anvil", "anvil or besu")
	rpcURL := flag.String("rpc", "http://127.0.0.1:8545", "EVM JSON-RPC URL")
	chainID := flag.Uint64("chain-id", 31337, "expected chain ID")
	timeout := flag.Duration("receipt-timeout", 30*time.Second, "receipt timeout")
	poll := flag.Duration("poll-interval", 10*time.Millisecond, "receipt poll interval")
	profileFlag := flag.String("profile", "poseidon2", "poseidon2 or sha256")
	scenario := flag.String("scenario", "canonical", "canonical or independent")
	threads := flag.Int("threads", 1, "1 (ST) or 8 (MT)")
	fixturePath := flag.String("fixture", "", "fixed proof fixture; derived when empty")
	live := flag.Bool("live-proofs", false, "generate proofs before submission")
	runNumber := flag.Int("run", 1, "run number")
	out := flag.String("out", "", "run JSON")
	flag.Parse()
	if *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *threads != 1 && *threads != 8 {
		fatal(fmt.Errorf("threads must be 1 or 8"))
	}
	profile, err := document.ParseProfile(*profileFlag)
	if err != nil {
		fatal(err)
	}
	if *scenario != "canonical" && *scenario != "independent" {
		fatal(fmt.Errorf("invalid scenario %q", *scenario))
	}
	if *fixturePath == "" {
		threadMode := map[int]string{1: "st", 8: "mt"}[*threads]
		*fixturePath = filepath.Join("testdata", "generated", fmt.Sprintf("%s-%s-%s-payloads.json", *scenario, profile, threadMode))
	}
	runtime.GOMAXPROCS(*threads)
	if err := run(*root, *backend, *rpcURL, *chainID, *timeout, *poll, profile, *scenario, *threads, *fixturePath, *live, *runNumber, *out); err != nil {
		fatal(err)
	}
}

func run(root, backend, rpcURL string, expectedChainID uint64, timeout, poll time.Duration, profile document.Profile, scenario string, threads int, fixturePath string, live bool, runNumber int, out string) error {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	ctx := context.Background()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	if chainID.Uint64() != expectedChainID {
		return fmt.Errorf("chain ID %s, want %d", chainID, expectedChainID)
	}
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	privateKey := os.Getenv("EVM_DEPLOYER_PK")
	if privateKey == "" && backend == "anvil" {
		privateKey = anvilDeployerKey
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return fmt.Errorf("deployer key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		return err
	}
	auth.Context = ctx
	auth.GasLimit = 29_500_000
	auth.GasTipCap = big.NewInt(1_000_000_000)
	auth.GasFeeCap = big.NewInt(100_000_000_000)
	var fixture fixtureFile
	var fixtureRaw []byte
	if live {
		fixture, err = generateLive(root, profile, scenario)
		if err == nil {
			fixtureRaw, err = json.Marshal(fixture)
		}
	} else {
		fixtureRaw, err = os.ReadFile(resolve(root, fixturePath))
		if err == nil {
			err = json.Unmarshal(fixtureRaw, &fixture)
		}
	}
	if err != nil {
		return err
	}
	if len(fixture.Order) == 0 || len(fixture.Cases) != len(fixture.Order) {
		return fmt.Errorf("invalid fixture: order=%d cases=%d", len(fixture.Order), len(fixture.Cases))
	}
	artifactPaths := map[string]string{
		"MerklePathVerifier": verifierArtifact(profile, "merklepath"),
		"EqVerifier":         verifierArtifact(profile, "eq"),
		"AddVerifier":        verifierArtifact(profile, "add"),
		"AndVerifier":        verifierArtifact(profile, "and"),
		"Ledger":             filepath.Join("contracts", "out", "BenchmarkSahaiLedger.sol", "BenchmarkSahaiLedger.json"),
	}
	loaded := map[string]forgeArtifact{}
	for name, path := range artifactPaths {
		loaded[name], err = loadArtifact(resolve(root, path))
		if err != nil {
			return fmt.Errorf("load %s from %s: %w", name, path, err)
		}
	}
	threadMode := map[int]string{1: "ST", 8: "MT"}[threads]
	result := runResult{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Run: runNumber, Backend: backend, RPC: rpcURL, ChainID: expectedChainID, BlockGasLimit: header.GasLimit, HashProfile: string(profile), Scenario: scenario, ThreadMode: threadMode, RequestedThreads: threads, GoMaxProcs: runtime.GOMAXPROCS(0), NumCPU: runtime.NumCPU(), Measurement: "Event receipt gasUsed and sequential witness/prove/encode/submit-to-receipt; independent-only seed transactions excluded", ReceiptTimeout: timeout.String(), PollInterval: poll.String(), FixtureSHA256: checksum(fixtureRaw), LiveProofs: live}
	addresses := map[string]common.Address{}
	for _, name := range []string{"MerklePathVerifier", "EqVerifier", "AddVerifier", "AndVerifier"} {
		address, tx, receipt, err := deploy(ctx, client, auth, loaded[name], timeout, poll)
		if err != nil {
			return fmt.Errorf("deploy %s: %w", name, err)
		}
		addresses[name] = address
		result.Deployments = append(result.Deployments, deployment(name, address, tx, receipt))
	}
	width := uint8(1)
	if profile == document.SHA256 {
		width = 2
	}
	address, tx, receipt, err := deploy(ctx, client, auth, loaded["Ledger"], timeout, poll, width, addresses["MerklePathVerifier"], addresses["EqVerifier"], addresses["AddVerifier"], addresses["AndVerifier"])
	if err != nil {
		return fmt.Errorf("deploy ledger: %w", err)
	}
	result.Deployments = append(result.Deployments, deployment("SahaiLedger", address, tx, receipt))
	ledger := bind.NewBoundContract(address, loaded["Ledger"].ABI, client, client, client)
	for _, base := range []struct {
		name, method string
		args         []any
	}{{"BaselineNoop", "benchmarkNoop", nil}, {"BaselineStorageWrite", "benchmarkStorageWrite", []any{new(big.Int).SetUint64(uint64(runNumber + 1))}}} {
		record, err := transact(ctx, client, auth, ledger, base.name, base.name, base.method, timeout, poll, base.args...)
		if err != nil {
			return err
		}
		result.Events = append(result.Events, record)
	}
	for _, caseName := range fixture.Order {
		payload, ok := fixture.Cases[caseName]
		if !ok {
			return fmt.Errorf("missing case %s", caseName)
		}
		var setupGas uint64
		if scenario == "independent" {
			setupGas, err = seedInputs(ctx, client, auth, ledger, payload, timeout, poll)
			if err != nil {
				return fmt.Errorf("seed %s: %w", caseName, err)
			}
		}
		method, args, err := eventCall(payload)
		if err != nil {
			return err
		}
		start := time.Now()
		tx, err := ledger.Transact(auth, method, args...)
		if err != nil {
			return fmt.Errorf("submit %s: %w", caseName, err)
		}
		receipt, err := waitReceipt(ctx, client, tx.Hash(), timeout, poll)
		duration := time.Since(start)
		if err != nil {
			return err
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("%s reverted: %s", caseName, tx.Hash())
		}
		zero, nonzero := byteCounts(tx.Data())
		result.Events = append(result.Events, eventResult{Case: caseName, Event: payload.Event, TxHash: tx.Hash().Hex(), BlockNumber: receipt.BlockNumber.Uint64(), GasUsed: receipt.GasUsed, Calldata: len(tx.Data()), ZeroBytes: zero, NonZeroBytes: nonzero, SubmitMS: ms(duration), SetupGas: setupGas, BuildMS: payload.Timing.BuildMS, WitnessMS: payload.Timing.WitnessMS, ProveMS: payload.Timing.ProveMS, EncodeMS: payload.Timing.EncodeMS, E2EMS: payload.Timing.BuildMS + payload.Timing.WitnessMS + payload.Timing.ProveMS + payload.Timing.EncodeMS + ms(duration)})
		fmt.Printf("backend=%s profile=%s threads=%s case=%-16s event=%-7s gas=%d receipt=%.3fms\n", backend, profile, threadMode, caseName, payload.Event, receipt.GasUsed, ms(duration))
	}
	return writeJSON(out, result)
}

func generateLive(root string, profile document.Profile, scenario string) (fixtureFile, error) {
	runtimes := map[string]*gadgetruntime.Loaded{}
	for _, g := range []string{"merklepath", "eq", "add", "and"} {
		loaded, err := gadgetruntime.Load(root, string(profile), g)
		if err != nil {
			return fixtureFile{}, err
		}
		runtimes[g] = loaded
	}
	start := time.Now()
	var fixtures []events.Fixture
	var err error
	if scenario == "canonical" {
		fixtures, err = events.BuildCanonical(profile)
	} else {
		fixtures, err = events.BuildIndependent(profile)
	}
	if err != nil {
		return fixtureFile{}, err
	}
	buildEach := time.Since(start) / time.Duration(len(fixtures))
	out := fixtureFile{Metadata: map[string]any{"hashProfile": profile, "scenario": scenario, "liveProofs": true}, Cases: map[string]events.Payload{}}
	for _, fixture := range fixtures {
		proofs := make([]events.Proof, 0, len(fixture.Assignments))
		var witnessTime, proveTime time.Duration
		for _, assignment := range fixture.Assignments {
			proof, witness, prove, err := runtimes[assignment.Gadget].Prove(assignment.Assignment)
			if err != nil {
				return fixtureFile{}, err
			}
			witnessTime += witness
			proveTime += prove
			public := make([]string, len(assignment.PublicInputs))
			for i, v := range assignment.PublicInputs {
				public[i] = v.String()
			}
			proofs = append(proofs, events.Proof{Gadget: assignment.Gadget, Proof: "0x" + hex.EncodeToString(proof.(proofMarshaler).MarshalSolidity()), PublicInput: public})
		}
		encodeStart := time.Now()
		payload, err := events.MakePayload(fixture, proofs, events.PayloadTiming{BuildMS: ms(buildEach), WitnessMS: ms(witnessTime), ProveMS: ms(proveTime)})
		if err != nil {
			return fixtureFile{}, err
		}
		payload.Timing.EncodeMS = ms(time.Since(encodeStart))
		out.Order = append(out.Order, fixture.Case)
		out.Cases[fixture.Case] = payload
	}
	return out, nil
}

func eventCall(payload events.Payload) (string, []any, error) {
	proofs := make([]gadgetProofABI, len(payload.Proofs))
	for i, p := range payload.Proofs {
		proofs[i] = gadgetProofABI{Proof: mustBytes(p.Proof), PublicInputs: bigSlice(p.PublicInput)}
	}
	outputs := make([]assetABI, len(payload.Outputs))
	for i, v := range payload.Outputs {
		outputs[i] = convertAsset(v)
	}
	ids := make([][32]byte, len(payload.Inputs))
	for i, v := range payload.Inputs {
		id, err := events.ParseID(v.DocHash)
		if err != nil {
			return "", nil, err
		}
		ids[i] = id
	}
	switch payload.Event {
	case "Entry":
		return "entry", []any{outputs[0], proofs}, nil
	case "Ship":
		return "ship", []any{ids[0], outputs[0], proofs}, nil
	case "Merge", "Process":
		return strings.ToLower(payload.Event), []any{[2][32]byte{ids[0], ids[1]}, outputs[0], proofs}, nil
	case "Split":
		return "split", []any{ids[0], [2]assetABI{outputs[0], outputs[1]}, proofs}, nil
	case "Exit":
		return "exit", []any{ids[0], outputs[0], proofs}, nil
	}
	return "", nil, fmt.Errorf("unknown Event %q", payload.Event)
}
func convertAsset(value events.AssetJSON) assetABI {
	id, err := events.ParseID(value.DocHash)
	if err != nil {
		panic(err)
	}
	return assetABI{DocumentType: [32]byte(crypto.Keccak256Hash([]byte(value.Type))), DocHash: id, Terminal: value.Terminal}
}
func seedInputs(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, contract *bind.BoundContract, payload events.Payload, timeout, poll time.Duration) (uint64, error) {
	var total uint64
	for _, input := range payload.Inputs {
		tx, err := contract.Transact(auth, "seedBenchmarkInput", convertAsset(input))
		if err != nil {
			return total, err
		}
		receipt, err := waitReceipt(ctx, client, tx.Hash(), timeout, poll)
		if err != nil {
			return total, err
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return total, fmt.Errorf("seed reverted")
		}
		total += receipt.GasUsed
	}
	return total, nil
}
func transact(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, contract *bind.BoundContract, caseName, eventName, method string, timeout, poll time.Duration, args ...any) (eventResult, error) {
	start := time.Now()
	tx, err := contract.Transact(auth, method, args...)
	if err != nil {
		return eventResult{}, err
	}
	receipt, err := waitReceipt(ctx, client, tx.Hash(), timeout, poll)
	duration := time.Since(start)
	if err != nil {
		return eventResult{}, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return eventResult{}, fmt.Errorf("%s reverted", eventName)
	}
	zero, nonzero := byteCounts(tx.Data())
	return eventResult{Case: caseName, Event: eventName, TxHash: tx.Hash().Hex(), BlockNumber: receipt.BlockNumber.Uint64(), GasUsed: receipt.GasUsed, Calldata: len(tx.Data()), ZeroBytes: zero, NonZeroBytes: nonzero, SubmitMS: ms(duration)}, nil
}
func loadArtifact(path string) (forgeArtifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return forgeArtifact{}, err
	}
	var value artifact
	if err = json.Unmarshal(raw, &value); err != nil {
		return forgeArtifact{}, err
	}
	parsed, err := abi.JSON(bytes.NewReader(value.ABI))
	if err != nil {
		return forgeArtifact{}, err
	}
	code, err := hex.DecodeString(strings.TrimPrefix(value.Bytecode.Object, "0x"))
	return forgeArtifact{ABI: parsed, Bytecode: code}, err
}
func deploy(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, a forgeArtifact, timeout, poll time.Duration, args ...any) (common.Address, *types.Transaction, *types.Receipt, error) {
	address, tx, _, err := bind.DeployContract(auth, a.ABI, a.Bytecode, client, args...)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	receipt, err := waitReceipt(ctx, client, tx.Hash(), timeout, poll)
	if err != nil {
		return common.Address{}, tx, nil, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return common.Address{}, tx, receipt, fmt.Errorf("deployment reverted")
	}
	return address, tx, receipt, nil
}
func deployment(name string, address common.Address, tx *types.Transaction, receipt *types.Receipt) deploymentResult {
	return deploymentResult{Name: name, Address: address.Hex(), TxHash: tx.Hash().Hex(), GasUsed: receipt.GasUsed, InputBytes: len(tx.Data())}
}
func waitReceipt(parent context.Context, client *ethclient.Client, hash common.Hash, timeout, poll time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
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
		case <-time.After(poll):
		}
	}
}
func bigSlice(values []string) []*big.Int {
	out := make([]*big.Int, len(values))
	for i, v := range values {
		out[i] = mustBig(v)
	}
	return out
}
func mustBig(value string) *big.Int {
	base := 10
	encoded := value
	if strings.HasPrefix(value, "0x") {
		base = 16
		encoded = strings.TrimPrefix(value, "0x")
	}
	x, ok := new(big.Int).SetString(encoded, base)
	if !ok {
		panic("invalid integer " + value)
	}
	return x
}
func mustBytes(value string) []byte {
	raw, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		panic(err)
	}
	return raw
}
func byteCounts(data []byte) (zero, nonzero int) {
	for _, v := range data {
		if v == 0 {
			zero++
		} else {
			nonzero++
		}
	}
	return
}
func checksum(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
func verifierArtifact(profile document.Profile, gadget string) string {
	prefix := "Poseidon2"
	if profile == document.SHA256 {
		prefix = "SHA256"
	}
	suffix := map[string]string{"merklepath": "MerklePath", "eq": "Eq", "add": "Add", "and": "And"}[gadget]
	return filepath.Join("contracts", "out", "PlonkVerifier.sol", prefix+suffix+"Verifier.json")
}
func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}
func ms(value time.Duration) float64 { return float64(value) / float64(time.Millisecond) }
func fatal(err error)                { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
