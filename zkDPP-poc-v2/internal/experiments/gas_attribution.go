package experiments

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/ethereum/go-ethereum/common"
	"os"
	"path/filepath"
	"strings"
)

// ReplayGasAttribution reuses exactly the three proofs leading to the official
// run-2 Transfer. It creates no proofs, keys, or historical output files.
func ReplayGasAttribution(root, out string) error {
	return ReplayGasAttributionVariant(root, out, true)
}

func ReplayGasAttributionVariant(root, out string, audited bool) error {
	exp, run := 1, 2
	if !audited {
		exp, run = 3, 1
	}
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	ctx := context.Background()
	e, err := NewEngine(root, ctx, audited)
	if err != nil {
		return err
	}
	defer e.Client.Close()
	if err = e.Initialize(0); err != nil {
		return err
	}
	var original struct {
		Rows []Measurement `json:"rows"`
	}
	if err = readJSON(filepath.Join(root, OutputDir, fmt.Sprintf("experiment-%d.json", exp)), &original); err != nil {
		return err
	}
	var txs []Transaction
	for _, suffix := range []string{"01", "02", "03"} {
		var found *Measurement
		for i := range original.Rows {
			r := &original.Rows[i]
			if r.Run == run && r.Case == fmt.Sprintf("exp%d-lifecycle-%s", exp, suffix) {
				found = r
				break
			}
		}
		if found == nil {
			return fmt.Errorf("missing original row")
		}
		var fixture struct {
			Profile string       `json:"profile"`
			Public  []fr.Element `json:"public"`
			Proof   string       `json:"proof"`
		}
		if err = readJSON(filepath.Join(root, found.ProofArtifact), &fixture); err != nil {
			return err
		}
		p, er := ProfileByName(fixture.Profile)
		if er != nil {
			return er
		}
		proof, er := hex.DecodeString(strings.TrimPrefix(fixture.Proof, "0x"))
		if er != nil {
			return er
		}
		data, er := eventData(e.ABI, &Event{Profile: p, Public: fixture.Public}, proof)
		if er != nil {
			return er
		}
		receipt, tx, er := e.send("gas-attribution-"+suffix, &e.Ledger, data)
		if er != nil {
			return er
		}
		txs = append(txs, tx)
		if suffix == "03" {
			if tx.Gas != found.Tx.Gas {
				return fmt.Errorf("replay gas %d != original %d", tx.Gas, found.Tx.Gas)
			}
			if err = writeJSON(filepath.Join(out, "replay.json"), map[string]any{"original": found, "transaction": tx, "receipt": receipt, "ledger": e.Ledger, "deployments": e.Deployments, "preparations": e.Preparations, "replayed": txs, "calldata": "0x" + hex.EncodeToString(data)}); err != nil {
				return err
			}
			var call any
			if err = e.Client.Client().CallContext(ctx, &call, "debug_traceTransaction", common.HexToHash(tx.Hash), map[string]any{"tracer": "callTracer"}); err != nil {
				return err
			}
			if err = writeJSON(filepath.Join(out, "calls.json"), call); err != nil {
				return err
			}
			// Disable child opcode logs by querying only root execution with callTracer?
			// Default struct logger is needed for exact root opcode/source attribution.
			var trace any
			if err = e.Client.Client().CallContext(ctx, &trace, "debug_traceTransaction", common.HexToHash(tx.Hash), map[string]any{"disableMemory": true, "disableStorage": true, "disableStack": true}); err != nil {
				return err
			}
			if err = writeJSON(filepath.Join(out, "opcodes.json"), trace); err != nil {
				return err
			}
		}
	}
	return nil
}
