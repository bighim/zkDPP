package scenario

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bighim/zkDPP/poc-v2/internal/merkle"
	"github.com/bighim/zkDPP/poc-v2/internal/protocol"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Raw struct {
	Profile struct {
		StateLength, TreeDepth, MaxProcessInputs, MaxProcessOutputs string
	} `json:"profile"`
	Epoch struct {
		Start, ProceedDelta, RecallDelta, RecallAt string
	} `json:"epoch"`
	Actors map[string]struct {
		SecretKey string `json:"secretKey"`
	} `json:"actors"`
	Documents map[string]protocol.DocumentInfo `json:"documents"`
	Entries   []struct {
		ID       string   `json:"id"`
		Document string   `json:"document"`
		Owner    string   `json:"owner"`
		Quantity string   `json:"quantity"`
		State    []string `json:"state"`
		Opening  string   `json:"opening"`
	} `json:"entries"`
	Openings  map[string]string `json:"openings"`
	Processes map[string]struct {
		Inputs           []string   `json:"inputs"`
		Outputs          []string   `json:"outputs"`
		OutputDocuments  []string   `json:"outputDocuments"`
		OutputQuantities []string   `json:"outputQuantities"`
		Owner            string     `json:"owner"`
		PublicDelta      []string   `json:"publicDelta"`
		Allocation       [][]string `json:"allocation"`
		OutputOpenings   []string   `json:"outputOpenings"`
	} `json:"processes"`
	Split struct {
		Input, Output1, Output2, Quantity1, Quantity2, Owner, Opening1, Opening2 string
	} `json:"split"`
	EventChain []string `json:"eventChain"`
}

type Membership struct {
	Root fr.Element
	Path merkle.Path
}

type TransferCase struct {
	Input, Change, Voucher, Sender, Receiver string
}

type ResolutionCase struct {
	Voucher, Output, Owner string
}

type MergeCase struct {
	Inputs [2]string
	Output string
	Owner  string
}

type SplitCase struct {
	Input, Output1, Output2, Owner string
}

type ProcessCase struct {
	Inputs, Outputs []string
	Owner           string
	M, N            int
	Delta           protocol.State
	Allocation      [protocol.MaxProcessArity][protocol.StateLength]uint64
}

type ExitCase struct {
	Input, Owner string
}

type Derived struct {
	Raw        Raw
	Owners     map[string]protocol.Owner
	Documents  map[string]fr.Element
	Notes      map[string]protocol.Note
	Vouchers   map[string]protocol.Voucher
	Nullifiers map[string]fr.Element
	VoucherNF  map[string]fr.Element
	MTRoots    map[string]fr.Element
	RVRoots    map[string]fr.Element
	MTWitness  map[string]Membership
	RVWitness  map[string]Membership
	Remainders map[string][protocol.StateLength]uint64
	Transfers  map[string]TransferCase
	Proceeds   map[string]ResolutionCase
	Recalls    map[string]ResolutionCase
	Merges     map[string]MergeCase
	Splits     map[string]SplitCase
	Processes  map[string]ProcessCase
	Exits      map[string]ExitCase
	EntryOrder []string
	MT         *merkle.Tree
	RVMT       *merkle.Tree
}

type Reference struct {
	DocumentHashes map[string]string           `json:"documentHashes"`
	Owners         map[string]OwnerRef         `json:"owners"`
	Notes          map[string]NoteRef          `json:"notes"`
	Vouchers       map[string]VoucherRef       `json:"vouchers"`
	Nullifiers     map[string]string           `json:"nullifiers"`
	VoucherNF      map[string]string           `json:"voucherNullifiers"`
	MTRoots        map[string]string           `json:"mtRoots"`
	RVRoots        map[string]string           `json:"rvRoots"`
	Remainders     map[string][]string         `json:"remainders"`
	Processes      map[string]ProcessReference `json:"processes"`
	Epoch          EpochReference              `json:"epoch"`
	FinalState     FinalStateReference         `json:"finalState"`
}

type OwnerRef struct {
	PublicKey string `json:"publicKey"`
	Address   string `json:"address"`
}

