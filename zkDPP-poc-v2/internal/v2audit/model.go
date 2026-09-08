package v2audit

import (
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Kind uint8

const (
	Entry Kind = iota
	Transfer
	Proceed
	Recall
	Merge
	Split
	Process
	Exit
	Issue
)

const (
	Note uint8 = iota + 1
	Voucher
	Claim
)

const (
	Active uint8 = iota
	Frozen
	Revoked
)

type Ref struct {
	ObjectType uint8
	RawID      fr.Element
}

func (r Ref) Key() string {
	return fmt.Sprintf("%d:%s", r.ObjectType, auditcrypto.EncodeField(r.RawID))
}

type Layout struct {
	Name                 string
	BaseCount            int
	ParentCount          int
	ParentType           uint8
	SpendPositions       []int
	OutputPositions      []int
	OutputTypes          []uint8
	FutureSpendPositions []int
}

func Shape(k Kind) (Layout, error) {
	switch k {
	case Entry:
		return Layout{"entry", 1, 0, Note, nil, []int{0}, []uint8{Note}, []int{0}}, nil
	case Transfer:
		return Layout{"transfer", 5, 1, Note, []int{1}, []int{2, 3}, []uint8{Voucher, Note}, []int{0, 1}}, nil
	case Proceed:
		return Layout{"proceed", 3, 1, Voucher, []int{1}, []int{2}, []uint8{Note}, []int{0}}, nil
	case Recall:
		return Layout{"recall", 4, 1, Voucher, []int{1}, []int{2}, []uint8{Note}, []int{0}}, nil
	case Merge:
		return Layout{"merge", 4, 2, Note, []int{1, 2}, []int{3}, []uint8{Note}, []int{0}}, nil
	case Split:
		return Layout{"split", 4, 1, Note, []int{1}, []int{2, 3}, []uint8{Note, Note}, []int{0, 1}}, nil
	case Process:
		return Layout{"process", 8, 3, Note, []int{3, 4, 5}, []int{6, 7}, []uint8{Note, Note}, []int{0, 1}}, nil
	case Exit:
		return Layout{"exit", 2, 1, Note, []int{1}, nil, nil, nil}, nil
	case Issue:
		return Layout{"issue", 4, 1, Note, []int{2}, []int{3}, []uint8{Claim}, nil}, nil
	default:
		return Layout{}, fmt.Errorf("unsupported EventKind %d", k)
	}
}

func (k Kind) String() string {
	l, err := Shape(k)
	if err != nil {
		return "unknown"
	}
	return l.Name
}

type Record struct {
	EventKind                            Kind
	PolicyRef                            fr.Element
	OutputRefs                           []Ref
	R1                                   auditcrypto.Point
	EncryptedParents, EncryptedOutputNfs []fr.Element
}

func NewRecord(k Kind, base []fr.Element, ct auditcrypto.Ciphertext) (Record, error) {
	l, err := Shape(k)
	if err != nil {
		return Record{}, err
	}
	if len(base) != l.BaseCount || len(ct.Data) != l.ParentCount+len(l.FutureSpendPositions) {
		return Record{}, fmt.Errorf("record shape mismatch")
	}
	r := Record{EventKind: k, R1: ct.R1}
	r.EncryptedParents = append([]fr.Element{}, ct.Data[:l.ParentCount]...)
	r.EncryptedOutputNfs = append([]fr.Element{}, ct.Data[l.ParentCount:]...)
	if k == Process || k == Issue {
		r.PolicyRef = base[0]
	}
	for i, pos := range l.OutputPositions {
		r.OutputRefs = append(r.OutputRefs, Ref{ObjectType: l.OutputTypes[i], RawID: base[pos]})
	}
	return r, r.Validate()
}

func (r Record) Validate() error {
	l, err := Shape(r.EventKind)
	if err != nil {
		return err
	}
	if len(r.OutputRefs) != len(l.OutputTypes) || len(r.EncryptedParents) != l.ParentCount || len(r.EncryptedOutputNfs) != len(l.FutureSpendPositions) {
		return fmt.Errorf("record length mismatch")
	}
	if r.EventKind != Process && r.EventKind != Issue && !r.PolicyRef.IsZero() {
		return fmt.Errorf("unexpected policy")
	}
	if err := auditcrypto.ValidatePoint(r.R1); err != nil {
		return err
	}
	for i, ref := range r.OutputRefs {
		if ref.ObjectType != l.OutputTypes[i] {
			return fmt.Errorf("output type/order mismatch")
		}
		for j := 0; j < i; j++ {
			if ref.Key() == r.OutputRefs[j].Key() {
				return fmt.Errorf("duplicate output")
			}
		}
	}
	return nil
}

func (r Record) Ciphertext() auditcrypto.Ciphertext {
	return auditcrypto.Ciphertext{R1: r.R1, Data: append(append([]fr.Element{}, r.EncryptedParents...), r.EncryptedOutputNfs...)}
}

func (r Record) Position(ref Ref) (int, error) {
	for i, v := range r.OutputRefs {
		if v.Key() == ref.Key() {
			return i, nil
		}
	}
	return 0, fmt.Errorf("producer does not contain object")
}

func (r Record) FutureSpendPosition(ref Ref) (int, error) {
	l, err := Shape(r.EventKind)
	if err != nil {
		return 0, err
	}
	pos, err := r.Position(ref)
	if err != nil {
		return 0, err
	}
	for encryptedPos, outputPos := range l.FutureSpendPositions {
		if outputPos == pos {
			return encryptedPos, nil
		}
	}
	return 0, fmt.Errorf("terminal output has no future spend value")
}

func Public(k Kind, base []fr.Element, ct auditcrypto.Ciphertext) ([]fr.Element, error) {
	if _, err := NewRecord(k, base, ct); err != nil {
		return nil, err
	}
	out := append([]fr.Element{}, base...)
	out = append(out, ct.R1.X, ct.R1.Y)
	return append(out, ct.Data...), nil
}

func Field(v *big.Int) (fr.Element, error) {
	var out fr.Element
	if v == nil || v.Sign() < 0 || v.Cmp(fr.Modulus()) >= 0 {
		return out, fmt.Errorf("noncanonical field")
	}
	out.SetBigInt(v)
	return out, nil
}

func Big(v fr.Element) *big.Int { return v.BigInt(new(big.Int)) }

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
