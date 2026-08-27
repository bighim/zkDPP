# zkDPP v1 구현 명세 — 쉽게 읽는 한국어 해설판

> 원문: [`zkdpp-v1-implementation-spec.md`](./zkdpp-v1-implementation-spec.md)  
> 원문 상태: normative draft, 2026-08-25  
> 이 문서의 역할: 원문을 이해하기 위한 해설판이며, 원문을 대체하지 않습니다.

## 이 문서를 읽는 방법

이 문서는 객체 이름과 수식을 먼저 외우게 하지 않습니다. 정상적인 공급망 흐름을 먼저 설명하고, 변수를 실제로 사용하는 절에서 바로 정의합니다.

필요한 목적에 따라 다음 순서로 읽으면 됩니다.

| 읽는 목적 | 먼저 읽을 절 |
|---|---|
| 전체 개념 이해 | §1 → §3~§6 → §8 |
| Policy와 상태 집행 이해 | §9 → §10 → §13 |
| 비공개 provenance 감사 이해 | §6 → §11 → §12 → §15 |
| 구현과 테스트 준비 | §13 → §16~§19 |

요구사항 표현은 다음과 같습니다.

| 표현 | 의미 |
|---|---|
| **필수(MUST)** | 구현이 반드시 지켜야 합니다. |
| **권장(SHOULD)** | 정당한 이유가 없으면 지켜야 합니다. |
| **선택(MAY)** | 구현 Profile이 선택할 수 있습니다. |
| **PROFILE** | Circuit과 key를 만들기 전에 구체값을 정해야 합니다. |
| **명세 확인 필요** | 원문만으로 의미가 충분히 정해지지 않았습니다. |

---

## 1. 한눈에 보는 zkDPP v1

### 1.1 정상적인 공급망 흐름

```text
Entry Issuer가 최초 원자재 Note 생성
  → Operator가 Transfer·Merge·Split·Process 수행
  → 최종 Note에서 Exit 또는 Issue 수행
  → Issue를 선택하면 DPP Claim 식별번호 h 등록
```

각 물품은 공개 ID만 온체인에 남기고, 자세한 질량·탄소·재활용 값과 소비한 물품의 연결은 숨깁니다.

### 1.2 문제가 생겼을 때의 흐름

```text
문제 객체 발견
  → Status Authority가 Frozen 처리
  → 객체를 만든 AuditRecord 조회
  → K명의 위원이 encryptedParents 복호화 지원
  → 이전 객체를 Entry까지 역추적
  → 문제없으면 Active, 문제면 Revoked
```

### 1.3 핵심 객체 네 개

| 객체 | 쉬운 의미 | 공개 식별값 |
|---|---|---|
| `Note` | 현재 사용할 수 있는 물품 상태입니다. | `cm` |
| `Voucher` | Proceed 또는 Recall을 기다리는 미확정 Transfer입니다. | `rv` |
| `Claim` | DPP가 Issue Policy를 통과했다는 공개 인증 식별정보입니다. | `h` |
| `AuditRecord` | 객체를 만든 Event와 암호화된 parent를 기록합니다. | `auditRecordId` |

### 1.4 이 시스템이 보장하는 것

구현은 신뢰된 Entry 이후 다음을 보장해야 합니다.

- 질량, 재활용 credit, 탄소 계산의 일관성을 검증합니다.
- 상세 State와 소비 객체의 연결을 일반 관찰자에게 숨깁니다.
- Note와 Voucher의 이중 소비를 막습니다.
- 승인된 Policy와 VK만 허용된 범위에서 사용하게 합니다.
- 최종 Claim을 특정 DPP 문서에 연결합니다.
- 모든 성공 Event에 실제 parent와 binding된 암호문을 남깁니다.
- 신뢰된 Status Authority가 객체를 동결하거나 철회할 수 있습니다.
- $K$-out-of-$N$ 협조로 과거 parent graph를 역추적할 수 있습니다.

### 1.5 이 시스템이 보장하지 않는 것

다음은 v1의 암호학적 보장이 아닙니다.

- 현실에서 입력한 질량·탄소·재활용 값이 진실인지 확인하지 않습니다.
- DPP 문서가 실제 물리 제품이나 batch와 연결됐는지 확인하지 않습니다.
- 물질 종류의 호환성이나 실제 재활용 함량을 확인하지 않습니다.
- 특정 인증제도나 규제 전체 준수를 주장하지 않습니다.
- 문제 ancestor의 모든 downstream 객체를 자동 탐색하지 않습니다.
- 이미 소비된 객체를 과거 상태로 되돌리지 않습니다.
- $K$명 이상의 복호화 위원이 공모하는 상황을 막지 않습니다.
- 악의적인 Status Authority의 권한 남용을 암호학적으로 증명하지 않습니다.
- 공개 Claim 식별번호에 강한 접근 통제를 제공하지 않습니다.

원문 대응: §1, §2

---

## 2. 참여자와 신뢰 경계

### 2.1 누가 무엇을 합니까?

| 원문 역할 | 쉬운 이름 | 책임 |
|---|---|---|
| `Entry Issuer` | 최초 등록기관 | 최초 `ELIGIBLE` Note를 신뢰하여 등록합니다. |
| `Policy Authority` | 정책 승인기관 | Policy, VK, 사용 범위를 승인하고 비활성화합니다. |
| `Operator` | 공급망 참여자 | private witness로 Event proof를 만듭니다. |
| `Status Authority` | 감사·상태 관리기관 | 객체를 동결·철회하고 감사 graph를 복원합니다. |
| `Key-share Custodian` | 복호화 지분 보관자 | 특정 ciphertext용 partial decryption을 만듭니다. |
| `DPP Verifier` | DPP 검증자 | Claim이 DPP에 맞게 연결됐고 Active인지 확인합니다. |
| `Router Contract` | 온체인 집행자 | proof, Policy, 이중 소비, 상태 전이를 검사합니다. |

### 2.2 누구의 공격을 막습니까?

모든 일반 Operator, prover, transaction 제출자, DPP publisher가 함께 공모할 수 있다고 가정합니다. 복호화 위원은 $K$명 미만까지 부패할 수 있다고 가정합니다.

구현은 다음 공격을 막아야 합니다.

- 존재하지 않거나 소유하지 않은 private 객체 소비
- 같은 Note 또는 Voucher의 중복 소비
- 승인되지 않은 Policy 또는 사용 범위 선택
- Event 계산을 위반한 출력 생성
- 실제 parent와 다른 `encryptedParents` 등록
- Frozen 또는 Revoked 객체 사용
- 과거 `Active` root를 재사용한 동결 우회
- 다른 DPP에 유효 Claim 복사