type NoteRef struct {
	Commitment   string   `json:"commitment"`
	DocumentHash string   `json:"documentHash"`
	Quantity     string   `json:"quantity"`
	State        []string `json:"state"`
	Opening      string   `json:"opening"`
	Index        string   `json:"index"`
	Owner        string   `json:"owner"`
}

type VoucherRef struct {
	Commitment string   `json:"commitment"`
	Quantity   string   `json:"quantity"`
	State      []string `json:"state"`
	DeltaEpoch string   `json:"deltaEpoch"`
	Opening    string   `json:"opening"`
	Index      string   `json:"index"`
}

type ProcessReference struct {
	M                string   `json:"m"`
	N                string   `json:"n"`
	InputAggregate   []string `json:"inputAggregate"`
	Intermediate     []string `json:"intermediate"`
	OutputAggregate  []string `json:"outputAggregate"`
	PublicInputCount string   `json:"publicInputCount"`
}

type EpochReference struct {
	RecallDeadline          string `json:"recallDeadline"`
	RecallAt                string `json:"recallAt"`
	RecallAtDeadlineAllowed bool   `json:"recallAtDeadlineAllowed"`
	ProceedAfterDeadline    bool   `json:"proceedAfterDeadlineAllowed"`
}

type FinalStateReference struct {
	MTLeaves   string `json:"mtLeaves"`
	RVMTLeaves string `json:"rvMTLeaves"`
}

func Load(path string) (*Derived, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw Raw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode canonical scenario: %w", err)
	}
	return Build(raw)
}

func LoadCanonical(root string) (*Derived, error) {
	return Load(filepath.Join(root, "testdata", "scenario", "canonical.json"))
}

