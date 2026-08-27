# zkDPP v1 구현 명세

- 상태: normative draft
- 기준일: 2026-08-25
- 대상: 새 mass-only, private-provenance zkDPP v1
- 현행 POC: `poc-v2/`는 비교 대상이며 이 명세의 구현이 아니다.

## 1. 문서 권한과 요구사항 표기

이 문서는 zkDPP v1을 구현하는 연구원이 따라야 할 normative 명세다.

기존 `Scheme/`와 `Scheme-v2/`는 현재 POC의 count 기반 모델을 설명한다.

두 문서군이 충돌하면 새 v1 구현에는 이 문서를 적용한다.

설계 이유와 비목표는 다음 두 문서에서 확인한다.

- [완료 설계 결정](./Previous%20Conversation/zkdpp-completed-design-decisions.md)
- [비공개 provenance 감사 설계](./Previous%20Conversation/zkdpp-private-provenance-audit-design.md)

`MUST`는 conforming implementation이 반드시 지켜야 하는 요구사항이다.

`SHOULD`는 정당한 이유가 없으면 지켜야 하는 요구사항이다.

`MAY`는 구현 profile이 선택할 수 있는 사항이다.

`PROFILE`은 회로 생성 전에 구체값을 정하고 기록해야 하는 항목이다.

## 2. 구현 목표와 비목표

### 2.1 목표

구현은 다음 속성을 제공해야 한다.

1. 신뢰된 Entry 이후의 질량, 재활용 credit와 탄소 회계를 검증한다.
2. 상세 상태와 소비 객체의 연결을 일반 관찰자에게 숨긴다.
3. 동일 Note 또는 Voucher의 이중 소비를 막는다.
4. 승인된 policy/VK만 해당 scope의 transition에 사용하게 한다.
5. 최종 sustainability claim을 정확한 DPP 문서에 binding한다.
6. 모든 transition에 proof-bound encrypted parent record를 남긴다.
7. trusted Status Authority가 live 객체를 동결하거나 철회하게 한다.
8. K-of-N 협조로 Status Authority가 parent graph를 역추적하게 한다.

### 2.2 비목표

구현은 다음 속성을 주장해서는 안 된다.

- 실세계 질량, 배출량 또는 재활용 입력의 진실성
- DPP 문서와 실제 물리 제품 또는 batch의 binding
- material-specific compatibility 또는 실제 재활용 함량
- ISCC PLUS 또는 EU Battery Regulation 전체 준수
- 문제 ancestor의 모든 downstream descendant 자동 탐색
- 이미 소비된 descendant의 소급 rollback
- K명 이상 key-share 보관자의 공모 저항성
- 악의적인 Status Authority에 대한 cryptographic accountability
- 공개 claim handle에 대한 강한 access control

## 3. 역할과 trust model

### 3.1 역할

| 역할 | 구현 책임 |
|---|---|
| Entry Issuer | 최초 `ELIGIBLE` Note의 신뢰된 등록 |
| Policy Authority | policy, VK, scope grant와 one-way disable 관리 |
| Operator | private witness를 사용한 transition proof 생성 |
| Status Authority | status update와 audit graph 복원 |
| Key-share Custodian | ciphertext별 partial decryption 생성 |
| DPP Verifier | DPP claim binding과 Claim 상태 검증 |
| Router Contract | proof, policy, spentness와 상태 전이 집행 |

### 3.2 Adversary

Adversary는 모든 일반 Operator, prover, transaction submitter와
DPP publisher를 제한 없이 공모시킬 수 있다.

Adversary는 K명 미만의 key-share custodian을 corrupt할 수 있다.

구현은 이 adversary가 다음 행위를 성공시키지 못하게 해야 한다.

- 존재하지 않거나 소유하지 않은 private 객체 소비
- 동일 객체의 이중 소비
- 허용되지 않은 policy 또는 scope 사용
- Event relation을 위반한 output 생성
- 실제 소비 parent와 다른 encrypted parent 등록
- Frozen 또는 Revoked 객체의 후속 사용
- 과거 Active status root를 사용한 freeze 우회
- 다른 DPP 문서에 유효 Claim 재부착

### 3.3 신뢰 가정

다음 주체와 입력은 v1에서 신뢰한다.

