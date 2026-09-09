package v2audit

import (
	"context"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/big"
	"testing"
)

func TestHotfixClaimAndLiveSibling(t *testing.T) {
	pkg, shares, err := keyFixture()
	if err != nil {
		t.Fatal(err)
	}
	master, err := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	if err != nil {
		t.Fatal(err)
	}
	defer master.SetInt64(0)
	m := NewMemorySource(10, "test-block")
	a, b, c, h := Ref{Note, zkhash.Element(1)}, Ref{Note, zkhash.Element(2)}, Ref{Note, zkhash.Element(3)}, Ref{Claim, zkhash.Element(4)}
	nfa, nfb, nfc := zkhash.Element(101), zkhash.Element(102), zkhash.Element(103)
	add := func(k Kind, outputs []Ref, parents, future, spent []fr.Element, r int64) {
		ct, e := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, append(append([]fr.Element{}, parents...), future...), big.NewInt(r))
		if e != nil {
			t.Fatal(e)
		}
		rec := Record{EventKind: k, OutputRefs: outputs, R1: ct.R1, EncryptedParents: ct.Data[:len(parents)], EncryptedOutputNfs: ct.Data[len(parents):]}
		if k == Issue {
			rec.PolicyRef = zkhash.Element(123)
		}
		if _, e = m.Add(rec, Note, spent); e != nil {
			t.Fatal(e)
		}
	}
	add(Entry, []Ref{a}, nil, []fr.Element{nfa}, nil, 31)
	add(Split, []Ref{b, c}, []fr.Element{a.RawID}, []fr.Element{nfb, nfc}, []fr.Element{nfa}, 32)
	add(Issue, []Ref{h}, []fr.Element{b.RawID}, nil, []fr.Element{nfb}, 33)
	trace, err := TraceForward(context.Background(), m, master, a, m.Current)
	if err != nil || len(trace.Claims) != 1 || len(trace.Frontier) != 1 {
		t.Fatalf("trace=%+v error=%v", trace, err)
	}
	m.Statuses[typedSpend(Note, nfc.Bytes())] = Revoked
	result := AuditAndFreeze(context.Background(), m, master, a, m.Current)
	if result.Outcome != CompleteAtCheckpoint || m.FreezeCalls != 0 || len(result.TargetResults) != 1 || !result.TargetResults[0].Blocked {
		t.Fatalf("result=%+v", result)
	}
	onlyClaim := AuditAndFreeze(context.Background(), m, master, h, m.Current)
	if onlyClaim.Outcome != NoLiveTargets {
		t.Fatalf("claim outcome=%s", onlyClaim.Outcome)
	}
	back, err := TraceBackward(context.Background(), m, master, h, m.Current)
	if err != nil || len(back.Entries) != 1 || back.Entries[0].Key() != a.Key() {
		t.Fatalf("backward=%+v err=%v", back, err)
	}
}

func TestExitTerminalAndMalformedProducer(t *testing.T) {
	m, key, ref := singleTarget(t)
	defer key.SetInt64(0)
	pkg, _, err := keyFixture()
	if err != nil {
		t.Fatal(err)
	}
	ct, err := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, []fr.Element{ref.RawID}, big.NewInt(77))
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Add(Record{EventKind: Exit, R1: ct.R1, EncryptedParents: ct.Data}, Note, []fr.Element{zkhash.Element(55)})
	if err != nil {
		t.Fatal(err)
	}
	r, err := TraceForward(context.Background(), m, key, ref, m.Current)
	if err != nil || len(r.Exits) != 1 || len(r.Frontier) != 0 {
		t.Fatalf("terminal=%+v %v", r, err)
	}
	bad := m.Records[2]
	bad.EventKind = Split
	m.Records[2] = bad
	result := AuditAndFreeze(context.Background(), m, key, ref, m.Current)
	if result.Outcome != TraceFailed || m.FreezeCalls != 0 {
		t.Fatal("invalid consumer accepted")
	}
}
