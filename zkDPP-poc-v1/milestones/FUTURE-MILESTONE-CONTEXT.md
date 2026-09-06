# M3~M9와 M6-B1 Future Milestone Context

**이 문서는 구현 명세가 아닙니다.** 완료된 단계의 결정과 아직 시작하지 않은 단계의 방향을 함께 보존합니다. 실제 구현 전에는 최신 코드와 결정을 대조하고 별도 Milestone 명세를 작성해야 합니다.

표기:

| 표기 | 의미 |
|---|---|
| `[합의]` | 현재까지 결정한 방향 |
| `[방향]` | 유력하지만 구현 전에 재검토할 내용 |
| `[미정]` | 구현 전에 반드시 선택할 내용 |
| `[폐기]` | 과거 문서에서 더 이상 사용하지 않을 가정 |

## 전역 폐기 가정

- `[폐기]` Entry의 `e`가 항상 0이어야 한다는 가정
- `[폐기]` `sk → pk → address` 소유자 계층
- `[폐기]` `Quantity + State[3]` 중복 수량 모델
- `[폐기]` Future Context만으로 구현을 시작할 수 있다는 가정

---

<a id="m3"></a>

## M3 — Transfer와 Voucher 생애주기

### 한 문장 목표

Note 일부 또는 전부를 Voucher로 전달하고 Proceed·Recall 중 하나로 한 번만 해결합니다.

### 이어받는 것

- M2 Note Tree와 commitment 등록
- M1 Private Spend와 Note nullifier
- 고정 ZK actor와 EVM 실행 기반

### 구현하려던 기능

```text
Transfer:
  Note → Voucher + Change Note

Proceed:
  Voucher → Receiver Note

Recall:
  Voucher → Sender Note
```

### 결정 상태

- `[합의]` Voucher는 pending Transfer를 나타내는 private payload입니다.
- `[합의]` Voucher Field 순서는 DocumentHash, AssetRole, `q_mass`, `a_rec`, `e`, sender·receiver address, private `deadlineEpoch`, opening입니다.
- `[합의]` Voucher Tree와 Voucher nullifier로 Proceed·Recall 중 한 번만 허용합니다.
- `[합의]` Proceed는 receiver의 `sk_owner`, Recall은 sender의 `sk_owner`를 검증합니다.
- `[합의]` Transfer는 Voucher와 Change Note를 원자적으로 생성하며 ELIGIBLE·WASTE를 모두 허용하고 DocumentHash·AssetRole을 유지합니다.
- `[합의]` Sender가 `q_voucher`를 선택하고 `a_rec`, `e`는 질량 비례로 나눕니다. Change를 내림 계산하고 첫 output인 Voucher가 residual을 받습니다.
- `[합의]` 운송 탄소 `delta_e_transport`는 M3에서 제외합니다.
- `[합의]` 전량 Transfer도 zero-State Change Note를 생성합니다.
- `[합의]` 600초 epoch를 사용하며 Recall은 deadline 이전만, Proceed는 deadline 전후 모두 허용합니다.
- `[합의]` Transfer에서 새 `rv`는 공개하지만 Proceed·Recall에서 소비하는 `rv`는 private witness입니다.
- `[합의]` `rvnf=H(VoucherNullifierTag,o_rv,rv)`이며 Sender·Receiver가 공유하는 opening으로 같은 resolution ID를 계산합니다.
- `[합의]` `deadlineRV[rv]` mapping 없이 private Voucher deadline과 public current epoch을 Recall Circuit에서 비교합니다.
- `[합의]` 정확한 Domain, public input 순서, 수도 코드와 Gate는 [M3 구현 명세](M3-transfer-voucher.md)를 따릅니다.
- `[폐기]` item-count Quantity와 PublicKey 기반 Voucher를 그대로 재사용하는 방식

### 의존성과 완료 후 행동

M2에 의존합니다. 완료되면 Note를 다른 참여자에게 pending 상태로 전달하고 수령 또는 회수할 수 있습니다.

---

<a id="m4"></a>

## M4 — Merge와 Split

### 한 문장 목표

ELIGIBLE Note를 합치거나 두 Note로 나누면서 State 보존과 정수 rounding을 검증합니다.

### 이어받는 것

- Private Spend·Note nullifier
- Note output commitment·Tree append
- $10^9$ scale의 `(q_mass,a_rec,e)`

