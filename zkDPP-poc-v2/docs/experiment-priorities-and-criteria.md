# zkDPP 실험 우선순위와 측정 기준

**공급망 동작을 완료하는 비용, 추가 기능의 대가, Provenance를 복원하고 미소비 객체를 Freeze하는 비용을 순서대로 확인합니다.**

이 문서는 **앞으로 수행할 실험 계획**입니다. 구현·측정이 완료됐다는 뜻은 아닙니다. 기존 zkDPP-poc-v2 M1-HF와 v1의 source·Artifact·Raw·Result는 보존합니다.

- **대표 동작은 Split**입니다. 부모 cm 하나와 출력 nf 두 개를 암호화하므로 감사 암호문은 3-Field입니다.
- **기본 성능과 Audit 추가 비용은 9개 Event 모두 측정**합니다. Issue는 Standard를 사용합니다.
- **Process 기본값은 2→1**입니다. 2→2·3→2·3→3·4→3으로 확장하며, 같은 공정 총량을 여러 Note로 나누는 시나리오를 사용합니다.

## 우선순위 — 본문도 이 순서로 읽습니다

| 순위 | 실험 | 확인할 질문 |
|---|---|---|
| **1** | **Event 기본 성능·E2E와 DPPClaim 검증** | 공급망 동작과 Claim 발급·확인을 완료하는 데 얼마가 드는가? |
| **2** | **직접 Transfer와 Voucher 전달 비교** | Recall 가능한 중간 Layer를 도입하면 전달 비용이 얼마나 증가하는가? |
| **3** | **모든 Event의 Audit 추가 비용** | 감사 가능한 연결을 남기기 위해 얼마를 더 부담하는가? |
| **4** | **Tracing·Freeze 확장성** | 관련 기록과 미소비 대상이 늘어도 정확하게 복원·동결할 수 있는가? |
| **5** | **State·검증 관계 확장** | 검증할 State 관계가 많아지면 비용이 어떻게 증가하는가? |
| **6** | **Process 입출력 확장** | 같은 공정을 더 많은 Note로 처리하면 비용이 어떻게 증가하는가? |
| **7** | **감사 암호문 복호화** | 암호문 수와 암호문당 Field 수가 복호화 비용에 각각 어떤 영향을 주는가? |
| **8** | **Master Key 복구** | 필요한 share 수와 전체 Committee 수가 복구 비용에 각각 어떤 영향을 주는가? |

**1~4번이 논문의 핵심 기능·비용 평가입니다.** 5~6번은 확장성을, 7~8번은 비용의 원인을 보완합니다. 실제 작업에서는 2·3번의 Audit 없는 경로를 함께 준비하고, 4번 분석에 필요한 7번을 앞당길 수 있습니다.

### 어떤 관점에서 실험을 찾나요?

| 평가 관점 | 주로 확인하는 것 | 해당 실험 |
|---|---|---|
| Supply Chain | State 전이와 제품의 Provenance가 이어지는가? | 1·4·5·6 |
| Token·상태 전이 | 새로운 객체와 전이가 어떤 기능을 추가하는가? | 2·6 |
| Blockchain 저장·실행 | transaction·저장·조회 비용이 얼마나 늘어나는가? | 1·2·3·4 |
| DPP 사용자 | Claim을 발급하고 확인하는 비용은 얼마인가? | 1 |
| Privacy·권한 | 누가 연결을 알 수 있고, 누가 소비할 수 있는가? | 2의 설계 배경·보안 분석 |
| 준비·반복 사용 | 한 번 준비하는 비용과 매번 사용하는 비용은 무엇인가? | 1의 Setup·Policy 준비, 각 실행 실험 |
| 실패·안전성 | 금지된 동작을 거부하고 실패를 정확히 보고하는가? | 모든 실험의 성공 기준 |

**Privacy 보장은 시간 측정으로 입증하지 않습니다.** 공개 정보·비밀 정보·권한에 대한 보안 분석과 비용 실험을 구분합니다.

## 공통 측정 기준

### Event E2E는 한 실행을 처음부터 끝까지 잽니다

**필요한 입력 Note·path·Policy를 확보한 상태에서 시작하여, witness 준비 → Prove → transaction 구성·제출 → 성공 receipt 확인까지 측정합니다.**

