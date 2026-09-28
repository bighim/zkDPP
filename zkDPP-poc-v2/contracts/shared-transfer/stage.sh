#!/bin/sh
# Build staging copies only. Canonical source and old experiment artifacts remain unchanged.
set -eu
cd "$(dirname "$0")/../.."
mkdir -p contracts/shared-transfer/shared/experiments
cp contracts/src/experiments/SharedSecretAuditLedger.sol contracts/shared-transfer/shared/experiments/
cp contracts/src/IFieldHasher.sol contracts/src/IZkVerifier.sol contracts/shared-transfer/shared/
cp contracts/experiments/shared/generated/experiments/Poseidon2BLS12381.sol contracts/shared-transfer/shared/
cp artifacts/development/shared-transfer/contracts/S0.sol artifacts/development/shared-transfer/contracts/S1.sol artifacts/development/shared-transfer/contracts/S2.sol contracts/shared-transfer/shared/

