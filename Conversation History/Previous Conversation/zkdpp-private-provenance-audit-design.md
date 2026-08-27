# zkDPP 비공개 provenance 감사와 선택적 동결 설계

> 상태: 대화 기반 설계 초안.
>
> 이 문서는 현재 POC의 구현 사실, 이번 대화에서 합의한 설계 결정,
> 그리고 아직 선택하지 않은 항목을 구분한다.
>
> 이 문서의 목적은 구현 명세를 대체하는 것이 아니라,
> 이후 Scheme와 POC를 수정할 때 사용할 논리적 기준을 제공하는 것이다.

## 1. 문제와 목표

zkDPP는 공급망 상태 전이가 승인된 정책을 따랐음을 증명한다.

그러나 정상 운영 중에는 입력과 출력의 연결을 공개하지 않아야 한다.

문제가 발견됐을 때에는 전체 원장을 되돌리거나 블록을 삭제하지 않는다.

대신 문제 전이의 의존 경로를 역방향으로 조사하고,
필요한 객체만 일시 동결하거나 최종 철회한다.

이 설계의 목표는 다음 네 가지다.

1. 제품·batch ID, 수량, 재활용 credit, 탄소 배출량,
   거래 상대방, 일반적인 parent-child 연결을 숨긴다.
2. 각 공급망 공정이 Policy Authority가 승인한 정책을 통과했음을 증명한다.
3. 문제가 된 전이에서 시작해 K-of-N 감사 위원회가
   과거 의존 그래프를 역추적할 수 있게 한다.
4. 조사 중에는 직접 영향을 받는 객체를 동결하고,
   정상 claim은 보존한다.

이 설계는 물리 세계의 측정값이 참이라는 사실을 새로 증명하지 않는다.

초기 입력량, 재활용 credit, 공정 배출량 등의 실세계 값은
정상적으로 수집되고 해당 전이에 올바르게 연결됐다고 가정한다.

영지식 증명이 보장하는 대상은 이 입력을 전제로 한 비공개 계산의 정확성,
정책 준수, 이중 소비 방지, 그리고 감사 연결 정보의 무결성이다.

## 2. 설계 경계와 비목표

### 2.1 삭제나 전체 롤백은 하지 않는다

블록체인에 이미 기록된 transaction이나 block을 제거하지 않는다.

문제 대응은 이후 전이와 claim의 사용 가능 여부를 바꾸는 방식으로 수행한다.

### 2.2 역방향 추적이 첫 번째 범위다

첫 스킴은 문제 transaction에서 과거 부모 전이로 향하는
backward tracing을 제공한다.

확정된 원인에서 모든 하류 descendant를 자동으로 찾는
forward tracing은 후속 과제다.

따라서 audit 전에 이미 소비된 출력의 하류 객체를
자동으로 동결하지는 못한다.

### 2.3 물리 제품에서 시작 transaction을 찾는 일은 외부 범위다

Status Authority는 외부 사건 조사 등을 통해
문제가 된 블록체인 transaction을 식별했다고 가정한다.

제품 또는 batch ID에서 해당 transaction을 검색하는 방식,
off-chain custody mapping, 검색 권한은 이 스킴의 범위 밖이다.

### 2.4 현실 표준은 모델의 동기와 추상화 출처다

이 연구는 상용 제품을 출시하거나 특정 규제 적합성을 인증하는 작업이 아니다.

EU Battery Passport는 DPP가 sustainability 정보를 다룰 필요가 있다는 동기를 제공한다.

ISCC PLUS Mass Balance는 혼합 물질의 credit allocation, 이중 계산 방지,
손실 반영 같은 CoC accounting pattern의 현실적 출처를 제공한다.

첫 스킴은 ISCC PLUS 전체 규칙이나 EU Battery Regulation 전체를 구현하지 않는다.

따라서 이 문서의 `(q, a_rec, e)`는
ISCC PLUS-inspired simplified policy model을 위한 연구 상태다.

이 값만으로 특정 Battery Passport 규제 claim을 충족한다고 주장하지 않는다.

다른 CoC policy나 DPP profile은 더 많은 상태, 다른 분모,
다른 claim 및 접근 규칙을 요구할 수 있다.

### 2.5 state 선정의 연구 논리

state가 갖춰야 할 성질만으로는 어떤 값을 state로 넣을지 결정할 수 없다.

반대로 현실 표준의 모든 field를 그대로 구현할 필요도 없다.

