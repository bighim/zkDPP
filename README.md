# zkDPP Supply-Chain Consistency

Digital Product Passport의 supply-chain transition 과정에서
consistency-critical State를 영지식증명으로 검증하는 연구 POC다.

## 저장소 구성

- `Scheme/`: protocol의 canonical specification, development specification,
  decision log
- `poc-v2/`: Go/gnark circuit, Solidity contract, canonical scenario와
  Anvil benchmark
- `Papers/`: 연구 과정에서 수집한 논문과 이해 노트
- `IEEE Blockchain Conference/`: IEEE Blockchain 관련 조사 자료
- `Azeroth Contract/`: 구조 비교를 위해 보관한 Azeroth 참고 구현

`Papers/`, `IEEE Blockchain Conference/`, `Azeroth Contract/`는 POC의
runtime dependency가 아니다. 각 외부 자료와 참고 구현의 저작권 및 이용 조건은
원저자와 원배포처에 따른다.

## Canonical 문서

1. `Scheme/Protocol Spec.tex`
2. `Scheme/Dev Spec.tex`
3. `Scheme/Decision Log.tex`

Protocol 의미가 충돌하면 Protocol Specification을 우선한다. 구현 layout과 benchmark
정의는 Development Specification을 따른다.

## POC

POC는 Entry, Transfer, Proceed, Recall, Merge, Split, Process, Exit의 8개 Event를
구현한다.

- Go, gnark v0.15.0
- PLONK-KZG on BLS12-381
- Poseidon2
- MT 및 rvMT depth 32
- Solidity, Foundry v1.7.1
- Prague Anvil

자세한 구조와 결과는 `poc-v2/README.md` 및
`poc-v2/POC Evaluation Report.tex`에서 확인한다.

## 재현

필수 환경:

- GNU Make
- Go 1.25.7
- Docker
- Docker Compose v2
- 최초 실행 시 Go module과 Docker image를 받기 위한 인터넷 연결

현재 Docker image platform은 Apple Silicon 측정 환경에 맞춰 `linux/arm64`로
고정되어 있다.

Clean clone에서 Go test, Setup, Contract test를 한 번에 실행한다.

```bash
cd poc-v2
make test-e2e
```

34개 circuit profile과 canonical 25-Event Anvil benchmark를 실행한다.

```bash
cd poc-v2
make benchmark benchmark-anvil
```

Setup 과정에서 생성되는 SRS, PLONK key, proof, generated verifier와 build cache는
Git에 포함하지 않는다.

`unsafekzg.NewSRS`와 PLONK proof는 randomness를 사용하므로 새로 생성되는 SRS, key,
proof와 verifier byte는 기존 측정 때와 다를 수 있다. Constraints와 protocol 동작은
동일하게 재현되지만 시간과 gas는 저장된 결과와 완전히 같지 않을 수 있다. 기존 측정
결과는 `poc-v2/benchmarks/`에 보존한다.

## 테스트 데이터

`poc-v2/testdata/scenario/canonical.json`의 secret key와
Anvil 기본 mnemonic은 재현 전용 공개 테스트 값이다. `Azeroth Contract/keys.json`의
private key도 원 구현의 테스트 값이다. 이 값을 실제 key로 사용하면 안 된다.
