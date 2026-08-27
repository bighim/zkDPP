# zkDPP v1 공통 이해

- 상태: discussion draft — 1~5번 합의 반영본
- 기준일: 2026-08-27
- 목적: 저와 동료가 zkDPP v1의 현재 구조를 짧은 시간 안에 같은 상태로 복원하기 위한 문서입니다.
- 상세 명세: [`zkdpp-v1-implementation-spec.md`](./zkdpp-v1-implementation-spec.md)

이 문서는 상세 구현 명세를 반복하지 않습니다. **현재 무엇을 만들고 있으며, 객체가 어떻게 변하고, 무엇을 검증·통제·감사하는지**를 하나의 예시로 기억하는 문서입니다.

이번 수정본에는 다음 다섯 결정을 반영했습니다.

1. State 표기 순서를 `(q_mass, a_rec, e)`로 통일합니다.
2. Process는 exact `(m,n)`별 PolicyRef·Circuit·VK를 사용합니다.
3. 별도의 `canonical DPP core` 객체를 만들지 않고 기존 `DocumentInfo`와
   `DocumentHash`를 Issue binding에 사용합니다.
4. `Factory`와 `Auditor`를 별도 protocol role로 두지 않습니다. 일반
   증명 생성자는 `Participant`, 감사 orchestration은 `Status Authority`의
   기능으로 표현합니다.
5. Note와 Voucher는 `poc-v2`의 depth-32 온체인 append-only Tree를
   재사용하고, 등록된 과거 membership root를 허용하되 현재
   nullifier/resolution 상태를 원자적으로 검사합니다. 외부 Indexer는
   필수 신뢰 구성요소가 아닌 선택적 조회 도구입니다. StatusTree는 별도로
   항상 최신 `statusRoot`만 허용합니다.

표기의 의미는 다음과 같습니다.

| 표기 | 의미 |
|---|---|
| **합의** | 현재 동일하게 이해하고 결정한 내용입니다. |
| **미확정** | 추가 논의 또는 Profile 결정이 필요합니다. |
| **범위 밖** | v1이 해결한다고 주장하지 않는 내용입니다. |

---

## 1. 한 줄로 기억하는 zkDPP

> zkDPP는 물품의 질량·재활용 credit·누적 탄소 State를 숨긴 채 공급망 Event의 계산 일관성을 증명하고, 승인된 Policy·동결·비공개 backward audit·DPP Claim을 함께 제공하는 시스템입니다.

전체 구조는 다음 네 질문으로 기억합니다.

```text
Product Flow : 물품 상태가 어떻게 변하는가?
Policy       : 어떤 Circuit을 누가 사용할 수 있는가?
Status       : 이 객체를 지금 사용할 수 있는가?
Audit        : 이 객체가 어디에서 만들어졌는가?
```

---

## 2. 하나의 예시로 보는 전체 구조

### 입력 원자재를 공정해 봅시다

각 물품의 State를 `(질량, 재활용 mass-balance credit, 누적 탄소)` 순서로 적습니다.

```text
원자재 A = (100, 20,  80)
원자재 B = ( 70, 10, 120)
원자재 C = (150,  0,  30)
            ----------------
입력 합계 = (320, 30, 230)
```

공정에서 질량 20이 감소하고 탄소 30이 추가되며 재활용 credit은 변하지 않는다고 가정합니다.

```text
입력 합계       = (320, 30, 230)
공정 변화       = (-20,  0, +30)
공정 후 전체 값 = (300, 30, 260)
```

공정 결과를 주제품과 폐기물로 나눕니다.

```text
주제품 D = (270, 30, 260)  → ELIGIBLE
폐기물 W = ( 30,   0,  0)  → WASTE
            ----------------
출력 합계 = (300, 30, 260)
```

### 같은 예시를 Protocol 객체로 봅시다

```text
Note A + Note B + Note C
           │
           │ Process proof
           ▼
      Note D + Note W
```

각 output에는 새로운 commitment가 생깁니다.

```text
주제품 D → NoteRef(cm_D)
폐기물 W → NoteRef(cm_W)
```

Process proof가 검증되어 transaction이 성공하면 Contract는 AuditRecord 하나를 만들고, 그 기록에 `processRecordId`를 부여합니다.

```text
Process transaction 성공
  → processRecordId 부여
  → auditRecords[processRecordId] 저장
       ├─ eventKind: Process
       ├─ outputRefs: NoteRef(cm_D), NoteRef(cm_W)
       ├─ parentCount: 3
       └─ encryptedParents:
            Encrypt(NoteRef(cm_A), NoteRef(cm_B), NoteRef(cm_C))
```

각 output이 어느 기록에서 생성됐는지 바로 찾을 수 있도록 별도 index도 저장합니다.

```text
producerOf[auditKey(NoteRef(cm_D))] = processRecordId
producerOf[auditKey(NoteRef(cm_W))] = processRecordId
```

---

## 3. Product를 어떻게 표현하는지 살펴봅시다

### 큰 그림

```text
Product
  ├─ DocumentHash : 어떤 물품 설명에 연결되는가?
  ├─ State        : 질량·재활용 credit·탄소 값은 얼마인가?
  ├─ assetRole    : ELIGIBLE인가 WASTE인가?
  └─ owner        : 누가 소비할 수 있는가?
           │
           ▼
        Note(cm)
```

