// Package audit handles the M7 record format and snapshot-scoped audit.
package audit

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/big"
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
)
const (
	Note    uint8 = 1
	Voucher uint8 = 2
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
	Name                            string
	BaseCount, ParentCount          int
	ParentType                      uint8
	SpendPositions, OutputPositions []int
	OutputTypes                     []uint8
}

func Shape(k Kind) (Layout, error) {
	switch k {
	case Entry:
		return Layout{"entry", 1, 0, Note, nil, []int{0}, []uint8{Note}}, nil
	case Transfer:
		return Layout{"transfer", 6, 1, Note, []int{1}, []int{2, 3}, []uint8{Voucher, Note}}, nil
	case Proceed:
		return Layout{"proceed", 3, 1, Voucher, []int{1}, []int{2}, []uint8{Note}}, nil
	case Recall:
		return Layout{"recall", 4, 1, Voucher, []int{1}, []int{2}, []uint8{Note}}, nil
	case Merge:
		return Layout{"merge", 4, 2, Note, []int{1, 2}, []int{3}, []uint8{Note}}, nil
	case Split:
		return Layout{"split", 4, 1, Note, []int{1}, []int{2, 3}, []uint8{Note, Note}}, nil
	case Process:
		return Layout{"process", 8, 3, Note, []int{3, 4, 5}, []int{6, 7}, []uint8{Note, Note}}, nil
	case Exit:
		return Layout{"exit", 2, 1, Note, []int{1}, nil, nil}, nil
	}
	return Layout{}, fmt.Errorf("unsupported EventKind %d", k)
}
func (k Kind) String() string {
	l, e := Shape(k)
	if e != nil {
		return "unknown"
	}
	return l.Name
}
func Context(k Kind, p []fr.Element) (fr.Element, error) {
	l, e := Shape(k)
	if e != nil {
		return fr.Element{}, e
	}
	if len(p) != l.BaseCount {
		return fr.Element{}, fmt.Errorf("invalid base public count")
	}
	v := []fr.Element{auditcrypto.AuditContextTag, zkhash.Element(uint64(k))}
	return zkhash.Hash(append(v, p...)...), nil
}

type Record struct {
	EventKind                            Kind
	PolicyRef                            fr.Element
	OutputRefs                           []Ref
	R1                                   auditcrypto.Point
	EncryptedParents, EncryptedOutputNfs []fr.Element
}

func NewRecord(k Kind, p []fr.Element, ct auditcrypto.Ciphertext) (Record, error) {
	l, e := Shape(k)
	if e != nil {
		return Record{}, e
	}
	if len(p) != l.BaseCount || len(ct.Data) != l.ParentCount+len(l.OutputTypes) {
		return Record{}, fmt.Errorf("record shape mismatch")
	}
	r := Record{EventKind: k, R1: ct.R1, EncryptedParents: append([]fr.Element{}, ct.Data[:l.ParentCount]...), EncryptedOutputNfs: append([]fr.Element{}, ct.Data[l.ParentCount:]...)}
	if k == Process {
		r.PolicyRef = p[0]
	}
	for i, pos := range l.OutputPositions {
		r.OutputRefs = append(r.OutputRefs, Ref{l.OutputTypes[i], p[pos]})
	}
	return r, r.Validate()
}
func (r Record) Validate() error {
	l, e := Shape(r.EventKind)
	if e != nil {
		return e
	}
	if len(r.OutputRefs) != len(l.OutputTypes) || len(r.EncryptedParents) != l.ParentCount || len(r.EncryptedOutputNfs) != len(l.OutputTypes) {
		return fmt.Errorf("record length mismatch")
	}
	if r.EventKind != Process && !r.PolicyRef.IsZero() {
		return fmt.Errorf("unexpected policy")
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
	return auditcrypto.ValidatePoint(r.R1)
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
func Public(k Kind, p []fr.Element, ct auditcrypto.Ciphertext) ([]fr.Element, error) {
	if _, e := NewRecord(k, p, ct); e != nil {
		return nil, e
	}
	v := append([]fr.Element{}, p...)
	v = append(v, ct.R1.X, ct.R1.Y)
	return append(v, ct.Data...), nil
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
func Fields(values []*big.Int) ([]fr.Element, error) {
	out := make([]fr.Element, len(values))
	for i, v := range values {
		f, e := Field(v)
		if e != nil {
			return nil, e
		}
		out[i] = f
	}
	return out, nil
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

// CipherArg matches the M7 ABI tuple; array lengths are not public field inputs.
type CipherArg struct {
	R1X, R1Y                             *big.Int
	EncryptedParents, EncryptedOutputNfs []*big.Int
}

func (r Record) ABI() CipherArg {
	c := CipherArg{R1X: Big(r.R1.X), R1Y: Big(r.R1.Y), EncryptedParents: []*big.Int{}, EncryptedOutputNfs: []*big.Int{}}
	for _, v := range r.EncryptedParents {
		c.EncryptedParents = append(c.EncryptedParents, Big(v))
	}
	for _, v := range r.EncryptedOutputNfs {
		c.EncryptedOutputNfs = append(c.EncryptedOutputNfs, Big(v))
	}
	return c
}
