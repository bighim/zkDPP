package m6case

import (
	statusmerge "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_merge"
	statusprivatespend "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_private_spend"
	statusproceed "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_proceed"
	statusprocess "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_process"
	statusrecall "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_recall"
	statussplit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_split"
	statustransfer "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_transfer"
	statusupdate "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_update"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m2case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m3case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m4case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m5case"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type Scenario struct {
	NoteStatusRoot, VoucherStatusRoot fr.Element
	StatusUpdateOldRoot               fr.Element
	StatusUpdateNewRoot               fr.Element
	StatusUpdate                      *statusupdate.Circuit
	StatusUnfreeze                    *statusupdate.Circuit
	StatusRevoke                      *statusupdate.Circuit
	PrivateSpend                      *statusprivatespend.Circuit
	Transfer                          *statustransfer.Circuit
	FullTransfer                      *statustransfer.Circuit
	Proceed                           *statusproceed.Circuit
	Recall                            *statusrecall.Circuit
	Merge                             *statusmerge.Circuit
	Split                             *statussplit.Circuit
	Process                           *statusprocess.Circuit
}

func Build(root string) (*Scenario, error) {
	noteStatus, voucherStatus := status.New(), status.New()
	statusPath := func(tree *status.Tree, index uint64) status.Path {
		path, _ := tree.Path(index)
		return path
	}
	m2, err := m2case.Build(root)
	if err != nil {
		return nil, err
	}
	m3, err := m3case.Build(root)
	if err != nil {
		return nil, err
	}
	m4, err := m4case.Build(root)
	if err != nil {
		return nil, err
	}
	m5, err := m5case.Build(root)
	if err != nil {
		return nil, err
	}

	oldRoot := noteStatus.Root()
	updatePath := statusPath(noteStatus, 1)
	newRoot := status.ComputeRoot(status.Frozen, 1, updatePath)
	update := statusupdate.Assignment(status.ObjectTypeNote, oldRoot, newRoot, 1, status.Active, status.Frozen, updatePath)
	frozenTree := status.New()
	_, _ = frozenTree.Apply(1, status.Active, status.Frozen)
	frozenPath := statusPath(frozenTree, 1)
	unfreeze := statusupdate.Assignment(status.ObjectTypeNote, frozenTree.Root(), oldRoot, 1, status.Frozen, status.Active, frozenPath)
	revoke := statusupdate.Assignment(status.ObjectTypeNote, frozenTree.Root(), status.ComputeRoot(status.Revoked, 1, frozenPath), 1, status.Frozen, status.Revoked, frozenPath)

	mergePaths := [2]status.Path{statusPath(noteStatus, 0), statusPath(noteStatus, 1)}
	processPaths := [3]status.Path{statusPath(noteStatus, 0), statusPath(noteStatus, 1), statusPath(noteStatus, 2)}
	return &Scenario{
		NoteStatusRoot: noteStatus.Root(), VoucherStatusRoot: voucherStatus.Root(),
		StatusUpdateOldRoot: oldRoot, StatusUpdateNewRoot: newRoot, StatusUpdate: update, StatusUnfreeze: unfreeze, StatusRevoke: revoke,
		PrivateSpend: statusprivatespend.FromBase(m2.ExitAssignment, noteStatus.Root(), statusPath(noteStatus, 0)),
		Transfer:     statustransfer.FromBase(m3.Partial.Assignment, noteStatus.Root(), statusPath(noteStatus, 0)),
		FullTransfer: statustransfer.FromBase(m3.Full.Assignment, noteStatus.Root(), statusPath(noteStatus, 1)),
		Proceed:      statusproceed.FromBase(m3.Proceed.Assignment, voucherStatus.Root(), statusPath(voucherStatus, 0)),
		Recall:       statusrecall.FromBase(m3.Recall.Assignment, voucherStatus.Root(), statusPath(voucherStatus, 1)),
		Merge:        statusmerge.FromBase(m4.Merge.Assignment, noteStatus.Root(), mergePaths),
		Split:        statussplit.FromBase(m4.Split.Assignment, noteStatus.Root(), statusPath(noteStatus, 2)),
		Process:      statusprocess.FromBase(m5.Assignment, noteStatus.Root(), processPaths),
	}, nil
}