### 2.3 누구를 신뢰합니까?

`신뢰한다`는 말은 해당 주체가 거짓말하면 현재 프로토콜이 잡아내지 못한다는 뜻입니다.

- Entry Issuer가 최초 객체를 정확히 발행하고 중복 발행하지 않는다고 믿습니다.
- Policy Authority가 현실에 적합한 Policy를 승인한다고 믿습니다.
- Status Authority가 정당한 감사와 상태 변경을 수행한다고 믿습니다.
- Ledger와 Router Contract가 명세대로 실행된다고 믿습니다.
- 운송·공정 배출량 같은 외부 운영 입력이 정확하다고 믿습니다.
- proof system, hash, threshold encryption이 안전하다고 가정합니다.

원문 대응: §3

---

## 3. Note: 사용할 수 있는 비공개 물품 상태

### 3.1 이번 절에서 사용하는 값

| 변수 | 의미 |
|---|---|
| `DocumentHash` | 물품 설명 또는 연결 문서의 hash입니다. |
| `assetRole` | `ELIGIBLE` 또는 `WASTE` 역할입니다. |
| $q_{\mathrm{mass}}$ | 물품의 전체 질량입니다. |
| $a_{\mathrm{rec}}$ | 귀속된 재활용 원료 질량 또는 credit입니다. |
| $e$ | Entry 이후 누적된 탄소발자국 절대량입니다. |
| `owner` | Note를 소비할 권한을 가진 소유자입니다. |
| `opening` | Commitment를 숨기기 위한 randomness입니다. |
| `cm` | 위 내용을 묶은 Note의 공개 식별값입니다. |

### 3.2 Note 안에는 무엇이 들어갑니까?

Note의 실제 내용은 private입니다.

