# zkDPP POC Milestone 상태

| Milestone | 상태 | 구현 결과 |
|---|---|---|
| M0. POC 분리 | 완료 | 독립 Go module, Contract project와 artifact 경계 |
| M1. Canonical Model | 완료 | 8 Event ID, object와 public-input manifest |
| M2. Canonical Scenario | 완료 | 단일 source JSON과 자동 derived reference vector |
| M3. Crypto와 Tree | 완료 | Poseidon2 H/CM/compressor, depth-32 MT/rvMT cross-layer vector |
| M4. Setup과 Contract 기반 | 완료 | relation별 unsafe KZG SRS, PLONK key/verifier, Main Contract |
| M5. Entry | 완료 | Go/Solidity E2E, positive/negative test |
| M6. Exit | 완료 | nf 소비와 Tree 불변 검증 |
| M7. Merge | 완료 | 2-to-1, input별 root, aggregate relation |
| M8. Split | 완료 | 1-to-2 deterministic first-output remainder |
| M9. Transfer | 완료 | voucher/change, MT/rvMT와 deadline storage |
| M10. Proceed | 완료 | receiver authorization과 rvnf 소비 |
| M11. Recall | 완료 | sender authorization과 strict deadline |
| M12. Process | 완료 | max-3 circuit에서 active 1-to-2 및 3-to-3 P/A와 residual relation |
| M13. 통합 회귀 | 완료 | 25-call, 8 Event canonical E2E와 atomic revert |
| M14. Profile 확장 | 완료 | State 0--3, Process arity 1--3의 34 configuration |
| M15. Benchmark | 완료 | 34개 profile의 constraints, time, size와 allocation 재측정 |
| M16. Anvil Receipt Gas | 완료 | 배포 10건과 Event 25건을 독립 Prague Anvil transaction으로 측정 |
| M17. Azeroth형 E2E | 완료 (historical) | 기존 live proof부터 receipt까지 clean-chain 5회 측정; 이후 재현은 1회 정책 적용 |

## 결과 위치

- Canonical input: `testdata/scenario/canonical.json`
- Derived reference: `testdata/reference/canonical-derived.json`
- Setup artifact: `artifacts/state-3/depth-32/`
- Benchmark raw data: `benchmarks/raw.json`
- Benchmark table: `benchmarks/summary.csv`
- Canonical Event gas: `benchmarks/anvil-transaction-gas.json`
- Anvil transaction gas: `benchmarks/anvil-transaction-gas.json`, `.csv`
- Live-proof E2E: `benchmarks/anvil-e2e-time.json`, `.csv`
- E2E raw runs: `benchmarks/e2e-runs/run-*.json`
- Human-readable report: `POC Evaluation Report.tex`

`totalAllocatedBytes`는 peak memory가 아니라 profile 실행 중 Go runtime의 누적 allocation
증가량이다. 34개 circuit profile 시간은 single smoke run이다. Anvil E2E만 clean chain
기존 5회 측정은 historical artifact로 보존한다. 이후 실행은 1회이며 median/min/max 필드는
schema 호환을 위해 동일한 단일 측정값을 기록한다. Artifact loading은 E2E 밖에서 별도로 측정한다.