func Build(raw Raw) (*Derived, error) {
	if err := validateProfile(raw); err != nil {
		return nil, err
	}
	d := &Derived{
		Raw: raw, Owners: map[string]protocol.Owner{}, Documents: map[string]fr.Element{}, Notes: map[string]protocol.Note{},
		Vouchers: map[string]protocol.Voucher{}, Nullifiers: map[string]fr.Element{}, VoucherNF: map[string]fr.Element{},
		MTRoots: map[string]fr.Element{}, RVRoots: map[string]fr.Element{}, MTWitness: map[string]Membership{},
		RVWitness: map[string]Membership{}, Remainders: map[string][3]uint64{}, Transfers: map[string]TransferCase{},
		Proceeds: map[string]ResolutionCase{}, Recalls: map[string]ResolutionCase{}, Merges: map[string]MergeCase{},
		Splits: map[string]SplitCase{}, Processes: map[string]ProcessCase{}, Exits: map[string]ExitCase{},
		MT: merkle.New(protocol.TreeDepth), RVMT: merkle.New(protocol.TreeDepth),
	}
	for name, actor := range raw.Actors {
		sk, err := parse(actor.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("actor %s: %w", name, err)
		}
		d.Owners[name] = protocol.NewOwner(sk)
	}
	for id, info := range raw.Documents {
		hash, err := protocol.HashDocumentInfo(info)
		if err != nil {
			return nil, fmt.Errorf("hash document %s: %w", id, err)
		}
		d.Documents[id] = hash
	}
	for _, entry := range raw.Entries {
		owner, err := d.owner(entry.Owner)
		if err != nil {
			return nil, err
		}
		state, err := parseState(entry.State)
		if err != nil {
			return nil, err
		}
		note := protocol.NewNoteFromHash(d.Documents[entry.Document], d.value(entry.Quantity), state, owner, d.value(entry.Opening))
		if err := d.appendNote("entry:"+entry.ID, entry.ID, &note); err != nil {
			return nil, err
		}
		d.EntryOrder = append(d.EntryOrder, entry.ID)
	}

	// The following sequence is the canonical 25-call scenario. Every operation
	// records enough metadata for assignments and reports to derive their inputs.
	initial, err := d.transfer("aluminum-initial", "aluminum-first", d.Notes["aluminum-first"].Quantity, "foil-manufacturer", d.value(raw.Epoch.RecallDelta), "transfer-aluminum-initial-change", "transfer-aluminum-initial-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.recall("aluminum-initial", initial, "recalled-aluminum", "recall-aluminum"); err != nil {
		return nil, err
	}

	retry, err := d.transfer("aluminum-retry", "recalled-aluminum", d.Notes["recalled-aluminum"].Quantity, "foil-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-aluminum-retry-change", "transfer-aluminum-retry-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("aluminum-retry", retry, "proceed-aluminum-first", "proceed-aluminum-retry"); err != nil {
		return nil, err
	}
	second, err := d.transfer("aluminum-second", "aluminum-second", d.Notes["aluminum-second"].Quantity, "foil-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-aluminum-second-change", "transfer-aluminum-second-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("aluminum-second", second, "proceed-aluminum-second", "proceed-aluminum-second"); err != nil {
		return nil, err
	}

	if err = d.process("foil-first"); err != nil {
		return nil, err
	}
	if err = d.process("foil-second"); err != nil {
		return nil, err
	}

	foilFirst, err := d.transfer("foil-first", "foil-first", d.Notes["foil-first"].Quantity, "cell-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-foil-first-change", "transfer-foil-first-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("foil-first", foilFirst, "proceed-foil-first", "proceed-foil-first"); err != nil {
		return nil, err
	}
	foilSecond, err := d.transfer("foil-second", "foil-second", d.Notes["foil-second"].Quantity, "cell-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-foil-second-change", "transfer-foil-second-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("foil-second", foilSecond, "proceed-foil-second", "proceed-foil-second"); err != nil {
		return nil, err
	}
	if err = d.merge("foil", "proceed-foil-first", "proceed-foil-second", "merged-foil", "merge-foil"); err != nil {
		return nil, err
	}

	cathode, err := d.transfer("cathode", "cathode", d.Notes["cathode"].Quantity, "cell-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-cathode-change", "transfer-cathode-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("cathode", cathode, "proceed-cathode", "proceed-cathode"); err != nil {
		return nil, err
	}
	anode, err := d.transfer("anode", "anode", d.Notes["anode"].Quantity, "cell-manufacturer", d.value(raw.Epoch.ProceedDelta), "transfer-anode-change", "transfer-anode-voucher")
	if err != nil {
		return nil, err
	}
	if _, err = d.proceed("anode", anode, "proceed-anode", "proceed-anode"); err != nil {
		return nil, err
	}

	if err = d.process("cell"); err != nil {
		return nil, err
	}
	if err = d.split(); err != nil {
		return nil, err
	}
	if err = d.exit("cell", raw.Split.Output1); err != nil {
		return nil, err
	}
	if err = d.exit("waste", "waste"); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Derived) transfer(name, inputName string, qTransfer uint64, receiverName string, delta uint64, changeOpening, voucherOpening string) (protocol.Voucher, error) {
	in, ok := d.Notes[inputName]
	if !ok {
		return protocol.Voucher{}, fmt.Errorf("transfer %s missing input %s", name, inputName)
	}
	if err := d.captureMT("transfer:"+name, in); err != nil {
		return protocol.Voucher{}, err
	}
	receiver, err := d.owner(receiverName)
	if err != nil {
		return protocol.Voucher{}, err
	}
	if qTransfer == 0 || qTransfer > in.Quantity {
		return protocol.Voucher{}, fmt.Errorf("invalid transfer quantity for %s", name)
	}
	qChange := in.Quantity - qTransfer
	changeState, rem := proportionalFloor(in.State, qChange, in.Quantity)
	transferState, err := subState(in.State, changeState)
	if err != nil {
		return protocol.Voucher{}, err
	}
	changeName := name + "-change"
	change := protocol.NewNoteFromHash(in.DocumentHash, qChange, changeState, in.Owner, d.opening(changeOpening))
	voucher := protocol.NewVoucher(in, qTransfer, transferState, receiver, delta, d.opening(voucherOpening))
	d.Nullifiers["transfer:"+name] = protocol.Nullifier(in.Owner, in.Commitment)
	d.Remainders["transfer:"+name+":change"] = rem
	if err := d.appendNote("transfer:"+name+":change", changeName, &change); err != nil {
		return protocol.Voucher{}, err
	}
	index, root, err := d.RVMT.Append(voucher.Commitment)
	if err != nil {
		return protocol.Voucher{}, err
	}
	voucher.Index = index
	d.Vouchers[name] = voucher
	d.RVRoots["transfer:"+name] = root
	senderName, err := d.ownerName(in.Owner)
	if err != nil {
		return protocol.Voucher{}, err
	}
	d.Transfers[name] = TransferCase{Input: inputName, Change: changeName, Voucher: name, Sender: senderName, Receiver: receiverName}
	return voucher, nil
}