이 연구는 현실 DPP와 CoC에서 관찰되는 핵심 tension을 선택해
분석 가능한 simplified model로 만든다.

```text
현실의 DPP 및 CoC 요구
  → 연구할 accounting과 privacy tension 선택
  → simplified policy와 claim 정의
  → 그 claim을 계산·보존·배분·중복 방지하는 최소 state 선택
  → policy-specific circuit/VK로 enforcement
```

이 흐름에서 현실 표준은 claim과 accounting pattern의 동기를 제공한다.

연구자는 모든 표준 field가 아니라 다음을 함께 드러내는 최소 상태를 선택한다.

- 질량의 보존, 분할, 합병 및 비율 claim
- 재활용 credit의 allocation과 이중 계산 방지
- 공정 및 운송을 거치는 sustainability metric의 누적
- 비공개 상태 전이와 최종 claim binding
- 문제가 생겼을 때의 선택적 freeze와 backward audit

따라서 `(q, a_rec, e)`의 선택은 완전한 외부 표준의 재현을 뜻하지 않는다.

이 세 값은 위 tension을 동시에 표현하는 의도적인 연구 모델의 최소 state다.

논문은 외부 표준이 이 정확한 세 값만을 요구한다고 주장하지 않는다.

논문은 다음 범위의 주장을 사용한다.

> 본 연구는 Mass Balance CoC에서 나타나는 allocation과 이중 계산 방지,
> 그리고 DPP sustainability claim의 비공개 binding을 연구하기 위한
> ISCC-inspired simplified private accounting model을 제안한다.

### 2.6 모델이 주장하지 않는 것

이 모델은 EU Battery Regulation 또는 ISCC PLUS 전체 준수를 증명하지 않는다.

이 모델은 실제 battery의 법정 carbon footprint나
material-specific recycled-content ratio를 완전하게 계산한다고 주장하지 않는다.

그런 주장을 하려면 별도의 규제 방법론, site-period accounting,
material vector, 측정 증빙 및 access rule을 추가해야 한다.

## 3. 역할과 신뢰 경계

### 3.1 Policy Authority

Policy Authority는 산업 정책을 승인 가능한 circuit constraint로 해석한다.

정책 식별자와 version에 대응하는 circuit 및 VK를 승인하고 Registry에 등록한다.

정책의 숫자, 단위, fixed-point scale, 반올림 규칙이 달라지면
새 policy/version과 새 VK를 등록한다.

### 3.2 Operator

Operator는 자신이 보유한 비공개 상태를 witness로 사용해
승인된 정책 circuit의 proof를 생성한다.

정확한 배합비, 거래 상대방, 수량, 배출량은 공개하지 않는다.

### 3.3 Status Authority

Status Authority는 외부 절차를 통해 문제 transaction을 식별한다.

그 Authority는 audit을 열고 직접 출력 객체를 동결하며,
조사 후 외부 판단에 따라 동결 해제 또는 철회를 수행한다.

이 Authority의 거버넌스, 판단 기준, 호출 권한 부여 방식은
첫 스킴의 암호학적 범위 밖이다.

### 3.4 K-of-N 감사 위원회

고정된 감사 위원회는 Setup 시 한 번 정한다.

위원의 교체, 탈퇴, 키 회전은 첫 스킴에서 다루지 않는다.

K명 이상이 협력해야 AuditRecord의 암호화된 부모 참조를 복호화할 수 있다.

위원회는 의존 그래프를 복원할 뿐,
물리적 원인이나 철회 여부를 판정하지 않는다.

### 3.5 구매자와 DPP 검증자

구매자는 자신의 직접 거래 범위에서만 필요한 정보를 얻는다.

권한 있는 구매자 또는 감사자는 자신이 제공받은 claim handle을 사용해
claim의 유효 상태를 확인할 수 있다.

소비자나 일반 관찰자에게 제품과 claim handle의 대응을 제공하지 않는다.

## 4. 정책 circuit과 Registry

정책마다 제약이 다르므로 Policy Authority는 정책별 circuit/VK를 승인한다.

각 circuit은 전이의 기본 형식과 해당 정책의 도메인 제약을 함께 검증한다.

```text
PolicyRef = (event kind, policy identifier, version, approved VK)
```

예를 들어 서로 다른 Process 정책은 서로 다른 circuit/VK를 가진다.