### State

$$
\mathrm{State}=(q_{\mathrm{mass}},a_{\mathrm{rec}},e)
$$

| 값 | 현재 의미 |
|---|---|
| $q_{\mathrm{mass}}$ | 물품의 전체 질량입니다. |
| $a_{\mathrm{rec}}$ | Note에 할당된 재활용 mass-balance credit의 절대 질량입니다. 실제 재활용 물질이 그만큼 포함됐다는 뜻은 아닙니다. |
| $e$ | Entry 이후 누적된 탄소발자국 절대량입니다. |

재활용 함량과 탄소집약도는 저장된 State로부터 계산합니다.

$$
\mathrm{RecycledContent}=\frac{a_{\mathrm{rec}}}{q_{\mathrm{mass}}}
$$

$$
\mathrm{CarbonIntensity}=\frac{e}{q_{\mathrm{mass}}}
$$

### DocumentHash

v1은 기존 PoC의 `DocumentInfo`를 그대로 사용합니다.

```text
DocumentInfo = (ProductName, LotID, Unit)
DocumentHash = HashDocumentInfo(DocumentInfo)
```

`DocumentHash`는 Note의 State와 별도로 제품·batch 설명에 binding하는 값입니다.
전체 DPP 문서의 모든 field를 hash한 값은 아닙니다.

### assetRole

```text
ELIGIBLE → 후속 Event와 Issue에 사용 가능
WASTE    → Exit만 가능
```

`assetRole`은 물질 종류가 아니라 Protocol에서의 사용 역할입니다. 값은 Note commitment 안에 숨겨지고 Circuit이 Policy가 정한 역할과 일치하는지 검증합니다.

---

## 4. 핵심 객체의 생애주기

먼저 **Protocol 의미 기준**으로 보면 다음과 같습니다. 여기서 `Note 생성`과 `Voucher 생성`은 private payload를 만든 뒤 그 commitment를 온체인에 등록한다는 뜻입니다.

```text
Entry:
  ∅ → Note

Transfer:
  Note → Voucher + Change Note

Proceed / Recall:
  Voucher → Note

Merge:
  Note + Note → Note

Split:
  Note → Note + Note

Process:
  Note[] → Note[]

Exit:
  Note → ∅

Issue:
  Note → Claim
```

온체인에는 private payload 전체가 아니라 다음 공개 참조와 상태가 등록됩니다.

| Protocol 객체 | 객체가 담는 내용 | 온체인에서 보이는 값 |
|---|---|---|
| `Note` | `DocumentHash`, `assetRole`, $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$, `owner`, `opening` | `cm`을 Note Tree에 추가하고 `NoteRef(cm)`을 outputRef로 기록 |
| `Voucher` | `DocumentHash`, `assetRole`, $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$, `senderOwner`, `receiverOwner`, `deltaEpoch`, `opening` | `rv`를 Voucher Tree에 추가하고 `VoucherRef(rv)`를 outputRef로 기록 |
| `Claim` | 별도의 Note·Voucher형 private payload는 없으며, DPP 문서에 `DPPClaim=(issuePolicyRef,h,claimNonce)`를 기록 | `claimRegistered[h]`를 기록하고 `ClaimRef(h)`를 outputRef로 기록 |

여기서 `assetRole`은 막연한 역할을 뜻하지 않습니다. Note 또는 Voucher가 Protocol에서 후속 사용 가능한 물품인지 폐기물인지를 나타내는 private field입니다.

```text
assetRole = ELIGIBLE
  → 후속 공급망 Event와 Issue에 사용할 수 있음

assetRole = WASTE
  → Exit만 사용할 수 있음
```

따라서 위 흐름을 **온체인 공개 참조 기준**으로만 다시 쓰면 다음과 같습니다.

```text
Note       → NoteRef(cm)
Voucher    → VoucherRef(rv)
Claim      → ClaimRef(h)

모든 성공 Event
  → AuditRecord 1개
  → 각 output의 producerOf index
```

오프체인 DPP 문서에는 Issue 후 다음 항목을 추가하는 구조로 이해하고 있습니다.

```text
DPPClaim = (issuePolicyRef, h, claimNonce)
```

---

## 5. 지원하는 핵심 기능

| 기능 | 기억할 내용 |
|---|---|
| State 검증 | Event별 질량·재활용 credit·탄소 관계를 검증합니다. |
| Private consumption | 어떤 기존 `cm` 또는 `rv`를 소비했는지 숨깁니다. |
| Double-spend 방지 | 공개 nullifier로 Note·Voucher 재사용을 막습니다. |
| Policy 통제 | 승인된 Circuit/VK를 허가된 scope에서만 사용합니다. |
| Status 집행 | Note·Voucher·Claim을 Active/Frozen/Revoked로 관리합니다. |
| 비공개 Audit | 암호화된 parents를 $K$명 협조로 복원해 Entry까지 역추적합니다. |
| DPP Claim | Final Note의 private State가 Issue Policy를 통과했음을 $h$로 DPP에 연결합니다. |

