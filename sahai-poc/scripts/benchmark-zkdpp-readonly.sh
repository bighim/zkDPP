#!/usr/bin/env bash
set -euo pipefail

SAHAI_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_ROOT="$(cd "$SAHAI_ROOT/../poc-v2" && pwd)"
OUTPUT_ROOT="$SAHAI_ROOT/benchmarks/zkdpp-poc"
ANVIL_PORT="${ANVIL_PORT:-18546}"
[[ "${RUNS:-1}" == "1" ]] || { echo "RUNS must be 1"; exit 1; }

TEMP_ROOT="$(mktemp -d /private/tmp/zkdpp-poc-readonly.XXXXXX)"
WORK_ROOT="$TEMP_ROOT/poc-v2"
cleanup() {
  (cd "$WORK_ROOT" && docker compose stop anvil >/dev/null 2>&1) || true
  (cd "$WORK_ROOT" && docker compose rm -f anvil >/dev/null 2>&1) || true
  chmod -R u+w "$TEMP_ROOT" >/dev/null 2>&1 || true
  rm -rf "$TEMP_ROOT" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "$WORK_ROOT" "$OUTPUT_ROOT"
rsync -a --exclude='.git' --exclude='.cache' --exclude='benchmarks' "$SOURCE_ROOT/" "$WORK_ROOT/"
mkdir -p "$WORK_ROOT/benchmarks"

(
  cd "$WORK_ROOT"
  ANVIL_PORT="$ANVIL_PORT" GOMAXPROCS=8 ./scripts/benchmark-anvil-gas.sh
  ANVIL_PORT="$ANVIL_PORT" RUNS=1 GOMAXPROCS=8 ./scripts/benchmark-anvil-e2e.sh
)

cp "$WORK_ROOT/benchmarks/anvil-transaction-gas.csv" "$OUTPUT_ROOT/anvil-mt-gas.csv"
cp "$WORK_ROOT/benchmarks/anvil-e2e-time.csv" "$OUTPUT_ROOT/anvil-mt-e2e.csv"
cp "$WORK_ROOT/benchmarks/e2e-runs/run-01.json" "$OUTPUT_ROOT/anvil-mt-e2e-run-01.json"

jq '. + {hashProfile:"poseidon2", threadMode:"MT", requestedThreads:8, goMaxProcs:8, sourceCheckoutReadOnly:true, runs:1}' \
  "$WORK_ROOT/benchmarks/anvil-transaction-gas.json" > "$OUTPUT_ROOT/anvil-mt-gas.json"
jq '. + {hashProfile:"poseidon2", threadMode:"MT", requestedThreads:8, goMaxProcs:8, sourceCheckoutReadOnly:true, runs:1}' \
  "$WORK_ROOT/benchmarks/anvil-e2e-time.json" > "$OUTPUT_ROOT/anvil-mt-e2e.json"

echo "zkDPP-POC read-only benchmark outputs: $OUTPUT_ROOT"
