package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/artifact"
	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/m7run"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Contract struct {
	ABI      json.RawMessage
	Bytecode struct{ Object string }
}
type Row struct {
	Run, Round     int
	Arm            string
	Warmup         bool
	SolidityProofs []string
}
type NativeRun struct {
	Rows                 []Row
	CanonicalAB, PublicC []string `json:",omitempty"`
}

func load(p string, v any) error {
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func contract(out, name string) (abi.ABI, []byte, error) {
	p := filepath.Join(out, "contracts/out", name, "PlonkVerifier.sol/PlonkVerifier.json")
	if name == "B" {
		p = filepath.Join(out, "contracts/out/PlonkVerifier.sol/PlonkVerifier.json")
	}
	if name == "router" {
		p = filepath.Join(out, "contracts/out/MergeProofRouter.sol/MergeProofRouter.json")
	}
	var c Contract
	if e := load(p, &c); e != nil {
		return abi.ABI{}, nil, e
	}
	a, e := abi.JSON(bytes.NewReader(c.ABI))
	if e != nil {
		return a, nil, e
	}
	b, e := hex.DecodeString(strings.TrimPrefix(c.Bytecode.Object, "0x"))
	return a, b, e
}
func decode(s string) []byte {
	b, e := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if e != nil {
		panic(e)
	}
	return b
}
func main() {
	out := flag.String("out", ".", "output")
	rpc := flag.String("rpc", "http://127.0.0.1:18548", "local Anvil")
	flag.Parse()
	if e := run(*out, *rpc); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(out, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c, e := ethclient.DialContext(ctx, url)
	if e != nil {
		return e
	}
	defer c.Close()
	var accounts []common.Address
	if e = c.Client().CallContext(ctx, &accounts, "eth_accounts"); e != nil {
		return e
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no unlocked local accounts")
	}
	chain, e := c.ChainID(ctx)
	if e != nil {
		return e
	}
	if chain.Uint64() != 31337 {
		return fmt.Errorf("local chain ID mismatch")
	}
	from := accounts[0]
	deployments := []map[string]any{}
	addresses := map[string]common.Address{}
	var routerABI abi.ABI
	for _, name := range []string{"path", "eq", "add", "B", "C", "router"} {
		a, code, e := contract(out, name)
		if e != nil {
			return e
		}
		if name == "router" {
			arg, e := a.Constructor.Inputs.Pack(addresses["path"], addresses["eq"], addresses["add"], addresses["B"], addresses["C"])
			if e != nil {
				return e
			}
			code = append(code, arg...)
			routerABI = a
		}
		receipt, tx, e := m7run.Send(ctx, c, from, nil, code, "deploy-"+name)
		if e != nil {
			return e
		}
		address := receipt.ContractAddress
		addresses[name] = address
		runtime, e := c.CodeAt(ctx, address, nil)
		if e != nil {
			return e
		}
		deployments = append(deployments, map[string]any{"name": name, "address": address.Hex(), "tx": tx, "runtimeBytes": len(runtime), "runtimeKeccak": crypto.Keccak256Hash(runtime).Hex()})
	}
	rows := []map[string]any{}
	for rn := 1; rn <= 3; rn++ {
		var native NativeRun
		if e = load(filepath.Join(out, "raw", fmt.Sprintf("native-run-%02d.json", rn)), &native); e != nil {
			return e
		}
		var ab [15]*big.Int
		var cp [9]*big.Int
		for i, s := range native.CanonicalAB {
			ab[i], _ = new(big.Int).SetString(s, 10)
		}
		for i, s := range native.PublicC {
			cp[i], _ = new(big.Int).SetString(s, 10)
		}
		for _, r := range native.Rows {
			if r.Warmup || r.Arm == "A-parallel-2" {
				continue
			}
			arm := r.Arm
			if arm == "A-sequential" {
				arm = "A"
			}
			var data []byte
			if arm == "A" {
				var ps [6][]byte
				for i, s := range r.SolidityProofs {
					ps[i] = decode(s)
				}
				data, e = routerABI.Pack("verifyA", ps, ab)
			} else if arm == "B" {
				data, e = routerABI.Pack("verifyB", decode(r.SolidityProofs[0]), ab)
			} else {
				data, e = routerABI.Pack("verifyC", decode(r.SolidityProofs[0]), cp)
			}
			if e != nil {
				return e
			}
			dest := addresses["router"]
			ret, e := c.CallContract(ctx, ethereum.CallMsg{From: from, To: &dest, Data: data}, nil)
			if e != nil {
				return e
			}
			if len(ret) != 32 || new(big.Int).SetBytes(ret).Cmp(big.NewInt(1)) != 0 {
				return fmt.Errorf("native/EVM mismatch %s/%d/%d", arm, rn, r.Round)
			}
			receipt, tx, e := m7run.Send(ctx, c, from, &dest, data, fmt.Sprintf("%s-run%d-round%d", arm, rn, r.Round))
			if e != nil {
				return e
			}
			zeros, nonzeros := 0, 0
			for _, x := range data {
				if x == 0 {
					zeros++
				} else {
					nonzeros++
				}
			}
			intrinsic := uint64(21000 + 4*zeros + 16*nonzeros)
			floor := uint64(21000 + 10*(zeros+4*nonzeros))
			pure := receipt.GasUsed - intrinsic
			if receipt.GasUsed <= floor {
				return fmt.Errorf("EIP-7623 floor binds; subtraction cannot isolate execution")
			}
			var trace json.RawMessage
			if r.Round == 0 {
				if e = c.Client().CallContext(ctx, &trace, "debug_traceTransaction", common.HexToHash(tx.Hash), map[string]any{"tracer": "callTracer"}); e != nil {
					return e
				}
				if e = artifact.WriteJSON(filepath.Join(out, "raw", fmt.Sprintf("evm-trace-%s-run%d.json", arm, rn)), trace); e != nil {
					return e
				}
			}
			rows = append(rows, map[string]any{"run": rn, "round": r.Round, "arm": arm, "nativeToEVM": true, "returnedTrue": true, "receipt": receipt, "transaction": tx, "canonicalABIBytes": len(data), "calldataZero": zeros, "calldataNonzero": nonzeros, "intrinsicGas": intrinsic, "EIP7623Floor": floor, "floorNotBinding": true, "verificationExecutionGas": pure, "calldataHex": "0x" + hex.EncodeToString(data)})
			fmt.Printf("EVM %s run=%d round=%d pure=%d receipt=%d ABI=%d\n", arm, rn, r.Round, pure, receipt.GasUsed, len(data))
		}
	}
	return artifact.WriteJSON(filepath.Join(out, "raw/evm.json"), map[string]any{"chainID": chain.String(), "rpc": url, "hardfork": "Prague", "deployments": deployments, "rows": rows, "scope": "Stateless canonical proof wrapper verification execution; receipt gas minus 21000+calldata intrinsic. EIP-7623 floor checked nonbinding. Not full transaction/application cost."})
}
