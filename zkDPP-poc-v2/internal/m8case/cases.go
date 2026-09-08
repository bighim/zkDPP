package m8case

import (
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	issuecircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/issue_claim"
	exitcircuit "github.com/bighim/zkDPP/zkDPP-poc-v2/features/m8_exit_dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/dpp"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/issuepolicy"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/note"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m5case"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/testkit"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
)

const EventExit = uint64(7)

type ExitCase struct {
	Name                   string
	Definition, Assignment frontend.Circuit
	Data                   dpp.PrivateData
	Context                fr.Element
	Ciphertext             auditcrypto.Ciphertext
	Randomness             *big.Int
	PublicInputs           []fr.Element
	ParentCM               fr.Element
	EncryptMillis          float64
}

type IssueCase struct {
	Name                   string
	Config                 issuepolicy.Config
	Definition, Assignment frontend.Circuit
	Data                   dpp.PrivateData
	PublicInputs           []fr.Element
}

type Scenario struct {
	Process      *m5case.Scenario
	EligibleExit ExitCase
	WasteExit    ExitCase
	Standard     IssueCase
	Strict       IssueCase
	CommitteePK  auditcrypto.PublicKey
	OwnerSK      fr.Element
}

func Build(root string, pk auditcrypto.PublicKey) (*Scenario, error) {
	process, err := m5case.Build(root)
	if err != nil {
		return nil, err
	}
	actors, err := testkit.LoadActors(filepath.Join(root, "testdata/common/actors-v1.json"))
	if err != nil {
		return nil, err
	}
	owner, err := testkit.ByID(actors, "actor-1")
	if err != nil {
		return nil, err
	}
	makeExit := func(name string, index int, opening, random int64) (ExitCase, error) {
		n := process.Outputs[index]
		data, err := dpp.New(n.DocumentHash, n.AssetRole, n.State, zkhash.Element(uint64(opening)))
		if err != nil {
			return ExitCase{}, err
		}
		nf := note.Nullifier(owner.SKOwner, n.Commitment)
		base := []fr.Element{process.FinalRoot, nf, data.Commitment}
		context := zkhash.Hash(append([]fr.Element{auditcrypto.AuditContextTag, zkhash.Element(EventExit)}, base...)...)
		r := big.NewInt(random)
		start := time.Now()
		ct, err := auditcrypto.Encrypt(pk, context, []fr.Element{n.Commitment}, r)
		elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
		if err != nil {
			return ExitCase{}, err
		}
		assignment := exitcircuit.Assignment(pk, n, owner.SKOwner, process.FinalRoot, process.FinalPaths[3+index], data, r, ct)
		inputs := append(append([]fr.Element{}, base...), ct.R1.X, ct.R1.Y, ct.Data[0])
		return ExitCase{name, exitcircuit.New(pk), assignment, data, context, ct, r, inputs, n.Commitment, elapsed}, nil
	}
	eligible, err := makeExit("eligible-exit", 0, 9201, 8201)
	if err != nil {
		return nil, err
	}
	waste, err := makeExit("waste-exit", 1, 9202, 8202)
	if err != nil {
		return nil, err
	}
	makeIssue := func(config issuepolicy.Config) IssueCase {
		assignment := issuecircuit.Assignment(config, eligible.Data)
		p := issuecircuit.Public(config, eligible.Data)
		return IssueCase{config.Name, config, issuecircuit.New(config), assignment, eligible.Data, p[:]}
	}
	return &Scenario{process, eligible, waste, makeIssue(issuepolicy.Standard()), makeIssue(issuepolicy.Strict()), pk, owner.SKOwner}, nil
}

func Boundary(config issuepolicy.Config, q, a, e, opening uint64) (IssueCase, error) {
	data, err := dpp.New(zkhash.Element(777), note.AssetRoleEligible, note.State{QMass: q, ARec: a, E: e}, zkhash.Element(opening))
	if err != nil {
		return IssueCase{}, err
	}
	assignment := issuecircuit.Assignment(config, data)
	p := issuecircuit.Public(config, data)
	return IssueCase{fmt.Sprintf("%s-boundary", config.Name), config, issuecircuit.New(config), assignment, data, p[:]}, nil
}
