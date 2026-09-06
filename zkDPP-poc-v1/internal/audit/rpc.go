package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strings"
)

type Deployment struct {
	ChainID           uint64
	Ledger            string
	DeploymentBlock   uint64
	LedgerCodeSHA256  string
	PublicKeyChecksum string
}
type RPCSource struct {
	Client     *ethclient.Client
	ABI        abi.ABI
	Deployment Deployment
	Requests   int
}

func CodeHash(code []byte) string { h := sha256.Sum256(code); return hex.EncodeToString(h[:]) }
func (s *RPCSource) Check(ctx context.Context, snap Snapshot) error {
	s.Requests++
	id, e := s.Client.ChainID(ctx)
	if e != nil {
		return e
	}
	if !id.IsUint64() || id.Uint64() != s.Deployment.ChainID {
		return fmt.Errorf("chain mismatch")
	}
	s.Requests++
	h, e := s.Client.HeaderByNumber(ctx, new(big.Int).SetUint64(snap.Number))
	if e != nil {
		return e
	}
	if !strings.EqualFold(h.Hash().Hex(), snap.Hash) || snap.Number < s.Deployment.DeploymentBlock {
		return fmt.Errorf("snapshot changed or predates deployment")
	}
	s.Requests++
	code, e := s.Client.CodeAt(ctx, common.HexToAddress(s.Deployment.Ledger), new(big.Int).SetUint64(snap.Number))
	if e != nil {
		return e
	}
	if len(code) == 0 || CodeHash(code) != s.Deployment.LedgerCodeSHA256 {
		return fmt.Errorf("deployment code mismatch")
	}
	return nil
}
func (s *RPCSource) view(ctx context.Context, snap Snapshot, name string, args ...any) ([]any, error) {
	data, e := s.ABI.Pack(name, args...)
	if e != nil {
		return nil, e
	}
	addr := common.HexToAddress(s.Deployment.Ledger)
	s.Requests++
	b, e := s.Client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, new(big.Int).SetUint64(snap.Number))
	if e != nil {
		return nil, e
	}
	return s.ABI.Unpack(name, b)
}
func (s *RPCSource) Producer(ctx context.Context, r Ref, snap Snapshot) (uint64, error) {
	if r.ObjectType != Note && r.ObjectType != Voucher {
		return 0, fmt.Errorf("invalid object type")
	}
	v, e := s.view(ctx, snap, "producerOf", r.ObjectType, Big(r.RawID))
	if e != nil {
		return 0, e
	}
	return idValue(v)
}
func (s *RPCSource) Spent(ctx context.Context, t uint8, v fr.Element, snap Snapshot) (uint64, error) {
	name := "noteSpentIn"
	if t == Voucher {
		name = "voucherSpentIn"
	} else if t != Note {
		return 0, fmt.Errorf("invalid object type")
	}
	x, e := s.view(ctx, snap, name, Big(v))
	if e != nil {
		return 0, e
	}
	return idValue(x)
}
func idValue(v []any) (uint64, error) {
	if len(v) != 1 {
		return 0, fmt.Errorf("ID response shape")
	}
	n, ok := v[0].(*big.Int)
	if !ok || !n.IsUint64() {
		return 0, fmt.Errorf("ID outside POC range")
	}
	return n.Uint64(), nil
}

type wireRef struct {
	ObjectType uint8
	RawId      *big.Int
}
type wireRecord struct {
	EventKind                            uint8
	PolicyRef                            *big.Int
	OutputRefs                           []wireRef
	R1X, R1Y                             *big.Int
	EncryptedParents, EncryptedOutputNfs []*big.Int
}