| 항목 | 측정 범위 | Event E2E |
|---|---|---|
| Setup | Circuit별 PK·VK 생성; compile·공통 SRS 생성은 별도 기록 | 제외 |
| witness 준비 | Native 계산·감사 암호화·witness 구성 | 포함 |
| Prove | 준비된 witness로 Proof 생성 | 포함 |
| 로컬 PLONK Verify | 생성한 Proof를 오프체인에서 검증하는 보조 측정 | 제외 |
| transaction | Proof 직렬화·인코딩·서명·제출·성공 receipt 확인 | 포함 |
| 온체인 Verify | Contract의 Proof 검증 | transaction 시간·gas에 포함 |
| 상세 사후 검사 | 예상 상태 대조·opcode trace 수집·Raw 저장 | 제외 |

- **이 실행에서 만든 Proof를 해당 transaction에 사용합니다.** 따로 측정한 Prove와 미리 만든 Proof의 제출 시간을 합산해 실측 E2E라고 부르지 않습니다.
- 전체 타이머와 단계별 타이머를 함께 둡니다. 중첩된 타이머를 중복 합산하지 않습니다.
- SRS·PK/VK 로딩, 배포, 선행 원장 구성은 측정 전에 끝냅니다.
- Anvil receipt 확인을 Production finality로 해석하지 않습니다.
- **DPPClaim 검증은 로컬 PLONK Verify가 아닙니다.** 별도 입력과 타이머는 실험 1에서 정합니다.

### 비교 조건과 결과 기록을 맞춥니다

- 같은 비교 안에서는 하드웨어, Go·gnark·Foundry·Solidity 버전, GOMAXPROCS, Proof system·Curve·Hash, SRS 정책, RPC 위치를 고정합니다.
- Anvil은 Prague·chain ID 31337·30M block gas limit, Go는 GOMAXPROCS=8을 기본으로 합니다. 새 실험의 실제 버전과 포트·Compose project를 실행 설정에 남깁니다.
- 동일 case의 반복은 같은 선행 원장 상태에서 시작합니다. 소비된 Note를 그대로 재사용하지 않고 clean chain 또는 검증된 상태 복원을 사용합니다.
- 입력 수와 출력 수, State 수와 검증 조건 수를 구분합니다. 함께 달라지는 연산·공개 입력·저장량도 기록합니다.
- 공통 SRS 비용은 한 번만 기록하고 Circuit별 Setup과 구분합니다. 확장 Circuit이 더 큰 domain을 요구하면 그 경계를 명시합니다.
- gas는 실제 receipt에서 읽습니다. SSTORE opcode 비용은 별도 trace에서 구하며, 총 gas 차이를 순수 저장 비용이라고 부르지 않습니다.
- Proof 저장 형식의 bytes와 transaction calldata bytes를 구분합니다. 메모리는 누적 allocation과 peak RSS를 혼용하지 않습니다.
- Master Key·secret share·private witness는 공개 Raw에 기록하지 않습니다.

### 반복 횟수와 성공 조건을 고정합니다

**기록 수는 workload 크기이고, 같은 workload를 다시 실행한 수는 반복 횟수입니다.**

최종 workload 범위, warm-up·반복 횟수, 대표 통계는 첫 공식 실행 전에 하나의 실행 설정으로 고정합니다. 본문의 **초기 범위**는 workload 제안이며, 기존 M1-HF의 case별 1회 측정을 새 실험의 반복 결과로 대체하지 않습니다.

모든 성공값·실패·재시도 이유를 보존합니다. 반복 결과가 있으면 원자료와 분산을 함께 제시하고, 단일 실행이면 평균·안정적인 증가율을 주장하지 않습니다. gas 차이는 calldata와 선행 상태를 확인한 뒤 해석합니다.

**성능값의 전제는 correctness 통과입니다.** 성공 receipt뿐 아니라 예상 State·생성·소비·감사 결과를 대조합니다. 잘못된 결과를 만든 빠른 실행은 성능 성공값으로 채택하지 않습니다.

## 1. Event 기본 성능·E2E와 DPPClaim 검증

**목적:** 공급망 Event를 실행하고 Claim을 발급·확인하는 기본 비용을 확보합니다.

### Event별 실행

대상은 **Entry·Transfer·Proceed·Recall·Merge·Split·Process·Exit·Issue**입니다. 각 Event의 기본 조건은 다음과 같습니다.

| 항목 | 조건 |
|---|---|
| Audit | 현재 Protocol처럼 포함 |
| Process | 실험 6의 2→1 시나리오 |
| Issue | Standard Policy |
| 선행 데이터 | 필요한 Note·Voucher·path·권한·Policy를 미리 준비 |
| 측정 | witness·Prove·transaction·E2E, constraints·public input 수, gas·calldata·Proof 크기 |
| 보조 측정 | 로컬 PLONK Verify 시간·메모리 |
| 성공 기준 | 실제 verifier 통과, 예상 Tree·소비·생성·AuditRecord·Claim 등록 결과 일치 |