### 역할 이름 사용 원칙

공식 protocol/security 역할은 `Participant`, `Entry Issuer`,
`Policy Authority`, `Status Authority`, `Key-share Custodian`,
`DPP Verifier`, `Ledger/Router`로 한정합니다.

`Prover client`는 Participant가 proof를 생성할 때 사용하는 기능명이고,
`Audit client`는 Status Authority가 share를 검증·결합하고 graph를 복원할
때 사용하는 기능명입니다. `Factory`와 `Auditor`는 별도 신뢰 경계나
암호학적 권한을 가진 protocol role로 정의하지 않습니다.

---

## 6. Policy가 Circuit 사용을 어떻게 통제하는지 살펴봅시다

```text
policyRecords[policyRef]
  → 이 Policy는 어떤 Event·exact input/output arity용이고 어떤 VK를 사용하는가?

policyGrants[scopeRef][policyRef]
  → 이 scope가 이 Policy를 사용해도 되는가?
```

Policy Circuit은 계산식, threshold, 단위, rounding, 출력 역할을 고정합니다. Contract는 Policy가 enabled인지, Event가 맞는지, grant가 있는지, 등록된 VK로 proof가 통과하는지 확인합니다.

### Policy별 Process Circuit의 입력·출력 개수

현재 구조에서는 Policy마다 실제 입력 개수 $m$과 출력 개수 $n$을 고정한 Circuit을 별도로 Setup합니다.

```text
3-to-2 Process Policy
  → 입력 Note 3개
  → 출력 Note 2개
  → 전용 Circuit, ProvingKey, VerifyingKey

2-to-1 Process Policy
  → 입력 Note 2개
  → 출력 Note 1개
  → 별도 Circuit, ProvingKey, VerifyingKey
```

$m,n$은 transaction마다 Prover가 선택하는 값이 아니라 Policy와 Circuit Setup 시점에 고정됩니다. 따라서 inactive slot과 zero padding을 사용하지 않으며, arity가 달라지면 새로운 PolicyRef, ProvingKey와 VerifyingKey가 필요합니다.

현재 v1 상세 명세도 같은 exact arity 구조를 사용합니다.

```text
PolicyRef -> (eventKind, inputArity, outputArity, relation, VK)
```

추상 프로토콜은 `{C_(P,m,n)}` circuit family로 표현하지만, 실제 배포에서는
필요한 유한한 `(m,n)` 조합만 생성하고 등록합니다.

> **미확정:** Policy Authority가 새 Policy/VK를 승인할 때 요구할 registration
> request 형식과 근거 문서 범위는 아직 정하지 않았습니다.

> **미확정:** 공개 `policyRef`를 통해 output role과 Process 정보가 어디까지 노출되는지 정해야 합니다.

---

## 7. Status가 객체 사용을 어떻게 통제하는지 살펴봅시다

```text
Active  → 정상 사용 가능
Frozen  → 조사 중 임시 사용 금지
Revoked → 영구 사용 금지
```

소비 Circuit은 private `auditKey`와 status path로 현재 `statusRoot`에서 대상이 Active임을 증명합니다. Contract는 과거 root가 아니라 현재 root만 허용합니다.

```text
허용 전이
Active → Frozen
Frozen → Active
Frozen → Revoked
```

> **미확정:** StatusTree 자료구조, 온체인 저장 범위, Tree 유지 주체와 path 제공 방식은 아직 정하지 않았습니다.

---

## 8. Audit으로 객체의 생성 이력을 추적해 봅시다

현재 Note `D`의 과거를 찾는 흐름은 다음과 같습니다.

```text
cm_D
  → auditKey_D = H(NOTE, cm_D)
  → producerOf[auditKey_D] = processRecordId
  → auditRecords[processRecordId]
  → encryptedParents 복호화
  → Note A, B, C
```

`producerOf`는 객체를 만든 AuditRecord를 찾는 index입니다. `auditRecords`는 Event 종류, Policy, outputs, parent 수, 암호화된 parent 목록을 저장합니다.

```text
auditRecords[processRecordId]
  ├─ eventKind: Process
  ├─ policyRef
  ├─ outputRefs: NoteRef(cm_D), NoteRef(cm_W)
  ├─ parentCount: 3
  └─ encryptedParents:
       Encrypt(NoteRef(cm_A), NoteRef(cm_B), NoteRef(cm_C))
```

감사자는 복원한 A·B·C에 대해 같은 과정을 반복하여 Entry까지 backward tracing합니다.

---

## 9. 최종 Note와 DPP를 어떻게 연결하는지 살펴봅시다

v1은 별도의 `canonical DPP core` 객체를 새로 정의하지 않습니다.
기존 `DocumentInfo`와 그 `DocumentHash`를 Issue binding에 사용합니다.

```text
DPP에서 ProductName, LotID, Unit 추출
  → DocumentInfo 구성
  → d = DocumentHash 계산
  → Final Note의 private State를 Issue Policy로 검사
  → public Claim handle h 생성·등록
  → DPP 문서에 DPPClaim 추가
```

