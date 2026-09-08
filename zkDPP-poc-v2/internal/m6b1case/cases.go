package m6b1case

import (
	"fmt"
	auditencryption "github.com/bighim/zkDPP/zkDPP-poc-v2/features/audit_encryption"
	auditprocess "github.com/bighim/zkDPP/zkDPP-poc-v2/features/audit_process_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m5case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/big"
	"time"
)

type Scenario struct {
	Base          *m5case.Scenario
	Message       []fr.Element
	Context       fr.Element
	Ciphertext    auditcrypto.Ciphertext
	Core          *auditencryption.Circuit
	Process       *auditprocess.Circuit
	Inputs        []fr.Element
	EncryptMillis float64
}

func Context(inputs []fr.Element) (fr.Element, error) {
	if len(inputs) != 8 {
		return fr.Element{}, fmt.Errorf("Process context requires eight public fields")
	}
	v := []fr.Element{auditcrypto.AuditContextTag, zkhash.Element(policy.EventProcess)}
	return zkhash.Hash(append(v, inputs...)...), nil
}
func Build(root string, pk auditcrypto.PublicKey, r *big.Int) (*Scenario, error) {
	s, err := m5case.Build(root)
	if err != nil {
		return nil, err
	}
	sk := s.Assignment.SKOwner.(fr.Element)
	in := []fr.Element{s.PolicyRef, s.ScopeRef, s.InputRoot}
	m := make([]fr.Element, 5)
	for i := 0; i < 3; i++ {
		in = append(in, note.Nullifier(sk, s.Inputs[i].Commitment))
		m[i] = s.Inputs[i].Commitment
	}
	for i := 0; i < 2; i++ {
		in = append(in, s.Outputs[i].Commitment)
		m[3+i] = note.Nullifier(sk, s.Outputs[i].Commitment)
	}
	l, err := Context(in)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	ct, err := auditcrypto.Encrypt(pk, l, m, r)
	elapsed := time.Since(start)
	if err != nil {
		return nil, err
	}
	all := append([]fr.Element{}, in...)
	all = append(all, ct.R1.X, ct.R1.Y)
	all = append(all, ct.Data...)
	return &Scenario{Base: s, Message: m, Context: l, Ciphertext: ct, Core: auditencryption.Assignment(pk, l, m, r, ct), Process: auditprocess.Assignment(pk, s.Assignment, r, ct), Inputs: all, EncryptMillis: float64(elapsed.Nanoseconds()) / 1e6}, nil
}
func FromRecord(inputs []fr.Element) (fr.Element, auditcrypto.Ciphertext, error) {
	if len(inputs) != 15 {
		return fr.Element{}, auditcrypto.Ciphertext{}, fmt.Errorf("record must contain fifteen fields")
	}
	l, err := Context(inputs[:8])
	if err != nil {
		return l, auditcrypto.Ciphertext{}, err
	}
	ct := auditcrypto.Ciphertext{R1: auditcrypto.Point{X: inputs[8], Y: inputs[9]}, Data: append([]fr.Element{}, inputs[10:]...)}
	if err = auditcrypto.ValidatePoint(ct.R1); err != nil {
		return fr.Element{}, auditcrypto.Ciphertext{}, err
	}
	return l, ct, nil
}
func Equal(a, b []fr.Element) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(&b[i]) {
			return false
		}
	}
	return true
}

func ExpectedMessage(root string) ([]fr.Element, error) {
	s, err := m5case.Build(root)
	if err != nil {
		return nil, err
	}
	sk := s.Assignment.SKOwner.(fr.Element)
	m := []fr.Element{s.Inputs[0].Commitment, s.Inputs[1].Commitment, s.Inputs[2].Commitment}
	for _, out := range s.Outputs {
		m = append(m, note.Nullifier(sk, out.Commitment))
	}
	return m, nil
}
