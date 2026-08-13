#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUNS="${RUNS:-1}"
ANVIL_PORT="${ANVIL_PORT:-8545}"
RPC_INTERNAL="http://anvil:8545"
RPC_HOST="http://127.0.0.1:${ANVIL_PORT}"
MNEMONIC="test test test test test test test test test test test junk"
RUN_DIR="$ROOT_DIR/benchmarks/e2e-runs"

cleanup() {
  docker compose stop anvil >/dev/null 2>&1 || true
  docker compose rm -f anvil >/dev/null 2>&1 || true
}
trap cleanup EXIT

derive_key() {
  docker compose run --rm foundry cast wallet private-key "$MNEMONIC" "$1" | tr -d '\r\n'
}

mkdir -p "$RUN_DIR"
find "$RUN_DIR" -maxdepth 1 -type f -name 'run-*.json' -delete

DEPLOYER_PK="$(derive_key 0)"
ALUMINUM_SUPPLIER_PK="$(derive_key 1)"
CATHODE_SUPPLIER_PK="$(derive_key 2)"
ANODE_SUPPLIER_PK="$(derive_key 3)"
FOIL_MANUFACTURER_PK="$(derive_key 4)"
CELL_MANUFACTURER_PK="$(derive_key 5)"

for run in $(seq 1 "$RUNS"); do
  cleanup
  ANVIL_PORT="$ANVIL_PORT" docker compose up -d --wait anvil

  ANVIL_PORT="$ANVIL_PORT" docker compose run --rm \
    -e ANVIL_DEPLOYER_PK="$DEPLOYER_PK" \
    foundry forge script script/DeployCanonical.s.sol:DeployCanonical \
    --rpc-url "$RPC_INTERNAL" \
    --broadcast \
    --slow \
    --non-interactive \
    -vv

  ANVIL_ALUMINUM_SUPPLIER_PK="$ALUMINUM_SUPPLIER_PK" \
  ANVIL_CATHODE_SUPPLIER_PK="$CATHODE_SUPPLIER_PK" \
  ANVIL_ANODE_SUPPLIER_PK="$ANODE_SUPPLIER_PK" \
  ANVIL_FOIL_MANUFACTURER_PK="$FOIL_MANUFACTURER_PK" \
  ANVIL_CELL_MANUFACTURER_PK="$CELL_MANUFACTURER_PK" \
  GOCACHE="$ROOT_DIR/.cache/go-build" \
  GOMODCACHE="$ROOT_DIR/.cache/gomod" \
  GOTOOLCHAIN=go1.25.7 \
  go run ./cmd/e2erun \
    -root "$ROOT_DIR" \
    -backend anvil \
    -rpc "$RPC_HOST" \
    -chain-id 31337 \
    -receipt-timeout 30s \
    -poll-interval 10ms \
    -broadcast contracts/broadcast/DeployCanonical.s.sol/31337/run-latest.json \
    -run "$run" \
    -out "benchmarks/e2e-runs/run-$(printf '%02d' "$run").json"
done

GOCACHE="$ROOT_DIR/.cache/go-build" GOMODCACHE="$ROOT_DIR/.cache/gomod" GOTOOLCHAIN=go1.25.7 \
go run ./cmd/e2ereport \
  -input 'benchmarks/e2e-runs/run-*.json' \
  -runs "$RUNS" \
  -out-json benchmarks/anvil-e2e-time.json \
  -out-csv benchmarks/anvil-e2e-time.csv
