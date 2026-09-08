package m7run

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"path/filepath"
	"strings"
	"time"
)

func Contract(root, source, name string) (abi.ABI, []byte, int, error) {
	var raw struct {
		ABI                        json.RawMessage
		Bytecode, DeployedBytecode struct{ Object string }
	}
	if e := Read(filepath.Join(root, "contracts/out", source, name+".json"), &raw); e != nil {
		return abi.ABI{}, nil, 0, e
	}
	a, e := abi.JSON(bytes.NewReader(raw.ABI))
	if e != nil {
		return a, nil, 0, e
	}
	code, e := hex.DecodeString(strings.TrimPrefix(raw.Bytecode.Object, "0x"))
	return a, code, len(strings.TrimPrefix(raw.DeployedBytecode.Object, "0x")) / 2, e
}
func CaseData(a abi.ABI, c FixedCase) ([]byte, error) {
	proof, p, record, e := c.Decode()
	if e != nil {
		return nil, e
	}
	args := []any{proof}
	if c.Kind == audit.Process {
		args = append(args, audit.Big(p[0]), audit.Big(p[1]), audit.Big(p[2]), [3]*big.Int{audit.Big(p[3]), audit.Big(p[4]), audit.Big(p[5])}, [2]*big.Int{audit.Big(p[6]), audit.Big(p[7])})
	} else {
		for _, v := range p {
			args = append(args, audit.Big(v))
		}
	}
	args = append(args, record.ABI())
	return a.Pack(c.Kind.String(), args...)
}
func View(ctx context.Context, c *ethclient.Client, a abi.ABI, to common.Address, name string, args ...any) ([]any, error) {
	data, e := a.Pack(name, args...)
	if e != nil {
		return nil, e
	}
	b, e := c.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if e != nil {
		return nil, e
	}
	return a.Unpack(name, b)
}

type Transaction struct {
	Name, Hash, ContractAddress, From  string
	BlockNumber, GasUsed               uint64
	CalldataBytes                      int
	PrepareMillis, SubmitReceiptMillis float64
	SSTORECount, SSTOREOpcodeGas       uint64
}

func Send(ctx context.Context, c *ethclient.Client, from common.Address, to *common.Address, data []byte, name string) (*types.Receipt, Transaction, error) {
	row := Transaction{Name: name, From: from.Hex(), CalldataBytes: len(data)}
	start := time.Now()
	gas, e := c.EstimateGas(ctx, ethereum.CallMsg{From: from, To: to, Data: data})
	if e != nil {
		return nil, row, e
	}
	if gas > 30000000 {
		return nil, row, fmt.Errorf("transaction exceeds 30M gas")
	}
	row.PrepareMillis = Millis(time.Since(start))
	tx := map[string]any{"from": from, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(gas)}
	if to != nil {
		tx["to"] = *to
	}
	start = time.Now()
	var hash common.Hash
	if e = c.Client().CallContext(ctx, &hash, "eth_sendTransaction", tx); e != nil {
		return nil, row, e
	}
	var receipt *types.Receipt
	for {
		receipt, e = c.TransactionReceipt(ctx, hash)
		if e == nil {
			break
		}
		if e != ethereum.NotFound {
			return nil, row, e
		}
		select {
		case <-ctx.Done():
			return nil, row, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	row.SubmitReceiptMillis = Millis(time.Since(start))
	row.Hash = hash.Hex()
	row.ContractAddress = receipt.ContractAddress.Hex()
	row.BlockNumber = receipt.BlockNumber.Uint64()
	row.GasUsed = receipt.GasUsed
	if receipt.Status != 1 {
		return receipt, row, fmt.Errorf("transaction reverted: %s", name)
	}
	return receipt, row, nil
}
func StoreTrace(ctx context.Context, c *ethclient.Client, hash string) (uint64, uint64, error) {
	var trace struct {
		StructLogs []struct {
			Op      string
			GasCost json.RawMessage
		}
	}
	if e := c.Client().CallContext(ctx, &trace, "debug_traceTransaction", common.HexToHash(hash), map[string]any{"disableStorage": true, "disableStack": true, "enableMemory": false}); e != nil {
		return 0, 0, e
	}
	if len(trace.StructLogs) == 0 {
		return 0, 0, fmt.Errorf("missing opcode trace")
	}
	var count, total uint64
	for _, l := range trace.StructLogs {
		if l.Op != "SSTORE" {
			continue
		}
		var n uint64
		if e := json.Unmarshal(l.GasCost, &n); e != nil {
			var s string
			if e = json.Unmarshal(l.GasCost, &s); e != nil {
				return 0, 0, e
			}
			v, ok := new(big.Int).SetString(s, 0)
			if !ok || !v.IsUint64() {
				return 0, 0, fmt.Errorf("bad gasCost")
			}
			n = v.Uint64()
		}
		count++
		total += n
	}
	return count, total, nil
}