```text
C(Process, recycling-step-A, v1)
C(Process, refining-step-B, v3)
C(Issue, battery-sustainability, v2)
```

Merkle membership, nullifier 정확성, 동결 상태 검사 같은 검사는
여러 circuit에 공통으로 포함될 수 있다.

이는 구현에서 공용 gadget으로 재사용할 수 있다는 뜻일 뿐이다.

각 정책 circuit은 여전히 이 검사를 자신의 relation 안에서 증명한다.

Proceed와 Recall처럼 도메인별 수율 규칙이 없는 전이는
하나의 공통 정책 version을 재사용할 수 있다.

모든 event가 서로 다른 숫자 parameter를 가져야 하는 것은 아니다.

PolicyRef와 version은 온체인에서 공개한다.

이는 어떤 승인 정책을 사용했는지에 대한 coarse metadata를 드러내지만,
입력과 출력의 연결이나 제품 정체성을 드러내지는 않는 것으로 본다.

## 5. 비공개 상태와 최종 sustainability claim

### 5.1 상태의 의미

첫 simplified research policy의 비공개 상태는 다음 세 값으로 표현한다.

```text
q      : 최종 제품 또는 batch의 전체 질량
a_rec  : 물리적 재활용량이 아닌 Mass Balance 재활용 credit allocation
e      : Entry부터 현재 note까지 누적된 탄소 배출량
```

`q`는 전체 제품 또는 batch를 분모로 사용한다.

특정 부품 또는 소재 하위 범위를 따로 분모로 두지 않는다.

Entry에서 `e = 0`으로 시작한다.

Entry 이전의 embodied carbon은 첫 profile의 경계 밖이다.

Transfer는 transport로 인한 비공개 `Δe_transport`를 `e`에 더한다.

Merge와 Split은 기존의 단순 비례 배분 모델을 유지한다.

Process의 기존 policy 의미와 allocation 규칙은 이번 설계에서 변경하지 않는다.

### 5.2 Issue policy의 최종 predicate

Issue는 최종 note가 다음 predicate를 만족함을 비공개로 증명한다.

\[
q > 0,
\qquad 0 \le a_{rec} \le q,
\qquad \frac{a_{rec}}{q} \ge \tau_{rec},
\qquad \frac{e}{q} \le \tau_{carbon}.
\]

재활용 기준과 탄소 기준은 독립적으로 모두 만족해야 한다.

공개되는 것은 두 값을 통과했다는 하나의 claim 사실이다.

`q`, `a_rec`, `e`, 비율, 세부 배출량은 공개하지 않는다.

`τ_rec`, `τ_carbon`, 단위, scale, 반올림 규칙은
특정 Issue policy/version의 circuit과 VK에 고정한다.

논문은 이 predicate를 policy-specific parameter의 일반 형식으로 사용한다.

특정 규제상 임계값을 구현하거나 적합성을 주장하려면,
그때 선택한 외부 policy profile의 근거와 함께 수치를 정해야 한다.

## 6. Document, Note, Voucher, Claim

### 6.1 기존 Document의 의미를 유지한다

`DocumentInfo`는 제품명, Lot ID, 단위 같은 제품 설명이다.

```text
DocumentHash = H(DocumentInfo)
```

DocumentHash는 제품 설명의 binding 값이다.

이는 동결 또는 감사의 단위가 아니다.

### 6.2 Note는 소비 가능한 비공개 장부 상태다

```text
Note = (cm, DocumentHash, q, state, opening, owner)
cm   = H(DocumentHash, q, state, owner, opening)
```

같은 DocumentHash를 가진 batch라도 Split, Transfer, Merge, Process 뒤에는
각기 새로운 commitment를 가진 서로 다른 Note가 만들어진다.

따라서 문제가 있는 특정 장부 상태를 지칭할 때는
DocumentHash가 아니라 해당 Note commitment가 필요하다.

### 6.3 Voucher와 Claim

Voucher는 Transfer 후 Proceed 또는 Recall을 기다리는 임시 객체다.

기존 POC의 voucher commitment를 `rv`로 표기한다.

Issue는 최종 note를 소비하고 claim handle `h`를 생성한다.

```text
h = H("zkDPP:Issue", cm_final, issuePolicyRef, o_claim)
```

`cm_final`과 새 무작위값 `o_claim`은 private witness다.

`h`는 공개되지만, 무작위값 때문에 공개 `h`만으로
최종 note나 제품/batch를 연결할 수 없어야 한다.

