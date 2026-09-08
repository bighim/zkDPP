package auditsplit

import (
	base "github.com/bighim/zkDPP/zkDPP-poc-v2/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Base  base.Circuit
	Audit circuitutil.AuditWitness

	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey) *Circuit {
	return &Circuit{CommitteePK: pk, Audit: circuitutil.NewAuditWitness(audit.Split)}
}
func (c *Circuit) Define(api frontend.API) error {
	b := &c.Base
	// 기존 Event의 모든 관계를 유지합니다.
	if err := b.Define(api); err != nil {
		return err
	}
	var parents, spends []frontend.Variable
	v, err := circuitutil.Commitment(api, b.Input)
	if err != nil {
		return err
	}
	parents = append(parents, v)
	for _, cm := range []frontend.Variable{b.CMOut1, b.CMOut2} {
		nf, err := circuitutil.Nullifier(api, b.SKOwner, cm)
		if err != nil {
			return err
		}
		spends = append(spends, nf)
	}
	// 실제 객체에서 얻은 값만 암호화 평문에 사용합니다.
	return circuitutil.AssertEventEncrypted(api, c.CommitteePK, audit.Split, []frontend.Variable{b.NoteRoot, b.NF, b.CMOut1, b.CMOut2}, parents, spends, c.Audit)
}
