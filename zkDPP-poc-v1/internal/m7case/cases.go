package m7case

import (
	"fmt"
	aentry "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_entry"
	aexit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_exit"
	amerge "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_merge"
	aproceed "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_proceed"
	aprocess "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_process_3_2"
	arecall "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_recall"
	asplit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_split"
	atransfer "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_transfer"
	bentry "github.com/bighim/zkDPP/zkDPP-poc-v1/features/entry"
	bmerge "github.com/bighim/zkDPP/zkDPP-poc-v1/features/merge"
	bexit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/private_spend"
	bproceed "github.com/bighim/zkDPP/zkDPP-poc-v1/features/proceed"
	process "github.com/bighim/zkDPP/zkDPP-poc-v1/features/process_policy_3_2"
	brecall "github.com/bighim/zkDPP/zkDPP-poc-v1/features/recall"
	bsplit "github.com/bighim/zkDPP/zkDPP-poc-v1/features/split"
	btransfer "github.com/bighim/zkDPP/zkDPP-poc-v1/features/transfer"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/allocation"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/document"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/voucher"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m3case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m4case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/m5case"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
	"math/big"
	"path/filepath"
	"time"
)

type Case struct {
	Name                   string
	Kind                   audit.Kind
	Definition, Assignment frontend.Circuit
	Base, Inputs, Message  []fr.Element
	Context                fr.Element
	Record                 audit.Record
	Randomness             *big.Int
	EncryptMillis          float64
}
type Group struct {
	Name                    string
	Events, Extra           []*Case
	NoteRoot, VoucherRoot   fr.Element
	NoteCount, VoucherCount uint64
	NotePaths, VoucherPaths []merkle.Path
}