**결과는 Event별로 제시합니다.** 여러 Event의 평균만으로 특정 Event의 비용을 가리지 않습니다. Tree append 위치·입력 규모도 함께 남깁니다.

### DPPClaim 검증

**Issue가 발급을 끝낸 뒤, DPPClaim 검증 프로그램의 실행 시간을 따로 측정합니다.** Issue E2E에는 합산하지 않습니다.

- **입력:** ProductName·LotID·Unit, DPPClaim의 issuePolicyRef·h·claimNonce, 요구하는 PolicyRef, 대상 chain·Ledger
- **시작:** 입력을 갖춘 검증 프로그램 호출
- **동작:** Snapshot 확보 → DocumentHash·h 재계산 → Claim 등록·producer·Issue 원본·Policy 대조 → Snapshot 일관성 확인
- **끝:** 유효·무효 또는 조회 오류 반환

기본 Policy는 Standard입니다. 실제 Anvil RPC를 사용하며, 매 실행은 빈 application cache에서 시작합니다. 노드 내부 cache까지 비웠다고 가정하지 않습니다.

| 측정값 | 의미 |
|---|---|
| **검증 총시간** | 프로그램 호출부터 최종 판정까지의 wall-clock |
| 로컬 계산 시간 | 문서 encoding·Hash·값 비교·응답 검증; RPC 대기 제외 |
| RPC 시간·호출 수 | Snapshot과 온체인 원본을 조회하는 실제 요청·응답 |
| 결과 | 수락·거부·조회 오류와 이유 |

**성공 기준:** 올바른 Claim을 수락하고, 잘못된 DPP·h·요구 Policy·Issue 원본을 거부합니다. 이는 등록된 Claim과 제시된 DPP의 연결 확인이며, Issue Proof를 다시 만드는 작업이 아닙니다. transaction 제출·Master Key 복구·감사 복호화는 필요하지 않습니다.

### Setup·새 Policy 준비

Circuit compile, 공통 SRS 생성, Circuit별 PK·VK Setup, verifier 배포, Policy 등록을 **별도 준비 비용 표**로 정리합니다. Process Grant 설정도 분리합니다.

준비 비용을 Event마다 더하지 않습니다. 기존 결과를 인용하면 source·Artifact·측정 조건의 일치를 확인하고, 변경된 Circuit의 Setup으로 표시하지 않습니다.

## 2. 직접 Transfer와 Voucher 전달 비교

**목적:** 중간 Layer를 도입해 수령·회수를 선택할 수 있게 만드는 실행 비용을 확인합니다.

동일 환경에 **Receiver Note를 직접 만드는 Transfer 비교군**을 구현합니다. Azeroth의 직접 전달 구조를 참고하되, 외부 논문의 수치를 가져와 비교하지 않습니다.

| 경로 | 완료 결과 | Proof·transaction의 기본 수 |
|---|---|---|
| 직접 Transfer | Receiver Note + Sender의 Change Note | 각 1개 |
| Transfer → Proceed | Receiver Note + Sender의 Change Note | 각 2개 |
| Transfer → Recall | 반환 Note + Sender의 Change Note | 각 2개 |

직접 전달의 출력 Note도 **owner-secret 기반 nf**를 유지합니다. opening 기반 직접 Tracing은 이 비교군에 추가하지 않습니다.

**고정 조건:** 입력 State·전달량·운송 탄소·Change 규칙·owner 검증·Note membership·소비 방지·Status와 실행 환경입니다. 전량 Transfer의 zero-State Change 규칙도 양쪽에서 맞춥니다. 새 commitment의 값까지 같을 필요는 없지만 최종 owner·State·DocumentHash는 의도한 결과와 같아야 합니다.

**두 방식 모두 Audit 없는 조건에서 먼저 비교합니다.** 필요한 비교군은 실험 3과 함께 준비합니다. Voucher Tree·추가 Proof·transaction은 중간 Layer의 비용이므로 그대로 셉니다.

### 경로 E2E

최초 입력 Note·path는 미리 준비합니다. Sender의 witness 준비에서 시작해 **최종 Receiver 또는 반환 Note의 receipt**에서 끝냅니다.