```text
DocumentInfo = (ProductName, LotID, Unit)
d = DocumentHash = HashDocumentInfo(DocumentInfo)
h = H("zkDPP:Issue:v1", d, issuePolicyRef, claimNonce)
```

```text
DPPClaim
  ├─ issuePolicyRef
  ├─ h
  └─ claimNonce
```

개념적인 DPP 구조는 다음과 같습니다.

```text
DPP {
  ProductName
  LotID
  Unit
  ...기타 DPP 정보...

  zkDPPClaim {
    issuePolicyRef
    h
    claimNonce
  }
}
```

Verifier는 DPP에서 `DocumentInfo`를 추출해 `d`와 `h`를 다시 계산하고,
`ClaimRef(h)`가 온체인에 등록됐으며 현재 Active인지 확인합니다. Claim은
owner와 nullifier를 가진 소유형 token이 아니라 공개 검증 식별정보입니다.

이 구조는 claim을 DPP에 기재된 제품·batch 설명에 binding하지만, 기타 DPP
field 전체의 무결성까지 보장하지 않습니다. 기타 field의 무결성은 DPP 서명
또는 VC 계층의 역할로 둡니다.

> **미확정:** 실제 DPP schema의 어떤 field를 `ProductName`, `LotID`, `Unit`에
> 대응시킬지와 DPP의 저장·배포 주체는 구현 profile에서 정해야 합니다.

---

## 10. 비용이 어디에서 발생하는지 살펴봅시다

### 하나의 Process가 만드는 비용을 따라가 봅시다

2장의 예시처럼 Note A·B·C를 소비해 Note D·W를 생성한다고 가정합니다.

```text
Process 실행 전: Policy Setup

3-to-2 Process Policy
  → 전용 Circuit 생성
  → ProvingKey 생성
  → VerifyingKey·Verifier 등록
```

Policy Setup은 매 transaction마다 수행하지 않습니다. 새로운 arity나 계산 규칙을 가진 Policy를 등록할 때 발생하는 일회성 비용입니다.

```text
1. Participant의 Prover client: off-chain proof 생성

입력 Note A·B·C
  ├─ Note Tree membership path 3개 검증
  ├─ Status path 3개 검증
  ├─ 질량·탄소·재활용 관계 계산
  └─ Parent A·B·C를 encryptedParents로 암호화
          ↓
       ZK proof 생성
```

이 구간은 proof를 생성하는 Participant가 부담합니다. 입력 수가 늘면 path 검증과 parent encryption이 늘어나므로 proof 생성시간과 메모리가 증가합니다. 이 비용 자체는 Contract gas가 아닙니다.

```text
2. Router Contract: on-chain 검증과 저장

ZK proof와 public input 수신
  → 등록된 VerifyingKey로 proof 검증
  → 입력 A·B·C의 nullifier 3개 저장
  → 출력 D·W를 Note Tree에 각각 append
  → D·W의 producerOf index 2개 저장
  → Process AuditRecord 1개와 encryptedParents 저장
```

이 구간은 transaction gas로 부담합니다. 입력 수는 nullifier 수를 늘리고, 출력 수는 Tree Update와 `producerOf` 수를 늘리며, ciphertext 크기는 AuditRecord 저장량을 늘립니다.

```text
3. Key-share Custodian: off-chain partial decryption

Status Authority가 감사 대상으로 선택한 encryptedParents
  → Custodian마다 자신의 key share로 partial decryption 생성
  → Partial-decryption share를 Status Authority의 Audit client에 전달
```

Custodian 한 명의 비용은 자신이 처리한 ciphertext 수에 따라 증가합니다. 감사에서 $V$개 record를 모두 처리한다면 Custodian 한 명은 최대 $V$개의 partial decryption을 계산합니다.

```text
4. Status Authority의 Audit client: off-chain graph 복원

문제 Note D
  → producerOf로 Process AuditRecord 조회
  → K명의 partial-decryption share 수집
  → Share 검증·결합
  → Parent A·B·C 복원
  → A·B·C의 AuditRecord를 같은 방식으로 반복 조회
```

Status Authority의 Audit client는 record 조회, share 요청·수집, share 검증·결합, parent graph 탐색을 부담합니다. 이 구간은 정상 transaction마다 발생하지 않고 감사할 때만 발생합니다.

전체 비용 흐름은 다음처럼 기억합니다.

```text
Policy 등록 시
  → Circuit·PK·VK 준비

정상 Event 시
  → Participant가 proof 생성 비용 부담
  → Contract가 verification·calldata·SSTORE gas 부담

문제 감사 시
  → 각 Custodian이 partial decryption 계산 부담
  → Status Authority의 Audit client가 share 수집·검증·결합과 graph 탐색 부담
```

### 비용 분석 방법

Big-O는 실제 gas 수치를 대신하지 않습니다. 이 문서에서는 어떤 입력이 증가할 때 어떤 연산과 storage write가 늘어나는지 설명하고, 실제 시간·gas 수치는 별도 benchmark에서 측정합니다.

### 주체별 측정 항목

