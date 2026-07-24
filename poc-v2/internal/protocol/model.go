package protocol

import (
	"fmt"
	"math/big"

	"github.com/bighim/zkDPP/poc-v2/internal/zkhash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const (
	StateLength           = 3
	MaxProcessArity       = 3
	TreeDepth             = 32
	AllocationDenominator = uint64(1_000_000_000)
	EpochSizeSeconds      = uint64(600)
)

type EventID uint8

const (
	EventEntry EventID = iota
	EventTransfer
	EventProceed
	EventRecall
	EventMerge
	EventSplit
	EventProcess
	EventExit
)

var EventNames = [...]string{
	"entry", "transfer", "proceed", "recall", "merge", "split", "process", "exit",
}

type State [StateLength]uint64

type DocumentInfo struct {
	ProductName string `json:"productName"`
	LotID       string `json:"lotId"`
	Unit        string `json:"unit"`
}

type Owner struct {
	SecretKey fr.Element
	PublicKey fr.Element
	Address   fr.Element
}

type Note struct {
	Commitment   fr.Element
	DocumentHash fr.Element
	Quantity     uint64
	State        State
	Opening      fr.Element
	Owner        Owner
	Index        uint64
}

type Voucher struct {
	Commitment      fr.Element
	DocumentHash    fr.Element
	Quantity        uint64
	State           State
	SenderAddress   fr.Element
	ReceiverAddress fr.Element
	DeltaEpoch      uint64
	Opening         fr.Element
	Index           uint64
}

type PublicInputManifest struct {
	Event  string   `json:"event"`
	ID     EventID  `json:"id"`
	Inputs []string `json:"inputs"`
}

func CanonicalManifests() []PublicInputManifest {
	return []PublicInputManifest{
		{Event: "entry", ID: EventEntry, Inputs: []string{"cm_new"}},
		{Event: "transfer", ID: EventTransfer, Inputs: []string{"rt", "cm_in", "nf", "rv", "cm_change", "delta_epoch"}},
		{Event: "proceed", ID: EventProceed, Inputs: []string{"rvrt", "rv", "rvnf", "cm_recv"}},
		{Event: "recall", ID: EventRecall, Inputs: []string{"rvrt", "rv", "rvnf", "cm_return"}},
		{Event: "merge", ID: EventMerge, Inputs: []string{"rt_1", "rt_2", "cm_1", "cm_2", "nf_1", "nf_2", "cm_out"}},
		{Event: "split", ID: EventSplit, Inputs: []string{"rt", "cm_in", "nf", "cm_out_1", "cm_out_2"}},
		{Event: "process", ID: EventProcess, Inputs: processInputs(MaxProcessArity, StateLength)},
		{Event: "exit", ID: EventExit, Inputs: []string{"rt", "cm_in", "nf"}},
	}
}

func processInputs(arity, stateLength int) []string {
	inputs := []string{"m", "n"}
	for k := 0; k < stateLength; k++ {
		inputs = append(inputs, fmt.Sprintf("p_%d", k))
	}
	for j := 0; j < arity; j++ {
		for k := 0; k < stateLength; k++ {
			inputs = append(inputs, fmt.Sprintf("A_%d_%d", j, k))
		}
	}
	for i := 0; i < arity; i++ {
		inputs = append(inputs, fmt.Sprintf("rt_%d", i), fmt.Sprintf("cm_in_%d", i), fmt.Sprintf("nf_%d", i))
	}
	for j := 0; j < arity; j++ {
		inputs = append(inputs, fmt.Sprintf("cm_out_%d", j))
	}
	return inputs
}

func NewOwner(secretKey uint64) Owner {
	sk := Element(secretKey)
	pk := zkhash.Hash(sk)
	return Owner{SecretKey: sk, PublicKey: pk, Address: zkhash.Hash(pk)}
}

func NewNote(info DocumentInfo, quantity uint64, state State, owner Owner, opening uint64) (Note, error) {
	documentHash, err := HashDocumentInfo(info)
	if err != nil {
		return Note{}, err
	}
	o := Element(opening)
	return Note{
		Commitment:   Commitment(documentHash, quantity, state, owner.Address, o),
		DocumentHash: documentHash, Quantity: quantity, State: state, Opening: o, Owner: owner,
	}, nil
}

func NewNoteFromHash(documentHash fr.Element, quantity uint64, state State, owner Owner, opening uint64) Note {
	o := Element(opening)
	return Note{
		Commitment:   Commitment(documentHash, quantity, state, owner.Address, o),
		DocumentHash: documentHash, Quantity: quantity, State: state, Opening: o, Owner: owner,
	}
}

func NewVoucher(note Note, quantity uint64, state State, receiver Owner, deltaEpoch, opening uint64) Voucher {
	o := Element(opening)
	inputs := []fr.Element{note.DocumentHash, Element(quantity)}
	for _, value := range state {
		inputs = append(inputs, Element(value))
	}
	inputs = append(inputs, note.Owner.Address, receiver.Address, Element(deltaEpoch), o)
	return Voucher{
		Commitment: zkhash.Hash(inputs...), DocumentHash: note.DocumentHash, Quantity: quantity,
		State: state, SenderAddress: note.Owner.Address, ReceiverAddress: receiver.Address,
		DeltaEpoch: deltaEpoch, Opening: o,
	}
}

func Commitment(documentHash fr.Element, quantity uint64, state State, address, opening fr.Element) fr.Element {
	inputs := []fr.Element{documentHash, Element(quantity)}
	for _, value := range state {
		inputs = append(inputs, Element(value))
	}
	inputs = append(inputs, address, opening)
	return zkhash.Hash(inputs...)
}

func Nullifier(owner Owner, commitment fr.Element) fr.Element {
	return zkhash.Hash(owner.SecretKey, commitment)
}

func VoucherNullifier(voucher Voucher) fr.Element {
	return zkhash.Hash(voucher.Commitment, voucher.Opening)
}

func Element(value uint64) fr.Element {
	var result fr.Element
	result.SetUint64(value)
	return result
}

func HashDocumentInfo(info DocumentInfo) (fr.Element, error) {
	return zkhash.DocumentHash(info.ProductName, info.LotID, info.Unit)
}

func EncodeDocumentInfo(info DocumentInfo) ([]byte, error) {
	return zkhash.EncodeDocumentInfo(info.ProductName, info.LotID, info.Unit)
}

func FieldDecimal(value fr.Element) string {
	var integer big.Int
	value.BigInt(&integer)
	return integer.String()
}