Issue proof는 `h`의 계산, 최종 note의 단일 소비,
그리고 Issue policy predicate 충족을 함께 증명한다.

### 6.4 Exit와 Issue의 구분

일반 Exit는 다음 의미를 가진다.

```text
Note → ∅
```

이는 폐기, 범위 이탈, claim 없이 종료하는 경우에 사용한다.

Issue는 DPP claim을 발행하는 특수한 종료 전이다.

```text
Note → Claim(h)
```

DPP claim을 발행하는 경로에서는 Exit를 먼저 수행하지 않고 Issue를 수행한다.

## 7. 감사와 상태 관리에 사용하는 AuditRef

Document와 혼동하지 않기 위해 감사·동결 대상은 `AuditRef`로 표기한다.

```text
AuditRef ∈ {
  NoteRef(cm),
  VoucherRef(rv),
  ClaimRef(h)
}
```

AuditRef는 새 비즈니스 객체가 아니다.

기존 Note, Voucher, Claim 중 어느 객체를 감사 또는 상태 관리하는지
명확히 가리키는 typed reference다.

구현에서는 `(type, rawID)` 또는 이 값의 domain-separated hash를
상태 tree와 mapping의 key로 사용할 수 있다.

정확한 직렬화는 구현 단계에서 정한다.

## 8. 전이와 AuditRecord

### 8.1 모든 전이에 하나의 AuditRecord를 남긴다

다음 아홉 전이는 각각 하나의 AuditRecord를 등록한다.

```text
Entry, Transfer, Proceed, Recall, Merge, Split, Process, Exit, Issue
```

Proceed와 Recall에도 record가 필요하다.

이 기록이 없으면 voucher에서 만들어진 note의 역추적이 끊긴다.

Exit에도 record가 필요하다.

Exit 자체가 의심 transaction일 수 있기 때문이다.

### 8.2 AuditRecord의 의미

AuditRecord는 전이마다 하나만 존재하는 온체인 감사 record다.

별도 protocol object로서 TraceCipher를 두지 않는다.

암호화된 부모 연결은 AuditRecord 안의 필드다.

```text
AuditRecord = {
  auditRecordId,
  policyRef,
  outputRefs,
  encryptedParents
}
```

`auditRecordId`는 contract가 성공한 전이에 부여하고 event로 남기는 식별자다.

Authority는 외부적으로 식별한 blockchain transaction의 receipt에서
이 식별자를 읽어 audit을 시작한다.

따라서 chain transaction hash와 audit record lookup key를 혼동하지 않는다.

`outputRefs`는 새로 생성된 NoteRef, VoucherRef, ClaimRef 목록이다.

`encryptedParents`는 실제 부모 AuditRef 목록의 K-of-N threshold encryption이다.

```text
encryptedParents = TE.Enc(PK_committee, actualParentAuditRefs)
```

`TE`의 구체 primitive는 아직 선택하지 않는다.

### 8.3 암호문은 proof에 결박돼야 한다

전이 proof는 다음을 함께 증명해야 한다.

```text
encryptedParents가 이번 전이에서 실제로 소비한
private parent AuditRef 목록을 암호화했다.
```

이 결박이 없으면 Operator는 정상 상태 전이 proof와 함께
전혀 다른 부모 목록을 암호문에 넣을 수 있다.

그러면 감사 때 잘못된 계보가 복원된다.

암호화 정확성은 주 transition proof 회로에서 증명한다.

구체 encryption primitive는 이 회로 부담을 고려해 나중에 선택한다.

### 8.4 공개 producer index

contract는 각 출력에 대해 다음 lookup을 유지한다.

```text
producerOf[AuditRef] = auditRecordId
```

이는 어떤 객체를 만든 전이를 찾게 해 준다.

부모 목록은 여전히 `encryptedParents` 안에 있으므로,
일반 관찰자는 입력에서 출력으로 향하는 edge를 알 수 없다.

다만 같은 전이에서 함께 생성된 sibling output의 묶음은 공개된다.

이 leakage는 허용한다.

## 9. 공개 statement와 privacy boundary

ZK transaction은 proof와 public input을 온체인 verifier에 제출한다.

Issue를 포함한 각 전이는 개념적으로 다음을 공개한다.

```text
proof
current note/voucher membership root
current status root
nullifier
정책 종류와 PolicyRef/version
새 output AuditRef
AuditRecord와 auditRecordId
```

