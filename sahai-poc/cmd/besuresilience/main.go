package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type result struct {
	GeneratedAt      string   `json:"generatedAt"`
	StoppedValidator string   `json:"stoppedValidator"`
	BlockBefore      uint64   `json:"blockBefore"`
	BlockAfter       uint64   `json:"blockAfter"`
	PeerCount        uint64   `json:"peerCount"`
	Validators       []string `json:"validators"`
	TransactionHash  string   `json:"transactionHash"`
	ReceiptBlock     uint64   `json:"receiptBlock"`
	FinalityMS       float64  `json:"submitToReceiptMs"`
	Passed           bool     `json:"passed"`
}

func main() {
	rpcURL := flag.String("rpc", "http://127.0.0.1:9545", "Besu JSON-RPC URL")
	privateKey := flag.String("private-key", "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", "allowlisted test key")
	timeout := flag.Duration("timeout", 30*time.Second, "transaction finality timeout")
	out := flag.String("out", "benchmarks/besu-validator-tolerance.json", "output JSON")
	flag.Parse()
	ctx := context.Background()
	client, err := rpc.DialContext(ctx, *rpcURL)
	must(err)
	defer client.Close()
	var beforeHex, afterHex, peersHex string
	var validators []string
	must(client.CallContext(ctx, &beforeHex, "eth_blockNumber"))
	key, err := crypto.HexToECDSA(strings.TrimPrefix(*privateKey, "0x"))
	must(err)
	address := crypto.PubkeyToAddress(key.PublicKey)
	eth := ethclient.NewClient(client)
	nonce, err := eth.PendingNonceAt(ctx, address)
	must(err)
	gasPrice, err := eth.SuggestGasPrice(ctx)
	must(err)
	if gasPrice.Sign() == 0 {
		gasPrice = big.NewInt(1)
	}
	tx := types.NewTx(&types.LegacyTx{Nonce: nonce, To: &address, Gas: 21_000, GasPrice: gasPrice})
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(31337)), key)
	must(err)
	started := time.Now()
	must(eth.SendTransaction(ctx, signed))
	receiptCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	var receipt *types.Receipt
	for receipt == nil {
		receipt, err = eth.TransactionReceipt(receiptCtx, signed.Hash())
		if err == nil {
			break
		}
		select {
		case <-receiptCtx.Done():
			must(receiptCtx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	must(client.CallContext(ctx, &afterHex, "eth_blockNumber"))
	must(client.CallContext(ctx, &peersHex, "net_peerCount"))
	must(client.CallContext(ctx, &validators, "qbft_getValidatorsByBlockNumber", "latest"))
	value := result{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), StoppedValidator: "validator4",
		BlockBefore: quantity(beforeHex), BlockAfter: quantity(afterHex), PeerCount: quantity(peersHex), Validators: validators,
		TransactionHash: signed.Hash().Hex(), ReceiptBlock: receipt.BlockNumber.Uint64(), FinalityMS: float64(time.Since(started).Microseconds()) / 1000,
	}
	value.Passed = receipt.Status == types.ReceiptStatusSuccessful && value.BlockAfter > value.BlockBefore && value.ReceiptBlock > value.BlockBefore && value.PeerCount >= 3 && len(value.Validators) == 4
	write(*out, value)
	if !value.Passed {
		panic(fmt.Errorf("validator tolerance gate failed: %+v", value))
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
