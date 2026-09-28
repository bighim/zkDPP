// Stream a diagnostic trace, retaining only root Ledger opcodes and their stacks.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	dir := "output/paper-supplement/transfer-gas-attribution/"
	if len(os.Args) == 2 {
		dir = os.Args[1] + "/"
	}
	b, e := os.ReadFile(dir + "replay.json")
	if e != nil {
		panic(e)
	}
	var r struct {
		Transaction struct{ Hash string } `json:"transaction"`
	}
	if e = json.Unmarshal(b, &r); e != nil {
		panic(e)
	}
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "debug_traceTransaction", "params": []any{r.Transaction.Hash, map[string]any{"disableMemory": true, "disableStorage": true, "disableStack": false}}})
	res, e := http.Post("http://127.0.0.1:18547", "application/json", bytes.NewReader(req))
	if e != nil {
		panic(e)
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	var roots []json.RawMessage
	count := 0
	// Walk JSON object keys until reaching structLogs, then decode one step at a time.
	for {
		t, e := dec.Token()
		if e != nil {
			panic(e)
		}
		if t == "structLogs" {
			break
		}
	}
	if _, e = dec.Token(); e != nil {
		panic(e)
	}
	for dec.More() {
		var step json.RawMessage
		if e = dec.Decode(&step); e != nil {
			panic(e)
		}
		var h struct{ Depth int }
		json.Unmarshal(step, &h)
		count++
		if h.Depth == 1 {
			roots = append(roots, step)
		}
	}
	out, _ := json.Marshal(map[string]any{"transaction": r.Transaction.Hash, "totalSteps": count, "rootSteps": roots})
	f, e := os.OpenFile(dir+"root-opcodes.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	f.Write(out)
	fmt.Println("steps", count, "root", len(roots))
}
