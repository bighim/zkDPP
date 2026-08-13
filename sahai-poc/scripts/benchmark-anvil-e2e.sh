#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
[[ "${RUNS:-1}" == "1" ]] || { echo "RUNS must be 1"; exit 1; }
ANVIL_PORT="${ANVIL_PORT:-18545}"
cleanup(){ ANVIL_PORT="$ANVIL_PORT" docker compose stop anvil >/dev/null 2>&1 || true; ANVIL_PORT="$ANVIL_PORT" docker compose rm -f anvil >/dev/null 2>&1 || true; }
trap cleanup EXIT
for profile in poseidon2 sha256; do
  run_dir="$ROOT_DIR/benchmarks/anvil-${profile}-mt-e2e-runs"
  mkdir -p "$run_dir"
  cleanup
  ANVIL_PORT="$ANVIL_PORT" docker compose up -d --wait anvil
  GOCACHE="$ROOT_DIR/.cache/go-build" GOMODCACHE="$ROOT_DIR/../poc-v2/.cache/gomod" GOTOOLCHAIN=go1.25.7 \
  go run ./cmd/evmrun -root "$ROOT_DIR" -backend anvil -rpc "http://127.0.0.1:${ANVIL_PORT}" -chain-id 31337 \
    -receipt-timeout 30s -poll-interval 10ms -profile "$profile" -scenario canonical -threads 8 -live-proofs \
    -run 1 -out "benchmarks/anvil-${profile}-mt-e2e-runs/run-01.json"
  GOCACHE="$ROOT_DIR/.cache/go-build" GOMODCACHE="$ROOT_DIR/../poc-v2/.cache/gomod" GOTOOLCHAIN=go1.25.7 \
  go run ./cmd/evmreport -input "benchmarks/anvil-${profile}-mt-e2e-runs/run-*.json" -runs 1 \
    -out-json "benchmarks/anvil-${profile}-mt-e2e.json" -out-csv "benchmarks/anvil-${profile}-mt-e2e.csv"
done