### 구현하려던 기능

```text
Merge:
  Note + Note → Note

Split:
  Note → Note + Note
```

### 결정 상태

- `[합의]` 논문 Protocol은 동일 ProductProfile을 Merge compatibility 조건으로 사용합니다.
- `[합의]` M4 POC는 ProductProfile을 Note에 추가하지 않고 같은 owner·같은 AssetRole만 Merge compatibility로 검사합니다.
- `[합의]` ELIGIBLE+ELIGIBLE과 WASTE+WASTE Merge를 허용하고 서로 다른 AssetRole은 거부합니다.
- `[합의]` Merge는 입력 State의 합과 output State가 일치해야 합니다.
- `[합의]` Merge·Split output은 새 DocumentHash를 사용할 수 있으며 POC Circuit은 input과의 DocumentHash 관계를 검사하지 않습니다.
- `[합의]` Split은 질량·credit·탄소 합을 보존해야 합니다.
- `[합의]` Merge 덧셈은 uint64 overflow를 검사합니다.
- `[합의]` Split은 정수 나눗셈의 residual을 잃지 않아야 합니다.
- `[합의]` Split은 M3와 동일하게 output 2를 내림 계산하고 output 1이 residual을 받습니다.
- `[합의]` Split의 `a_rec`, `e`는 질량 비례로 강제합니다.
- `[합의]` Split input은 양의 질량이어야 하지만 output 하나의 질량 0은 허용합니다.
- `[합의]` 상세 public input·수도 코드·Gate는 [M4 구현 명세](M4-merge-split.md)를 따릅니다.
- `[폐기]` 모든 State를 단순 정수 kg 단위로만 표현한다는 가정

### 의존성과 완료 후 행동

M2에 의존하며 M3와 독립적으로 먼저 구현할 수도 있습니다. 완료되면 private lot의 합·분할을 증명할 수 있습니다.

---

<a id="m5"></a>

## M5 — Process와 Policy Circuit

### 한 문장 목표

Policy별 exact `(m,n)` Circuit으로 여러 input State를 주제품·폐기물 State로 변환합니다.

### 이어받는 것

- 다중 Private Spend와 output Note 등록
- Merge·Split에서 검증한 합·배분·rounding 기반
- Policy별 PK·VK를 생성할 개발용 SRS lifecycle

### 구현하려던 기능

```text
Process:
  Note[m] → Note[n]

Factory 규칙 제출
  → Certifier 검토·Circuit Setup
  → Factory proof
  → Contract VK 검증
```

### 결정 상태

- `[합의]` Policy별 exact `(m,n)` Circuit을 사용합니다.
- `[합의]` Circuit은 numeric State 관계를 검증하고 Product Type의 현실 진실성은 외부 검토에 둡니다.
- `[합의]` M5 POC는 exact 3-to-2이며 input 3개는 같은 owner의 ELIGIBLE, output 0은 ELIGIBLE, output 1은 WASTE입니다.
- `[합의]` $D=10^9$, lossRate=62,500,000, carbonIntensity=93,750,000을 Circuit constant로 사용합니다.
- `[합의]` 실제 `q_loss`, `delta_e_process`는 private witness이며 input 총질량과 고정 rate의 floor 관계를 검증합니다.
- `[합의]` Allocation coefficient와 output Role은 Circuit constant입니다. 질량은 ELIGIBLE 90%·WASTE 10%, 탄소와 `a_rec`은 ELIGIBLE 100%입니다.
- `[합의]` output 1 WASTE를 floor하고 output 0 ELIGIBLE이 residual을 받습니다. WASTE는 `a_rec=0`, `e=0`입니다.
- `[합의]` PolicyRef는 `(eventKind,authorityId,policyId,version)`, ScopeRef는 `(sk_owner,policyRef)`에 Poseidon2로 binding합니다.
- `[합의]` Policy Authority는 등록된 EVM account와 증가하는 authorityId를 사용합니다. Operator identity는 `sk_owner`입니다.
- `[합의]` PolicyRecord는 eventKind·arity·vkHash·verifierRef·enabled와 Authority family/version을 기록합니다.
- `[합의]` 같은 PolicyRef 재등록·수정·재활성화는 금지하고 새 규칙은 새 version으로 등록합니다. Grant는 revoke·regrant를 허용합니다.
- `[합의]` 같은 Policy version 안의 scopeRef linking과 public Policy fingerprint를 허용합니다.
- `[합의]` constant verifier를 공식 경로로 사용하고 optimized VK storage generic verifier를 architecture ablation으로 함께 구현·측정합니다.
- `[합의]` `vkHash`는 SHA-256 canonical on-chain optimized VK encoding입니다.
- `[합의]` 상세 이유는 [M5 Background](M5-process-policy-background.md), 구현 기준은 [M5 명세](M5-process-policy.md)를 따릅니다.
- `[폐기]` `M_max`, `N_max`와 inactive slot padding

