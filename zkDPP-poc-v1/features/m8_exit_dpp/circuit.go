package m8exitdpp

import (
	"math/big"

	base "github.com/bighim/zkDPP/zkDPP-poc-v1/features/private_spend"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/circuitutil"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/dpp"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/merkle"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/note"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	ed "github.com/consensys/gnark/std/algebra/native/twistededwards"
)

type Circuit struct {
	Base              base.Circuit
	DPPCommitment     frontend.Variable `gnark:",public"`
	R1                ed.Point          `gnark:",public"`
	EncryptedParentCM frontend.Variable `gnark:",public"`
	DPP               circuitutil.DPPWitness
	Randomness        frontend.Variable
	CommitteePK       auditcrypto.PublicKey `gnark:"-"`
}

const eventExit = uint64(7)

func New(pk auditcrypto.PublicKey) *Circuit { return &Circuit{CommitteePK: pk} }

func (c *Circuit) Define(api frontend.API) error {
	if err := c.Base.Define(api); err != nil {
		return err
	}
	circuitutil.AssertDPP(api, c.DPP)
	api.AssertIsEqual(c.DPP.DocumentHash, c.Base.Note.DocumentHash)
	api.AssertIsEqual(c.DPP.AssetRole, c.Base.Note.AssetRole)
	api.AssertIsEqual(c.DPP.QMass, c.Base.Note.QMass)
	api.AssertIsEqual(c.DPP.ARec, c.Base.Note.ARec)
	api.AssertIsEqual(c.DPP.E, c.Base.Note.E)
	dppCM, err := circuitutil.DPPCommitment(api, c.DPP)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.DPPCommitment, dppCM)
	parentCM, err := circuitutil.Commitment(api, c.Base.Note)
	if err != nil {
		return err
	}
	context, err := circuitutil.Hash(api, auditcrypto.AuditContextTag, eventExit, c.Base.NoteRoot, c.Base.NF, c.DPPCommitment)
	if err != nil {
		return err
	}
	return circuitutil.AssertEncrypted(api, c.CommitteePK, context, []frontend.Variable{parentCM}, c.Randomness, c.R1, []frontend.Variable{c.EncryptedParentCM})
}

func Assignment(pk auditcrypto.PublicKey, n note.Note, skOwner, root fr.Element, path merkle.Path, data dpp.PrivateData, randomness *big.Int, ciphertext auditcrypto.Ciphertext) *Circuit {
	c := New(pk)
	c.Base = *base.Assignment(n, skOwner, root, path)
	c.DPPCommitment = data.Commitment
	c.R1 = ed.Point{X: ciphertext.R1.X, Y: ciphertext.R1.Y}
	c.EncryptedParentCM = ciphertext.Data[0]
	c.DPP = circuitutil.DPPWitness{DocumentHash: data.DocumentHash, AssetRole: uint64(data.AssetRole), QMass: data.State.QMass, ARec: data.State.ARec, E: data.State.E, Opening: data.Opening}
	c.Randomness = new(big.Int).Set(randomness)
	return c
}
