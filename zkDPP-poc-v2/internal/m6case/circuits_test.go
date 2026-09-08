package m6case_test

import (
	"testing"

	statusmerge "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_merge"
	statusprivatespend "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_private_spend"
	statusproceed "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_proceed"
	statusprocess "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_process"
	statusrecall "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_recall"
	statussplit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_split"
	statustransfer "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_transfer"
	statusupdate "github.com/bighim/zkDPP/zkDPP-poc-v2/features/status_update"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/status"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m6case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
)

func TestM6ValidCircuits(t *testing.T) {
	s, err := m6case.Build("../..")
	if err != nil {
		t.Fatal(err)
	}
	assert := test.NewAssert(t)
	assert.SolvingSucceeded(&statusupdate.Circuit{}, s.StatusUpdate, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusupdate.Circuit{}, s.StatusUnfreeze, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusupdate.Circuit{}, s.StatusRevoke, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusprivatespend.Circuit{}, s.PrivateSpend, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statustransfer.Circuit{}, s.Transfer, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusproceed.Circuit{}, s.Proceed, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusrecall.Circuit{}, s.Recall, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusmerge.Circuit{}, s.Merge, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statussplit.Circuit{}, s.Split, test.WithCurves(ecc.BLS12_381))
	assert.SolvingSucceeded(&statusprocess.Circuit{}, s.Process, test.WithCurves(ecc.BLS12_381))
}

func TestM6RejectsInvalidStatus(t *testing.T) {
	s, err := m6case.Build("../..")
	if err != nil {
		t.Fatal(err)
	}
	assert := test.NewAssert(t)
	badRoot := *s.Transfer
	badRoot.NoteStatusRoot = 123
	assert.SolvingFailed(&statustransfer.Circuit{}, &badRoot, test.WithCurves(ecc.BLS12_381))
	badPath := *s.Process
	badPath.StatusPaths[1].Siblings[0] = 999
	assert.SolvingFailed(&statusprocess.Circuit{}, &badPath, test.WithCurves(ecc.BLS12_381))
	badTransition := *s.StatusUpdate
	badTransition.OldStatus = uint8(status.Active)
	badTransition.NewStatus = uint8(status.Revoked)
	assert.SolvingFailed(&statusupdate.Circuit{}, &badTransition, test.WithCurves(ecc.BLS12_381))
}
