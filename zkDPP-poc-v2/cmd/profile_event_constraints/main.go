// Compile-only attribution of existing Event circuits; never generates proofs or keys.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/experiments"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/v2case"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	gp "github.com/consensys/gnark/profile"
	pp "github.com/google/pprof/profile"
)

type Result struct {
	Name   string
	Total  int
	Groups map[string]int64
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: profile_event_constraints NEW_OUTPUT_DIRECTORY")
	}
	dir := os.Args[1]
	if err := os.Mkdir(dir, 0755); err != nil {
		panic(err)
	}
	pkg, _, err := v2case.KeyFixture()
	if err != nil {
		panic(err)
	}
	results := []Result{}
	for _, p := range experiments.Profiles() {
		if p.Kind != experiments.Transfer && p.Kind != experiments.Proceed && p.Kind != experiments.Recall {
			continue
		}
		path := filepath.Join(dir, p.Name+".pprof")
		session := gp.Start(gp.WithPath(path), gp.WithoutOperations())
		cs, e := frontend.Compile(ecc.BLS12_381.ScalarField(), scs.NewBuilder, experiments.NewCircuit(p, pkg.PublicKey))
		session.Stop()
		if e != nil {
			panic(e)
		}
		f, e := os.Open(path)
		if e != nil {
			panic(e)
		}
		prof, e := pp.Parse(f)
		f.Close()
		if e != nil {
			panic(e)
		}
		r := Result{Name: p.Name, Total: cs.GetNbConstraints(), Groups: map[string]int64{}}
		var sum int64
		for _, s := range prof.Sample {
			stack := ""
			for _, loc := range s.Location {
				for _, l := range loc.Line {
					if l.Function != nil {
						stack += l.Function.Name + "\n"
					}
				}
			}
			group := "other relations / compiler"
			switch {
			case strings.Contains(stack, "AssertMasterEncrypted"):
				group = "audit: range and ciphertext checks"
				if strings.Contains(stack, "ScalarMul") {
					group = "audit: scalar multiplications"
				} else if strings.Contains(stack, "circuitutil.Hash") {
					group = "audit: key and masks"
				}
			case strings.Contains(stack, "experiments.ratio"):
				group = "state allocation: ratio"
			case strings.Contains(stack, "AssertMembership"):
				group = "input membership"
			case strings.Contains(stack, "AssertNote"):
				group = "Note validity / ranges"
			case strings.Contains(stack, "AssertVoucher"):
				group = "Voucher validity / ranges"
			case strings.Contains(stack, "VoucherCommitment"):
				group = "Voucher commitment"
			case strings.Contains(stack, "circuitutil.Commitment"):
				group = "Note commitment"
			case strings.Contains(stack, "VoucherNullifier"):
				group = "Voucher nullifier"
			case strings.Contains(stack, "circuitutil.Nullifier"):
				group = "Note nullifier"
			case strings.Contains(stack, "circuitutil.Address"):
				group = "owner address"
			}
			r.Groups[group] += s.Value[0]
			sum += s.Value[0]
		}
		if sum != int64(r.Total) {
			panic(fmt.Sprintf("profile count %d != CCS %d", sum, r.Total))
		}
		results = append(results, r)
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
	}
	b, e := json.MarshalIndent(results, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "constraints.json"), append(b, '\n'), 0644); e != nil {
		panic(e)
	}
}
