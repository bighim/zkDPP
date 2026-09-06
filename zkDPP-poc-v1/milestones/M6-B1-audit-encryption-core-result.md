# M6-B1 감사 암호화 코어 — 무엇을 확인했나요?

- 상태: 완료
- 구현 기준: [명세](M6-B1-audit-encryption-core.md), [Background](M6-B1-audit-encryption-core-background.md)
- Raw: [Circuit](../output/m6-b1-circuit.json), [gas](../output/m6-b1-anvil-gas.json), [복호화](../output/m6-b1-decryption.json), [checksum·실행 이력](../output/m6-b1-generated-checksums.json)
- 실행 횟수: Setup·evaluate 각 1회, 전체 Go·Foundry suite 각 1회입니다. Gas는 추적 설정 실패 후 2번째 시도에 성공했으며, 복호화 workload는 크기별 1회입니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

**감사에 필요한 정보를 암호화해 저장하고, 위원 두 명이 협력하면 정확히 복원하는 기능을 만들었습니다. 실제 Process에 연결해 엉뚱한 정보를 암호화하는 것도 막았습니다.**

| 질문 | M6-B1 결과 |
|---|---|
| 이전 상태 | M6는 StatusTree로 사용 가능 여부를 집행했지만 감사 암호화 코어는 없었습니다. |
| 이번 목표 | 감사 정보를 숨기고, 올바른 암호화와 2-of-3 복원을 검증하는 것입니다. |
| 실제 구현 | 공통 암호화·복호화 함수, 실제 Process와 암호문을 함께 검증하는 Circuit, 실험용 저장 Contract입니다. |
| 이제 가능한 것 | 실제 부모 cm 3개·출력이 나중에 사용할 nf 2개를 숨겨 기록하고, 위원 두 명으로 복원합니다. |
| 검증 | 전체 Go suite·Foundry 25개 test 통과, 세 위원 쌍 모두 동일 원문 복원입니다. |
| 대표 성능 | Process와 암호화를 함께 증명하는 데 약 1.75초, 15개 값의 검증·저장에 736,120 gas, 암호문 1,000개 복원에 약 683 ms입니다. |
| 미포함 | 전체 AuditRecord 원장, 다른 Event 적용, 그래프 탐색, nullifier 기반 동결 전환입니다. |
| 다음 단계 | 이 코어를 각 Event·원장·소비 기록에 연결하는 통합 설계입니다. 기존 M6는 그대로 유지합니다. |