func (d *Derived) proceed(name string, voucher protocol.Voucher, outputName, openingName string) (protocol.Note, error) {
	if err := d.captureRV("proceed:"+name, voucher); err != nil {
		return protocol.Note{}, err
	}
	receiverName, err := d.ownerNameByAddress(voucher.ReceiverAddress)
	if err != nil {
		return protocol.Note{}, err
	}
	receiver := d.Owners[receiverName]
	note := protocol.NewNoteFromHash(voucher.DocumentHash, voucher.Quantity, voucher.State, receiver, d.opening(openingName))
	d.VoucherNF["proceed:"+name] = protocol.VoucherNullifier(voucher)
	if err := d.appendNote("proceed:"+name, outputName, &note); err != nil {
		return protocol.Note{}, err
	}
	d.Proceeds[name] = ResolutionCase{Voucher: name, Output: outputName, Owner: receiverName}
	return note, nil
}

func (d *Derived) recall(name string, voucher protocol.Voucher, outputName, openingName string) (protocol.Note, error) {
	if err := d.captureRV("recall:"+name, voucher); err != nil {
		return protocol.Note{}, err
	}
	senderName, err := d.ownerNameByAddress(voucher.SenderAddress)
	if err != nil {
		return protocol.Note{}, err
	}
	sender := d.Owners[senderName]
	note := protocol.NewNoteFromHash(voucher.DocumentHash, voucher.Quantity, voucher.State, sender, d.opening(openingName))
	d.VoucherNF["recall:"+name] = protocol.VoucherNullifier(voucher)
	if err := d.appendNote("recall:"+name, outputName, &note); err != nil {
		return protocol.Note{}, err
	}
	d.Recalls[name] = ResolutionCase{Voucher: name, Output: outputName, Owner: senderName}
	return note, nil
}

func (d *Derived) process(name string) error {
	spec, ok := d.Raw.Processes[name]
	if !ok {
		return fmt.Errorf("missing process %s", name)
	}
	if len(spec.Inputs) < 1 || len(spec.Inputs) > 3 || len(spec.Outputs) < 1 || len(spec.Outputs) > 3 {
		return fmt.Errorf("invalid process arity %s", name)
	}
	if len(spec.Outputs) != len(spec.OutputDocuments) || len(spec.Outputs) != len(spec.OutputQuantities) || len(spec.Outputs) != len(spec.OutputOpenings) || len(spec.Allocation) != len(spec.Outputs) {
		return fmt.Errorf("inconsistent process outputs %s", name)
	}
	owner, err := d.owner(spec.Owner)
	if err != nil {
		return err
	}
	var aggregate protocol.State
	for i, inputName := range spec.Inputs {
		note, exists := d.Notes[inputName]
		if !exists {
			return fmt.Errorf("process %s missing input %s", name, inputName)
		}
		if !note.Owner.Address.Equal(&owner.Address) {
			return fmt.Errorf("process %s input owner mismatch", name)
		}
		if err := d.captureMT(fmt.Sprintf("process:%s:%d", name, i), note); err != nil {
			return err
		}
		aggregate, err = addState(aggregate, note.State)
		if err != nil {
			return err
		}
		d.Nullifiers[fmt.Sprintf("process:%s:%d", name, i)] = protocol.Nullifier(owner, note.Commitment)
	}
	delta, err := parseState(spec.PublicDelta)
	if err != nil {
		return err
	}
	if aggregate[0] < delta[0] {
		return fmt.Errorf("process %s total loss exceeds aggregate", name)
	}
	intermediate := protocol.State{aggregate[0] - delta[0], aggregate[1] + delta[1], aggregate[2] + delta[2]}
	var allocation [3][3]uint64
	for j := range spec.Allocation {
		if len(spec.Allocation[j]) != 3 {
			return fmt.Errorf("process %s allocation row %d", name, j)
		}
		for k := 0; k < 3; k++ {
			allocation[j][k] = d.value(spec.Allocation[j][k])
		}
	}
	var outputs [3]protocol.State
	for k := 0; k < 3; k++ {
		var sumA, allocated uint64
		for j := 0; j < len(spec.Outputs); j++ {
			sumA += allocation[j][k]
		}
		if sumA != protocol.AllocationDenominator {
			return fmt.Errorf("process %s allocation column %d sums to %d", name, k, sumA)
		}
		for j := 1; j < len(spec.Outputs); j++ {
			q, r := quotientRemainder(intermediate[k], allocation[j][k], protocol.AllocationDenominator)
			outputs[j][k] = q
			key := fmt.Sprintf("process:%s:output%d", name, j+1)
			rem := d.Remainders[key]
			rem[k] = r
			d.Remainders[key] = rem
			allocated += q
		}
		outputs[0][k] = intermediate[k] - allocated
	}
	for j, outputName := range spec.Outputs {
		note := protocol.NewNoteFromHash(d.Documents[spec.OutputDocuments[j]], d.value(spec.OutputQuantities[j]), outputs[j], owner, d.opening(spec.OutputOpenings[j]))
		if err := d.appendNote(fmt.Sprintf("process:%s:%d", name, j), outputName, &note); err != nil {
			return err
		}
	}
	d.Processes[name] = ProcessCase{Inputs: append([]string(nil), spec.Inputs...), Outputs: append([]string(nil), spec.Outputs...), Owner: spec.Owner, M: len(spec.Inputs), N: len(spec.Outputs), Delta: delta, Allocation: allocation}
	return nil
}