### 의존성과 완료 후 행동

M2·M4에 의존합니다. 완료되면 승인된 공정 규칙에 따른 private 다입력·다출력 State transition을 증명할 수 있습니다.

---

<a id="m6"></a>

## M6 — Active·Frozen·Revoked 상태 집행

아래 합의는 완료된 M6의 구현 기준입니다. 이후 nullifier 기반 상태 전환 구상은 [M6-B1 이후 연결](#m6-b1)과 [M7](#m7)에서 구분합니다. M6-B1은 암호화 코어만 검증하며 기존 M6 Main Contract를 변경하지 않습니다.

### 한 문장 목표

문제가 있는 Note·Voucher를 일시 동결하거나 영구 철회하고 소비 Circuit에서 현재 상태를 확인합니다.

### 이어받는 것

- 생성된 Note·Voucher identifier와 insertion index
- 모든 소비 Event의 private input 검증
- Status Authority 역할

### 결정 상태

- `[합의]` 상태는 Active, Frozen, Revoked입니다.
- `[합의]` Frozen 객체는 사용할 수 없고 Revoked는 terminal입니다.
- `[합의]` 상태와 provenance graph는 별도 개념입니다.
- `[합의]` NoteStatusTree와 VoucherStatusTree를 분리하고 depth 32를 사용합니다.
- `[합의]` 별도 Hash 기반 위치를 만들지 않고 기존 Note·Voucher insertion index를 Status index로 재사용합니다.
- `[합의]` Active는 empty leaf `0`, Frozen은 `1`, Revoked는 `2`이며 정상 output 생성은 Status root를 바꾸지 않습니다.
- `[합의]` StatusTree node·path는 오프체인 Indexer가 유지하고 Contract는 최신 Note·Voucher Status root만 저장합니다.
- `[합의]` 소비 Circuit은 기존 private membership index를 같은 객체의 private Status path에도 사용합니다.
- `[합의]` Status Authority는 constructor에서 지정한 immutable EVM account입니다.
- `[합의]` Status Authority가 private path로 StatusUpdate ZKP를 만들고 Contract는 proof 검증 후 new root만 SSTORE합니다.
- `[합의]` Status 변경 대상 object ID·index·전이는 공개하지만 이후 private 소비와의 연결은 공개하지 않습니다.
- `[합의]` transaction 하나당 leaf 하나만 변경하며 Batch·multiproof를 지원하지 않습니다.
- `[합의]` M6의 `ZkDPPStatusLedger`와 Status-aware Circuit은 현재 구현된 Main Protocol이며 M1~M5 구현은 구성 요소 검증용 baseline입니다. M7의 nf 기반 전환 이후까지 StatusTree 사용을 고정한다는 뜻은 아닙니다.
- `[합의]` M6는 Note·Voucher Status만 구현하고 Claim은 M8로 미룹니다.
- `[방향]` 소비된 객체는 Status Authority가 update 대상으로 선택하지 않으며 M7 AuditRecord가 live 대상 식별 근거를 보강합니다.
- `[방향]` M6 당시 후속 과제였던 downstream 탐색·순차 동결은 현재 [M7 명세](M7-audit-tracing.md)에서 nf 기반으로 구체화했습니다. M6의 과거 구현과는 구분합니다.
- `[폐기]` 소비할 object ID나 index를 public input으로 공개해 mapping을 직접 조회하는 방식
- `[폐기]` 모든 output에 명시적 Active leaf를 생성하는 방식
- `[폐기]` Contract가 Status path를 받아 Poseidon2 old·new root를 직접 계산하는 방식
- `[폐기]` 현재 논의 없이 과거의 추상 StatusTree 요구사항을 곧바로 구현 명세로 취급하는 방식

상세 의사결정은 [M6 Background](M6-status-enforcement-background.md), 구현 기준은 [M6 명세](M6-status-enforcement.md)를 따릅니다.

### 의존성과 완료 후 행동

M3·M5 객체 정의에 의존하며, M6에서는 Active인 private Note·Voucher만 다음 Event에서 사용하도록 구현했습니다. Claim Status는 M8에서 당시의 Main Protocol을 기준으로 설계하며, M6 StatusTree의 확장을 미리 확정하지 않습니다.

---

<a id="m6-b1"></a>

## M6-B1 — 재사용 감사 암호화 코어 검증

### 한 문장 목표

여러 Event에서 재사용할 감사 암호화 코어를 만들고, 실제 3→2 Process의 올바른 감사 정보를 암호화했다는 PLONK 증명과 2-of-3 원문 복원을 검증합니다.

### 이어받는 것

- M1~M5의 Note·nullifier·membership·Process 관계와 기존 Poseidon2입니다.
- 기존 Artifact·verifier 생성·Anvil 측정 기반입니다.
- M6 이후 nullifier 기반 전방 추적·상태 전환을 준비한다는 목적이며, M6 StatusTree 코드를 교체하는 단계는 아닙니다.

### 결정 상태

- **[합의]** YAGNI를 최우선으로 하며 코어의 구현 가능성·정확성·성능을 먼저 확인합니다.
- **[합의]** 암호화 곡선은 Jubjub이고 proof는 기존 BLS12-381 기반 PLONK입니다.
- **[합의]** 배포자가 오프체인에서 신뢰 Setup을 수행하고 정직한 위원 세 명에게 shares를 분배합니다. 복호화는 2-of-3입니다.
- **[합의]** Auditor와 위원은 정직하며 승인된 기록의 온체인 원본만 복호화합니다. Participant의 감사 정보는 Circuit으로 검증합니다.
- **[합의]** transaction당 감사 메시지 벡터 하나·대칭키 하나·암호문 하나를 사용합니다.
- **[합의]** 부모 참조와 출력의 소비 nf/rvnf를 암호화합니다. 공개 output commitment는 중복 암호화하지 않습니다.
- **[합의]** 종류·개수는 공개 Event 구조에서 해석하고 고정 순서를 따릅니다. 대표 Process 평문은 $(cm_A,cm_B,cm_C,nf_D,nf_E)$입니다.
- **[합의]** 같은 키에서 위치별 Poseidon2 마스크를 파생하고 Field 덧셈·뺄셈으로 암호화·복호화합니다.
- **[합의]** 독립 5-Field Circuit과 실제 M5 Process adapter를 구분합니다. 1-Field 재사용은 correctness에서 확인합니다.
- **[합의]** 원 TDH2의 $\Gamma,R_2,e,f$와 위원 응답 증명은 이번 신뢰 모델에서 제외합니다. 원 TDH2의 일반적인 선택 암호문 공격 보안을 그대로 주장하지 않습니다.
- **[합의]** 위원은 share 자체가 아니라 partial decryption을 제공하고, Auditor는 master secret 없이 평문을 복원합니다.
- **[합의]** 작은 진단용 Contract로 proof 검증·기록·원본 복원 흐름과 gas를 확인합니다. Main Ledger의 소비·PolicyGrant·Tree·Status 집행은 하지 않습니다.
- **[합의]** Participant 비용, 온체인 검증·저장 비용, 1·10·100·1,000개 암호문의 오프라인 복호화 비용을 분리합니다. 공식 case별 측정은 한 번입니다.
- **[폐기]** 한 번의 벡터 암호화를 여러 개의 독립 키·Threshold 암호문으로 해석하는 방식입니다.
- **[폐기]** $\Gamma$ 생성을 위한 Zcash GroupHash·hash-to-curve와 e/f용 scalar 변환을 B1에 구현하는 계획입니다.
- **[폐기]** 암호화 코어 완료를 전체 전방 추적·동결 Main Protocol 완료로 표시하는 방식입니다.

### 이후에 연결할 내용

- **[방향]** 모든 성공 Event에 감사 평문과 등록 검증을 연결하고 실제 AuditRecord·producerOf·noteSpentIn·voucherSpentIn을 원자적으로 갱신합니다.
- **[방향]** 감사 프로그램은 snapshot에서 기록을 복호화하고, 부모 방향 또는 소비 기록을 통한 downstream 방향으로 탐색합니다. 같은 transaction의 복호화 결과를 재사용합니다.
- **[합의]** 후속 경로는 Note·Voucher의 nullifier를 key로 Active·Frozen·Revoked를 관리해 별도 StatusTree를 대체합니다. 기존 membership Tree는 유지합니다. 이 전환은 아직 구현하지 않았습니다.
- **[합의]** 8개 Event·AuditRecord·조회·nf 상태와 대표 시나리오를 M7에서 구현·검증했습니다.
- **[미정]** Claim과 관련된 감사·상태·DPP 기능은 M8의 실제 설계에서 정합니다.

### 의존성과 완료 후 행동

M5의 실제 witness·회로 관계와 이전 EVM 도구를 사용합니다. 완료되면 감사 정보 벡터를 Native와 Circuit에서 같은 방식으로 암호화하고, 검증된 대표 기록에서 정확한 원문을 복원할 수 있습니다.

선택 이유는 [M6-B1 Background](M6-B1-audit-encryption-core-background.md), 함수·수도 코드·검증·측정 기준은 [M6-B1 구현 명세](M6-B1-audit-encryption-core.md)를 따릅니다. 코어·Process 연결·진단 원본 복원은 완료됐으며 실제 결과와 한계는 [M6-B1 Result](M6-B1-audit-encryption-core-result.md)에 있습니다. 이후 원장·탐색·동결 통합은 여전히 별도 설계 대상입니다.

---

<a id="m7"></a>

## M7 — 감사 기록·추적·nf 기반 동결 통합

### 한 문장 목표

검증된 암호화 코어를 실제 Event·원장에 연결해 부모·자손을 추적하고, 선정한 미소비 대상을 nf 기반으로 동결했습니다. 관계·API는 [M7 단일 명세](M7-audit-tracing.md), 실제 결과는 [M7 Result](M7-audit-tracing-result.md)에 있습니다. 별도 Background는 없습니다.

### 이어받는 것

- M1~M5의 Entry·Exit·Transfer·Proceed·Recall·Merge·Split·Process 관계
- Note·Voucher 참조를 구분하는 요구사항; Claim은 M8 범위
- M6-B1에서 검증을 마친 암호화 코어와 정직한 위원회·Auditor 가정
- 기존 M6의 상태 집행 목적과 nullifier 기반 전환 구상

### 구현하려던 기능

```text
정상 Event:
  actual parents + output 소비 nullifier
  → transaction 단위 암호문 → AuditRecord

Audit:
  AuditRecord ID → 원본 확인 → 2 partial decryptions
  → 부모와 출력 소비값 복원
  → 생성 또는 소비 기록 조회
```

### 결정 상태

- `[합의]` 전체 설계의 AuditRef는 Note·Voucher·Claim 종류를 구분합니다. M7의 구체 표현은 실제 Note·Voucher Event에 맞춰 정하며 Claim 구현은 M8에 둡니다.
- `[합의]` M7에서 연결하는 성공 Event마다 감사 기록을 남깁니다. 기록 하나라는 의미와 모든 공개값을 storage에 복사한다는 구현을 혼동하지 않습니다.
- `[합의]` Circuit은 실제 부모 cm/rv와 생성한 객체가 소비될 때 사용할 올바른 nf/rvnf가 암호문에 들어 있는지 검증합니다.
- `[합의]` Committee는 DAG를 만들지 않고 ciphertext별 partial decryption만 제공합니다.
- `[합의]` Status Authority의 감사 실행기가 partial decryption 결과를 결합합니다. 위원 secret share 자체를 수집하지 않습니다.
- `[합의]` 암호화 코어·대표 Process 연결·2-of-3 복원은 M6-B1에서 완료했습니다. 원장 통합에서 재사용하되 모든 Event 적용이 끝났다고 해석하지 않습니다.
- `[합의]` Note는 nf, Voucher는 rvnf로 상태를 조회하는 방식으로 전환합니다. 별도 StatusTree·Active path·StatusUpdate proof는 대체하고 기존 membership Tree와 M6의 과거 구현·결과는 보존합니다.
- `[합의]` AuditRecord에 암호문·공개점·outputRefs·eventKind·policyRef를 저장합니다. 기록 ID는 mapping key이며 중복 필드로 저장하지 않습니다. 부모 수는 배열 길이로 읽습니다.
- `[합의]` 기존 public input 전체를 복사하지 않습니다. 암호문도 calldata에만 둔다는 해석은 폐기합니다. AuditRecorded 로그로 원본 transaction을 찾고 문맥을 재계산합니다.
- `[합의]` 승인받은 Participant가 자기 최초 Note를 Entry합니다. 승인자가 다른 owner를 위해 대신 Note를 발행하는 흐름을 기본 시나리오로 두지 않습니다.
- `[방향]` 부모와 출력 소비값을 같은 transaction 승인으로 복원하고 backward/forward 탐색에 사용합니다.
- `[방향]` producerOf와 noteSpentIn·voucherSpentIn으로 생성·소비 기록을 연결합니다.
- `[방향]` snapshot의 미소비 leaf를 찾아 Status Authority가 선정한 대상만 순차 동결합니다. Contract의 자동 재귀 추적·미래 모든 자손의 자동 동결은 보장하지 않습니다.
- `[합의]` 8개 Event별 평문·공개 입력 순서, 기록·소비·Tree의 원자성, producerOf·spentIn·status 조회는 M7 명세를 따릅니다. 새 ZkDPPAuditLedger를 사용하고 기존 M6와 상태 migration은 하지 않습니다.
- `[합의]` 복호화 원문은 한 감사 실행의 메모리에서만 재사용하고, 두 방향의 측정은 각각 빈 캐시로 시작합니다.
- `[미정]` 실제 서비스의 요청 승인·키 수명 관리·배포 운영은 코어 POC와 분리해 판단합니다.
- `[폐기]` Authority가 master secret key를 복구·폐기하는 방식을 기본안으로 사용한다는 가정
- `[폐기]` TDH2 전체 구현과 악의적인 위원 응답 검증을 현재 신뢰 모델의 필수 조건으로 두는 가정
- `[폐기]` 코어의 primitive 선택·구현 검증·원장 통합이 모두 같은 완료 상태라는 가정
- `[폐기]` B1의 공개 입력 15개 저장을 최소 AuditRecord 형식이나 필수 SSTORE 비용으로 취급하는 가정

### 감사 정보는 어디에서 읽나요?

**암호문과 outputRefs는 AuditRecord에서 읽습니다.** 부모와 출력 소비값은 같은 키로 암호화한 벡터의 두 부분이며 공개점도 함께 보관합니다. 이 범위와 별개로 noteRoot·policyScopeRef·입력 nf·proof 등을 모두 Record에 복사하지 않습니다.

문맥은 AuditRecorded의 기록 ID로 찾은 검증된 transaction 공개 입력에서 재계산합니다. 원본의 Event 종류·출력·암호문과 저장 Record의 일치를 확인합니다. 암호문을 calldata에만 두는 구조로 전환하지 않습니다.

Contract는 과거 transaction 내용을 임의로 조회하는 감사 프로그램과 다릅니다. 이후 소비·동결 판단에 직접 필요한 nullifier·status 등은 상태로 보관할 수 있습니다. producerOf·noteSpentIn·voucherSpentIn도 역할에 맞게 최소 저장 범위를 정합니다. 모든 SSTORE를 없애겠다는 의미는 아닙니다.

### Entry에서 누가 자기 secret을 사용하나요?

**A는 권한을 부여하고, Participant B가 자기 Note를 만들어 Entry합니다.**

1. A는 B에게 Entry 호출 권한을 부여합니다.
2. B는 자기 owner secret으로 주소를 계산하고 그 주소의 최초 Note를 만듭니다.
3. B는 같은 secret과 cm으로 소비 nf를 계산해 암호화하고, 올바른 관계를 증명합니다.
4. Contract는 Entry 호출 권한과 proof를 확인합니다. Entry는 Note 생성이지 nf 소비 처리가 아닙니다.

현재 코드에서 A 역할은 Admin, B의 호출 권한은 entryIssuers[account]입니다. 이 mapping은 모든 공급망 참여자의 일반 자격이 아니라 Entry 권한만 나타냅니다. A에게 B의 secret을 주거나 별도 owner의 승인을 다시 받는 과정은 필요하지 않습니다.

M1~M6 baseline Entry는 Note 주소를 commitment에 포함하지만 secret에서 주소를 재계산하거나 출력 소비 nf를 암호화하지 않습니다. M7 감사 Entry는 이 검사를 추가했습니다. EVM 호출자와 ZK owner가 자동으로 동일인임을 증명하는 것은 아니며 별도 일반 Participant 등록 시스템도 추가하지 않았습니다.

### 완료 시나리오는 무엇인가요?

**Entry→Split→Split에서 깊이가 있는 부모·자손 추적을 확인합니다.** 아래는 명세의 기대 동작이며 실제 완료 결과가 아닙니다.

1. 승인받은 Participant가 자기 Note A를 Entry합니다.
2. A를 B·C로 Split하고 B를 D·E로 다시 Split합니다. 이때 A~E는 객체 이름입니다.
3. D에서 B를 거쳐 A의 Entry까지 backward tracing합니다.
4. A에서 소비 기록을 따라 미소비 C·D·E를 찾습니다. 이미 소비된 A·B는 제외합니다.
5. 선정한 세 nf를 Freeze하고 소비 거부를 확인합니다. C는 Unfreeze 후 Exit, E는 Revoke 후 지속 거부를 확인합니다.

별도 Voucher 예시에서는 Transfer로 생성한 rv를 Freeze하면 Proceed·Recall 모두 실패하고, Unfreeze 후 허용된 분기 하나만 성공함을 확인합니다. Merge·Process를 포함한 나머지 Event 연결·원자성·측정은 명세의 기능별 테스트로 구체화합니다.

감사 기준 snapshot과 실제 동결 시점은 구분합니다. 그 사이 소비된 대상을 미소비 동결 성공으로 보고하지 않으며, Contract의 자동 재귀 추적이나 미래 자손의 자동 동결을 약속하지 않습니다.

### 의존성과 완료 후 행동

M3~M6의 Event·상태 목적과 M6-B1 코어를 실제로 연결했습니다. 기존 M6 코드·완료된 Result는 보존했습니다. 자세한 한계와 측정은 M7 Result에서 확인합니다.

---

<a id="m8"></a>

## M8 — Exit·DPP·Issue Claim

구현 결과: [M8 Result](M8-exit-dpp-issue-result.md)

### 한 문장 목표

private Note를 Exit하여 finalized DPP commitment로 확정하고, 제품 정보와 State를 공개하지 않은 채 Policy별 Sustainability Claim을 반복해서 연결합니다.

### 이어받는 것

- M7의 private Note 소비·nf 상태·AuditRecord·producerOf·backward tracing
- M5의 Authority·Policy family·version·immutable PolicyRecord·one-way disable
- M6-B1의 감사 암호화 코어와 M7의 암호문 저장 경계

### 구현하려던 기능

```text
Exit:
  private Note → Finalized DPP(dppCommitment)

Issue:
  Finalized DPP → 같은 DPP + Policy Claim
```

### 결정 상태

- `[합의]` 공급망 안의 개념적 DPP Object는 private Note로 표현하며 공급망 단계에는 Claim을 붙이지 않습니다.
- `[합의]` Exit가 Note를 nf로 소비하고 같은 DocumentHash·AssetRole·State를 가진 dppCommitment를 생성합니다.
- `[합의]` dppCommitment는 DPPTag, DocumentHash, AssetRole, State와 dppOpening을 포함하고 Note owner·opening·nf는 포함하지 않습니다.
- `[합의]` Issue는 DPP를 소비하지 않으며 서로 다른 Issue Policy version에 반복할 수 있습니다.
- `[합의]` Claim은 dppCommitment와 issuePolicyRef의 조합이며 같은 조합은 하나만 허용합니다.
- `[합의]` Issue에는 PolicyGrant·policyScopeRef를 사용하지 않고 등록된 Policy verifier와 DPP private 원문 지식으로 증명합니다.
- `[합의]` Standard V1은 재활용률 10% 이상·탄소집약도 1.00 이하, Strict V2는 11% 이상·0.97 이하입니다.
- `[합의]` Claim은 Active·Frozen·Revoked 상태를 Policy version별로 독립 관리합니다.
- `[합의]` Issue AuditRecord에는 숨겨진 부모·미래 소비값이 없으므로 암호문을 저장하지 않습니다.
- `[방향]` 구현 전 기준은 [M8 Background](M8-exit-dpp-issue-background.md)와 [M8 구현 명세](M8-exit-dpp-issue.md)입니다.
- `[합의]` 위 설계는 M8에서 구현·검증됐으며 실제 Circuit·gas·감사 결과는 M8 Result를 기준으로 합니다.
- `[미정]` M8 구현 이후 DPP ownership transfer나 component DPP 재귀 구조를 별도 단계로 다룰지 결정합니다.
- `[폐기]` Issue가 Note를 직접 소비하면서 공개 Hash와 Claim nonce를 동시에 만드는 구조입니다.
- `[폐기]` 공급망 중간 Claim을 M8에 포함하거나 별도 Claim Tree를 만드는 가정입니다.

### 의존성과 완료 후 행동

M5~M7에 의존합니다. 완료되면 제3자는 dppCommitment와 issuePolicyRef만으로 Claim 등록 여부·상태·Issue 기록을 조회하고, Auditor는 Exit를 통해 기존 upstream provenance로 이동할 수 있습니다.

---

<a id="m9"></a>

## M9 — 최종 통합·Universal SRS·Anvil 평가

구현 결과: [M9 Result](M9-final-integration-result.md)

### 한 문장 목표

M8 Main Protocol의 최종 Circuit 10개를 하나의 $2^{17}$ universal SRS로 Setup하고, ZkDPPClaimLedger 하나에서 전체 공급망 흐름을 Anvil로 재현합니다.

### 이어받는 것

- M8 Main Protocol이 실제 사용하는 고정 Event Circuit 7개와 Policy Circuit 3개
- Event별 constraints·gas·E2E 결과
- M8 ZkDPPClaimLedger·AuditRecord·nf 상태·DPP·Claim

### 구현하려던 기능

```text
최종 Circuit 10개 compile
  → plonk.SRSSize 대조
  → 2^17 universal canonical SRS
  → 2^14·2^15·2^16·2^17 Lagrange SRS
  → Circuit별 PK·VK·verifier

다단계 공급망 전체 Event
  → ZkDPPClaimLedger
  → Anvil 2회
  → 최종 SRS·Circuit·gas·E2E·감사 평가
```

### 결정 상태

- `[합의]` 현재 가장 큰 Process가 요구하는 $2^{17}$ domain을 지원하는 canonical SRS 131,075 points를 생성합니다.
- `[합의]` domain $2^{14}$·$2^{15}$·$2^{16}$·$2^{17}$의 Lagrange SRS와 Circuit별 PK·VK를 생성합니다.
- `[합의]` Universal SRS·Lagrange 변환·Policy Setup 비용을 분리합니다.
- `[합의]` 최종 Main Contract는 ZkDPPClaimLedger 하나이며 별도 Router를 만들지 않습니다.
- `[합의]` Entry·Transfer·Proceed·Recall·Merge·Split·Exit verifier는 constructor에 고정합니다.
- `[합의]` Process 3-to-2·Issue Standard V1·Issue Strict V2 verifier는 PolicyRecord의 verifierRef로 선택합니다.
- `[합의]` 현재 M8의 Process verifier constructor·PolicyRecord 중복을 제거합니다.
- `[합의]` poc-v2의 Recall 재시도·다중 Supplier·Merge·Process·Split 흐름을 현재 M8 의미로 재구성합니다.
- `[합의]` 공식 측정은 기본 2회이며 시간·메모리 차이가 작은 값 대비 20%를 넘을 때만 3회차를 추가합니다.
- `[합의]` Gas가 다르면 평균내지 않고 transaction·state·calldata·bytecode 차이를 조사합니다.
- `[합의]` final public-input manifest와 Artifact checksum을 생성합니다.
- `[폐기]` Besu·QBFT·validator·RPC node 평가를 M9에 포함합니다.
- `[폐기]` 별도 Router를 추가합니다.
- `[폐기]` 현재 필요보다 큰 $2^{20}$ SRS를 선제 생성합니다.
- `[폐기]` 외부 프로젝트와 성능 순위를 비교합니다.
- `[폐기]` 회로마다 서로 무관한 최종 SRS를 생성한다는 방식
- `[합의]` 위 설계는 M9에서 구현·검증됐으며 실제 SRS·Circuit·Anvil·감사 수치는 M9 Result를 기준으로 합니다.

### 의존성과 완료 후 행동

상세 구현 기준은 [M9 명세](M9-final-integration.md)입니다. 완료되면 clean checkout에서 하나의 development universal SRS·최종 Contract·Anvil 시나리오·평가 보고서를 재현할 수 있습니다.
