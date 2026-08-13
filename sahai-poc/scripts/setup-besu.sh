#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BESU_DIR="$ROOT_DIR/besu"
IMAGE="hyperledger/besu@sha256:5c319f8f5f3449438c03ea7fa2c9bf24b866dc55ac98d802bb41ad793e740587"
GENERATED="$BESU_DIR/generated"
DATA="$BESU_DIR/data"

rm -rf "$GENERATED" "$DATA"
mkdir -p "$DATA/rpc"
mkdir -p "$DATA/unauthorized"

if ! docker run --rm -v "$BESU_DIR:/config" "$IMAGE" operator generate-blockchain-config \
  --config-file=/config/qbftConfigFile.json \
  --to=/config/generated; then
  # Besu 26.7.1 can return "Output directory already exists" after it has
  # successfully materialized the requested files through a bind mount.
  [[ -f "$GENERATED/genesis.json" ]] || exit 1
  generated_keys="$(find "$GENERATED/keys" -name key.priv -type f | wc -l | tr -d ' ')"
  [[ "$generated_keys" == "4" ]] || exit 1
fi

KEY_DIRS=()
while IFS= read -r key_dir; do
  KEY_DIRS+=("$key_dir")
done < <(find "$GENERATED/keys" -mindepth 1 -maxdepth 1 -type d | sort)
if [[ "${#KEY_DIRS[@]}" -ne 4 ]]; then
  echo "expected four validator key directories, got ${#KEY_DIRS[@]}" >&2
  exit 1
fi

NODE_ENODES=()
for index in 0 1 2 3; do
  node_number=$((index + 1))
  node_dir="$DATA/validator${node_number}"
  mkdir -p "$node_dir/database"
  cp "${KEY_DIRS[$index]}/key.priv" "$node_dir/key.priv"
  cp "${KEY_DIRS[$index]}/key.pub" "$node_dir/key.pub"
  public_key="$(tr -d '\r\n' < "$node_dir/key.pub")"
  NODE_ENODES+=("enode://${public_key#0x}@validator${node_number}:30303")
done

# Test-only deterministic RPC node identity. It is not a funded transaction key.
printf '%064x' 5 > "$DATA/rpc/key.priv"
docker run --rm -v "$BESU_DIR:/config" "$IMAGE" public-key export \
  --node-private-key-file=/config/data/rpc/key.priv \
  --to=/config/data/rpc/key.pub
rpc_public="$(tr -d '\r\n' < "$DATA/rpc/key.pub")"
NODE_ENODES+=("enode://${rpc_public#0x}@rpc:30303")

# A fixed identity intentionally omitted from nodes-allowlist for the negative gate.
printf '%064x' 1 > "$DATA/unauthorized/key.priv"
docker run --rm -v "$BESU_DIR:/config" "$IMAGE" public-key export \
  --node-private-key-file=/config/data/unauthorized/key.priv \
  --to=/config/data/unauthorized/key.pub

{
  printf 'accounts-allowlist=['
  printf '"%s",' \
    0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266 \
    0x70997970c51812dc3a010c7d01b50e0d17dc79c8 \
    0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc \
    0x90f79bf6eb2c4f870365e785982e1f101e93b906 \
    0x15d34aaf54267db7d7c367839aaf71a00a2c6a65
  printf '"%s",' 0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc
  printf '"%s"]\n' 0x2b5ad5c4795c026514f8317c7a215e218dccd6cf
  printf 'nodes-allowlist=['
  for index in 0 1 2 3; do printf '"%s",' "${NODE_ENODES[$index]}"; done
  printf '"%s"]\n' "${NODE_ENODES[4]}"
} > "$DATA/permissions_config.toml"

printf '[' > "$DATA/static-nodes.json"
for index in 0 1 2 3; do printf '"%s",' "${NODE_ENODES[$index]}" >> "$DATA/static-nodes.json"; done
printf '"%s"]\n' "${NODE_ENODES[4]}" >> "$DATA/static-nodes.json"
for node_dir in "$DATA/validator1" "$DATA/validator2" "$DATA/validator3" "$DATA/validator4" "$DATA/rpc"; do
  cp "$DATA/permissions_config.toml" "$node_dir/permissions_config.toml"
  cp "$DATA/static-nodes.json" "$node_dir/static-nodes.json"
done
printf '["%s"]\n' "${NODE_ENODES[0]}" > "$DATA/unauthorized/static-nodes.json"

printf '%s\n' "${NODE_ENODES[0]}" > "$DATA/bootnode.enode"
printf 'BESU_BOOTNODES=%s\n' "${NODE_ENODES[0]}" > "$DATA/besu.env"
shasum -a 256 "$GENERATED/genesis.json" "$DATA/permissions_config.toml" "$DATA/static-nodes.json" > "$DATA/checksums.sha256"
echo "Besu QBFT files generated under $BESU_DIR"
cat "$DATA/checksums.sha256"