Transfer 뒤에 생기는 **Voucher 정보의 로컬 전달과 최신 membership path 확보는 경로 E2E에 포함**하고 따로 기록합니다. 같은 장비의 역할별 프로그램으로 즉시 이어 실행하며 사람의 대기나 외부 네트워크 지연을 포함했다고 주장하지 않습니다.

**측정값:** 경로 전체·단계별 시간, Proof·transaction 수, gas·calldata 합계, Tree append·저장량입니다. 별도로 잰 두 Event 시간을 합산한 값과 실제 경로 E2E를 구분합니다.

**성공 기준:** 정상 수령·회수 후 owner와 State가 맞고, Proceed·Recall 중 하나가 성공하면 다른 쪽은 거부되어야 합니다. 잘못된 owner secret·deadline·Frozen 상태도 검사합니다. Recall이 없는 직접 전달과 Recall 경로는 동일 기능 비교가 아니라 **추가 기능의 비용**으로 설명합니다.

### 논문에 남길 설계 이유 — 실험 추가가 아닙니다

| 중간 Layer의 목적 | 설명 |
|---|---|
| **Recall 지원** | Receiver의 Proceed 전에 Sender가 허용된 deadline 안에서 회수합니다. 두 경로는 같은 rvnf를 소비합니다. |
| **Sender의 후속 소비 추적을 막으면서 Audit 허용** | Voucher에서 Receiver만 미래 nf를 계산할 수 있는 Note로 전환합니다. Auditor는 Proceed의 감사 암호문에서 그 nf를 복원합니다. |

Sender는 rvnf를 알기 때문에 **Proceed와 그때 생성된 cm까지는 연결**할 수 있습니다. 숨기는 대상은 **그 Receiver Note의 다음 소비**입니다. 이 경계는 비밀키와 암호화 관계에 대한 보안 분석으로 설명하며 시간 측정으로 증명하지 않습니다.

opening만 아는 제3자는 소비할 수 없습니다. Proceed는 Receiver secret, Recall은 Sender secret과 deadline 조건을 요구합니다. **nf를 아는 것과 소비 권한을 가진 것은 다릅니다.**

opening 기반 직접 전달을 생각했던 논의는 이 Privacy 차이를 이해하기 위한 배경으로만 둡니다. 새로운 nf 관계의 구현·비교 실험은 추가하지 않습니다.

## 3. 모든 Event의 Audit 추가 비용

**목적:** 감사 가능한 Provenance를 남기기 위해 Participant와 Contract가 추가로 부담하는 비용을 확인합니다.

**9개 Event 각각에서 Audit 없는 경우와 있는 경우를 비교합니다.** Split로 측정 경로를 먼저 확인한 뒤 전체 Event에 적용합니다.

| Event | 암호화할 값 | 감사 Field 수 |
|---|---|---:|
| Entry | 출력 nf | 1 |
| Transfer | 부모 cm, Voucher rvnf, Change nf | 3 |
| Proceed | 부모 rv, Receiver Note nf | 2 |
| Recall | 부모 rv, 반환 Note nf | 2 |
| Merge | 부모 cm 2개, 출력 nf | 3 |
| **Split** | **부모 cm, 출력 nf 2개** | **3** |
| Process 2→1 | 부모 cm 2개, 출력 nf | 3 |
| Exit | 부모 cm | 1 |
| Issue Standard | 부모 cm | 1 |

각 행은 **공개점 하나와 Field 배열로 된 암호문 한 개**입니다. Issue의 Claim에는 미래 소비 nf가 없습니다.

### 비교군에서 유지할 기능

| 양쪽에서 유지 | Audit 없는 쪽에서 제거·대체 |
|---|---|
| State·owner·membership 관계, Tree append | 감사 암호화와 Circuit의 암호문 관계 검증 제거 |
| nullifier 중복 소비 방지·Status | 소비 기록 ID 대신 필요한 spent 상태 유지 |
| Entry 권한·Policy·Grant | AuditRecord와 Tracing 전용 producer·consumer 연결 제거 |
| Claim 등록·Policy 결합·중복 방지 | Claim 발급을 확인할 최소 근거 유지 |

Claim은 **등록 상태와 h·PolicyRef가 포함된 발급 Event 로그**를 남겨 기존 transaction과 대조할 수 있게 합니다. Audit 없는 비교군에서 원래 AuditRecord를 요구하는 조회 경로를 그대로 호출하지 않습니다. 이는 같은 저장 구조를 유지하는 비교가 아니라 **필수 기능은 유지하고 Audit을 제외하는 비교**입니다.

**고정 조건:** 같은 Event 입력·State·Policy·Tree 시작 상태·권한·환경입니다. 공개 입력과 저장 구조의 차이는 추가 기능의 비용으로 기록합니다.

