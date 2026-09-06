# M8 Exit·DPP·Issue — 왜 이 구조인가요?

**공급망 안에서는 DPP Object를 private Note로 관리하고, Exit에서 공개 DPP commitment를 만든 뒤 Issue로 Policy Claim을 추가합니다.**

- 문서 성격: M8 의사결정 배경입니다.
- 구현 기준: [M8 구현 명세](M8-exit-dpp-issue.md)
- 이전 결과: [M7 Result](M7-audit-tracing-result.md)
- 원칙: **YAGNI가 최우선입니다.** DPP ownership transfer·재귀 구성·규제 DPP 전체를 구현하지 않습니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

| 질문 | M8 결정 |
|---|---|
| 공급망 안의 DPP는 무엇인가요? | 개념적인 DPP Object의 현재 private State를 Note로 표현합니다. |
| Exit는 무엇을 하나요? | Note를 nf로 소비하고 DPPRef(dppCommitment)를 만듭니다. |
| Issue는 무엇을 하나요? | DPP를 소비하지 않고 특정 IssuePolicy Claim을 추가합니다. |
| Claim은 어떻게 식별하나요? | dppCommitment와 issuePolicyRef의 조합입니다. |
| 무엇을 숨기나요? | DocumentHash·State·AssetRole·dppOpening입니다. |
| 무엇을 공개하나요? | dppCommitment와 만족한 issuePolicyRef입니다. |
| 누가 proof를 만들 수 있나요? | dppCommitment의 private 원문을 아는 사람입니다. |
| Grant가 필요한가요? | Issue에는 PolicyGrant·policyScopeRef를 사용하지 않습니다. |
| Claim 상태가 있나요? | Policy version별로 Active·Frozen·Revoked를 관리합니다. |

전체 전이는 다음입니다.

$$
\mathrm{Note}
\xrightarrow{\mathrm{Exit}}
\mathrm{Finalized\ DPP}
$$

$$
\mathrm{Finalized\ DPP}
\xrightarrow{\mathrm{IssuePolicy}}
\mathrm{같은\ DPP}+\mathrm{Claim}
$$

## 1. 기존 Conversation 설계와 무엇이 달라졌나요?

### 기존 설계

기존 Conversation 문서는 외부 DPP가 먼저 존재하고 Issue가 Final Note를 소비하면서 Claim을 즉시 만드는 구조였습니다.

~~~text
기존 Issue:
  Note → ClaimRef(h)

DPPClaim:
  issuePolicyRef, h, 공개 Claim nonce
~~~

기존 DPP Verifier는 DPP의 ProductName·LotID·Unit을 읽어 DocumentHash와 h를 다시 계산했습니다. 현재 Note 모델에서 Unit은 이미 제거됐습니다.

### 이번 결정

M8은 종료와 Claim 발행을 분리합니다.

~~~text
Exit:
  Note → DPPRef(dppCommitment)

Issue:
  DPPRef(dppCommitment)
    → 같은 DPP에 issuePolicyRef Claim 추가
~~~

이 변경에는 세 가지 의미가 있습니다.

1. 공급망 안에서는 Claim을 붙이지 않습니다.
2. Exit가 Note를 한 번 소비하고 DPP를 final 상태로 전환합니다.
3. 하나의 DPP는 서로 다른 IssuePolicy Claim을 여러 개 가질 수 있습니다.

별도의 claimId·Claim nonce·Claim Tree를 만들지 않습니다. 하나의 DPP·동일 policyRef에는 Claim 하나만 허용합니다.

## 2. DPP System과 DPP Object는 어떻게 다른가요?

**zkDPP System은 전체 Protocol이고, DPP Object는 그 안에서 추적하는 물품·batch·제품·폐기물입니다.**

~~~text
zkDPP System
├─ Entry·Transfer·Merge·Split·Process
├─ Exit·Issue
├─ Policy
├─ Audit
└─ Status

DPP Object
├─ private 제품·State 정보
├─ provenance
└─ DPPClaim[]
~~~

공급망 단계에서는 다음 대응을 사용합니다.