- Entry Issuer의 최초 발행과 중복 발행 방지
- Policy Authority의 policy 적합성 판단
- Status Authority의 audit 요청과 status 조치
- ledger와 Router Contract의 정확한 실행
- 운송·공정 배출량을 포함한 외부 운영 입력
- standard proof-system, hash와 encryption security assumption

## 4. 공통 type과 domain separation

### 4.1 EventKind

구현은 다음 아홉 Event를 구분해야 한다.

```text
Entry, Transfer, Proceed, Recall, Merge,
Split, Process, Exit, Issue
```

EventKind의 concrete integer encoding은 `PROFILE-EVENT`에서 고정한다.

### 4.2 AssetRole

```text
AssetRole = ELIGIBLE | WASTE
```

`AssetRole`은 private Note 또는 Voucher field다.

이는 material identity가 아니다.

`ELIGIBLE`은 sustainability attribute와 후속 transition을 허용한다.

`WASTE`는 `Exit`만 허용하는 terminal role이다.

### 4.3 Domain tag

서로 다른 object와 hash purpose는 다른 domain tag를 사용해야 한다.

최소 tag 집합은 다음과 같다.

```text
zkDPP:Note:v1
zkDPP:Voucher:v1
zkDPP:Nullifier:v1
zkDPP:VoucherNullifier:v1
zkDPP:AuditRef:v1
zkDPP:Issue:v1
zkDPP:PolicyRef:v1
zkDPP:ScopeRef:v1
```

Concrete field encoding은 `PROFILE-HASH`에서 고정한다.

## 5. Protocol object

### 5.1 Note

Note opening은 다음 private tuple이다.

```text
NoteOpening = (
  DocumentHash,
  assetRole,
  q_mass,
  a_rec,
  e,
  owner,
  opening
)
```

Commitment는 다음 relation을 따라야 한다.

```text
cm = H(
  "zkDPP:Note:v1",
  DocumentHash,
  assetRole,
  q_mass,
  a_rec,
  e,
  owner,
  opening
)
```

`q_mass`와 `a_rec`은 같은 질량 단위와 scale을 사용해야 한다.

`e`는 Entry 이후 누적 scaled `kgCO2e` 절대량이다.

모든 live Note는 다음 local bound를 만족해야 한다.

```text
0 <= a_rec <= q_mass
```

WASTE Note는 다음 값을 가져야 한다.

```text
a_rec = 0
e = 0
```

### 5.2 Voucher

Voucher는 pending Transfer를 나타내는 private payload다.

```text
VoucherOpening = (
  DocumentHash,
  assetRole,
  q_mass,
  a_rec,
  e,
  senderOwner,
  receiverOwner,
  deltaEpoch,
  opening
)
```

`rv`는 `zkDPP:Voucher:v1` domain의 commitment여야 한다.

Transfer는 `ELIGIBLE` Voucher만 생성한다.

Proceed와 Recall은 Voucher payload를 변경하지 않고 Note로 옮겨야 한다.

### 5.3 Claim

Issue는 final Note를 소비하고 public Claim handle `h`를 생성한다.

```text
d = H(canonical DPP core without the zkDPP claim entry)
h = H("zkDPP:Issue:v1", d, issuePolicyRef, claimNonce)
```

DPP claim entry는 다음 tuple을 포함해야 한다.

```text
DPPClaim = (issuePolicyRef, h, claimNonce)
```

`claimNonce`는 공개 nonce이며 access token이 아니다.

### 5.4 AuditRef

```text
AuditRef = NoteRef(cm) | VoucherRef(rv) | ClaimRef(h)
```

AuditRef는 별도 business object가 아니다.

구현은 type과 raw identifier를 domain-separated key에 binding해야 한다.

```text
auditKey = H("zkDPP:AuditRef:v1", typeTag, rawID)
```

`typeTag`와 `rawID` serialization은 `PROFILE-AUDIT-ENC`에서 고정한다.

### 5.5 Nullifier

Note와 Voucher는 서로 다른 nullifier domain을 사용해야 한다.

Nullifier는 consumed object와 owner secret에 binding돼야 한다.

Nullifier 값은 public이어야 한다.

Consumed `cm` 또는 `rv` 자체는 private witness여야 한다.

## 6. On-chain state

Router Contract는 최소 다음 논리 상태를 유지해야 한다.

```text
noteRoot
voucherRoot
statusRoot
noteNullifiers[nf]
voucherNullifiers[rvnf]
claimRegistered[h]
producerOf[auditKey]
auditRecords[auditRecordId]
policyRecords[policyRef]
policyGrants[policyScopeRef][policyRef]
```

