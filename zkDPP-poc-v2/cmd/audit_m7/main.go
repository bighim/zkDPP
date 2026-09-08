package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/audit"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"os"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(8)
	root := flag.String("root", ".", "project root")
	rpc := flag.String("rpc", "http://127.0.0.1:18545", "RPC")
	manifest := flag.String("manifest", "", "deployment manifest")
	block := flag.Uint64("block", 0, "snapshot block")
	direction := flag.String("direction", "forward", "forward or backward")
	typ := flag.Uint("type", 1, "1 Note, 2 Voucher")
	raw := flag.String("id", "", "canonical 32-byte hex cm/rv")
	flag.Parse()
	if e := run(*root, *rpc, *manifest, *block, *direction, *typ, *raw); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(root, rpc, path string, block uint64, direction string, typ uint, raw string) error {
	if path == "" || block == 0 || typ < 1 || typ > 2 {
		return fmt.Errorf("manifest, snapshot block and valid object type required")
	}
	var d audit.Deployment
	if e := m7run.Read(path, &d); e != nil {
		return e
	}
	committee, e := m7run.Committee(root)
	if e != nil {
		return e
	}
	if d.PublicKeyChecksum != committee.Public.Checksum {
		return fmt.Errorf("committee mismatch")
	}
	id, e := auditcrypto.DecodeField(raw)
	if e != nil {
		return e
	}
	c, e := ethclient.Dial(rpc)
	if e != nil {
		return e
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	a, _, _, e := m7run.Contract(root, "ZkDPPAuditLedger.sol", "ZkDPPAuditLedger")
	if e != nil {
		return e
	}
	src := &audit.RPCSource{Client: c, ABI: a, Deployment: d}
	snap := audit.Snapshot{Number: block}
	// Read and then pin an explicit block hash, never silently fall back to latest.
	h, e := c.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if e != nil {
		return e
	}
	snap.Hash = h.Hash().Hex()
	result, e := audit.Trace(ctx, src, committee, direction, audit.Ref{ObjectType: uint8(typ), RawID: id}, snap)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Result      audit.TraceResult
		RPCRequests int
		Warning     string
	}{result, src.Requests, "Authorized audit output contains decrypted spend values. This command does not freeze objects or persist a cache."})
}
