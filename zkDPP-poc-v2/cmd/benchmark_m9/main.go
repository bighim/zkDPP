package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/finalsrs"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m8audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m9audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m9run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type setupFile struct {
	Attempt int
	Result  finalsrs.RunResult
}
type setupReport struct {
	RunCount, Attempt, SelectedRun int
	MaxDomain, CanonicalPoints     int
	Runs                           []finalsrs.RunResult
	ThirdRunTriggers               []string
}
type circuitReport struct {
	RunCount         int
	Relations        int
	Runs             []finalsrs.RunResult
	ThirdRunTriggers []string
}
type environment struct {
	Go, OS, Arch           string
	GOMAXPROCS             int
	Foundry, Solidity, EVM string
	ChainID, BlockGasLimit uint64
	RPC                    string
}
type chainResult struct {
	Run                                            int
	Ledger, CodeHash                               string
	LedgerRuntimeBytes                             int
	Transactions                                   []m7run.Transaction
	NoteLeaves, VoucherLeaves, AuditRecords        uint64
	ProductDPP, WasteDPP                           string
	StandardActive, StrictRevoked, Product2Unspent bool
	RejectedChecks                                 int
}
type anvilReport struct {
	RunCount, Attempt int
	Environment       environment
	Runs              []chainResult
	GasIdentical      bool
	ThirdRunReason    string
}
type auditResult struct {
	Run      int
	Backward m8audit.Result
	Forward  m9audit.Result
	Expected bool
}
type finalAuditReport struct {
	RunCount, Attempt int
	Environment       environment
	Runs              []auditResult
	ThirdRunReason    string
}

