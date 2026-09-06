package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6b1case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m6b1run"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const project = "zkdpp-m6-b1"
const rpcURL = "http://127.0.0.1:18545"

// Public Anvil test account, unrelated to ZK owner and committee secrets.
const anvilTestKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

type Fixture struct {
	Profile           string   `json:"profile"`
	PublicKeyChecksum string   `json:"publicKeyChecksum"`
	Proof             string   `json:"proof"`
	PublicInputs      []string `json:"publicInputs"`
}

func loadFixture(root string, c auditcrypto.PublicConfig) (Fixture, []fr.Element, error) {
	var f Fixture
	if e := m6b1run.ReadJSON(filepath.Join(root, "contracts", "test", "fixtures", "m6-b1-proofs.json"), &f); e != nil {
		return f, nil, e
	}
	if f.Profile != auditcrypto.Profile || f.PublicKeyChecksum != c.Checksum || len(f.PublicInputs) != 15 {
		return f, nil, fmt.Errorf("fixture/public-key binding mismatch")
	}
	values := make([]fr.Element, 15)
	for i, s := range f.PublicInputs {
		v, e := auditcrypto.DecodeField(s)
		if e != nil {
			return f, nil, e
		}
		values[i] = v
	}
	return f, values, nil
}
func main() {
	root := flag.String("root", ".", "project root")
	mode := flag.String("mode", "gas", "gas, decrypt or checks")
	flag.Parse()
	runtime.GOMAXPROCS(8)
	var e error
	switch *mode {
	case "gas":
		e = gas(*root)
	case "decrypt":
		e = decrypt(*root)
	case "checks":
		e = m6b1run.FinalChecks(*root)
	default:
		e = fmt.Errorf("unknown mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

type Workload struct {
	Ciphertexts                                                        int
	FieldsPerCiphertext                                                int
	PartialResponses                                                   int
	RunCount                                                           int
	Member1Millis, Member2Millis, CombineAndDecryptMillis, TotalMillis float64
	AllocatedBytes, HeapBefore, HeapAfter                              uint64
	UniqueR1                                                           bool
	RecoveredAll                                                       bool
	CiphertextDigest                                                   string
}

func decrypt(root string) (err error) {
	attempt, finish, e := m6b1run.Begin(root, "decrypt", "output/m6-b1-decryption.json")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, e := auditcrypto.LoadCommittee(m6b1run.ArtifactDir(root, "committee"))
	if e != nil {
		return e
	}
	_, values, e := loadFixture(root, committee.Public)
	if e != nil {
		return e
	}
	l, _, e := m6b1case.FromRecord(values)
	if e != nil {
		return e
	}
	message, e := m6b1case.ExpectedMessage(root)
	if e != nil {
		return e
	}
	results := []Workload{}
	for _, n := range []int{1, 10, 100, 1000} {
		cts := make([]auditcrypto.Ciphertext, n)
		seen := map[string]bool{}
		digest := sha256.New()
		for i := range cts {
			r, e := auditcrypto.RandomScalar(nil)
			if e != nil {
				return e
			}
			cts[i], e = auditcrypto.Encrypt(committee.PK, l, message, r)
			if e != nil {
				return e
			}
			r.SetInt64(0)
			encoded := auditcrypto.EncodeCiphertext(cts[i])
			id := encoded[0] + encoded[1]
			if seen[id] {
				return fmt.Errorf("duplicate nonce point in workload")
			}
			seen[id] = true
			for _, word := range encoded {
				digest.Write([]byte(word))
			}
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		var d1, d2, dc time.Duration
		start := time.Now()
		for _, ct := range cts {
			t := time.Now()
			p1, e := auditcrypto.PartialDecrypt(committee.Shares[0], ct.R1)
			d1 += time.Since(t)
			if e != nil {
				return e
			}
			t = time.Now()
			p2, e := auditcrypto.PartialDecrypt(committee.Shares[1], ct.R1)
			d2 += time.Since(t)
			if e != nil {
				return e
			}
			t = time.Now()
			plain, e := auditcrypto.CombineAndDecrypt(l, ct, []auditcrypto.Partial{p1, p2})
			dc += time.Since(t)
			if e != nil {
				return e
			}
			if !m6b1case.Equal(plain, message) {
				return fmt.Errorf("workload plaintext mismatch")
			}
		}
		total := time.Since(start)
		runtime.ReadMemStats(&after)
		w := Workload{Ciphertexts: n, FieldsPerCiphertext: 5, PartialResponses: 2 * n, RunCount: 1, Member1Millis: m6b1run.Millis(d1), Member2Millis: m6b1run.Millis(d2), CombineAndDecryptMillis: m6b1run.Millis(dc), TotalMillis: m6b1run.Millis(total), AllocatedBytes: after.TotalAlloc - before.TotalAlloc, HeapBefore: before.HeapAlloc, HeapAfter: after.HeapAlloc, UniqueR1: true, RecoveredAll: true, CiphertextDigest: hex.EncodeToString(digest.Sum(nil))}
		results = append(results, w)
		fmt.Printf("decrypt workload=%d total=%.3fms recovered=true\n", n, w.TotalMillis)
	}
	out := struct {
		RunCount, Attempt        int
		Environment              m6b1run.Environment
		PublicKeyChecksum, Scope string
		Results                  []Workload
	}{1, attempt, m6b1run.Env(), committee.Public.Checksum, "Offline native core throughput. Setup/encryption/file loading/network/graph traversal excluded. Total includes plaintext equality checks; phase timings include normal input validation.", results}
	return m6b1run.Write(root, "output/m6-b1-decryption.json", out)
}

type Tx struct {
	Name, Hash, ContractAddress        string
	BlockNumber, GasUsed               uint64
	CalldataBytes                      int
	PrepareMillis, SubmitReceiptMillis float64
}
type rawContract struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

func contract(root, source, name string) (abi.ABI, []byte, error) {
	var raw rawContract
	if e := m6b1run.ReadJSON(filepath.Join(root, "contracts", "out", source, name+".json"), &raw); e != nil {
		return abi.ABI{}, nil, e
	}
	a, e := abi.JSON(bytes.NewReader(raw.ABI))
	if e != nil {
		return a, nil, e
	}
	b, e := hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x"))
	return a, b, e
}
func docker(root string, args ...string) error {
	c := exec.Command("docker", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "ANVIL_PORT=18545", "FOUNDRY_IMAGE="+m6b1run.FoundryImage)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}
func compose(root string, args ...string) error {
	return docker(root, append([]string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.m6-b1.yml", "-p", project}, args...)...)
}
func gas(root string) (err error) {
	attempt, finish, e := m6b1run.Begin(root, "gas", "output/m6-b1-anvil-gas.json")
	if e != nil {
		return e
	}
	defer func() { finish(err) }()
	committee, e := auditcrypto.LoadCommittee(m6b1run.ArtifactDir(root, "committee"))
	if e != nil {
		return e
	}
	f, values, e := loadFixture(root, committee.Public)
	if e != nil {
		return e
	}
	proof, e := hex.DecodeString(strings.TrimPrefix(f.Proof, "0x"))
	if e != nil {
		return e
	}
	if conn, e := net.DialTimeout("tcp", "127.0.0.1:18545", 300*time.Millisecond); e == nil {
		conn.Close()
		return fmt.Errorf("port 18545 already occupied; refusing to stop another service")
	}
	probe := exec.Command("docker", "compose", "-p", project, "ps", "-q")
	probe.Dir = root
	b, e := probe.CombinedOutput()
	if e != nil {
		return fmt.Errorf("Docker project check: %w: %s", e, b)
	}
	if strings.TrimSpace(string(b)) != "" {
		return fmt.Errorf("benchmark project already has containers; refusing replacement")
	}
	if e = compose(root, "up", "-d", "anvil"); e != nil {
		return e
	}
	defer func() {
		if e := compose(root, "down"); e != nil && err == nil {
			err = fmt.Errorf("benchmark cleanup: %w", e)
		}
	}()
	if e = compose(root, "run", "--rm", "foundry", "forge", "build"); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var client *ethclient.Client
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		c, e := ethclient.Dial(rpcURL)
		if e == nil {
			if _, e = c.BlockNumber(ctx); e == nil {
				client = c
				break
			}
			c.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	if client == nil {
		return fmt.Errorf("Anvil readiness timeout")
	}
	defer client.Close()
	id, e := client.ChainID(ctx)
	if e != nil {
		return e
	}
	if id.Uint64() != 31337 {
		return fmt.Errorf("unexpected chain ID")
	}
	header, e := client.HeaderByNumber(ctx, nil)
	if e != nil {
		return e
	}
	if header.GasLimit != 30000000 {
		return fmt.Errorf("unexpected block gas limit")
	}
	var version string
	if e = client.Client().CallContext(ctx, &version, "web3_clientVersion"); e != nil {
		return e
	}
	if !strings.Contains(version, "1.7.1") {
		return fmt.Errorf("unexpected Anvil version: %s", version)
	}
	imageID := ""
	inspect := exec.Command("docker", "image", "inspect", m6b1run.FoundryImage, "--format", "{{.Id}}")
	if b, e := inspect.Output(); e == nil {
		imageID = strings.TrimSpace(string(b))
	}
	_, verifierCode, e := contract(root, "AuditProcessVerifier.sol", "AuditProcessVerifier")
	if e != nil {
		return e
	}
	storeABI, storeCode, e := contract(root, "AuditEncryptionStore.sol", "AuditEncryptionStore")
	if e != nil {
		return e
	}
	txs := []Tx{}
	verifier, t, e := transact(ctx, client, id, nil, verifierCode, "verifier-deployment")
	if e != nil {
		return e
	}
	txs = append(txs, t)
	args, e := storeABI.Pack("", verifier)
	if e != nil {
		return e
	}
	addr, t, e := transact(ctx, client, id, nil, append(storeCode, args...), "store-deployment")
	if e != nil {
		return e
	}
	txs = append(txs, t)
	var words [15]*big.Int
	for i := range values {
		words[i] = values[i].BigInt(new(big.Int))
	}
	data, e := storeABI.Pack("verifyOnly", proof, words)
	if e != nil {
		return e
	}
	_, verifyTx, e := transact(ctx, client, id, &addr, data, "verify-only")
	if e != nil {
		return e
	}
	txs = append(txs, verifyTx)
	next, e := view(ctx, client, storeABI, addr, "nextRecordId")
	if e != nil {
		return e
	}
	if next[0].(*big.Int).Uint64() != 1 {
		return fmt.Errorf("verify-only changed state")
	}
	data, e = storeABI.Pack("verifyAndStore", proof, words)
	if e != nil {
		return e
	}
	_, storeTx, e := transact(ctx, client, id, &addr, data, "verify-and-store")
	if e != nil {
		return e
	}
	txs = append(txs, storeTx)
	next, e = view(ctx, client, storeABI, addr, "nextRecordId")
	if e != nil {
		return e
	}
	if next[0].(*big.Int).Uint64() != 2 {
		return fmt.Errorf("record count mismatch")
	}
	parts := []auditcrypto.Partial{}
	var actualContext fr.Element
	var actualCT auditcrypto.Ciphertext
	// Each committee actor fetches the same immutable on-chain original by ID.
	for i := 0; i < 2; i++ {
		got, e := view(ctx, client, storeABI, addr, "getRecord", big.NewInt(1))
		if e != nil {
			return e
		}
		array, ok := got[0].([15]*big.Int)
		if !ok {
			return fmt.Errorf("unexpected record ABI")
		}
		actual := make([]fr.Element, 15)
		for j, v := range array {
			if v.Sign() < 0 || v.Cmp(fr.Modulus()) >= 0 {
				return fmt.Errorf("noncanonical on-chain field")
			}
			actual[j].SetBigInt(v)
		}
		if !m6b1case.Equal(actual, values) {
			return fmt.Errorf("stored public inputs differ from verified fixture")
		}
		actualContext, actualCT, e = m6b1case.FromRecord(actual)
		if e != nil {
			return e
		}
		part, e := auditcrypto.PartialDecrypt(committee.Shares[i], actualCT.R1)
		if e != nil {
			return e
		}
		parts = append(parts, part)
	}
	plain, e := auditcrypto.CombineAndDecrypt(actualContext, actualCT, parts)
	if e != nil {
		return e
	}
	expected, e := m6b1case.ExpectedMessage(root)
	if e != nil {
		return e
	}
	if !m6b1case.Equal(plain, expected) {
		return fmt.Errorf("on-chain original did not restore expected plaintext")
	}
	var trace struct {
		StructLogs []struct {
			Op      string          `json:"op"`
			GasCost json.RawMessage `json:"gasCost"`
		} `json:"structLogs"`
	}
	if e = client.Client().CallContext(ctx, &trace, "debug_traceTransaction", storeTx.Hash, map[string]any{"disableStorage": true, "disableStack": true, "enableMemory": false, "enableReturnData": false}); e != nil {
		return fmt.Errorf("SSTORE trace: %w", e)
	}
	var count, gasCost uint64
	for _, log := range trace.StructLogs {
		if log.Op == "SSTORE" {
			v, e := quantity(log.GasCost)
			if e != nil {
				return e
			}
			count++
			gasCost += v
		}
	}
	if count == 0 {
		return fmt.Errorf("trace contains no SSTORE")
	}
	out := struct {
		RunCount, Attempt                                                             int
		Environment                                                                   m6b1run.Environment
		PublicKeyChecksum, FoundryImage, ImageID, ClientVersion, RPC, Hardfork, Scope string
		ChainID, BlockGasLimit                                                        uint64
		Transactions                                                                  []Tx
		IncrementalRecordGas                                                          int64
		SSTORECount, SSTOREOpcodeGas                                                  uint64
		StoredFieldCount, LogicalRecordBytes, CiphertextBytes                         int
		RecordID, NextRecordID                                                        uint64
		VerifyOnlyUnchanged, CommitteeFetchedOriginal, RecoveredMessageMatches        bool
	}{1, attempt, m6b1run.Env(), committee.Public.Checksum, m6b1run.FoundryImage, imageID, version, rpcURL, "prague", "Diagnostic Process verification/storage, not Main Ledger Event gas. SSTORE opcode sum is separate from total transaction delta.", 31337, 30000000, txs, int64(storeTx.GasUsed) - int64(verifyTx.GasUsed), count, gasCost, 15, 480, 224, 1, 2, true, true, true}
	fmt.Printf("verify=%d store=%d SSTORE=%d recovered=true\n", verifyTx.GasUsed, storeTx.GasUsed, gasCost)
	return m6b1run.Write(root, "output/m6-b1-anvil-gas.json", out)
}
func quantity(raw json.RawMessage) (uint64, error) {
	var n uint64
	if json.Unmarshal(raw, &n) == nil {
		return n, nil
	}
	var s string
	if e := json.Unmarshal(raw, &s); e != nil {
		return 0, e
	}
	return strconv.ParseUint(s, 0, 64)
}
func view(ctx context.Context, c *ethclient.Client, a abi.ABI, addr common.Address, method string, args ...any) ([]any, error) {
	data, e := a.Pack(method, args...)
	if e != nil {
		return nil, e
	}
	b, e := c.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if e != nil {
		return nil, e
	}
	return a.Unpack(method, b)
}
func transact(ctx context.Context, c *ethclient.Client, id *big.Int, to *common.Address, data []byte, name string) (common.Address, Tx, error) {
	key, e := crypto.HexToECDSA(anvilTestKey)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	start := time.Now()
	nonce, e := c.PendingNonceAt(ctx, from)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	price, e := c.SuggestGasPrice(ctx)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	gas, e := c.EstimateGas(ctx, ethereum.CallMsg{From: from, To: to, Data: data})
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	if gas > 30000000 {
		return common.Address{}, Tx{}, fmt.Errorf("transaction exceeds 30M gas")
	}
	unsigned := types.NewTx(&types.LegacyTx{Nonce: nonce, To: to, Value: big.NewInt(0), Gas: gas, GasPrice: price, Data: data})
	signed, e := types.SignTx(unsigned, types.LatestSignerForChainID(id), key)
	if e != nil {
		return common.Address{}, Tx{}, e
	}
	prepare := time.Since(start)
	start = time.Now()
	if e = c.SendTransaction(ctx, signed); e != nil {
		return common.Address{}, Tx{}, e
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		receipt, e := c.TransactionReceipt(ctx, signed.Hash())
		if e == nil {
			if receipt.Status != 1 {
				return common.Address{}, Tx{}, fmt.Errorf("%s transaction reverted", name)
			}
			tx := Tx{Name: name, Hash: receipt.TxHash.Hex(), ContractAddress: receipt.ContractAddress.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), GasUsed: receipt.GasUsed, CalldataBytes: len(data), PrepareMillis: m6b1run.Millis(prepare), SubmitReceiptMillis: m6b1run.Millis(time.Since(start))}
			if to != nil {
				tx.ContractAddress = to.Hex()
			}
			return receipt.ContractAddress, tx, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return common.Address{}, Tx{}, fmt.Errorf("%s receipt timeout", name)
}
