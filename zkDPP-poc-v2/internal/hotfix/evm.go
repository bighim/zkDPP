package hotfix

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2dpp"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	bls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Tx struct {
	Name, Hash, BlockHash, ContractAddress string
	Block, Gas, Status                     uint64
	CalldataBytes                          int
	Millis                                 float64
	SSTORECount, SSTOREGas                 uint64
}
type Deployment struct {
	Name, Address, CodeHash string
	RuntimeBytes            int
	ArtifactHash            string
}
type EVMReport struct {
	Mode          string
	MockVerifiers bool
	ChainID       int
	Transactions  []Tx
	Deployments   []Deployment
	Groups        []map[string]any
	Audits        []map[string]any
	FixtureHash   string
}
type EVM struct {
	root         string
	ctx          context.Context
	c            *ethclient.Client
	from, ledger common.Address
	a            abi.ABI
	report       *EVMReport
	source       *v2audit.RPCSource
}

var relations = []string{"entry", "transfer", "proceed", "recall", "merge", "split", "exit", "process", "issue-standard-v1", "issue-strict-v2"}
var wrappers = []string{"HFEntry", "HFTransfer", "HFProceed", "HFRecall", "HFMerge", "HFSplit", "HFExit", "HFProcess", "HFStandard", "HFStrict"}