| 개념 | 암호학적 표현 | 온체인 공개값 |
|---|---|---|
| DPP Object의 현재 State | Note | cm |
| 전달 대기 상태 | Voucher | rv |
| Exit된 DPP | DPPPrivateData | dppCommitment |

Exit가 DPP를 처음 만드는 것은 아닙니다. 기존 DPP Object의 active 공급망 전이를 종료하고 Claim을 연결할 공개 commitment를 만듭니다.

재귀적인 component DPP는 M7 provenance DAG로 개념적으로 표현할 수 있지만, M8은 DPP 안에 하위 DPP 문서를 넣는 기능이나 Re-entry를 구현하지 않습니다.

## 3. 왜 cm을 그대로 사용하지 않나요?

Note commitment에는 owner address와 Note opening이 들어 있습니다.

$$
cm=
H(
\mathrm{NoteTag},
\mathrm{DocumentHash},
\mathrm{AssetRole},
q_{\mathrm{mass}},
a_{\mathrm{rec}},
e,
\mathrm{ownerAddress},
\mathrm{noteOpening}
)
$$

cm을 외부 DPP identifier로 재사용하면 다음 문제가 있습니다.

- 공개 DPP가 Note 생성 transaction과 직접 연결됩니다.
- 과거 Note owner에 binding된 값을 DPP 전달 후에도 사용하게 됩니다.
- Exit에서 소비된 Note와 계속 Claim을 보관하는 DPP가 같은 identifier를 사용합니다.

그래서 DPP의 제품·State 정보만 다시 commitment합니다.

$$
\mathrm{dppCommitment}
=
H(
\mathrm{DPPTag},
\mathrm{DocumentHash},
\mathrm{AssetRole},
q_{\mathrm{mass}},
a_{\mathrm{rec}},
e,
\mathrm{dppOpening}
)
$$

owner address·Note opening·Note nf는 포함하지 않습니다. Exit Circuit이 Note와 DPPPrivateData의 DocumentHash·Role·State가 같은지 확인합니다.

## 4. dppOpening은 소유권 secret인가요?

**아닙니다. dppOpening은 commitment hiding과 randomization을 위한 값입니다.**

State·DocumentHash만 Hash하면 예상 후보를 반복 계산하는 dictionary attack이 가능할 수 있습니다. dppOpening은 같은 State의 서로 다른 DPP commitment도 구분합니다.

하지만 복사 가능한 값을 전달한다고 과거 Owner가 잊는 것은 아닙니다.

~~~text
A가 DPP private data를 B에게 전달
  → A도 복사본을 가질 수 있음
  → B도 같은 commitment를 열 수 있음
~~~

따라서 dppOpening을 현재 Owner만 아는 독점적 소유권 credential로 해석하지 않습니다. M8 Claim은 제품에 대해 검증된 사실이며 Owner token이 아닙니다.

과거 Owner가 private data를 계속 안다면 실제 State가 만족하는 다른 Policy Claim을 만들 수 있습니다. 다음 제약만 강제합니다.

- 만족하지 않는 Claim은 proof를 만들 수 없습니다.
- 동일 DPP·동일 policyRef Claim은 중복 발행할 수 없습니다.
- Claim 상태는 Status Authority만 변경합니다.

DPP ownership rotation·법적 소유자 확인은 별도 기능입니다.

## 5. Claim은 왜 두 값으로 식별하나요?

**Claim은 DPP와 Policy의 관계입니다.**

~~~text
ClaimRef
├─ dppCommitment
└─ issuePolicyRef
~~~

별도 Claim Hash를 추가해도 결국 같은 두 값을 다시 Hash할 뿐입니다. 이번에는 다음 mapping을 Claim의 생성·중복 방지·Audit index로 함께 사용합니다.

~~~text
claimRecordOf[dppCommitment][issuePolicyRef]
  = Issue AuditRecord ID
~~~

값이 0이면 Claim이 없고, 0이 아니면 생성 기록이 있습니다. 같은 Policy family의 새 version은 다른 policyRef이므로 별도 Claim입니다.

DPP의 off-chain 표현도 간단합니다.

~~~text
DPP
├─ dppCommitment
└─ claims[]
   ├─ issuePolicyRef V1
   └─ issuePolicyRef V2
