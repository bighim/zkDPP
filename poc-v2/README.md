# zkDPP Protocol POC

이 폴더는 `../Scheme/Protocol Spec.tex`을 구현하는 독립 POC다.

## Source of truth

우선순위는 다음과 같다.

1. Protocol 의미, 객체와 Relation: `../Scheme/Protocol Spec.tex`
2. 구현 layout, ABI, artifact와 benchmark: `../Scheme/Dev Spec.tex`
3. 결정 이유와 반려안: `../Scheme/Decision Log.tex`

문서가 충돌하면 Protocol 의미는 Protocol Specification을 따른다. 구현 세부사항이 없거나
서로 충돌하면 임의로 결정하지 않고 Development Specification의 Open Decisions에 기록한다.

## 구현 목표

- Event: Entry, Transfer, Proceed, Recall, Merge, Split, Process, Exit
- StateVector 기본 길이: 3
- MT 및 rvMT depth: 32
- Process 기본 profile: M_max = N_max = 3, active 1 <= m,n <= 3
- Proof: PLONK-KZG on BLS12-381
- Hash: gnark BLS12-381 default Poseidon2
- POC SRS: unsafekzg.NewSRS
- Contract test: Foundry v1.7.1, Prague EVM

## 현재 상태

- [x] 새 module과 재사용 경계 생성
- [x] Poseidon2 native/circuit primitive 분리
- [x] Depth-32 native/circuit Merkle primitive 분리
- [x] canonical data model과 public input manifest
- [x] canonical scenario와 derived reference vector
- [x] MT 및 rvMT Contract component
- [x] 8개 Event circuit과 PLONK verifier
- [x] 8개 Event Contract integration
- [x] positive/negative E2E test
- [x] State/Process arity 34-configuration benchmark matrix

## Canonical scenario

사람이 결정한 입력은 `testdata/scenario/canonical.json` 한 곳에만 둔다. Derived hash,
commitment, nullifier, voucher, Merkle root와 remainder는 다음 명령으로 생성한다.

```bash
make generate-reference
```

결과는 `testdata/reference/canonical-derived.json`이다. Entry State는 모두 nano scale의
whole-unit 값이며, Split, Transfer와 Process는 nonzero remainder를 포함한다.

## 재현 명령

필수 환경은 GNU Make, Go 1.25.7, Docker와 Docker Compose v2다. 최초 실행 시
Go module과 Docker image를 받기 위한 인터넷 연결이 필요하다. 현재 Docker image
platform은 Apple Silicon 측정 환경에 맞춰 `linux/arm64`로 고정되어 있다.

Clean clone에서 전체 기능 검증:

```bash
make test-e2e
```

전체 성능 측정:

```bash
make benchmark
make benchmark-anvil
```

- `make test-e2e`: Go test, Setup과 Contract test를 순서대로 실행
- `make setup`: canonical State-3, depth-32의 8개 verifier와 proof fixture 생성
- `make test-contract`: 실제 PLONK proof로 8개 Event E2E 및 negative test 수행
- `make benchmark`: Poseidon2, depth-32의 34개 compile/setup/prove/verify profile 실행
- `make benchmark-anvil-gas`: 고정 proof로 10개 배포와 25개 Event를 독립 Anvil transaction으로 제출
- `make benchmark-anvil-e2e`: live proof 생성부터 receipt까지 clean Anvil scenario를 5회 측정
- `make benchmark-anvil`: receipt gas와 E2E benchmark를 모두 실행

빠른 E2E smoke run은 `make benchmark-anvil-e2e RUNS=1`로 실행한다. Host port 8545가
사용 중이면 `ANVIL_PORT=18545`처럼 변경할 수 있다.

## Benchmark 결과

- Raw result: `benchmarks/raw.json`
- Flat table: `benchmarks/summary.csv`
- Anvil transaction gas: `benchmarks/anvil-transaction-gas.json`, `.csv`
- Anvil live-proof E2E time: `benchmarks/anvil-e2e-time.json`, `.csv`
- E2E raw runs: `benchmarks/e2e-runs/run-*.json`

34개 configuration은 Non-Process 7개 Event의 State 0--3 profile 28개, Process 3-to-3의
State 0--3 profile 4개, State-3 Process 1-to-1 및 2-to-2 profile 2개다. SHA와 depth scaling은
포함하지 않는다.

측정 환경, constraints, native prove/verify 시간, Anvil receipt gas와 E2E 시간은
`POC Evaluation Report.tex` 및 `benchmarks/summary.csv`에서 확인한다.
Anvil E2E는 clean chain 5회의 median과 min/max를 기록한다.

## 재현성 경계

`artifacts/`, generated verifier, proof fixture와 build cache는 Git에 포함하지 않는다.
`make setup`과 benchmark 명령이 필요한 파일을 다시 생성한다.

`unsafekzg.NewSRS`와 PLONK proof는 randomness를 사용하므로 새 실행의 SRS, key, proof와
verifier byte는 기존 측정 때와 다를 수 있다. Constraints와 protocol 동작은 동일하게
재현되지만 시간과 gas는 저장된 값과 완전히 같지 않을 수 있다. 이 저장소의
`benchmarks/` 결과가 현재 Evaluation Report에 사용한 측정 원본이다.
