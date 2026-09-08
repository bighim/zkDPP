package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m8audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m8run"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const project = "zkdpp-m8"
const rpcURL = "http://127.0.0.1:18545"

type environment struct {
	Go, OS, Arch           string
	GOMAXPROCS             int
	Foundry, Solidity, EVM string
	ChainID, BlockGasLimit uint64
	RPC                    string
}
type finalState struct {
	NoteLeaves, AuditRecords                                               uint64
	DPPProduced, StandardClaim, StrictClaim, StandardActive, StrictRevoked bool
}
type gasReport struct {
	RunCount, Attempt  int
	Environment        environment
	PublicKeyChecksum  string
	LedgerRuntimeBytes int
	Transactions       []m7run.Transaction
	Final              finalState
	NegativeChecks     int
}
type auditReport struct {
	RunCount, Attempt     int
	Environment           environment
	PublicKeyChecksum     string
	VerifyClaimCallMillis float64
	Claim                 m8audit.Result
	Expected              bool
}

func main() {
	mode := flag.String("mode", "", "gas|audit|checks")
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var err error
	if *mode == "checks" {
		err = checks(*root)
	} else {
		err = run(*root, *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func env() environment {
	return environment{runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), "1.7.1", "0.8.30", "Prague", 31337, 30000000, rpcURL}
}
func compose(root string, args ...string) error {
	all := append([]string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.m8.yml", "-p", project}, args...)
	c := exec.Command("docker", all...)
	c.Dir = root
	c.Env = append(os.Environ(), "ANVIL_PORT=18545", "FOUNDRY_IMAGE="+m8run.FoundryImage)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func start(root string) error {
	if c, err := net.DialTimeout("tcp", "127.0.0.1:18545", 300*time.Millisecond); err == nil {
		c.Close()
		return fmt.Errorf("port 18545 already in use")
	}
	if err := compose(root, "up", "-d", "anvil"); err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		c, err := ethclient.Dial(rpcURL)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = c.BlockNumber(ctx)
			cancel()
			c.Close()
			if err == nil {
				return compose(root, "run", "--rm", "foundry", "forge", "build")
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("Anvil not ready")
}
func stop(root string) { _ = compose(root, "down", "--remove-orphans") }

func run(root, mode string) (runErr error) {
	if mode != "gas" && mode != "audit" {
		return fmt.Errorf("unknown mode")
	}
	output := "output/m8-anvil-gas.json"
	if mode == "audit" {
		output = "output/m8-audit.json"
	}
	attempt, finish, err := m8run.Begin(root, mode, output)
	if err != nil {
		return err
	}
	defer func() { finish(runErr) }()
	if err = m8run.CheckBaseline(root); err != nil {
		return err
	}
	if err = start(root); err != nil {
		return err
	}
	defer stop(root)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	var accounts []common.Address
	if err = client.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil || len(accounts) < 5 {
		return fmt.Errorf("Anvil accounts: %w", err)
	}
	committee, err := m8run.Committee(root)
	if err != nil {
		return err
	}
	fixture, err := m8run.LoadFixture(root)
	if err != nil {
		return err
	}
	m7fixture, err := m7run.LoadFixture(root)
	if err != nil {
		return err
	}
	var processGroup m7run.FixedGroup
	for _, g := range m7fixture.Groups {
		if g.Name == "process" {
			processGroup = g
		}
	}
	if len(processGroup.Events) != 4 {
		return fmt.Errorf("missing M7 process fixture")
	}
	ledgerABI, _, ledgerSize, err := m7run.Contract(root, "ZkDPPClaimLedger.sol", "ZkDPPClaimLedger")
	if err != nil {
		return err
	}
	if ledgerSize > 24576 {
		return fmt.Errorf("ledger runtime %d exceeds EIP-170", ledgerSize)
	}
	transactions := []m7run.Transaction{}
	send := func(from common.Address, to *common.Address, data []byte, name string) (*types.Receipt, error) {
		receipt, row, e := m7run.Send(ctx, client, from, to, data, name)
		if e != nil {
			return receipt, e
		}
		if mode == "gas" {
			row.SSTORECount, row.SSTOREOpcodeGas, e = m7run.StoreTrace(ctx, client, row.Hash)
			if e != nil {
				return receipt, e
			}
			transactions = append(transactions, row)
		}
		return receipt, nil
	}
	deploy := func(source, name string, args ...any) (common.Address, error) {
		a, code, size, e := m7run.Contract(root, source, name)
		if e != nil {
			return common.Address{}, e
		}
		if size > 24576 {
			return common.Address{}, fmt.Errorf("%s runtime %d", name, size)
		}
		packed, e := a.Pack("", args...)
		if e != nil {
			return common.Address{}, e
		}
		receipt, e := send(accounts[0], nil, append(code, packed...), name+"/deployment")
		if e != nil {
			return common.Address{}, e
		}
		return receipt.ContractAddress, nil
	}
	hasher, err := deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if err != nil {
		return err
	}
	names := []string{"M7EntryVerifier", "M7TransferVerifier", "M7ProceedVerifier", "M7RecallVerifier", "M7MergeVerifier", "M7SplitVerifier", "AuditProcessVerifier", "M8ExitVerifier", "IssueStandardV1Verifier", "IssueStrictV2Verifier"}
	addresses := make([]common.Address, len(names))
	for i, n := range names {
		addresses[i], err = deploy(n+".sol", n)
		if err != nil {
			return err
		}
	}
	var baseVerifiers [8]common.Address
	copy(baseVerifiers[:], addresses[:8])
	ledger, err := deploy("ZkDPPClaimLedger.sol", "ZkDPPClaimLedger", baseVerifiers, hasher, accounts[4])
	if err != nil {
		return err
	}
	code, err := client.CodeAt(ctx, ledger, nil)
	if err != nil {
		return err
	}
	deployHeader, e := client.HeaderByNumber(ctx, nil)
	if e != nil {
		return e
	}
	deploymentBlock := deployHeader.Number.Uint64()
	call := func(from common.Address, name string, args ...any) error {
		data, e := ledgerABI.Pack(name, args...)
		if e != nil {
			return e
		}
		_, e = send(from, &ledger, data, name)
		return e
	}
	if err = call(accounts[0], "setEntryIssuer", accounts[1], true); err != nil {
		return err
	}
	if err = call(accounts[0], "registerPolicyAuthority", accounts[0]); err != nil {
		return err
	}
	process := processGroup.Events[3]
	_, processBase, _, err := process.Decode()
	if err != nil {
		return err
	}
	if err = call(accounts[0], "reservePolicy", uint8(6)); err != nil {
		return err
	}
	dummyHash := [32]byte{}
	dummyHash[31] = 1
	if err = call(accounts[0], "registerPolicy", audit.Big(processBase[0]), uint8(3), uint8(2), dummyHash, addresses[6]); err != nil {
		return err
	}
	if err = call(accounts[0], "setPolicyGrant", audit.Big(processBase[0]), audit.Big(processBase[1]), true); err != nil {
		return err
	}
	stdProof, stdInputs, err := fixture.Standard.Decode()
	if err != nil {
		return err
	}
	strictProof, strictInputs, err := fixture.Strict.Decode()
	if err != nil {
		return err
	}
	stdHash, err := hash32(fixture.StandardVKHash)
	if err != nil {
		return err
	}
	strictHash, err := hash32(fixture.StrictVKHash)
	if err != nil {
		return err
	}
	if err = call(accounts[0], "reservePolicy", uint8(8)); err != nil {
		return err
	}
	if err = call(accounts[0], "registerPolicy", audit.Big(stdInputs[0]), uint8(1), uint8(1), stdHash, addresses[8]); err != nil {
		return err
	}
	if err = call(accounts[0], "reservePolicyVersion", uint64(2)); err != nil {
		return err
	}
	if err = call(accounts[0], "registerPolicy", audit.Big(strictInputs[0]), uint8(1), uint8(1), strictHash, addresses[9]); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		data, e := m7run.CaseData(ledgerABI, processGroup.Events[i])
		if e != nil {
			return e
		}
		if _, e = send(accounts[1], &ledger, data, processGroup.Events[i].Name); e != nil {
			return e
		}
	}
	data, err := m7run.CaseData(ledgerABI, process)
	if err != nil {
		return err
	}
	if _, err = send(accounts[1], &ledger, data, "process"); err != nil {
		return err
	}
	exitProof, exitInputs, err := fixture.EligibleExit.Decode()
	if err != nil {
		return err
	}
	exitAudit := audit.CipherArg{R1X: audit.Big(exitInputs[3]), R1Y: audit.Big(exitInputs[4]), EncryptedParents: []*big.Int{audit.Big(exitInputs[5])}, EncryptedOutputNfs: []*big.Int{}}
	data, err = ledgerABI.Pack("exit", exitProof, audit.Big(exitInputs[0]), audit.Big(exitInputs[1]), audit.Big(exitInputs[2]), exitAudit)
	if err != nil {
		return err
	}
	if _, err = send(accounts[1], &ledger, data, "exit-dpp"); err != nil {
		return err
	}
	data, err = ledgerABI.Pack("issue", stdProof, audit.Big(stdInputs[0]), audit.Big(stdInputs[1]))
	if err != nil {
		return err
	}
	if _, err = send(accounts[1], &ledger, data, "issue-standard"); err != nil {
		return err
	}
	negative := 0
	if mode == "gas" {
		row, e := sendRevert(ctx, client, accounts[1], ledger, data, "issue-standard-duplicate")
		if e != nil {
			return e
		}
		transactions = append(transactions, row)
		negative++
	}
	data, err = ledgerABI.Pack("issue", strictProof, audit.Big(strictInputs[0]), audit.Big(strictInputs[1]))
	if err != nil {
		return err
	}
	if _, err = send(accounts[1], &ledger, data, "issue-strict"); err != nil {
		return err
	}
	if err = call(accounts[4], "setClaimStatus", audit.Big(stdInputs[1]), audit.Big(stdInputs[0]), uint8(1)); err != nil {
		return err
	}
	if err = call(accounts[4], "setClaimStatus", audit.Big(stdInputs[1]), audit.Big(stdInputs[0]), uint8(0)); err != nil {
		return err
	}
	if err = call(accounts[4], "setClaimStatus", audit.Big(strictInputs[1]), audit.Big(strictInputs[0]), uint8(1)); err != nil {
		return err
	}
	if err = call(accounts[4], "setClaimStatus", audit.Big(strictInputs[1]), audit.Big(strictInputs[0]), uint8(2)); err != nil {
		return err
	}
	verifyData, err := ledgerABI.Pack("verifyClaim", audit.Big(stdInputs[1]), audit.Big(stdInputs[0]))
	if err != nil {
		return err
	}
	callStart := time.Now()
	raw, err := client.CallContract(ctx, ethereumCall(ledger, verifyData), nil)
	callMS := m8run.Millis(time.Since(callStart))
	if err != nil {
		return err
	}
	values, err := ledgerABI.Unpack("verifyClaim", raw)
	if err != nil || len(values) != 3 || !values[0].(bool) || values[1].(uint8) != 0 {
		return fmt.Errorf("verifyClaim result")
	}
	if mode == "gas" {
		if _, err = send(accounts[1], &ledger, verifyData, "verifyClaim-transaction"); err != nil {
			return err
		}
	}
	if mode == "gas" {
		state := finalState{NoteLeaves: 5, AuditRecords: 7, DPPProduced: true, StandardClaim: true, StrictClaim: true, StandardActive: true, StrictRevoked: true}
		report := gasReport{1, attempt, env(), committee.Public.Checksum, ledgerSize, transactions, state, negative}
		return m8run.Write(root, output, report)
	}
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	snap := audit.Snapshot{Number: header.Number.Uint64(), Hash: header.Hash().Hex()}
	source := &audit.RPCSource{Client: client, ABI: ledgerABI, Deployment: audit.Deployment{ChainID: 31337, Ledger: ledger.Hex(), DeploymentBlock: deploymentBlock, LedgerCodeSHA256: audit.CodeHash(code), PublicKeyChecksum: committee.Public.Checksum}}
	claim, err := m8audit.TraceClaim(ctx, source, committee, stdInputs[1], stdInputs[0], snap)
	if err != nil {
		return err
	}
	expected := claim.Metrics.Records == 6 && claim.Metrics.Decryptions == 2 && claim.Metrics.Responses == 4 && len(claim.Upstream.Entries) == 3
	return m8run.Write(root, output, auditReport{1, attempt, env(), committee.Public.Checksum, callMS, claim, expected})
}

func ethereumCall(to common.Address, data []byte) ethereum.CallMsg {
	return ethereum.CallMsg{To: &to, Data: data}
}
func hash32(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return out, fmt.Errorf("VK hash")
	}
	copy(out[:], b)
	return out, nil
}
func sendRevert(ctx context.Context, c *ethclient.Client, from, to common.Address, data []byte, name string) (m7run.Transaction, error) {
	row := m7run.Transaction{Name: name, From: from.Hex(), CalldataBytes: len(data)}
	tx := map[string]any{"from": from, "to": to, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(30000000)}
	var hash common.Hash
	start := time.Now()
	if err := c.Client().CallContext(ctx, &hash, "eth_sendTransaction", tx); err != nil {
		return row, err
	}
	for {
		receipt, err := c.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status != 0 {
				return row, fmt.Errorf("expected revert")
			}
			row.Hash = hash.Hex()
			row.BlockNumber = receipt.BlockNumber.Uint64()
			row.GasUsed = receipt.GasUsed
			row.SubmitReceiptMillis = m8run.Millis(time.Since(start))
			return row, nil
		}
		select {
		case <-ctx.Done():
			return row, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func checks(root string) error {
	if err := m8run.CheckBaseline(root); err != nil {
		return err
	}
	for _, name := range []string{"m8-circuit.json", "m8-anvil-gas.json", "m8-audit.json"} {
		var h struct{ RunCount int }
		if err := m8run.Read(filepath.Join(root, "output", name), &h); err != nil {
			return err
		}
		if h.RunCount != 1 {
			return fmt.Errorf("run count %s", name)
		}
	}
	var circuit struct {
		Results []struct {
			Relation                  string
			Constraints, PublicInputs int
			VKHash                    string
		}
		TotalProofCalls     map[string]int
		ProofReloadVerified bool
	}
	if err := m8run.Read(filepath.Join(root, "output/m8-circuit.json"), &circuit); err != nil {
		return err
	}
	if len(circuit.Results) != 3 || !circuit.ProofReloadVerified || circuit.TotalProofCalls["exit-dpp"] != 2 {
		return fmt.Errorf("incomplete M8 Circuit report")
	}
	wantPublic := map[string]int{"exit-dpp": 6, "issue-standard-v1": 2, "issue-strict-v2": 2}
	for _, r := range circuit.Results {
		loaded, e := m8run.Load(root, r.Relation)
		if e != nil {
			return e
		}
		if r.Constraints != loaded.Manifest.Constraints || r.PublicInputs != wantPublic[r.Relation] || r.PublicInputs != loaded.Manifest.PublicInputs {
			return fmt.Errorf("Circuit/artifact mismatch: %s", r.Relation)
		}
	}
	fixture, e := m8run.LoadFixture(root)
	if e != nil {
		return e
	}
	m7sum, e := artifact.Checksum(filepath.Join(root, "contracts/test/fixtures/m7-proofs.json"))
	if e != nil {
		return e
	}
	if fixture.M7FixtureChecksum != m7sum {
		return fmt.Errorf("M7 fixture changed")
	}
	var gas gasReport
	if err := m8run.Read(filepath.Join(root, "output/m8-anvil-gas.json"), &gas); err != nil {
		return err
	}
	if gas.LedgerRuntimeBytes > 24576 || gas.Final.NoteLeaves != 5 || gas.Final.AuditRecords != 7 || !gas.Final.DPPProduced || !gas.Final.StandardClaim || !gas.Final.StrictClaim || !gas.Final.StandardActive || !gas.Final.StrictRevoked || gas.NegativeChecks != 1 {
		return fmt.Errorf("gas/final state mismatch")
	}
	var a auditReport
	if err := m8run.Read(filepath.Join(root, "output/m8-audit.json"), &a); err != nil {
		return err
	}
	if !a.Expected || a.Claim.Metrics.Records != 6 || a.Claim.Metrics.Decryptions != 2 || a.Claim.Metrics.Responses != 4 {
		return fmt.Errorf("audit result mismatch")
	}
	files, err := m8run.GeneratedFiles(root)
	if err != nil {
		return err
	}
	var base m8run.Baseline
	if err = m8run.Read(m8run.Dir(root, "baseline.json"), &base); err != nil {
		return err
	}
	attempts := []m8run.Attempt{}
	paths, _ := filepath.Glob(m8run.Dir(root, "attempts", "*.json"))
	for _, p := range paths {
		var x m8run.Attempt
		if err = m8run.Read(p, &x); err != nil {
			return err
		}
		attempts = append(attempts, x)
	}
	return m8run.Write(root, "output/m8-generated-checksums.json", struct {
		Algorithm             string
		ProtectedUnchanged    bool
		ProtectedFiles, Files map[string]string
		Attempts              []m8run.Attempt
	}{"SHA-256", true, base.Files, files, attempts})
}

var _ abi.ABI
var _ fr.Element