Note와 Voucher membership structure의 concrete layout은 profile에서 정한다.

Contract는 transition proof에 사용된 membership root를 검증해야 한다.

Status proof에는 과거 root가 아니라 current `statusRoot`만 허용해야 한다.

## 7. Policy와 VK Registry

### 7.1 PolicyRef

PolicyRef는 최소 다음 의미에 binding돼야 한다.

```text
PolicyRef = H(
  "zkDPP:PolicyRef:v1",
  eventKind,
  policyIdentifier,
  version
)
```

계산식, threshold, 단위, scale, rounding과 output-role constraint는
policy-specific circuit 또는 relation에 고정해야 한다.

Prover가 transaction마다 normative parameter를 고르면 안 된다.

Normative relation 또는 VK가 바뀌면 새 PolicyRef를 등록해야 한다.

### 7.2 PolicyRecord

```text
PolicyRecord = {
  eventKind,
  vkHash,
  verifierRef,
  enabled
}
```

PolicyRecord는 등록 시 `enabled = true`여야 한다.

Policy Authority는 `true -> false`만 수행할 수 있다.

Disabled PolicyRef는 다시 활성화하면 안 된다.

Disable은 신규 proof만 차단해야 한다.

과거 accepted transition과 Claim에는 소급하면 안 된다.

### 7.3 PolicyGrant

```text
PolicyGrant[policyScopeRef, policyRef] = allowed
```

Policy Authority는 scope와 policy의 적합성을 off-chain에서 판단한다.

Contract는 등록된 grant만 기계적으로 강제한다.

`policyScopeRef`는 company 또는 recipe를 직접 노출하지 않는
opaque identifier여야 한다.

Concrete credential과 grant-update lifecycle은 `PROFILE-AUTH`에서 정한다.

### 7.4 Submission acceptance

Contract는 proof 검증 전에 다음 조건을 확인해야 한다.

1. PolicyRecord가 존재하고 enabled다.
2. PolicyRecord의 EventKind가 호출 Event와 같다.
3. 제출자가 policyScopeRef에 대해 승인돼 있다.
4. PolicyGrant가 true다.
5. proof statement가 PolicyRef와 policyScopeRef에 binding돼 있다.
6. 등록된 VK 또는 verifier로 proof가 검증된다.

## 8. StatusTree와 상태 집행

### 8.1 상태 의미

```text
Status = Active | Frozen | Revoked
```

StatusTree에서 entry가 없으면 Active로 해석한다.

Frozen과 Revoked만 exception leaf로 기록한다.

### 8.2 허용 transition

```text
Active -> Frozen
Frozen -> Active
Frozen -> Revoked
```

다른 transition은 모두 거부해야 한다.

Revoked는 terminal이다.

Direct `Active -> Revoked`는 허용하면 안 된다.

### 8.3 적용 대상

상태 집행 대상은 다음과 같다.

- live NoteRef
- unresolved VoucherRef
- active ClaimRef

Frozen Note는 모든 소비 Event에서 거부해야 한다.

Frozen Voucher는 Proceed와 Recall에서 거부해야 한다.

Frozen Claim은 valid DPP claim으로 반환하면 안 된다.

Revoked object는 영구적으로 사용할 수 없어야 한다.

### 8.4 Private input status proof

소비 circuit은 각 private parent AuditRef에 대해 다음을 증명해야 한다.

```text
StatusLookup(currentStatusRoot, parentAuditKey) = Active
```

Status membership 또는 non-membership path는 private witness여야 한다.

`statusRoot`는 public input이어야 한다.

Contract는 proof의 `statusRoot`가 current root와 같은지 확인해야 한다.

### 8.5 Status update

Contract는 Status Authority 전용 상태 변경 interface를 제공해야 한다.

개념적인 interface는 다음과 같다.

```text
updateStatus(targetRef, nextStatus)
```

Contract는 targetRef가 생성된 object인지 확인해야 한다.

Contract는 current status와 nextStatus의 전이가 허용되는지 확인해야 한다.

Contract는 StatusTree root를 원자적으로 갱신해야 한다.

Concrete function name과 ABI는 `PROFILE-STATUS`에서 정한다.

Private consumption 때문에 Contract는 Note/Voucher liveness를 알 수 없다.

Status Authority가 live target을 선택한다는 사실은 trust assumption이다.

