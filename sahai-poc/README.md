# RSA-free Sahai-POC

Sahai et al. (2020)의 Event predicate workload를 gnark PLONK-KZG와 EVM에서 재현하고 기존 `poc-v2`(`zkDPP-POC`)와 비교하는 독립 POC다.

> **현재 상태: provisional.** 현재 benchmark와 보고서는 Merkle `LeafIndex`가
> private witness였던 artifact로 생성되었다. Sahai 논문과 같이 index를 public statement로
> binding하고 Event contract가 schema leaf 0을 강제하도록 수정한 뒤, circuit/key/verifier와
> 성능 수치를 재생성해야 한다. 현재 결과를 최종 protocol-conformant 수치로 인용하지 않는다.

공식 비교 구현은 RSA accumulator를 제거한다. `DocHash`가 숨겨진 `DocInfo`의 commitment와 ledger document ID를 겸하며, `InDocs`가 연결된 직접 provenance DAG를 보존한다. 모든 input은 항상 consumed 처리되는 hardened semantics만 사용한다.

- 기본 hash profile: Poseidon2
- ablation profile: SHA-256
- 공식 Event/E2E 비교: MT, `GOMAXPROCS=8`
- ST gadget 결과: Sahai 논문 Table III 참고용 diagnostic only
- proof system/curve: PLONK-KZG / BLS12-381
- EVM: Solidity 0.8.30, Prague, Anvil 및 Besu QBFT

`poc-v2`의 protocol, circuit, contract semantics는 변경하지 않았다. 비교 실험을 위해
E2E runner에 backend/RPC/chain ID/receipt timeout/poll interval 옵션을 추가하고, Anvil과
Besu에서 같은 시나리오를 실행하도록 parameterization했다. 새 비교 결과는
`sahai-poc/benchmarks` 아래에도 복사해 보고서 입력으로 사용한다.

```bash
make conformance
make setup
make test-go
make event-fixtures
make test-contract
make benchmark-anvil
make benchmark-anvil-e2e
make benchmark-besu
make compare-poc-v2
make report
```

`make reproduce`는 test와 benchmark를 각 조합당 한 번만 실행한다. RSA-free adaptation은 accumulator 기반 upstream set membership/non-membership 및 contamination query를 재현하지 않으므로, 논문 RSA 수치와 직접 동등 비교하지 않는다.

## Git에 보존하는 경계

- 보존: source, contract, test, benchmark 요약 JSON/CSV, 환경 metadata, TeX 보고서, milestone.
- 재생성: Go/Foundry cache, CCS/SRS/key, generated Solidity verifier, proof fixture,
  Besu data/genesis, raw run directory, LaTeX/PDF/rendering 산출물.
- benchmark 요약은 실험 근거와 보고서 입력이므로 자동 재생성 가능하더라도
  Git에 보존한다. Raw transaction/run file은 요약에서 중복되므로 ignore한다.
