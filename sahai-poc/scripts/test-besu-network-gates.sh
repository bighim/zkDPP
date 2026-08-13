#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
BESU_PORT="${BESU_PORT:-9545}"
BESU_UNAUTH_PORT="${BESU_UNAUTH_PORT:-9546}"
GO_BIN="${GO_BIN:-$ROOT_DIR/../poc-v2/.cache/gomod/golang.org/toolchain@v0.0.1-go1.25.7.darwin-arm64/bin/go}"

restore() {
  docker compose --env-file besu/data/besu.env --profile besu start validator4 >/dev/null 2>&1 || true
  docker compose --env-file besu/data/besu.env --profile besu-negative stop unauthorized >/dev/null 2>&1 || true
  docker compose --env-file besu/data/besu.env --profile besu-negative rm -f unauthorized >/dev/null 2>&1 || true
}
trap restore EXIT

docker compose --env-file besu/data/besu.env --profile besu stop validator4
GOCACHE="$ROOT_DIR/.cache/go-build" \
GOMODCACHE="$ROOT_DIR/../poc-v2/.cache/gomod" \
GOTOOLCHAIN=local \
"$GO_BIN" run ./cmd/besuresilience \
  -rpc "http://127.0.0.1:${BESU_PORT}" \
  -out benchmarks/besu-validator-tolerance.json
docker compose --env-file besu/data/besu.env --profile besu start validator4

BESU_UNAUTH_PORT="$BESU_UNAUTH_PORT" \
docker compose --env-file besu/data/besu.env --profile besu-negative up -d --wait unauthorized
GOCACHE="$ROOT_DIR/.cache/go-build" \
GOMODCACHE="$ROOT_DIR/../poc-v2/.cache/gomod" \
GOTOOLCHAIN=local \
"$GO_BIN" run ./cmd/besunodecheck \
  -rpc "http://127.0.0.1:${BESU_UNAUTH_PORT}" \
  -out benchmarks/besu-node-permission-smoke.json