**측정값:** Native 암호화, witness·Prove·E2E, constraints·public input·Proof 크기, gas·calldata·SSTORE입니다. Event별 두 값, 절대 차이와 증가율을 함께 제시합니다.

**성공 기준:** 양쪽의 공급망 결과와 필수 검사가 같아야 합니다. Audit 있는 쪽은 실제 부모·출력 nf와 다른 암호화, 암호문 변조를 거부해야 합니다.

Process 확장 구성도 실험 6에서 같은 방식으로 비교합니다. 같은 실행을 여러 결과표에서 인용할 수 있지만 독립 반복으로 세지 않습니다.

## 4. Tracing·Freeze 확장성

**목적:** 관련 공급망이 커질 때 cm·rv의 연결 복원과 미소비 대상 동결의 비용·정확성을 확인합니다.

대표 연결은 **cm_A → cm_B·cm_C, cm_B → cm_D·cm_E**입니다. 미소비 대상은 cm_C·cm_D·cm_E입니다. Split 기록은 3-Field, 시작 Entry 기록은 1-Field라는 차이를 유지합니다.

### 변수와 측정

| 하위 실험 | 변경할 변수 | 통제·기록할 내용 |
|---|---|---|
| 관련 기록 수 | 시작점에서 도달할 수 있는 고유 기록 수 | 동일한 그래프 생성 규칙·Event 구성 |
| 연결 구조 | 깊이·분기·Merge 합류·공유 조상 | 가능한 한 같은 기록 규모; 달라지는 기록·연결 수도 기록 |
| Freeze 대상 수 | 미소비 Note·Voucher 수 | 초기 Status·소비 여부, Tracing과 동결 비용 분리 |

Forward와 Backward는 각각 빈 application cache로 시작합니다. 같은 감사 실행에서는 같은 기록을 한 번만 복호화합니다. 실제 Proof로 만든 원장을 RPC로 조회하며 **원장 구성 비용은 감사 측정에서 제외**합니다.

| 측정 구간 | 시작 → 끝 |
|---|---|
| Tracing 단독 | Master Key·Snapshot 확보 후 → graph·frontier·terminal 반환 |
| Freeze 단독 | target 집합 확보 후 → 개별 동결과 결과 Snapshot 확인 |
| AuditAndFreeze 전체 | share가 준비된 상태에서 키 복구·Snapshot 생성 시작 → 최종 결과 반환 |

**기록할 값:** 방문 객체·고유 기록·연결·복호화·cache 재사용 수, RPC 수·시간, 탐색 시간, frontier·terminal 수, Freeze 시도·성공·실패 수, gas, 전체 시간입니다.

### 성공 기준

- 예상 graph와 부모·자손 연결이 일치합니다. Merge의 공유 조상은 중복으로 복호화하지 않습니다.
- **Claim·Exit는 terminal로 검사**하고 미래 nullifier를 조회하지 않습니다.
- 동결 완료 판정은 **미소비 Note·Voucher target에만** 적용합니다.
- 모든 target을 같은 결과 Snapshot에서 확인하며 Frozen·Revoked는 차단된 상태로 인정합니다.
- Snapshot 이후 소비되거나 transaction 결과가 불명확하면 INCOMPLETE입니다. Trace 실패 시 Freeze transaction은 0개입니다.
- 기존 코드의 chain·Ledger·원본·Snapshot 검사를 유지하며 새 자손 fallback은 실행하지 않습니다.

깊이와 기록 수가 함께 늘어난 결과를 깊이만의 영향이라고 부르지 않습니다. 전체 원장의 무관한 기록 수·동시 부하는 이번 범위에서 제외합니다.

## 5. State·검증 관계 확장

**목적:** State 수의 영향과 Sustainability 조건의 복잡도를 구분합니다.

### 실제 Claim 조건 비교

| 조건 | 필요한 State | 검증 관계 |
|---|---|---|
| 재활용률 기준 | 질량·재활용 질량 | 재활용 질량이 질량 대비 기준 이상 |
| 탄소집약도 기준 | 질량·탄소량 | 탄소량이 질량 대비 기준 이하 |
| 두 조건 모두 | 질량·재활용 질량·탄소량 | 위 두 관계를 함께 검증 |

이 하위 실험은 **현재 세 State를 유지하고 활성 검증 조건만 바꿉니다.** 따라서 결과 이름은 State 개수 비교가 아니라 **Claim 조건 비교**입니다. 기본 Issue의 Standard 측정과 구분하며 새로운 실험용 Policy로 기록합니다.