func Definitions(pk auditcrypto.PublicKey) map[audit.Kind]frontend.Circuit {
	return map[audit.Kind]frontend.Circuit{
		audit.Entry: aentry.New(pk), audit.Exit: aexit.New(pk), audit.Transfer: atransfer.New(pk),
		audit.Proceed: aproceed.New(pk), audit.Recall: arecall.New(pk), audit.Merge: amerge.New(pk), audit.Split: asplit.New(pk), audit.Process: aprocess.New(pk),
	}
}
func New(name string, k audit.Kind, base frontend.Circuit, sk fr.Element, parents, spends []fr.Element, pk auditcrypto.PublicKey) (*Case, error) {
	w, e := frontend.NewWitness(base, ecc.BLS12_381.ScalarField())
	if e != nil {
		return nil, e
	}
	pub, e := w.Public()
	if e != nil {
		return nil, e
	}
	p := []fr.Element(pub.Vector().(fr.Vector))
	context, e := audit.Context(k, p)
	if e != nil {
		return nil, e
	}
	m := append(append([]fr.Element{}, parents...), spends...)
	r, e := auditcrypto.RandomScalar(nil)
	if e != nil {
		return nil, e
	}
	start := time.Now()
	ct, e := auditcrypto.Encrypt(pk, context, m, r)
	elapsed := time.Since(start)
	if e != nil {
		return nil, e
	}
	return WithCipher(name, k, base, sk, parents, spends, pk, r, ct, float64(elapsed.Nanoseconds())/1e6)
}
func WithCipher(name string, k audit.Kind, base frontend.Circuit, sk fr.Element, parents, spends []fr.Element, pk auditcrypto.PublicKey, r *big.Int, ct auditcrypto.Ciphertext, encMS float64) (*Case, error) {
	w, e := frontend.NewWitness(base, ecc.BLS12_381.ScalarField())
	if e != nil {
		return nil, e
	}
	pub, e := w.Public()
	if e != nil {
		return nil, e
	}
	p := []fr.Element(pub.Vector().(fr.Vector))
	l, e := audit.Shape(k)
	if e != nil {
		return nil, e
	}
	context, e := audit.Context(k, p)
	if e != nil {
		return nil, e
	}
	record, e := audit.NewRecord(k, p, ct)
	if e != nil {
		return nil, e
	}
	env := circuitutil.NewAuditWitness(k)
	env.R1 = ed.Point{X: ct.R1.X, Y: ct.R1.Y}
	env.Randomness = r
	for i := range env.Parents {
		env.Parents[i] = ct.Data[i]
	}
	for i := range env.OutputNfs {
		env.OutputNfs[i] = ct.Data[l.ParentCount+i]
	}
	var assignment, definition frontend.Circuit
	switch v := base.(type) {
	case *bentry.Circuit:
		c := aentry.New(pk)
		c.Base = *v
		c.SKOwner = sk
		c.Audit = env
		assignment = c
		definition = aentry.New(pk)
	case *bexit.Circuit:
		c := aexit.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = aexit.New(pk)
	case *btransfer.Circuit:
		c := atransfer.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = atransfer.New(pk)
	case *bproceed.Circuit:
		c := aproceed.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = aproceed.New(pk)
	case *brecall.Circuit:
		c := arecall.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = arecall.New(pk)
	case *bmerge.Circuit:
		c := amerge.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = amerge.New(pk)
	case *bsplit.Circuit:
		c := asplit.New(pk)
		c.Base = *v
		c.Audit = env
		assignment = c
		definition = asplit.New(pk)
	case *process.Circuit:
		assignment = aprocess.Assignment(pk, v, r, ct)
		definition = aprocess.New(pk)
	default:
		return nil, fmt.Errorf("unsupported base Circuit %T", base)
	}
	inputs, e := audit.Public(k, p, ct)
	if e != nil {
		return nil, e
	}
	return &Case{name, k, definition, assignment, p, inputs, append(append([]fr.Element{}, parents...), spends...), context, record, r, encMS}, nil
}
func Build(root string, pk auditcrypto.PublicKey) ([]Group, error) {
	actors, e := testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v1.json"))
	if e != nil {
		return nil, e
	}
	a1, _ := testkit.ByID(actors, "actor-1")
	a2, _ := testkit.ByID(actors, "actor-2")
	var buildErr error
	makeCase := func(name string, k audit.Kind, b frontend.Circuit, sk fr.Element, parents, spends []fr.Element) *Case {
		if buildErr != nil {
			return nil
		}
		c, e := New(name, k, b, sk, parents, spends, pk)
		if e != nil {
			buildErr = e
		}
		return c
	}
	entry := func(name string, n note.Note) *Case {
		return makeCase(name, audit.Entry, bentry.Assignment(n), a1.SKOwner, nil, []fr.Element{note.Nullifier(a1.SKOwner, n.Commitment)})
	}
	tree := merkle.New()
	vals := []note.State{{10e9, 3e9, 5e9}, {6e9, 1800e6, 3e9}, {4e9, 1200e6, 2e9}, {2e9, 600e6, 1e9}, {4e9, 1200e6, 2e9}}
	ns := make([]note.Note, 5)
	for i, s := range vals {
		d, e := document.Hash(document.DocumentInfo{ProductName: "M7 Material", LotID: fmt.Sprintf("M7-%c", 'A'+i)})
		if e != nil {
			return nil, e
		}
		ns[i], e = note.New(d, s, note.AssetRoleEligible, a1.Address, zkhash.Element(uint64(7001+i)))
		if e != nil {
			return nil, e
		}
	}
	g := Group{Name: "note-graph", VoucherRoot: merkle.New().Root()}
	g.Events = append(g.Events, entry("entry-a", ns[0]))
	tree.Append(ns[0].Commitment)
	for _, step := range [][3]int{{0, 1, 2}, {1, 3, 4}} {
		i, j, k := step[0], step[1], step[2]
		path, _ := tree.Path(uint64(i))
		_, _, rem, e := allocation.ByMass(ns[i].State, ns[j].State.QMass)
		if e != nil {
			return nil, e
		}
		b := bsplit.Assignment(ns[i], a1.SKOwner, tree.Root(), path, ns[j], ns[k], rem)
		g.Events = append(g.Events, makeCase(fmt.Sprintf("split-%c", 'a'+i), audit.Split, b, a1.SKOwner, []fr.Element{ns[i].Commitment}, []fr.Element{note.Nullifier(a1.SKOwner, ns[j].Commitment), note.Nullifier(a1.SKOwner, ns[k].Commitment)}))
		tree.Append(ns[j].Commitment)
		tree.Append(ns[k].Commitment)
	}
	for i := 2; i < 5; i++ {
		path, _ := tree.Path(uint64(i))
		b := bexit.Assignment(ns[i], a1.SKOwner, tree.Root(), path)
		c := makeCase(fmt.Sprintf("exit-%c", 'a'+i), audit.Exit, b, a1.SKOwner, []fr.Element{ns[i].Commitment}, nil)
		if i == 2 {
			g.Events = append(g.Events, c)
		} else {
			g.Extra = append(g.Extra, c)
		}
	}
	g.NoteRoot = tree.Root()
	g.NoteCount = tree.Count()
	for i := uint64(0); i < tree.Count(); i++ {
		p, _ := tree.Path(i)
		g.NotePaths = append(g.NotePaths, p)
	}
	groups := []Group{g}
	v, e := m3case.Build(root)
	if e != nil {
		return nil, e
	}
	vg := Group{Name: "voucher", NoteRoot: v.FinalNoteRoot, VoucherRoot: v.FinalVoucherRoot, NoteCount: v.FinalNoteCount, VoucherCount: v.FinalVoucherCount, NotePaths: v.FinalNotePaths, VoucherPaths: v.FinalVoucherPaths}
	for _, x := range v.Entries {
		vg.Events = append(vg.Events, entry(x.Name, x.Note))
	}
	for _, x := range []m3case.TransferCase{v.Partial, v.Full} {
		vg.Events = append(vg.Events, makeCase("transfer-"+x.Name, audit.Transfer, x.Assignment, a1.SKOwner, []fr.Element{x.Input.Commitment}, []fr.Element{voucher.Nullifier(x.Voucher.Opening, x.Voucher.Commitment), note.Nullifier(a1.SKOwner, x.Change.Commitment)}))
		if x.Name == "partial" {
			x := v.Proceed
			vg.Events = append(vg.Events, makeCase("proceed", audit.Proceed, x.Assignment, a2.SKOwner, []fr.Element{x.Voucher.Commitment}, []fr.Element{note.Nullifier(a2.SKOwner, x.Output.Commitment)}))
		}
	}
	vg.Events = append(vg.Events, makeCase("recall", audit.Recall, v.Recall.Assignment, a1.SKOwner, []fr.Element{v.Recall.Voucher.Commitment}, []fr.Element{note.Nullifier(a1.SKOwner, v.Recall.Output.Commitment)}))
	groups = append(groups, vg)
	m, e := m4case.Build(root)
	if e != nil {
		return nil, e
	}
	mg := Group{Name: "merge", NoteRoot: m.FinalRoot, VoucherRoot: merkle.New().Root(), NoteCount: m.FinalCount, NotePaths: m.FinalPaths}
	for _, x := range m.Entries {
		mg.Events = append(mg.Events, entry(x.Name, x.Note))
	}
	mg.Events = append(mg.Events, makeCase("merge", audit.Merge, m.Merge.Assignment, a1.SKOwner, []fr.Element{m.Merge.Inputs[0].Commitment, m.Merge.Inputs[1].Commitment}, []fr.Element{note.Nullifier(a1.SKOwner, m.Merge.Output.Commitment)}))
	mg.Events = append(mg.Events, makeCase("split-merged", audit.Split, m.Split.Assignment, a1.SKOwner, []fr.Element{m.Split.Input.Commitment}, []fr.Element{note.Nullifier(a1.SKOwner, m.Split.Outputs[0].Commitment), note.Nullifier(a1.SKOwner, m.Split.Outputs[1].Commitment)}))
	groups = append(groups, mg)
	p, e := m5case.Build(root)
	if e != nil {
		return nil, e
	}
	pt := merkle.New()
	pg := Group{Name: "process", VoucherRoot: merkle.New().Root()}
	for i, n := range p.Inputs {
		pg.Events = append(pg.Events, entry(fmt.Sprintf("process-input-%d", i), n))
		pt.Append(n.Commitment)
	}
	pg.Events = append(pg.Events, makeCase("process", audit.Process, p.Assignment, a1.SKOwner, []fr.Element{p.Inputs[0].Commitment, p.Inputs[1].Commitment, p.Inputs[2].Commitment}, []fr.Element{note.Nullifier(a1.SKOwner, p.Outputs[0].Commitment), note.Nullifier(a1.SKOwner, p.Outputs[1].Commitment)}))
	for _, n := range p.Outputs {
		pt.Append(n.Commitment)
	}
	pg.NoteRoot = pt.Root()
	pg.NoteCount = pt.Count()
	for i := uint64(0); i < pt.Count(); i++ {
		p, _ := pt.Path(i)
		pg.NotePaths = append(pg.NotePaths, p)
	}
	groups = append(groups, pg)
	if buildErr != nil {
		return nil, buildErr
	}
	return groups, nil
}
