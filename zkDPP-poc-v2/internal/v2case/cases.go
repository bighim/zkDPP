package v2case

import (
	"fmt"
	"math/big"
	"path/filepath"

	entrybase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	mergebase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/merge"
	spendbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/private_spend"
	proceedbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	processbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/process_policy_3_2"
	splitbase "github.com/bighim/zkDPP/zkDPP-poc-v2/features/split"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/features/v2events"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/claim"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/policy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2circuit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

type Case struct {
	Name                   string
	Definition, Assignment frontend.Circuit
	PublicNames            []string
	Message                []fr.Element
	Ciphertext             auditcrypto.Ciphertext
}

type Suite struct {
	Package auditcrypto.ExternalKeyPackage
	Shares  [3]auditcrypto.Share
	Cases   []Case
}

func KeyFixture() (auditcrypto.ExternalKeyPackage, [3]auditcrypto.Share, error) {
	// This deterministic fixture models an already completed external DKG.
	x, a := big.NewInt(7919), big.NewInt(104729)
	q := auditcrypto.Order()
	var shares [3]auditcrypto.Share
	for i := range shares {
		v := new(big.Int).Mul(a, big.NewInt(int64(i+1)))
		v.Add(v, x).Mod(v, q)
		shares[i] = auditcrypto.Share{ID: uint8(i + 1), Value: v}
	}
	g := auditcrypto.Generator()
	var point auditcrypto.Point
	point.ScalarMultiplication(&g, x)
	pkg, err := auditcrypto.PackageFromShares("zkdpp-v2-m1-hotfix-fixture", auditcrypto.PublicKey{Point: point}, shares)
	x.SetInt64(0)
	a.SetInt64(0)
	return pkg, shares, err
}

func Build(root string) (*Suite, error) {
	pkg, shares, err := KeyFixture()
	if err != nil {
		return nil, err
	}
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v2.json"))
	if err != nil {
		return nil, err
	}
	sender, err := testkit.ByID(actors, "actor-1")
	if err != nil {
		return nil, err
	}
	receiver, err := testkit.ByID(actors, "actor-2")
	if err != nil {
		return nil, err
	}
	doc, err := document.Hash(document.DocumentInfo{ProductName: "Battery material", LotID: "V2-M1", Unit: "kg"})
	if err != nil {
		return nil, err
	}
	opening := uint64(20001)
	newNote := func(state note.State, owner fr.Element) (note.Note, error) {
		v, e := note.New(doc, state, note.AssetRoleEligible, owner, zkhash.Element(opening))
		opening++
		return v, e
	}
	newWaste := func(state note.State, owner fr.Element) (note.Note, error) {
		v, e := note.New(doc, state, note.AssetRoleWaste, owner, zkhash.Element(opening))
		opening++
		return v, e
	}
	noteTree, voucherTree := merkle.New(), merkle.New()
	noteIndex := map[string]uint64{}
	voucherIndex := map[string]uint64{}
	appendNote := func(v note.Note) {
		noteIndex[v.Commitment.String()] = noteTree.Count()
		_, _, _ = noteTree.Append(v.Commitment)
	}
	appendVoucher := func(v voucher.Voucher) {
		voucherIndex[v.Commitment.String()] = voucherTree.Count()
		_, _, _ = voucherTree.Append(v.Commitment)
	}
	pathNote := func(v note.Note) merkle.Path { p, _ := noteTree.Path(noteIndex[v.Commitment.String()]); return p }
	pathVoucher := func(v voucher.Voucher) merkle.Path {
		p, _ := voucherTree.Path(voucherIndex[v.Commitment.String()])
		return p
	}

	s := &Suite{Package: pkg, Shares: shares}
	randomness := int64(30001)
	add := func(name string, definition, assignment frontend.Circuit, public []string, message []fr.Element, audit *v2circuit.AuditWitness) error {
		ct, e := auditcrypto.EncryptWithMasterPublicKey(pkg.PublicKey, message, big.NewInt(randomness))
		randomness++
		if e != nil {
			return e
		}
		audit.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
		audit.Randomness = big.NewInt(randomness - 1)
		for i := range audit.Parents {
			audit.Parents[i] = ct.Data[i]
		}
		for i := range audit.OutputNfs {
			audit.OutputNfs[i] = ct.Data[len(audit.Parents)+i]
		}
		s.Cases = append(s.Cases, Case{name, definition, assignment, public, append([]fr.Element(nil), message...), ct})
		return nil
	}

	entryNote, err := newNote(note.State{QMass: 100e9, ARec: 20e9, E: 80e9}, sender.Address)
	if err != nil {
		return nil, err
	}
	entry := v2events.NewEntry(pkg.PublicKey)
	entry.Base = *entrybase.Assignment(entryNote)
	entry.SKOwner = sender.SKOwner
	if err = add("entry", v2events.NewEntry(pkg.PublicKey), entry, []string{"cm", "R1X", "R1Y", "encryptedOutputNf"}, []fr.Element{note.Nullifier(sender.SKOwner, entryNote.Commitment)}, &entry.Audit); err != nil {
		return nil, err
	}
	appendNote(entryNote)

	vState, changeState, rem, err := voucher.AllocateWithTransport(entryNote.State, 60e9, 1e9)
	if err != nil {
		return nil, err
	}
	v, err := voucher.New(doc, note.AssetRoleEligible, vState, sender.Address, receiver.Address, 500, zkhash.Element(opening))
	opening++
	if err != nil {
		return nil, err
	}
	change, err := newNote(changeState, sender.Address)
	if err != nil {
		return nil, err
	}
	transfer := v2events.NewTransfer(pkg.PublicKey)
	transfer.NoteRoot, transfer.NF, transfer.RVNew, transfer.CMChange, transfer.D = noteTree.Root(), note.Nullifier(sender.SKOwner, entryNote.Commitment), v.Commitment, change.Commitment, uint64(500)
	transfer.SenderSecret, transfer.Input, transfer.InputPath = sender.SKOwner, noteWitness(entryNote), circuitPath(pathNote(entryNote))
	transfer.Voucher, transfer.Change = voucherWitness(v), noteWitness(change)
	transfer.RemainderA, transfer.RemainderE, transfer.DeltaTransport = rem.ARec, rem.E, uint64(1e9)
	msg := []fr.Element{entryNote.Commitment, voucher.Nullifier(v.Opening, v.Commitment), note.Nullifier(sender.SKOwner, change.Commitment)}
	if err = add("transfer", v2events.NewTransfer(pkg.PublicKey), transfer, []string{"noteRoot", "nf", "rvNew", "cmChange", "D", "R1X", "R1Y", "encryptedParentCM", "encryptedVoucherRvnf", "encryptedChangeNf"}, msg, &transfer.Audit); err != nil {
		return nil, err
	}
	appendVoucher(v)
	appendNote(change)

	receiverNote, err := newNote(v.State, receiver.Address)
	if err != nil {
		return nil, err
	}
	proceed := v2events.NewProceed(pkg.PublicKey)
	proceed.Base = *proceedbase.Assignment(v, receiver.SKOwner, voucherTree.Root(), pathVoucher(v), receiverNote)
	msg = []fr.Element{v.Commitment, note.Nullifier(receiver.SKOwner, receiverNote.Commitment)}
	if err = add("proceed", v2events.NewProceed(pkg.PublicKey), proceed, []string{"voucherRoot", "rvnf", "cmReceiver", "R1X", "R1Y", "encryptedParentRV", "encryptedReceiverNf"}, msg, &proceed.Audit); err != nil {
		return nil, err
	}
	appendNote(receiverNote)

	returnNote, err := newNote(v.State, sender.Address)
	if err != nil {
		return nil, err
	}
	recall := v2events.NewRecall(pkg.PublicKey)
	recall.VoucherRoot, recall.RVNF, recall.CMReturn, recall.D = voucherTree.Root(), voucher.Nullifier(v.Opening, v.Commitment), returnNote.Commitment, uint64(500)
	recall.SenderSecret, recall.Voucher, recall.VoucherPath, recall.Output = sender.SKOwner, voucherWitness(v), circuitPath(pathVoucher(v)), noteWitness(returnNote)
	msg = []fr.Element{v.Commitment, note.Nullifier(sender.SKOwner, returnNote.Commitment)}
	if err = add("recall", v2events.NewRecall(pkg.PublicKey), recall, []string{"voucherRoot", "rvnf", "cmReturn", "D", "R1X", "R1Y", "encryptedParentRV", "encryptedReturnNf"}, msg, &recall.Audit); err != nil {
		return nil, err
	}

	mergeA, err := newNote(note.State{QMass: 40e9, ARec: 10e9, E: 30e9}, receiver.Address)
	if err != nil {
		return nil, err
	}
	mergeB, err := newNote(note.State{QMass: 60e9, ARec: 10e9, E: 50e9}, receiver.Address)
	if err != nil {
		return nil, err
	}
	appendNote(mergeA)
	appendNote(mergeB)
	mergeOut, err := newNote(note.State{QMass: 100e9, ARec: 20e9, E: 80e9}, receiver.Address)
	if err != nil {
		return nil, err
	}
	merge := v2events.NewMerge(pkg.PublicKey)
	merge.Base = *mergebase.Assignment(mergeA, mergeB, receiver.SKOwner, noteTree.Root(), pathNote(mergeA), pathNote(mergeB), mergeOut)
	msg = []fr.Element{mergeA.Commitment, mergeB.Commitment, note.Nullifier(receiver.SKOwner, mergeOut.Commitment)}
	if err = add("merge", v2events.NewMerge(pkg.PublicKey), merge, []string{"noteRoot", "nf1", "nf2", "cmOut", "R1X", "R1Y", "encryptedParentCM1", "encryptedParentCM2", "encryptedOutputNf"}, msg, &merge.Audit); err != nil {
		return nil, err
	}
	appendNote(mergeOut)

	splitFirst, splitSecond, splitRem, err := allocation.ByMass(mergeOut.State, 70e9)
	if err != nil {
		return nil, err
	}
	splitA, err := newNote(splitFirst, receiver.Address)
	if err != nil {
		return nil, err
	}
	splitB, err := newNote(splitSecond, receiver.Address)
	if err != nil {
		return nil, err
	}
	split := v2events.NewSplit(pkg.PublicKey)
	split.Base = *splitbase.Assignment(mergeOut, receiver.SKOwner, noteTree.Root(), pathNote(mergeOut), splitA, splitB, splitRem)
	msg = []fr.Element{mergeOut.Commitment, note.Nullifier(receiver.SKOwner, splitA.Commitment), note.Nullifier(receiver.SKOwner, splitB.Commitment)}
	if err = add("split", v2events.NewSplit(pkg.PublicKey), split, []string{"noteRoot", "nf", "cmOut1", "cmOut2", "R1X", "R1Y", "encryptedParentCM", "encryptedOutputNf1", "encryptedOutputNf2"}, msg, &split.Audit); err != nil {
		return nil, err
	}
	appendNote(splitA)
	appendNote(splitB)

	processInputs := [3]note.Note{}
	states := [3]note.State{{QMass: 120e9, ARec: 20e9, E: 91e9}, {QMass: 100e9, ARec: 10e9, E: 70e9}, {QMass: 100e9, E: 70e9}}
	for i := range processInputs {
		processInputs[i], err = newNote(states[i], receiver.Address)
		if err != nil {
			return nil, err
		}
		appendNote(processInputs[i])
	}
	var processPaths [3]merkle.Path
	for i := range processPaths {
		processPaths[i] = pathNote(processInputs[i])
	}
	result, err := policy.Apply(states)
	if err != nil {
		return nil, err
	}
	eligible, err := newNote(result.Eligible, receiver.Address)
	if err != nil {
		return nil, err
	}
	waste, err := newWaste(result.Waste, receiver.Address)
	if err != nil {
		return nil, err
	}
	process := v2events.NewProcess(pkg.PublicKey)
	process.Base = *processbase.Assignment(processInputs, receiver.SKOwner, noteTree.Root(), processPaths, [2]note.Note{eligible, waste}, result)
	msg = []fr.Element{processInputs[0].Commitment, processInputs[1].Commitment, processInputs[2].Commitment, note.Nullifier(receiver.SKOwner, eligible.Commitment), note.Nullifier(receiver.SKOwner, waste.Commitment)}
	if err = add("process", v2events.NewProcess(pkg.PublicKey), process, []string{"policyRef", "policyScopeRef", "noteRoot", "nf1", "nf2", "nf3", "cmEligible", "cmWaste", "R1X", "R1Y", "encryptedParentCM1", "encryptedParentCM2", "encryptedParentCM3", "encryptedEligibleNf", "encryptedWasteNf"}, msg, &process.Audit); err != nil {
		return nil, err
	}
	appendNote(eligible)
	appendNote(waste)

	exit := v2events.NewExit(pkg.PublicKey)
	exit.Base = *spendbase.Assignment(waste, receiver.SKOwner, noteTree.Root(), pathNote(waste))
	msg = []fr.Element{waste.Commitment}
	if err = add("exit", v2events.NewExit(pkg.PublicKey), exit, []string{"noteRoot", "nf", "R1X", "R1Y", "encryptedParentCM"}, msg, &exit.Audit); err != nil {
		return nil, err
	}

	issueNotes := []note.Note{splitA, splitB}
	for issueIndex, cfg := range []issuepolicy.Config{issuepolicy.Standard(), issuepolicy.Strict()} {
		issueNote := issueNotes[issueIndex]
		issue := v2events.NewIssue(pkg.PublicKey, cfg)
		issue.PolicyRef, issue.NoteRoot, issue.NF = cfg.PolicyRef, noteTree.Root(), note.Nullifier(receiver.SKOwner, issueNote.Commitment)
		issue.SKOwner, issue.Note, issue.Path, issue.ClaimNonce = receiver.SKOwner, noteWitness(issueNote), circuitPath(pathNote(issueNote)), zkhash.Element(44000+cfg.Version)
		issue.H = claim.Handle(issueNote.DocumentHash, cfg.PolicyRef, zkhash.Element(44000+cfg.Version))
		msg = []fr.Element{issueNote.Commitment}
		if err = add(cfg.Name, v2events.NewIssue(pkg.PublicKey, cfg), issue, []string{"issuePolicyRef", "noteRoot", "nf", "h", "R1X", "R1Y", "encryptedParentCM"}, msg, &issue.Audit); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func noteWitness(v note.Note) circuitutil.NoteWitness {
	return circuitutil.NoteWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, Address: v.Address, Opening: v.Opening}
}
func voucherWitness(v voucher.Voucher) circuitutil.VoucherWitness {
	return circuitutil.VoucherWitness{DocumentHash: v.DocumentHash, AssetRole: uint64(v.AssetRole), QMass: v.State.QMass, ARec: v.State.ARec, E: v.State.E, SenderAddress: v.SenderAddress, ReceiverAddress: v.ReceiverAddress, DeadlineBlock: v.DeadlineBlock, Opening: v.Opening}
}
func circuitPath(v merkle.Path) circuitutil.MerklePath {
	p := circuitutil.MerklePath{Index: v.Index}
	for i := range v.Siblings {
		p.Siblings[i] = v.Siblings[i]
	}
	return p
}

func (s *Suite) Validate() error {
	if len(s.Cases) != 10 {
		return fmt.Errorf("relations=%d, want 10", len(s.Cases))
	}
	master, err := auditcrypto.RecoverMasterKey(s.Package, []auditcrypto.Share{s.Shares[0], s.Shares[2]})
	if err != nil {
		return err
	}
	defer master.SetInt64(0)
	for _, c := range s.Cases {
		plain, e := auditcrypto.DecryptWithMasterKey(master, c.Ciphertext)
		if e != nil || len(plain) != len(c.Message) {
			return fmt.Errorf("%s decrypt: %w", c.Name, e)
		}
		for i := range plain {
			if !plain[i].Equal(&c.Message[i]) {
				return fmt.Errorf("%s plaintext %d", c.Name, i)
			}
		}
	}
	return nil
}