### 같은 관계를 적용한 State 개수 비교

**Merge 2→1을 고정**하고 합산 가능한 State마다 같은 uint64 범위와 보존 관계를 검증합니다.

$$
x_{\mathrm{out},i}=x_{\mathrm{in1},i}+x_{\mathrm{in2},i}
$$

- **변경:** 검증하는 State 개수. 초기 범위는 1·2·3·4·8·16개입니다.
- **고정:** Note 입출력, State당 범위·합산 관계, owner·membership, Audit 포함 조건
- **구현 범위:** 실험용 State 벡터와 commitment·Circuit. 검증하지 않는 dummy 값은 추가하지 않습니다.
- **측정:** constraints·domain, Proof 수·크기, witness·Prove·E2E·gas
- **성공 기준:** 모든 합산·범위·commitment 관계 확인, State 하나만 변조해도 거부

이 실험은 **같은 종류의 State 보존 검증을 늘리는 비용**입니다. 추가한 모든 차원이 실제 산업의 Sustainability 지표라는 주장은 하지 않습니다.

Sahai와 관계를 비교할 때도 “State 하나당 Proof 하나”라고 전제하지 않습니다. 같은 조건·범위에서 인증 경로 공유와 gadget별 Proof 구성을 대조합니다. zkDPP의 Proof 수가 하나여도 Prover 비용과 domain은 증가할 수 있습니다.

## 6. Process 입출력 확장

**목적:** 같은 총량과 같은 공정 규칙을 유지한 채, 소비·생성하는 Note 수의 영향을 확인합니다.

배분 규칙을 새 Protocol 논점으로 남기지 않고, **WASTE가 별도로 발생하지 않는 하나의 공정 시나리오**를 실험용으로 고정합니다. 기존 손실률·탄소 추가율과 floor·잔여분 배분 방식을 재사용합니다.

### 같은 공정 총량을 사용합니다

State 순서는 **(q_mass, a_rec, e)**입니다. 표의 단위는 kg·kg·kgCO2e이며, 실제 계산은 모두 기존 **10^9 scale의 정수**로 수행합니다.

| 공통 조건 | 값 |
|---|---|
| 입력 State 합계 | **(320, 30, 231)** |
| 질량 손실률 | **6.25%** — 손실 질량 20 |
| 추가 탄소집약도 | **0.09375 kgCO2e/kg** — 입력 질량 320에 대해 탄소 30 추가 |
| WASTE 생성률 | **0%** |
| 출력 State 합계 | **(300, 30, 261)** |
| 입력·출력 Role | 모두 **ELIGIBLE** |

질량 손실 20은 기존 공정의 손실 항목입니다. WASTE Note를 만들어 놓고 출력 수에서 숨기는 것이 아닙니다. 모든 구성에서 동일하게 WASTE 생성률을 0으로 둡니다.

입력 M개는 총 State를 균등하게 나눠 준비합니다. **입력 2~M에는 각 State의 scaled 정수 총량을 M으로 나눈 floor를 배분합니다. 입력 1에는 잔여분을 배분합니다.** 모든 입력은 질량이 양수이며 같은 owner·DocumentHash를 사용합니다. 서로 다른 opening으로 Note를 구분합니다.

### 출력 수만 바꿔 배분합니다

| 출력 수 N | 출력 Note의 State |
|---|---|
| **1** | **(300, 30, 261)** |
| **2** | (200, 20, 174), (100, 10, 87) |
| **3** | (150, 15, 130.5), (100, 10, 87), (50, 5, 43.5) |

출력 질량은 위 표로 고정합니다. 재활용 질량·탄소는 질량 비례로 배분합니다. **출력 2~N을 floor로 계산하고 출력 1에 잔여분**을 둡니다. N=1이면 공정 결과 전체가 유일한 출력입니다.

표의 State는 **측정용 witness 값**입니다. 값을 Circuit 상수로 넣어 계산을 생략하지 않고, 실제 입력에서 손실·탄소 추가·출력 배분·commitment 관계를 검증합니다.

이 시나리오의 모든 출력은 재활용률 10%, 탄소집약도 0.87로 **Standard Issue를 통과**합니다. Strict 통과를 주장하지 않습니다.

### 다섯 구성을 비교합니다

