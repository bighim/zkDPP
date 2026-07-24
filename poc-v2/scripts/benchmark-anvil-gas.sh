#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RPC_URL="http://anvil:8545"
MNEMONIC="test test test test test test test test test test test junk"

cleanup() {
  docker compose stop anvil >/dev/null 2>&1 || true
  docker compose rm -f anvil >/dev/null 2>&1 || true
}
trap cleanup EXIT

derive_key() {
  docker compose run --rm foundry cast wallet private-key "$MNEMONIC" "$1" | tr -d '\r\n'
}

cleanup
docker compose up -d --wait anvil

DEPLOYER_PK="$(derive_key 0)"
ALUMINUM_SUPPLIER_PK="$(derive_key 1)"
CATHODE_SUPPLIER_PK="$(derive_key 2)"
ANODE_SUPPLIER_PK="$(derive_key 3)"
FOIL_MANUFACTURER_PK="$(derive_key 4)"
CELL_MANUFACTURER_PK="$(derive_key 5)"

docker compose run --rm \
  -e ANVIL_DEPLOYER_PK="$DEPLOYER_PK" \
  -e ANVIL_ALUMINUM_SUPPLIER_PK="$ALUMINUM_SUPPLIER_PK" \
  -e ANVIL_CATHODE_SUPPLIER_PK="$CATHODE_SUPPLIER_PK" \
  -e ANVIL_ANODE_SUPPLIER_PK="$ANODE_SUPPLIER_PK" \
  -e ANVIL_FOIL_MANUFACTURER_PK="$FOIL_MANUFACTURER_PK" \
  -e ANVIL_CELL_MANUFACTURER_PK="$CELL_MANUFACTURER_PK" \
  foundry forge script script/CanonicalAnvilBenchmark.s.sol:CanonicalAnvilBenchmark \
  --rpc-url "$RPC_URL" \
  --broadcast \
  --slow \
  --non-interactive \
  -vv

GOCACHE="$ROOT_DIR/.cache/go-build" GOMODCACHE="$ROOT_DIR/.cache/gomod" GOTOOLCHAIN=go1.25.7 go run ./cmd/anvilreport \
  -broadcast contracts/broadcast/CanonicalAnvilBenchmark.s.sol/31337/run-latest.json \
  -out-json benchmarks/anvil-transaction-gas.json \
  -out-csv benchmarks/anvil-transaction-gas.csv
