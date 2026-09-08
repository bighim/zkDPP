package v2audit

import (
	"context"
	"fmt"
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
}

type RPCSource struct {
	Client             *ethclient.Client
	ABI                abi.ABI
	Ledger             common.Address
	StatusAuthority    common.Address
	FreezeTransactions []RPCTransaction
}

func NewRPCSource(client *ethclient.Client, contractABI abi.ABI, ledger, statusAuthority common.Address) *RPCSource {
	return &RPCSource{Client: client, ABI: contractABI, Ledger: ledger, StatusAuthority: statusAuthority}
}

func (s *RPCSource) CheckSnapshot(ctx context.Context, snapshot Snapshot) error {
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
	header, err := s.Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash().Hex()}, nil
}
func (s *RPCSource) call(ctx context.Context, method string, block uint64, args ...any) ([]any, error) {
	data, err := s.ABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	raw, err := s.Client.CallContract(ctx, ethereum.CallMsg{To: &s.Ledger, Data: data}, new(big.Int).SetUint64(block))
	if err != nil {
		return nil, err
	}
	return s.ABI.Unpack(method, raw)
}
func (s *RPCSource) uint(ctx context.Context, method string, block uint64, args ...any) (*big.Int, error) {
	values, err := s.call(ctx, method, block, args...)
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
	v, err := s.uint(ctx, "producerOf", snapshot.BlockNumber, ref.ObjectType, Big(ref.RawID))
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
	v, err := s.uint(ctx, method, snapshot.BlockNumber, new(big.Int).SetBytes(spend[:]))
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
	values, err := s.call(ctx, method, snapshot.BlockNumber, new(big.Int).SetBytes(spend[:]))
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
	values, err := s.call(ctx, "getAuditRecord", snapshot.BlockNumber, new(big.Int).SetUint64(id))
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
	return result, result.Validate()
}

func (s *RPCSource) Freeze(ctx context.Context, objectType uint8, spend [32]byte) error {
	data, err := s.ABI.Pack("setStatus", objectType, new(big.Int).SetBytes(spend[:]), uint8(Frozen))
	if err != nil {
		return err
	}
	gas, err := s.Client.EstimateGas(ctx, ethereum.CallMsg{From: s.StatusAuthority, To: &s.Ledger, Data: data})
	if err != nil {
		return err
	}
	tx := map[string]any{"from": s.StatusAuthority, "to": s.Ledger, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(gas)}
	var hash common.Hash
	if err = s.Client.Client().CallContext(ctx, &hash, "eth_sendTransaction", tx); err != nil {
		return err
	}
	var receipt *types.Receipt
	for {
		receipt, err = s.Client.TransactionReceipt(ctx, hash)
		if err == nil {
			break
		}
		if err != ethereum.NotFound {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	if receipt.Status != 1 {
		return fmt.Errorf("Freeze transaction reverted")
	}
	s.FreezeTransactions = append(s.FreezeTransactions, RPCTransaction{Hash: hash.Hex(), BlockNumber: receipt.BlockNumber.Uint64(), GasUsed: receipt.GasUsed, CalldataBytes: len(data)})
	return nil
}