**736,120 gas는 최소 AuditRecord 저장 비용이 아닙니다.** 암호문은 7개 Field이지만 이번 실험은 조회 편의를 위해 기존 공개값 8개까지 합쳐 15개를 저장했습니다. [저장한 내용과 이유](#4-어떤-상태를-저장하나요)를 구분해서 읽어야 합니다.

## 1. 이전 milestone에서 무엇이 달라졌나요?

**M6 Main Contract를 바꾸지 않고, M5 Process에 감사 정보를 추가하는 별도 실험 경로를 만들었습니다.**

| 이전 기능 | 이번에 추가한 기능 |
|---|---|
| M5의 private 3→2 Process 검증 | 같은 Process와 감사 암호문의 연결을 함께 증명합니다. |
| 기존 Poseidon2·Note·nullifier 계산 | 동일 계산을 재사용해 실제 감사 평문을 구성합니다. |
| M6의 StatusTree 집행 | 변경하지 않았습니다. 이번에는 동결 기능을 연결하지 않습니다. |

따라서 아래 gas는 Main Ledger의 Process 실행 비용이 아니라 **감사 Process proof를 검증하고 기록하는 진단 비용**입니다.

## 2. 실제로 어떤 흐름이 가능한가요?

**원자재 A·B·C로 제품 D와 폐기물 E를 만드는 같은 예시로 세 기능을 확인했습니다.**

$$
(cm_A,cm_B,cm_C)\xrightarrow{\mathrm{Process}}(cm_D,cm_E)
$$

### 먼저 무엇을 암호화했나요?

| 숨길 정보 | 감사할 때 필요한 이유 |
|---|---|
| 부모 $cm_A,cm_B,cm_C$ | 어떤 원자재에서 왔는지 거슬러 올라가기 위해서입니다. |
| 출력의 $nf_D,nf_E$ | D·E가 나중에 어디에서 소비됐는지 찾기 위해서입니다. |

이 다섯 값을 하나의 키로 암호화했습니다. Field는 여기서 cm이나 nf 같은 값 하나를 담는 숫자 단위입니다.

$$
\mathbf M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

공통 함수는 값을 받아 암호화하고, 위원들의 응답을 받아 복원합니다. Jubjub과 기존 Poseidon2를 사용하며, 키 하나에서 위치별 마스크를 만들어 각 값을 숨깁니다.

### 단순히 암호화만 하면 충분한가요?

**아닙니다. 실제로 A를 소비했는데 관계없는 X를 암호화하면, 복호화가 성공해도 감사가 끊깁니다.** 그래서 두 Circuit을 나누어 확인했습니다.

| Circuit | 확인하는 질문 |
|---|---|
| 독립 암호화 Circuit | 입력받은 값을 올바르게 암호화했는가? |
| 암호화를 추가한 Process Circuit | Process 규칙을 지켰고, 실제 부모와 올바른 출력 소비 nf를 암호화했는가? |

테스트에서도 엉뚱한 부모를 정상적으로 암호화하면 첫 Circuit은 통과했지만 두 번째는 실패했습니다. 두 번째 Circuit은 임의의 평문 배열을 신뢰하지 않고 실제 Note와 owner secret에서 감사 정보를 계산합니다.

**Process에 제출할 proof는 하나입니다.** 두 proof를 순서대로 제출하는 것이 아니라, Process 규칙과 암호화 관계를 하나의 Circuit에서 함께 증명합니다. 문서·코드의 Process adapter는 기존 M5 Circuit에 이 검사를 덧붙인 구현을 뜻합니다.

기존 M5의 숫자 예시도 유지했습니다. State 순서는 질량·재활용 귀속량·탄소이며, 입력 합 (320, 30, 230)은 주제품 (270, 30, 260)과 폐기물 (30, 0, 0)로 변환됩니다. 질량·재활용 단위는 kg, 탄소는 kgCO2e입니다.

### 저장한 원본을 실제로 복원했나요?

**네. 실험용 Contract에 기록하고, 그 기록을 읽어서 원래 다섯 값과 비교했습니다.**

1. Participant가 암호문과 Process proof를 준비했습니다.
2. AuditEncryptionStore가 proof를 검증하고 같은 공개값을 새 기록 ID에 저장했습니다.
3. 위원 두 명이 같은 기록을 각각 읽어 자기 share로 partial decryption을 만들었습니다.
4. Auditor가 두 응답을 결합하자 부모 cm 세 개와 출력 소비 nf 두 개가 정확히 돌아왔습니다.

온체인 원본 실험은 위원 1·2로 수행했습니다. 별도 correctness test에서는 1·3, 2·3 조합도 같은 원문을 복원했습니다.

**이것은 전체 AuditRecord 원장 구현이 아닙니다.** Note 소비·Tree 갱신·PolicyGrant·Status 집행·그래프 탐색에는 아직 연결하지 않았습니다.

## 3. 무엇이 공개되고 무엇이 숨겨지나요?

| 경로 | 공개 입력 | 비공개 내용 |
|---|---|---|
| 독립 코어 | 문맥 $L$, 공개점 $R_1$의 좌표 2개, 암호화된 값 5개 — 총 8개입니다. | 평문 5개와 암호화 난수입니다. |
| 감사 Process | 기존 M5 입력 8개 뒤에 공개점 좌표 2개·암호화된 값 5개 — 총 15개입니다. | 기존 Note·owner secret·path와 암호화 난수입니다. |

Process의 기존 공개 입력은 PolicyRef, ScopeRef, noteRoot, 입력 nf 3개, 출력 cm 2개입니다. 문맥 $L$은 이 값과 PROCESS EventKind를 정해진 순서로 Hash한 값입니다. 암호화·복호화에서 같은 키와 마스크를 만들 때 사용하며, 공개 입력에서 계산하므로 별도 공개 입력으로 추가하지 않았습니다.

**Proof에 공개 입력으로 전달하는 것과 Contract storage에 복사하는 것은 별개입니다.** 공개 입력이 15개라는 사실만으로 15개 모두를 별도 저장해야 하는 것은 아닙니다.

위원회 공개키는 **Circuit 상수**입니다. Participant가 witness로 다른 공개키를 선택할 수 없습니다. 부모 cm과 출력 소비 nf는 평문으로 공개하지 않습니다.

감사 승인은 transaction 한 묶음입니다. 한 키를 복원하면 그 묶음의 다섯 값을 함께 읽습니다.

## 4. 어떤 상태를 저장하나요?

**현재 구현은 검증한 공개값 15개를 storage에 기록합니다. 원본을 쉽게 조회하기 위한 실험용 선택이며, 암호화에 필수인 저장량은 아닙니다.**

### 왜 암호화된 값 5개만으로는 부족한가요?

암호화된 값만 있어서는 위원이 복호화에 필요한 계산을 시작할 수 없습니다. 공개점 $R_1$도 필요합니다.

| 복원에 사용하는 정보 | 역할 | 현재 표현 |
|---|---|---|
| $C_0,\ldots,C_4$ | 부모 cm 3개·출력 nf 2개를 숨긴 내용입니다. | 5개 Field입니다. |
| 공개점 $R_1$ | 위원이 자기 share로 partial decryption을 계산하는 데 사용합니다. | 압축하지 않은 좌표 2개입니다. |
| 문맥 $L$ | 암호화 때와 같은 키·마스크를 재현합니다. | 기존 Process 공개 입력에서 재계산합니다. |

따라서 **현재 형식의 암호문 자체는 7개 Field·224 B**입니다. 이는 메시지 부분 5개와 공개점 좌표 2개의 합이며, 전체 AuditRecord의 최소 크기를 확정한 값은 아닙니다.

### 그러면 왜 15개를 저장했나요?

**기록 ID 하나만 조회해 문맥과 암호문을 모두 얻도록, 기존 Process 공개값 8개까지 복사했기 때문입니다.**

| 현재 storage에 넣은 내용 | 개수 | 이유 |
|---|---:|---|
| PolicyRef·ScopeRef·noteRoot·입력 nf 3개·출력 cm 2개 | 8개 | 조회한 기록만으로 $L$을 재계산합니다. |
| $R_1$ 좌표와 암호화된 값 | 7개 | 두 위원의 응답으로 감사 평문을 복원합니다. |
| 합계 | **15개·480 B** | 현재 실험용 기록의 논리 데이터 크기입니다. |

감사 프로그램이 **해당 transaction의 검증된 공개 입력을 확실하게 조회할 수 있다면**, 그 원본으로 $L$을 재계산할 수 있습니다. 그러면 기존 공개값 8개를 AuditRecord storage에 중복 저장하지 않아도 됩니다. 다만 기록 ID와 원래 transaction의 대응, 원본의 일치·조회 가능성은 확보해야 합니다.

**이 중복 저장을 줄인 방식은 아직 구현·측정하지 않았습니다.** 현재 코드는 계속 15개를 저장하며, 기존 gas를 7개만 저장한 비용으로 해석하거나 단순 비례로 환산하지 않습니다.

### Contract는 어떤 순서로 기록하나요?

1. Process와 암호화 관계를 함께 증명한 proof를 검증합니다.
2. 성공하면 검증한 공개값 15개를 신규 ID의 storage에 저장합니다.
3. 기록 ID를 알리는 Event를 발생시킵니다.

**단순히 Event 로그만 남기는 것이 아닙니다.** 저장된 원본 배열은 getRecord로 읽습니다. 부모 cm과 출력 소비 nf는 암호문 안에 있으며 평문으로 저장하지 않습니다.

| 상태 | 역할 | 변경 시점 |
|---|---|---|
| 고정 Process verifier | 감사 Process proof를 검증합니다. | 배포 시 고정합니다. |
| nextRecordId | 다음 신규 기록 ID입니다. | 정상 저장 후 1 증가합니다. |
| records[id] | 검증한 공개 입력 15개의 원본입니다. | verifyAndStore에서 신규 ID에만 기록합니다. |

verifyOnly는 proof만 검증하고 아무것도 저장하지 않습니다. verifyAndStore는 위 세 단계를 수행합니다. 두 함수의 gas 차이를 측정했으며, 어느 쪽도 실제 Note를 소비하거나 Tree를 갱신하지 않습니다.

Canonical 실행의 최종 상태는 **기록 ID 1개, nextRecordId=2**입니다. 저장값·복원값이 fixture와 일치했습니다. 같은 유효 proof를 다른 ID에 다시 기록할 수 있는 것은 의도한 진단 범위이며, 실제 공급망의 중복 소비를 허용한다는 뜻이 아닙니다.

## 5. 계획과 실제 구현이 달랐나요?

**Protocol·암호화 관계의 의미 변경은 없습니다.** 실행과 측정에서 다음 차이를 기록했습니다.

| 항목 | 실제 처리 | 영향 |
|---|---|---|
| 저장 범위 해석 | 명세대로 공개 입력 15개를 저장했습니다. | 조회 편의를 위한 중복 저장이며, 최소 AuditRecord 저장 구조를 구현·측정한 것은 아닙니다. |
| Gas 첫 시도 | transaction·원본 복원 후 SSTORE trace가 비어 있어 실패했습니다. | 첫 시도의 숫자를 공식 성공값으로 사용하지 않았습니다. |
| Gas 재시도 | B1 전용 Compose에 Anvil steps-tracing을 추가하고 clean chain에서 다시 실행했습니다. | Gas는 2회 시도·1회 성공입니다. Setup·proof·test는 반복하지 않았습니다. |
| CPU 식별 | Go 측정의 sandbox에서는 arm64만 기록됐고, gas 실행 환경에서 Apple M1 Pro를 확인했습니다. | 기존 측정 JSON을 덮어쓰지 않았습니다. |
| 메모리 | Prove 할당량·전후 heap, 복호화 workload 할당량을 기록했습니다. | Native Encrypt 단독 할당량은 수집하지 않았으며 peak RSS로 주장하지 않습니다. |

테스트 후 변경한 코드는 B1의 추적 설정과 결과 checksum 수집뿐입니다. 암호화·Circuit·Contract 의미는 변경하지 않았습니다.

## 6. 어떤 correctness를 확인했나요?

| 범위 | 실제 결과 |
|---|---|
| Native 복원 | 1-Field·5-Field, 위원 쌍 {1,2}·{1,3}·{2,3}, 순서 변경 모두 동일 원문입니다. |
| Field·encoding | 0·최댓값·wraparound 복원, 잘못된 길이·범위·점·subgroup·scalar를 확인했습니다. |
| 응답 | 부족한 응답·중복 ID·허용되지 않은 ID를 거부했습니다. |
| 암호화 관계 | 잘못된 난수·공개점·암호문·문맥·위원회 공개키를 거부했습니다. |
| Process 연결 | 잘못된 부모·출력 nf·배열 순서로 만든 정상 암호문을 거부했습니다. 기존 owner·path·State·Role·Policy 검사도 유지했습니다. |
| Contract | invalid proof·공개 Field 범위·없는 ID를 거부하고, 실패 후 기록·ID가 불변임을 확인했습니다. |
| 전체 회귀 | cache 없이 전체 Go suite 1회 통과, 기존 18개를 포함한 Foundry 25개 test 1회 통과입니다. |
| 원본 복원 | 두 위원이 같은 온체인 기록을 각각 읽고 복원한 다섯 값이 fixture와 일치했습니다. |

형식만 맞는 악의적인 위원 응답이나 임의 암호문의 인증 실패를 증명했다고 주장하지 않습니다. 한 응답을 거부하는 API 테스트 자체도 threshold 보안 증명은 아닙니다.

## 7. 무엇을 어디까지 측정했나요?

| 관점 | 포함 | 제외 |
|---|---|---|
| Participant | Native 암호화, compile·SRS·Setup, witness·Prove·Verify, Prove 할당량·heap, proof·key 크기입니다. | 실제 사용자 네트워크·Main Ledger transaction입니다. |
| 온체인 | verifier·기록 Contract 배포, 고정 proof 검증·저장 receipt gas, calldata, SSTORE trace입니다. | Note Tree·PolicyGrant·Status·AuditRecord 전체 원장입니다. |
| 위원회·Auditor | 순차 partial decryption 2개, 결합·키·마스크·원문 복원, 원문 대조입니다. | Setup·암호문 생성·파일 로딩·위원 네트워크·그래프 탐색입니다. |

Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, macOS arm64, GOMAXPROCS=8입니다. Proof는 BLS12-381 PLONK-KZG, 암호화는 Jubjub·기존 Poseidon2입니다.

온체인은 Foundry·Anvil 1.7.1, Prague, chain ID 31337, 30M block gas limit입니다. 별도 zkdpp-m6-b1 프로젝트만 생성·정리했으며 다른 컨테이너는 중단하지 않았습니다.

## 8. 실제 성능은 어느 정도인가요?

### Participant는 얼마나 계산하나요?

**암호문 자체를 만드는 데 약 0.66 ms, Process와 올바른 암호화를 함께 증명하는 데 약 1.75초가 걸렸습니다.** 암호화 시간과 proof 생성 시간은 서로 다른 작업의 비용입니다.

| Circuit | Constraints | Public 입력 | Prove ms | Verify ms |
|---|---:|---:|---:|---:|
| 독립 5-Field 암호화 | 14,535 | 8 | 613.959 | 3.341 |
| 감사 Process 3→2 | 93,578 | 15 | 1,750.162 | 2.748 |

Prove는 Participant가 proof를 만드는 시간이고 Verify는 Go 프로그램이 proof를 확인하는 시간입니다. Constraints는 Circuit이 강제하는 계산 관계의 수이며 gas나 실행 시간 자체는 아닙니다.

독립 코어에는 Merkle path가 없습니다. 감사 Process에는 기존 depth-32 membership path 3개가 포함되며 Status path는 없습니다.

기존 [M5 Raw](../output/m5-circuit.json)의 69,122 constraints보다 24,456개 증가했습니다. 증가분에는 암호화뿐 아니라 부모 재계산·출력 nf·문맥 계산도 포함됩니다. 서로 다른 실행 시점의 Prove 시간을 안정적인 성능 비율로 비교하지 않습니다.

| 준비 구간 | 독립 코어 ms | 감사 Process ms |
|---|---:|---:|
| Compile | 13 | 40 |
| 개발 SRS 생성 | 454 | 3,023 |
| PLONK Setup | 98 | 554 |
| Witness 생성 | 0.080 | 0.172 |

위원회 키 생성은 0.320 ms입니다. Native 암호화는 0.661 ms이며 **같은 암호문을 두 Circuit에서 사용한 한 번의 측정값**입니다.

| Prove 메모리 | 독립 코어 B | 감사 Process B |
|---|---:|---:|
| 누적 할당량 | 32,262,232 | 592,563,608 |
| 시작 heap | 4,495,928 | 30,728,400 |
| 종료 heap | 23,827,320 | 174,850,488 |

누적 할당량은 실행 중 새로 할당한 총량이고 heap은 두 시점의 snapshot입니다. peak RSS나 최대 동시 사용 메모리는 아닙니다.

두 proof 모두 binary 664 B, Solidity 형식 1,056 B입니다. 독립 코어의 Solidity 크기는 직렬화 크기만 확인했으며 전용 verifier Contract는 배포하지 않았습니다.

### 온체인 검증·저장은 얼마나 드나요?

**736,120 gas는 proof 검증과 15개 값의 storage 기록을 합친 비용입니다. 최소 AuditRecord 저장 비용이나 실제 Main Ledger의 Process 전체 비용이 아닙니다.**

| Transaction | Receipt gas | Calldata·배포 데이터 B |
|---|---:|---:|
| 감사 Process verifier 배포 | 1,602,696 | 7,193 |
| AuditEncryptionStore 배포 | 347,845 | 1,446 |
| proof만 검증 — verifyOnly | 397,249 | 1,604 |
| 검증 후 15개 값 저장 — verifyAndStore | 736,120 | 1,604 |

검증·저장 경로의 추가 비용은 **338,871 gas**입니다. 이 전체를 순수 SSTORE라고 부르지 않습니다.

- 실제 trace의 SSTORE는 **16회·334,400 gas**입니다. 공개 Field 15개 저장과 기록 ID 갱신입니다.
- 나머지 **4,471 gas 차이**에는 조회·배열 저장 제어·Event·반환 등 경로 차이가 포함됩니다.
- 암호문 자체는 7 Field·224 B입니다. 진단 기록은 문맥 원자료까지 포함한 15 Field·480 B입니다. Storage metadata와 calldata는 별도입니다.

**공개값 8개의 중복 저장을 생략한 경로의 gas는 미측정입니다.** 저장량을 줄일 수 있다는 설계상의 설명과 현재 구현의 실측 결과를 구분합니다.

Raw의 prepare·submit-to-receipt 시간은 고정 proof transaction 구간입니다. 새 proof 생성부터 시작한 감사 E2E로 해석하지 않습니다.

### 위원 두 명은 얼마나 빨리 복원하나요?

**암호문 1,000개를 로컬에서 순차 복원하는 데 약 683 ms가 걸렸으며, 모든 원문이 일치했습니다.** 다섯 값마다 위원 응답을 받는 것이 아니라, 암호문 한 묶음마다 두 응답을 받습니다.

| 서로 다른 암호문 수 | 위원 1 ms | 위원 2 ms | 결합·복원 ms | 전체 ms |
|---:|---:|---:|---:|---:|
| 1 | 0.237 | 0.230 | 0.787 | 1.254 |
| 10 | 1.928 | 1.887 | 6.427 | 10.244 |
| 100 | 12.811 | 12.518 | 42.718 | 68.070 |
| 1,000 | 128.734 | 125.937 | 428.566 | 683.415 |

각 암호문은 새 난수·공개점으로 생성했고 모두 같은 5-Field 원문을 정확히 복원했습니다. 암호문당 응답은 두 개이며, Field마다 응답을 요구하지 않습니다.

각 행은 workload 한 번입니다. 1,000개 행을 반복 실험의 평균으로 표현하지 않습니다. 전체에는 원문 대조와 반복문 비용이 포함되고 각 단계에는 입력 검증이 포함됩니다. **실제 그래프 감사나 위원 네트워크 성능은 아닙니다.**

## 9. 어떻게 재현하나요?

Go 1.25.7과 Docker, 기존 M1~M6 generated verifier·fixture를 갖춘 별도 실험 사본에서 다음 순서입니다. 전체 Foundry 회귀는 과거 생성물도 필요하므로, 소스만 받은 새 checkout에서는 과거 milestone의 Setup·fixture 준비가 먼저 필요합니다.

1. make setup-m6-b1 — 위원회 설정과 두 Circuit의 개발 keys를 생성합니다.
2. make evaluate-m6-b1 — 두 proof를 각각 한 번 생성·검증하고 고정 Solidity fixture를 만듭니다.
3. GOFLAGS=-count=1 GOMAXPROCS=8 make test-go — 전체 Go correctness입니다.
4. make test-contract-m6-b1 — 기존 회귀를 포함한 Foundry suite입니다.
5. make benchmark-m6-b1-gas — 독립 Anvil에서 고정 proof의 검증·저장과 원본 복원을 확인합니다.
6. make benchmark-m6-b1-decrypt — 네 복호화 workload를 순차 실행합니다.
7. make check-m6-b1 — 보존 기준·생성물 checksum을 확인합니다. 이번 실행의 baseline.json은 개발 Artifact에 있으며, 새 실험에서는 실행 전 파일 checksum 기준을 별도로 준비해야 합니다.

make benchmark-m6-b1은 gas·decrypt를 묶은 대안이며 개별 명령과 중복 실행하지 않습니다.

공식 Raw가 이미 있으면 중복 측정을 거부합니다. 새 실험은 기존 Raw와 Artifact를 보존한 별도 실험 사본에서 수행해야 합니다. 기존 위원회 설정을 발견하면 checksum·shares를 확인해 재사용하며, 불완전한 설정을 덮어쓰지 않습니다.

## 10. Artifact와 checksum은 어디에 있나요?

개발 생성물은 artifacts/development/m6-b1 아래에 분리했습니다.

| 종류 | 독립 코어 B | 감사 Process B |
|---|---:|---:|
| CCS | 197,389 | 1,429,295 |
| PK | 1,622,160 | 12,632,208 |
| VK | 49,144 | 49,144 |
| Canonical SRS | 835,204 | 6,340,228 |
| Lagrange SRS | 835,060 | 6,340,084 |

공개 설정은 384 B, 위원별 private 파일은 각각 187 B입니다. 위원회 폴더는 0700, share 파일은 0600이며 Git에서 제외됩니다. master secret·공유 계수·대칭키는 저장하지 않습니다.

기존 코드·원본 문서·M1~M6 생성물 **364개 파일의 SHA-256이 보존 기준과 일치**했습니다. 두 Circuit·고정 fixture·세 측정 JSON의 위원회 공개키 식별정보도 같습니다.

네 Raw JSON은 output 폴더에 보존합니다. 재생성 가능한 keys·proof·generated verifier·fixture·테스트 로그는 Git 대상이 아닙니다. [checksum JSON](../output/m6-b1-generated-checksums.json)에 보호 대상·신규 생성물·실패를 포함한 시도 이력을 기록했습니다.

전체 test 로그와 correctness.json은 개발 Artifact 폴더에 있습니다. check 명령은 그 파일을 다시 Hash할 뿐 correctness suite나 benchmark를 재실행하지 않습니다.

## 11. 어떤 한계를 남겼나요?

- 신뢰 Setup·정직한 위원회·Auditor·검증된 원본만 처리한다는 전제를 사용합니다.
- 이 구성은 원 TDH2 그대로가 아니며 일반적인 CCA 보안이나 악의적 위원 응답 검증을 주장하지 않습니다.
- Go 메모리의 완전한 secret 삭제·상수 시간 실행·Production key 보관은 보장하지 않습니다.
- Circuit은 난수 범위·관계를 확인하지만 Participant가 정말 무작위로 생성했는지는 증명하지 않습니다.
- 위원회 공개키가 Circuit에 고정되어 있으므로 키 변경에는 새 Circuit Setup이 필요합니다.
- 개발용 unsafe SRS이며 최종 universal SRS가 아닙니다.
- 1-Field 재사용·5-Field Process 연결을 검증한 것이며, 모든 Event 적용 완료는 아닙니다.

해결되지 않은 correctness 실패는 없습니다. Gas 첫 시도의 추적 실패와 Native Encrypt 단독 메모리 미수집은 앞 절에 명시했습니다.

## 12. 다음 단계는 무엇을 이어받나요?

**“올바른 감사 정보를 숨겨 두고, 위원회 협조로 다시 읽는 부품”이 준비된 상태입니다.**

복원한 nf로 소비 기록을 찾고 → 다음 cm으로 이동하고 → 미소비 leaf를 찾아 Freeze하는 연결은 아직 없습니다. 다음 단계가 이어받을 것은 암호화·복호화 함수와 실제 Event의 올바른 정보를 암호화하도록 강제하는 Circuit 기능입니다.

후속 명세에서는 모든 Event와 Voucher를 연결하고, AuditRecord·producerOf·소비 mapping·snapshot 탐색·nullifier 기반 동결을 다룹니다. 기존 공개 입력을 중복 저장할지, 검증된 transaction 원본에서 읽을지도 그 통합 단계에서 정할 사항입니다. 이 기능들은 이번에 구현하지 않았으며 기존 M6 Main Contract도 교체하지 않았습니다.