| 구성 | 입력 M | 출력 N | 감사 Field 수 | 직전 구성과의 차이 |
|---|---:|---:|---:|---|
| **2→1** | 2 | 1 | 3 | 기본값 |
| 2→2 | 2 | 2 | 4 | 출력 +1 |
| 3→2 | 3 | 2 | 5 | 입력 +1 |
| 3→3 | 3 | 3 | 6 | 출력 +1 |
| 4→3 | 4 | 3 | 7 | 입력 +1 |

**각 구성에서 Audit 없는 경우와 있는 경우를 측정**합니다. 부모 cm M개와 출력 Note의 미래 nf N개를 암호화하므로 감사 Field는 M+N개입니다. 구성에 따른 membership·배분·commitment·암호화·저장 연산 증가를 함께 기록합니다.

같은 공정 파라미터를 사용하되 arity가 다르므로 구성별 Circuit·PolicyRef·VK를 구분합니다. Native 계산·Circuit·Contract의 입력 배열·arity 검사·감사 배열을 새 구성에 맞춥니다. 가능한 공통 코드를 사용하고, 단순 입출력 수 증가를 별도의 산업 공정으로 해석하지 않습니다.

**측정값:** constraints·public input·domain, witness·Prove·E2E, Proof 크기, gas·calldata, 소비·append·감사 Field 수입니다.

**성공 기준:** 모든 구성에서 입력·출력 합계가 위 표와 일치해야 합니다. uint64 범위·overflow·양수 질량·owner·membership·Policy·암호문 관계를 검사합니다. 별도 correctness 입력에서는 나누어떨어지지 않는 배분의 floor·잔여분도 확인합니다.

**기존 결과와의 차이:** M1-HF의 3→2는 WASTE 생성률 10%로 ELIGIBLE (270,30,261)과 WASTE (30,0,0)을 만듭니다. 그 결과는 보존하고 WASTE 생성·Exit 규칙의 회귀 검사에 사용합니다. 새 실험의 3→2는 같은 arity라도 **WASTE 생성률 0%의 다른 Policy**이므로 과거 측정값을 재사용하지 않습니다.

## 7. 감사 암호문 복호화

**목적:** Circuit·RPC·탐색을 제외하고 암호화 코어의 복호화 비용을 분리합니다.

암호문 한 개는 다음 전체입니다.

$$
CT=(R_1,C_0,\ldots,C_{f-1})
$$

**f는 암호화된 Field 개수**입니다. 각 C는 독립 암호문이 아닙니다. AuditRecord의 encryptedParents와 encryptedOutputNfs는 이 배열의 앞·뒤 부분입니다.

기본값은 **Split에 해당하는 3-Field**입니다. 부모 cm 하나와 출력 nf 두 개를 암호화합니다. 같은 Field 수가 Transfer·Merge에도 해당합니다.

| 하위 실험 | 변경 변수 | 고정 조건 | 초기 범위 |
|---|---|---|---|
| 많은 기록 | 독립 암호문 수 | 암호문당 3 Field | 1·10·100·1,000개 |
| 큰 기록 | 암호문당 Field 수 | 암호문 1개의 복호화 | 1·2·3·5·10·20 Field |

- **사전 준비:** Master Key 복구, Field 원문·독립 암호문 생성, 파일 로딩
- **측정:** 입력·공개점 검사 → 공유점 계산 → 키·위치별 마스크 유도 → 모든 Field 복원
- **사후 검사:** 모든 Field의 원문 일치
- **측정값:** 총시간·암호문당 시간·처리량·allocation

암호문마다 새로운 암호화 난수를 사용합니다. 같은 암호문의 반복 복호화를 독립 기록 증가로 세지 않습니다. 짧은 작업은 반복 묶음으로 재되 묶음 크기와 반복 횟수를 따로 남깁니다.

**이 실험은 Circuit 변경 없이 수행할 수 있습니다.** 큰 Field 벡터를 복호화한 결과를 같은 크기의 Event Proof·온체인 실행 지원으로 해석하지 않습니다. 기존 5-Field 측정은 원문 대조까지 포함했으므로 새 복호화 전용 타이머와 직접 혼합하지 않습니다.

## 8. Master Key 복구

**목적:** 감사 시작 시 필요한 권한 복구의 계산 비용을 확인합니다.

**k는 필요한 share 수, n은 전체 Committee 수**입니다. 실제 복구에는 정확히 k개를 사용합니다.

| 하위 실험 | 고정 | 변경할 초기 범위 |
|---|---|---|
| 필요한 승인 수 | n=10 | k=2, 3, 5, 7, 10 |
| 전체 위원 수 | k=2 | n=3, 5, 10, 20 |

