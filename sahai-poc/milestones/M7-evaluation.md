# M7 - Besu QBFT permissioned backend

- Status: PASS
- Besu 26.7.1, validator 4 + RPC 1, chain ID 31337, Prague, 30M gas로 실행했다.
- 동일 Poseidon2 MT fixture checksum/calldata에서 Anvil/Besu gas가 6개 Event와 2개 baseline 모두 정확히 일치했다.
- outside node, outside account, Contract participant role 없는 account를 모두 거부했다.
- validator 1개 중단 상태에서 transaction finality를 확인했다.
- Poseidon2 MT live-proof E2E/finality를 1회 측정했다.
- ST fixture로 잘못 실행된 최초 진단 run은 `invalid-st-fixture-run.json`으로 분리하고 공식 결과에서 제외했다.
