package v2circuit

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
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

func NewAuditWitness(parents, outputs int) AuditWitness {
	return AuditWitness{Parents: make([]frontend.Variable, parents), OutputNfs: make([]frontend.Variable, outputs)}
}

func AssertAudit(api frontend.API, pk auditcrypto.PublicKey, parents, spends []frontend.Variable, a AuditWitness) error {
	if len(parents) != len(a.Parents) || len(spends) != len(a.OutputNfs) {
		return fmt.Errorf("audit Circuit shape mismatch")
	}
	message := append(append([]frontend.Variable{}, parents...), spends...)
	cipher := append(append([]frontend.Variable{}, a.Parents...), a.OutputNfs...)
	return circuitutil.AssertMasterEncrypted(api, pk, message, a.Randomness, a.R1, cipher)
}
