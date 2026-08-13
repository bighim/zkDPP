# zkDPP POC Benchmark

이 폴더의 결과는 새 `poc-v2`의 Poseidon2, BLS12-381 PLONK-KZG, depth-32 profile만 담는다.
기존 POC와 SHA 결과는 포함하지 않는다.

## 구성

- Non-Process: 7 Event x State length 0, 1, 2, 3 = 28
- Process State scaling: 3-to-3 x State length 0, 1, 2, 3 = 4
- Process arity scaling: State-3의 1-to-1, 2-to-2 = 2
- 총 34 configuration

## 측정 경계

- `compile_ms`: gnark frontend compile
- `setup_ms`: `unsafekzg.NewSRS`와 PLONK setup
- `prove_ms`, `verify_ms`: Go PLONK prover와 verifier 단일 실행
- `total_allocated_bytes`: profile 수행 중 Go runtime `TotalAlloc` 증가량이며 peak RSS가 아님
- `anvil receipt gas`: 별도 top-level transaction의 `receipt.gasUsed`. Transaction intrinsic,
  calldata/creation input과 transaction별 cold access를 포함한다.
- `live E2E`: witness 생성 직전부터 PLONK proof 생성, ABI encoding, transaction 서명과
  Anvil receipt 수신까지다. Compile, Setup, deployment와 artifact loading은 제외한다.

## Anvil 결과

- `anvil-transaction-gas.json/csv`: 고정 proof를 이용한 배포 10건과 Event 25건의 receipt gas
- `anvil-e2e-time.json/csv`: live proof clean-chain 1회 결과와 median/min/max
- `e2e-runs/run-*.json`: 각 반복의 원시 시간, transaction hash, gas와 calldata 크기

논문용 transaction gas는 `anvil-transaction-gas.json`의 fixed-proof receipt를 사용한다.
Live proof는 매번 proof bytes가 달라 calldata zero-byte 수와 gas가 조금 변하므로 E2E 결과의
gas는 보조값이다.

## 주의

Profile 시간 결과는 Apple M1 Pro 10-core, 32 GB, macOS 26.3에서 수행한 single smoke run이다.
Anvil E2E 시간은 clean chain 1회를 사용한다. median/min/max 필드는 schema 호환을 위해
유지하며 동일한 단일 측정값을 담는다.
SRS는 process-local cache를 사용할 수 있으므로 `setup_ms`는 production setup 성능이나
독립 반복 평균으로 해석하면 안 된다. 비교에는 constraints, key/SRS 크기, proof 크기와
Anvil receipt gas를 함께 사용해야 한다.
