package m3case

import (
	"path/filepath"

	entrycircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/entry"
	proceedcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/proceed"
	recallcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/recall"
	transfercircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	TransferEpoch = uint64(100)
	DeltaEpoch    = uint64(6)
	RecallEpoch   = uint64(105)
)

type EntryCase struct {
	Name, ActorID string
	Note          note.Note
	Assignment    *entrycircuit.Circuit
}
type TransferCase struct {
	Name       string
	Input      note.Note
	Voucher    voucher.Voucher
	Change     note.Note
	Remainder  voucher.Remainder
	Assignment *transfercircuit.Circuit
}
type ProceedCase struct {
	Name       string
	Voucher    voucher.Voucher
	Output     note.Note
	Assignment *proceedcircuit.Circuit
}
type RecallCase struct {
	Name       string
	Voucher    voucher.Voucher
	Output     note.Note
	Assignment *recallcircuit.Circuit
}

type Scenario struct {
	Entries           []EntryCase
	Partial           TransferCase
	Proceed           ProceedCase
	Full              TransferCase
	Recall            RecallCase
	FinalNoteRoot     fr.Element
	FinalVoucherRoot  fr.Element
	FinalNoteCount    uint64
	FinalVoucherCount uint64
	FinalNotePaths    []merkle.Path
	FinalVoucherPaths []merkle.Path
}

func Build(root string) (*Scenario, error) {
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata", "common", "actors-v1.json"))
	if err != nil {
		return nil, err
	}
	actor1, _ := testkit.ByID(actors, "actor-1")
	actor2, _ := testkit.ByID(actors, "actor-2")
	actor3, _ := testkit.ByID(actors, "actor-3")
	noteTree, voucherTree := merkle.New(), merkle.New()

	makeNote := func(product, lot string, state note.State, opening uint64) (note.Note, error) {
		doc, err := document.Hash(document.DocumentInfo{ProductName: product, LotID: lot})
		if err != nil {
			return note.Note{}, err
		}
		return note.New(doc, state, note.AssetRoleEligible, actor1.Address, zkhash.Element(opening))
	}
	first, err := makeNote("M3 Partial Material", "M3-P-001", note.State{QMass: 3 * note.MassScale, ARec: note.MassScale, E: 2 * note.CarbonScale}, 3001)
	if err != nil {
		return nil, err
	}
	second, err := makeNote("M3 Full Material", "M3-F-001", note.State{QMass: 5 * note.MassScale, ARec: 2 * note.MassScale, E: 4 * note.CarbonScale}, 3002)
	if err != nil {
		return nil, err
	}
	entries := []EntryCase{{"partial-input", "actor-1", first, entrycircuit.Assignment(first)}, {"full-input", "actor-1", second, entrycircuit.Assignment(second)}}
	for _, item := range entries {
		if _, _, err := noteTree.Append(item.Note.Commitment); err != nil {
			return nil, err
		}
	}

	partialRoot := noteTree.Root()
	partialPath, err := noteTree.Path(0)
	if err != nil {
		return nil, err
	}
	vState, cState, rem, err := voucher.Allocate(first.State, note.MassScale)
	if err != nil {
		return nil, err
	}
	partialVoucher, err := voucher.New(first.DocumentHash, first.AssetRole, vState, actor1.Address, actor2.Address, TransferEpoch+DeltaEpoch, zkhash.Element(4001))
	if err != nil {
		return nil, err
	}
	partialChange, err := note.New(first.DocumentHash, cState, first.AssetRole, actor1.Address, zkhash.Element(5001))
	if err != nil {
		return nil, err
	}
	partial := TransferCase{Name: "partial", Input: first, Voucher: partialVoucher, Change: partialChange, Remainder: rem, Assignment: transfercircuit.Assignment(first, actor1.SKOwner, partialRoot, partialPath, partialVoucher, partialChange, rem, TransferEpoch, DeltaEpoch)}
	if _, _, err := noteTree.Append(partialChange.Commitment); err != nil {
		return nil, err
	}
	partialVoucherIndex, _, err := voucherTree.Append(partialVoucher.Commitment)
	if err != nil {
		return nil, err
	}
	proceedPath, err := voucherTree.Path(partialVoucherIndex)
	if err != nil {
		return nil, err
	}
	proceedOut, err := note.New(partialVoucher.DocumentHash, partialVoucher.State, partialVoucher.AssetRole, actor2.Address, zkhash.Element(6001))
	if err != nil {
		return nil, err
	}
	proceed := ProceedCase{Name: "partial", Voucher: partialVoucher, Output: proceedOut, Assignment: proceedcircuit.Assignment(partialVoucher, actor2.SKOwner, voucherTree.Root(), proceedPath, proceedOut)}
	if _, _, err := noteTree.Append(proceedOut.Commitment); err != nil {
		return nil, err
	}

	fullRoot := noteTree.Root()
	fullPath, err := noteTree.Path(1)
	if err != nil {
		return nil, err
	}
	fullVState, fullCState, fullRem, err := voucher.Allocate(second.State, second.State.QMass)
	if err != nil {
		return nil, err
	}
	fullVoucher, err := voucher.New(second.DocumentHash, second.AssetRole, fullVState, actor1.Address, actor3.Address, TransferEpoch+DeltaEpoch, zkhash.Element(4002))
	if err != nil {
		return nil, err
	}
	fullChange, err := note.New(second.DocumentHash, fullCState, second.AssetRole, actor1.Address, zkhash.Element(5002))
	if err != nil {
		return nil, err
	}
	full := TransferCase{Name: "full", Input: second, Voucher: fullVoucher, Change: fullChange, Remainder: fullRem, Assignment: transfercircuit.Assignment(second, actor1.SKOwner, fullRoot, fullPath, fullVoucher, fullChange, fullRem, TransferEpoch, DeltaEpoch)}
	if _, _, err := noteTree.Append(fullChange.Commitment); err != nil {
		return nil, err
	}
	fullVoucherIndex, _, err := voucherTree.Append(fullVoucher.Commitment)
	if err != nil {
		return nil, err
	}
	recallPath, err := voucherTree.Path(fullVoucherIndex)
	if err != nil {
		return nil, err
	}
	recallOut, err := note.New(fullVoucher.DocumentHash, fullVoucher.State, fullVoucher.AssetRole, actor1.Address, zkhash.Element(6002))
	if err != nil {
		return nil, err
	}
	recall := RecallCase{Name: "full", Voucher: fullVoucher, Output: recallOut, Assignment: recallcircuit.Assignment(fullVoucher, actor1.SKOwner, voucherTree.Root(), recallPath, recallOut, RecallEpoch)}
	if _, _, err := noteTree.Append(recallOut.Commitment); err != nil {
		return nil, err
	}

	s := &Scenario{Entries: entries, Partial: partial, Proceed: proceed, Full: full, Recall: recall, FinalNoteRoot: noteTree.Root(), FinalVoucherRoot: voucherTree.Root(), FinalNoteCount: noteTree.Count(), FinalVoucherCount: voucherTree.Count()}
	for i := uint64(0); i < noteTree.Count(); i++ {
		p, _ := noteTree.Path(i)
		s.FinalNotePaths = append(s.FinalNotePaths, p)
	}
	for i := uint64(0); i < voucherTree.Count(); i++ {
		p, _ := voucherTree.Path(i)
		s.FinalVoucherPaths = append(s.FinalVoucherPaths, p)
	}
	return s, nil
}