## 9. AuditRecord와 threshold decryption

### 9.1 Record

모든 성공한 Event는 정확히 하나의 AuditRecord를 생성해야 한다.

```text
AuditRecord = {
  auditRecordId,
  eventKind,
  policyRef,
  outputRefs,
  parentCount,
  encryptedParents
}
```

Contract는 성공한 transition에 auditRecordId를 부여해야 한다.

각 public output에 대해 다음 index를 기록해야 한다.

```text
producerOf[auditKey(outputRef)] = auditRecordId
```

같은 transition의 sibling output grouping은 public leakage로 허용한다.

### 9.2 Parent binding

실제 parent AuditRef 목록은 private witness여야 한다.

`encryptedParents`는 public proof input이어야 한다.

Transition circuit은 다음 relation을 증명해야 한다.

```text
encryptedParents = TE.Enc(
  committeePK,
  Encode(actualParentAuditRefs, parentCount),
  encryptionRandomness
)
```

Parent slot padding과 parentCount는 Event arity에 일치해야 한다.

Entry의 parentCount는 0이어야 한다.

Exit의 outputRefs는 빈 목록이어야 한다.

Issue의 outputRefs는 `ClaimRef(h)`를 포함해야 한다.

Ciphertext가 실제 consumed parents와 다르면 proof가 성립하면 안 된다.

### 9.3 Threshold interface

Concrete threshold-encryption primitive는 `PROFILE-TE`에서 정한다.

Primitive는 최소 다음 interface를 제공해야 한다.

```text
(committeePK, {skShare_i}, {pkShare_i}) = TE.Setup(K, N)
ciphertext = TE.Enc(committeePK, message, randomness)
partial_i = TE.PartialDecrypt(skShare_i, ciphertext)
ok = TE.VerifyShare(pkShare_i, ciphertext, partial_i)
message = TE.Combine(ciphertext, {partial_i}_{i in I})
```

`TE.Combine`은 `|I| >= K`일 때만 성공해야 한다.

Key-share custodian은 장기 skShare를 외부에 전달하면 안 된다.

Partial decryption은 해당 ciphertext에만 유효해야 한다.

Status Authority는 정상 protocol의 designated combiner다.

Partial-decryption exchange와 plaintext graph는 off-chain에 둔다.

On-chain AuditCase object는 두지 않는다.

## 10. Common transition statement

### 10.1 Public statement

각 Event proof는 필요한 subset의 다음 값을 공개해야 한다.

```text
eventKind
policyRef
policyScopeRef
current note or voucher root
current statusRoot
nullifier or voucherNullifier
new output AuditRefs
encryptedParents
Event-specific public metadata
```

Consumed `cm` 또는 `rv`를 public input으로 두면 안 된다.

Issue는 public output으로 `h`를 포함해야 한다.

Transfer deadline metadata의 공개 범위는 기존 recall semantics를 유지해야 한다.

### 10.2 Private witness

각 Event proof는 필요한 subset의 다음 값을 숨겨야 한다.

```text
consumed NoteRef or VoucherRef
Note or Voucher opening
membership path
status path
owner secret
q_mass, a_rec, e
q_loss
delta_e_transport
delta_e_process
actual parent AuditRefs
threshold-encryption randomness
private allocation values
```

### 10.3 Common circuit checks

각 소비 circuit은 다음 조건을 검증해야 한다.

1. Private parent opening이 commitment와 일치한다.
2. Parent commitment가 current membership root에 포함된다.
3. Prover가 parent 소비 권한을 갖는다.
4. Nullifier가 parent와 owner secret에서 정확히 파생된다.
5. Parent AuditRef가 current status root에서 Active다.
6. Event-specific relation이 성립한다.
7. Output commitment와 public output AuditRef가 일치한다.
8. encryptedParents가 실제 private parent 목록을 암호화한다.
9. PolicyRef와 policyScopeRef가 proof statement에 binding된다.

### 10.4 Contract state update

Proof가 검증되면 Contract는 하나의 atomic state update를 수행해야 한다.

1. 사용된 nullifier를 spent로 기록한다.
2. 새 Note, Voucher 또는 Claim을 등록한다.
3. 새 membership root를 계산한다.
4. AuditRecord를 저장하고 auditRecordId를 부여한다.
5. 모든 output에 producerOf index를 기록한다.
6. Event-specific deadline 또는 resolution state를 기록한다.

