package v2events

import (
	"math/big"

	entrybase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	mergebase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/merge"
	spendbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/private_spend"
	proceedbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	processbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	splitbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2circuit"
	"github.com/consensys/gnark/frontend"
)

const productBits = 94

type EntryCircuit struct {
	Base        entrybase.Circuit
	Audit       v2circuit.AuditWitness
	SKOwner     frontend.Variable
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewEntry(pk auditcrypto.PublicKey) *EntryCircuit {
	return &EntryCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(0, 1)}
}

func (c *EntryCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	owner, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	api.AssertIsEqual(owner, c.Base.Note.Address)
	nf, err := circuitutil.Nullifier(api, c.SKOwner, c.Base.Commitment)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, nil, []frontend.Variable{nf}, c.Audit)
}

type TransferCircuit struct {
	NoteRoot frontend.Variable `gnark:",public"`
	NF       frontend.Variable `gnark:",public"`
	RVNew    frontend.Variable `gnark:",public"`
	CMChange frontend.Variable `gnark:",public"`
	D        frontend.Variable `gnark:",public"`

	SenderSecret   frontend.Variable
	Input          circuitutil.NoteWitness
	InputPath      circuitutil.MerklePath
	Voucher        circuitutil.VoucherWitness
	Change         circuitutil.NoteWitness
	RemainderA     frontend.Variable
	RemainderE     frontend.Variable
	DeltaTransport frontend.Variable
	Audit          v2circuit.AuditWitness
	CommitteePK    auditcrypto.PublicKey `gnark:"-"`
}

func NewTransfer(pk auditcrypto.PublicKey) *TransferCircuit {
	return &TransferCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(1, 2)}
}

func (c *TransferCircuit) Define(api frontend.API) error {
	api.ToBinary(c.D, 64)
	api.ToBinary(c.DeltaTransport, 64)
	circuitutil.AssertNote(api, c.Input)
	api.AssertIsEqual(c.Input.AssetRole, uint64(note.AssetRoleEligible))
	owner, err := circuitutil.Address(api, c.SenderSecret)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Input.Address, owner)
	inputCM, err := circuitutil.Commitment(api, c.Input)
	if err != nil {
		return err
	}
	if err = circuitutil.AssertMembership(api, c.NoteRoot, inputCM, c.InputPath); err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.SenderSecret, inputCM)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.NF, nf)
	circuitutil.AssertVoucher(api, c.Voucher)
	circuitutil.AssertNote(api, c.Change)
	api.AssertIsEqual(c.Voucher.AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Change.AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsDifferent(c.Voucher.QMass, 0)
	api.AssertIsEqual(c.Input.QMass, api.Add(c.Voucher.QMass, c.Change.QMass))
	api.AssertIsEqual(c.Voucher.DocumentHash, c.Input.DocumentHash)
	api.AssertIsEqual(c.Change.DocumentHash, c.Input.DocumentHash)
	api.AssertIsEqual(c.Voucher.SenderAddress, owner)
	api.AssertIsEqual(c.Change.Address, owner)
	api.ToBinary(c.RemainderA, 64)
	api.ToBinary(c.RemainderE, 64)
	api.AssertIsEqual(api.Mul(c.Change.QMass, c.Input.ARec), api.Add(api.Mul(c.Input.QMass, c.Change.ARec), c.RemainderA))
	api.AssertIsEqual(api.Mul(c.Change.QMass, c.Input.E), api.Add(api.Mul(c.Input.QMass, c.Change.E), c.RemainderE))
	api.AssertIsLessOrEqual(api.Add(c.RemainderA, 1), c.Input.QMass)
	api.AssertIsLessOrEqual(api.Add(c.RemainderE, 1), c.Input.QMass)
	api.AssertIsEqual(c.Input.ARec, api.Add(c.Voucher.ARec, c.Change.ARec))
	api.AssertIsEqual(c.Voucher.E, api.Add(api.Sub(c.Input.E, c.Change.E), c.DeltaTransport))
	api.AssertIsEqual(c.Voucher.DeadlineBlock, c.D)
	rv, err := circuitutil.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.RVNew, rv)
	changeCM, err := circuitutil.Commitment(api, c.Change)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMChange, changeCM)
	rvnf, err := circuitutil.VoucherNullifier(api, c.Voucher.Opening, rv)
	if err != nil {
		return err
	}
	changeNF, err := circuitutil.Nullifier(api, c.SenderSecret, changeCM)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{inputCM}, []frontend.Variable{rvnf, changeNF}, c.Audit)
}

