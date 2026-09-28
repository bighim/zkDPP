// Attribute observed Ledger storage slots; all writes must match the declared layout.
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

func read(n string, v any) {
	b, e := os.ReadFile(dir + n)
	if e != nil {
		panic(e)
	}
	if e = json.Unmarshal(b, v); e != nil {
		panic(e)
	}
}
func num(s string) *big.Int {
	x, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		panic(s)
	}
	return x
}
func n(x int64) *big.Int               { return big.NewInt(x) }
func add(x *big.Int, y int64) *big.Int { return new(big.Int).Add(x, n(y)) }
func hash(xs ...*big.Int) *big.Int {
	b := []byte{}
	for _, x := range xs {
		b = append(b, x.FillBytes(make([]byte, 32))...)
	}
	return new(big.Int).SetBytes(crypto.Keccak256(b))
}
func main() {
	var t struct {
		RootSteps []struct {
			PC                   int
			Gas, GasCost, Refund uint64
			Op                   string
			Stack                []string
		}
	}
	read("root-opcodes.json", &t)
	var r struct {
		Calldata    string
		Transaction struct{ Gas uint64 }
		Deployments []struct{ Name, Address string }
		Original    struct{ ProofArtifact string }
	}
	read("replay.json", &r)
	b, e := os.ReadFile(r.Original.ProofArtifact)
	if e != nil {
		panic(e)
	}
	var fixture struct{ Public []json.RawMessage }
	json.Unmarshal(b, &fixture)
	// Public field values use gnark's JSON field encoding. Decode as decimal strings/numbers.
	pub := func(i int) *big.Int {
		var s string
		if json.Unmarshal(fixture.Public[i], &s) != nil {
			s = string(fixture.Public[i])
		}
		x, ok := new(big.Int).SetString(s, 10)
		if !ok {
			panic("public encoding")
		}
		return x
	}
	slots := map[string]string{}
	put := func(x *big.Int, g string) { slots[x.Text(16)] = g }
	// Seven fixed verifier slots, followed by two Tree structs (37 slots each).
	// Tree: acceptedRoots, nodes, zeroes[33], leafCount, currentRoot.
	for _, tr := range []struct{ base, index int64 }{{7, 2}, {44, 0}} {
		put(n(tr.base+35), "tree_storage")
		put(n(tr.base+36), "tree_storage")
		pos := tr.index
		for level := 0; level <= 32; level++ {
			idx := int64((uint64(1)<<uint(32-level))-1) + pos
			put(hash(n(idx), n(tr.base+1)), "tree_storage")
			pos >>= 1
		}
	}
	// Recover the two new roots from the actual SSTORE values, then validate root-history slots.
	for _, s := range t.RootSteps {
		if s.Op != "SSTORE" {
			continue
		}
		key := num(s.Stack[len(s.Stack)-1])
		v := num(s.Stack[len(s.Stack)-2])
		if key.Cmp(n(43)) == 0 {
			put(hash(v, n(7)), "tree_storage")
		}
		if key.Cmp(n(80)) == 0 {
			put(hash(v, n(44)), "tree_storage")
		}
	}
	// Record ID=3 after two Entries. AuditRecord occupies seven head slots.
	put(n(90), "audit_storage")
	base := hash(n(3), n(91))
	for i := int64(0); i < 7; i++ {
		put(add(base, i), "audit_storage")
	}
	for _, a := range []struct{ offset, len int64 }{{2, 4}, {5, 1}, {6, 2}} {
		h := hash(add(base, a.offset))
		for j := int64(0); j < a.len; j++ {
			put(add(h, j), "audit_storage")
		}
	}
	// Public nf, rvNew and cmChange.
	put(hash(pub(1), n(85)), "object_storage")
	put(hash(pub(2), n(83)), "object_storage")
	put(hash(pub(3), n(82)), "object_storage")
	put(hash(pub(2), hash(n(2), n(84))), "object_storage")
	put(hash(pub(3), hash(n(1), n(84))), "object_storage")
	addresses := map[string]string{}
	for _, d := range r.Deployments {
		if d.Name == "Poseidon2BLS12381" {
			addresses[strings.ToLower(d.Address)] = "tree_hash_calls"
		}
		if d.Name == "V14" {
			addresses[strings.ToLower(d.Address)] = "proof_call"
		}
	}
	groups := map[string]uint64{}
	counts := map[string]int{}
	writes := []any{}
	for i, s := range t.RootSteps {
		cost := s.GasCost
		if i+1 < len(t.RootSteps) {
			if s.Gas < t.RootSteps[i+1].Gas {
				panic("gas increased")
			}
			cost = s.Gas - t.RootSteps[i+1].Gas
		}
		group := "other_execution"
		if s.Op == "STATICCALL" {
			to := "0x" + fmt.Sprintf("%040x", num(s.Stack[len(s.Stack)-2]))
			g, ok := addresses[to]
			if !ok {
				panic("unexpected call " + to)
			}
			group = g
			counts[group]++
		}
		if s.Op == "SSTORE" {
			key := num(s.Stack[len(s.Stack)-1])
			g, ok := slots[key.Text(16)]
			if !ok {
				panic("unclassified SSTORE " + key.Text(16))
			}
			group = g
			counts[group]++
			writes = append(writes, map[string]any{"pc": s.PC, "slot": "0x" + key.Text(16), "gas": cost, "group": group})
		}
		groups[group] += cost
	}
	data := strings.TrimPrefix(r.Calldata, "0x")
	intrinsic := uint64(21000)
	for i := 0; i < len(data); i += 2 {
		if data[i:i+2] == "00" {
			intrinsic += 4
		} else {
			intrinsic += 16
		}
	}
	groups["transaction_intrinsic"] = intrinsic
	sum := uint64(0)
	for _, v := range groups {
		sum += v
	}
	if sum != r.Transaction.Gas {
		panic(fmt.Sprintf("sum %d != receipt %d", sum, r.Transaction.Gas))
	}
	if counts["proof_call"] != 1 || counts["tree_hash_calls"] != 64 {
		panic("call counts")
	}
	if t.RootSteps[len(t.RootSteps)-1].Refund != 0 {
		panic("refund requires separate accounting")
	}
	out := map[string]any{"groups": groups, "counts": counts, "sum": sum, "receiptGas": r.Transaction.Gas, "writes": writes, "notes": "Call gas includes caller CALL opcode overhead and all nested execution. Tree storage includes nodes, root history, root and leafCount. Audit storage includes record counter. Other execution includes ABI handling, checks, loops, reads and logs. Storage layout derived from exact Solidity declarations and checked against every observed SSTORE; no unmatched writes."}
	raw, _ := json.MarshalIndent(out, "", "  ")
	f, e := os.OpenFile(dir+"breakdown.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	f.Write(raw)
	f.Close()
	v, _ := json.MarshalIndent(groups, "", "  ")
	fmt.Println(string(v))
	fmt.Println(counts, "total", sum)
}
