package m9audit

import (
	"context"
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const DPP uint8 = 3

type specialRecord struct {
	EventKind  uint8
	OutputRefs []audit.Ref
	R1         auditcrypto.Point
	Data       []fr.Element
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

func loadExit(ctx context.Context, src *audit.RPCSource, snap audit.Snapshot, id uint64) (specialRecord, []fr.Element, error) {
	addr := common.HexToAddress(src.Deployment.Ledger)
	data, e := src.ABI.Pack("getAuditRecord", new(big.Int).SetUint64(id))
	if e != nil {
		return specialRecord{}, nil, e
	}
	src.Requests++
	raw, e := src.Client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, new(big.Int).SetUint64(snap.Number))
	if e != nil {
		return specialRecord{}, nil, e
	}
	values, e := src.ABI.Unpack("getAuditRecord", raw)
	if e != nil || len(values) != 1 {
		return specialRecord{}, nil, fmt.Errorf("record response")
	}
	w := abi.ConvertType(values[0], new(wireRecord)).(*wireRecord)
	x, e := audit.Field(w.R1X)
	if e != nil {
		return specialRecord{}, nil, e
	}
	y, e := audit.Field(w.R1Y)
	if e != nil {
		return specialRecord{}, nil, e
	}
	r := specialRecord{EventKind: w.EventKind, R1: auditcrypto.Point{X: x, Y: y}}
	r.Data, e = audit.Fields(append(append([]*big.Int{}, w.EncryptedParents...), w.EncryptedOutputNfs...))
	if e != nil {
		return r, nil, e
	}
	for _, v := range w.OutputRefs {
		f, e := audit.Field(v.RawId)
		if e != nil {
			return r, nil, e
		}
		r.OutputRefs = append(r.OutputRefs, audit.Ref{ObjectType: v.ObjectType, RawID: f})
	}
	ev := src.ABI.Events["AuditRecorded"]
	src.Requests++
	logs, e := src.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(src.Deployment.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.Number), Addresses: []common.Address{addr}, Topics: [][]common.Hash{{ev.ID}, {common.BigToHash(new(big.Int).SetUint64(id))}}})
	if e != nil || len(logs) != 1 {
		return r, nil, fmt.Errorf("audit log")
	}
	src.Requests++
	tx, pending, e := src.Client.TransactionByHash(ctx, logs[0].TxHash)
	if e != nil || pending || tx.To() == nil || *tx.To() != addr {
		return r, nil, fmt.Errorf("exit transaction")
	}
	method, e := src.ABI.MethodById(tx.Data()[:4])
	if e != nil || method.Name != "exit" {
		return r, nil, fmt.Errorf("exit selector")
	}
	args, e := method.Inputs.Unpack(tx.Data()[4:])
	if e != nil || len(args) != 5 {
		return r, nil, fmt.Errorf("exit args")
	}
	base, e := audit.Fields([]*big.Int{args[1].(*big.Int), args[2].(*big.Int), args[3].(*big.Int)})
	if e != nil {
		return r, nil, e
	}
	cipher := abi.ConvertType(args[4], new(audit.CipherArg)).(*audit.CipherArg)
	if len(cipher.EncryptedParents) != 1 || len(cipher.EncryptedOutputNfs) != 0 || cipher.R1X.Cmp(w.R1X) != 0 || cipher.R1Y.Cmp(w.R1Y) != 0 || cipher.EncryptedParents[0].Cmp(w.EncryptedParents[0]) != 0 {
		return r, nil, fmt.Errorf("exit original mismatch")
	}
	dppEvent := src.ABI.Events["DPPFinalized"]
	src.Requests++
	dppLogs, e := src.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(src.Deployment.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.Number), Addresses: []common.Address{addr}, Topics: [][]common.Hash{{dppEvent.ID}, {common.BigToHash(args[3].(*big.Int))}, {common.BigToHash(new(big.Int).SetUint64(id))}}})
	if e != nil || len(dppLogs) != 1 {
		return r, nil, fmt.Errorf("DPPFinalized log")
	}
	return r, base, nil
}