type ProceedCircuit struct {
	Base        proceedbase.Circuit
	Audit       v2circuit.AuditWitness
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewProceed(pk auditcrypto.PublicKey) *ProceedCircuit {
	return &ProceedCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(1, 1)}
}

func (c *ProceedCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	api.AssertIsEqual(c.Base.Voucher.AssetRole, uint64(note.AssetRoleEligible))
	parent, err := circuitutil.VoucherCommitment(api, c.Base.Voucher)
	if err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.Base.ReceiverSecret, c.Base.CMReceiver)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{parent}, []frontend.Variable{nf}, c.Audit)
}

type RecallCircuit struct {
	VoucherRoot frontend.Variable `gnark:",public"`
	RVNF        frontend.Variable `gnark:",public"`
	CMReturn    frontend.Variable `gnark:",public"`
	D           frontend.Variable `gnark:",public"`

	SenderSecret frontend.Variable
	Voucher      circuitutil.VoucherWitness
	VoucherPath  circuitutil.MerklePath
	Output       circuitutil.NoteWitness
	Audit        v2circuit.AuditWitness
	CommitteePK  auditcrypto.PublicKey `gnark:"-"`
}

func NewRecall(pk auditcrypto.PublicKey) *RecallCircuit {
	return &RecallCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(1, 1)}
}

func (c *RecallCircuit) Define(api frontend.API) error {
	api.ToBinary(c.D, 64)
	circuitutil.AssertVoucher(api, c.Voucher)
	api.AssertIsEqual(c.Voucher.AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Voucher.DeadlineBlock, c.D)
	rv, err := circuitutil.VoucherCommitment(api, c.Voucher)
	if err != nil {
		return err
	}
	if err = circuitutil.AssertMembership(api, c.VoucherRoot, rv, c.VoucherPath); err != nil {
		return err
	}
	sender, err := circuitutil.Address(api, c.SenderSecret)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Voucher.SenderAddress, sender)
	rvnf, err := circuitutil.VoucherNullifier(api, c.Voucher.Opening, rv)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.RVNF, rvnf)
	circuitutil.AssertNote(api, c.Output)
	api.AssertIsEqual(c.Output.DocumentHash, c.Voucher.DocumentHash)
	api.AssertIsEqual(c.Output.AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Output.QMass, c.Voucher.QMass)
	api.AssertIsEqual(c.Output.ARec, c.Voucher.ARec)
	api.AssertIsEqual(c.Output.E, c.Voucher.E)
	api.AssertIsEqual(c.Output.Address, sender)
	cm, err := circuitutil.Commitment(api, c.Output)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.CMReturn, cm)
	nf, err := circuitutil.Nullifier(api, c.SenderSecret, cm)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{rv}, []frontend.Variable{nf}, c.Audit)
}

type MergeCircuit struct {
	Base        mergebase.Circuit
	Audit       v2circuit.AuditWitness
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewMerge(pk auditcrypto.PublicKey) *MergeCircuit {
	return &MergeCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(2, 1)}
}

func (c *MergeCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	api.AssertIsEqual(c.Base.Inputs[0].AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Base.Inputs[1].AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsEqual(c.Base.Inputs[0].DocumentHash, c.Base.Inputs[1].DocumentHash)
	api.AssertIsEqual(c.Base.Output.DocumentHash, c.Base.Inputs[0].DocumentHash)
	parents := make([]frontend.Variable, 2)
	for i := range c.Base.Inputs {
		cm, err := circuitutil.Commitment(api, c.Base.Inputs[i])
		if err != nil {
			return err
		}
		parents[i] = cm
	}
	nf, err := circuitutil.Nullifier(api, c.Base.SKOwner, c.Base.CMOut)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, parents, []frontend.Variable{nf}, c.Audit)
}

type SplitCircuit struct {
	Base        splitbase.Circuit
	Audit       v2circuit.AuditWitness
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewSplit(pk auditcrypto.PublicKey) *SplitCircuit {
	return &SplitCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(1, 2)}
}