func (d *Derived) merge(name, input1, input2, output, openingName string) error {
	a, b := d.Notes[input1], d.Notes[input2]
	if !a.Owner.Address.Equal(&b.Owner.Address) {
		return fmt.Errorf("merge %s owner mismatch", name)
	}
	if !a.DocumentHash.Equal(&b.DocumentHash) {
		return fmt.Errorf("merge %s document mismatch", name)
	}
	if err := d.captureMT("merge:"+name+":1", a); err != nil {
		return err
	}
	if err := d.captureMT("merge:"+name+":2", b); err != nil {
		return err
	}
	state, err := addState(a.State, b.State)
	if err != nil {
		return err
	}
	note := protocol.NewNoteFromHash(a.DocumentHash, a.Quantity+b.Quantity, state, a.Owner, d.opening(openingName))
	if err := d.appendNote("merge:"+name, output, &note); err != nil {
		return err
	}
	ownerName, err := d.ownerName(a.Owner)
	if err != nil {
		return err
	}
	d.Nullifiers["merge:"+name+":1"] = protocol.Nullifier(a.Owner, a.Commitment)
	d.Nullifiers["merge:"+name+":2"] = protocol.Nullifier(a.Owner, b.Commitment)
	d.Merges[name] = MergeCase{Inputs: [2]string{input1, input2}, Output: output, Owner: ownerName}
	return nil
}

func (d *Derived) split() error {
	spec := d.Raw.Split
	in := d.Notes[spec.Input]
	owner, err := d.owner(spec.Owner)
	if err != nil {
		return err
	}
	if !in.Owner.Address.Equal(&owner.Address) {
		return fmt.Errorf("split owner mismatch")
	}
	if err := d.captureMT("split:cell", in); err != nil {
		return err
	}
	q1, q2 := d.value(spec.Quantity1), d.value(spec.Quantity2)
	if q1+q2 != in.Quantity {
		return fmt.Errorf("split quantities do not conserve quantity")
	}
	s2, rem := proportionalFloor(in.State, q2, in.Quantity)
	s1, err := subState(in.State, s2)
	if err != nil {
		return err
	}
	n1 := protocol.NewNoteFromHash(in.DocumentHash, q1, s1, owner, d.opening(spec.Opening1))
	n2 := protocol.NewNoteFromHash(in.DocumentHash, q2, s2, owner, d.opening(spec.Opening2))
	d.Remainders["split:cell:output2"] = rem
	d.Nullifiers["split:cell"] = protocol.Nullifier(owner, in.Commitment)
	if err := d.appendNote("split:cell:1", spec.Output1, &n1); err != nil {
		return err
	}
	if err := d.appendNote("split:cell:2", spec.Output2, &n2); err != nil {
		return err
	}
	d.Splits["cell"] = SplitCase{Input: spec.Input, Output1: spec.Output1, Output2: spec.Output2, Owner: spec.Owner}
	return nil
}