유효한 package·share 생성과 파일 로딩은 측정 전에 끝냅니다. 같은 Curve·키 길이·검사 정책과 로컬 순차 실행을 사용합니다.

**측정 단계:** package·share 검사 → 보간을 통한 Master Key 복구 → 공개키 일치 확인입니다. 각 단계와 전체 시간·allocation을 기록합니다.

n만 바꾸면 보간에 사용하는 share 수는 같습니다. 전체 package 검사 비용과 보간 비용을 구분해야 n의 영향을 설명할 수 있습니다.

**성공 기준:** 서로 다른 유효한 위원 조합에서 같은 키를 복구합니다. 부족한 share, 중복 ID, 다른 session, 변조 share를 거부합니다.

현재 2-of-3 전용 코어는 실험용 k-of-n 일반화가 필요합니다. **이 실험은 DKG·Committee 통신·승인 대기·Key Rotation을 측정하지 않습니다.** 복구한 키를 암호문마다 다시 복구하지 않습니다.

## 실행 전에 고정할 설정과 결과 해석

**Process의 총량·Role·배분 규칙은 실험 6에서 정했습니다.** 구현 시 해당 시나리오를 검사하는 Circuit·Policy·Contract 경로를 추가합니다.

실행 전에 다음 항목을 하나의 설정으로 고정합니다.

1. Trace graph의 크기·형태·Event 구성과 기대 frontier·terminal
2. 본문 초기 범위의 최종 workload 목록, warm-up·반복 횟수·대표 통계
3. 직접 Transfer·Audit 없는 경로에서 유지·제거한 검사와 저장 항목
4. source·Circuit·SRS·PK/VK·PolicyRef와 측정 환경 식별정보

이 문서는 **실험 계획과 측정 기준**입니다. 구현 API와 Raw schema까지 완료된 명세는 아닙니다. 새 구현·측정은 별도 경로에서 수행하며 자동으로 기존 결과를 덮어쓰지 않습니다.

| 현재 확인된 것 | 이번에 추가할 것 |
|---|---|
| 실제 9개 Event 실행, 개별 Prove·Verify·gas | 같은 실행의 연속 E2E, 직접 전달 비교 |
| 실제 RPC DPPClaim 검증 성공 | 독립 검증 시간·RPC 수 |
| 작은 연결 graph의 Tracing·Freeze | 크기·구조·대상 수 변화 |
| 고정 State·기존 Process 3→2 | State 관계 확장, 실험용 Process 다섯 구성 |
| 5-Field 복호화·2-of-3 복구 | 3-Field 기본과 Field 수 변화, k·n 분리 변화 |

선형성·병목·성능 향상은 결과를 얻은 뒤 판단합니다. PLONK domain 경계의 계단형 변화, RPC 비중, calldata·저장 상태 차이를 함께 봅니다.

**제외 범위:** 무관한 전체 원장 크기, 동시 부하·TPS, DKG·Key Rotation, post-snapshot 자손 fallback, opening 기반 직접 Tracing의 구현 실험입니다.

## 근거 문서와 코드

- [결과 보고서 작성 계획](experiment-result-report-plan.md): 같은 실험·수치·결론을 Markdown과 LaTeX로 작성·검토하는 기준
- [Protocol 구조](../ARCHITECTURE.md): 객체·Event·원장·Audit의 구분
- [M1-HF Result](../milestones/M1-HF-spec-conformance-result.md): 기존 검증·측정과 한계
- [M1-HF 명세](../milestones/M1-HF-spec-conformance.md): 보존할 correctness 기준
- [기존 Process Policy](../internal/core/policy/policy.go): 손실률·탄소 추가율·WASTE 계산
- [질량 비례 배분](../internal/core/allocation/allocation.go): floor·잔여분 처리
- [DPPClaim 검증](../internal/v2dpp/verify.go): Hash 재계산·원장 대조
- [감사 정보 구성](../internal/v2audit/model.go): 부모·미래 소비값의 순서와 개수
- [복호화·키 복구 측정](../internal/hotfix/key.go): 기존 workload·타이머 범위
- [감사 암호화 코어](../internal/core/auditcrypto/crypto.go): 키 복구·Field 벡터 복호화

관련 연구는 **평가 질문·비교 조건을 정하는 참고**로 사용합니다. Token Recipes의 입력 수별 비용, Sahai의 관계별 Proof 구성, PrivChain의 기능 추가 비용, DAG-Based Tokens의 조회 방식, Azeroth의 실행·SNARK 비용 분리를 참고하되 외부 수치로 직접적인 성능 순위를 매기지 않습니다.