func (c *SplitCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	api.AssertIsEqual(c.Base.Input.AssetRole, uint64(note.AssetRoleEligible))
	for i := range c.Base.Outputs {
		api.AssertIsEqual(c.Base.Outputs[i].DocumentHash, c.Base.Input.DocumentHash)
		zero := api.IsZero(c.Base.Outputs[i].QMass)
		api.AssertIsEqual(api.Mul(zero, c.Base.Outputs[i].ARec), 0)
		api.AssertIsEqual(api.Mul(zero, c.Base.Outputs[i].E), 0)
	}
	parent, err := circuitutil.Commitment(api, c.Base.Input)
	if err != nil {
		return err
	}
	spends := make([]frontend.Variable, 2)
	for i, cm := range []frontend.Variable{c.Base.CMOut1, c.Base.CMOut2} {
		spends[i], err = circuitutil.Nullifier(api, c.Base.SKOwner, cm)
		if err != nil {
			return err
		}
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{parent}, spends, c.Audit)
}

type ProcessCircuit struct {
	Base        processbase.Circuit
	Audit       v2circuit.AuditWitness
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewProcess(pk auditcrypto.PublicKey) *ProcessCircuit {
	return &ProcessCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(3, 2)}
}

func (c *ProcessCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	parents := make([]frontend.Variable, 3)
	for i := range c.Base.Inputs {
		cm, err := circuitutil.Commitment(api, c.Base.Inputs[i])
		if err != nil {
			return err
		}
		parents[i] = cm
	}
	spends := make([]frontend.Variable, 2)
	for i := range c.Base.CMOut {
		nf, err := circuitutil.Nullifier(api, c.Base.SKOwner, c.Base.CMOut[i])
		if err != nil {
			return err
		}
		spends[i] = nf
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, parents, spends, c.Audit)
}

type ExitCircuit struct {
	Base        spendbase.Circuit
	Audit       v2circuit.AuditWitness
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewExit(pk auditcrypto.PublicKey) *ExitCircuit {
	return &ExitCircuit{CommitteePK: pk, Audit: v2circuit.NewAuditWitness(1, 0)}
}

func (c *ExitCircuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	parent, err := circuitutil.Commitment(api, c.Base.Note)
	if err != nil {
		return err
	}
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{parent}, nil, c.Audit)
}

type IssueCircuit struct {
	PolicyRef frontend.Variable `gnark:",public"`
	NoteRoot  frontend.Variable `gnark:",public"`
	NF        frontend.Variable `gnark:",public"`
	H         frontend.Variable `gnark:",public"`

	SKOwner     frontend.Variable
	Note        circuitutil.NoteWitness
	Path        circuitutil.MerklePath
	ClaimNonce  frontend.Variable
	Audit       v2circuit.AuditWitness
	Config      issuepolicy.Config    `gnark:"-"`
	CommitteePK auditcrypto.PublicKey `gnark:"-"`
}

func NewIssue(pk auditcrypto.PublicKey, config issuepolicy.Config) *IssueCircuit {
	return &IssueCircuit{CommitteePK: pk, Config: config, Audit: v2circuit.NewAuditWitness(1, 0)}
}

func (c *IssueCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.PolicyRef, c.Config.PolicyRef)
	circuitutil.AssertNote(api, c.Note)
	api.AssertIsEqual(c.Note.AssetRole, uint64(note.AssetRoleEligible))
	api.AssertIsDifferent(c.Note.QMass, 0)
	owner, err := circuitutil.Address(api, c.SKOwner)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.Note.Address, owner)
	cm, err := circuitutil.Commitment(api, c.Note)
	if err != nil {
		return err
	}
	if err = circuitutil.AssertMembership(api, c.NoteRoot, cm, c.Path); err != nil {
		return err
	}
	nf, err := circuitutil.Nullifier(api, c.SKOwner, cm)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.NF, nf)
	recycledHave := api.Mul(c.Note.ARec, issuepolicy.Denominator)
	recycledNeed := api.Mul(c.Note.QMass, c.Config.MinRecycledRate)
	carbonHave := api.Mul(c.Note.E, issuepolicy.Denominator)
	carbonLimit := api.Mul(c.Note.QMass, c.Config.MaxCarbonIntensity)
	api.ToBinary(recycledHave, productBits)
	api.ToBinary(recycledNeed, productBits)
	api.ToBinary(carbonHave, productBits)
	api.ToBinary(carbonLimit, productBits)
	api.AssertIsLessOrEqual(recycledNeed, recycledHave)
	api.AssertIsLessOrEqual(carbonHave, carbonLimit)
	h, err := circuitutil.Hash(api, zkhash.IssueTag, c.Note.DocumentHash, c.PolicyRef, c.ClaimNonce)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.H, h)
	return v2circuit.AssertAudit(api, c.CommitteePK, []frontend.Variable{cm}, nil, c.Audit)
}

func Scalar(v uint64) *big.Int { return new(big.Int).SetUint64(v) }