$$
\mathrm{NoteOpening}=
(\mathrm{DocumentHash},\mathrm{assetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{owner},\mathrm{opening})
$$

온체인에는 실제 내용 대신 commitment `cm`이 나타납니다.

$$
cm=
H(\mathrm{DOMAIN}_{\mathrm{Note}},\mathrm{DocumentHash},\mathrm{assetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{owner},\mathrm{opening})
$$

정확한 domain tag는 `zkDPP:Note:v1`입니다.

### 3.3 Note가 항상 만족해야 하는 조건

$q_{\mathrm{mass}}$와 $a_{\mathrm{rec}}$은 같은 질량 단위와 scale을 사용해야 합니다. 모든 live Note는 다음 조건을 만족해야 합니다.

$$
0\leq a_{\mathrm{rec}}\leq q_{\mathrm{mass}}
$$

`WASTE` Note는 다음 조건을 만족해야 합니다.

$$
a_{\mathrm{rec}}=0,\qquad e=0
$$

이는 폐기물에 재활용 credit이나 탄소를 배분하지 않는 v1 Policy입니다.

### 3.4 AssetRole은 물질 종류가 아닙니다

| 역할 | 의미 |
|---|---|
| `ELIGIBLE` | 후속 Event와 Sustainability Claim에 사용할 수 있습니다. |
| `WASTE` | `Exit`만 수행할 수 있습니다. |

`assetRole`은 private field이며, 알루미늄·플라스틱 같은 material identity를 나타내지 않습니다.

원문 대응: §4.2, §5.1

---

## 4. Voucher: 아직 확정되지 않은 Transfer

### 4.1 이번 절에서 사용하는 값

| 변수 | 의미 |
|---|---|
| `rv` | Voucher의 공개 commitment입니다. |
| `senderOwner` | 물품을 보낸 소유자입니다. |
| `receiverOwner` | 물품을 받을 소유자입니다. |
| `deltaEpoch` | Sender가 Recall할 수 있는 기간입니다. |
| `rvnf` | Voucher가 Proceed 또는 Recall로 해결됐음을 나타냅니다. |

Voucher는 다음 private payload를 commitment로 묶습니다.

$$
\mathrm{VoucherOpening}=
(\mathrm{DocumentHash},\mathrm{assetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,
\mathrm{senderOwner},\mathrm{receiverOwner},\mathrm{deltaEpoch},\mathrm{opening})
$$

정확한 commitment domain tag는 `zkDPP:Voucher:v1`입니다.

### 4.2 Voucher의 생애주기

```text
Sender Note
  → Transfer
  ├─ Sender의 Change Note
  └─ Receiver용 Voucher
       ├─ Proceed → Receiver Note
       └─ Recall  → Sender 반환 Note
```

Transfer는 `ELIGIBLE` Voucher만 생성합니다. Proceed와 Recall은 Voucher의 `DocumentHash`, 역할, 질량, 재활용 credit, 탄소를 변경하지 않고 Note로 옮겨야 합니다.

Deadline은 Voucher 전체의 만료가 아니라 Sender의 Recall 권한 종료 시점입니다. Deadline 전에는 Proceed와 Recall이 경쟁하고, 한쪽이 `rvnf`를 먼저 기록하면 다른 쪽은 실패합니다. Deadline 이후에는 Proceed만 허용됩니다.

원문 대응: §5.2, §11.2~§11.4

---

## 5. Claim과 DPP: 최종 검증 결과를 공개하는 방법

### 5.1 이번 절에서 사용하는 값

| 변수 | 의미 |
|---|---|
| $d$ | zkDPP Claim 항목을 제외한 DPP core의 hash입니다. |
| `issuePolicyRef` | 어떤 Issue Policy를 통과했는지 나타냅니다. |
| `claimNonce` | Claim 인스턴스를 구분하는 공개 난수입니다. |
| $h$ | 온체인에 등록되는 공개 Claim 식별번호입니다. |

### 5.2 Issue가 만드는 것

Issue는 final Note를 한 번 소비하고 후속 Note 대신 `ClaimRef(h)`를 생성합니다.

```text
Final Note
  → Issue Policy 검사
  → Final Note nullifier 기록
  → ClaimRef(h) 등록
```

DPP core와 Claim 식별번호는 다음처럼 계산합니다.

$$
d=H(\text{canonical DPP core without the zkDPP Claim field})
$$

$$
h=H(\mathrm{DOMAIN}_{\mathrm{Issue}},d,\mathrm{issuePolicyRef},\mathrm{claimNonce})
$$

정확한 Issue domain tag는 `zkDPP:Issue:v1`입니다.

DPP 문서 안에는 다음 공개 항목이 들어갑니다.

$$
\mathrm{DPPClaim}=(\mathrm{issuePolicyRef},h,\mathrm{claimNonce})
$$

`claimNonce`는 공개 난수일 뿐 비밀번호나 access token이 아닙니다.

### 5.3 Sustainability State는 어디에 있습니까?

정확한 $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$는 final Note 안에 private witness로 남습니다. Issue Circuit은 이 값이 Policy 기준을 통과했는지만 증명합니다. DPP Claim 항목에는 정확한 State가 들어가지 않습니다.

### 5.4 명세 확인 필요: DocumentHash와 DPP core

원문은 Issue Circuit이 다음 조건을 증명하도록 요구합니다.

$$
\mathrm{FinalNote.DocumentHash}=d
$$

하지만 이전 설계에서는 `DocumentHash`를 제품명·Lot ID·단위 같은 `DocumentInfo`의 hash로 설명했습니다. 현재 원문은 `DocumentInfo`와 DPP core가 같은 객체인지, final Note의 `DocumentHash`를 언제 DPP core hash로 설정하는지 정의하지 않습니다.

따라서 구현 전에 다음 중 어떤 의미인지 결정해야 합니다.

- final Note의 `DocumentHash`가 DPP core 전체를 hash합니다.
- DPP core가 기존 `DocumentInfo`와 동일합니다.
- 두 hash는 별도이며 Issue proof가 다른 binding relation을 사용합니다.

이 해설판은 이 문제를 임의로 결정하지 않습니다.

원문 대응: §5.3, §11.9, §12, §14 `PROFILE-DPP`

---

## 6. AuditRef와 Nullifier: 객체의 종류와 소비 여부 구분

### 6.1 AuditRef는 새 물품이 아닙니다

AuditRef는 감사할 객체를 가리키는 typed reference입니다.

$$
\mathrm{AuditRef}
\in
\{\mathrm{NoteRef}(cm),\mathrm{VoucherRef}(rv),\mathrm{ClaimRef}(h)\}
$$

이번 절에서 사용하는 값은 다음과 같습니다.

| 변수 | 의미 |
|---|---|
| `typeTag` | Note·Voucher·Claim 중 어떤 종류인지 표시합니다. |
| `rawID` | 실제 ID인 `cm`, `rv`, `h` 중 하나입니다. |
| `auditKey` | 종류와 ID를 하나로 묶은 감사용 key입니다. |

$$
\mathrm{auditKey}=H(\mathrm{DOMAIN}_{\mathrm{AuditRef}},\mathrm{typeTag},\mathrm{rawID})
$$

정확한 domain tag는 `zkDPP:AuditRef:v1`입니다.

예를 들어 `NoteRef(123)`과 `ClaimRef(123)`은 rawID가 같더라도 type이 다르므로 서로 다른 auditKey를 가져야 합니다. `typeTag`와 `rawID`의 고정 byte encoding은 `PROFILE-AUDIT-ENC`에서 정합니다.

### 6.2 Nullifier는 사용 완료 표시입니다

| 값 | 의미 |
|---|---|
| `nf` | Note가 이미 소비됐음을 나타냅니다. |
| `rvnf` | Voucher가 Proceed 또는 Recall로 해결됐음을 나타냅니다. |

Note와 Voucher는 서로 다른 nullifier domain을 사용해야 합니다. Nullifier는 소비 객체와 owner secret에 binding돼야 합니다.

Nullifier는 public이지만, 소비된 `cm` 또는 `rv` 자체는 private witness여야 합니다. 따라서 외부 관찰자는 중복 소비를 감지하면서도 어떤 공개 commitment가 소비됐는지 바로 연결할 수 없어야 합니다.

원문 대응: §4.3, §5.4, §5.5

---

## 7. Contract가 기억하는 온체인 상태

온체인 상태는 사용 목적별로 보면 쉽습니다.

### 7.1 객체의 존재와 중복 소비

| 상태 | 관리하는 것 |
|---|---|
| `noteRoot` | 등록된 Note commitment의 membership root입니다. |
| `voucherRoot` | 등록된 Voucher commitment의 membership root입니다. |
| `noteNullifiers[nf]` | Note가 이미 소비됐는지 기록합니다. |
| `voucherNullifiers[rvnf]` | Voucher가 이미 해결됐는지 기록합니다. |
| `claimRegistered[h]` | Claim 식별번호가 Issue로 발행됐는지 기록합니다. |

Note와 Voucher membership 자료구조의 구체적인 layout은 `PROFILE-MEMBERSHIP`에서 정합니다. Contract는 proof가 사용한 root를 허용할 수 있는지 검증해야 합니다.

### 7.2 동결과 철회

| 상태 | 관리하는 것 |
|---|---|
| `statusRoot` | Note·Voucher·Claim의 Active/Frozen/Revoked 예외 상태를 요약합니다. |

Status proof에는 과거 root가 아니라 현재 `statusRoot`만 사용할 수 있습니다.

### 7.3 감사 이력

| 상태 | 관리하는 것 |
|---|---|
| `producerOf[auditKey]` | 해당 객체를 만든 AuditRecord ID를 가리킵니다. |
| `auditRecords[auditRecordId]` | Event, Policy, output, 암호화된 parent를 저장합니다. |

### 7.4 Policy 사용 권한

| 상태 | 관리하는 것 |
|---|---|
| `policyRecords[policyRef]` | 승인된 Policy의 Event 종류, VK, 활성 상태를 기록합니다. |
| `policyGrants[policyScopeRef][policyRef]` | 특정 scope에서 Policy 사용이 허용됐는지 기록합니다. |

원문 대응: §6

---

## 8. 정상 Event 흐름

구현은 다음 아홉 Event를 구분해야 합니다.

```text
Entry, Transfer, Proceed, Recall, Merge,
Split, Process, Exit, Issue
```

Event의 정수 encoding은 `PROFILE-EVENT`에서 고정하고 Circuit, Registry, Contract가 같은 값을 사용해야 합니다.

### 8.1 Entry: 최초 Note 등록

원문 relation은 다음과 같습니다.

$$
\varnothing\longrightarrow \mathrm{Note}(\mathrm{ELIGIBLE},q_{\mathrm{mass}},a_{\mathrm{rec}},e=0)
$$

이번 Event에서 사용하는 값은 다음과 같습니다.

| 변수 | 의미 |
|---|---|
| $q_{\mathrm{mass}}$ | 최초 물품 질량입니다. |
| $a_{\mathrm{rec}}$ | 최초 재활용 credit입니다. |
| $e$ | Entry 시점 누적 탄소이며 0이어야 합니다. |

Circuit은 다음 조건을 검사해야 합니다.

$$
q_{\mathrm{mass}}>0
$$

$$
0\leq a_{\mathrm{rec}}\leq q_{\mathrm{mass}}
$$

$$
e=0,\qquad \mathrm{assetRole}=\mathrm{ELIGIBLE}
$$

Contract는 호출자가 신뢰된 Entry Issuer인지 확인해야 합니다. Entry에는 parent가 없으므로 암호화된 parent payload도 빈 목록에 binding돼야 합니다.

### 8.2 Transfer: 일부를 보내고 Voucher 생성

```text
Sender Note
  → Receiver용 Voucher
  → Sender의 Change Note
```

이번 Event에서 사용하는 첨자는 다음과 같습니다.

| 첨자 | 의미 |
|---|---|
| `in` | 입력 Note입니다. |
| `send` | Receiver에게 보내는 Voucher 부분입니다. 원문의 `T`에 해당합니다. |
| `change` | Sender에게 남는 Note 부분입니다. 원문의 `C`에 해당합니다. |

질량은 정확히 나뉘어야 합니다.

$$
q_{\mathrm{in}}=q_{\mathrm{send}}+q_{\mathrm{change}}
$$

$$
q_{\mathrm{send}}>0,\qquad q_{\mathrm{change}}\geq0
$$

재활용 credit은 Change 쪽을 floor로 계산하고 나머지를 Voucher에 줍니다.

$$
a_{\mathrm{change}}
=
\left\lfloor
\frac{a_{\mathrm{in}}q_{\mathrm{change}}}{q_{\mathrm{in}}}
\right\rfloor
$$

$$
a_{\mathrm{send}}=a_{\mathrm{in}}-a_{\mathrm{change}}
$$

탄소도 같은 방식으로 나눈 뒤 운송 탄소를 Voucher에 추가합니다.

$$
e_{\mathrm{change}}
=
\left\lfloor
\frac{e_{\mathrm{in}}q_{\mathrm{change}}}{q_{\mathrm{in}}}
\right\rfloor
$$

$$
e_{\mathrm{send}}
=
e_{\mathrm{in}}-e_{\mathrm{change}}+\Delta e_{\mathrm{transport}}
$$

$$
\Delta e_{\mathrm{transport}}\geq0
$$

Voucher와 Change Note는 입력의 `DocumentHash`와 `ELIGIBLE` 역할을 보존해야 합니다. 전량 Transfer에서도 값이 0인 Change Note를 생성해야 합니다.

### 8.3 Proceed와 Recall: Voucher 해결

Proceed는 Receiver가 unresolved Voucher를 소비해 Receiver Note로 확정합니다. Recall은 Sender가 같은 Voucher를 소비해 Sender Note로 되돌립니다.

두 Event 모두 Voucher의 다음 값을 그대로 보존해야 합니다.

- `DocumentHash`
- `assetRole`
- $q_{\mathrm{mass}}$
- $a_{\mathrm{rec}}$
- $e$

Proceed 또는 Recall이 먼저 성공하면 `rvnf`가 기록되므로 다른 Event는 같은 Voucher를 다시 해결할 수 없습니다.

Recall은 설정된 deadline 이전에만 가능합니다. Deadline 이후 unresolved Voucher에는 Proceed만 허용됩니다.

### 8.4 Merge: 같은 DocumentHash의 두 Note 합치기

Merge는 같은 owner와 `DocumentHash`를 가진 두 `ELIGIBLE` Note만 합칩니다.

$$
q_{\mathrm{out}}=q_1+q_2
$$

$$
a_{\mathrm{out}}=a_1+a_2
$$

$$
e_{\mathrm{out}}=e_1+e_2
$$

`DocumentHash`가 다른 입력은 Merge가 아니라 Process를 사용해야 합니다.

### 8.5 Split: Note 하나를 두 개로 나누기

Split은 하나의 `ELIGIBLE` Note를 같은 owner와 `DocumentHash`를 가진 두 Note로 나눕니다.

$$
q_{\mathrm{in}}=q_1+q_2
$$

Output 2의 재활용 credit과 탄소를 floor로 계산합니다.

$$
a_2=
\left\lfloor
\frac{a_{\mathrm{in}}q_2}{q_{\mathrm{in}}}
\right\rfloor,
\qquad
e_2=
\left\lfloor
\frac{e_{\mathrm{in}}q_2}{q_{\mathrm{in}}}
\right\rfloor
$$

나머지를 Output 1에 줍니다.

$$
a_1=a_{\mathrm{in}}-a_2,
\qquad
e_1=e_{\mathrm{in}}-e_2
$$

질량 0 output도 허용하지만 그 output의 $a_{\mathrm{rec}}$와 $e$는 0이어야 합니다.

### 8.6 Process: 여러 입력을 주제품과 폐기물로 변환

Process는 $m$개의 입력과 $n$개의 출력을 처리합니다. Profile은 최대 입력 수 $M_{\max}$와 최대 출력 수 $N_{\max}$를 고정해야 합니다.

$$
1\leq m\leq M_{\max},
\qquad
1\leq n\leq N_{\max}
$$

사용하지 않는 slot은 Profile이 정한 canonical zero padding을 사용합니다. 모든 active input은 `ELIGIBLE`이어야 합니다.

Policy는 active output 중 최소 하나를 `ELIGIBLE`로 미리 고정해야 합니다. Prover가 output 값을 본 후 자신에게 유리하게 `WASTE` 역할을 선택하면 안 됩니다. 모든 것을 폐기하려면 Process가 아니라 Exit를 사용합니다.

#### 질량

이번 관계에서 사용하는 값은 다음과 같습니다.

| 변수 | 의미 |
|---|---|
| $Q_{\mathrm{in}}$ | 모든 active input 질량의 합입니다. |
| $Q_{\mathrm{out}}$ | 모든 주제품·폐기물 output 질량의 합입니다. |
| $q_{\mathrm{loss}}$ | 공정 중 사라진 질량입니다. |

$$
Q_{\mathrm{in}}=\sum q_{\mathrm{input}}
$$

$$
Q_{\mathrm{out}}
=
\sum q_{\mathrm{eligible}}+\sum q_{\mathrm{waste}}
$$

$$
Q_{\mathrm{in}}=Q_{\mathrm{out}}+q_{\mathrm{loss}},
\qquad q_{\mathrm{loss}}\geq0
$$

#### 재활용 credit

입력 credit은 `ELIGIBLE` output에만 배분합니다.

$$
\sum a_{\mathrm{input}}=\sum a_{\mathrm{eligible}}
$$

$$
0\leq a_{\mathrm{eligible},j}\leq q_{\mathrm{eligible},j}
$$

$$
a_{\mathrm{waste},j}=0
$$

Eligible output 사이의 구체적인 credit 배분값은 private일 수 있습니다.

#### 탄소

| 변수 | 의미 |
|---|---|
| $E_{\mathrm{total}}$ | 입력 탄소와 이번 공정 탄소를 합한 값입니다. |
| $\Delta e_{\mathrm{process}}$ | 이번 공정에서 추가된 탄소입니다. |

$$
E_{\mathrm{total}}
=
\sum e_{\mathrm{input}}+\Delta e_{\mathrm{process}}
$$

$$
\Delta e_{\mathrm{process}}\geq0
$$

$$
\sum e_{\mathrm{eligible}}=E_{\mathrm{total}},
\qquad e_{\mathrm{waste},j}=0
$$

탄소는 `ELIGIBLE` output 질량에 비례해 배분하고, 나눗셈 rounding residual은 첫 번째 `ELIGIBLE` output이 받아야 합니다.

Output `DocumentHash`는 input과 달라질 수 있습니다. v1은 material compatibility를 검사하지 않습니다.

### 8.7 Exit와 Issue: 공급망 종료

#### Exit

Exit는 Note를 successor 없이 소비합니다.

$$
\mathrm{Note}\longrightarrow\varnothing
$$

`ELIGIBLE`과 `WASTE` Note 모두 Exit할 수 있습니다. `WASTE` Note가 사용할 수 있는 유일한 Event는 Exit입니다. Exit의 `outputRefs`는 빈 목록이어야 합니다.

#### Issue

Issue는 `ELIGIBLE` final Note를 한 번 소비하고 `ClaimRef(h)`를 생성합니다.

Issue Policy는 다음 조건을 검사합니다.

$$
q_{\mathrm{mass}}>0,
\qquad
0\leq a_{\mathrm{rec}}\leq q_{\mathrm{mass}}
$$

$$
\frac{a_{\mathrm{rec}}}{q_{\mathrm{mass}}}\geq\tau_{\mathrm{rec}}
$$

$$
\frac{e}{q_{\mathrm{mass}}}\leq\tau_{\mathrm{carbon}}
$$

Circuit에서는 division을 사용하지 않고 Profile scale을 이용한 cross multiplication으로 검사해야 합니다.

예를 들어 threshold가 같은 scale로 정수화되어 있다면 개념적으로 다음 형태입니다.

$$
a_{\mathrm{rec}}\cdot\mathrm{scale}
\geq
q_{\mathrm{mass}}\cdot\tau_{\mathrm{rec}}
$$

$$
e\cdot\mathrm{scale}
\leq
q_{\mathrm{mass}}\cdot\tau_{\mathrm{carbon}}
$$

Issue Circuit은 final Note의 `DocumentHash`와 $d$가 같고, $h$가 정확한 domain-separated hash임을 증명해야 합니다.

원문 대응: §4.1, §11

---

## 9. Policy와 VK: 승인된 계산 규칙만 사용

### 9.1 PolicyRef

이번 절에서 사용하는 값은 다음과 같습니다.

| 변수 | 의미 |
|---|---|
| `eventKind` | Entry·Transfer·Process 같은 Event 종류입니다. |
| `policyIdentifier` | Policy를 구분하는 이름 또는 ID입니다. |
| `version` | Policy 버전입니다. |
| `policyRef` | 위 세 값을 hash한 공개 Policy 식별값입니다. |

$$
\mathrm{policyRef}
=
H(\mathrm{DOMAIN}_{\mathrm{PolicyRef}},\mathrm{eventKind},\mathrm{policyIdentifier},\mathrm{version})
$$

정확한 domain tag는 `zkDPP:PolicyRef:v1`입니다.

계산식, threshold, 단위, scale, rounding, output 역할은 Policy-specific Circuit 또는 relation에 고정해야 합니다. Prover가 transaction마다 이 값을 선택하면 안 됩니다.

관계식이나 VK가 바뀌면 새 `policyRef`와 VK를 등록해야 합니다.

### 9.2 PolicyRecord

Policy Authority는 다음 정보를 등록합니다.

| 필드 | 의미 |
|---|---|
| `eventKind` | 이 Policy가 적용될 Event입니다. |
| `vkHash` | VerifyingKey의 hash입니다. |
| `verifierRef` | 실제 verifier 위치입니다. |
| `enabled` | 신규 proof에 사용할 수 있는지 나타냅니다. |

Policy는 `enabled=true`로 등록합니다. 상태 변경은 `true→false`만 허용하고 다시 활성화하면 안 됩니다.

Disable은 이후 신규 proof만 막습니다. 이미 승인된 transition과 Claim을 소급해서 무효화하면 안 됩니다.

### 9.3 PolicyGrant와 scope

`policyScopeRef`는 어떤 조직·recipe·사용 범위인지를 직접 노출하지 않는 opaque identifier입니다.

$$
\mathrm{PolicyGrant}[\mathrm{policyScopeRef},\mathrm{policyRef}]=\mathrm{allowed}
$$

Policy Authority는 scope와 Policy의 적합성을 off-chain에서 판단합니다. Contract는 등록된 grant만 기계적으로 검사합니다. 구체적인 자격증명과 grant 변경 절차는 `PROFILE-AUTH`에서 정합니다.

### 9.4 Contract가 proof 전에 확인할 것

1. `PolicyRecord`가 존재하고 enabled인지 확인합니다.
2. Policy의 EventKind가 호출 Event와 같은지 확인합니다.
3. 제출자가 해당 `policyScopeRef`에 승인됐는지 확인합니다.
4. `PolicyGrant`가 true인지 확인합니다.
5. proof가 `policyRef`와 `policyScopeRef`에 binding됐는지 확인합니다.
6. 등록된 VK 또는 verifier로 proof가 검증되는지 확인합니다.

원문 대응: §7

---

## 10. StatusTree: Active, Frozen, Revoked 집행

### 10.1 상태의 의미

| 상태 | 의미 |
|---|---|
| `Active` | 정상적으로 사용할 수 있습니다. |
| `Frozen` | 조사 중이므로 임시 사용 금지입니다. |
| `Revoked` | 문제가 확정되어 영구 사용 금지입니다. |

StatusTree에 key가 없으면 `Active`로 해석합니다. 따라서 Tree에는 `Frozen`과 `Revoked` 예외만 기록합니다.

### 10.2 허용되는 상태 변경

```text
Active → Frozen
Frozen → Active
Frozen → Revoked
```

`Revoked`는 terminal입니다. `Active→Revoked`와 `Revoked→Active`는 금지합니다.

### 10.3 어떤 객체에 적용합니까?

- 아직 소비되지 않은 `NoteRef`
- 아직 해결되지 않은 `VoucherRef`
- 현재 유효한 `ClaimRef`

Frozen Note는 모든 소비 Event에서 거부합니다. Frozen Voucher는 Proceed와 Recall에서 거부합니다. Frozen·Revoked Claim은 유효한 DPP Claim으로 표시하면 안 됩니다.

### 10.4 private input의 상태 증명

소비하는 Note 또는 Voucher의 ID는 private이므로 Contract가 mapping을 직접 조회하지 않습니다. Circuit 안에서 해당 `auditKey`가 현재 `statusRoot`에서 Active임을 증명합니다.

$$
\mathrm{StatusLookup}(\mathrm{currentStatusRoot},\mathrm{parentAuditKey})
=
\mathrm{Active}
$$

상태 path는 private witness이고 `statusRoot`는 public input입니다. Contract는 proof의 root가 과거 root가 아니라 현재 root와 같은지 확인해야 합니다.

### 10.5 Status Authority의 상태 변경

Contract는 Status Authority 전용 interface를 제공해야 합니다.

```text
updateStatus(targetRef, nextStatus)
```

Contract는 다음을 확인합니다.

- `targetRef`가 실제로 생성된 객체인지 확인합니다.
- 현재 상태에서 `nextStatus`로 이동할 수 있는지 확인합니다.
- `statusRoot`를 원자적으로 갱신합니다.

private consumption 때문에 Contract는 Note나 Voucher가 아직 live인지 직접 알 수 없습니다. Status Authority가 live target을 올바르게 고른다는 것은 신뢰 가정입니다. 구체적인 ABI는 `PROFILE-STATUS`에서 정합니다.

원문 대응: §8

---

## 11. AuditRecord: 비공개 parent를 나중에 추적

### 11.1 모든 Event가 남기는 기록

모든 성공 Event는 정확히 하나의 AuditRecord를 생성해야 합니다.

| 필드 | 의미 |
|---|---|
| `auditRecordId` | AuditRecord의 순번 또는 식별자입니다. |
| `eventKind` | 어떤 Event가 실행됐는지 나타냅니다. |
| `policyRef` | 어떤 Policy를 사용했는지 나타냅니다. |
| `outputRefs` | 이 Event가 공개적으로 생성한 객체들입니다. |
| `parentCount` | 실제 parent 개수입니다. |
| `encryptedParents` | 실제 private parent 목록의 ciphertext입니다. |

Contract는 각 output에 다음 index를 기록합니다.

$$
\mathrm{producerOf}[\mathrm{auditKey}(\mathrm{outputRef})]
=
\mathrm{auditRecordId}
$$

따라서 현재 객체에서 그것을 만든 Event로 한 단계 이동할 수 있습니다. 같은 Event에서 나온 sibling output들이 함께 생성됐다는 사실은 공개됩니다.

### 11.2 실제 parent와 ciphertext binding

실제 parent `AuditRef` 목록과 암호화 randomness는 private witness입니다. `encryptedParents`는 public proof input입니다.

Circuit은 다음 관계를 증명해야 합니다.

$$
\mathrm{encryptedParents}
=
\operatorname{TE.Enc}
(\mathrm{committeePK},\operatorname{Encode}(\mathrm{actualParentRefs},\mathrm{parentCount}),\mathrm{randomness})
$$

즉, 공개 ciphertext가 실제로 소비한 private parent를 암호화했다는 사실까지 proof에 포함됩니다. 다른 parent를 암호화하면 proof가 성립하면 안 됩니다.

Event별 특수 조건은 다음과 같습니다.

- Entry의 `parentCount`는 0입니다.
- Parent slot padding과 `parentCount`는 Event 최대 arity에 맞아야 합니다.
- Exit의 `outputRefs`는 빈 목록입니다.
- Issue의 `outputRefs`는 `ClaimRef(h)`를 포함합니다.

원문 대응: §9.1, §9.2

---

## 12. Threshold decryption: K명의 협조로 한 ciphertext만 열기

### 12.1 큰 그림

```text
AuditRecord의 encryptedParents
  → K명의 위원이 각각 partial decryption 생성
  → Status Authority가 share 검증·결합
  → 해당 record의 parent AuditRef 복원
```

위원은 장기 secret share 자체를 Status Authority에게 보내면 안 됩니다. 특정 ciphertext에만 유효한 partial decryption을 보냅니다.

### 12.2 필요한 interface

| 연산 | 의미 |
|---|---|
| `TE.Setup(K,N)` | 위원회 public key와 각 위원의 key share를 만듭니다. |
| `TE.Enc` | 실제 parent 목록을 committee public key로 암호화합니다. |
| `TE.PartialDecrypt` | 위원 한 명이 특정 ciphertext용 share를 만듭니다. |
| `TE.VerifyShare` | 잘못된 partial decryption을 검사합니다. |
| `TE.Combine` | 최소 $K$개의 valid share로 plaintext를 복원합니다. |

$$
(\mathrm{committeePK},\{\mathrm{skShare}_i\},\{\mathrm{pkShare}_i\})
=
\operatorname{TE.Setup}(K,N)
$$

$$
\mathrm{partial}_i
=
\operatorname{TE.PartialDecrypt}(\mathrm{skShare}_i,\mathrm{ciphertext})
$$

`TE.Combine`은 valid share가 $K$개 이상일 때만 성공해야 합니다. 구체적인 암호 방식은 `PROFILE-TE`에서 정합니다.

Status Authority가 정상적인 combiner입니다. Partial-decryption 교환, 복원된 graph, AuditCase 정보는 off-chain에 두며 온체인 AuditCase 객체는 만들지 않습니다.

원문 대응: §9.3

---

## 13. 모든 Event proof가 공통으로 하는 일

### 13.1 공개하는 값

각 Event는 필요한 subset의 다음 값을 public statement에 포함합니다.

| 공개값 | 용도 |
|---|---|
| `eventKind` | 어떤 Event proof인지 구분합니다. |
| `policyRef`, `policyScopeRef` | 승인된 규칙과 사용 범위를 고정합니다. |
| 현재 Note 또는 Voucher root | private parent의 존재를 검증합니다. |
| 현재 `statusRoot` | parent가 Active인지 검증합니다. |
| `nf` 또는 `rvnf` | 중복 소비를 막습니다. |
| 새 output AuditRefs | 새로 생성한 객체를 등록합니다. |
| `encryptedParents` | 실제 parent의 암호화 기록입니다. |
| Event별 metadata | deadline 등 Event에 필요한 공개 정보입니다. |

소비된 `cm` 또는 `rv` 자체는 public input이면 안 됩니다. Issue는 public output으로 $h$를 포함해야 합니다. Transfer deadline metadata는 기존 Recall semantics를 유지해야 합니다.

### 13.2 숨기는 값

각 Event는 필요한 subset의 다음 값을 private witness로 둡니다.

- 소비한 NoteRef 또는 VoucherRef
- Note 또는 Voucher opening
- membership path와 status path
- owner secret
- $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$
- $q_{\mathrm{loss}}$
- $\Delta e_{\mathrm{transport}}$, $\Delta e_{\mathrm{process}}$
- 실제 parent AuditRef 목록
- threshold-encryption randomness
- private allocation 값

### 13.3 Circuit의 공통 검사

각 소비 Circuit은 다음을 검사합니다.

1. Private opening이 commitment와 일치합니다.
2. Parent commitment가 현재 membership root에 포함됩니다.
3. Prover가 parent를 소비할 권한을 가집니다.
4. Nullifier가 parent와 owner secret에서 정확히 계산됩니다.
5. Parent AuditRef가 현재 status root에서 Active입니다.
6. Event별 질량·탄소·credit 관계가 성립합니다.
7. Output commitment와 공개 output AuditRef가 일치합니다.
8. `encryptedParents`가 실제 private parent 목록을 암호화합니다.
9. Proof가 `policyRef`와 `policyScopeRef`에 binding됩니다.

### 13.4 Contract의 원자적 갱신

Proof가 성공하면 Contract는 한 transaction에서 다음을 모두 수행합니다.

1. Nullifier를 spent로 기록합니다.
2. 새 Note, Voucher 또는 Claim을 등록합니다.
3. 새 membership root를 계산합니다.
4. AuditRecord를 저장하고 ID를 부여합니다.
5. 각 output의 `producerOf` index를 기록합니다.
6. Deadline 또는 Voucher resolution 같은 Event별 상태를 기록합니다.

하나라도 실패하면 전체 transaction을 revert해야 합니다. 일부 상태만 갱신되면 안 됩니다.

원문 대응: §10

---

## 14. DPP Claim 검증

DPP Verifier는 다음 순서로 검증합니다.

1. DPP 문서에서 zkDPP Claim 항목을 제외한 canonical DPP core를 만듭니다.
2. Profile의 canonicalization과 hash로 $d$를 계산합니다.
3. `issuePolicyRef`, $d$, `claimNonce`로 $h$를 다시 계산합니다.
4. 계산한 $h$가 DPP Claim 항목의 $h$와 같은지 확인합니다.
5. `ClaimRef(h)`가 온체인 Issue output으로 등록됐는지 확인합니다.
6. `ClaimRef(h)`의 현재 상태가 Active인지 확인합니다.

Frozen 또는 Revoked Claim은 valid로 표시하면 안 됩니다.

Policy가 나중에 disable되더라도 이미 정상 발행된 Claim을 자동으로 무효화하면 안 됩니다. 기존 Claim의 유효성은 Claim 자신의 현재 상태로 판단합니다.

원문 대응: §12

---

## 15. Off-chain backward audit

### 15.1 현재 live 객체 조사

```text
live AuditRef 식별
  → Frozen
  → producerOf에서 AuditRecord 조회
  → K개 partial decryption 결합
  → parent AuditRefs 복원
  → 각 parent의 producer record 반복 조회
  → Entry까지 도달
```

구체적인 절차는 다음과 같습니다.

1. Status Authority가 조사할 live AuditRef를 외부적으로 식별합니다.
2. 조사 중 사용되지 않도록 대상을 Frozen으로 변경합니다.
3. `producerOf`에서 시작 AuditRecord를 찾습니다.
4. $K$명의 위원이 해당 ciphertext용 partial decryption을 만듭니다.
5. Authority가 share를 검증·결합해 parent refs를 복원합니다.
6. 각 parent의 producer record를 Entry까지 반복 조회합니다.
7. 외부 감사 절차가 graph와 별도 증거를 평가합니다.
8. 문제가 없으면 target을 Active로 되돌립니다.
9. 문제가 확인되면 target을 Revoked로 변경합니다.

### 15.2 과거 record 조사

Historical audit은 외부에서 선택한 `auditRecordId`에서 시작합니다. 이 감사는 어떤 live AuditRef 상태도 자동으로 변경하지 않습니다.

Case ID, partial-decryption 요청, plaintext graph, 증거 자료는 off-chain audit system이 관리합니다. Contract에 별도 audit-open call이나 AuditCase 객체를 만들지 않습니다.

원문 대응: §13

---

## 16. 구현 전에 확정해야 하는 Profile

Circuit key와 canonical test vector를 만들기 전에 다음 선택을 모두 확정하고 Profile manifest와 Decision Log에 기록해야 합니다.

| Profile | 확정할 내용 | 반드시 지킬 조건 |
|---|---|---|
| `PROFILE-PROOF` | Proof system, curve, setup | 기존 PLONK-KZG/BLS12-381은 후보일 뿐입니다. |
| `PROFILE-HASH` | Hash와 domain encoding | 모든 object가 동일한 canonical encoding을 사용해야 합니다. |
| `PROFILE-NUMERIC` | Scale, bit width, bounds | 음수 오용과 field wraparound를 막아야 합니다. |
| `PROFILE-ARITY` | $M_{\max}$, $N_{\max}$, padding | bounded arbitrary input/output을 지원해야 합니다. |
| `PROFILE-MEMBERSHIP` | Note·Voucher tree | Private leaf membership을 증명할 수 있어야 합니다. |
| `PROFILE-STATUS` | Authenticated dictionary | Private Active lookup과 온체인 update가 가능해야 합니다. |
| `PROFILE-TE` | $K$-out-of-$N$ encryption | Circuit 내 encryption과 검증 가능한 share가 필요합니다. |
| `PROFILE-AUDIT-ENC` | AuditRef와 parent slot encoding | Type collision이 없는 canonical encoding이 필요합니다. |
| `PROFILE-AUTH` | Role과 scope credential | Entry·Policy·Status 권한을 분리해야 합니다. |
| `PROFILE-DPP` | DPP canonical bytes와 hash | Claim 항목 제외 규칙이 필요합니다. |
| `PROFILE-NONCE` | `claimNonce` | 균등 생성과 canonical encoding이 필요합니다. |
| `PROFILE-EVENT` | EventKind encoding | Circuit·Registry·Contract가 같은 encoding을 사용해야 합니다. |

Profile 변경이 Policy relation이나 public-input manifest를 바꾸면 새 PolicyRef와 VK를 만들어야 합니다.

### 16.1 Threshold encryption 후보의 합격 조건

- $K$명 미만 share로 plaintext를 복원할 수 없어야 합니다.
- $K$개 valid share로 deterministic하게 복호화할 수 있어야 합니다.
- 잘못된 share를 검증하거나 honest-share 가정을 명시해야 합니다.
- Parent vector와 `parentCount`를 canonical하게 encoding해야 합니다.
- Circuit에서 encryption correctness를 증명할 수 있어야 합니다.
- Constraint 수와 public ciphertext 크기를 측정할 수 있어야 합니다.
- 장기 secret share를 재구성하지 않는 partial-decryption interface가 있어야 합니다.

### 16.2 Status 자료구조 후보의 합격 조건

- 없는 key를 Active로 증명할 수 있어야 합니다.
- Frozen과 Revoked leaf를 구분해야 합니다.
- Private key lookup path를 Circuit에서 검증할 수 있어야 합니다.
- Contract가 current root를 원자적으로 갱신할 수 있어야 합니다.
- 과거 root를 transaction acceptance에서 거부할 수 있어야 합니다.
- AuditRef type collision을 막을 수 있어야 합니다.

원문 대응: §14

---

## 17. 구현 적합성 테스트

### 17.1 반드시 성공해야 하는 흐름

- Entry부터 Issue까지 모든 Event가 포함된 canonical scenario
- Transfer의 Proceed path와 Recall path
- 서로 다른 active $m,n$을 사용하는 Process
- `ELIGIBLE`과 `WASTE` output이 함께 있는 Process
- live Note, Voucher, Claim의 freeze와 unfreeze
- live object freeze 후 backward audit
- historical record에서 시작하는 backward audit
- Policy disable 전 proof 승인과 disable 후 신규 proof 거부

### 17.2 반드시 실패해야 하는 공격

#### 객체와 권한

- 위조된 Note 또는 Voucher opening
- 잘못된 owner secret
- 중복 Note nullifier
- 중복 Voucher resolution

#### 상태

- Frozen Note 소비
- Frozen Voucher의 Proceed 또는 Recall
- Frozen 또는 Revoked Claim 검증
- 과거 `statusRoot` replay
- 권한 없는 status update
- `Active→Revoked` 직접 전이
- `Revoked→Active` 전이

#### Policy

- Disabled PolicyRef 사용
- 잘못된 EventKind의 Policy 사용
- 누락된 PolicyGrant
- Proof와 다른 `policyScopeRef` 제출

#### Event 계산

- 입력보다 큰 질량 output
- 재활용 credit 생성 또는 누락
- WASTE output에 credit 또는 탄소 배분
- 잘못된 비례 rounding
- 잘못된 운송·공정 탄소 합계
- Prover가 사후에 WASTE 역할 선택

#### 감사와 Claim

- 실제 parent와 다른 `encryptedParents`
- 부족하거나 잘못된 partial-decryption share
- DPP core mismatch
- `claimNonce` 또는 `issuePolicyRef` mismatch
- 같은 final Note로 Issue 재실행

원문 대응: §15

---

## 18. 구현 담당자가 제공해야 하는 결과물

1. 확정된 implementation Profile manifest
2. 각 Event Circuit source와 public-input manifest
3. Native reference implementation과 canonical vector
4. ProvingKey·VerifyingKey 생성 절차
5. Router·Registry·StatusTree Contract source
6. AuditRecord encoder와 off-chain audit client
7. Threshold setup·partial decryption·combine 도구
8. Positive·negative conformance test
9. Constraint·proving time·proof size·verification gas benchmark
10. 기존 POC와 새 v1의 migration/change matrix

원문 대응: §16

---

## 19. 현재 poc-v2에서 달라지는 부분

현재 `poc-v2/`는 비교 대상이며 이 명세의 구현이 아닙니다. v1 구현에는 최소 다음 변경이 필요합니다.

1. Item count 기반 Quantity를 제거하고 $q_{\mathrm{mass}}$로 통일합니다.
2. StateVector를 `assetRole`, $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$ 의미로 교체합니다.
3. Event별 질량·credit·탄소 관계를 적용합니다.
4. 공개 consumed commitment와 Voucher ID를 private witness로 옮깁니다.
5. 현재 `statusRoot`에 대한 private Active proof를 추가합니다.
6. AuditRecord, `encryptedParents`, `producerOf`를 추가합니다.
7. Issue Circuit, ClaimRef, DPP verification을 추가합니다.
8. Policy-specific VK Registry와 PolicyGrant를 추가합니다.
9. One-way Policy disable과 AuditRef 상태 변경을 추가합니다.
10. Threshold partial-decryption audit client를 추가합니다.

이 목록은 구현 순서를 강제하지 않습니다. Dependency와 실험 위험에 따라 milestone을 정할 수 있습니다.

원문 대응: §17

---

## 부록 A. Domain tag 모음

| 용도 | 정확한 문자열 |
|---|---|
| Note commitment | `zkDPP:Note:v1` |
| Voucher commitment | `zkDPP:Voucher:v1` |
| Note nullifier | `zkDPP:Nullifier:v1` |
| Voucher nullifier | `zkDPP:VoucherNullifier:v1` |
| AuditRef key | `zkDPP:AuditRef:v1` |
| Issue Claim | `zkDPP:Issue:v1` |
| PolicyRef | `zkDPP:PolicyRef:v1` |
| ScopeRef | `zkDPP:ScopeRef:v1` |

구체적인 field encoding은 `PROFILE-HASH`에서 고정합니다.

## 부록 B. 원문 절 대응표

| 원문 절 | 이 해설판 |
|---|---|
| §1 문서 권한 | 문서 머리말·읽는 방법 |
| §2 목표와 비목표 | §1 한눈에 보는 zkDPP v1 |
| §3 역할과 trust model | §2 참여자와 신뢰 경계 |
| §4 Type과 domain | §3, §6, §8, 부록 A |
| §5 Protocol object | §3~§6 |
| §6 On-chain state | §7 Contract가 기억하는 상태 |
| §7 Policy와 VK Registry | §9 Policy와 VK |
| §8 StatusTree | §10 StatusTree |
| §9 AuditRecord와 threshold decryption | §11, §12 |
| §10 Common transition | §13 모든 Event의 공통 동작 |
| §11 Event relation | §8 정상 Event 흐름 |
| §12 DPP verification | §14 DPP Claim 검증 |
| §13 Off-chain audit | §15 Backward audit |
| §14 Implementation profile | §16 구현 Profile |
| §15 Conformance tests | §17 적합성 테스트 |
| §16 Required artifacts | §18 구현 결과물 |
| §17 POC change boundary | §19 poc-v2 변경점 |
