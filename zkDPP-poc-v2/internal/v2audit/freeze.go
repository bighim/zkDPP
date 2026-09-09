package v2audit

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Snapshot struct {
	BlockNumber uint64 `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}

type Source interface {
	CheckSnapshot(context.Context, Snapshot) error
	Record(context.Context, uint64, Snapshot) (Record, error)
	Producer(context.Context, Ref, Snapshot) (uint64, error)
	SpentIn(context.Context, uint8, [32]byte, Snapshot) (uint64, error)
	Status(context.Context, uint8, [32]byte, Snapshot) (uint8, bool, error)
	Freeze(context.Context, uint8, [32]byte) error
	Checkpoint(context.Context) (Snapshot, error)
}

type Frontier struct {
	Ref        Ref
	SpendValue [32]byte
}

type Metrics struct {
	VisitedObjects, UniqueRecords, Decryptions int
	FreezeTransactions, SnapshotChecks         int
}

type Outcome string

const (
	TraceFailed          Outcome = "TRACE_FAILED"
	NoLiveTargets        Outcome = "NO_LIVE_TARGETS"
	CompleteAtCheckpoint Outcome = "COMPLETE_AT_CHECKPOINT"
	Incomplete           Outcome = "INCOMPLETE"
)

type Result struct {
	Outcome              Outcome
	Snapshot, Checkpoint Snapshot
	Targets              []Frontier
	Metrics              Metrics
	Reason               string `json:",omitempty"`
	TargetResults        []TargetResult
	Trace                TraceResult
}

type TargetResult struct {
	Target                                        Frontier
	Observation                                   Snapshot
	InitialStatus, FinalStatus                    uint8
	InitialSpent, FinalSpent                      bool
	Attempted, Confirmed, Blocked                 bool
	Error                                         string
	ObservationError, SubmissionError, FinalError string
	Transaction                                   *RPCTransaction
}

type Edge struct {
	Parent, Child Ref
	RecordID      uint64
}
type TraceResult struct {
	Frontier         []Frontier
	Claims, Exits    []Ref
	Graph            []Edge
	Metrics          Metrics
	Objects, Entries []Ref
	RecordIDs        []uint64
}

func Forward(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot) ([]Frontier, Metrics, error) {
	return forward(ctx, src, master, start, snap, nil)
}

func TraceForward(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot) (TraceResult, error) {
	var out TraceResult
	frontier, metrics, err := forward(ctx, src, master, start, snap, &out)
	out.Frontier, out.Metrics = frontier, metrics
	return out, err
}

func forward(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot, trace *TraceResult) ([]Frontier, Metrics, error) {
	var metrics Metrics
	if start.ObjectType != Note && start.ObjectType != Voucher && start.ObjectType != Claim {
		return nil, metrics, fmt.Errorf("start must be Note or Voucher")
	}
	if err := src.CheckSnapshot(ctx, snap); err != nil {
		return nil, metrics, err
	}
	metrics.SnapshotChecks++
	queue := []Ref{start}
	seenObjects := map[string]bool{}
	seenRecords := map[uint64]bool{}
	plaintextCache := map[uint64][]fr.Element{}
	decrypt := func(id uint64, record Record) ([]fr.Element, error) {
		if plain, ok := plaintextCache[id]; ok {
			return plain, nil
		}
		plain, e := auditcrypto.DecryptWithMasterKey(master, record.Ciphertext())
		if e != nil {
			return nil, e
		}
		plaintextCache[id] = plain
		if !seenRecords[id] {
			seenRecords[id] = true
			metrics.UniqueRecords++
			metrics.Decryptions++
		}
		if trace != nil {
			found := false
			for _, rid := range trace.RecordIDs {
				if rid == id {
					found = true
				}
			}
			if !found {
				trace.RecordIDs = append(trace.RecordIDs, id)
			}
		}
		return plain, nil
	}
	var frontier []Frontier
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if seenObjects[ref.Key()] {
			continue
		}
		seenObjects[ref.Key()] = true
		metrics.VisitedObjects++
		if trace != nil {
			trace.Objects = append(trace.Objects, ref)
		}
		producer, err := src.Producer(ctx, ref, snap)
		if err != nil || producer == 0 {
			return nil, metrics, fmt.Errorf("producer lookup: %w", err)
		}
		record, err := src.Record(ctx, producer, snap)
		if err != nil {
			return nil, metrics, err
		}
		if err = record.Validate(); err != nil {
			return nil, metrics, err
		}
		if trace != nil {
			found := false
			for _, id := range trace.RecordIDs {
				if id == producer {
					found = true
				}
			}
			if !found {
				trace.RecordIDs = append(trace.RecordIDs, producer)
			}
		}
		if _, err = record.Position(ref); err != nil {
			return nil, metrics, err
		}
		if ref.ObjectType == Claim {
			if record.EventKind != Issue {
				return nil, metrics, fmt.Errorf("Claim producer is not Issue")
			}
			if trace != nil {
				trace.Claims = append(trace.Claims, ref)
			}
			continue
		}
		position, err := record.FutureSpendPosition(ref)
		if err != nil {
			return nil, metrics, err
		}
		plain, err := decrypt(producer, record)
		if err != nil {
			return nil, metrics, err
		}
		spend := plain[len(record.EncryptedParents)+position]
		spendKey := spend.Bytes()
		consumer, err := src.SpentIn(ctx, ref.ObjectType, spendKey, snap)
		if err != nil {
			return nil, metrics, err
		}
		if consumer == 0 {
			frontier = append(frontier, Frontier{ref, spendKey})
			continue
		}
		child, err := src.Record(ctx, consumer, snap)
		if err != nil {
			return nil, metrics, err
		}
		if consumer <= producer {
			return nil, metrics, fmt.Errorf("noncausal consumer")
		}
		if err = child.Validate(); err != nil {
			return nil, metrics, err
		}
		shape, _ := Shape(child.EventKind)
		if shape.ParentType != ref.ObjectType {
			return nil, metrics, fmt.Errorf("consumer parent type mismatch")
		}
		childPlain, err := decrypt(consumer, child)
		if err != nil {
			return nil, metrics, err
		}
		matched := false
		for _, parent := range childPlain[:len(child.EncryptedParents)] {
			if parent.Equal(&ref.RawID) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, metrics, fmt.Errorf("consumer record does not contain parent")
		}
		if len(child.OutputRefs) == 0 {
			if child.EventKind != Exit {
				return nil, metrics, fmt.Errorf("unexpected terminal")
			}
			if trace != nil {
				trace.Exits = append(trace.Exits, ref)
			}
		}
		for _, childRef := range child.OutputRefs {
			if trace != nil {
				trace.Graph = append(trace.Graph, Edge{ref, childRef, consumer})
			}
		}
		queue = append(queue, child.OutputRefs...)
	}
	if err := src.CheckSnapshot(ctx, snap); err != nil {
		return nil, metrics, err
	}
	metrics.SnapshotChecks++
	sort.Slice(frontier, func(i, j int) bool { return frontier[i].Ref.Key() < frontier[j].Ref.Key() })
	return frontier, metrics, nil
}

func AuditAndFreeze(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot) Result {
	trace, err := TraceForward(ctx, src, master, start, snap)
	frontier := trace.Frontier
	result := Result{Snapshot: snap, Targets: frontier, Metrics: trace.Metrics, Trace: trace}
	if err != nil {
		result.Outcome, result.Reason = TraceFailed, err.Error()
		return result
	}
	if len(frontier) == 0 {
		result.Outcome = NoLiveTargets
		return result
	}
	result.TargetResults = make([]TargetResult, len(frontier))
	complete := true
	lowerBound := snap.BlockNumber
	for i, target := range frontier {
		row := &result.TargetResults[i]
		row.Target = target
		current, e := src.Checkpoint(ctx)
		if e != nil {
			row.Error = e.Error()
			complete = false
			continue
		}
		row.Observation = current
		if current.BlockNumber < snap.BlockNumber {
			row.Error = "observation predates snapshot"
			complete = false
			continue
		}
		if current.BlockNumber > lowerBound {
			lowerBound = current.BlockNumber
		}
		if e = src.CheckSnapshot(ctx, current); e != nil {
			row.Error = e.Error()
			complete = false
			continue
		}
		status, spent, e := src.Status(ctx, target.Ref.ObjectType, target.SpendValue, current)
		if e == nil {
			e = src.CheckSnapshot(ctx, current)
		}
		if e != nil {
			row.Error = e.Error()
			row.ObservationError = e.Error()
			complete = false
			continue
		}
		row.InitialStatus, row.InitialSpent = status, spent
		if spent {
			complete = false
		}
		if spent || status != Active {
			continue
		}
		row.Attempted = true
		e = src.Freeze(ctx, target.Ref.ObjectType, target.SpendValue)
		if detailed, ok := src.(interface{ LastTransaction() *RPCTransaction }); ok {
			row.Transaction = detailed.LastTransaction()
		}
		if row.Transaction != nil && row.Transaction.BlockNumber > lowerBound {
			lowerBound = row.Transaction.BlockNumber
		}
		if e != nil {
			row.Error = e.Error()
			row.SubmissionError = e.Error()
			if row.Transaction == nil || row.Transaction.State == "UNCERTAIN" {
				complete = false
			}
			continue
		}
		row.Confirmed = true
		result.Metrics.FreezeTransactions++
	}
	checkpoint, err := src.Checkpoint(ctx)
	if err != nil {
		result.Outcome, result.Reason = Incomplete, err.Error()
		return result
	}
	result.Checkpoint = checkpoint
	if checkpoint.BlockNumber < lowerBound {
		result.Outcome, result.Reason = Incomplete, "checkpoint predates observation"
		return result
	}
	if err = src.CheckSnapshot(ctx, checkpoint); err != nil {
		result.Outcome, result.Reason = Incomplete, err.Error()
		return result
	}
	for i, target := range frontier {
		status, spent, e := src.Status(ctx, target.Ref.ObjectType, target.SpendValue, checkpoint)
		row := &result.TargetResults[i]
		if row.Transaction != nil && row.Transaction.State == "CONFIRMED" {
			if validator, ok := src.(interface {
				ValidateReceipt(context.Context, RPCTransaction, Snapshot) error
			}); ok {
				if e := validator.ValidateReceipt(ctx, *row.Transaction, checkpoint); e != nil {
					complete = false
					row.Error = e.Error()
				}
			}
		}
		row.FinalStatus, row.FinalSpent = status, spent
		row.Blocked = e == nil && !spent && (status == Frozen || status == Revoked)
		if e != nil {
			row.Error = e.Error()
			row.FinalError = e.Error()
		}
		if !row.Blocked {
			complete = false
		}
	}
	if err = src.CheckSnapshot(ctx, checkpoint); err != nil {
		result.Outcome, result.Reason = Incomplete, err.Error()
		return result
	}
	if complete {
		result.Outcome = CompleteAtCheckpoint
	} else {
		result.Outcome = Incomplete
	}
	return result
}
