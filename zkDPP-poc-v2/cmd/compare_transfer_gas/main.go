// Attribute the no-audit trace with the same boundaries as the audited trace.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"os"
	"strings"
)

const dir = "output/paper-supplement/transfer-gas-attribution-plain/"

func read(p string, v any) {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	if e = json.Unmarshal(b, v); e != nil {
		panic(e)
	}
}
func n(x int64) *big.Int { return big.NewInt(x) }
func hexnum(s string) *big.Int {
	x, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		panic(s)
	}
	return x
}
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
	read(dir+"root-opcodes.json", &t)
	var r struct {
		Calldata    string
		Transaction struct{ Gas uint64 }
		Deployments []struct{ Name, Address, CodeHash string }
		Original    struct{ ProofArtifact string }
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
			panic(s)
		}
		return x
	}
	slots := map[string]string{}
	put := func(x *big.Int, g string) { slots[x.Text(16)] = g }
	for _, tr := range []struct{ base, index int64 }{{7, 2}, {44, 0}} {
		put(n(tr.base+35), "tree_storage")
		put(n(tr.base+36), "tree_storage")
		for l := int64(0); l < 33; l++ {
			put(n(tr.base+2+l), "tree_storage")
		}
		pos := tr.index
		for level := 0; level <= 32; level++ {
			prefix := int64((uint64(1) << uint(32-level)) - 1)
			put(hash(n(prefix+pos), n(tr.base+1)), "tree_storage")
			if level < 32 {
				put(hash(n(prefix+(pos^1)), n(tr.base+1)), "tree_storage")
			}
			pos >>= 1
		}
	}
	for _, s := range t.RootSteps {
		if s.Op != "SSTORE" {
			continue
		}
		key := hexnum(s.Stack[len(s.Stack)-1])
		v := hexnum(s.Stack[len(s.Stack)-2])
		if key.Cmp(n(43)) == 0 {
			put(hash(v, n(7)), "tree_storage")
		}
		if key.Cmp(n(80)) == 0 {
			put(hash(v, n(44)), "tree_storage")
		}
	}
	// No producerOf: noteSpent is slot 84 and noteStatusByNf is slot 86.
	put(hash(pub(1), n(84)), "object_storage")
	put(hash(pub(1), n(86)), "object_storage")
	put(hash(pub(2), n(83)), "object_storage")
	put(hash(pub(3), n(82)), "object_storage")
	put(hash(pub(0), n(7)), "other")
	put(n(1), "other")
	addresses := map[string]string{strings.ToLower(r.Deployments[1].Address): "proof"}
	for _, d := range r.Deployments {
		if d.Name == "Poseidon2BLS12381" {
			addresses[strings.ToLower(d.Address)] = "tree_hash"
		}
	}
	groups := map[string]uint64{"audit": 0}
	counts := map[string]int{}
	ops := map[string]uint64{}
	for i, s := range t.RootSteps {
		cost := s.GasCost
		if i+1 < len(t.RootSteps) {
			if s.Gas < t.RootSteps[i+1].Gas {
				panic("gas")
			}
			cost = s.Gas - t.RootSteps[i+1].Gas
		}
		g := "other"
		if s.Op == "STATICCALL" {
			to := "0x" + fmt.Sprintf("%040x", hexnum(s.Stack[len(s.Stack)-2]))
			var ok bool
			g, ok = addresses[to]
			if !ok {
				panic(to)
			}
		}
		if s.Op == "SLOAD" || s.Op == "SSTORE" {
			var ok bool
			g, ok = slots[hexnum(s.Stack[len(s.Stack)-1]).Text(16)]
			if !ok {
				panic("unclassified " + s.Op + " " + s.Stack[len(s.Stack)-1])
			}
		}
		groups[g] += cost
		counts[g+"/"+s.Op]++
		ops[s.Op] += cost
	}
	intrinsic := uint64(21000)
	d := strings.TrimPrefix(r.Calldata, "0x")
	for i := 0; i < len(d); i += 2 {
		if d[i:i+2] == "00" {
			intrinsic += 4
		} else {
			intrinsic += 16
		}
	}
	groups["other"] += intrinsic
	sum := uint64(0)
	for _, v := range groups {
		sum += v
	}
	if sum != r.Transaction.Gas || sum != 3221268 {
		panic(fmt.Sprintf("sum %d receipt %d", sum, r.Transaction.Gas))
	}
	if t.RootSteps[len(t.RootSteps)-1].Refund != 0 {
		panic("refund")
	}
	if counts["proof/STATICCALL"] != 1 || counts["tree_hash/STATICCALL"] != 64 {
		panic("calls")
	}
	var old struct{ Groups map[string]uint64 }
	read("output/paper-supplement/transfer-gas-attribution/breakdown.json", &old)
	var reads struct {
		StorageReads map[string]struct{ Gas uint64 }
	}
	read("output/paper-supplement/transfer-gas-attribution/other-execution-breakdown.json", &reads)
	treeReads := uint64(0)
	for _, k := range []string{"tree_sibling_nodes", "tree_default_nodes", "tree_metadata", "accepted_output_roots"} {
		treeReads += reads.StorageReads[k].Gas
	}
	auditReads := uint64(0)
	for _, k := range []string{"audit_counter", "audit_record_heads", "audit_output_refs"} {
		auditReads += reads.StorageReads[k].Gas
	}
	objectReads := uint64(0)
	for _, k := range []string{"input_spent_check", "input_status_check", "output_duplicate_check"} {
		objectReads += reads.StorageReads[k].Gas
	}
	on := map[string]uint64{"tree_hash": old.Groups["tree_hash_calls"], "tree_storage": old.Groups["tree_storage"] + treeReads, "proof": old.Groups["proof_call"], "audit": old.Groups["audit_storage"] + auditReads, "object_storage": old.Groups["object_storage"] + objectReads, "other": old.Groups["other_execution"] - treeReads - auditReads - objectReads + old.Groups["transaction_intrinsic"]}
	delta := map[string]int64{}
	totalDelta := int64(0)
	for k, v := range on {
		delta[k] = int64(v) - int64(groups[k])
		totalDelta += delta[k]
	}
	if totalDelta != 356146 {
		panic(totalDelta)
	}
	result := map[string]any{"auditOff": groups, "auditOn": on, "delta": delta, "auditOverhead": totalDelta, "offReceiptGas": sum, "onReceiptGas": sum + uint64(totalDelta), "offOpcodeGas": ops, "offCounts": counts, "offIntrinsicGas": intrinsic, "note": "Original representative run 1 (off) vs run 2 (on). Same State and Tree shape; distinct commitments and proofs. Differences include their calldata bytes. Existing medians are each represented by an actual transaction. No claim of pure cryptographic overhead or whole-function attribution."}
	b, _ := json.MarshalIndent(result, "", "  ")
	fout, e := os.OpenFile(dir+"comparison.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	fout.Write(b)
	fout.Close()
	fmt.Println(string(b))
}
