package m6b1case

import (
	"bytes"
	"math/big"
	"testing"

	auditencryption "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_encryption"
	auditprocess "github.com/bighim/zkDPP/zkDPP-poc-v1/features/audit_process_3_2"
	"github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v1/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
)

func TestCircuitsAndEventBinding(t *testing.T) {
	pk, _, err := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{1}, 1024)))
	if err != nil {
		t.Fatal(err)
	}
	r := big.NewInt(7)
	makeScenario := func() *Scenario {
		s, e := Build("../..", pk, r)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := makeScenario()
	solve := func(c, a frontend.Circuit) error { return test.IsSolved(c, a, ecc.BLS12_381.ScalarField()) }
	for _, item := range []struct {
		name                   string
		definition, assignment frontend.Circuit
		count                  int
	}{
		{"core", auditencryption.New(pk, 5), s.Core, 8},
		{"process", auditprocess.New(pk), s.Process, 15},
	} {
		t.Run(item.name, func(t *testing.T) {
			if e := solve(item.definition, item.assignment); e != nil {
				t.Fatal(e)
			}
			ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, item.definition)
			if e != nil {
				t.Fatal(e)
			}
			if ccs.GetNbPublicVariables() != item.count {
				t.Fatalf("public count %d", ccs.GetNbPublicVariables())
			}
			w, e := frontend.NewWitness(item.assignment, ecc.BLS12_381.ScalarField())
			if e != nil {
				t.Fatal(e)
			}
			pub, e := w.Public()
			if e != nil {
				t.Fatal(e)
			}
			got := []fr.Element(pub.Vector().(fr.Vector))
			want := s.Inputs
			if item.name == "core" {
				want = append([]fr.Element{s.Context, s.Ciphertext.R1.X, s.Ciphertext.R1.Y}, s.Ciphertext.Data...)
			}
			if !Equal(got, want) {
				t.Fatal("public input order mismatch")
			}
		})
	}
	one := []fr.Element{zkhash.Element(9)}
	ct, e := auditcrypto.Encrypt(pk, s.Context, one, r)
	if e != nil {
		t.Fatal(e)
	}
	if e = solve(auditencryption.New(pk, 1), auditencryption.Assignment(pk, s.Context, one, r, ct)); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*auditencryption.Circuit){
		"nonce-zero":     func(c *auditencryption.Circuit) { c.Randomness = 0 },
		"nonce-q":        func(c *auditencryption.Circuit) { c.Randomness = auditcrypto.Order() },
		"nonce-too-wide": func(c *auditencryption.Circuit) { c.Randomness = new(big.Int).Lsh(big.NewInt(1), 252) },
		"point":          func(c *auditencryption.Circuit) { c.R1.X = 0; c.R1.Y = 1 },
		"plaintext":      func(c *auditencryption.Circuit) { c.Message[0] = 42 },
		"ciphertext":     func(c *auditencryption.Circuit) { c.Data[0] = 42 },
		"context":        func(c *auditencryption.Circuit) { c.Context = 42 },
	} {
		t.Run(name, func(t *testing.T) {
			a := makeScenario().Core
			mutate(a)
			if solve(auditencryption.New(pk, 5), a) == nil {
				t.Fatal("invalid relation accepted")
			}
		})
	}
	// Same wrong payload is valid generic encryption but not valid Process auditing.
	for _, name := range []string{"wrong-parent", "wrong-output-nf", "parent-order", "output-order", "wrong-context"} {
		t.Run(name, func(t *testing.T) {
			s := makeScenario()
			wrong := append([]fr.Element{}, s.Message...)
			l := s.Context
			switch name {
			case "wrong-parent":
				wrong[0] = zkhash.Element(42)
			case "wrong-output-nf":
				wrong[3] = zkhash.Element(42)
			case "parent-order":
				wrong[0], wrong[1] = wrong[1], wrong[0]
			case "output-order":
				wrong[3], wrong[4] = wrong[4], wrong[3]
			case "wrong-context":
				l = zkhash.Element(42)
			}
			ct, e := auditcrypto.Encrypt(pk, l, wrong, r)
			if e != nil {
				t.Fatal(e)
			}
			if e = solve(auditencryption.New(pk, 5), auditencryption.Assignment(pk, l, wrong, r, ct)); e != nil {
				t.Fatal("generic relation should accept its own message", e)
			}
			a := auditprocess.Assignment(pk, s.Base.Assignment, r, ct)
			if solve(auditprocess.New(pk), a) == nil {
				t.Fatal("Process accepted semantically wrong audit payload")
			}
		})
	}
	other, _, e := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{2}, 1024)))
	if e != nil {
		t.Fatal(e)
	}
	ct, e = auditcrypto.Encrypt(other, s.Context, s.Message, r)
	if e != nil {
		t.Fatal(e)
	}
	if solve(auditencryption.New(pk, 5), auditencryption.Assignment(pk, s.Context, s.Message, r, ct)) == nil {
		t.Fatal("wrong committee accepted")
	}
	for name, mutate := range map[string]func(*auditprocess.Circuit){
		"owner":  func(c *auditprocess.Circuit) { c.Process.SKOwner = 42 },
		"path":   func(c *auditprocess.Circuit) { c.Process.Paths[0].Siblings[0] = 42 },
		"policy": func(c *auditprocess.Circuit) { c.Process.PolicyRef = 42 },
		"state":  func(c *auditprocess.Circuit) { c.Process.Outputs[0].QMass = 42 },
		"role":   func(c *auditprocess.Circuit) { c.Process.Outputs[1].AssetRole = 0 },
	} {
		t.Run("base-"+name, func(t *testing.T) {
			a := makeScenario().Process
			mutate(a)
			if solve(auditprocess.New(pk), a) == nil {
				t.Fatal("M5 relation bypassed")
			}
		})
	}
}
