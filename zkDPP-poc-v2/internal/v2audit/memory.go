package v2audit

import (
	"context"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

// MemorySource is the M1 local audit/indexing fixture. It is not an operating
// server; it mirrors immutable ledger relations for correctness and benchmarks.
type MemorySource struct {
	Current         Snapshot
	Records         map[uint64]Record
	Producers       map[string]uint64
	Spends          map[string]uint64
	Statuses        map[string]uint8
	NextID          uint64
	FreezeCalls     int
	FailLookups     bool
	ConsumeOnFreeze map[string]bool
}

func NewMemorySource(number uint64, hash string) *MemorySource {
	return &MemorySource{Current: Snapshot{number, hash}, Records: map[uint64]Record{}, Producers: map[string]uint64{}, Spends: map[string]uint64{}, Statuses: map[string]uint8{}, ConsumeOnFreeze: map[string]bool{}, NextID: 1}
}
func typedSpend(t uint8, value [32]byte) string { return fmt.Sprintf("%d:%x", t, value) }
func (m *MemorySource) Add(record Record, inputType uint8, spends []fr.Element) (uint64, error) {
	if err := record.Validate(); err != nil {
		return 0, err
	}
	id := m.NextID
	m.NextID++
	m.Records[id] = record
	for _, ref := range record.OutputRefs {
		if m.Producers[ref.Key()] != 0 {
			return 0, fmt.Errorf("duplicate output")
		}
		m.Producers[ref.Key()] = id
	}
	for _, spend := range spends {
		key := typedSpend(inputType, spend.Bytes())
		if m.Spends[key] != 0 {
			return 0, fmt.Errorf("duplicate spend")
		}
		m.Spends[key] = id
	}
	return id, nil
}
func (m *MemorySource) CheckSnapshot(_ context.Context, s Snapshot) error {
	if s != m.Current {
		return fmt.Errorf("snapshot mismatch")
	}
	return nil
}
func (m *MemorySource) Record(_ context.Context, id uint64, _ Snapshot) (Record, error) {
	r, ok := m.Records[id]
	if !ok {
		return Record{}, fmt.Errorf("record missing")
	}
	return r, nil
}
func (m *MemorySource) Producer(_ context.Context, r Ref, _ Snapshot) (uint64, error) {
	if m.FailLookups {
		return 0, fmt.Errorf("lookup failed")
	}
	return m.Producers[r.Key()], nil
}
func (m *MemorySource) SpentIn(_ context.Context, t uint8, v [32]byte, _ Snapshot) (uint64, error) {
	return m.Spends[typedSpend(t, v)], nil
}
func (m *MemorySource) Status(_ context.Context, t uint8, v [32]byte, _ Snapshot) (uint8, bool, error) {
	key := typedSpend(t, v)
	return m.Statuses[key], m.Spends[key] != 0, nil
}
func (m *MemorySource) Freeze(_ context.Context, t uint8, v [32]byte) error {
	key := typedSpend(t, v)
	if m.ConsumeOnFreeze[key] {
		m.Spends[key] = m.NextID
		return fmt.Errorf("consumed after snapshot")
	}
	if m.Spends[key] != 0 || m.Statuses[key] != Active {
		return fmt.Errorf("not active and unspent")
	}
	m.Statuses[key] = Frozen
	m.FreezeCalls++
	return nil
}
func (m *MemorySource) Checkpoint(_ context.Context) (Snapshot, error) { return m.Current, nil }
