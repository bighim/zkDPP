# Evaluation source

These files identify the evaluation code used by the paper. They extend base
commit `483f4a0280d832f95a6842997ca89e35bcbb0879`; the recorded source files
are preserved without changing the circuits, measurements, or algorithms.

| Evaluation | Source | Supporting code |
| --- | --- | --- |
| Mixed production, transfer resolution and containment | [driver](../cmd/integrated_containment/main.go), [scenario](../internal/v2case/integrated_containment.go) | [proof setup](../internal/hotfix/integration_proofs.go), [ledger execution](../internal/hotfix/integration_evm.go), [audit source](../internal/hotfix/integration_source.go), [result checks](mixed/audit_results.py) |
| Same-function Merge, U/D | [circuits and native comparison](../internal/mergesame), [native entry](../cmd/mergesame/main.go), [EVM entry](../cmd/sameevm/main.go) | [router and ledger adapter](same-merge/contracts/src/SameMergeRouter.sol), [ledger controls](same-merge/contracts/test/SameMerge.t.sol), [aggregation](same-merge/summarize.py) |
| Sahai-style A/B and audit-inclusive C | [circuits and native comparison](../internal/mergecompare), [native entry](../cmd/mergecompare/main.go), [EVM entry](../cmd/mergeevm/main.go) | [router](three-merge/contracts/src/MergeProofRouter.sol), [controls](three-merge/contracts/test/MergeCompare.t.sol), [aggregation](three-merge/summarize.py) |
| Parent-reference and lookup-table comparison | [standalone Go module](audit-query/harness), [driver](audit-query/harness/cmd/auditcompare/main.go) | [aggregation](audit-query/analyze_results.py) |

The first three evaluations use the existing `zkDPP-poc-v2` Go module. The
audit-query harness has its own `go.mod` and `go.sum`. The proof setup code
generates verifier contracts and proof fixtures needed by the Solidity tests.
This source publication does not add generated keys, setup artifacts, or new
benchmark results.

## Current numerical data

The source index above records the b11b6e7 publication. The 2026-10-16 run01 numerical refresh is provided as [current tables](../docs/paper-data/current-results.md), [official numeric observations and aggregation instructions](../docs/paper-data/README.md). This numeric refresh preserves the original measurements and regenerates the published CSVs without rerunning benchmarks.
