package v2audit

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"reflect"
	"time"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type RPCTransaction struct {
	Hash          string `json:"hash"`
	BlockNumber   uint64 `json:"blockNumber"`
	GasUsed       uint64 `json:"gasUsed"`
	CalldataBytes int    `json:"calldataBytes"`
	BlockHash     string `json:"blockHash"`
	State         string `json:"state"`
	Error         string `json:"error,omitempty"`
}

type LedgerContext struct {
	ChainID         uint64
	CodeHash        string
	DeploymentBlock uint64
}

type RPCSource struct {
	Client             *ethclient.Client
	ABI                abi.ABI
	Ledger             common.Address
	StatusAuthority    common.Address
	FreezeTransactions []RPCTransaction
	Context            LedgerContext
	Calls              int
	last               *RPCTransaction
}

func NewRPCSource(client *ethclient.Client, contractABI abi.ABI, ledger, statusAuthority common.Address) *RPCSource {
	return &RPCSource{Client: client, ABI: contractABI, Ledger: ledger, StatusAuthority: statusAuthority}
}

func (s *RPCSource) CheckSnapshot(ctx context.Context, snapshot Snapshot) error {
	s.Calls++
	id, e := s.Client.ChainID(ctx)
	if e != nil {
		return e
	}
	if s.Context.ChainID == 0 || id.Uint64() != s.Context.ChainID {
		return fmt.Errorf("chain identity mismatch")
	}
	if snapshot.BlockNumber < s.Context.DeploymentBlock {
		return fmt.Errorf("snapshot predates deployment")
	}
	var code hexutil.Bytes
	s.Calls++
	if e = s.Client.Client().CallContext(ctx, &code, "eth_getCode", s.Ledger, map[string]any{"blockHash": snapshot.BlockHash, "requireCanonical": true}); e != nil {
		return e
	}
	if s.Context.CodeHash == "" || crypto.Keccak256Hash(code).Hex() != s.Context.CodeHash {
		return fmt.Errorf("runtime identity mismatch")
	}
	s.Calls++
	header, err := s.Client.HeaderByNumber(ctx, new(big.Int).SetUint64(snapshot.BlockNumber))
	if err != nil {
		return err
	}
	if header.Hash().Hex() != snapshot.BlockHash {
		return fmt.Errorf("snapshot block hash mismatch")
	}
	return nil
}
func (s *RPCSource) Checkpoint(ctx context.Context) (Snapshot, error) {
	s.Calls++
	header, err := s.Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash().Hex()}, nil
}
func (s *RPCSource) call(ctx context.Context, method string, snapshot Snapshot, args ...any) ([]any, error) {
	data, err := s.ABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	s.Calls++
	var raw hexutil.Bytes
	err = s.Client.Client().CallContext(ctx, &raw, "eth_call", map[string]any{"to": s.Ledger, "data": hexutil.Bytes(data)}, map[string]any{"blockHash": snapshot.BlockHash, "requireCanonical": true})
	if err != nil {
		return nil, err
	}
	return s.ABI.Unpack(method, raw)
}
func (s *RPCSource) uint(ctx context.Context, method string, snapshot Snapshot, args ...any) (*big.Int, error) {
	values, err := s.call(ctx, method, snapshot, args...)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("%s output count", method)
	}
	value, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("%s output type %T", method, values[0])
	}
	return value, nil
}

func (s *RPCSource) Producer(ctx context.Context, ref Ref, snapshot Snapshot) (uint64, error) {
	v, err := s.uint(ctx, "producerOf", snapshot, ref.ObjectType, Big(ref.RawID))
	if err != nil {
		return 0, err
	}
	return v.Uint64(), nil
}
func (s *RPCSource) SpentIn(ctx context.Context, objectType uint8, spend [32]byte, snapshot Snapshot) (uint64, error) {
	method := "noteSpentIn"
	if objectType == Voucher {
		method = "voucherSpentIn"
	}
	v, err := s.uint(ctx, method, snapshot, new(big.Int).SetBytes(spend[:]))
	if err != nil {
		return 0, err
	}
	return v.Uint64(), nil
}
func (s *RPCSource) Status(ctx context.Context, objectType uint8, spend [32]byte, snapshot Snapshot) (uint8, bool, error) {
	spent, err := s.SpentIn(ctx, objectType, spend, snapshot)
	if err != nil {
		return 0, false, err
	}
	method := "noteStatusByNf"
	if objectType == Voucher {
		method = "voucherStatusByNf"
	}
	values, err := s.call(ctx, method, snapshot, new(big.Int).SetBytes(spend[:]))
	if err != nil {
		return 0, false, err
	}
	if len(values) != 1 {
		return 0, false, fmt.Errorf("status output")
	}
	status, ok := values[0].(uint8)
	if !ok {
		return 0, false, fmt.Errorf("status type %T", values[0])
	}
	return status, spent != 0, nil
}

