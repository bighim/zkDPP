package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	b1 "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m6b1run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/solgen"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	plonkbls "github.com/consensys/gnark/backend/plonk/bls12-381"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const project = "zkdpp-m7"
const rpcURL = "http://127.0.0.1:18545"

type AuditRow struct {
	Group, Direction                         string
	Snapshot                                 audit.Snapshot
	Metrics                                  audit.Metrics
	RPCRequests, Leaves, Entries, Terminated int
	Relationships                            int
	Expected                                 bool
}
type GroupResult struct {
	Name                                               string
	Ledger                                             string
	NoteCount, VoucherCount, Records                   uint64
	RootsAndPathsMatch, OriginalsRestored, StatusGates bool
}
type report struct {
	RunCount, Attempt                       int
	Mode                                    string
	Environment                             b1.Environment
	PublicKeyChecksum, ClientVersion, Image string
	ChainID, BlockGasLimit                  uint64
	LedgerRuntimeBytes                      int
	Transactions                            []m7run.Transaction
	Audits                                  []AuditRow
	Groups                                  []GroupResult
	OriginalRestores, NegativeChecks        int
}

func main() {
	root := flag.String("root", ".", "project root")
	mode := flag.String("mode", "gas", "gas, audit, checks")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var e error
	if *mode == "checks" {
		e = m7run.FinalChecks(*root)
	} else {
		e = run(*root, *mode)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func compose(root string, args ...string) error {
	c := exec.Command("docker", append([]string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.m7.yml", "-p", project}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), "ANVIL_PORT=18545", "FOUNDRY_IMAGE="+m7run.FoundryImage)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func run(root, mode string) (err error) {
	if mode != "gas" && mode != "audit" {
		return fmt.Errorf("unknown mode")
	}
	output := "output/m7-anvil-gas.json"
	if mode == "audit" {
		output = "output/m7-audit.json"
	}
	attempt, finish, e := m7run.Begin(root, mode, output)
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, e := m7run.Committee(root)
	if e != nil {
		return e
	}
	fixture, e := m7run.LoadFixture(root)
	if e != nil {
		return e
	}
	oracle, e := m7case.Build(root, committee.PK)
	if e != nil {
		return e
	}
	if conn, e := net.DialTimeout("tcp", "127.0.0.1:18545", 300*time.Millisecond); e == nil {
		conn.Close()
		return fmt.Errorf("port occupied; refusing to stop another service")
	}
	probe := exec.Command("docker", "compose", "-p", project, "ps", "-q")
	probe.Dir = root
	existing, e := probe.CombinedOutput()
	if e != nil {
		return fmt.Errorf("project inventory: %w: %s", e, existing)
	}
	if strings.TrimSpace(string(existing)) != "" {
		return fmt.Errorf("existing M7 containers; refusing replacement")
	}
	if e = compose(root, "up", "-d", "anvil"); e != nil {
		return e
	}
	defer func() {
		if e := compose(root, "down"); e != nil && err == nil {
			err = e
		}
	}()
	if e = compose(root, "run", "--rm", "foundry", "forge", "build"); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var client *ethclient.Client
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); {
		c, e := ethclient.Dial(rpcURL)
		if e == nil {
			if _, e = c.BlockNumber(ctx); e == nil {
				client = c
				break
			}
			c.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	if client == nil {
		return fmt.Errorf("Anvil not ready")
	}
	defer client.Close()
	var accounts []common.Address
	if e = client.Client().CallContext(ctx, &accounts, "eth_accounts"); e != nil {
		return e
	}
	if len(accounts) < 5 {
		return fmt.Errorf("Anvil account inventory")
	}
	var version string
	if e = client.Client().CallContext(ctx, &version, "web3_clientVersion"); e != nil {
		return e
	}
	if !strings.Contains(version, "1.7.1") {
		return fmt.Errorf("unexpected Anvil version %s", version)
	}
	chain, e := client.ChainID(ctx)
	if e != nil {
		return e
	}
	head, e := client.HeaderByNumber(ctx, nil)
	if e != nil {
		return e
	}
	if chain.Uint64() != 31337 || head.GasLimit != 30000000 {
		return fmt.Errorf("chain profile mismatch")
	}
	rep := report{RunCount: 1, Attempt: attempt, Mode: mode, Environment: b1.Env(), PublicKeyChecksum: committee.Public.Checksum, ClientVersion: version, Image: m7run.FoundryImage, ChainID: 31337, BlockGasLimit: 30000000}
	send := func(from common.Address, to *common.Address, data []byte, name string) (m7run.Transaction, error) {
		receipt, row, e := m7run.Send(ctx, client, from, to, data, name)
		if e != nil {
			return row, e
		}
		if to != nil {
			row.ContractAddress = to.Hex()
		}
		if mode == "gas" {
			row.SSTORECount, row.SSTOREOpcodeGas, e = m7run.StoreTrace(ctx, client, row.Hash)
			if e != nil {
				return row, e
			}
		}
		_ = receipt
		rep.Transactions = append(rep.Transactions, row)
		if e = m7run.Write(root, fmt.Sprintf("artifacts/development/m7/runs/%s-%02d-transactions.json", mode, attempt), rep.Transactions); e != nil {
			return row, e
		}
		return row, nil
	}
	deploy := func(source, name string, args ...any) (common.Address, m7run.Transaction, error) {
		a, code, size, e := m7run.Contract(root, source, name)
		if e != nil {
			return common.Address{}, m7run.Transaction{}, e
		}
		if size > 24576 {
			return common.Address{}, m7run.Transaction{}, fmt.Errorf("%s runtime %d exceeds EIP-170", name, size)
		}
		packed, e := a.Pack("", args...)
		if e != nil {
			return common.Address{}, m7run.Transaction{}, e
		}
		row, e := send(accounts[0], nil, append(code, packed...), name+"/deployment")
		return common.HexToAddress(row.ContractAddress), row, e
	}
	hasher, _, e := deploy("Poseidon2BLS12381.sol", "Poseidon2BLS12381")
	if e != nil {
		return e
	}
	names := []string{"M7EntryVerifier", "M7TransferVerifier", "M7ProceedVerifier", "M7RecallVerifier", "M7MergeVerifier", "M7SplitVerifier", "AuditProcessVerifier", "M7ExitVerifier"}
	var verifiers [8]common.Address
	for i, name := range names {
		v, _, e := deploy(name+".sol", name)
		if e != nil {
			return e
		}
		verifiers[i] = v
	}
	a, _, size, e := m7run.Contract(root, "ZkDPPAuditLedger.sol", "ZkDPPAuditLedger")
	if e != nil {
		return e
	}
	rep.LedgerRuntimeBytes = size
	loaded, e := m7run.Load(root, audit.Process)
	if e != nil {
		return e
	}
	optimized, e := solgen.ExtractOptimizedVK(loaded.VK.(*plonkbls.VerifyingKey))
	if e != nil {
		return e
	}
	hashBytes, e := hex.DecodeString(optimized.SHA256)
	if e != nil {
		return e
	}
	var vkHash [32]byte
	copy(vkHash[:], hashBytes)
	for gi, g := range fixture.Groups {
		ledger, row, e := deploy("ZkDPPAuditLedger.sol", "ZkDPPAuditLedger", verifiers, hasher, accounts[4])
		if e != nil {
			return e
		}
		code, e := client.CodeAt(ctx, ledger, nil)
		if e != nil {
			return e
		}
		d := audit.Deployment{ChainID: 31337, Ledger: ledger.Hex(), DeploymentBlock: row.BlockNumber, LedgerCodeSHA256: audit.CodeHash(code), PublicKeyChecksum: committee.Public.Checksum}
		if e = m7run.Write(root, "artifacts/development/m7/deployments/"+mode+"-"+g.Name+".json", d); e != nil {
			return e
		}
		call := func(from common.Address, name string, args ...any) error {
			data, e := a.Pack(name, args...)
			if e != nil {
				return e
			}
			_, e = send(from, &ledger, data, g.Name+"/"+name)
			return e
		}
		if e = call(accounts[0], "setEntryIssuer", accounts[1], true); e != nil {
			return e
		}
		if g.Name == "process" {
			_, p, _, e := g.Events[len(g.Events)-1].Decode()
			if e != nil {
				return e
			}
			if e = call(accounts[0], "registerPolicyAuthority", accounts[0]); e != nil {
				return e
			}
			if e = call(accounts[0], "reservePolicy", uint8(6)); e != nil {
				return e
			}
			if e = call(accounts[0], "registerPolicy", audit.Big(p[0]), uint8(3), uint8(2), vkHash, verifiers[6]); e != nil {
				return e
			}
			if e = call(accounts[0], "setPolicyGrant", audit.Big(p[0]), audit.Big(p[1]), true); e != nil {
				return e
			}
		}
		status := func(typ uint8, nf fr.Element, v uint8) error {
			return call(accounts[4], "setStatus", typ, audit.Big(nf), v)
		}
		rejected := func(data []byte) error {
			_, e := client.CallContract(ctx, ethereum.CallMsg{From: accounts[1], To: &ledger, Data: data}, nil)
			if e == nil {
				return fmt.Errorf("negative call accepted")
			}
			rep.NegativeChecks++
			return nil
		}
		snapshot := func() (audit.Snapshot, error) {
			h, e := client.HeaderByNumber(ctx, nil)
			if e != nil {
				return audit.Snapshot{}, e
			}
			return audit.Snapshot{Number: h.Number.Uint64(), Hash: h.Hash().Hex()}, nil
		}
		measure := func(direction string, start audit.Ref, snap audit.Snapshot) (audit.TraceResult, error) {
			src := &audit.RPCSource{Client: client, ABI: a, Deployment: d}
			t, e := audit.Trace(ctx, src, committee, direction, start, snap)
			if e != nil {
				return t, e
			}
			rep.Audits = append(rep.Audits, AuditRow{g.Name, direction, snap, t.Metrics, src.Requests, len(t.Leaves), len(t.Entries), len(t.Terminated), len(t.Relationships), true})
			return t, nil
		}
		for ei, c := range g.Events {
			_, p, rec, e := c.Decode()
			if e != nil {
				return e
			}
			if c.Kind == audit.Transfer || c.Kind == audit.Recall {
				h, e := client.HeaderByNumber(ctx, nil)
				if e != nil {
					return e
				}
				epoch := audit.Big(p[3]).Uint64()
				if c.Kind == audit.Transfer {
					epoch = audit.Big(p[4]).Uint64()
				}
				next := epoch * 600
				if next <= h.Time {
					next = h.Time + 1
				}
				if next/600 != epoch || (next+4)/600 != epoch {
					return fmt.Errorf("fixture epoch no longer available")
				}
				var result any
				if e = client.Client().CallContext(ctx, &result, "evm_setNextBlockTimestamp", next); e != nil {
					return e
				}
			}
			data, e := m7run.CaseData(a, c)
			if e != nil {
				return e
			}
			// Reject frozen inputs with the exact same proof before allowing the Event.
			if c.Kind != audit.Entry {
				l, _ := audit.Shape(c.Kind)
				nf := p[l.SpendPositions[0]]
				if g.Name == "note-graph" && c.Kind == audit.Exit {
					if e = rejected(data); e != nil {
						return e
					}
					if e = status(audit.Note, nf, audit.Active); e != nil {
						return e
					}
				} else {
					if e = status(l.ParentType, nf, audit.Frozen); e != nil {
						return e
					}
					if e = rejected(data); e != nil {
						return e
					}
					if e = status(l.ParentType, nf, audit.Active); e != nil {
						return e
					}
				}
			}
			if _, e = send(accounts[1], &ledger, data, g.Name+"/"+c.Name); e != nil {
				return e
			}
			snap, e := snapshot()
			if e != nil {
				return e
			}
			src := &audit.RPCSource{Client: client, ABI: a, Deployment: d}
			original, e := src.Load(ctx, uint64(ei+1), snap)
			if e != nil {
				return e
			}
			parts := make([]auditcrypto.Partial, 2)
			for i := range parts {
				parts[i], e = auditcrypto.PartialDecrypt(committee.Shares[i], original.Record.R1)
				if e != nil {
					return e
				}
			}
			plain, e := auditcrypto.CombineAndDecrypt(original.Context, original.Record.Ciphertext(), parts)
			if e != nil {
				return e
			}
			if !audit.Equal(plain, oracle[gi].Events[ei].Message) {
				return fmt.Errorf("original plaintext mismatch: %s", c.Name)
			}
			rep.OriginalRestores++
			for _, ref := range rec.OutputRefs {
				id, e := src.Producer(ctx, ref, snap)
				if e != nil || id != uint64(ei+1) {
					return fmt.Errorf("producer mismatch")
				}
			}
			l, _ := audit.Shape(c.Kind)
			for _, i := range l.SpendPositions {
				id, e := src.Spent(ctx, l.ParentType, p[i], snap)
				if e != nil || id != uint64(ei+1) {
					return fmt.Errorf("spentIn mismatch")
				}
			}
			if g.Name == "note-graph" && ei == 2 {
				if mode == "audit" {
					_, _, first, _ := g.Events[0].Decode()
					back, e := measure("backward", rec.OutputRefs[0], snap)
					if e != nil {
						return e
					}
					if back.Metrics.UniqueRecords != 3 || back.Metrics.Decryptions != 2 || back.Metrics.Responses != 4 || len(back.Entries) != 1 || len(back.Relationships) != 4 {
						return fmt.Errorf("backward expected counts")
					}
					forward, e := measure("forward", first.OutputRefs[0], snap)
					if e != nil {
						return e
					}
					if forward.Metrics.UniqueRecords != 3 || forward.Metrics.Decryptions != 3 || forward.Metrics.Responses != 6 || len(forward.Leaves) != 3 || len(forward.Relationships) != 4 {
						return fmt.Errorf("forward expected counts")
					}
					_, _, split1, _ := g.Events[1].Decode()
					want := []audit.Ref{split1.OutputRefs[1], rec.OutputRefs[0], rec.OutputRefs[1]}
					for i, leaf := range forward.Leaves {
						if leaf.Ref.Key() != want[i].Key() {
							return fmt.Errorf("leaf set/order mismatch")
						}
					}
				}
				for _, exit := range append([]m7run.FixedCase{g.Events[3]}, g.Extra...) {
					_, ep, _, e := exit.Decode()
					if e != nil {
						return e
					}
					if e = status(audit.Note, ep[1], audit.Frozen); e != nil {
						return e
					}
					bad, e := m7run.CaseData(a, exit)
					if e != nil {
						return e
					}
					if e = rejected(bad); e != nil {
						return e
					}
				}
			}
		}
		if g.Name == "note-graph" {
			_, p, _, _ := g.Events[3].Decode()
			data, e := a.Pack("setStatus", audit.Note, audit.Big(p[1]), audit.Frozen)
			if e != nil {
				return e
			}
			_, e = client.CallContract(ctx, ethereum.CallMsg{From: accounts[4], To: &ledger, Data: data}, nil)
			if e == nil {
				return fmt.Errorf("spent target freeze accepted")
			}
			rep.NegativeChecks++
			_, ep, _, _ := g.Extra[1].Decode()
			if e = status(audit.Note, ep[1], audit.Revoked); e != nil {
				return e
			}
			bad, e := m7run.CaseData(a, g.Extra[1])
			if e != nil {
				return e
			}
			if e = rejected(bad); e != nil {
				return e
			}
		}
		// Compare final roots, counts and every current membership path with Go.
		for _, tree := range []struct {
			rootName, countName, pathName string
			root                          fr.Element
			count                         uint64
			paths                         bool
		}{{"currentNoteRoot", "noteLeafCount", "getNotePath", oracle[gi].NoteRoot, oracle[gi].NoteCount, false}, {"currentVoucherRoot", "voucherLeafCount", "getVoucherPath", oracle[gi].VoucherRoot, oracle[gi].VoucherCount, true}} {
			v, e := m7run.View(ctx, client, a, ledger, tree.rootName)
			if e != nil {
				return e
			}
			if v[0].(*big.Int).Cmp(audit.Big(tree.root)) != 0 {
				return fmt.Errorf("Go/EVM root mismatch %s", g.Name)
			}
			v, e = m7run.View(ctx, client, a, ledger, tree.countName)
			if e != nil {
				return e
			}
			if v[0].(*big.Int).Uint64() != tree.count {
				return fmt.Errorf("leaf count mismatch")
			}
			paths := oracle[gi].NotePaths
			if tree.paths {
				paths = oracle[gi].VoucherPaths
			}
			for i, p := range paths {
				v, e := m7run.View(ctx, client, a, ledger, tree.pathName, big.NewInt(int64(i)))
				if e != nil {
					return e
				}
				got, e := audit.Fields(v[1].([]*big.Int))
				if e != nil {
					return e
				}
				if !audit.Equal(got, p.Siblings[:]) {
					return fmt.Errorf("Go/EVM path mismatch")
				}
			}
		}
		if mode == "audit" {
			snap, e := snapshot()
			if e != nil {
				return e
			}
			_, _, first, _ := g.Events[0].Decode()
			if _, e = measure("forward", first.OutputRefs[0], snap); e != nil {
				return e
			}
			if g.Name != "note-graph" {
				_, _, last, _ := g.Events[len(g.Events)-1].Decode()
				if _, e = measure("backward", last.OutputRefs[0], snap); e != nil {
					return e
				}
			}
		}
		if g.Name == "process" {
			_, p, _, _ := g.Events[len(g.Events)-1].Decode()
			if e = call(accounts[0], "disablePolicy", audit.Big(p[0])); e != nil {
				return e
			}
		}
		rep.Groups = append(rep.Groups, GroupResult{g.Name, ledger.Hex(), oracle[gi].NoteCount, oracle[gi].VoucherCount, uint64(len(g.Events)), true, true, true})
		fmt.Printf("%s records=%d roots/paths/originals/status=OK\n", g.Name, len(g.Events))
	}
	return m7run.Write(root, output, rep)
}
