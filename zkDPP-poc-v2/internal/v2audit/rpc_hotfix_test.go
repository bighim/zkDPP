package v2audit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	h "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReceiptErrorPreservesTransactionHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage
			Method string
		}
		json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "eth_estimateGas":
			result = "0x10000"
		case "eth_sendTransaction":
			result = "0x" + strings.Repeat("ab", 32)
		case "eth_getTransactionReceipt":
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "receipt temporarily unavailable"}})
			return
		default:
			t.Errorf("unexpected %s", req.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	c, e := ethclient.Dial(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	a, e := abi.JSON(strings.NewReader(`[{"type":"function","name":"setStatus","inputs":[{"type":"uint8","name":"objectType"},{"type":"uint256","name":"spendValue"},{"type":"uint8","name":"newStatus"}],"outputs":[]}]`))
	if e != nil {
		t.Fatal(e)
	}
	s := NewRPCSource(c, a, common.Address{1}, common.Address{2})
	value := h.Element(3)
	if e = s.Freeze(context.Background(), Note, value.Bytes()); e == nil {
		t.Fatal("receipt error lost")
	}
	tx := s.LastTransaction()
	if tx == nil || tx.Hash != "0x"+strings.Repeat("ab", 32) || tx.State != "UNCERTAIN" || tx.Error == "" {
		t.Fatalf("lost tx: %+v", tx)
	}
}

func TestWrongRPCIdentityAndHashSelector(t *testing.T) {
	for _, kind := range []string{"chain", "code"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage
					Method string
					Params []json.RawMessage
				}
				json.NewDecoder(r.Body).Decode(&req)
				var result any
				switch req.Method {
				case "eth_chainId":
					result = "0x7a69"
				case "eth_getCode":
					var selector map[string]any
					if e := json.Unmarshal(req.Params[1], &selector); e != nil || selector["requireCanonical"] != true || selector["blockHash"] != "0x"+strings.Repeat("12", 32) {
						t.Error("state is not pinned to canonical hash")
					}
					result = "0x6000"
				default:
					t.Errorf("unexpected RPC %s", req.Method)
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer server.Close()
			c, e := ethclient.Dial(server.URL)
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			s := NewRPCSource(c, abi.ABI{}, common.Address{1}, common.Address{2})
			s.Context = LedgerContext{ChainID: 31337, CodeHash: "wrong", DeploymentBlock: 1}
			if kind == "chain" {
				s.Context.ChainID = 1
			}
			if e = s.CheckSnapshot(context.Background(), Snapshot{BlockNumber: 10, BlockHash: "0x" + strings.Repeat("12", 32)}); e == nil {
				t.Fatal("wrong identity accepted")
			}
		})
	}
}

type failingSource struct {
	*MemorySource
	BadCheckpoint bool
	FinalReorg    bool
	checks        int
	Uncertain     bool
	FailStatus    bool
}

func (s *failingSource) Checkpoint(c context.Context) (Snapshot, error) {
	if s.BadCheckpoint {
		return Snapshot{BlockNumber: 0, BlockHash: "old"}, nil
	}
	return s.MemorySource.Checkpoint(c)
}
func (s *failingSource) CheckSnapshot(c context.Context, p Snapshot) error {
	s.checks++
	if s.FinalReorg && s.checks >= 5 {
		return fmt.Errorf("reorg")
	}
	return s.MemorySource.CheckSnapshot(c, p)
}
func (s *failingSource) Freeze(c context.Context, t uint8, v [32]byte) error {
	if s.Uncertain {
		s.Statuses[typedSpend(t, v)] = Frozen
		return fmt.Errorf("receipt uncertainty")
	}
	return s.MemorySource.Freeze(c, t, v)
}
func (s *failingSource) LastTransaction() *RPCTransaction {
	if s.Uncertain {
		return &RPCTransaction{Hash: "known", State: "UNCERTAIN"}
	}
	return nil
}
func (s *failingSource) Status(c context.Context, t uint8, v [32]byte, p Snapshot) (uint8, bool, error) {
	if s.FailStatus {
		return 0, false, fmt.Errorf("lookup unavailable")
	}
	return s.MemorySource.Status(c, t, v, p)
}
func singleTarget(t *testing.T) (*MemorySource, *big.Int, Ref) {
	t.Helper()
	pkg, shares, e := keyFixture()
	if e != nil {
		t.Fatal(e)
	}
	master, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if e != nil {
		t.Fatal(e)
	}
	ref := Ref{Note, h.Element(44)}
	ct, e := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{h.Element(55)}, big.NewInt(66))
	if e != nil {
		t.Fatal(e)
	}
	m := NewMemorySource(10, "block-10")
	_, e = m.Add(Record{EventKind: Entry, OutputRefs: []Ref{ref}, R1: ct.R1, EncryptedOutputNfs: ct.Data}, Note, nil)
	if e != nil {
		t.Fatal(e)
	}
	return m, master, ref
}
func TestFailedCheckpointsAndUncertainTransactions(t *testing.T) {
	for _, name := range []string{"old", "reorg", "uncertain", "lookup"} {
		t.Run(name, func(t *testing.T) {
			m, key, ref := singleTarget(t)
			defer key.SetInt64(0)
			s := &failingSource{MemorySource: m, BadCheckpoint: name == "old", FinalReorg: name == "reorg", Uncertain: name == "uncertain", FailStatus: name == "lookup"}
			r := AuditAndFreeze(context.Background(), s, key, ref, m.Current)
			if r.Outcome != Incomplete || len(r.TargetResults) != 1 {
				t.Fatalf("false completion %+v", r)
			}
			if name == "uncertain" && (r.TargetResults[0].Transaction == nil || r.TargetResults[0].Transaction.Hash != "known") {
				t.Fatal("lost hash")
			}
		})
	}
}
