package v2audit

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type memorySource struct {
	snapshot     Snapshot
	records      map[uint64]Record
	producer     map[string]uint64
	spent        map[string]uint64
	status       map[string]uint8
	freezeCalls  int
	failProducer bool
}

func spendKey(t uint8, v [32]byte) string { return fmt.Sprintf("%d:%x", t, v) }
func (m *memorySource) CheckSnapshot(_ context.Context, s Snapshot) error {
	if s != m.snapshot {
		return fmt.Errorf("snapshot mismatch")
	}
	return nil
}
func (m *memorySource) Record(_ context.Context, id uint64, _ Snapshot) (Record, error) {
	r, ok := m.records[id]
	if !ok {
		return Record{}, fmt.Errorf("record missing")
	}
	return r, nil
}
func (m *memorySource) Producer(_ context.Context, r Ref, _ Snapshot) (uint64, error) {
	if m.failProducer {
		return 0, fmt.Errorf("forced lookup failure")
	}
	return m.producer[r.Key()], nil
}
func (m *memorySource) SpentIn(_ context.Context, t uint8, v [32]byte, _ Snapshot) (uint64, error) {
	return m.spent[spendKey(t, v)], nil
}
func (m *memorySource) Status(_ context.Context, t uint8, v [32]byte, _ Snapshot) (uint8, bool, error) {
	return m.status[spendKey(t, v)], m.spent[spendKey(t, v)] != 0, nil
}
func (m *memorySource) Freeze(_ context.Context, t uint8, v [32]byte) error {
	k := spendKey(t, v)
	if m.spent[k] != 0 || m.status[k] != Active {
		return fmt.Errorf("not live")
	}
	m.status[k] = Frozen
	m.freezeCalls++
	return nil
}
func (m *memorySource) Checkpoint(_ context.Context) (Snapshot, error) { return m.snapshot, nil }

func TestAuditAndFreezeEntireFrontier(t *testing.T) {
	pkg, shares, err := keyFixture()
	if err != nil {
		t.Fatal(err)
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[1]})
	if err != nil {
		t.Fatal(err)
	}
	defer master.SetInt64(0)
	refs := func(values ...uint64) []Ref {
		out := make([]Ref, len(values))
		for i, v := range values {
			out[i] = Ref{Note, zkhash.Element(v)}
		}
		return out
	}
	a, b, c, d, e := refs(1, 2, 3, 4, 5)[0], refs(1, 2, 3, 4, 5)[1], refs(1, 2, 3, 4, 5)[2], refs(1, 2, 3, 4, 5)[3], refs(1, 2, 3, 4, 5)[4]
	spends := []fr.Element{zkhash.Element(101), zkhash.Element(102), zkhash.Element(103), zkhash.Element(104), zkhash.Element(105)}
	record := func(kind Kind, outputs []Ref, parents, future []fr.Element, r int64) Record {
		msg := append(append([]fr.Element{}, parents...), future...)
		ct, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, msg, big.NewInt(r))
		if err != nil {
			t.Fatal(err)
		}
		return Record{EventKind: kind, OutputRefs: outputs, R1: ct.R1, EncryptedParents: ct.Data[:len(parents)], EncryptedOutputNfs: ct.Data[len(parents):]}
	}
	m := &memorySource{snapshot: Snapshot{77, "0xabc"}, records: map[uint64]Record{}, producer: map[string]uint64{}, spent: map[string]uint64{}, status: map[string]uint8{}}
	m.records[1] = record(Entry, []Ref{a}, nil, []fr.Element{spends[0]}, 11)
	m.records[2] = record(Split, []Ref{b, c}, []fr.Element{a.RawID}, []fr.Element{spends[1], spends[2]}, 12)
	m.records[3] = record(Split, []Ref{d, e}, []fr.Element{b.RawID}, []fr.Element{spends[3], spends[4]}, 13)
	for _, x := range []struct {
		r  Ref
		id uint64
	}{{a, 1}, {b, 2}, {c, 2}, {d, 3}, {e, 3}} {
		m.producer[x.r.Key()] = x.id
	}
	m.spent[spendKey(Note, spends[0].Bytes())] = 2
	m.spent[spendKey(Note, spends[1].Bytes())] = 3
	result := AuditAndFreeze(context.Background(), m, master, a, m.snapshot)
	if result.Outcome != CompleteAtCheckpoint || len(result.Targets) != 3 || result.Metrics.FreezeTransactions != 3 {
		t.Fatalf("result=%+v", result)
	}
	if m.freezeCalls != 3 {
		t.Fatalf("freeze calls=%d", m.freezeCalls)
	}
}

func TestTraceFailureNeverFreezes(t *testing.T) {
	pkg, shares, err := keyFixture()
	if err != nil {
		t.Fatal(err)
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[2]})
	if err != nil {
		t.Fatal(err)
	}
	defer master.SetInt64(0)
	m := &memorySource{snapshot: Snapshot{1, "0x1"}, records: map[uint64]Record{}, producer: map[string]uint64{}, spent: map[string]uint64{}, status: map[string]uint8{}, failProducer: true}
	result := AuditAndFreeze(context.Background(), m, master, Ref{Note, zkhash.Element(1)}, m.snapshot)
	if result.Outcome != TraceFailed || m.freezeCalls != 0 {
		t.Fatalf("result=%+v calls=%d", result, m.freezeCalls)
	}
}

func TestPostSnapshotConsumptionIsIncomplete(t *testing.T) {
	pkg, shares, err := keyFixture()
	if err != nil {
		t.Fatal(err)
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, []auditcrypto.Share{shares[0], shares[1]})
	if err != nil {
		t.Fatal(err)
	}
	defer master.SetInt64(0)
	ref := Ref{Note, zkhash.Element(8)}
	spend := zkhash.Element(88)
	ct, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{spend}, big.NewInt(9))
	if err != nil {
		t.Fatal(err)
	}
	m := NewMemorySource(8, "0x8")
	record := Record{EventKind: Entry, OutputRefs: []Ref{ref}, R1: ct.R1, EncryptedOutputNfs: ct.Data}
	if _, err = m.Add(record, Note, nil); err != nil {
		t.Fatal(err)
	}
	m.ConsumeOnFreeze[typedSpend(Note, spend.Bytes())] = true
	result := AuditAndFreeze(context.Background(), m, master, ref, m.Current)
	if result.Outcome != Incomplete {
		t.Fatalf("outcome=%s", result.Outcome)
	}
}

func keyFixture() (auditcrypto.ExternalKeyPackage, [3]auditcrypto.Share, error) {
	x, a := big.NewInt(37), big.NewInt(19)
	q := auditcrypto.Order()
	var shares [3]auditcrypto.Share
	for i := range shares {
		v := new(big.Int).Mul(a, big.NewInt(int64(i+1)))
		v.Add(v, x).Mod(v, q)
		shares[i] = auditcrypto.Share{ID: uint8(i + 1), Value: v}
	}
	g := auditcrypto.Generator()
	var p auditcrypto.Point
	p.ScalarMultiplication(&g, x)
	pkg, err := auditcrypto.PackageFromShares("freeze-test", auditcrypto.PublicKey{Point: p}, shares)
	return pkg, shares, err
}
