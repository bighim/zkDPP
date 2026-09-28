// Read-only postprocessing of the previously measured Transfer trace.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"os"
	"strings"
)

const dir = "output/paper-supplement/transfer-gas-attribution/"

func num(s string) *big.Int {
	x, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		panic(s)
	}
	return x
}
func hash(xs ...*big.Int) *big.Int {
	var b []byte
	for _, x := range xs {
		b = append(b, x.FillBytes(make([]byte, 32))...)
	}
	return new(big.Int).SetBytes(crypto.Keccak256(b))
}
func read(p string, v any) {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	if e = json.Unmarshal(b, v); e != nil {
		panic(e)
	}
}

type cost struct {
	Gas   uint64
	Count int
}

func main() {
	var t struct {
		RootSteps []struct {
			Gas, GasCost uint64
			Op           string
			Stack        []string
		}
	}
	read(dir+"root-opcodes.json", &t)
	var r struct {
		Original struct{ ProofArtifact string }
	}
	read(dir+"replay.json", &r)
	var f struct{ Public []json.RawMessage }
	read(r.Original.ProofArtifact, &f)
	pub := func(i int) *big.Int {
		var s string
		if json.Unmarshal(f.Public[i], &s) != nil {
			s = string(f.Public[i])
		}
		x, ok := new(big.Int).SetString(s, 10)
		if !ok {
			panic("field")
		}
		return x
	}
	slots := map[string]string{}
	put := func(x *big.Int, s string) { slots[x.Text(16)] = s }
	n := big.NewInt
	for _, tr := range []struct{ base, index int64 }{{7, 2}, {44, 0}} {
		put(n(tr.base+35), "tree_metadata")
		put(n(tr.base+36), "tree_metadata")
		for l := int64(0); l < 33; l++ {
			put(n(tr.base+2+l), "tree_default_nodes")
		}
		pos := tr.index
		for l := 0; l < 32; l++ {
			idx := int64((uint64(1)<<uint(32-l))-1) + (pos ^ 1)
			put(hash(n(idx), n(tr.base+1)), "tree_sibling_nodes")
			pos >>= 1
		}
	}
	put(hash(pub(0), n(7)), "accepted_input_root")
	for _, s := range t.RootSteps {
		if s.Op != "SSTORE" {
			continue
		}
		key := num(s.Stack[len(s.Stack)-1])
		value := num(s.Stack[len(s.Stack)-2])
		if key.Cmp(n(43)) == 0 {
			put(hash(value, n(7)), "accepted_output_roots")
		}
		if key.Cmp(n(80)) == 0 {
			put(hash(value, n(44)), "accepted_output_roots")
		}
	}
	put(n(1), "verifier_address")
	put(hash(pub(1), n(85)), "input_spent_check")
	put(hash(pub(1), n(87)), "input_status_check")
	put(hash(pub(2), n(83)), "output_duplicate_check")
	put(hash(pub(3), n(82)), "output_duplicate_check")
	put(n(90), "audit_counter")
	base := hash(n(3), n(91))
	refs := hash(new(big.Int).Add(base, n(2)))
	for j := int64(0); j < 4; j++ {
		put(new(big.Int).Add(refs, n(j)), "audit_output_refs")
	}
	for j := int64(0); j < 7; j++ {
		put(new(big.Int).Add(base, n(j)), "audit_record_heads")
	}
	loads := map[string]cost{}
	ops := map[string]cost{}
	groups := map[string]cost{}
	observed := []any{}
	inc := func(m map[string]cost, k string, g uint64) { x := m[k]; x.Gas += g; x.Count++; m[k] = x }
	for i, s := range t.RootSteps {
		if s.Op == "SSTORE" || s.Op == "STATICCALL" {
			continue
		}
		g := s.GasCost
		if i+1 < len(t.RootSteps) {
			g = s.Gas - t.RootSteps[i+1].Gas
		}
		inc(ops, s.Op, g)
		group := "context"
		switch {
		case s.Op == "SLOAD":
			group = "storage_reads"
			key := num(s.Stack[len(s.Stack)-1])
			name, ok := slots[key.Text(16)]
			if !ok {
				panic("unclassified SLOAD " + key.Text(16))
			}
			inc(loads, name, g)
			observed = append(observed, map[string]any{"slot": "0x" + key.Text(16), "group": name, "gas": g})
		case s.Op == "JUMP" || s.Op == "JUMPI" || s.Op == "JUMPDEST":
			group = "control_flow"
		case strings.HasPrefix(s.Op, "PUSH") || strings.HasPrefix(s.Op, "DUP") || strings.HasPrefix(s.Op, "SWAP") || s.Op == "POP":
			group = "stack_operations"
		case s.Op == "MLOAD" || s.Op == "MSTORE" || s.Op == "MSTORE8" || s.Op == "MCOPY" || strings.HasSuffix(s.Op, "COPY"):
			group = "memory_and_copy"
		case s.Op == "KECCAK256":
			group = "keccak256"
		case strings.HasPrefix(s.Op, "LOG"):
			group = "event_logs"
		case strings.Contains("|ADD|SUB|MUL|DIV|MOD|LT|GT|SLT|SGT|EQ|ISZERO|AND|OR|XOR|NOT|SHL|SHR|SAR|", "|"+s.Op+"|"):
			group = "arithmetic_comparison_bitwise"
		}
		inc(groups, group, g)
	}
	sum := uint64(0)
	for _, v := range groups {
		sum += v.Gas
	}
	if sum != 386206 {
		panic(sum)
	}
	out := map[string]any{"groups": groups, "opcodes": ops, "storageReads": loads, "observedReads": observed, "sum": sum, "method": "Existing root trace only; excluded SSTORE and STATICCALL already attributed. Every SLOAD key classified; group totals reconcile to original 386206 Gas. These are opcode categories, not whole Solidity function costs."}
	b, _ := json.MarshalIndent(out, "", "  ")
	fp, e := os.OpenFile(dir+"other-execution-breakdown.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	fp.Write(b)
	fp.Close()
	for _, m := range []map[string]cost{groups, loads} {
		b, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(b))
	}
}