func (d *Derived) exit(name, input string) error {
	note := d.Notes[input]
	if err := d.captureMT("exit:"+name, note); err != nil {
		return err
	}
	ownerName, err := d.ownerName(note.Owner)
	if err != nil {
		return err
	}
	d.Nullifiers["exit:"+name] = protocol.Nullifier(note.Owner, note.Commitment)
	d.Exits[name] = ExitCase{Input: input, Owner: ownerName}
	return nil
}

func (d *Derived) captureMT(name string, note protocol.Note) error {
	path, err := d.MT.Path(note.Index)
	if err != nil {
		return err
	}
	d.MTWitness[name] = Membership{Root: d.MT.Root(), Path: path}
	return nil
}

func (d *Derived) captureRV(name string, voucher protocol.Voucher) error {
	path, err := d.RVMT.Path(voucher.Index)
	if err != nil {
		return err
	}
	d.RVWitness[name] = Membership{Root: d.RVMT.Root(), Path: path}
	return nil
}

func (d *Derived) appendNote(rootName, noteName string, note *protocol.Note) error {
	index, root, err := d.MT.Append(note.Commitment)
	if err != nil {
		return err
	}
	note.Index = index
	d.Notes[noteName] = *note
	d.MTRoots[rootName] = root
	return nil
}

func (d *Derived) owner(name string) (protocol.Owner, error) {
	owner, ok := d.Owners[name]
	if !ok {
		return protocol.Owner{}, fmt.Errorf("unknown owner %s", name)
	}
	return owner, nil
}