하나라도 실패하면 전체 transition을 revert해야 한다.

## 11. Event relation

### 11.1 Entry

```text
empty -> Note(ELIGIBLE, q_mass, a_rec, e = 0)
```

Circuit은 다음을 검사해야 한다.

```text
q_mass > 0
0 <= a_rec <= q_mass
e = 0
assetRole = ELIGIBLE
```

Contract는 caller가 trusted Entry Issuer인지 확인해야 한다.

Entry의 encrypted parent payload는 empty list에 binding돼야 한다.

### 11.2 Transfer

```text
Note_in -> Voucher_T + Note_change
q_in = q_T + q_C
q_T > 0
q_C >= 0
```

Credit 배분은 다음과 같아야 한다.

```text
a_C = floor(a_in * q_C / q_in)
a_T = a_in - a_C
```

Inherited carbon과 transport carbon은 다음과 같아야 한다.

```text
e_C = floor(e_in * q_C / q_in)
e_T = e_in - e_C + delta_e_transport
delta_e_transport >= 0
```

Voucher와 change Note는 DocumentHash와 ELIGIBLE role을 보존해야 한다.

전량 Transfer도 zero-valued change Note를 생성해야 한다.

### 11.3 Proceed

Proceed는 receiver가 unresolved Voucher를 소비해야 한다.

Output Note의 DocumentHash, role, q_mass, a_rec와 e는
Voucher payload와 같아야 한다.

Proceed는 Voucher가 Recall 또는 Proceed로 이미 해결되지 않았는지 확인해야 한다.

### 11.4 Recall

Recall은 sender가 unresolved Voucher를 소비해야 한다.

Output Note의 DocumentHash, role, q_mass, a_rec와 e는
Voucher payload와 같아야 한다.

Recall은 configured deadline 이전에만 허용해야 한다.

Deadline 이후 unresolved Voucher에는 Proceed만 허용해야 한다.

### 11.5 Merge

Merge는 같은 owner와 DocumentHash를 가진 두 ELIGIBLE Note를 소비해야 한다.

```text
q_out = q_1 + q_2
a_out = a_1 + a_2
e_out = e_1 + e_2
```

다른 DocumentHash를 혼합하는 transition은 Process를 사용해야 한다.

### 11.6 Split

Split은 하나의 ELIGIBLE Note를 두 Note로 나눠야 한다.

```text
q_in = q_1 + q_2
a_in = a_1 + a_2
e_in = e_1 + e_2
```

Output 2는 floor allocation을 사용해야 한다.

```text
a_2 = floor(a_in * q_2 / q_in)
e_2 = floor(e_in * q_2 / q_in)
a_1 = a_in - a_2
e_1 = e_in - e_2
```

두 output은 owner, DocumentHash와 ELIGIBLE role을 보존해야 한다.

질량 0 output은 허용하며 그 output의 a_rec와 e는 0이어야 한다.

### 11.7 Process

Process profile은 finite `M_max`와 `N_max`를 고정해야 한다.

```text
1 <= m <= M_max
1 <= n <= N_max
```

Inactive slot은 profile의 canonical zero padding을 사용해야 한다.

모든 active input은 ELIGIBLE이어야 한다.

각 Process policy는 active output 중 하나 이상을 ELIGIBLE로 고정해야 한다.

전체 폐기는 Process가 아니라 Exit로 표현해야 한다.

질량 relation은 다음과 같아야 한다.

```text
Q_in = sum(q_input)
Q_out = sum(q_eligible_output) + sum(q_waste_output)
Q_in = Q_out + q_loss
q_loss >= 0
```

Credit relation은 다음과 같아야 한다.

```text
sum(a_input) = sum(a_eligible_output)
0 <= a_eligible_output[j] <= q_eligible_output[j]
a_waste_output[j] = 0
```

Eligible output 사이의 credit allocation은 private일 수 있다.

Carbon relation은 다음과 같아야 한다.

```text
E_total = sum(e_input) + delta_e_process
sum(e_eligible_output) = E_total
e_waste_output[j] = 0
delta_e_process >= 0
```

Carbon은 eligible output 질량에 비례해 배분해야 한다.

첫 번째 eligible output이 rounding residual을 받아야 한다.

Policy circuit은 각 active output의 AssetRole을 고정해야 한다.

Prover가 output 값을 본 뒤 WASTE role을 선택하면 안 된다.

