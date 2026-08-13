package main

import (
	"context"
	"encoding/hex"
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
	GeneratedAt       string   `json:"generatedAt"`
	RPC               string   `json:"rpc"`
	ChainID           uint64   `json:"chainId"`
	BlockBefore       uint64   `json:"blockBefore"`
	BlockAfter        uint64   `json:"blockAfter"`
	PeerCount         uint64   `json:"peerCount"`
	Validators        []string `json:"validators"`
	NodesAllowlist    []string `json:"nodesAllowlist"`
	AccountsAllowlist []string `json:"accountsAllowlist"`
	BLSG1AddOutput    string   `json:"bls12G1AddOutput"`
	Passed            bool     `json:"passed"`
}

func main() {
	rpcURL := flag.String("rpc", "http://127.0.0.1:9545", "Besu JSON-RPC URL")
	out := flag.String("out", "benchmarks/besu-network-smoke.json", "output JSON")
	wait := flag.Duration("block-wait", 2500*time.Millisecond, "block progress observation window")
	flag.Parse()

	ctx := context.Background()
	client, err := rpc.DialContext(ctx, *rpcURL)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	var chainHex, blockBeforeHex, blockAfterHex, peerHex string
	var validators, nodes, accounts []string
	call := map[string]string{
		"to":   "0x000000000000000000000000000000000000000b",
		"data": "0x" + hex.EncodeToString(make([]byte, 256)),
	}
	var blsOutput string
	must(client.CallContext(ctx, &chainHex, "eth_chainId"))
	must(client.CallContext(ctx, &blockBeforeHex, "eth_blockNumber"))
	must(client.CallContext(ctx, &blsOutput, "eth_call", call, "latest"))
	time.Sleep(*wait)
	must(client.CallContext(ctx, &peerHex, "net_peerCount"))
	must(client.CallContext(ctx, &validators, "qbft_getValidatorsByBlockNumber", "latest"))
	must(client.CallContext(ctx, &nodes, "perm_getNodesAllowlist"))
	must(client.CallContext(ctx, &accounts, "perm_getAccountsAllowlist"))
	must(client.CallContext(ctx, &blockAfterHex, "eth_blockNumber"))

	value := result{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), RPC: *rpcURL,
		ChainID: quantity(chainHex), BlockBefore: quantity(blockBeforeHex), BlockAfter: quantity(blockAfterHex),
		PeerCount: quantity(peerHex), Validators: validators, NodesAllowlist: nodes, AccountsAllowlist: accounts,
		BLSG1AddOutput: blsOutput,
	}
	zeroG1 := "0x" + strings.Repeat("00", 128)
	value.Passed = value.ChainID == 31337 && value.BlockAfter > value.BlockBefore && value.PeerCount >= 4 &&
		len(value.Validators) == 4 && len(value.NodesAllowlist) == 5 && len(value.AccountsAllowlist) == 7 &&
		strings.EqualFold(value.BLSG1AddOutput, zeroG1)
	write(*out, value)
	if !value.Passed {
		panic(fmt.Errorf("Besu smoke gate failed: %+v", value))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func quantity(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		panic(err)
	}
}