func (d *Derived) ownerName(owner protocol.Owner) (string, error) {
	return d.ownerNameByAddress(owner.Address)
}
func (d *Derived) ownerNameByAddress(address fr.Element) (string, error) {
	for name, owner := range d.Owners {
		if owner.Address.Equal(&address) {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown owner address")
}

func (d *Derived) Reference() Reference {
	ref := Reference{DocumentHashes: map[string]string{}, Owners: map[string]OwnerRef{}, Notes: map[string]NoteRef{}, Vouchers: map[string]VoucherRef{}, Nullifiers: map[string]string{}, VoucherNF: map[string]string{}, MTRoots: map[string]string{}, RVRoots: map[string]string{}, Remainders: map[string][]string{}, Processes: map[string]ProcessReference{}}
	for name, hash := range d.Documents {
		ref.DocumentHashes[name] = protocol.FieldDecimal(hash)
	}
	for name, owner := range d.Owners {
		ref.Owners[name] = OwnerRef{PublicKey: protocol.FieldDecimal(owner.PublicKey), Address: protocol.FieldDecimal(owner.Address)}
	}
	for name, note := range d.Notes {
		owner, _ := d.ownerName(note.Owner)
		ref.Notes[name] = NoteRef{Commitment: protocol.FieldDecimal(note.Commitment), DocumentHash: protocol.FieldDecimal(note.DocumentHash), Quantity: u(note.Quantity), State: stateStrings(note.State), Opening: protocol.FieldDecimal(note.Opening), Index: u(note.Index), Owner: owner}
	}
	for name, voucher := range d.Vouchers {
		ref.Vouchers[name] = VoucherRef{Commitment: protocol.FieldDecimal(voucher.Commitment), Quantity: u(voucher.Quantity), State: stateStrings(voucher.State), DeltaEpoch: u(voucher.DeltaEpoch), Opening: protocol.FieldDecimal(voucher.Opening), Index: u(voucher.Index)}
	}
	for name, value := range d.Nullifiers {
		ref.Nullifiers[name] = protocol.FieldDecimal(value)
	}
	for name, value := range d.VoucherNF {
		ref.VoucherNF[name] = protocol.FieldDecimal(value)
	}
	for name, value := range d.MTRoots {
		ref.MTRoots[name] = protocol.FieldDecimal(value)
	}
	for name, value := range d.RVRoots {
		ref.RVRoots[name] = protocol.FieldDecimal(value)
	}
	for name, value := range d.Remainders {
		ref.Remainders[name] = stateStrings(value)
	}
	for name, process := range d.Processes {
		var inputStates, outputStates []protocol.State
		for _, item := range process.Inputs {
			inputStates = append(inputStates, d.Notes[item].State)
		}
		for _, item := range process.Outputs {
			outputStates = append(outputStates, d.Notes[item].State)
		}
		inputAggregate := sumStates(inputStates...)
		intermediate := protocol.State{inputAggregate[0] - process.Delta[0], inputAggregate[1] + process.Delta[1], inputAggregate[2] + process.Delta[2]}
		ref.Processes[name] = ProcessReference{M: u(uint64(process.M)), N: u(uint64(process.N)), InputAggregate: stateStrings(inputAggregate), Intermediate: stateStrings(intermediate), OutputAggregate: stateStrings(sumStates(outputStates...)), PublicInputCount: u(uint64(len(protocol.CanonicalManifests()[6].Inputs)))}
	}
	deadline := d.value(d.Raw.Epoch.Start) + d.value(d.Raw.Epoch.RecallDelta)
	ref.Epoch = EpochReference{RecallDeadline: u(deadline), RecallAt: d.Raw.Epoch.RecallAt, RecallAtDeadlineAllowed: false, ProceedAfterDeadline: true}
	ref.FinalState = FinalStateReference{MTLeaves: u(d.MT.Count()), RVMTLeaves: u(d.RVMT.Count())}
	return ref
}

func WriteReference(path string, reference Reference) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reference, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func validateProfile(raw Raw) error {
	values := []struct {
		name, got string
		want      uint64
	}{{"stateLength", raw.Profile.StateLength, 3}, {"treeDepth", raw.Profile.TreeDepth, 32}, {"maxProcessInputs", raw.Profile.MaxProcessInputs, 3}, {"maxProcessOutputs", raw.Profile.MaxProcessOutputs, 3}}
	for _, value := range values {
		parsed, err := parse(value.got)
		if err != nil || parsed != value.want {
			return fmt.Errorf("profile %s=%q, want %d", value.name, value.got, value.want)
		}
	}
	return nil
}

func proportionalFloor(state protocol.State, numerator, denominator uint64) (protocol.State, [3]uint64) {
	var out protocol.State
	var remainder [3]uint64
	for k := range state {
		out[k], remainder[k] = quotientRemainder(state[k], numerator, denominator)
	}
	return out, remainder
}

func quotientRemainder(value, numerator, denominator uint64) (uint64, uint64) {
	product := new(big.Int).Mul(new(big.Int).SetUint64(value), new(big.Int).SetUint64(numerator))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(product, new(big.Int).SetUint64(denominator), r)
	return q.Uint64(), r.Uint64()
}

func addState(a, b protocol.State) (protocol.State, error) {
	var out protocol.State
	for k := range a {
		out[k] = a[k] + b[k]
		if out[k] < a[k] {
			return out, fmt.Errorf("state overflow")
		}
	}
	return out, nil
}
func subState(a, b protocol.State) (protocol.State, error) {
	var out protocol.State
	for k := range a {
		if a[k] < b[k] {
			return out, fmt.Errorf("state underflow")
		}
		out[k] = a[k] - b[k]
	}
	return out, nil
}
func sumStates(states ...protocol.State) protocol.State {
	var out protocol.State
	for _, state := range states {
		for k := range out {
			out[k] += state[k]
		}
	}
	return out
}
func parseState(values []string) (protocol.State, error) {
	if len(values) != 3 {
		return protocol.State{}, fmt.Errorf("state length=%d, want 3", len(values))
	}
	var out protocol.State
	for i, value := range values {
		parsed, err := parse(value)
		if err != nil {
			return out, err
		}
		out[i] = parsed
	}
	return out, nil
}
func parse(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse uint64 %q: %w", value, err)
	}
	return parsed, nil
}
func (d *Derived) value(value string) uint64 {
	parsed, err := parse(value)
	if err != nil {
		panic(err)
	}
	return parsed
}
func (d *Derived) opening(name string) uint64 {
	value, ok := d.Raw.Openings[name]
	if !ok {
		panic("missing opening " + name)
	}
	return d.value(value)
}
func stateStrings(state protocol.State) []string {
	return []string{u(state[0]), u(state[1]), u(state[2])}
}
func u(value uint64) string { return strconv.FormatUint(value, 10) }
