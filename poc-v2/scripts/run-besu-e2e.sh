#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
RUN="${RUN:-1}"
BESU_PORT="${BESU_PORT:-9545}"
RPC_CONTAINER="http://host.docker.internal:${BESU_PORT}"
RPC_HOST="http://127.0.0.1:${BESU_PORT}"
MNEMONIC="test test test test test test test test test test test junk"
OUT="${OUT:-benchmarks/besu-e2e-runs/run-$(printf '%02d' "$RUN").json}"

derive_key() {
  docker compose run --rm foundry cast wallet private-key "$MNEMONIC" "$1" | tr -d '\r\n'
}

DEPLOYER_PK="$(derive_key 0)"
ALUMINUM_SUPPLIER_PK="$(derive_key 1)"
CATHODE_SUPPLIER_PK="$(derive_key 2)"
ANODE_SUPPLIER_PK="$(derive_key 3)"
FOIL_MANUFACTURER_PK="$(derive_key 4)"
CELL_MANUFACTURER_PK="$(derive_key 5)"

docker compose run --rm \
  -e ANVIL_DEPLOYER_PK="$DEPLOYER_PK" \
  foundry forge script script/DeployCanonical.s.sol:DeployCanonical \
  --rpc-url "$RPC_CONTAINER" \
  --broadcast \
  --slow \
  --non-interactive \
  -vv

mkdir -p "$(dirname "$OUT")"
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
  -backend besu \
  -rpc "$RPC_HOST" \
  -chain-id 31337 \
  -receipt-timeout 120s \
  -poll-interval 50ms \
  -broadcast contracts/broadcast/DeployCanonical.s.sol/31337/run-latest.json \
  -run "$RUN" \
  -out "$OUT"
