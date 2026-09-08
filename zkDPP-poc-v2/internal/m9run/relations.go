package m9run

import (
	"fmt"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m8case"
	"github.com/consensys/gnark/frontend"
)

type Relation struct {
	Name, Class            string
	EventKind              uint8
	PolicyRef              string
	Public                 []string
	Definition, Assignment frontend.Circuit
}

func Relations(root string, pk auditcrypto.PublicKey) ([]Relation, error) {
	groups, err := m7case.Build(root, pk)
	if err != nil {
		return nil, err
	}
	representatives := map[audit.Kind]*m7case.Case{}
	for _, g := range groups {
		for _, c := range g.Events {
			if _, ok := representatives[c.Kind]; !ok {
				representatives[c.Kind] = c
			}
		}
	}
	inputs := map[audit.Kind][]string{
		audit.Entry:    {"cm", "R1X", "R1Y", "encryptedOutputNf"},
		audit.Transfer: {"noteRoot", "nf", "rvNew", "cmChange", "transferEpoch", "deltaEpoch", "R1X", "R1Y", "encryptedParentCM", "encryptedVoucherRvnf", "encryptedChangeNf"},
		audit.Proceed:  {"voucherRoot", "rvnf", "cmReceiver", "R1X", "R1Y", "encryptedParentRV", "encryptedReceiverNf"},
		audit.Recall:   {"voucherRoot", "rvnf", "cmReturn", "currentEpoch", "R1X", "R1Y", "encryptedParentRV", "encryptedReturnNf"},
		audit.Merge:    {"noteRoot", "nf1", "nf2", "cmOut", "R1X", "R1Y", "encryptedParentCM1", "encryptedParentCM2", "encryptedOutputNf"},
		audit.Split:    {"noteRoot", "nf", "cmOut1", "cmOut2", "R1X", "R1Y", "encryptedParentCM", "encryptedOutputNf1", "encryptedOutputNf2"},
		audit.Process:  {"policyRef", "policyScopeRef", "noteRoot", "nf1", "nf2", "nf3", "cmEligible", "cmWaste", "R1X", "R1Y", "encryptedParentCM1", "encryptedParentCM2", "encryptedParentCM3", "encryptedEligibleNf", "encryptedWasteNf"},
	}
	names := map[audit.Kind]string{audit.Entry: "audit-entry", audit.Transfer: "audit-transfer", audit.Proceed: "audit-proceed", audit.Recall: "audit-recall", audit.Merge: "audit-merge", audit.Split: "audit-split", audit.Process: "audit-process-3-2"}
	order := []audit.Kind{audit.Entry, audit.Transfer, audit.Proceed, audit.Recall, audit.Merge, audit.Split, audit.Process}
	out := make([]Relation, 0, 10)
	for _, k := range order {
		c := representatives[k]
		if c == nil {
			return nil, fmt.Errorf("missing representative %s", k)
		}
		class := "fixed-event"
		policyRef := ""
		if k == audit.Process {
			class = "policy"
			policyRef = c.Inputs[0].String()
		}
		out = append(out, Relation{names[k], class, uint8(k), policyRef, inputs[k], c.Definition, c.Assignment})
	}
	m8, err := m8case.Build(root, pk)
	if err != nil {
		return nil, err
	}
	out = append(out, Relation{"exit-dpp", "fixed-event", uint8(audit.Exit), "", []string{"noteRoot", "nf", "dppCommitment", "R1X", "R1Y", "encryptedParentCM"}, m8.EligibleExit.Definition, m8.EligibleExit.Assignment})
	out = append(out, Relation{m8.Standard.Name, "policy", 8, m8.Standard.Config.PolicyRef.String(), []string{"issuePolicyRef", "dppCommitment"}, m8.Standard.Definition, m8.Standard.Assignment})
	out = append(out, Relation{m8.Strict.Name, "policy", 8, m8.Strict.Config.PolicyRef.String(), []string{"issuePolicyRef", "dppCommitment"}, m8.Strict.Definition, m8.Strict.Assignment})
	return out, nil
}