func decodeRecord(v any) (Record, error) {
	w := abi.ConvertType(v, new(wireRecord)).(*wireRecord)
	p, e := Field(w.PolicyRef)
	if e != nil {
		return Record{}, e
	}
	x, e := Field(w.R1X)
	if e != nil {
		return Record{}, e
	}
	y, e := Field(w.R1Y)
	if e != nil {
		return Record{}, e
	}
	r := Record{EventKind: Kind(w.EventKind), PolicyRef: p, R1: auditcrypto.Point{X: x, Y: y}}
	r.EncryptedParents, e = Fields(w.EncryptedParents)
	if e != nil {
		return r, e
	}
	r.EncryptedOutputNfs, e = Fields(w.EncryptedOutputNfs)
	if e != nil {
		return r, e
	}
	for _, v := range w.OutputRefs {
		f, e := Field(v.RawId)
		if e != nil {
			return r, e
		}
		r.OutputRefs = append(r.OutputRefs, Ref{v.ObjectType, f})
	}
	return r, r.Validate()
}
func FlattenBase(values []any) ([]fr.Element, error) {
	var raw []*big.Int
	for _, v := range values {
		switch x := v.(type) {
		case *big.Int:
			raw = append(raw, x)
		case [2]*big.Int:
			raw = append(raw, x[:]...)
		case [3]*big.Int:
			raw = append(raw, x[:]...)
		default:
			return nil, fmt.Errorf("unsupported argument %T", v)
		}
	}
	return Fields(raw)
}
func (s *RPCSource) Load(ctx context.Context, id uint64, snap Snapshot) (Loaded, error) {
	if id == 0 {
		return Loaded{}, fmt.Errorf("missing record")
	}
	v, e := s.view(ctx, snap, "getAuditRecord", new(big.Int).SetUint64(id))
	if e != nil {
		return Loaded{}, e
	}
	if len(v) != 1 {
		return Loaded{}, fmt.Errorf("record response")
	}
	r, e := decodeRecord(v[0])
	if e != nil {
		return Loaded{}, e
	}
	ev := s.ABI.Events["AuditRecorded"]
	addr := common.HexToAddress(s.Deployment.Ledger)
	s.Requests++
	logs, e := s.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(s.Deployment.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.Number), Addresses: []common.Address{addr}, Topics: [][]common.Hash{{ev.ID}, {common.BigToHash(new(big.Int).SetUint64(id))}}})
	if e != nil {
		return Loaded{}, e
	}
	if len(logs) != 1 || logs[0].Removed {
		return Loaded{}, fmt.Errorf("missing/duplicate/removed original log")
	}
	log := logs[0]
	s.Requests++
	tx, pending, e := s.Client.TransactionByHash(ctx, log.TxHash)
	if e != nil {
		return Loaded{}, e
	}
	if pending || tx.To() == nil || *tx.To() != addr {
		return Loaded{}, fmt.Errorf("unsupported indirect/pending transaction")
	}
	s.Requests++
	receipt, e := s.Client.TransactionReceipt(ctx, log.TxHash)
	if e != nil {
		return Loaded{}, e
	}
	if receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockHash != log.BlockHash || receipt.BlockNumber.Uint64() > snap.Number {
		return Loaded{}, fmt.Errorf("invalid receipt")
	}
	found := false
	for _, l := range receipt.Logs {
		if l.Index == log.Index && l.Address == addr && len(l.Topics) == 2 && l.Topics[0] == ev.ID && l.Topics[1] == common.BigToHash(new(big.Int).SetUint64(id)) {
			found = true
		}
	}
	if !found {
		return Loaded{}, fmt.Errorf("receipt missing original audit event")
	}
	s.Requests++
	block, e := s.Client.HeaderByNumber(ctx, receipt.BlockNumber)
	if e != nil {
		return Loaded{}, e
	}
	if block.Hash() != receipt.BlockHash {
		return Loaded{}, fmt.Errorf("original block changed")
	}
	return DecodeOriginal(s.ABI, r, tx.Data())
}
func DecodeOriginal(contractABI abi.ABI, r Record, data []byte) (Loaded, error) {
	if e := r.Validate(); e != nil {
		return Loaded{}, e
	}
	if len(data) < 4 {
		return Loaded{}, fmt.Errorf("short calldata")
	}
	method, e := contractABI.MethodById(data[:4])
	if e != nil {
		return Loaded{}, e
	}
	if method.Name != r.EventKind.String() {
		return Loaded{}, fmt.Errorf("event selector mismatch")
	}
	args, e := method.Inputs.Unpack(data[4:])
	if e != nil {
		return Loaded{}, e
	}
	if len(args) < 3 {
		return Loaded{}, fmt.Errorf("argument shape")
	}
	p, e := FlattenBase(args[1 : len(args)-1])
	if e != nil {
		return Loaded{}, e
	}
	a := abi.ConvertType(args[len(args)-1], new(CipherArg)).(*CipherArg)
	fields := append([]*big.Int{a.R1X, a.R1Y}, a.EncryptedParents...)
	fields = append(fields, a.EncryptedOutputNfs...)
	values, e := Fields(fields)
	if e != nil {
		return Loaded{}, e
	}
	shape, e := Shape(r.EventKind)
	if e != nil {
		return Loaded{}, e
	}
	if len(a.EncryptedParents) != shape.ParentCount || len(a.EncryptedOutputNfs) != len(shape.OutputTypes) {
		return Loaded{}, fmt.Errorf("cipher array lengths")
	}
	ct := auditcrypto.Ciphertext{R1: auditcrypto.Point{X: values[0], Y: values[1]}, Data: values[2:]}
	expected, e := NewRecord(r.EventKind, p, ct)
	if e != nil {
		return Loaded{}, e
	}
	if !expected.PolicyRef.Equal(&r.PolicyRef) || !expected.R1.Equal(&r.R1) || !Equal(expected.Ciphertext().Data, r.Ciphertext().Data) {
		return Loaded{}, fmt.Errorf("cipher/metadata mismatch")
	}
	for i, v := range expected.OutputRefs {
		if v.Key() != r.OutputRefs[i].Key() {
			return Loaded{}, fmt.Errorf("output reference mismatch")
		}
	}
	l, e := Context(r.EventKind, p)
	return Loaded{r, p, l}, e
}