func contract(root, source, name string) (abi.ABI, []byte, string, error) {
	p := filepath.Join(root, "contracts/hotfix/out", source, name+".json")
	var raw struct {
		ABI      json.RawMessage
		Bytecode struct{ Object string }
	}
	if e := Read(p, &raw); e != nil {
		return abi.ABI{}, nil, "", e
	}
	a, e := abi.JSON(bytes.NewReader(raw.ABI))
	if e != nil {
		return a, nil, "", e
	}
	code, e := hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x"))
	hash, _ := Hash(p)
	return a, code, hash, e
}
func (e *EVM) send(name string, to *common.Address, data []byte) (common.Address, error) {
	start := time.Now()
	receipt, row, err := m7run.Send(e.ctx, e.c, e.from, to, data, name)
	if err != nil {
		return common.Address{}, err
	}
	count, gas, err := m7run.StoreTrace(e.ctx, e.c, row.Hash)
	if err != nil {
		return common.Address{}, err
	}
	e.report.Transactions = append(e.report.Transactions, Tx{Name: name, Hash: row.Hash, BlockHash: receipt.BlockHash.Hex(), ContractAddress: receipt.ContractAddress.Hex(), Block: receipt.BlockNumber.Uint64(), Gas: receipt.GasUsed, Status: receipt.Status, CalldataBytes: len(data), Millis: float64(time.Since(start).Nanoseconds()) / 1e6, SSTORECount: count, SSTOREGas: gas})
	return receipt.ContractAddress, nil
}
func (e *EVM) deploy(source, name string, args ...any) (common.Address, abi.ABI, error) {
	a, code, hash, err := contract(e.root, source, name)
	if err != nil {
		return common.Address{}, a, err
	}
	arg, err := a.Constructor.Inputs.Pack(args...)
	if err != nil {
		return common.Address{}, a, err
	}
	addr, err := e.send("deploy-"+name, nil, append(code, arg...))
	if err != nil {
		return addr, a, err
	}
	runtime, err := e.c.CodeAt(e.ctx, addr, nil)
	if err != nil {
		return addr, a, err
	}
	if len(runtime) > 24576 {
		return addr, a, fmt.Errorf("EIP-170 %s: %d", name, len(runtime))
	}
	e.report.Deployments = append(e.report.Deployments, Deployment{name, addr.Hex(), crypto.Keccak256Hash(runtime).Hex(), len(runtime), hash})
	return addr, a, nil
}
func (e *EVM) call(name string, args ...any) error {
	data, err := e.a.Pack(name, args...)
	if err != nil {
		return err
	}
	_, err = e.send(name, &e.ledger, data)
	return err
}
func (e *EVM) view(name string, args ...any) ([]any, error) {
	return m7run.View(e.ctx, e.c, e.a, e.ledger, name, args...)
}
func (e *EVM) initialize() error {
	hasher, _, err := e.deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if err != nil {
		return err
	}
	addresses := make([]common.Address, 10)
	for i, n := range wrappers {
		addresses[i], _, err = e.deploy(n+".sol", n)
		if err != nil {
			return err
		}
	}
	fixed := [7]common.Address{}
	copy(fixed[:], addresses[:7])
	e.ledger, e.a, err = e.deploy("ZkDPPV2Ledger.sol", "ZkDPPV2Ledger", fixed, hasher, e.from)
	if err != nil {
		return err
	}
	dep := e.report.Transactions[len(e.report.Transactions)-1]
	code := e.report.Deployments[len(e.report.Deployments)-1]
	e.source = v2audit.NewRPCSource(e.c, e.a, e.ledger, e.from)
	e.source.Context = v2audit.LedgerContext{ChainID: 31337, CodeHash: code.CodeHash, DeploymentBlock: dep.Block}
	if err = e.call("setEntryIssuer", e.from, true); err != nil {
		return err
	}
	if err = e.call("registerPolicyAuthority", e.from); err != nil {
		return err
	}
	refs := []fr.Element{policy.CanonicalRef(), issuepolicy.Standard().PolicyRef, issuepolicy.Strict().PolicyRef}
	for i, r := range refs {
		if i < 2 {
			kind := uint8(6)
			if i == 1 {
				kind = 8
			}
			if err = e.call("reservePolicy", kind); err != nil {
				return err
			}
		} else {
			if err = e.call("reservePolicyVersion", uint64(2)); err != nil {
				return err
			}
		}
		f, err := os.Open(filepath.Join(e.root, Dir, "circuits", relations[7+i], "verifying.key"))
		if err != nil {
			return err
		}
		var vk bls.VerifyingKey
		_, err = vk.ReadFrom(f)
		f.Close()
		if err != nil {
			return err
		}
		optimized, err := solgen.ExtractOptimizedVK(&vk)
		if err != nil {
			return err
		}
		vb, _ := hex.DecodeString(optimized.SHA256)
		var hash [32]byte
		copy(hash[:], vb)
		in, out := uint8(1), uint8(1)
		if i == 0 {
			in, out = 3, 2
		}
		if err = e.call("registerPolicy", v2audit.Big(r), in, out, hash, addresses[7+i]); err != nil {
			return err
		}
	}
	return nil
}
func eventData(a abi.ABI, event ProofEvent) ([]byte, error) {
	proof, err := hexutil.Decode(event.Proof)
	if err != nil {
		return nil, err
	}
	p := make([]fr.Element, len(event.Public))
	for i, v := range event.Public {
		p[i], err = auditcrypto.DecodeField(v)
		if err != nil {
			return nil, err
		}
	}
	layout, err := v2audit.Shape(event.Kind)
	if err != nil {
		return nil, err
	}
	if len(p) != layout.BaseCount+2+layout.ParentCount+len(layout.FutureSpendPositions) {
		return nil, fmt.Errorf("public length")
	}
	args := []any{proof}
	if event.Kind == v2audit.Process {
		args = append(args, v2audit.Big(p[0]), v2audit.Big(p[1]), v2audit.Big(p[2]), [3]*big.Int{v2audit.Big(p[3]), v2audit.Big(p[4]), v2audit.Big(p[5])}, [2]*big.Int{v2audit.Big(p[6]), v2audit.Big(p[7])})
	} else {
		for _, v := range p[:layout.BaseCount] {
			args = append(args, v2audit.Big(v))
		}
	}
	cipher := v2audit.CipherABI{R1X: v2audit.Big(p[layout.BaseCount]), R1Y: v2audit.Big(p[layout.BaseCount+1])}
	for _, v := range p[layout.BaseCount+2 : layout.BaseCount+2+layout.ParentCount] {
		cipher.EncryptedParents = append(cipher.EncryptedParents, v2audit.Big(v))
	}
	for _, v := range p[layout.BaseCount+2+layout.ParentCount:] {
		cipher.EncryptedOutputNfs = append(cipher.EncryptedOutputNfs, v2audit.Big(v))
	}
	args = append(args, cipher)
	return a.Pack(event.Kind.String(), args...)
}
func (e *EVM) execute(ev ProofEvent) error {
	if ev.Kind == v2audit.Process {
		scope, err := auditcrypto.DecodeField(ev.Public[1])
		if err != nil {
			return err
		}
		if err = e.call("setPolicyGrant", v2audit.Big(policy.CanonicalRef()), v2audit.Big(scope), true); err != nil {
			return err
		}
	}
	data, err := eventData(e.a, ev)
	if err != nil {
		return err
	}
	_, err = e.send(ev.Name, &e.ledger, data)
	return err
}
func (e *EVM) group(g ProofGroup) error {
	for _, ev := range g.Events {
		if err := e.execute(ev); err != nil {
			return fmt.Errorf("%s: %w", ev.Name, err)
		}
	}
	for _, row := range []struct {
		name string
		want *big.Int
	}{{"currentNoteRoot", v2audit.Big(g.NoteRoot)}, {"currentVoucherRoot", v2audit.Big(g.VoucherRoot)}, {"noteLeafCount", new(big.Int).SetUint64(g.NoteCount)}, {"voucherLeafCount", new(big.Int).SetUint64(g.VoucherCount)}, {"nextAuditRecordId", big.NewInt(int64(len(g.Events) + 1))}} {
		v, err := e.view(row.name)
		if err != nil {
			return err
		}
		if v[0].(*big.Int).Cmp(row.want) != 0 {
			return fmt.Errorf("%s mismatch", row.name)
		}
	}
	snap, err := e.source.Checkpoint(e.ctx)
	if err != nil {
		return err
	}
	for i, ev := range g.Events {
		r, err := e.source.Record(e.ctx, uint64(i+1), snap)
		if err != nil {
			return err
		}
		if !v2audit.Equal(r.Ciphertext().Data, ev.Record.Ciphertext().Data) {
			return fmt.Errorf("record plaintext boundary")
		}
		for _, ref := range r.OutputRefs {
			id, err := e.source.Producer(e.ctx, ref, snap)
			if err != nil || id != uint64(i+1) {
				return fmt.Errorf("producer mismatch")
			}
		}
	}
	e.report.Groups = append(e.report.Groups, map[string]any{"name": g.Name, "events": len(g.Events), "rootsAndRecordsMatched": true})
	return nil
}
func RunEVM(root, mode string) error {
	target := filepath.Join(root, "output/m1-hotfix-"+mode+".json")
	if _, e := os.Stat(target); e == nil {
		return fmt.Errorf("output exists %s", target)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:18546")
	if e != nil {
		return fmt.Errorf("port 18546 unavailable: %w", e)
	}
	listener.Close()
	args := []string{"compose", "-f", "docker-compose.hotfix.yml", "-p", "zkdpp-v2-m1-hotfix"}
	docker := func(extra ...string) error {
		cmd := exec.Command("docker", append(append([]string{}, args...), extra...)...)
		cmd.Dir = root
		out, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("docker: %w: %s", e, out)
		}
		return nil
	}
	if e = docker("up", "-d", "anvil"); e != nil {
		return e
	}
	defer docker("down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, e := ethclient.DialContext(ctx, "http://127.0.0.1:18546")
	if e != nil {
		return e
	}
	defer c.Close()
	var accounts []common.Address
	for i := 0; i < 100; i++ {
		e = c.Client().CallContext(ctx, &accounts, "eth_accounts")
		if e == nil && len(accounts) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if e != nil || len(accounts) == 0 {
		return fmt.Errorf("Anvil unavailable")
	}
	var fixture ProofFile
	if e = Read(filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json"), &fixture); e != nil {
		return e
	}
	fh, _ := Hash(filepath.Join(root, "contracts/test/fixtures/v2-m1-hotfix-proofs.json"))
	report := EVMReport{Mode: mode, ChainID: 31337, FixtureHash: fh}
	pkg, shares, e := v2case.KeyFixture()
	if e != nil {
		return e
	}
	release := make([]auditcrypto.ReleasedShare, 2)
	for i := range release {
		release[i] = auditcrypto.ReleasedShare{Profile: pkg.Profile, SessionID: pkg.SessionID, PackageChecksum: auditcrypto.PackageChecksum(pkg), Share: shares[i]}
	}
	defer func() {
		for i := range shares {
			shares[i].Value.SetInt64(0)
		}
	}()
	for _, g := range fixture.Groups {
		evm := &EVM{root: root, ctx: ctx, c: c, from: accounts[0], report: &report}
		if e = evm.initialize(); e != nil {
			return e
		}
		if e = evm.group(g); e != nil {
			return e
		}
		if mode == "audit" {
			master, e := auditcrypto.RecoverReleased(pkg, release)
			if e != nil {
				return e
			}
			defer master.SetInt64(0)
			evm.source.Calls = 0
			snap, e := evm.source.Checkpoint(ctx)
			if e != nil {
				return e
			}
			start := time.Now()
			forward, e := v2audit.TraceForward(ctx, evm.source, master, g.Start, snap)
			if e != nil {
				return e
			}
			report.Audits = append(report.Audits, map[string]any{"group": g.Name, "direction": "forward", "trace": forward, "millis": float64(time.Since(start).Nanoseconds()) / 1e6, "rpcCalls": evm.source.Calls})
			if g.Name == "lifecycle" {
				if len(forward.Claims) != 2 || len(forward.Exits) != 1 {
					return fmt.Errorf("missing lifecycle terminals")
				}
				for _, claim := range g.ClaimValues {
					dpp := v2dpp.DPP{ProductName: g.Document.ProductName, LotID: g.Document.LotID, Unit: g.Document.Unit, Claim: claim}
					if e = v2dpp.Verify(ctx, evm.source, snap, dpp); e != nil {
						return e
					}
				}
				start = time.Now()
				evm.source.Calls = 0
				back, e := v2audit.TraceBackward(ctx, evm.source, master, v2audit.Ref{ObjectType: 3, RawID: g.ClaimValues[0].Handle}, snap)
				if e != nil {
					return e
				}
				if len(back.Entries) != 4 {
					return fmt.Errorf("backward entries %d", len(back.Entries))
				}
				report.Audits = append(report.Audits, map[string]any{"group": g.Name, "direction": "backward", "trace": back, "millis": float64(time.Since(start).Nanoseconds()) / 1e6, "rpcCalls": evm.source.Calls})
			} else {
				start = time.Now()
				evm.source.Calls = 0
				result := v2audit.AuditAndFreeze(ctx, evm.source, master, g.Start, snap)
				if result.Outcome != v2audit.CompleteAtCheckpoint || len(result.Targets) != 3 {
					return fmt.Errorf("freeze result %s %s", result.Outcome, result.Reason)
				}
				report.Audits = append(report.Audits, map[string]any{"group": g.Name, "result": result, "millis": float64(time.Since(start).Nanoseconds()) / 1e6, "transactions": evm.source.FreezeTransactions, "rpcCalls": evm.source.Calls})
				for _, tx := range evm.source.FreezeTransactions {
					n, gas, e := m7run.StoreTrace(ctx, c, tx.Hash)
					if e != nil {
						return e
					}
					report.Transactions = append(report.Transactions, Tx{Name: "freeze", Hash: tx.Hash, Block: tx.BlockNumber, BlockHash: tx.BlockHash, Gas: tx.GasUsed, Status: 1, CalldataBytes: tx.CalldataBytes, SSTORECount: n, SSTOREGas: gas})
				}
				// Keep one target terminally revoked; an already-blocked frontier is complete.
				eNF, e := auditcrypto.DecodeField(g.Extra[2].Public[1])
				if e != nil {
					return e
				}
				var revoked v2audit.Frontier
				for _, t := range result.Targets {
					if t.SpendValue == eNF.Bytes() {
						revoked = t
					}
				}
				if revoked.Ref.ObjectType == 0 {
					return fmt.Errorf("missing E target")
				}
				if e = evm.call("setStatus", revoked.Ref.ObjectType, new(big.Int).SetBytes(revoked.SpendValue[:]), uint8(2)); e != nil {
					return e
				}
				mixedSnap, e := evm.source.Checkpoint(ctx)
				if e != nil {
					return e
				}
				txBefore := len(evm.source.FreezeTransactions)
				mixed := v2audit.AuditAndFreeze(ctx, evm.source, master, g.Start, mixedSnap)
				if mixed.Outcome != v2audit.CompleteAtCheckpoint || len(evm.source.FreezeTransactions) != txBefore {
					return fmt.Errorf("mixed blocked frontier")
				}
				report.Audits = append(report.Audits, map[string]any{"group": "frozen-revoked", "result": mixed})
				for _, t := range result.Targets {
					if t.Ref.Key() == revoked.Ref.Key() {
						continue
					}
					if e = evm.call("setStatus", t.Ref.ObjectType, new(big.Int).SetBytes(t.SpendValue[:]), uint8(0)); e != nil {
						return e
					}
				}
				// Consume C after the snapshot. Audit must keep C as a failed target.
				raceSnap, e := evm.source.Checkpoint(ctx)
				if e != nil {
					return e
				}
				if e = evm.execute(g.Extra[0]); e != nil {
					return e
				}
				race := v2audit.AuditAndFreeze(ctx, evm.source, master, g.Start, raceSnap)
				if race.Outcome != v2audit.Incomplete {
					return fmt.Errorf("race falsely complete")
				}
				report.Audits = append(report.Audits, map[string]any{"group": "post-snapshot-race", "result": race})
				// A wrong configured chain must fail before any Freeze submission.
				evm.source.Context.ChainID = 1
				before := len(evm.source.FreezeTransactions)
				failed := v2audit.AuditAndFreeze(ctx, evm.source, master, g.Start, raceSnap)
				evm.source.Context.ChainID = 31337
				if failed.Outcome != v2audit.TraceFailed || len(evm.source.FreezeTransactions) != before {
					return fmt.Errorf("trace failure submitted Freeze")
				}
				report.Audits = append(report.Audits, map[string]any{"group": "wrong-chain", "result": failed})
			}
			master.SetInt64(0)
		}
	}
	return Write(target, report)
}