Issue는 여기에 `h`를 공개 output으로 포함한다.

반면 다음 값은 private witness로 남는다.

```text
소비하는 cm 또는 rv
Merkle path와 note/voucher opening
q, a_rec, e, Δe_transport
제품/batch ID와 DocumentInfo
Operator와 거래 상대방 정보
실제 부모 AuditRef 목록
```

현재 POC는 `cm_in`과 같은 소비 commitment를 public input으로 사용한다.

새 설계는 이를 private witness로 옮겨야 한다.

그렇지 않으면 public observer가 input-output edge를 직접 구성할 수 있다.

공개 chain에서는 다음 익명 metadata가 남는다.

```text
proof, 정책/version, root, nullifier, output AuditRef,
AuditRecord 생성 시점, claim 상태 변경 시점
```

특정 policy의 사용 빈도, 시점, 익명 claim의 동결 또는 철회는
외부 정보와 결합될 경우 추론 단서가 될 수 있다.

첫 설계는 이 metadata leakage를 감수한다.

## 10. 동결과 철회를 위한 StatusTree

### 10.1 왜 별도 상태 tree가 필요한가

새 설계에서 소비되는 NoteRef 또는 VoucherRef는 private witness다.

따라서 contract는 공개 mapping만으로
그 객체가 Frozen인지 직접 조회할 수 없다.

입력 ID를 다시 공개하면 parent-child privacy가 깨진다.

그래서 소비 proof 안에서 다음을 증명해야 한다.

```text
내가 소비하는 private AuditRef는
현재 StatusTree에서 Active다.
```

### 10.2 예외만 기록하는 상태 tree

StatusTree는 AuditRef를 key로 하는 authenticated 상태 사전이다.

```text
tree에 항목 없음     → Active
tree에 Frozen 기록   → Frozen
tree에 Revoked 기록  → Revoked
```

새 note, voucher, claim은 기본적으로 Active다.

따라서 정상 전이가 새 객체를 만들 때마다 Active leaf를 쓰지 않는다.

Status Authority가 audit을 열 때 Frozen entry를 기록한다.

동결 해제는 다시 기본 Active 상태로 되돌린다.

최종 철회는 Revoked entry를 남긴다.

Note와 Voucher의 실제 존재 여부는 각자의 note/voucher membership proof가 보장한다.

Claim은 Issue로 등록된 `h`인지도 별도로 확인해야 한다.

### 10.3 현재 root만 허용한다

소비 proof는 반드시 contract의 현재 `statusRoot`에 대해
Active 상태를 증명해야 한다.

동결 전의 과거 root를 허용하면 보유자가 오래된 Active path를 이용해
Frozen 객체를 소비할 수 있다.

### 10.4 audit 시작 시의 동결 범위

Status Authority는 다음 호출을 수행한다.

```text
beginAudit(caseID, auditRecordId)
```

contract는 해당 AuditRecord의 모든 직접 `outputRefs`에
Frozen 상태를 기록한다.

이미 소비된 output에도 Frozen entry는 기록될 수 있다.

그 경우에는 이미 진행된 하류 전이를 되돌리지 못한다.

아직 소비되지 않은 직접 output은 이후 전이에서
Active proof를 만들 수 없으므로 멈춘다.

이 한계는 forward tracing을 보류한 설계와 일치한다.

### 10.5 성능 상태

StatusTree는 각 소비 입력에 대해 추가 authenticated path를 요구한다.

Merge와 Process처럼 입력이 여러 개인 전이는 그만큼 path가 늘어난다.

상태 갱신과 proof 비용이 실용적인지는 아직 Unknown이다.

이 설계는 privacy를 유지한 freeze 집행을 위한 요구사항이며,
향후 constraint 수, proving time, verification gas를 측정해야 한다.

## 11. audit 절차

1. Status Authority는 외부적으로 문제 blockchain transaction을 식별한다.
2. Authority는 receipt에서 `auditRecordId`를 얻는다.
3. Authority는 `beginAudit(caseID, auditRecordId)`를 호출한다.
4. contract는 시작 record의 직접 output을 Frozen으로 기록한다.
5. K명 이상의 감사 위원회가 해당 AuditRecord를 복호화한다.
6. 복호화된 parent AuditRef마다 `producerOf`를 조회한다.
7. 조회된 이전 AuditRecord를 복호화하며 Entry까지 반복한다.
8. 외부 절차는 복원된 그래프와 별도 증거를 바탕으로
   동결 해제 또는 철회를 결정한다.

