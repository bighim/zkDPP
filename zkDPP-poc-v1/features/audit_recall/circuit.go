package auditrecall

import (
	base "github.com/bighim/zkDPP/zkDPP-poc-v1/features/recall"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/consensys/gnark/frontend"
)

type Circuit struct {
	Base  base.Circuit
	Audit circuitutil.AuditWitness

	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func New(pk auditcrypto.PublicKey) *Circuit {
	return &Circuit{CommitteePK: pk, Audit: circuitutil.NewAuditWitness(audit.Recall)}
}
func (c *Circuit) Define(api frontend.API) error {
	b := &c.Base
	// 기존 Event의 모든 관계를 유지합니다.
	if err := b.Define(api); err != nil {
		return err
	}
	var parents, spends []frontend.Variable
	v, err := circuitutil.VoucherCommitment(api, b.Voucher)
	if err != nil {
		return err
	}
	parents = append(parents, v)
	nf, err := circuitutil.Nullifier(api, b.SenderSecret, b.CMReturn)
	if err != nil {
		return err
	}
	spends = append(spends, nf)
	// 실제 객체에서 얻은 값만 암호화 평문에 사용합니다.
	return circuitutil.AssertEventEncrypted(api, c.CommitteePK, audit.Recall, []frontend.Variable{b.VoucherRoot, b.RVNF, b.CMReturn, b.CurrentEpoch}, parents, spends, c.Audit)
}
