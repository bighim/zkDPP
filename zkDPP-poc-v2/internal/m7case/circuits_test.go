package m7case

import (
	"bytes"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"math/big"
	"reflect"
	"testing"
)

func TestAuditCircuits(t *testing.T) {
	pk, _, e := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{3}, 1024)))
	if e != nil {
		t.Fatal(e)
	}
	groups, e := Build("../..", pk)
	if e != nil {
		t.Fatal(e)
	}
	compiled := map[audit.Kind]bool{}
	for _, g := range groups {
		for _, c := range append(append([]*Case{}, g.Events...), g.Extra...) {
			t.Run(g.Name+"/"+c.Name, func(t *testing.T) {
				if e := test.IsSolved(c.Definition, c.Assignment, ecc.BLS12_381.ScalarField()); e != nil {
					t.Fatal(e)
				}
				if !compiled[c.Kind] {
					ccs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, c.Definition)
					if e != nil {
						t.Fatal(e)
					}
					if ccs.GetNbPublicVariables() != len(c.Inputs) {
						t.Fatal("public count")
					}
					compiled[c.Kind] = true
				}
				w, e := frontend.NewWitness(c.Assignment, ecc.BLS12_381.ScalarField())
				if e != nil {
					t.Fatal(e)
				}
				pub, _ := w.Public()
				if !audit.Equal([]fr.Element(pub.Vector().(fr.Vector)), c.Inputs) {
					t.Fatal("public order")
				}
				// Change the public ciphertext, without altering private plaintext/r.
				copy := reflect.New(reflect.TypeOf(c.Assignment).Elem())
				copy.Elem().Set(reflect.ValueOf(c.Assignment).Elem())
				if c.Kind == audit.Process {
					copy.Elem().FieldByName("R1").FieldByName("X").Set(reflect.ValueOf(frontend.Variable(0)))
				} else {
					copy.Elem().FieldByName("Audit").FieldByName("R1").FieldByName("X").Set(reflect.ValueOf(frontend.Variable(0)))
				}
				if test.IsSolved(c.Definition, copy.Interface().(frontend.Circuit), ecc.BLS12_381.ScalarField()) == nil {
					t.Fatal("corrupt point accepted")
				}
			})
		}
	}
	if len(compiled) != 8 {
		t.Fatal("missing Event")
	}
}
func TestWrongSemanticPlaintext(t *testing.T) {
	pk, _, e := auditcrypto.TrustedSetup(bytes.NewReader(bytes.Repeat([]byte{4}, 1024)))
	if e != nil {
		t.Fatal(e)
	}
	groups, e := Build("../..", pk)
	if e != nil {
		t.Fatal(e)
	}
	seen := map[audit.Kind]bool{}
	for _, g := range groups {
		for _, c := range g.Events {
			if seen[c.Kind] {
				continue
			}
			seen[c.Kind] = true
			t.Run(c.Kind.String(), func(t *testing.T) {
				v := reflect.ValueOf(c.Assignment).Elem()
				field := "Base"
				if c.Kind == audit.Process {
					field = "Process"
				}
				b := reflect.New(v.FieldByName(field).Type())
				b.Elem().Set(v.FieldByName(field))
				base := b.Interface().(frontend.Circuit)
				shape, _ := audit.Shape(c.Kind)
				sk := fr.Element{}
				// Entry needs its extra owner witness; the other base Circuits carry the secret.
				if c.Kind == audit.Entry {
					sk = v.FieldByName("SKOwner").Interface().(fr.Element)
				}
				for j := range c.Message {
					wrong := append([]fr.Element{}, c.Message...)
					wrong[j].Add(&wrong[j], new(fr.Element).SetOne())
					ct, e := auditcrypto.Encrypt(pk, c.Context, wrong, c.Randomness)
					if e != nil {
						t.Fatal(e)
					}
					a, e := WithCipher("wrong", c.Kind, base, sk, c.Message[:shape.ParentCount], c.Message[shape.ParentCount:], pk, c.Randomness, ct, 0)
					if e != nil {
						t.Fatal(e)
					}
					if test.IsSolved(a.Definition, a.Assignment, ecc.BLS12_381.ScalarField()) == nil {
						t.Fatalf("wrong message position %d accepted", j)
					}
				}
				if c.Kind != audit.Process {
					for _, r := range []*big.Int{big.NewInt(0), auditcrypto.Order()} {
						copy := reflect.New(v.Type())
						copy.Elem().Set(v)
						copy.Elem().FieldByName("Audit").FieldByName("Randomness").Set(reflect.ValueOf(frontend.Variable(r)))
						if test.IsSolved(c.Definition, copy.Interface().(frontend.Circuit), ecc.BLS12_381.ScalarField()) == nil {
							t.Fatal("invalid nonce accepted")
						}
					}
				}
			})
		}
	}
}