Output DocumentHash는 input과 달라질 수 있다.

v1은 material compatibility를 검사하지 않는다.

### 11.8 Exit

Exit는 Note를 successor 없이 terminal하게 소비해야 한다.

ELIGIBLE과 WASTE Note를 모두 Exit할 수 있다.

WASTE Note가 사용할 수 있는 유일한 Event는 Exit다.

Exit의 outputRefs는 empty list여야 한다.

### 11.9 Issue

Issue는 ELIGIBLE final Note를 한 번 소비해야 한다.

Policy circuit은 다음 predicate를 검사해야 한다.

```text
q_mass > 0
0 <= a_rec <= q_mass
a_rec / q_mass >= tau_rec
e / q_mass <= tau_carbon
```

Division은 허용하지 않는다.

Profile의 threshold scale을 사용한 cross multiplication을 적용해야 한다.

Issue circuit은 final Note의 DocumentHash와 `d`가 같음을 증명해야 한다.

Issue circuit은 `h`가 정확한 domain-separated hash임을 증명해야 한다.

Issue는 public ClaimRef(h)를 등록해야 한다.

## 12. DPP verification

DPP verifier는 다음 절차를 수행해야 한다.

1. zkDPP claim entry를 제외한 canonical DPP core를 만든다.
2. Profile의 canonicalization과 hash로 `d`를 계산한다.
3. issuePolicyRef, d와 claimNonce로 `h`를 다시 계산한다.
4. 계산한 h가 DPP claim entry의 h와 같은지 확인한다.
5. ClaimRef(h)가 on-chain Issue output으로 등록됐는지 확인한다.
6. ClaimRef(h)의 current status가 Active인지 확인한다.

Frozen 또는 Revoked Claim을 valid로 표시하면 안 된다.

Policy disable은 이미 발행된 Claim을 자동으로 invalid로 만들면 안 된다.

## 13. Off-chain backward audit

### 13.1 Live-object audit

1. Status Authority가 조사할 live AuditRef를 외부적으로 식별한다.
2. Authority가 대상 상태를 Frozen으로 변경한다.
3. Authority가 producerOf에서 시작 AuditRecord를 조회한다.
4. K명의 custodian이 record ciphertext의 partial decryption을 만든다.
5. Authority가 share를 검증하고 결합해 parent refs를 얻는다.
6. Authority가 각 parent의 producer record를 Entry까지 반복 조회한다.
7. 외부 절차가 graph와 별도 증거를 평가한다.
8. 문제없으면 target을 Active로 되돌린다.
9. 문제가 확인되면 target을 Revoked로 변경한다.

### 13.2 Historical audit

Historical audit은 Contract의 audit-open call을 사용하지 않는다.

Authority는 외부적으로 선택한 auditRecordId에서 같은 절차를 시작한다.

Historical audit은 어떤 AuditRef status도 자동으로 바꾸지 않는다.

Case ID, partial-decryption request, plaintext graph와 evidence는
off-chain audit system이 관리한다.

## 14. Implementation profile

회로 key와 canonical vector를 생성하기 전에 모든 PROFILE을 닫아야 한다.

선택은 profile manifest와 Decision Log에 기록해야 한다.

| Profile ID | 선택해야 하는 값 | 기본 후보 또는 제약 |
|---|---|---|
| PROFILE-PROOF | proof system, curve, setup | 현 POC의 PLONK-KZG/BLS12-381 재사용 후보 |
| PROFILE-HASH | hash, domain encoding | 현 POC의 Poseidon2 재사용 후보 |
| PROFILE-NUMERIC | scale, bit width, bounds | non-negative, no field wraparound 필수 |
| PROFILE-ARITY | M_max, N_max, padding | protocol은 bounded arbitrary m,n 요구 |
| PROFILE-MEMBERSHIP | Note/Voucher tree | private leaf membership 지원 필수 |
| PROFILE-STATUS | authenticated dictionary | private Active lookup과 on-chain update 필수 |
| PROFILE-TE | K-of-N encryption | in-circuit Enc와 verifiable share 필수 |
| PROFILE-AUDIT-ENC | AuditRef, parent slots | type-safe canonical encoding 필수 |
| PROFILE-AUTH | role, scope credential | Entry, Policy, Status authority 분리 필수 |
| PROFILE-DPP | canonical bytes와 hash | claim entry 제외 규칙 필수 |
| PROFILE-NONCE | claimNonce | uniform generation과 canonical encoding 필수 |
| PROFILE-EVENT | EventKind encoding | circuit, registry, contract에서 동일해야 함 |

