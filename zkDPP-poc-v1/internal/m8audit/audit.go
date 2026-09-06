package m8audit

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m8case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const DPP uint8 = 3

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
type specialRecord struct {
	EventKind  uint8
	PolicyRef  fr.Element
	OutputRefs []audit.Ref
	Ciphertext auditcrypto.Ciphertext
}
type Metrics struct {
	Records, Decryptions, Responses, RPCRequests                                           int
	LookupMillis, Member1Millis, Member2Millis, CombineMillis, UpstreamMillis, TotalMillis float64
}
type Result struct {
	Snapshot                    audit.Snapshot
	IssueRecordID, ExitRecordID uint64
	DPPCommitment, ParentCM     string
	Upstream                    audit.TraceResult
	Metrics                     Metrics
}

func TraceClaim(ctx context.Context, src *audit.RPCSource, committee auditcrypto.Committee, dppCommitment, policyRef fr.Element, snap audit.Snapshot) (out Result, err error) {
	out.Snapshot = snap
	out.DPPCommitment = dppCommitment.String()
	start := time.Now()
	defer func() {
		out.Metrics.TotalMillis = ms(time.Since(start))
		out.Metrics.RPCRequests = src.Requests
		out.Metrics.LookupMillis = out.Metrics.TotalMillis - out.Metrics.Member1Millis - out.Metrics.Member2Millis - out.Metrics.CombineMillis - out.Metrics.UpstreamMillis
	}()
	if err = src.Check(ctx, snap); err != nil {
		return out, err
	}
	issueID, err := viewID(ctx, src, snap, "claimRecordOf", audit.Big(dppCommitment), audit.Big(policyRef))
	if err != nil || issueID == 0 {
		if err == nil {
			err = fmt.Errorf("missing Claim")
		}
		return out, err
	}
	out.IssueRecordID = issueID
	issue, issueBase, err := loadSpecial(ctx, src, snap, issueID)
	if err != nil {
		return out, err
	}
	if issue.EventKind != 8 || !issue.PolicyRef.Equal(&policyRef) || len(issueBase) != 2 || !issueBase[0].Equal(&policyRef) || !issueBase[1].Equal(&dppCommitment) || len(issue.OutputRefs) != 0 || len(issue.Ciphertext.Data) != 0 || !issue.Ciphertext.R1.X.IsZero() || !issue.Ciphertext.R1.Y.IsZero() {
		return out, fmt.Errorf("invalid Issue original")
	}
	if err = checkEvent(ctx, src, snap, "ClaimIssued", []common.Hash{common.BigToHash(audit.Big(dppCommitment)), common.BigToHash(audit.Big(policyRef)), common.BigToHash(new(big.Int).SetUint64(issueID))}); err != nil {
		return out, err
	}
	exitID, err := viewID(ctx, src, snap, "producerOf", DPP, audit.Big(dppCommitment))
	if err != nil || exitID == 0 {
		if err == nil {
			err = fmt.Errorf("missing DPP producer")
		}
		return out, err
	}
	out.ExitRecordID = exitID
	exit, base, err := loadSpecial(ctx, src, snap, exitID)
	if err != nil {
		return out, err
	}
	if exit.EventKind != 7 || len(base) != 3 || !base[2].Equal(&dppCommitment) || len(exit.OutputRefs) != 1 || exit.OutputRefs[0].ObjectType != DPP || !exit.OutputRefs[0].RawID.Equal(&dppCommitment) || len(exit.Ciphertext.Data) != 1 {
		return out, fmt.Errorf("invalid Exit original")
	}
	if err = checkEvent(ctx, src, snap, "DPPFinalized", []common.Hash{common.BigToHash(audit.Big(dppCommitment)), common.BigToHash(new(big.Int).SetUint64(exitID))}); err != nil {
		return out, err
	}
	contextValue := zkhash.Hash(auditcrypto.AuditContextTag, zkhash.Element(m8case.EventExit), base[0], base[1], base[2])
	partials := make([]auditcrypto.Partial, 2)
	for i := 0; i < 2; i++ {
		t := time.Now()
		partials[i], err = auditcrypto.PartialDecrypt(committee.Shares[i], exit.Ciphertext.R1)
		elapsed := ms(time.Since(t))
		if i == 0 {
			out.Metrics.Member1Millis = elapsed
		} else {
			out.Metrics.Member2Millis = elapsed
		}
		if err != nil {
			return out, err
		}
		out.Metrics.Responses++
	}
	t := time.Now()
	plain, err := auditcrypto.CombineAndDecrypt(contextValue, exit.Ciphertext, partials)
	out.Metrics.CombineMillis = ms(time.Since(t))
	if err != nil {
		return out, err
	}
	if len(plain) != 1 {
		return out, fmt.Errorf("Exit plaintext length")
	}
	out.Metrics.Decryptions++
	out.ParentCM = plain[0].String()
	t = time.Now()
	up, err := audit.Trace(ctx, src, committee, "backward", audit.Ref{ObjectType: audit.Note, RawID: plain[0]}, snap)
	out.Metrics.UpstreamMillis = ms(time.Since(t))
	if err != nil {
		return out, err
	}
	out.Upstream = up
	out.Metrics.Records = 2 + up.Metrics.UniqueRecords
	out.Metrics.Decryptions += up.Metrics.Decryptions
	out.Metrics.Responses += up.Metrics.Responses
	return out, nil
}