감사 결과는 물리적 오염의 최초 원인을 암호학적으로 단정하지 않는다.

그래프는 어떤 온체인 전이들이 의존했는지를 보여 준다.

원인 판단과 법적·운영적 조치는 외부 절차의 책임이다.

## 12. DPP claim의 상태와 접근

Issue가 성공하면 ClaimRef(h)는 기본 Active 상태다.

동결 또는 철회된 claim은 `h`를 통해 검증할 때 사용 가능하지 않다.

공개 blockchain에서는 `h`, policy/version, claim의 상태 변화가 관찰 가능하다.

이것은 claim 내용이나 제품 identity를 공개하지 않지만,
익명 claim의 존재와 상태 변화라는 metadata를 남긴다.

첫 설계는 이 leakage를 허용한다.

권한 있는 구매자 또는 감사자에게만 실제 제품/batch와 `h`의 대응을 제공한다.

이 대응 정보의 전달, 저장, 접근 통제는 off-chain 범위다.

`h`를 알고 있는 제3자는 공개 원장에서 상태를 직접 조회할 수 있다.

`h`를 알아도 검증을 막아야 하는 강한 access control은
permissioned ledger, encrypted claim, credential 또는 gateway를 추가로 요구한다.

이는 첫 설계의 범위 밖이다.

## 13. 현재 POC와의 관계

현재 POC는 Entry, Transfer, Proceed, Recall, Merge, Split, Process,
Exit의 여덟 전이를 구현한다.

새 설계는 기존 Process policy의 의미를 변경하지 않는다.

다만 다음 구현 변경이 필요하다.

1. Issue circuit과 ClaimRef(h) registry를 추가한다.
2. 소비 commitment와 voucher ID를 public input에서 private witness로 옮긴다.
3. 각 전이에 AuditRecord와 proof-bound `encryptedParents`를 추가한다.
4. `producerOf` index와 contract-assigned `auditRecordId`를 추가한다.
5. current StatusTree root와 private Active proof를 각 소비 relation에 추가한다.
6. Status Authority의 freeze, unfreeze, revoke 상태 갱신을 추가한다.
7. Transfer에 비공개 `Δe_transport` 반영을 추가한다.

이 목록은 구현 계획이 아니라 설계에서 요구되는 차이를 나타낸다.

각 변경의 constraint, proving time, gas 영향은 측정 전까지 Unknown이다.

## 14. 미해결점과 후속 검증

### 14.1 추상 설계에서 남은 항목

- `AuditRef`, AuditRecord, StatusTree의 정확한 encoding과 ABI
- Status Authority의 실제 거버넌스와 호출 권한
- 현실 제품/batch와 claim handle 또는 suspect transaction의 대응 절차

특정 규제 적합성 또는 ISCC PLUS 전체 준수를 주장하려는 경우에는,
별도로 실제 policy profile, threshold, 분모, 측정 경계를 근거와 함께 정해야 한다.

### 14.2 구현 및 실험에서 결정할 항목

- K-of-N threshold encryption primitive와 key format
- 주 circuit 안의 encryption relation 구현 방식과 비용
- StatusTree의 구체 구조, depth, hash, update proof 형식
- circuit별 추가 constraint, proving time, verification gas
- policy circuit과 VK Registry의 구체 storage 및 version 관리

### 14.3 명시적으로 보류한 기능

- confirmed root에서 모든 downstream descendant를 찾는 forward tracing
- 이미 소비된 하류 객체를 자동으로 freeze 또는 revoke하는 기능
- 실세계 측정값의 oracle, issuer attestation, sensor provenance 검증
- `h` 자체에 대한 강한 on-chain access control

## 15. 요약

정상 운영에서는 각 정책 전이가 비공개 상태를 소비하고 새 상태를 만든다.

일반 관찰자는 승인된 policy/version, proof, 익명 output, 시점만 본다.

입력과 출력의 연결, 제품 identity, 수량과 sustainability 값은 숨긴다.

각 전이는 proof-bound AuditRecord를 남긴다.

문제가 된 transaction이 외부적으로 식별되면 Status Authority는
그 record의 직접 출력을 우선 동결한다.

K-of-N 감사 위원회는 암호화된 부모 참조를 열고,
producer index를 따라 과거 의존 그래프를 복원한다.

조사 결과에 따른 해제 또는 철회는 외부 판단에 맡긴다.
