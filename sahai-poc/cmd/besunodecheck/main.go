package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

type result struct {
	GeneratedAt string           `json:"generatedAt"`
	RPC         string           `json:"rpc"`
	PeerCount   uint64           `json:"peerCount"`
	Peers       []map[string]any `json:"adminPeers"`
	Passed      bool             `json:"passed"`
}

func main() {
	rpcURL := flag.String("rpc", "http://127.0.0.1:9546", "unauthorized Besu JSON-RPC URL")
	wait := flag.Duration("wait", 3*time.Second, "connection rejection observation window")
	out := flag.String("out", "benchmarks/besu-node-permission-smoke.json", "output JSON")
	flag.Parse()
	time.Sleep(*wait)
	ctx := context.Background()
	client, err := rpc.DialContext(ctx, *rpcURL)
	must(err)
	defer client.Close()
	var peerHex string
	var peers []map[string]any
	must(client.CallContext(ctx, &peerHex, "net_peerCount"))
	must(client.CallContext(ctx, &peers, "admin_peers"))
	value := result{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), RPC: *rpcURL, PeerCount: quantity(peerHex), Peers: peers}
	value.Passed = value.PeerCount == 0 && len(value.Peers) == 0
	write(*out, value)
	if !value.Passed {
		panic(fmt.Errorf("outside-node rejection gate failed: %+v", value))
	}
}

func quantity(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
	must(err)
	return parsed
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(raw, '\n'), 0o644))
}