func (s *RPCSource) Record(ctx context.Context, id uint64, snapshot Snapshot) (Record, error) {
	values, err := s.call(ctx, "getAuditRecord", snapshot, new(big.Int).SetUint64(id))
	if err != nil {
		return Record{}, err
	}
	if len(values) != 1 {
		return Record{}, fmt.Errorf("record output count")
	}
	v := reflect.ValueOf(values[0])
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return Record{}, fmt.Errorf("record output type %T", values[0])
	}
	field := func(name string) reflect.Value { return v.FieldByName(name) }
	result := Record{EventKind: Kind(field("EventKind").Uint())}
	policy, err := Field(field("PolicyRef").Interface().(*big.Int))
	if err != nil {
		return Record{}, err
	}
	result.PolicyRef = policy
	refs := field("OutputRefs")
	for i := 0; i < refs.Len(); i++ {
		item := refs.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		raw, err := Field(item.FieldByName("RawId").Interface().(*big.Int))
		if err != nil {
			return Record{}, err
		}
		result.OutputRefs = append(result.OutputRefs, Ref{uint8(item.FieldByName("ObjectType").Uint()), raw})
	}
	r1x, err := Field(field("R1X").Interface().(*big.Int))
	if err != nil {
		return Record{}, err
	}
	r1y, err := Field(field("R1Y").Interface().(*big.Int))
	if err != nil {
		return Record{}, err
	}
	result.R1.X = r1x
	result.R1.Y = r1y
	for _, spec := range []struct {
		name string
		out  *[]fr.Element
	}{{"EncryptedParents", &result.EncryptedParents}, {"EncryptedOutputNfs", &result.EncryptedOutputNfs}} {
		slice := field(spec.name)
		for i := 0; i < slice.Len(); i++ {
			value, err := Field(slice.Index(i).Interface().(*big.Int))
			if err != nil {
				return Record{}, err
			}
			*spec.out = append(*spec.out, value)
		}
	}
	if err = result.Validate(); err != nil {
		return Record{}, err
	}
	if err = s.VerifyOriginal(ctx, id, snapshot, result); err != nil {
		return Record{}, err
	}
	return result, nil
}

