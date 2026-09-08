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
}

func Forward(ctx context.Context, src Source, master *big.Int, start Ref, snap Snapshot) ([]Frontier, Metrics, error) {
	var metrics Metrics
	if start.ObjectType != Note && start.ObjectType != Voucher {
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
	frontier, metrics, err := Forward(ctx, src, master, start, snap)
	result := Result{Snapshot: snap, Targets: frontier, Metrics: metrics}
	if err != nil {
		result.Outcome, result.Reason = TraceFailed, err.Error()
		return result
	}
	if len(frontier) == 0 {
		result.Outcome = NoLiveTargets
		return result
	}
	for _, target := range frontier {
		status, spent, e := src.Status(ctx, target.Ref.ObjectType, target.SpendValue, snap)
		if e != nil {
			result.Outcome, result.Reason = Incomplete, e.Error()
			return result
		}
		if spent || status != Active {
			continue
		}
		if e = src.Freeze(ctx, target.Ref.ObjectType, target.SpendValue); e != nil {
			continue
		}
		result.Metrics.FreezeTransactions++
	}
	checkpoint, err := src.Checkpoint(ctx)
	if err != nil {
		result.Outcome, result.Reason = Incomplete, err.Error()
		return result
	}
	result.Checkpoint = checkpoint
	complete := true
	for _, target := range frontier {
		status, spent, e := src.Status(ctx, target.Ref.ObjectType, target.SpendValue, checkpoint)
		if e != nil || spent || status != Frozen {
			complete = false
		}
	}
	if complete {
		result.Outcome = CompleteAtCheckpoint
	} else {
		result.Outcome = Incomplete
	}
	return result
}