~~~

제품 정보는 Owner가 원하면 보여줄 수 있지만 Claim 확인의 필수 입력이 아닙니다.

## 6. 왜 Issue에 PolicyGrant가 없나요?

Process PolicyGrant는 특정 private Note owner scope가 특정 공정 규칙을 사용하도록 제한합니다. Issue의 목적은 다릅니다.

~~~text
Policy Authority:
  어떤 Sustainability 기준을 공식 Claim으로 사용할지 승인

DPP private 원문을 아는 사람:
  그 State가 기준을 만족한다는 proof 생성
~~~

IssuePolicy의 사용 권한은 다음 두 조건으로 충분합니다.

1. Policy Authority가 verifier·VK hash·version을 등록했습니다.
2. Prover가 dppCommitment의 원문과 만족하는 State를 알고 있습니다.

Issue에는 policyScopeRef·PolicyGrant를 추가하지 않습니다. Policy Authority가 Policy를 disable하면 신규 Issue만 막고 이미 발행한 Claim에는 소급하지 않습니다.

## 7. 왜 두 개의 IssuePolicy를 사용하나요?

한 DPP에 여러 Policy version Claim을 붙일 수 있는지 확인하기 위해 같은 family의 V1·V2를 사용합니다.

| Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

M5 Process의 ELIGIBLE output은 다음입니다.

~~~text
q_mass = 270 kg
a_rec  = 30 kg
e      = 260 kgCO2e
~~~

재활용률은 약 11.11%, 탄소집약도는 약 0.963이므로 두 Policy를 모두 통과합니다. 서로 다른 기능을 억지로 추가하지 않고 version별 Claim 등록과 독립 상태만 확인합니다.

## 8. Issue AuditRecord에는 왜 암호문이 없나요?

Issue의 입력 dppCommitment는 공개이고 Claim도 공개된 DPP·Policy 조합입니다. 숨겨진 객체 참조나 미래 소비 nullifier가 없습니다.

~~~text
Issue AuditRecord
├─ eventKind = ISSUE
├─ policyRef
├─ outputRefs = []
├─ encryptedParents = []
└─ encryptedOutputNfs = []
~~~

Issue transaction의 dppCommitment와 claimRecordOf를 이용해 생성 관계를 찾습니다. AuditRecord에 공개 input 전체를 다시 저장하지 않습니다.

~~~text
(dppCommitment, issuePolicyRef)
  → claimRecordOf
  → Issue transaction
  → producerOf[DPP][dppCommitment]
  → Exit AuditRecord
  → encrypted parent cm
~~~

현재 M7 AuditRecord decoder는 암호문을 전제로 하므로 M8은 빈 암호문·사용하지 않는 R1을 명시적으로 지원해야 합니다. 임의의 잘못된 곡선점을 정상 암호문으로 처리하는 것이 아니라 **암호문이 없는 Event 형식**으로 구분합니다.

## 9. 공식 DPP 전체를 구현하나요?

아닙니다. M8의 DPP는 private Sustainability State와 Claim을 연결하는 Protocol abstraction입니다.

- 규제상 필수 제품 identifier·데이터 캐리어·접근권한 전체를 구현하지 않습니다.
- ProductName·LotID를 제3자에게 반드시 공개하지 않습니다.
- DPP Owner가 원하면 별도 제품 정보를 보여줄 수 있지만 zkDPP Claim 검증 조건은 아닙니다.
- 동일한 디지털 DPP의 물리 복제·QR/NFC·외부 Credential은 범위 밖입니다.

## 10. 무엇을 구현 명세에서 확인하면 되나요?

[M8 구현 명세](M8-exit-dpp-issue.md)는 다음을 자체적으로 반복합니다.

- DPPPrivateData·Domain·Field 순서
- 새 Exit와 Issue의 전체 관계
- 두 IssuePolicy 상수·등록 interaction
- Contract 상태·함수·AuditRecord
- DPP Claim 표시·제3자 확인
- 성공·실패·원자성·측정·Artifact 기준

Background는 결정 이유를 보존하고 구현 명세를 대신하지 않습니다.