func viewID(ctx context.Context, src *audit.RPCSource, snap audit.Snapshot, name string, args ...any) (uint64, error) {
	data, err := src.ABI.Pack(name, args...)
	if err != nil {
		return 0, err
	}
	addr := common.HexToAddress(src.Deployment.Ledger)
	src.Requests++
	b, err := src.Client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, new(big.Int).SetUint64(snap.Number))
	if err != nil {
		return 0, err
	}
	v, err := src.ABI.Unpack(name, b)
	if err != nil || len(v) != 1 {
		return 0, fmt.Errorf("%s response: %w", name, err)
	}
	n, ok := v[0].(*big.Int)
	if !ok || !n.IsUint64() {
		return 0, fmt.Errorf("%s ID", name)
	}
	return n.Uint64(), nil
}

func checkEvent(ctx context.Context, src *audit.RPCSource, snap audit.Snapshot, name string, indexed []common.Hash) error {
	ev, ok := src.ABI.Events[name]
	if !ok {
		return fmt.Errorf("missing %s ABI", name)
	}
	topics := [][]common.Hash{{ev.ID}}
	for _, v := range indexed {
		topics = append(topics, []common.Hash{v})
	}
	addr := common.HexToAddress(src.Deployment.Ledger)
	src.Requests++
	logs, err := src.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(src.Deployment.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.Number), Addresses: []common.Address{addr}, Topics: topics})
	if err != nil {
		return err
	}
	if len(logs) != 1 || logs[0].Removed {
		return fmt.Errorf("missing or duplicate %s", name)
	}
	return nil
}

func loadSpecial(ctx context.Context, src *audit.RPCSource, snap audit.Snapshot, id uint64) (specialRecord, []fr.Element, error) {
	data, err := src.ABI.Pack("getAuditRecord", new(big.Int).SetUint64(id))
	if err != nil {
		return specialRecord{}, nil, err
	}
	addr := common.HexToAddress(src.Deployment.Ledger)
	src.Requests++
	raw, err := src.Client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, new(big.Int).SetUint64(snap.Number))
	if err != nil {
		return specialRecord{}, nil, err
	}
	values, err := src.ABI.Unpack("getAuditRecord", raw)
	if err != nil || len(values) != 1 {
		return specialRecord{}, nil, fmt.Errorf("record response: %w", err)
	}
	w := abi.ConvertType(values[0], new(wireRecord)).(*wireRecord)
	record := specialRecord{EventKind: w.EventKind}
	if record.PolicyRef, err = audit.Field(w.PolicyRef); err != nil {
		return record, nil, err
	}
	x, err := audit.Field(w.R1X)
	if err != nil {
		return record, nil, err
	}
	y, err := audit.Field(w.R1Y)
	if err != nil {
		return record, nil, err
	}
	record.Ciphertext.R1 = auditcrypto.Point{X: x, Y: y}
	record.Ciphertext.Data, err = audit.Fields(append(append([]*big.Int{}, w.EncryptedParents...), w.EncryptedOutputNfs...))
	if err != nil {
		return record, nil, err
	}
	for _, r := range w.OutputRefs {
		f, e := audit.Field(r.RawId)
		if e != nil {
			return record, nil, e
		}
		record.OutputRefs = append(record.OutputRefs, audit.Ref{ObjectType: r.ObjectType, RawID: f})
	}
	ev := src.ABI.Events["AuditRecorded"]
	src.Requests++
	logs, err := src.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(src.Deployment.DeploymentBlock), ToBlock: new(big.Int).SetUint64(snap.Number), Addresses: []common.Address{addr}, Topics: [][]common.Hash{{ev.ID}, {common.BigToHash(new(big.Int).SetUint64(id))}}})
	if err != nil || len(logs) != 1 {
		return record, nil, fmt.Errorf("audit log: %w", err)
	}
	log := logs[0]
	src.Requests++
	tx, pending, err := src.Client.TransactionByHash(ctx, log.TxHash)
	if err != nil || pending || tx.To() == nil || *tx.To() != addr {
		return record, nil, fmt.Errorf("original transaction")
	}
	src.Requests++
	receipt, err := src.Client.TransactionReceipt(ctx, log.TxHash)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber.Uint64() > snap.Number {
		return record, nil, fmt.Errorf("original receipt")
	}
	method, err := src.ABI.MethodById(tx.Data()[:4])
	if err != nil {
		return record, nil, err
	}
	args, err := method.Inputs.Unpack(tx.Data()[4:])
	if err != nil {
		return record, nil, err
	}
	if record.EventKind == 8 {
		if method.Name != "issue" || len(args) != 3 {
			return record, nil, fmt.Errorf("Issue selector")
		}
		p, err := audit.Fields([]*big.Int{args[1].(*big.Int), args[2].(*big.Int)})
		return record, p, err
	}
	if record.EventKind != 7 || method.Name != "exit" || len(args) != 5 {
		return record, nil, fmt.Errorf("Exit selector")
	}
	base, err := audit.Fields([]*big.Int{args[1].(*big.Int), args[2].(*big.Int), args[3].(*big.Int)})
	if err != nil {
		return record, nil, err
	}
	cipher := abi.ConvertType(args[4], new(audit.CipherArg)).(*audit.CipherArg)
	if len(cipher.EncryptedParents) != 1 || len(cipher.EncryptedOutputNfs) != 0 {
		return record, nil, fmt.Errorf("Exit cipher shape")
	}
	if cipher.R1X.Cmp(w.R1X) != 0 || cipher.R1Y.Cmp(w.R1Y) != 0 || cipher.EncryptedParents[0].Cmp(w.EncryptedParents[0]) != 0 {
		return record, nil, fmt.Errorf("Exit storage/calldata mismatch")
	}
	return record, base, nil
}

func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func ParseABI(value string) (abi.ABI, error) { return abi.JSON(strings.NewReader(value)) }