| 측정 주체 | 실제로 측정할 값 |
|---|---|
| Participant의 Prover client | Constraint 수, witness 생성시간, proving time, peak memory, proof bytes |
| On-chain Verifier·Router | Verification gas, calldata bytes·gas, SSTORE 항목별 gas, transaction `gasUsed` |
| Key-share Custodian | Ciphertext당 partial-decryption 시간, share bytes, 처리 ciphertext 수 |
| Status Authority의 Audit client | 방문 AuditRecord 수, share 수집시간, share 검증·combine 시간, 전체 audit latency |

### 정상 Event에서 공통으로 발생하는 비용

| 언제 발생하는가? | 처리하는 객체·작업 | 부담 주체 | Big-O |
|---|---|---|---:|
| 기존 객체 $m$개를 소비할 때 | [Membership path 검증](#cost-proof-generation) | Participant의 Prover client | $O(mD)$ |
| 기존 객체 $m$개를 소비할 때 | Status path 검증 | Participant의 Prover client | StatusTree 구현 후 확정 |
| 기존 객체 $m$개를 소비할 때 | [Nullifier 저장](#cost-process-sstore) | Router Contract | $O(m)$ |
| 실제 parent $m$개를 기록할 때 | [Parent encryption](#cost-threshold-encryption) | Participant의 Prover client | Threshold Encryption 방식에 따라 결정 |
| Note·Voucher output $n$개를 만들 때 | [Tree Update](#cost-tree-update) | Router Contract | $O(nD)$ |
| Public outputRef $n$개를 만들 때 | [`producerOf` 저장](#cost-producer-of) | Router Contract | $O(n)$ |
| 모든 성공 Event | [AuditRecord 저장](#cost-audit-record) | Router Contract | $O(1+n+\lvert ct\rvert)$ |
| 모든 Event proof | [Proof verification](#cost-verification-gas) | On-chain Verifier | `PROFILE-PROOF` 결정 후 확정 |

### 특정 Event나 상황에서만 발생하는 비용

| 발생 시점 | 추가되는 작업 | 부담 주체 | Big-O |
|---|---|---|---:|
| Entry | 입력·nullifier 없이 Note Tree에 output 하나 추가 | Router Contract | $O(D)$ |
| Transfer | Voucher·Change Note Tree Update와 Recall deadline 저장 | Router Contract | $O(D)+O(1)$ |
| Proceed·Recall | Voucher nullifier 저장과 output Note Tree Update | Router Contract | $O(1)+O(D)$ |
| Exit | 입력 nullifier와 output 없는 AuditRecord 저장 | Router Contract | $O(1+\lvert ct\rvert)$ |
| Issue | `claimRegistered[h]`, `producerOf`, AuditRecord 저장 | Router Contract | $O(1+\lvert ct\rvert)$ |
| Frozen·Active·Revoked 변경 | [Status 자료구조 갱신](#cost-status-update) | Status Authority·Router | StatusTree 구현 후 확정 |
| Audit 실행 | [Ciphertext별 partial decryption](#cost-audit) | Custodian | 1명당 $O(V)$, $K$명은 $O(KV)$ |
| Audit 실행 | [Share 검증·결합과 graph 탐색](#cost-audit) | Status Authority의 Audit client | $O(KV)$ |
| 새 Policy 등록 | [Circuit Setup·PK·VK·Verifier 등록](#cost-policy-arity) | Policy Authority와 구현 담당자 | Policy 수 $P$에 대해 $O(P)$ |

$m$은 소비하는 입력 수, $n$은 생성하는 output 수, $D$는 Note·Voucher Tree depth, $\lvert ct\rvert$는 `encryptedParents` ciphertext의 storage 크기, $V$는 감사에서 방문하는 AuditRecord 수, $K$는 필요한 Custodian 수입니다.

---

## 11. 무엇을 신뢰하고 무엇을 보장하지 않는지 살펴봅시다

### 신뢰 가정

여기서 `신뢰한다`는 것은 단순히 정직할 것으로 기대한다는 뜻이 아닙니다. 해당 주체나 입력이 잘못되더라도 현재 Protocol이 그 오류를 암호학적으로 찾아내지 못한다는 뜻입니다.

| 신뢰 대상 | 신뢰하는 내용 | 이 가정이 깨지면 발생하는 문제 |
|---|---|---|
| `Entry Issuer` | 최초 Note의 값이 정확하고 같은 물품을 중복 발행하지 않습니다. | 잘못된 질량·credit·탄소가 정상 입력처럼 공급망에 들어갈 수 있습니다. |
| `Policy Authority` | 승인한 Policy와 VK가 의도한 현실 규칙을 올바르게 표현합니다. | ZK proof는 잘못 승인된 규칙을 정확히 실행했다는 사실만 증명하게 됩니다. |
| `Status Authority` | 정당한 대상을 감사하고 Frozen·Revoked 상태를 올바르게 적용합니다. | 정상 객체를 부당하게 동결하거나 문제 객체를 해제할 수 있습니다. |
| Ledger와 `Router Contract` | 등록된 규칙에 따라 transaction과 상태 변경을 정확히 실행합니다. | Nullifier, root, Policy, Status와 AuditRecord 집행을 신뢰할 수 없습니다. |
| 외부 운영 입력 | 운송·공정 배출량과 측정값이 정확합니다. | Circuit 계산은 맞아도 현실을 반영하지 않는 결과가 나올 수 있습니다. |
| Proof system·hash·encryption | 사용하는 암호 기술의 보안 가정이 성립합니다. | Proof 위조, commitment 연결, parent 정보의 비공개성을 보장할 수 없습니다. |

핵심 경계는 다음과 같습니다.

```text
현실 입력과 Policy가 타당한가?
  → 신뢰 기관과 외부 검증이 책임집니다.

주어진 입력으로 계산을 올바르게 수행했는가?
  → ZK Circuit이 검증합니다.

승인된 proof와 현재 상태만 허용했는가?
  → Router Contract가 집행합니다.
```

### v1이 보장하지 않는 범위

| 보장하지 않는 내용 | 의미 |
|---|---|
| 실세계 입력값의 진실성 | 센서·문서·담당자가 제출한 질량·배출량·재활용 입력이 현실과 같은지 검증하지 않습니다. |
| 물리 제품과 DPP의 binding | 특정 DPP 문서가 실제 제품이나 batch에 물리적으로 부착됐는지 검증하지 않습니다. |
| Material compatibility와 실제 재활용 함량 | 특정 원자재 조합이 현실 공정에 적합한지 또는 재활용 원료가 실제로 포함됐는지 확인하지 않습니다. |
| 인증제도·규제 전체 준수 | ISCC PLUS나 EU Battery Regulation 전체 준수를 주장하지 않습니다. |
| 모든 downstream descendant 자동 탐색 | 문제 ancestor에서 파생된 현재 leaf 전체를 자동으로 찾지 않습니다. 현재 Audit은 parent 방향의 backward tracing입니다. |
| 이미 소비된 객체의 rollback | 이미 완료된 과거 Event를 취소하거나 장부 상태를 되돌리지 않습니다. |
| $K$명 이상 Custodian의 공모 저항성 | Threshold 이상 위원이 공모해 ciphertext를 복호화하는 상황을 막지 않습니다. |
| 악의적인 Status Authority의 책임 증명 | Status Authority의 부당한 감사·동결을 암호학적으로 입증하지 않습니다. |
| 공개 Claim handle의 강한 접근 통제 | 공개된 $h$를 누가 열람하거나 복사할 수 있는지 통제하지 않습니다. 다른 `DocumentInfo`를 가진 DPP에 재사용하는 것은 binding 검증으로 막지만, 동일한 `DocumentInfo`의 복제나 물리 제품 binding은 막지 않습니다. |

---

## 부록 A. 성능 분석 상세

성능 비용은 정상 Event, 상태 변경, 감사의 세 구간에서 서로 다르게 발생합니다.

```text
정상 Event
  → ZK proof 생성
  → Contract proof 검증
  → Tree와 Event 상태 SSTORE

Status 변경
  → Frozen·Active·Revoked 상태 갱신

Audit
  → K명의 partial decryption과 parent graph 탐색
```

이번 절에서는 다음 기호를 사용합니다.

| 기호 | 의미 |
|---|---|
| $m$ | Event가 소비하는 입력 객체 수입니다. |
| $n$ | Event가 생성하는 출력 객체 수입니다. |
| $D$ | Note·Voucher Tree depth이며 현재 32입니다. |
| $D_{\mathrm{status}}$ | StatusTree depth이며 아직 미확정입니다. |
| $C_{\mathrm{Enc}}(m)$ | $m$개 parent를 Circuit에서 암호화하는 constraint 비용입니다. |
| $C_{\mathrm{TE}}(m)$ | $m$개 parent를 담은 ciphertext가 차지하는 storage word 수입니다. |
| $V$ | 한 번의 감사에서 방문하는 AuditRecord 수입니다. |
| $K$ | 복호화에 필요한 위원 수입니다. |

<a id="cost-threshold-encryption"></a>

### Threshold Encryption과 일반 Encryption의 비용 차이

항상 같다고 말할 수 없습니다. 구체적인 Threshold Encryption 방식은 아직 `PROFILE-TE`에서 정하지 않았기 때문입니다.
threshold의 추가 비용은 주로 복호화 단계에 나타납니다. 다만 현재 zkDPP는 Circuit 안에서 다음 관계까지 증명해야 합니다.

```text
encryptedParents
  = TE.Enc(committeePK, actualParentRefs, randomness)
```

Parent encoding, curve 연산, ciphertext 형식에 따라 Circuit constraint와 ciphertext 크기가 일반 encryption보다 커질 수 있습니다. 정확한 차이는 Threshold Encryption primitive를 선택한 뒤 측정해야 합니다.

대칭키 encryption과 비교하면 public-key 기반 Threshold Encryption은 같은 수준의 연산이라고 볼 수 없습니다. 이 문서에서 비교 가능한 대상은 같은 기반 primitive의 일반 public-key encryption입니다.

<a id="cost-proof-generation"></a>

### 정상 Event의 proof 생성 비용

이 비용은 **스마트 컨트랙트 gas가 아니라 Participant의 Prover client가 오프체인에서 부담하는 계산 비용**입니다. Circuit constraint가 늘어나면 proof 생성시간, CPU 사용량과 메모리가 증가합니다.

입력 객체 하나를 소비할 때 Note·Voucher membership path 하나와 Status path 하나를 검증합니다.

```text
입력 객체 1개
  ├─ Membership path: depth D
  └─ Status path: depth D_status
```

입력이 $m$개이면 Tree path 검증은 다음과 같이 증가합니다.

$$
O(mD)+O(mD_{\mathrm{status}})
$$

여기에 실제 parent $m$개를 암호화하는 비용과 Event별 계산 비용이 추가됩니다. Event 계산 constraint를 $C_{\mathrm{Event}}(m,n)$이라고 하면 전체 구조는 다음과 같습니다.

$$
O(mD+mD_{\mathrm{status}})
+C_{\mathrm{Enc}}(m)
+C_{\mathrm{Event}}(m,n)
$$

Policy Circuit은 실제 $m$-to-$n$ 구조로 Setup하므로 사용하지 않는 input·output slot의 path와 encryption 비용은 포함되지 않습니다.

Membership path와 Status path는 private witness이므로 path 자체를 calldata로 보내지 않습니다. 따라서 Tree depth가 커져 proof 생성이 느려지더라도 같은 비율로 온체인 gas가 증가하는 것은 아닙니다.

<a id="cost-verification-gas"></a>

### Contract의 proof verification gas

Contract는 Prover가 만든 proof를 등록된 VerifyingKey로 검증합니다.

```text
오프체인
  → Witness 계산
  → ZK proof 생성

온체인
  → Proof와 public input 제출
  → Verifier 실행
```

Verification gas는 Circuit constraint 수와 단순 비례하지 않습니다. Proof system, public input 수, proof 크기와 verifier 구현에 따라 결정됩니다.

입력·출력 수가 늘어 public input이나 calldata가 증가하면 gas도 증가할 수 있지만, Membership·Status path처럼 private witness에서만 사용하는 계산은 주로 오프체인 proving 비용에 영향을 줍니다.

따라서 정상 Event의 온체인 gas는 다음 항목을 별도로 봐야 합니다.

```text
Proof verification gas
Public input·ciphertext calldata gas
Tree Update SSTORE
Nullifier·producerOf·AuditRecord SSTORE
```

<a id="cost-partial-decryption"></a>

### Partial Decryption과 온체인 gas

현재 명세에서는 들지 않습니다. 다음 작업은 모두 off-chain에서 진행합니다.

```text
Key-share Custodian
  → partial decryption 생성

Status Authority
  → share 검증
  → K개 share 결합
  → parent AuditRefs 복원
```

Partial decryption, share 전달, share 검증, combine 결과는 Contract에 제출하지 않습니다. 온체인 AuditCase 객체도 만들지 않습니다.

다만 정상 Event가 `encryptedParents`를 calldata로 제출하고 `auditRecords`에 저장하므로 **ciphertext 생성과 저장 비용**은 정상 transaction에 포함됩니다.

```text
오프체인 Audit 비용
  → partial decryption과 combine

온체인 정상 Event 비용
  → ciphertext calldata와 AuditRecord SSTORE
```

<a id="cost-process-sstore"></a>

### 정상 Process Event의 SSTORE

$m$개의 Note를 소비하고 $n$개의 Note를 생성하는 Process를 기준으로 봅니다.

#### 1. 입력별 Nullifier 저장

```text
noteNullifiers[nf_1] = true
...
noteNullifiers[nf_m] = true
```

입력 수에 비례하므로 다음 비용이 발생합니다.

$$
O(m)
$$

<a id="cost-tree-update"></a>

#### 2. 출력별 Note Tree 갱신

> **합의 — v1 프로토타입 membership profile:** Note와 Voucher는
> `poc-v2`의 depth-32 온체인 append-only Tree를 재사용합니다.
> Contract가 Tree node를 직접 저장하므로 외부 Indexer는 필수가 아니며,
> 있다면 path 조회 성능을 높이는 선택적 구성요소로만 사용합니다.

Output 하나를 append할 때 leaf부터 depth 32의 root까지 node를 갱신하고 다음 상태도 기록합니다.

```text
Tree path nodes
leafCount
currentRoot
acceptedRoots[newRoot]
```

입력 membership은 `acceptedRoots`에 등록된 과거 root로도 증명할 수
있습니다. Append-only Tree에서 과거 inclusion 자체는 유효하므로, Contract가
현재 Note nullifier 또는 Voucher resolution 상태를 같은 transaction에서
원자적으로 검사하면 재사용은 막힌다고 봅니다. 이 규칙은 StatusTree에
재사용하지 않으며, status 증명은 항상 Contract의 최신
`statusRoot`를 사용해야 합니다.

출력 $n$개를 append하는 Tree Update 비용은 다음과 같습니다.

$$
O(nD)
$$

$D=32$가 고정되어 있어도 output 하나마다 여러 SSTORE가 발생한다는 실질적인 비용은 남습니다.

<a id="cost-producer-of"></a>

#### 3. 출력별 producerOf index 저장

```text
producerOf[auditKey(output_1)] = processRecordId
...
producerOf[auditKey(output_n)] = processRecordId
```

따라서 다음 비용이 발생합니다.

$$
O(n)
$$

<a id="cost-audit-record"></a>

#### 4. AuditRecord 저장

모든 성공 Event는 AuditRecord 하나를 저장합니다.

```text
auditRecords[processRecordId]
  ├─ eventKind
  ├─ policyRef
  ├─ outputRefs                 n개
  ├─ parentCount
  └─ encryptedParents          C_TE(m) words
```

고정 field는 $O(1)$이고, outputRefs는 $O(n)$이며, ciphertext 저장량은 $O(C_{\mathrm{TE}}(m))$입니다.

$$
O(1+n+C_{\mathrm{TE}}(m))
$$

현재 명세의 AuditRecord는 단순한 EVM event log가 아니라 `auditRecords[auditRecordId]`로 조회하는 온체인 storage입니다. 별도의 Solidity event를 추가한다면 LOG gas는 발생하지만 SSTORE와는 다른 비용입니다.

#### 5. Event별 추가 상태 저장

| Event | Tree 외 추가 저장 |
|---|---|
| Note 소비 Event | 입력마다 `noteNullifiers[nf]` |
| Proceed·Recall | `voucherNullifiers[rvnf]` |
| Issue | `claimRegistered[h]` |
| Transfer | Recall deadline 또는 resolution metadata |
| 모든 output Event | Output마다 `producerOf[auditKey]` |
| 모든 성공 Event | AuditRecord 1개 |

PolicyRecord와 PolicyGrant는 Policy 등록·권한 변경 시 저장하며, 정상 Event마다 새로 SSTORE하지 않습니다. 정상 Event에서는 기존 값을 읽어 사용 가능 여부를 확인합니다.

<a id="cost-total-sstore"></a>

### Event당 온체인 저장 비용

Process Event의 주요 SSTORE 증가량을 합치면 다음과 같습니다.

$$
O(nD)+O(m)+O(n)+O(C_{\mathrm{TE}}(m))
$$

단순화하면 다음과 같습니다.

$$
O(nD+m+n+C_{\mathrm{TE}}(m))
$$

이 식은 gas 수치가 아니라 **어떤 값이 증가할 때 storage write가 늘어나는지**를 나타냅니다. 실제 gas는 새 slot 기록인지 기존 slot 갱신인지, field packing과 ciphertext encoding에 따라 달라집니다.

<a id="cost-status-update"></a>

### Active·Frozen·Revoked 변경 비용

일반 Event는 객체의 Status를 변경하지 않습니다. Circuit이 현재 `statusRoot`에서 private input이 Active임을 증명하고, Contract는 current root와 같은지만 확인합니다.

```text
일반 Event
  → Status SSTORE 없음
  → current statusRoot 확인
```

Status SSTORE는 Status Authority가 상태를 바꿀 때 발생합니다.

```text
Active → Frozen
Frozen → Active
Frozen → Revoked
```

현재 설계에서 entry가 없으면 Active이므로 새 객체를 만들 때 Active 상태를 별도 SSTORE하지 않습니다.

StatusTree의 구체적인 구현 방식은 아직 정하지 않았습니다. 따라서 정확한 Big-O도 아직 확정할 수 없습니다.

```text
일반 mapping을 사용한다면
  → 상태값 갱신은 O(1) SSTORE

온체인 Merkle Tree node를 직접 갱신한다면
  → depth를 D_status라고 할 때 O(D_status)
```

Private Active proof를 지원해야 하므로 단순 mapping만으로 충분한지는 별도 설계가 필요합니다.

<a id="cost-audit"></a>

### Audit 비용

한 AuditRecord를 복호화하려면 $K$명의 partial-decryption share가 필요합니다. 감사에서 $V$개의 AuditRecord를 방문한다고 가정합니다.

#### Key-share Custodian

감사에 참여하는 각 Custodian이 방문 record의 ciphertext를 모두 처리하면, Custodian 한 명은 $V$개의 partial decryption을 계산합니다.

$$
O(V)\quad\text{per Custodian}
$$

$K$명이 만든 전체 share 수와 위원회 전체 계산량은 다음과 같습니다.

$$
K\times V
$$

$$
O(KV)\quad\text{for all participating Custodians}
$$

#### Status Authority의 Audit client

Audit client는 $V$개의 AuditRecord를 조회하고, 각 record마다 $K$개의 share를 수집·검증·결합합니다.

$$
O(V)\quad\text{record lookup}
$$

$$
O(KV)\quad\text{share verification and combine}
$$

따라서 Status Authority 측 주요 계산·통신량도 $O(KV)$로 요약할 수 있습니다. 이 비용은 off-chain 비용이며 온체인 gas가 아닙니다. Provenance graph가 깊거나 parent가 많아 방문 record 수 $V$가 커질수록 감사 시간이 증가합니다.

<a id="cost-policy-arity"></a>

### Policy별 exact arity의 비용 영향

Policy Circuit은 실제 $m$-to-$n$ arity로 Setup하므로 inactive slot과 padding proof 비용은 없습니다.

대신 arity 또는 계산 관계가 다르면 별도 Policy가 필요합니다.

```text
Policy 수 증가
  → Circuit Setup 증가
  → ProvingKey·VerifyingKey 증가
  → Verifier와 Registry 관리 증가
```
