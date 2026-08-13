#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
[[ "${RUNS:-1}" == "1" ]] || { echo "RUNS must be 1"; exit 1; }
BESU_PORT="${BESU_PORT:-9545}"
DEPLOYER_PK="${EVM_DEPLOYER_PK:-ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80}"
GO_BIN="$ROOT_DIR/../poc-v2/.cache/gomod/golang.org/toolchain@v0.0.1-go1.25.7.darwin-arm64/bin/go"
GO_ENV=(env GOCACHE="$ROOT_DIR/.cache/go-build" GOMODCACHE="$ROOT_DIR/../poc-v2/.cache/gomod" GOTOOLCHAIN=local)
SERVICES=(validator1 validator2 validator3 validator4 rpc)
RUN_DIR="$ROOT_DIR/benchmarks/besu-poseidon2-gas-runs"

cleanup() {
  docker compose --env-file besu/data/besu.env --profile besu stop "${SERVICES[@]}" >/dev/null 2>&1 || true
  docker compose --env-file besu/data/besu.env --profile besu rm -f "${SERVICES[@]}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$RUN_DIR"
if [[ -f "$RUN_DIR/run-01.json" ]]; then
  previous_checksum="$(jq -r '.fixtureSha256' "$RUN_DIR/run-01.json")"
  expected_checksum="$(shasum -a 256 testdata/generated/canonical-poseidon2-mt-payloads.json | awk '{print $1}')"
  if [[ "$previous_checksum" != "$expected_checksum" ]]; then
    mv "$RUN_DIR/run-01.json" "$RUN_DIR/invalid-st-fixture-run.json"
  else
    echo "valid Besu gas run already exists; refusing to repeat" >&2
    exit 1
  fi
fi
./scripts/setup-besu.sh
cp besu/data/checksums.sha256 "$RUN_DIR/run-01-network.sha256"
BESU_PORT="$BESU_PORT" docker compose --env-file besu/data/besu.env --profile besu up -d --wait "${SERVICES[@]}"

"${GO_ENV[@]}" "$GO_BIN" run ./cmd/besusmoke \
  -rpc "http://127.0.0.1:${BESU_PORT}" -out benchmarks/besu-network-smoke.json
EVM_DEPLOYER_PK="$DEPLOYER_PK" "${GO_ENV[@]}" "$GO_BIN" run ./cmd/evmrun \
  -root "$ROOT_DIR" -backend besu -rpc "http://127.0.0.1:${BESU_PORT}" -chain-id 31337 \
  -receipt-timeout 120s -poll-interval 50ms -profile poseidon2 -scenario canonical -threads 8 \
  -run 1 -out benchmarks/besu-poseidon2-gas-runs/run-01.json
"${GO_ENV[@]}" "$GO_BIN" run ./cmd/besupermission \
  -root "$ROOT_DIR" -rpc "http://127.0.0.1:${BESU_PORT}" \
  -run benchmarks/besu-poseidon2-gas-runs/run-01.json -out benchmarks/besu-permission-smoke.json
BESU_PORT="$BESU_PORT" GO_BIN="$GO_BIN" ./scripts/test-besu-network-gates.sh
"${GO_ENV[@]}" "$GO_BIN" run ./cmd/evmreport \
  -input 'benchmarks/besu-poseidon2-gas-runs/run-*.json' -runs 1 \
  -out-json benchmarks/besu-poseidon2-gas.json -out-csv benchmarks/besu-poseidon2-gas.csv
"${GO_ENV[@]}" "$GO_BIN" run ./cmd/backendparity \
  -anvil benchmarks/anvil-poseidon2-gas.json -besu benchmarks/besu-poseidon2-gas.json \
  -out benchmarks/backend-parity.json