Profile 선택이 policy relation 또는 public-input manifest를 바꾸면
새 PolicyRef와 VK를 생성해야 한다.

### 14.1 PROFILE-TE acceptance conditions

TE 후보는 다음 조건을 모두 만족해야 한다.

- K명 미만 share로 plaintext를 복구할 수 없음
- K개 valid share로 deterministic decryption 가능
- malformed share 검증 또는 명시적 honest-share assumption
- parent vector와 parentCount의 canonical encoding
- circuit에서 encryption correctness 증명 가능
- constraint와 public ciphertext size 측정 가능
- key share를 재구성하지 않는 partial-decryption interface

### 14.2 PROFILE-STATUS acceptance conditions

Status structure 후보는 다음 조건을 모두 만족해야 한다.

- 없는 key를 Active로 증명 가능
- Frozen과 Revoked leaf를 구분 가능
- private key lookup path를 circuit에서 검증 가능
- current root를 contract가 원자적으로 갱신 가능
- 이전 root를 transaction acceptance에서 거부 가능
- AuditRef type collision을 방지 가능

## 15. Conformance tests

### 15.1 Positive tests

구현은 최소 다음 positive flow를 제공해야 한다.

- Entry부터 Issue까지 모든 Event를 포함한 canonical scenario
- Transfer의 Proceed path와 Recall path
- Process의 서로 다른 active m,n profile
- ELIGIBLE과 WASTE output이 함께 존재하는 Process
- live Note, Voucher와 Claim의 freeze/unfreeze
- live object freeze 후 backward audit
- historical record에서 시작하는 backward audit
- policy disable 전 승인과 disable 후 신규 proof 거부

### 15.2 Negative tests

구현은 최소 다음 공격을 거부해야 한다.

- forged Note 또는 Voucher opening
- wrong owner secret
- duplicate Note nullifier
- duplicate Voucher resolution
- Frozen Note consumption
- Frozen Voucher Proceed 또는 Recall
- Frozen 또는 Revoked Claim verification
- historical statusRoot replay
- unauthorized status update
- direct Active to Revoked update
- Revoked to Active update
- disabled PolicyRef 사용
- wrong EventKind policy 사용
- missing PolicyGrant
- proof와 다른 policyScopeRef 제출
- input보다 큰 mass output
- credit creation 또는 omission
- WASTE output에 credit 또는 carbon allocation
- wrong proportional rounding
- wrong transport 또는 process carbon total
- prover-chosen WASTE role
- actual parents와 다른 encryptedParents
- insufficient or invalid partial-decryption shares
- DPP core mismatch
- claimNonce 또는 issuePolicyRef mismatch
- final Note의 재사용 Issue

## 16. Required implementation artifacts

구현 담당자는 최소 다음 artifact를 제공해야 한다.

1. 확정된 implementation profile manifest
2. 각 Event의 circuit source와 public-input manifest
3. native reference implementation과 canonical vectors
4. proving/verifying key generation 절차
5. Router, Registry와 StatusTree contract source
6. AuditRecord encoder와 off-chain audit client
7. threshold setup, partial-decryption과 combine tooling
8. positive/negative conformance tests
9. constraint, proving time, proof size와 verification gas benchmark
10. current POC와 새 v1의 migration/change matrix

## 17. Current POC change boundary

현재 `poc-v2/`에서 새 v1으로 이동할 때 최소 다음을 변경해야 한다.

1. item-count Quantity를 제거하고 q_mass로 통일한다.
2. StateVector를 assetRole, q_mass, a_rec와 e 의미로 교체한다.
3. Event별 mass, credit와 carbon relation을 적용한다.
4. public consumed commitment와 Voucher ID를 private witness로 옮긴다.
5. current statusRoot의 private Active proof를 추가한다.
6. AuditRecord, encryptedParents와 producerOf를 추가한다.
7. Issue circuit, ClaimRef와 DPP verification을 추가한다.
8. policy-specific VK Registry와 PolicyGrant를 추가한다.
9. one-way policy disable과 AuditRef status update를 추가한다.
10. threshold partial-decryption audit client를 추가한다.

이 목록은 구현 순서를 강제하지 않는다.

구현 담당자는 dependency와 실험 위험에 따라 milestone을 정할 수 있다.