func (s *RPCSource) Freeze(ctx context.Context, objectType uint8, spend [32]byte) error {
	s.last = &RPCTransaction{State: "REJECTED"}
	data, err := s.ABI.Pack("setStatus", objectType, new(big.Int).SetBytes(spend[:]), uint8(Frozen))
	if err != nil {
		return err
	}
	s.Calls++
	gas, err := s.Client.EstimateGas(ctx, ethereum.CallMsg{From: s.StatusAuthority, To: &s.Ledger, Data: data})
	if err != nil {
		return err
	}
	tx := map[string]any{"from": s.StatusAuthority, "to": s.Ledger, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(gas)}
	var hash common.Hash
	s.Calls++
	if err = s.Client.Client().CallContext(ctx, &hash, "eth_sendTransaction", tx); err != nil {
		s.last.State = "UNCERTAIN"
		s.last.Error = err.Error()
		return err
	}
	s.last = &RPCTransaction{Hash: hash.Hex(), State: "UNCERTAIN", CalldataBytes: len(data)}
	var receipt *types.Receipt
	for attempt := 0; attempt < 100; attempt++ {
		s.Calls++
		receipt, err = s.Client.TransactionReceipt(ctx, hash)
		if err == nil {
			break
		}
		if err != ethereum.NotFound {
			s.last.Error = err.Error()
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	if receipt == nil {
		s.last.Error = "receipt confirmation deadline"
		return fmt.Errorf("receipt confirmation deadline")
	}
	s.last.BlockNumber = receipt.BlockNumber.Uint64()
	s.last.BlockHash = receipt.BlockHash.Hex()
	s.last.GasUsed = receipt.GasUsed
	if receipt.Status != 1 {
		s.last.State = "REJECTED"
		return fmt.Errorf("Freeze transaction reverted")
	}
	s.last.State = "CONFIRMED"
	s.FreezeTransactions = append(s.FreezeTransactions, *s.last)
	return nil
}

func (s *RPCSource) LastTransaction() *RPCTransaction {
	if s.last == nil {
		return nil
	}
	v := *s.last
	return &v
}
func (s *RPCSource) ValidateReceipt(ctx context.Context, tx RPCTransaction, snap Snapshot) error {
	if tx.BlockNumber > snap.BlockNumber {
		return fmt.Errorf("receipt after checkpoint")
	}
	s.Calls++
	r, e := s.Client.TransactionReceipt(ctx, common.HexToHash(tx.Hash))
	if e != nil {
		return e
	}
	s.Calls++
	h, e := s.Client.HeaderByNumber(ctx, r.BlockNumber)
	if e != nil {
		return e
	}
	if r.Status != 1 || r.BlockHash != h.Hash() || r.BlockHash.Hex() != tx.BlockHash {
		return fmt.Errorf("receipt is not canonical")
	}
	return nil
}

type CipherABI struct {
	R1X, R1Y                             *big.Int
	EncryptedParents, EncryptedOutputNfs []*big.Int
}

func (s *RPCSource) VerifyOriginal(ctx context.Context, id uint64, snap Snapshot, r Record) error {
	event, ok := s.ABI.Events["AuditRecorded"]
	if !ok {
		return fmt.Errorf("missing audit event ABI")
	}
	logs, e := s.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(s.Context.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.BlockNumber), Addresses: []common.Address{s.Ledger}, Topics: [][]common.Hash{{event.ID}, {common.BigToHash(new(big.Int).SetUint64(id))}}})
	s.Calls++
	if e != nil {
		return e
	}
	if len(logs) != 1 {
		return fmt.Errorf("audit original log cardinality")
	}
	log := logs[0]
	s.Calls++
	tx, pending, e := s.Client.TransactionByHash(ctx, log.TxHash)
	if e != nil {
		return e
	}
	if pending || tx.To() == nil || *tx.To() != s.Ledger {
		return fmt.Errorf("wrong original destination")
	}
	s.Calls++
	receipt, e := s.Client.TransactionReceipt(ctx, log.TxHash)
	if e != nil {
		return e
	}
	s.Calls++
	header, e := s.Client.HeaderByNumber(ctx, receipt.BlockNumber)
	if e != nil {
		return e
	}
	if receipt.Status != 1 || receipt.BlockHash != log.BlockHash || header.Hash() != log.BlockHash || receipt.BlockNumber.Uint64() > snap.BlockNumber {
		return fmt.Errorf("original receipt mismatch")
	}
	data := tx.Data()
	if len(data) < 4 {
		return fmt.Errorf("short original")
	}
	method, e := s.ABI.MethodById(data[:4])
	if e != nil {
		return e
	}
	if method.Name != r.EventKind.String() {
		return fmt.Errorf("original EventKind mismatch")
	}
	args, e := method.Inputs.Unpack(data[4:])
	if e != nil {
		return e
	}
	encoded, e := method.Inputs.Pack(args...)
	if e != nil {
		return e
	}
	if !bytes.Equal(encoded, data[4:]) {
		return fmt.Errorf("noncanonical calldata")
	}
	var base []fr.Element
	for _, a := range args[1 : len(args)-1] {
		v := reflect.ValueOf(a)
		if v.Kind() == reflect.Array {
			for i := 0; i < v.Len(); i++ {
				x, e := Field(v.Index(i).Interface().(*big.Int))
				if e != nil {
					return e
				}
				base = append(base, x)
			}
		} else {
			x, e := Field(a.(*big.Int))
			if e != nil {
				return e
			}
			base = append(base, x)
		}
	}
	a := abi.ConvertType(args[len(args)-1], new(CipherABI)).(*CipherABI)
	cipher := r.Ciphertext()
	if a.R1X.Cmp(Big(r.R1.X)) != 0 || a.R1Y.Cmp(Big(r.R1.Y)) != 0 {
		return fmt.Errorf("original point mismatch")
	}
	fields := append(append([]*big.Int{}, a.EncryptedParents...), a.EncryptedOutputNfs...)
	if len(fields) != len(cipher.Data) {
		return fmt.Errorf("original cipher length")
	}
	for i, v := range fields {
		if v.Cmp(Big(cipher.Data[i])) != 0 {
			return fmt.Errorf("original ciphertext mismatch")
		}
	}
	expected, e := NewRecord(r.EventKind, base, cipher)
	if e != nil {
		return e
	}
	if expected.EventKind != r.EventKind || !expected.PolicyRef.Equal(&r.PolicyRef) || len(expected.OutputRefs) != len(r.OutputRefs) || !Equal(expected.EncryptedParents, r.EncryptedParents) || !Equal(expected.EncryptedOutputNfs, r.EncryptedOutputNfs) {
		return fmt.Errorf("original record mismatch")
	}
	for i := range expected.OutputRefs {
		if expected.OutputRefs[i].Key() != r.OutputRefs[i].Key() {
			return fmt.Errorf("original output mismatch")
		}
	}
	shape, _ := Shape(r.EventKind)
	for _, pos := range shape.SpendPositions {
		sid, e := s.SpentIn(ctx, shape.ParentType, base[pos].Bytes(), snap)
		if e != nil {
			return e
		}
		if sid != id {
			return fmt.Errorf("original spentIn mismatch")
		}
	}
	return nil
}
func (s *RPCSource) ClaimRegistered(ctx context.Context, h fr.Element, snap Snapshot) (bool, error) {
	v, e := s.call(ctx, "claimRegistered", snap, Big(h))
	if e != nil {
		return false, e
	}
	return v[0].(bool), nil
}
func (s *RPCSource) CheckIssuePolicy(ctx context.Context, ref fr.Element, snap Snapshot) error {
	v, e := s.call(ctx, "policyRecords", snap, Big(ref))
	if e != nil {
		return e
	}
	if len(v) != 9 {
		return fmt.Errorf("policy ABI mismatch")
	}
	if v[0].(uint64) == 0 || v[3].(uint8) != uint8(Issue) || v[4].(uint8) != 1 || v[5].(uint8) != 1 {
		return fmt.Errorf("wrong Issue Policy")
	}
	return nil
}
