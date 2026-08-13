# RSA-free Sahai-POC conformance specification

이 문서는 Sahai 논문의 원형과 공식 비교에 사용하는 RSA-free adaptation을 구분한다.

## Document와 provenance

- **[Paper]** `DocPrime`은 document ID이고, `DocHash`는 숨겨진 `DocInfo`의 salted Merkle commitment이다. `DocAccumulator`는 upstream document ID 집합을 요약하고 `InDocs`는 직접 provenance edge를 기록한다.
- **[Adaptation]** 공식 `Sahai-POC`에서는 `DocPrime`, prime certificate, `DocAccumulator`, membership/non-membership, RPoKE를 제거한다.
- **[Adaptation]** `DocHash`를 commitment이자 ledger document ID로 사용한다. `InDocs`는 유지하므로 직접 provenance DAG는 보존하지만 accumulator 기반 contamination query는 제공하지 않는다.
- **[Adaptation]** 모든 input은 한 번만 사용할 수 있다. duplicate `DocHash`, consumed input, terminal input reference를 거부하며 별도의 paper mode는 없다.

## Hash profile

- **[Paper]** `DocInfo`는 32개의 128-bit attribute이며 leaf당 4개 attribute, leaf 8개, sibling 3개로 구성된다.
- **[Adaptation]** 기본 profile은 Poseidon2다. attribute commitment `Poseidon2(value,salt)`, 4개 commitment의 leaf hash, `Poseidon2(left,right)` internal compression을 사용하며 digest는 BLS12-381 scalar field element 1개로 공개한다.
- **[Adaptation]** SHA-256 ablation은 `SHA-256(value||salt)`와 기존 64-byte leaf/internal compression을 사용하며 digest를 128-bit limb 2개로 공개한다.
- **[Experiment]** profile별 circuit, key, verifier, golden vector를 별도로 생성한다. 서로 다른 profile의 proof/verifier는 호환하지 않는다.

## Event predicates

| Event | 전이 | 검증 관계 | 독립 proof bundle |
|---|---:|---|---:|
| Entry | 0→1 | output Sender = Recipient | Path 1, Eq 1 |
| Ship | 1→1 | input Recipient = output Sender, ItemCode와 Quantity 유지 | Path 2, Eq 3 |
| Merge | 2→1 | 두 input recipient = output sender, quantity 합 | Path 3, Eq 2, Add 1 |
| Split | 1→2 | input recipient = 각 output sender, output quantity 합 = input | Path 3, Eq 2, Add 1 |
| Process | 2→1 | 이 POC에서는 Merge와 같은 relation profile | Path 3, Eq 2, Add 1 |
| Exit | 1→1 terminal | output Sender = Recipient | Path 1, Eq 1 |

- **[Paper]** Merge predicate에 없는 `ItemCode` equality는 추가하지 않는다.
- **[Adaptation]** Exit는 논문 Table V/VI의 Path 1, Eq 1을 따른다. 새 terminal `DocHash`를 만들고 기존 input을 consumed 처리한다.

## Canonical scenario

- **[Experiment]** A100 Entry → Ship → Split(40,60) → Merge(100), B50 Entry → Ship, A100+B50 Process(150) → Exit의 연결된 8 transaction을 공식 비교에 사용한다.
- **[Experiment]** Entry와 Ship은 각각 2 case, 나머지는 1 case다. 동일 Event의 값은 case 중앙값으로 집계한다.
- **[Experiment]** Event별 독립 fixture는 component diagnostic에만 사용하고 공식 zkDPP 비율에는 쓰지 않는다.

## Measurement policy

- **[Experiment]** 공식 Event 및 zkDPP 비교는 MT(`GOMAXPROCS=8`)만 사용한다. 독립 proof 호출은 항상 순차적이며 gnark 내부 병렬성만 사용한다. 이미 측정한 gadget ST는 논문 Table III 참고용 진단값일 뿐 공식 protocol 비교에는 사용하지 않는다.
- **[Experiment]** gas는 fixed proof transaction의 receipt `gasUsed`를 profile마다 한 번 측정한다. gas를 금전 fee로 바꾸지 않는다.
- **[Experiment]** live-proof E2E는 MT에서 한 번 측정하며 build, witness, prove, encode, submit-to-receipt를 분리한다.
- **[Experiment]** Anvil을 공식 EVM 비교 backend로 사용한다. Besu QBFT는 gas parity와 finality latency를 별도 section에서 다룬다.
- **[Experiment]** `poc-v2`는 수정하지 않고 읽기 전용으로 실행한다. 새 결과는 모두 `sahai-poc/benchmarks`에 저장한다.
