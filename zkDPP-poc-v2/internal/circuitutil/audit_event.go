package circuitutil

import (
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

type AuditWitness struct {
	R1         ed.Point            `gnark:",public"`
	Parents    []frontend.Variable `gnark:",public"`
	OutputNfs  []frontend.Variable `gnark:",public"`
	Randomness frontend.Variable
}

func NewAuditWitness(kind audit.Kind) AuditWitness {
	l, e := audit.Shape(kind)
	if e != nil {
		panic(e)
	}
	return AuditWitness{Parents: make([]frontend.Variable, l.ParentCount), OutputNfs: make([]frontend.Variable, len(l.OutputTypes))}
}
func AssertEventEncrypted(api frontend.API, pk auditcrypto.PublicKey, k audit.Kind, p, parents, spends []frontend.Variable, a AuditWitness) error {
	l, e := audit.Shape(k)
	if e != nil {
		return e
	}
	if len(p) != l.BaseCount || len(parents) != l.ParentCount || len(spends) != len(l.OutputTypes) || len(a.Parents) != len(parents) || len(a.OutputNfs) != len(spends) {
		return fmt.Errorf("audit Circuit shape mismatch")
	}
	contextFields := []frontend.Variable{auditcrypto.AuditContextTag, uint64(k)}
	contextFields = append(contextFields, p...)
	context, e := Hash(api, contextFields...)
	if e != nil {
		return e
	}
	message := append(append([]frontend.Variable{}, parents...), spends...)
	cipher := append(append([]frontend.Variable{}, a.Parents...), a.OutputNfs...)
	return AssertEncrypted(api, pk, context, message, a.Randomness, a.R1, cipher)
}