func main() {
	mode := flag.String("mode", "", "setup|anvil|audit|checks")
	root := flag.String("root", ".", "project root")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var e error
	switch *mode {
	case "setup":
		e = setup(*root)
	case "anvil":
		e = anvil(*root)
	case "audit":
		e = auditRun(*root)
	case "checks":
		e = checks(*root)
	default:
		e = fmt.Errorf("unknown mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func rate(a, b float64) float64 {
	m := math.Min(a, b)
	if m == 0 {
		if math.Max(a, b) == 0 {
			return 0
		}
		return 1
	}
	return math.Abs(a-b) / m
}
func triggers(a, b finalsrs.RunResult) []string {
	out := []string{}
	if rate(a.CanonicalMillis, b.CanonicalMillis) > .2 {
		out = append(out, "canonical-srs")
	}
	for d, v := range a.LagrangeMillis {
		if rate(v, b.LagrangeMillis[d]) > .2 {
			out = append(out, fmt.Sprintf("lagrange-%d", d))
		}
	}
	bm := map[string]finalsrs.RelationResult{}
	for _, v := range b.Relations {
		bm[v.Name] = v
	}
	for _, v := range a.Relations {
		x := bm[v.Name]
		if rate(v.SetupMillis, x.SetupMillis) > .2 {
			out = append(out, v.Name+"/setup")
		}
		if rate(v.ProveMillis, x.ProveMillis) > .2 {
			out = append(out, v.Name+"/prove")
		}
		if rate(v.VerifyMillis, x.VerifyMillis) > .2 {
			out = append(out, v.Name+"/verify")
		}
	}
	return out
}
func setup(root string) (runErr error) {
	attempt, finish, e := m9run.Begin(root, "setup-benchmark", "output/m9-srs.json")
	if e != nil {
		return e
	}
	defer func() { finish(runErr) }()
	var first setupFile
	if e = m9run.Read(filepath.Join(root, "artifacts/development/m9/run-1.json"), &first); e != nil {
		return e
	}
	second, e := finalsrs.Run(root, 2, false)
	if e != nil {
		return e
	}
	runs := []finalsrs.RunResult{first.Result, second}
	why := triggers(first.Result, second)
	if len(why) > 0 {
		third, e := finalsrs.Run(root, 3, false)
		if e != nil {
			return e
		}
		runs = append(runs, third)
	}
	s := setupReport{len(runs), attempt, 1, finalsrs.MaxDomain, finalsrs.CanonicalPoints, runs, why}
	if e = m9run.Write(root, "output/m9-srs.json", s); e != nil {
		return e
	}
	return m9run.Write(root, "output/m9-circuit.json", circuitReport{len(runs), 10, runs, why})
}

const project = "zkdpp-m9"
const rpcURL = "http://127.0.0.1:18545"

func env() environment {
	return environment{runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), "1.7.1", "0.8.30", "Prague", 31337, 30000000, rpcURL}
}
func compose(root string, args ...string) error {
	all := append([]string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.m9.yml", "-p", project}, args...)
	c := exec.Command("docker", all...)
	c.Dir = root
	c.Env = append(os.Environ(), "ANVIL_PORT=18545", "FOUNDRY_IMAGE="+m9run.FoundryImage)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func start(root string) error {
	if c, e := net.DialTimeout("tcp", "127.0.0.1:18545", 300*time.Millisecond); e == nil {
		c.Close()
		return fmt.Errorf("port 18545 already in use")
	}
	if e := compose(root, "up", "-d", "anvil"); e != nil {
		return e
	}
	for i := 0; i < 60; i++ {
		c, e := ethclient.Dial(rpcURL)
		if e == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, e = c.BlockNumber(ctx)
			cancel()
			c.Close()
			if e == nil {
				return compose(root, "run", "--rm", "foundry", "forge", "build")
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("Anvil not ready")
}
func stop(root string) { _ = compose(root, "down", "--remove-orphans") }

func anvil(root string) (runErr error) {
	attempt, finish, e := m9run.Begin(root, "anvil", "output/m9-anvil.json")
	if e != nil {
		return e
	}
	defer func() { finish(runErr) }()
	report := anvilReport{RunCount: 2, Attempt: attempt, Environment: env()}
	for i := 1; i <= 2; i++ {
		r, _, e := runChain(root, i, false)
		if e != nil {
			return e
		}
		report.Runs = append(report.Runs, r)
	}
	report.GasIdentical = compareGas(report.Runs[0], report.Runs[1])
	if !report.GasIdentical {
		return fmt.Errorf("Anvil gas mismatch; inspect runs before retry")
	}
	return m9run.Write(root, "output/m9-anvil.json", report)
}
func auditRun(root string) (runErr error) {
	attempt, finish, e := m9run.Begin(root, "audit", "output/m9-audit.json")
	if e != nil {
		return e
	}
	defer func() { finish(runErr) }()
	report := finalAuditReport{RunCount: 2, Attempt: attempt, Environment: env()}
	for i := 1; i <= 2; i++ {
		_, a, e := runChain(root, i, true)
		if e != nil {
			return e
		}
		report.Runs = append(report.Runs, *a)
	}
	if rate(report.Runs[0].Backward.Metrics.TotalMillis, report.Runs[1].Backward.Metrics.TotalMillis) > .2 || rate(report.Runs[0].Forward.TotalMillis, report.Runs[1].Forward.TotalMillis) > .2 {
		_, a, e := runChain(root, 3, true)
		if e != nil {
			return e
		}
		report.Runs = append(report.Runs, *a)
		report.RunCount = 3
		report.ThirdRunReason = "audit time difference exceeded 20 percent"
	}
	return m9run.Write(root, "output/m9-audit.json", report)
}

func runChain(root string, run int, withAudit bool) (result chainResult, auditOut *auditResult, runErr error) {
	if e := start(root); e != nil {
		return result, nil, e
	}
	defer stop(root)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client, e := ethclient.Dial(rpcURL)
	if e != nil {
		return result, nil, e
	}
	defer client.Close()
	var accounts []common.Address
	if e = client.Client().CallContext(ctx, &accounts, "eth_accounts"); e != nil || len(accounts) < 5 {
		return result, nil, fmt.Errorf("accounts: %w", e)
	}
	fixture, e := m9run.LoadFixture(root)
	if e != nil {
		return result, nil, e
	}
	contractABI, _, size, e := m7run.Contract(root, "final/ZkDPPClaimLedger.sol", "ZkDPPClaimLedger")
	if e != nil {
		return result, nil, e
	}
	if size > 24576 {
		return result, nil, fmt.Errorf("EIP-170 %d", size)
	}
	txs := []m7run.Transaction{}
	send := func(from common.Address, to *common.Address, data []byte, name string) (*types.Receipt, error) {
		receipt, row, e := m7run.Send(ctx, client, from, to, data, name)
		if e == nil {
			row.SSTORECount, row.SSTOREOpcodeGas, _ = m7run.StoreTrace(ctx, client, row.Hash)
			txs = append(txs, row)
		}
		return receipt, e
	}
	deploy := func(source, name string, args ...any) (common.Address, error) {
		a, code, sz, e := m7run.Contract(root, source, name)
		if e != nil {
			return common.Address{}, e
		}
		if sz > 24576 {
			return common.Address{}, fmt.Errorf("%s size", name)
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
	hasher, e := deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if e != nil {
		return result, nil, e
	}
	names := []string{"M9AuditEntryVerifier", "M9AuditTransferVerifier", "M9AuditProceedVerifier", "M9AuditRecallVerifier", "M9AuditMergeVerifier", "M9AuditSplitVerifier", "M9ExitDPPVerifier", "M9AuditProcessVerifier", "M9IssueStandardVerifier", "M9IssueStrictVerifier"}
	addresses := make([]common.Address, 10)
	for i, n := range names {
		addresses[i], e = deploy(n+".sol", n)
		if e != nil {
			return result, nil, e
		}
	}
	var fixed [7]common.Address
	copy(fixed[:], addresses[:7])
	ledger, e := deploy("final/ZkDPPClaimLedger.sol", "ZkDPPClaimLedger", fixed, hasher, accounts[4])
	if e != nil {
		return result, nil, e
	}
	code, e := client.CodeAt(ctx, ledger, nil)
	if e != nil {
		return result, nil, e
	}
	head, e := client.HeaderByNumber(ctx, nil)
	if e != nil {
		return result, nil, e
	}
	deploymentBlock := head.Number.Uint64()
	call := func(from common.Address, name string, args ...any) error {
		data, e := contractABI.Pack(name, args...)
		if e != nil {
			return e
		}
		_, e = send(from, &ledger, data, name)
		return e
	}
	if e = call(accounts[0], "setEntryIssuer", accounts[1], true); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "setEntryIssuer", accounts[3], true); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "registerPolicyAuthority", accounts[0]); e != nil {
		return result, nil, e
	}
	process := fixture.Events[15]
	_, pp, e := process.Decode()
	if e != nil {
		return result, nil, e
	}
	processHash, e := vkHash(root, "audit-process-3-2")
	if e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "reservePolicy", uint8(6)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "registerPolicy", audit.Big(pp[0]), uint8(3), uint8(2), processHash, addresses[7]); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "setPolicyGrant", audit.Big(pp[0]), audit.Big(pp[1]), true); e != nil {
		return result, nil, e
	}
	_, sp, e := fixture.Standard.Decode()
	if e != nil {
		return result, nil, e
	}
	stdHash, e := vkHash(root, "issue-standard-v1")
	if e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "reservePolicy", uint8(8)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "registerPolicy", audit.Big(sp[0]), uint8(1), uint8(1), stdHash, addresses[8]); e != nil {
		return result, nil, e
	}
	_, tp, e := fixture.Strict.Decode()
	if e != nil {
		return result, nil, e
	}
	strictHash, e := vkHash(root, "issue-strict-v2")
	if e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "reservePolicyVersion", uint64(2)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[0], "registerPolicy", audit.Big(tp[0]), uint8(1), uint8(1), strictHash, addresses[9]); e != nil {
		return result, nil, e
	}
	rejected := func(from common.Address, data []byte) error {
		_, e := client.CallContract(ctx, ethereum.CallMsg{From: from, To: &ledger, Data: data}, nil)
		if e == nil {
			return fmt.Errorf("negative accepted")
		}
		result.RejectedChecks++
		return nil
	}
	status := func(t uint8, nf fr.Element, v uint8) error {
		return call(accounts[4], "setStatus", t, audit.Big(nf), v)
	}
	for i, ev := range fixture.Events {
		if ev.Epoch != 0 {
			if e = setEpoch(ctx, client, ev.Epoch); e != nil {
				return result, nil, e
			}
		}
		data, e := caseData(contractABI, ev)
		if e != nil {
			return result, nil, e
		}
		if i == 5 {
			nf, e := auditcrypto.DecodeField(fixture.FirstVoucherNF)
			if e != nil {
				return result, nil, e
			}
			if e = status(2, nf, 1); e != nil {
				return result, nil, e
			}
			if e = rejected(accounts[1], data); e != nil {
				return result, nil, e
			}
			if e = status(2, nf, 0); e != nil {
				return result, nil, e
			}
		}
		if i == 15 {
			nf, e := auditcrypto.DecodeField(fixture.ProcessInputNF)
			if e != nil {
				return result, nil, e
			}
			if e = status(1, nf, 1); e != nil {
				return result, nil, e
			}
			if e = rejected(accounts[2], data); e != nil {
				return result, nil, e
			}
			if e = status(1, nf, 0); e != nil {
				return result, nil, e
			}
		}
		from := accounts[2]
		if ev.EventKind == 0 {
			if i < 2 {
				from = accounts[1]
			} else {
				from = accounts[3]
			}
		}
		if _, e = send(from, &ledger, data, ev.Name); e != nil {
			return result, nil, e
		}
	}
	exit := func(f m9run.FixedProof) (fr.Element, error) {
		proof, p, e := f.Decode()
		if e != nil {
			return fr.Element{}, e
		}
		a := audit.CipherArg{R1X: audit.Big(p[3]), R1Y: audit.Big(p[4]), EncryptedParents: []*big.Int{audit.Big(p[5])}, EncryptedOutputNfs: []*big.Int{}}
		data, e := contractABI.Pack("exit", proof, audit.Big(p[0]), audit.Big(p[1]), audit.Big(p[2]), a)
		if e != nil {
			return fr.Element{}, e
		}
		if _, e = send(accounts[2], &ledger, data, f.Name); e != nil {
			return fr.Element{}, e
		}
		return p[2], nil
	}
	productDPP, e := exit(fixture.ProductExit)
	if e != nil {
		return result, nil, e
	}
	issue := func(f m9run.FixedProof) (fr.Element, error) {
		proof, p, e := f.Decode()
		if e != nil {
			return fr.Element{}, e
		}
		data, e := contractABI.Pack("issue", proof, audit.Big(p[0]), audit.Big(p[1]))
		if e != nil {
			return fr.Element{}, e
		}
		if _, e = send(accounts[2], &ledger, data, f.Name); e != nil {
			return fr.Element{}, e
		}
		return p[0], nil
	}
	stdRef, e := issue(fixture.Standard)
	if e != nil {
		return result, nil, e
	}
	strictRef, e := issue(fixture.Strict)
	if e != nil {
		return result, nil, e
	}
	wasteDPP, e := exit(fixture.WasteExit)
	if e != nil {
		return result, nil, e
	}
	stdProof, _, _ := fixture.Standard.Decode()
	bad, e := contractABI.Pack("issue", stdProof, audit.Big(stdRef), audit.Big(wasteDPP))
	if e != nil {
		return result, nil, e
	}
	if e = rejected(accounts[2], bad); e != nil {
		return result, nil, e
	}
	if e = call(accounts[4], "setClaimStatus", audit.Big(productDPP), audit.Big(stdRef), uint8(1)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[4], "setClaimStatus", audit.Big(productDPP), audit.Big(stdRef), uint8(0)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[4], "setClaimStatus", audit.Big(productDPP), audit.Big(strictRef), uint8(1)); e != nil {
		return result, nil, e
	}
	if e = call(accounts[4], "setClaimStatus", audit.Big(productDPP), audit.Big(strictRef), uint8(2)); e != nil {
		return result, nil, e
	}
	result.Run = run
	result.Ledger = ledger.Hex()
	result.CodeHash = audit.CodeHash(code)
	result.LedgerRuntimeBytes = size
	result.Transactions = txs
	result.NoteLeaves = fixture.FinalNoteCount
	result.VoucherLeaves = fixture.FinalVoucherCount
	result.AuditRecords = 21
	result.ProductDPP = productDPP.String()
	result.WasteDPP = wasteDPP.String()
	result.StandardActive = true
	result.StrictRevoked = true
	result.Product2Unspent = true
	if !withAudit {
		return result, nil, nil
	}
	head, e = client.HeaderByNumber(ctx, nil)
	if e != nil {
		return result, nil, e
	}
	snap := audit.Snapshot{Number: head.Number.Uint64(), Hash: head.Hash().Hex()}
	src := &audit.RPCSource{Client: client, ABI: contractABI, Deployment: audit.Deployment{ChainID: 31337, Ledger: ledger.Hex(), DeploymentBlock: deploymentBlock, LedgerCodeSHA256: audit.CodeHash(code), PublicKeyChecksum: fixture.PublicKeyChecksum}}
	committee, e := m9run.Committee(root)
	if e != nil {
		return result, nil, e
	}
	back, e := m8audit.TraceClaim(ctx, src, committee, productDPP, stdRef, snap)
	if e != nil {
		return result, nil, e
	}
	startRef, e := auditcrypto.DecodeField(fixture.EntryAluminumA)
	if e != nil {
		return result, nil, e
	}
	forward, e := m9audit.Forward(ctx, src, committee, audit.Ref{ObjectType: audit.Note, RawID: startRef}, snap)
	if e != nil {
		return result, nil, e
	}
	aout := &auditResult{Run: run, Backward: back, Forward: forward, Expected: len(back.Upstream.Entries) == 4 && len(forward.DPPs) >= 1 && len(forward.Leaves) >= 1}
	if !aout.Expected {
		return result, nil, fmt.Errorf("audit expected graph mismatch")
	}
	return result, aout, nil
}

func caseData(a abi.ABI, f m9run.FixedProof) ([]byte, error) {
	proof, p, e := f.Decode()
	if e != nil {
		return nil, e
	}
	k := f.EventKind
	b := len(p)
	parents := 0
	outputs := 1
	if k != 0 {
		parents = 1
	}
	if k == 4 {
		parents = 2
	}
	if k == 6 {
		parents = 3
	}
	if k == 1 || k == 5 || k == 6 {
		outputs = 2
	}
	base := b - 2 - parents - outputs
	arg := audit.CipherArg{R1X: audit.Big(p[base]), R1Y: audit.Big(p[base+1]), EncryptedParents: []*big.Int{}, EncryptedOutputNfs: []*big.Int{}}
	for i := 0; i < parents; i++ {
		arg.EncryptedParents = append(arg.EncryptedParents, audit.Big(p[base+2+i]))
	}
	for i := 0; i < outputs; i++ {
		arg.EncryptedOutputNfs = append(arg.EncryptedOutputNfs, audit.Big(p[base+2+parents+i]))
	}
	args := []any{proof}
	if k == 6 {
		args = append(args, audit.Big(p[0]), audit.Big(p[1]), audit.Big(p[2]), [3]*big.Int{audit.Big(p[3]), audit.Big(p[4]), audit.Big(p[5])}, [2]*big.Int{audit.Big(p[6]), audit.Big(p[7])})
	} else {
		for _, v := range p[:base] {
			args = append(args, audit.Big(v))
		}
	}
	args = append(args, arg)
	names := []string{"entry", "transfer", "proceed", "recall", "merge", "split", "process"}
	return a.Pack(names[k], args...)
}
func setEpoch(ctx context.Context, c *ethclient.Client, epoch uint64) error {
	head, e := c.HeaderByNumber(ctx, nil)
	if e != nil {
		return e
	}
	next := epoch * 600
	if next <= head.Time {
		next = head.Time + 1
	}
	var out any
	return c.Client().CallContext(ctx, &out, "evm_setNextBlockTimestamp", next)
}
func vkHash(root, name string) ([32]byte, error) {
	var out [32]byte
	l, e := finalsrs.Load(root, name)
	if e != nil {
		return out, e
	}
	vk := l.VK.(*plonkbls.VerifyingKey)
	o, e := solgen.ExtractOptimizedVK(vk)
	if e != nil {
		return out, e
	}
	b, e := hex.DecodeString(o.SHA256)
	if e != nil {
		return out, e
	}
	copy(out[:], b)
	return out, nil
}
func compareGas(a, b chainResult) bool {
	if len(a.Transactions) != len(b.Transactions) {
		return false
	}
	for i := range a.Transactions {
		if a.Transactions[i].Name != b.Transactions[i].Name || a.Transactions[i].GasUsed != b.Transactions[i].GasUsed || a.Transactions[i].CalldataBytes != b.Transactions[i].CalldataBytes {
			return false
		}
	}
	return a.CodeHash == b.CodeHash
}
func checks(root string) error {
	for _, n := range []string{"m9-srs.json", "m9-circuit.json", "m9-anvil.json", "m9-audit.json"} {
		var h struct{ RunCount int }
		if e := m9run.Read(filepath.Join(root, "output", n), &h); e != nil {
			return e
		}
		if h.RunCount < 2 || h.RunCount > 3 {
			return fmt.Errorf("run count %s", n)
		}
	}
	var base struct{ Files map[string]string }
	if e := m9run.Read(filepath.Join(root, "artifacts/development/m9/baseline.json"), &base); e != nil {
		return e
	}
	for p, want := range base.Files {
		got, e := artifact.Checksum(filepath.Join(root, p))
		if e != nil {
			return e
		}
		if got != want {
			return fmt.Errorf("protected file changed: %s", p)
		}
	}
	var s setupReport
	if e := m9run.Read(filepath.Join(root, "output/m9-srs.json"), &s); e != nil {
		return e
	}
	if s.MaxDomain != finalsrs.MaxDomain || s.CanonicalPoints != finalsrs.CanonicalPoints || len(s.Runs) < 2 || len(s.Runs) > 3 {
		return fmt.Errorf("SRS report")
	}
	if len(s.Runs[0].Relations) != 10 {
		return fmt.Errorf("relation count")
	}
	universal := filepath.Join(root, "artifacts/final/srs/universal-canonical.bin")
	sum, e := artifact.Checksum(universal)
	if e != nil {
		return e
	}
	if sum != s.Runs[0].CanonicalChecksum {
		return fmt.Errorf("universal checksum")
	}
	for _, r := range s.Runs[0].Relations {
		l, e := finalsrs.Load(root, r.Name)
		if e != nil {
			return e
		}
		if l.Manifest.UniversalSRSChecksum != sum || l.Manifest.DomainSize != r.DomainSize || len(l.Manifest.PublicNames) != l.Manifest.PublicInputs {
			return fmt.Errorf("manifest %s", r.Name)
		}
	}
	var ar anvilReport
	if e = m9run.Read(filepath.Join(root, "output/m9-anvil.json"), &ar); e != nil {
		return e
	}
	if len(ar.Runs) != 2 || !ar.GasIdentical || ar.Runs[0].LedgerRuntimeBytes > 24576 || ar.Runs[0].AuditRecords != 21 {
		return fmt.Errorf("Anvil report")
	}
	var au finalAuditReport
	if e = m9run.Read(filepath.Join(root, "output/m9-audit.json"), &au); e != nil {
		return e
	}
	if len(au.Runs) < 2 || len(au.Runs) > 3 {
		return fmt.Errorf("audit runs")
	}
	for _, r := range au.Runs {
		if !r.Expected || len(r.Backward.Upstream.Entries) != 4 || len(r.Forward.DPPs) < 1 {
			return fmt.Errorf("audit graph")
		}
	}
	policyHashes := map[string]string{}
	for _, name := range []string{"audit-process-3-2", "issue-standard-v1", "issue-strict-v2"} {
		l, e := finalsrs.Load(root, name)
		if e != nil {
			return e
		}
		optimized, e := solgen.ExtractOptimizedVK(l.VK.(*plonkbls.VerifyingKey))
		if e != nil {
			return e
		}
		if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/circuits", name, "optimized-vk.json"), optimized); e != nil {
			return e
		}
		policyHashes[name] = optimized.SHA256
	}
	var srsManifest map[string]any
	if e = m9run.Read(filepath.Join(root, "artifacts/final/srs/manifest.json"), &srsManifest); e != nil {
		return e
	}
	generation := []map[string]any{}
	for _, run := range s.Runs {
		generation = append(generation, map[string]any{"run": run.Run, "persisted": run.Persisted, "canonicalMillis": run.CanonicalMillis, "canonicalAllocatedBytes": run.CanonicalAllocatedBytes})
	}
	srsManifest["generationRuns"] = generation
	delete(srsManifest,"selectedRun")
	if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/srs/manifest.json"), srsManifest); e != nil {
		return e
	}
	wrappers := map[string][2]string{"audit-entry": {"M9AuditEntryVerifier.sol", "M9AuditEntryVerifier"}, "audit-transfer": {"M9AuditTransferVerifier.sol", "M9AuditTransferVerifier"}, "audit-proceed": {"M9AuditProceedVerifier.sol", "M9AuditProceedVerifier"}, "audit-recall": {"M9AuditRecallVerifier.sol", "M9AuditRecallVerifier"}, "audit-merge": {"M9AuditMergeVerifier.sol", "M9AuditMergeVerifier"}, "audit-split": {"M9AuditSplitVerifier.sol", "M9AuditSplitVerifier"}, "exit-dpp": {"M9ExitDPPVerifier.sol", "M9ExitDPPVerifier"}, "audit-process-3-2": {"M9AuditProcessVerifier.sol", "M9AuditProcessVerifier"}, "issue-standard-v1": {"M9IssueStandardVerifier.sol", "M9IssueStandardVerifier"}, "issue-strict-v2": {"M9IssueStrictVerifier.sol", "M9IssueStrictVerifier"}}
	for relation, w := range wrappers {
		sourcePath := filepath.Join(root, "contracts/src", w[0])
		sourceHash, e := artifact.Checksum(sourcePath)
		if e != nil {
			return e
		}
		var compiled struct{ DeployedBytecode struct{ Object string } }
		if e = m9run.Read(filepath.Join(root, "contracts/out", w[0], w[1]+".json"), &compiled); e != nil {
			return e
		}
		code, e := hex.DecodeString(strings.TrimPrefix(compiled.DeployedBytecode.Object, "0x"))
		if e != nil {
			return e
		}
		var manifest map[string]any
		if e = m9run.Read(filepath.Join(root, "artifacts/final/circuits", relation, "manifest.json"), &manifest); e != nil {
			return e
		}
		manifest["verifierSource"] = w[0]
		manifest["verifierSourceChecksum"] = sourceHash
		manifest["verifierRuntimeCodeHash"] = audit.CodeHash(code)
		if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/circuits", relation, "manifest.json"), manifest); e != nil {
			return e
		}
	}
	files := map[string]string{}
	roots := []string{"artifacts/final", "internal/finalsrs", "internal/m9run", "internal/m9case", "internal/m9audit", "cmd/setup_m9_final", "cmd/evaluate_m9", "cmd/benchmark_m9"}
	for _, dir := range roots {
		e := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			h, e := artifact.Checksum(p)
			if e == nil {
				files[filepath.ToSlash(rel)] = h
			}
			return e
		})
		if e != nil {
			return e
		}
	}
	for _, pattern := range []string{"contracts/src/final/*.sol", "contracts/src/M9*Verifier.sol", "contracts/src/generated/m9/*/*", "contracts/test/M9*.sol", "contracts/test/fixtures/m9-*.json", "docker-compose.m9.yml", "milestones/M9-*.md", "output/m9-*.json"} {
		paths, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, p := range paths {
			if strings.HasSuffix(p, "m9-generated-checksums.json") {
				continue
			}
			rel, _ := filepath.Rel(root, p)
			h, e := artifact.Checksum(p)
			if e != nil {
				return e
			}
			files[filepath.ToSlash(rel)] = h
		}
	}
	if e = artifact.WriteJSON(filepath.Join(root, "artifacts/final/verifiers/checksums.json"), struct {
		Algorithm             string
		PolicyVKHashes, Files map[string]string
	}{"SHA-256", policyHashes, files}); e != nil {
		return e
	}
	attempts := []m9run.Attempt{}
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts/development/m9/attempts/*.json"))
	for _, p := range paths {
		var a m9run.Attempt
		if e = m9run.Read(p, &a); e != nil {
			return e
		}
		attempts = append(attempts, a)
	}
	return m9run.Write(root, "output/m9-generated-checksums.json", struct {
		Algorithm             string
		ProtectedUnchanged    bool
		ProtectedFiles, Files map[string]string
		Attempts              []m9run.Attempt
	}{"SHA-256", true, base.Files, files, attempts})
}
