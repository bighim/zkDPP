package auditentry

import (
	base "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Base        base.Circuit
	Audit       circuitutil.AuditWitness
	SKOwner     frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey) *Circuit {
	return &Circuit{CommitteePK: pk, Audit: circuitutil.NewAuditWitness(audit.Entry)}
}
func (c *Circuit) Define(api frontend.API) error {
	b := &c.Base
	// 기존 Event의 모든 관계를 유지합니다.
	if err := b.Define(api); err != nil {
		return err
	}
	var parents, spends []frontend.Variable

	address, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	api.AssertIsEqual(address, b.Note.Address)
	v, err := circuitutil.Nullifier(api, c.SKOwner, b.Commitment)
	if err != nil {
		return err
	}
	spends = append(spends, v)
	// 실제 객체에서 얻은 값만 암호화 평문에 사용합니다.
	return circuitutil.AssertEventEncrypted(api, c.CommitteePK, audit.Entry, []frontend.Variable{b.Commitment}, parents, spends, c.Audit)
}
