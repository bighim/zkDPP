# zkDPP v2 프로토콜 명세

## 1. 목적과 명세 범위

zkDPP는 공급망의 비공개 회계 상태를 Note와 Voucher로 표현하고, 각 생산·유통
전이가 해당 Event의 고정 규칙 또는 등록된 정책을 준수함을 영지식증명으로 검증하는 프로토콜이다.
최종 Note에서 발행한 공개 Claim은 외부 DPP의 지정된 문서 필드와 연결된다.
감사자는 외부 committee 절차에서 전달받은 K-of-N 키 조각으로 복호화하여
원장의 실제 선행·후속 흐름을 복원한다. 문제 시작점의 사용 차단이 승인되면 그 지점에서 파생된 미소비
Note·미해결 Voucher 전체를 동결하고 차단 결과를 확인한다.

이 문서는 객체, 원장 상태, 정책, 아홉 Event, DPP 검증, 양방향 추적,
감사 후 동결 및 보안 요구사항을 정의한다. 규범적 규칙은 이 문서 안에
기술한다. “해야 한다”와 알고리즘의 `require`는 필수 조건이다.
배포에 필요한 구체 암호 알고리즘과 encoding 등의 매개변수는 제16장의
배포 profile로 고정한다. 특정 구현의 완성도나 보안 증명 완료를 뜻하지 않는다.

### 1.1 제공하려는 기능

1. 정상 운영에서 Note/Voucher의 private state와 실제 소비 parent를 숨긴다.
2. 허가된 전이만 승인하고 동일 객체의 중복 소비를 거부한다.
3. Issue 정책을 통과한 Note와 DPP의 지정 문서 필드를 공개 Claim으로 연결한다.
4. 승인된 감사에서 accepted digital graph의 선행 경로를 복원한다.
5. 확정 snapshot에서 후속 경로, 미소비 frontier, Claim 및 Exit 종단을 복원한다.
6. 차단 승인을 받은 시작점의 전체 frontier를 동결하고 공통 확정 시점에서
   대상별 결과와 전체 완료 여부를 확인한다.

### 1.2 보장 범위의 한계

검증 대상은 원장이 승인한 디지털 전이와 제출된 값 사이의 관계다.
실세계 질량·재활용 credit·탄소 값의 진실성, 공정의 실제 수행, DPP와
물리 제품의 결합, 오염 원인의 판정은 이 관계만으로 보장되지 않는다.
감사는 디지털 의존 경로를 복원하며 실제 물리적 피해 범위를 판정하지 않는다.

Snapshot 이후 생성되는 모든 descendant의 원자적 봉쇄, 완료된 소비의
rollback, Claim의 동결·철회, K명 이상 custodian의 공모 저항성,
악의적인 Status Authority의 행동에 대한 암호학적 책임 추적,
외부 위원회 절차의 항상 가능한 키 전달도 보장하지 않는다. 성능과 비용은 별도 측정 대상이다.

## 2. 역할, 소유권과 신뢰 모델

### 2.1 역할

| 역할 | 책임 |
|---|---|
| Participant | private opening과 소유권 비밀을 관리하고 Prover client로 전이를 증명한다. |
| Entry Issuer | 최초 Note 등록을 승인하고 초기 값의 적합성과 중복 등록 방지를 책임진다. |
| Policy Authority | 정책 relation·circuit·VK를 검토하고 등록, scope grant 및 정책 비활성화를 관리한다. |
| Auditor / Status Authority | 외부에서 전달받은 키 조각으로 감사하고, 감사 시작점과 차단 실행을 결정하며 Note/Voucher 상태를 변경한다. |
| Committee 구성원(custodian) | 자기 key share를 관리하고 audit 시작 전 외부 절차에 따라 auditor에게 전달한다. |
| Router/Ledger | proof와 현재 제어 상태를 검증하고 승인된 상태 갱신을 원자적으로 실행한다. |
| DPP verifier | 외부 DPP의 Claim 참조와 공개 원장을 이용해 문서·정책 결합을 확인한다. |

Prover client와 감사 client는 소프트웨어 구성요소다. 감사 client는 auditor가
사용하는 프로그램이며 별도의 참여자나 신뢰 주체가 아니다. 이 프로그램이
키 조각을 받아 복호화하는 것은 auditor가 그 키 조각으로 감사하는 것과 같다.
하나의 주체가 여러 운영 역할을 맡더라도 각 권한 검사는 유지한다.

### 2.2 소유권 lifecycle

`owner = OwnerKey(ownerSecret)`로 private 소유권 식별자를 정의한다.
소유권 증명은 해당 secret의 지식을 요구한다. `owner`를 transaction caller의
공개 주소와 자동으로 동일시하지 않는다. 제출자·issuer·relayer의 인증 방식은
배포 profile에서 명시한다.

- Entry의 생성자와 최초 Note owner는 같은 주체다.
- Merge와 Split은 input owner를 output에서 유지한다.
- Process의 모든 input과 output Note는 공정 수행 주체의 소유다.
- 회사 간 소유권 이전은 `Transfer → Voucher → Proceed`로 수행한다.
- Proceed의 output Note는 receiver가 생성하고 소유한다.
- Recall의 output Note는 sender가 생성하고 소유한다.

따라서 Note를 생성하는 prover는 그 output의 미래 nullifier를 계산할
owner secret을 안다. Transfer sender는 Voucher opening을 생성하므로
Voucher의 미래 resolution nullifier를 계산할 수 있다.

### 2.3 신뢰와 공격자

Entry Issuer의 초기 등록 판단, Policy Authority의 정책·권한 등록,
Status Authority의 시작점 선택·복호화 결과 해석·올바른 식별자 집행을 신뢰한다.
차단 실행 승인 후에는 전체 frontier가 대상이며, 일부만 선택한 결과를
`AuditAndFreeze`의 완료로 보고할 수 없다.

일반 Participant, prover, submitter, DPP publisher는 공모하거나 잘못된
witness·proof·문서를 제출할 수 있다. 비인가 공격자는 K개 미만의 committee
key share만 얻는다고 가정한다. 신뢰된 auditor에게 승인된 키 조각을 제공하는
일은 이 비인가 노출과 구별한다.

Audit은 **동일 committee key에 속하는 서로 다른 K명 이상의 유효한 키 조각을
auditor가 외부 절차로 전달받았다**는 전제에서 시작한다. Committee의 승인·인증·
안전한 전달과 전달된 조각의 정당성은 그 외부 절차의 책임이다. Audit 내부에서
구성원에게 암호문별 응답을 요청하거나 응답 증명을 받도록 요구하지 않는다.
유효한 키 전달 전제가 충족되지 않은 경우의 악의적 조각 검출은 이 감사
모델의 보장이 아니다. 식별 가능한 입력 오류와 조회·복호화 실패는 거부한다.

원장은 승인 순서, 확정 history, 인증된 상태 읽기와 원자적 transaction 실행을
제공한다고 가정한다. Indexer나 RPC가 반환한 자료를 인증 없이 참으로 간주하지
않는다. Voucher opening의 외부 전달은 비밀성·인증·전달 가용성을 가정한다.

## 3. 표기, 암호 인터페이스와 수치 규칙

### 3.1 기본 표기

```text
EventKind = Entry | Transfer | Proceed | Recall | Merge | Split | Process | Exit | Issue
AssetRole = ELIGIBLE | WASTE
Status = Active(0) | Frozen(1) | Revoked(2)
ObjectRef = NoteRef(cm) | VoucherRef(rv) | ClaimRef(h)
aid = 1부터 증가하는 accepted AuditRecord 식별자
H_t = 감사 기준의 확정된 immutable ledger prefix
H_r = 동결 결과를 확인하는 공통 확정 ledger prefix
```

`L_H[x]`는 prefix `H`에서의 원장 값이다. `current`는 transaction 실행 또는
관찰 시점의 현재 상태다. `H_t`의 값과 현재 상태를 같은 값으로 취급하지 않는다.
유효한 snapshot 식별자는 chain·ledger·확정 block hash 또는 이에 대응하는
accepted sequence를 특정해야 한다. 미확정 block number만으로 확정을 가정하지 않는다.

### 3.2 Encoding, hash와 commitment

`H(tag, ...)`는 목적별 domain과 canonical tuple encoding을 사용하는 hash다.
각 type과 필드 경계가 구분되어야 하고 동일 값의 여러 encoding을 허용하지 않는다.
논리 domain은 다음과 같다.

```text
zkDPP:Note:v2                 zkDPP:Voucher:v2
zkDPP:ObjectRef:v2            zkDPP:PolicyRef:v2
zkDPP:Nullifier:v2            zkDPP:VoucherResolutionSecret:v2
zkDPP:VoucherNullifier:v2     zkDPP:Issue:v2
```

`OwnerKey`, `HashDocumentInfo`, Merkle leaf/node hash 및 scope encoding도
용도를 혼동할 수 없게 profile에서 고정한다. Hash-to-field, 문자열 정규화,
type tag와 배열 encoding은 prover·contract·감사 client·DPP verifier에서
동일해야 한다. Hash의 충돌 저항성만으로 commitment hiding이나 nullifier
unlinkability가 성립한다고 간주하지 않는다.

Note/Voucher의 `opening`과 owner secret은 충분한 엔트로피를 가지며
profile의 유효 domain에서 생성한다. 공개 commitment에서 private payload가
숨겨지고 서로 다른 유효 opening의 동일 commitment 생성이 어렵도록 구성한다.

### 3.3 영지식증명 인터페이스

```text
ZK.Setup(relation, profile) -> (pk, vk)
ZK.Prove(pk, publicStatement, privateWitness) -> proof
ZK.Verify(vk, publicStatement, proof) -> accept | reject
```

여기서 `pk`는 proving key이며 threshold encryption public key와 구분한다.
정책별 relation과 public-input 순서를 고정하고 한 전이에 하나의 최종 proof를
사용한다. Completeness, soundness 및 공개 leakage를 제외한 witness 비공개성을
요구한다. 구체 setup 신뢰와 다중 proof 환경의 ZK 가정은 profile의 책임이다.

### 3.4 Threshold encryption 인터페이스

```text
TE.Setup(K, N, profile) -> (pk_TE, {sk_i})
TE.Enc(pk_TE, message, randomness; context) -> ciphertext
TE.Decrypt(pk_TE, ciphertext, ReleasedShares; context) -> message | fail

ReleasedShares = 외부에서 auditor에게 전달된, 동일 key에 속하는 유효한 {(i, sk_i)}
                 서로 다른 구성원 수 >= K
```

`1 <= K <= N`이다. Committee의 키 전달은 audit 외부의 선행 절차이며,
`TE.Decrypt`는 전달이 완료된 키 조각을 입력으로 받는다. Auditor는 동일 구성원
조각의 중복, 수량 부족, noncanonical encoding과 식별 가능한 key context 불일치를
거부한다. 전달된 secret 값 자체의 정당성은 제2.3절의 외부 입력 전제다.

복호화는 auditor 내부에서 수행한다. 구현은 받은 키 조각으로 partial decryption을
로컬 계산하여 결합하거나, 선택한 scheme에 따라 복호화 키를 복원할 수 있다.
이 로컬 중간 계산을 committee의 원격 응답으로 취급하지 않는다.
별도의 암호문별 응답 프로토콜이나 partial-decryption correctness proof는
필수 interface가 아니다.

비인가 공격자의 K개 미만 키 노출에 대한 비밀성과, 유효한 키 조각으로 올바른
암호문을 복호화하는 정확성을 요구한다. Canonical message type·길이를 검사하고
발견한 복호화 오류는 `fail`로 처리한다. 구체 key 생성·보관·교체와 수령 입력
형식은 profile에서 고정한다. Key 교체를 지원하면 과거 ciphertext의 key 식별과
복호화 가능성을 정의하고, 고정 key profile은 운영 중 임의 교체를 허용하지 않는다.

### 3.5 정수 회계와 반올림

회계 값은 음이 아닌 정수다. `q_mass`와 `a_rec`은 동일 질량 단위와 scale,
`e`는 고정 scale의 `kgCO2e` 절대량을 사용한다. 문서의 `Unit`과 이 수치
encoding의 관계는 profile에서 고정하며 floating point로 승인 여부를 정하지 않는다.

모든 입력·출력·증분·합계·곱·나머지는 정해진 범위에 있어야 한다.
유한체의 모듈러 등식으로 음수나 overflow가 회계식 통과로 바뀌지 않도록
정수 의미를 증명해야 한다. 범위는 지원 arity의 합계와 threshold 비교의
교차곱까지 포함한다.

```text
y = floor(x * q / Q)
  iff Q > 0, x * q = Q * y + r, 0 <= r < Q
```

위 등식과 범위를 정수로 검사한다. 분모 0인 배분은 허용하지 않는다.
Scale·threshold·반올림 방식은 정책의 상수이며 prover가 전이마다 선택하지 않는다.

## 4. 객체와 식별자

### 4.1 DocumentInfo

```text
DocumentInfo = (ProductName, LotID, Unit)
d = DocumentHash = HashDocumentInfo(DocumentInfo)
```

세 필드는 외부 DPP에서 profile이 지정한 경로와 규칙으로 추출한다.
`HashDocumentInfo`는 세 값 전체의 deterministic canonical encoding에
결합된다. 누락·잘못된 type·모호한 encoding은 거부한다.
추가 DPP 필드가 이 hash에 자동으로 포함되는 것은 아니다.

### 4.2 Note

```text
NoteOpening = (d, assetRole, q_mass, a_rec, e, owner, opening)
cm = H("zkDPP:Note:v2", d, assetRole, q_mass, a_rec, e, owner, opening)
owner = OwnerKey(ownerSecret)
nf = H("zkDPP:Nullifier:v2", cm, ownerSecret)
```

Private state는 `(assetRole, q_mass, a_rec, e)`다. 모든 Note는
`0 <= a_rec <= q_mass`를 만족한다. `WASTE`는 `a_rec = e = 0`이며 Exit로만
소비할 수 있다. 참조 정책의 Entry는
초기 탄소 입력을 허용하고, 생략하면 0으로 시작한다. 따라서 `e`는 Entry에서
입력한 초기 탄소를 기준으로 이후 전이의 증분과 배분을 반영한 값이다.
초기 값의 실제 의미와 진실성은 Entry Issuer가 책임지며, 기본값 0이 등록
이전의 실제 탄소 배출이 없었다는 증거는 아니다.

`cm`은 생성 때 공개된다. `nf`는 어느 Event로 소비하든 동일하다.
생성 시 미래 `nf`는 암호화해 기록하며 공개하거나 spent로 만들지 않는다.
소비 시 proof의 공개 입력으로 `nf`를 제출한다.

### 4.3 Voucher

```text
VoucherOpening = (
  d, assetRole, q_mass, a_rec, e,
  senderOwner, receiverOwner, recallDeadlineEpoch, opening
)
rv = H("zkDPP:Voucher:v2",
       d, assetRole, q_mass, a_rec, e,
       senderOwner, receiverOwner, recallDeadlineEpoch, opening)
s_res = H("zkDPP:VoucherResolutionSecret:v2", opening)
rvnf = H("zkDPP:VoucherNullifier:v2", rv, s_res)
```

Voucher는 미해결 Transfer를 나타낸다. Transfer는 `ELIGIBLE` Voucher만
만들며 local numeric bound는 Note와 같다. Sender와 receiver는 서로 다른
소유권 witness로 각각 Recall과 Proceed를 수행하지만 동일한 `rvnf`를
공개한다. 먼저 accepted된 한 경로만 소비를 완료할 수 있다.

Sender는 high-entropy opening을 생성하고 전체 `VoucherOpening`을 보관한다.
동일 payload를 제출 전 또는 제출과 함께 receiver에게 인증된 비밀 외부
channel로 전달해야 한다. Receiver는 Proceed, sender는 Recall witness로
사용한다. 이 전달의 실패 복구나 전달 사실에 대한 온체인 증명은 제공하지 않는다.

### 4.4 Claim과 DPPClaim

```text
h = H("zkDPP:Issue:v2", d, issuePolicyRef, claimNonce)
DPPClaim = (issuePolicyRef, h, claimNonce)
```

Issue는 Note를 소비하여 공개 terminal 객체 `ClaimRef(h)`를 만든다.
Claim에는 owner, private opening, membership tree, 미래 nullifier 또는
status가 없다. 이후 Event의 input으로 소비되지 않는다.

`DPPClaim`은 외부 DPP에 첨부하는 참조다. `claimNonce`는 공개 nonce이며
접근 권한 증표가 아니다. Issue proof에서 d와 nonce를 witness로 사용할 수
있지만, DPP에 게시한 nonce가 비밀이라는 보장을 할 수 없다.
후보 문서 값이 알려져 있을 때 공개 nonce가 문서 추측을 막아 주지도 않는다.

### 4.5 Typed ObjectRef

```text
objectKey(NoteRef(cm))    = H("zkDPP:ObjectRef:v2", NoteType, cm)
objectKey(VoucherRef(rv)) = H("zkDPP:ObjectRef:v2", VoucherType, rv)
objectKey(ClaimRef(h))    = H("zkDPP:ObjectRef:v2", ClaimType, h)
```

Type tag와 raw identifier를 항상 함께 다룬다. Raw 값이 같은 서로 다른
type의 객체를 동일 객체로 합치지 않는다. Status 조회용 Note `nf`와 Voucher
`rvnf`도 서로 다른 typed key다. `ObjectRef` 자체는 별도의 회계 객체가 아니다.

## 5. Policy, relation과 권한 등록

### 5.1 Event별 검증 경로와 단일 proof

Event별 규칙의 선택과 권한 검사는 다음 세 경로를 사용한다.

| Event | 규칙과 verifier 선택 | PolicyRef·scope 처리 |
|---|---|---|
| Entry, Transfer, Proceed, Recall, Merge, Split, Exit | 배포 때 고정한 Event별 relation과 verifier | 동적 policy 등록·scope grant를 사용하지 않는다. AuditRecord.policyRef는 0이다. |
| Process | 등록된 Process policy의 relation과 verifier | 실제 PolicyRef와 scope를 proof에 결합하고 enabled·grant를 검사한다. |
| Issue | 등록된 Issue policy의 relation과 verifier | 실제 issuePolicyRef를 proof와 Claim에 결합하고 enabled를 검사한다. Scope grant는 사용하지 않는다. |

일반 Event의 고정 verifier도 해당 Event의 회계·소유권·membership·감사 binding
전체를 검사한다. 각 일반 Event에 여러 정책을 등록하거나 사용 권한 체계를
추가할 필요는 없다. Entry의 issuer 권한과 모든 소비의 ownership·status 검사는
이 선택 방식과 별개로 적용된다.

모든 전이는 하나의 최종 proof를 사용한다.

```text
고정 경로:
  CircuitRel_kind = CoreRel_kind AND FixedAccountingRel_kind

등록 경로(Process, Issue):
  p = PolicyRef = H("zkDPP:PolicyRef:v2",
                    eventKind, inputArity, outputArity, policyIdentifier, version)
  CircuitRel_(kind,p) = CoreRel_kind AND PolicyRel_(kind,p)
```

등록 정책의 `policyIdentifier`는 profile이 지정한 namespace에서 유일하게
해석된다. 등록 경로의 p는 고정 경로의 표식 0과 구별되는 유효 식별자여야 한다.
`CoreRel`은 commitment/opening, membership, ownership, 소비 식별자,
distinctness, exact arity, output·감사 ciphertext binding, 수치 범위와
Event binding을 포함한다. 등록 경로에는 p binding을, Process에는 scope
binding을 추가한다. 현재 status와 spentness는 Router가
공개 소비 식별자로 집행한다.

`PolicyRel`은 회계식, threshold, scale, 반올림, output role vector와 추가
domain predicate를 고정한다. 두 relation은 하나의 circuit으로 구성된다.
등록 경로에서 arity가 다르면 별도 policy와 VK를 사용하고 relation이나 VK를
바꾸면 새 정책 identity로 등록한다. 고정 경로의 relation·VK·public-input 순서는
배포 profile에 고정하며 prover가 바꿀 수 없다. 어느 경로에서도 inactive slot으로
다른 arity를 위장하지 않는다.

### 5.2 Registry와 manifest

```text
PolicyRecord = {
  eventKind, inputArity, outputArity, vkHash, verifierRef, enabled
}
policyRecords[p] = PolicyRecord
policyGrants[policyScopeRef][p] = allowed   // Process 정책에만 적용
```

Policy Authority는 Process와 Issue의 immutable·versioned 공개 relation manifest를 배포한다.
Manifest에는 p, Event kind, exact input/output type·arity, 정책 상수,
public-input 순서, circuit source와 VK의 결합을 명시해야 한다.
Registry는 등록된 VK/verifier를 고정하며 prover가 VK를 선택하게 하지 않는다.
Manifest는 배포·검토 자료이고 별도의 mutable on-chain 객체가 아니다.

고정 경로는 배포 profile에 Event kind·exact shape·고정 회계식·public-input
순서와 circuit/VK·verifier의 대응을 공개한다. 해당 Event를 PolicyRecord로
등록할 필요는 없다. Entry의 input arity 0과 Exit의 output arity 0은 고정
Event relation에서 표현하므로 registry의 arity 허용 범위를 확대할 이유가 아니다.

Record는 `enabled = true`로 등록한다. Verifier metadata는 변경할 수 없고
enabled만 `true → false`로 변경할 수 있다. 다시 활성화하지 않는다.
Disable은 이후 제출에 적용하며 과거 accepted Event와 Claim을 소급 무효화하지 않는다.

### 5.3 Process scope authorization

Process의 `policyScopeRef`는 회사나 recipe를 직접 노출하지 않는 opaque identifier다.
Caller 또는 credential이 그 scope를 사용할 권한이 있고 해당 `(scope,p)`
grant가 허용되어야 한다. Proof와 Router가 같은 p·scope를 검사한다.
소유권 지식만으로 모든 scope의 정책 사용 권한을 얻지 않는다.
구체 credential과 grant 갱신 방식은 배포 profile에서 고정한다.

이 scope·grant 검사는 Process에 적용한다. Issue와 고정 경로에는 scope
입력·grant를 요구하지 않는다. Entry에는 trusted issuer 검사를 적용한다.

### 5.4 참조 회계 정책

제9장은 질량·재활용 credit·탄소에 대한 참조 정책을 정의한다.
고정 Event는 해당 고정 relation으로, Process와 Issue는 해당 등록 정책으로
공통 lifecycle 조건과 제9장의 relation을 함께 강제한다.
등록 경로에서는 다른 회계 정책도 별도 manifest와 circuit/VK로 등록할 수 있지만 공통
보안·lifecycle 검사를 생략할 수 없다. 실제로 강제하지 않는 참조 회계의
보존식이나 Claim 의미를 제공한다고 주장할 수 없다.

## 6. 원장 상태와 status 집행

### 6.1 논리 상태

```text
NoteTree    = {nodes, leafCount, currentRoot, acceptedRoots}
VoucherTree = {nodes, leafCount, currentRoot, acceptedRoots}

noteSpentIn[nf]               // 0 또는 소비한 aid
voucherSpentIn[rvnf]          // 0 또는 해결한 aid
noteStatusByNf[nf]            // 기본값 Active
voucherStatusByNf[rvnf]       // 기본값 Active

claimRegistered[h]
producerOf[objectKey]
auditRecords[aid]
nextAuditRecordId
fixedVerifiers[eventKind]      // 고정 경로의 논리적 선택표; 개별 저장 필드로 구현 가능
policyRecords[p]
policyGrants[scope][p]
roleConfiguration
deploymentProfile
```

Note와 Voucher는 분리된 depth-32 append-only Merkle tree에 등록한다.
Node·leaf count·현재 root와 accepted historical roots를 온체인에 보관하며,
공개 원장 자료로 membership path를 재구성할 수 있어야 한다.
외부 indexer는 선택적인 조회 최적화다. Accepted 과거 root의 membership도
허용하지만 소비 승인에는 현재 `spentIn`과 status를 검사한다.
Tree 용량을 넘는 append는 전체 전이를 거부한다.

`aid`는 1부터 사용하므로 `spentIn = 0`은 consumer가 없다는 뜻이다.
소비 시 0을 실제 aid로 한 번 바꾸고 다시 덮어쓰지 않는다.
별도의 authoritative boolean spent set을 중복 관리하지 않는다.
Record, producer index와 accepted history도 생성 후 변경하지 않는다.

### 6.2 Status 전이

```text
Active -> Frozen
Frozen -> Active
Frozen -> Revoked
```

Revoked는 terminal이다. `Active → Revoked`는 허용하지 않는다.
Status Authority만 다음 논리 interface를 호출할 수 있다.

```text
updateNoteStatus(nf, newStatus)
updateVoucherStatus(rvnf, newStatus)
```

Router는 권한, identifier의 canonical encoding, 현재 `spentIn = 0`,
허용된 status edge를 검사한다. 모든 Note/Voucher 소비는 현재 status가
Active인 경우에만 허용된다. Frozen 또는 Revoked 상태에서는 Exit를 포함해
어떤 소비 Event도 통과하지 않는다.

Mapping 기본값 Active는 그 key의 객체가 실제 존재한다는 증거가 아니다.
SA가 올바른 복호화 식별자를 제출한다고 신뢰하며, status transaction이
그 식별자와 private 객체의 결합을 다시 증명하지는 않는다.
별도 StatusTree·status root·private status path는 사용하지 않는다.
Claim이나 AuditRecord 자체에는 status를 두지 않는다.

## 7. AuditRecord와 proof binding

### 7.1 Record 구조

```text
AuditRecord = {
  auditRecordId,
  eventKind,
  policyRef,
  outputRefs,                  // ordered typed public outputs
  parentCount,
  encryptedParents,            // 실제 ordered parent ObjectRef 목록
  encryptedOutputNfs           // ordered spendable output의 미래 소비 식별자
}
```

모든 accepted Event는 하나의 immutable record를 남긴다. `parentCount`와
`outputRefs` 길이·type·순서는 제5.1절에서 선택한 relation의 exact shape와 일치한다.
실제 parent reference는 공개 입력 소비 식별자와 구분되는 private witness다.
고정 경로의 record에는 policyRef를 0으로 기록하며 Process와 Issue에는 실제 p를
기록한다. 고정 경로의 p=0은 정책 선택 입력이 아니며 Event와 배포 verifier로
규칙을 식별한다.

`encryptedParents`는 실제 ordered parent 목록을 암호화한다.
Parent 0개일 때는 empty list에 결합하며, 그 canonical 표현은 profile에서
정한다. `encryptedOutputNfs`는 `outputRefs`에서 Note와 Voucher만 순서대로
고른 목록과 일대일로 대응한다. Claim에 대응하는 미래 소비 ciphertext는 없다.

```text
Transfer.outputRefs         = [VoucherRef(rv_T), NoteRef(cm_C)]
Transfer.encryptedOutputNfs = [TE.Enc(rvnf_T), TE.Enc(nf_C)]
```

이 표기에서 생략된 committee key, randomness와 context는 제3.4절을 따른다.

### 7.2 암호문과 전이의 결합

Event circuit은 다음 전체를 증명해야 한다.

1. Parent plaintext가 membership·ownership·회계 검사에 실제 사용한 입력이다.
2. 각 output reference가 해당 private opening의 commitment 또는 Claim handle이다.
3. Output Note의 미래 nf가 cm와 해당 output owner secret에서 계산된다.
4. Output Voucher의 미래 rvnf가 rv와 해당 Voucher opening에서 계산된다.
5. 암호문들이 위 plaintext를 지정 committee public key로 올바르게 암호화한다.
6. 배열 수·type·순서와 ciphertext 위치가 정확히 대응한다.
7. Event kind와 관련 공개 metadata가 같은 전이에 결합된다. 등록 경로의 p와
   Process의 scope도 해당 proof statement에 결합된다. 고정 경로의 kind는
   Router의 호출 경로와 고정 verifier 선택으로도 결합할 수 있다.

다른 객체의 미래 식별자나 임의 parent를 암호화한 proof는 거부되어야 한다.
Ciphertext와 context를 바꾸고 원래 proof를 재사용할 수 없어야 한다.
공개 statement와 암호 context는 같은 accepted transaction 자료에서 재구성한다.
`auditRecordId`는 승인 시 배정되므로 prover가 미리 알아야 하는 암호화
context 필드로 요구하지 않는다. 구체 layout은 profile에서 고정한다.

### 7.3 감사 시 자료 검증

감사 client는 선택한 ledger와 prefix에 record가 존재하고 producer index,
output 위치·type, Event shape와 공개 transaction 자료가 일치하는지 확인한다.
복호화에는 외부에서 수령한 `ReleasedShares`와 해당 key·ciphertext·context를
사용한다. Canonical decoding 또는 일관성 검증 실패는 `fail`이다.

Record 자료가 없거나 원장 조회가 실패하면 전체 성공 결과를 추측해 만들지 않는다.
부분 진단 자료를 보관할 수는 있지만 완성된 graph나 전체 frontier로 취급하지 않는다.
의사코드의 `recordContext`는 제7.2절에 따라 해당 accepted transaction에서
재구성한 암호 context다.
Committee에서 auditor로 키 조각을 전달하는 절차는 이미 충족된 audit 입력
전제다. Record마다 committee에 다시 연락하지 않아도 된다.

## 8. Setup, 증명과 공통 전이 실행

### 8.1 Setup

배포자는 profile, chain·ledger 식별, role 권한과 committee 구성을 고정한다.
Threshold setup을 실행하고 각 committee 구성원에게 자기 share와 key 식별
정보를 설치한다. 실제 감사 전에 필요한 조각은 외부 절차로 auditor에게 전달한다.
Note/Voucher tree를 비우고 초기 root를 accepted root로 등록한다.
Spent·producer·Claim·record mapping은 비어 있으며 첫 aid는 1이다.
Status mapping의 미등록 값은 Active다.

고정 Event의 circuit/VK와 verifier를 배포 구성에 설치한다. Policy Authority는
Process·Issue의 circuit/VK를 검토하여 PolicyRecord를 등록하고 Process의
허용 scope에 grant를 설정한다. Prover와 verifier는 동일한 배포 profile 또는
등록 정책 manifest, proving/verifying key와 public-input encoding을 사용한다.

### 8.2 Statement와 witness

Event별로 필요한 공개 입력은 다음 범주에서 exact shape로 정한다.

```text
PublicStatement = (
  eventKind,
  policyRef,                  // Process·Issue
  policyScopeRef,             // Process
  input membership root(s),
  input Note nf[] 또는 Voucher rvnf,
  ordered outputRefs,
  parentCount, encryptedParents, encryptedOutputNfs,
  eventSpecificMetadata
)
```

Private witness는 실제 parent reference/opening, membership path, 권한 있는
owner secret, output opening, 회계 값과 증분·배분, encryption randomness,
필요한 credential witness, Issue의 문서 hash·nonce 등이다.
Public/private 필드의 정확한 순서를 고정 경로의 배포 profile 또는 등록 정책
manifest에 고정한다. 위 tuple은 논리적 구성이다. 고정 경로는 p·scope 입력을
생략하고 kind를 호출 경로·고정 verifier로 결합할 수 있다.

Prover는 opening과 path를 준비하고 input 소비 식별자, output reference와
미래 소비 식별자를 계산한다. 실제 parent와 output 미래 식별자를 암호화한 뒤
제5.1절의 해당 전체 relation에 대한 proof 하나를 생성하여 statement와 함께 제출한다.
현재 status와 deadline은 proof 생성 시점의 로컬 상태로 확정하지 않는다.

### 8.3 공통 circuit 검사

모든 Event는 적용 가능한 다음 조건을 강제한다.

- Parent opening과 commitment의 일치, 올바른 type tree의 membership.
- Input owner 또는 Voucher의 해당 sender/receiver secret 지식과 소비 권한.
- 실제 입력에서 정확히 계산한 공개 nf/rvnf.
- 동일 Event 내 input ObjectRef, typed input spendId, output objectKey의
  각 집합에 대한 pairwise distinctness. 같은 입력을 두 slot에 넣을 수 없다.
- Output opening·소유권·reference 일치와 제7.2절의 암호문 binding 전체.
- Event kind와 exact input/output type·arity의 결합. 등록 경로는 p를,
  Process는 scope도 결합한다. 고정 kind는 Router의 verifier 선택과 함께 결합한다.
- Event별 relation, private 회계 값의 범위, 모든 연산의 no-wraparound.

입력이 없으면 해당 membership·소비 검사는 빈 집합에 적용한다.
출력이 없으면 output 관련 배열도 정확히 비어 있어야 한다.
선택한 고정 relation 또는 registry metadata와 다른 길이의 배열이나 비활성
padding slot은 허용하지 않는다.

### 8.4 Router 검사와 원자적 갱신

Router는 현재 원장에서 다음을 확인한다.

1. 고정 Event는 해당 배포 verifier와 exact shape를 사용한다. Process·Issue는
   PolicyRecord가 존재하고 enabled이며 kind·arity가 제출과 같아야 한다.
2. Process에서는 제출자 또는 credential의 scope 권한과 `(scope,p)` grant가 유효하다.
3. Entry issuer 권한 및 해당 Event의 runtime deadline 조건을 만족한다.
4. 사용한 membership root가 accepted root다.
5. 모든 typed input spendId의 현재 `spentIn = 0`이고 status가 Active다.
6. Input spendId와 output key에 중복이 없고 모든 output key가 fresh하다.
   Claim은 등록 이력도 없어야 한다.
7. 해당 고정 verifier 또는 등록된 verifier가 정확한 전체 public statement의 proof를 승인한다.

모두 통과하면 하나의 transaction에서 다음을 수행한다.

```text
aid <- nextAuditRecordId
각 Note input nf:       noteSpentIn[nf] <- aid
각 Voucher input rvnf:  voucherSpentIn[rvnf] <- aid
각 Note output:         NoteTree에 append하고 새 root를 accepted로 등록
각 Voucher output:      VoucherTree에 append하고 새 root를 accepted로 등록
각 Claim output h:      claimRegistered[h] <- true
auditRecords[aid] <- immutable AuditRecord
각 output r:            producerOf[objectKey(r)] <- aid
nextAuditRecordId <- aid + 1
```

하나라도 실패하면 전체 갱신을 revert한다. Spent index만 기록되거나,
output은 만들어졌는데 record·producer index가 누락되는 중간 상태는 없다.
Historical root에 포함되어도 이미 소비되었거나 현재 차단된 입력은 거부한다.

## 9. 아홉 Event의 참조 relation

아래 식의 `q`, `a`, `e`, `d`는 각각 `q_mass`, `a_rec`, 누적 탄소,
`DocumentHash`다. 각 절은 제8장의 공통 검사에 추가된다.
모든 입력·출력의 local bound와 소유권 규칙은 계속 적용된다.

| Event | Ordered input → output | Parent 수 | 미래 소비 ciphertext 수 |
|---|---|---:|---:|
| Entry | `[] → [Note]` | 0 | 1 |
| Transfer | `[Note] → [Voucher, Change Note]` | 1 | 2 |
| Proceed | `[Voucher] → [receiver Note]` | 1 | 1 |
| Recall | `[Voucher] → [sender Note]` | 1 | 1 |
| Merge | `[Note_1, Note_2] → [Note]` | 2 | 1 |
| Split | `[Note] → [Note_1, Note_2]` | 1 | 2 |
| Process | `m개의 Note → n개의 Note` | m | n |
| Exit | `[Note] → []` | 1 | 0 |
| Issue | `[Note] → [Claim]` | 1 | 0 |

### 9.1 Entry

```text
[] -> Note(ELIGIBLE, q, a, e = e_initial)
q > 0
0 <= a <= q
e_initial >= 0
e_initial은 profile의 canonical numeric range 안에 있어야 함
```

Entry 입력에서 초기 탄소를 생략하면 client는 `e_initial = 0`으로 정규화한다.
명시한 유효한 초기 탄소 값은 그대로 사용한다. 기본값은 입력 처리 규칙이며
회로가 `e_initial = 0`을 강제한다는 뜻이 아니다. 음수·범위 초과 등 잘못된
명시 입력은 거부하고 기본값 0으로 대체하지 않는다.

정규화한 e_initial은 Note의 e 필드로 commitment와 proof witness에 결합된다.
별도의 온체인 기본값 flag나 초기 탄소용 객체를 만들지 않는다.

Trusted issuer의 승인이 필요하며 생성자와 최초 owner는 같은 주체다.
생성자는 해당 owner secret으로 output nf를 계산하고 암호화한다.
Parent payload는 empty list다. 원장은 초기 값의 물리적 진실성과 동일 물량의
중복 등록 여부를 ZK로 알아내지 않으며 이 판단은 Entry Issuer가 책임진다.

### 9.2 Transfer

```text
Note_in(ELIGIBLE) -> Voucher_T(ELIGIBLE) + Note_C(ELIGIBLE)
q_in = q_T + q_C
q_T > 0, q_C >= 0

a_C = floor(a_in * q_C / q_in)
a_T = a_in - a_C
e_C = floor(e_in * q_C / q_in)
e_T = e_in - e_C + delta_e_transport
delta_e_transport >= 0
```

두 output은 input d를 보존한다. Voucher의 sender와 Change Note owner는
input owner다. Receiver는 Voucher에 고정한다. Transport 탄소 증분은
Voucher 부분에 더하며 실제 운송량·배출량의 진실성은 외부 입력에 의존한다.
전량 Transfer도 `q_C = a_C = e_C = 0`인 Change Note를 생성한다.

Public absolute deadline `D = recallDeadlineEpoch`가 Voucher opening의 D와
같음을 증명한다. Router는 실행 시점에 `D > CurrentEpoch()`와 profile의
허용 deadline 범위를 검사한다. `CurrentEpoch()`는 profile이 고정한
block-height 기반 discrete epoch이며 로컬 시계나 proof 생성 시각이 아니다.

### 9.3 Proceed

```text
Voucher -> receiver Note
(d, assetRole, q, a, e)_out = (d, assetRole, q, a, e)_voucher
```

Receiver secret으로 Voucher의 receiver 소유권을 증명한다. Output도 같은
receiver 소유다. Opening에서 공통 rvnf를 계산하고 Router가 현재 미해결·Active를
확인한다. Proceed에는 deadline 조건이 없다.

### 9.4 Recall

```text
Voucher -> sender Note
(d, assetRole, q, a, e)_out = (d, assetRole, q, a, e)_voucher
CurrentEpoch() <= D
```

Sender secret으로 Voucher의 sender 소유권을 증명하며 output도 sender 소유다.
Public D와 private Voucher의 D가 같아야 한다. Router가 transaction 실행 시
위 시간 조건을 검사한다. 기한과 같은 epoch의 Recall은 허용하며 기한 이후에는
미해결 Voucher를 Proceed로만 해결할 수 있다.

Proceed와 Recall은 같은 rvnf를 사용한다. 먼저 승인된 전이가 소비를 기록하며
다른 경로는 거부된다. 기한 전에 만든 Recall proof도 기한 이후 제출하면 거부된다.
별도의 Voucher deadline mapping을 두지 않고 proof에 결합된 공개 D를 검사한다.

### 9.5 Merge

서로 다른 두 `ELIGIBLE` Note는 같은 owner와 d를 가져야 한다.
Output은 그 owner·d·ELIGIBLE role을 유지한다.

```text
q_out = q_1 + q_2
a_out = a_1 + a_2
e_out = e_1 + e_2
```

다른 d를 가진 물량의 결합은 Process로 표현한다. 같은 Note를 두 입력으로
반복하여 합계를 부풀리는 것은 공통 distinctness 검사에서 거부한다.

### 9.6 Split

하나의 `ELIGIBLE` Note를 두 Note로 나누며 두 output의 owner·d·role을 보존한다.

```text
q_in > 0
q_in = q_1 + q_2
a_in = a_1 + a_2
e_in = e_1 + e_2

a_2 = floor(a_in * q_2 / q_in)
e_2 = floor(e_in * q_2 / q_in)
a_1 = a_in - a_2
e_1 = e_in - e_2
```

질량 0 output을 허용하며 그 output의 a와 e도 0이어야 한다.
Output 2의 floor와 output 1의 residual 순서를 바꾸지 않는다.

### 9.7 Process

각 정책은 정확한 `(m,n)`과 ordered output role vector를 고정한다.
`m >= 1`, `n >= 1`이고 지원할 유한 arity 집합은 profile에 명시한다.
모든 input은 서로 다른 ELIGIBLE Note이며 input과 output은 모두 processor 소유다.
Policy는 하나 이상의 output을 ELIGIBLE로 고정해야 한다.
값을 본 뒤 prover가 output role을 바꿀 수 없으며 전체 폐기는 Exit로 표현한다.

`I_E`와 `I_W`를 각각 ELIGIBLE/WASTE output index 집합이라 하자.

```text
Q_in = sum_i(q_input[i])
Q_E = sum_(j in I_E)(q_output[j])
Q_W = sum_(j in I_W)(q_output[j])
Q_in = Q_E + Q_W + q_loss
q_loss >= 0
Q_E > 0

sum_i(a_input[i]) = sum_(j in I_E)(a_output[j])
0 <= a_output[j] <= q_output[j]             for j in I_E
a_output[j] = 0                            for j in I_W

E_total = sum_i(e_input[i]) + delta_e_process
delta_e_process >= 0
j0 = ordered output 중 첫 ELIGIBLE index
e_output[j] = floor(E_total * q_output[j] / Q_E)  for j in I_E, j != j0
e_output[j0] = E_total - sum_(j in I_E, j != j0)(e_output[j])
e_output[j] = 0                                 for j in I_W
```

Credit은 local bound 내에서 Eligible output 사이에 비공개 배분할 수 있다.
탄소는 Eligible 질량에 비례해 배분하고 첫 Eligible output에 반올림 잔여량을
준다. `Q_E > 0`은 이 나눗셈의 정의역 조건이다. WASTE는 credit과 탄소를
보유하지 않는다. 질량 손실과 공정 탄소 증분의 실제 값은 외부 입력에 의존하며
정책이 추가 검사를 둘 수 있다.

Output d는 input과 달라질 수 있다. 이 참조 relation은 물질 간 물리적
호환성을 판정하지 않는다. 출력 하나의 질량이 0이라는 사실만으로 다른 slot의
역할이나 residual 수신 순서를 변경하지 않는다.

### 9.8 Exit

```text
Note(ELIGIBLE 또는 WASTE) -> []
```

Owner가 Note를 한 번 소비하여 successor 없이 종료한다. WASTE의 유일한
소비 경로다. 실제 Note parent는 암호화해 남기며 outputRefs와
encryptedOutputNfs는 모두 빈 목록이다. Exit는 Claim을 생성하지 않는다.

### 9.9 Issue

```text
Note(ELIGIBLE) -> ClaimRef(h)
q > 0
0 <= a <= q
a / q >= tau_rec
e / q <= tau_carbon
```

Owner가 final Note를 한 번 소비한다. Threshold는 정책이 고정한 scale을
반영한 유리 상수다. `tau_rec = u_R/v_R`, `tau_carbon = u_C/v_C`로
표현할 때 양의 분모와 호환 단위를 사용하여 실제 circuit은 다음을 검사한다.

```text
v_R * a >= u_R * q
v_C * e <= u_C * q
```

두 비교는 정수 범위 안의 교차곱이다. 유한체 나눗셈으로 비율을 판정하지 않는다.
Private d는 소비 Note의 DocumentHash와 같아야 하며,
`h = H("zkDPP:Issue:v2", d, p, claimNonce)`를 증명한다.
Claim 등록·producer index·actual-parent record·Note 소비는 원자적이다.
Issue output에는 미래 소비 ciphertext가 없다.

같은 Note로 다른 nonce나 정책의 Claim을 다시 발행하거나 Issue 후 Exit할 수 없다.
반대로 Exit한 Note에서 Claim을 발행할 수도 없다.

## 10. 공개 DPP 검증

`VerifyDPPClaim`은 알려진 배포 context, 외부 DPP와 그 안의 DPPClaim을 입력받는다.
검증에 사용한 원장의 확정 prefix를 `H_v`라 한다.

```text
VerifyDPPClaim(ledgerContext, H_v, DPP, (p, h, claimNonce)):
  require ledgerContext와 H_v가 유효함
  info <- profile에 따라 DPP의 ProductName, LotID, Unit 추출
  require info와 claim tuple이 canonical함
  d <- HashDocumentInfo(info)
  require h == H("zkDPP:Issue:v2", d, p, claimNonce)
  require claimRegistered_H_v[h] == true
  aid <- producerOf_H_v[objectKey(ClaimRef(h))]
  require aid가 존재함
  record <- auditRecords_H_v[aid]
  require record.eventKind == Issue, record.policyRef == p
  require record.outputRefs == [ClaimRef(h)]
  return valid
```

실패한 조건이 하나라도 있으면 Claim 검증을 거부한다.
Verifier는 해당 정책의 공개 의미를 확인하여 어떤 predicate가 승인됐는지 해석한다.

유효한 Claim은 등록 당시 해당 Issue relation을 통과한 Note가 지정 문서 필드에
결합되었다는 의미다. Claim status를 조회하지 않으며 정책의 사후 disable도
과거 Claim을 무효화하지 않는다. 추가 DPP 필드, 물리 제품, 이후의 안전성이나
전체 규제 준수를 증명하는 결과로 확대하지 않는다.

## 11. Backward tracing

### 11.1 기준 graph와 시작점

Accepted graph는 객체와 Event를 vertex로 갖는다. 실제 입력 객체에서
소비 Event로, Event에서 각 output 객체로 edge가 있다.
Backward tracing은 시작 객체의 producer에서 실제 parent를 복호화하며
선행 Event를 따라 Entry까지 올라간다. 조회 중 서로 다른 원장 시점을 섞지
않도록 하나의 확정 prefix `H_t`에서 수행한다.

ObjectRef 시작은 그 객체와 producer의 존재·일치를 검증한다.
Record ID 시작은 해당 Event 자체에서 선행 경로를 추적하므로 output 없는
Exit도 시작점이 될 수 있다. 한 output의 producer를 읽었다고 그 Event의
다른 output까지 시작 객체의 선행 경로로 추가하지 않는다.

### 11.2 알고리즘

```text
TraceBackward(H_t, start, ReleasedShares):
  require 유효한 ledger prefix H_t
  require 외부 키 전달 전제가 충족되고 수령 입력의 수량·ID·encoding이 유효함
  if start is a valid ObjectRef at H_t:
    jobs <- [(producerOf_H_t[start], [start])]
  else if start is an existing auditRecordId at H_t:
    jobs <- [(start, auditRecords_H_t[start].outputRefs)]
  else:
    return fail

  G <- empty graph
  entryRecords <- empty set
  expandedRecords <- empty set

  while jobs is not empty:
    (aid, reachedOutputs) <- pop(jobs)
    record <- H_t에서 검증한 auditRecords[aid]
    require reachedOutputs 각각이 이 record의 output과 producer index에 일치함
    G에 aid, reachedOutputs, 각 (aid -> reachedOutput) edge 추가
    if aid in expandedRecords:
      continue

    parents <- TE.Decrypt(pk_TE, encryptedParents, ReleasedShares; recordContext)
    require parents의 수, type, 순서, distinctness가 record shape와 일치함
    expandedRecords에 aid 추가
    if parents is empty:
      require record.eventKind == Entry
      entryRecords에 aid 추가
    else:
      for each parent in parents:
        parentAid <- 검증된 producerOf_H_t[parent]
        require parentAid < aid
        G에 parent와 (parent -> aid) edge 추가
        jobs에 (parentAid, [parent]) 추가

  return (G, entryRecords)
```

위와 이하 의사코드의 `producerOf[r]`는 `producerOf[objectKey(r)]`의 축약이다.
Empty parent의 canonical 표현은 복호화가 불필요할 수 있지만 empty-list
binding 검증을 생략하는 의미는 아니다.

### 11.3 결과와 실패

한 producer가 여러 도달 객체를 만든 경우 해당 ciphertext의 복호화는
cache할 수 있다. 이미 확장한 record라도 새로 도달한 output edge는 보존한다.
모든 실제 parent를 추적하며 record 누락·수령 입력 오류·복호화 오류는 `fail`이다.
이 조회는 status를 변경하지 않는다. 문제 시작점을 결정한 뒤 차단하려면
제13장의 승인된 절차를 실행한다.

## 12. Snapshot 기반 Forward tracing

### 12.1 Consumer와 frontier

Note/Voucher r의 producer에서 r에 대응하는 미래 소비 ciphertext를 찾고
수령한 키 조각으로 복호화한다. 그 식별자로 `H_t`의 typed spentIn을 조회한다.
값이 0이면 snapshot의 frontier이고 aid이면 그 Event가 consumer다.
Claim에는 consumer 조회를 하지 않으며 공개 terminal로 기록한다.

ObjectRef 시작은 `H_t`에서 유효한 객체여야 한다. Record ID 시작은 그
record의 전체 output을 시작 객체로 사용한다. Output 없는 record를 시작으로
지정하면 시작 객체 집합이 비어 있는 결과다. 해당 record 자체의 선행 경로를
조사하려면 Backward tracing을 사용한다.

### 12.2 알고리즘

```text
TraceForward(H_t, start, ReleasedShares):
  require 유효한 ledger prefix H_t
  require 외부 키 전달 전제가 충족되고 수령 입력의 수량·ID·encoding이 유효함
  if start is a valid ObjectRef at H_t:
    queue <- [start]
  else if start is an existing auditRecordId at H_t:
    queue <- auditRecords_H_t[start].outputRefs
  else:
    return fail

  G <- empty graph
  frontierNotes, frontierVouchers, terminalClaims, terminatedExits <- empty sets
  visitedObjects, expandedRecords <- empty sets

  while queue is not empty:
    r <- pop(queue)
    if r in visitedObjects:
      continue
    producerRecord <- producerOf_H_t[r]로 조회·검증한 record
    require r의 존재, type, output 위치와 producer index가 일치함
    visitedObjects와 G에 r 추가

    if r is ClaimRef(h):
      require claimRegistered_H_t[h]와 Issue producer가 일치함
      terminalClaims에 r 추가
      continue

    ct <- producerRecord에서 r에 대응하는 encryptedOutputNfs 항목
    spendId <- TE.Decrypt(pk_TE, ct, ReleasedShares; recordContext)
    require spendId가 canonical하고 같은 typed 객체·식별자 대응에 모순이 없음
    consumerAid <- r의 type에 맞는 spentIn_H_t[spendId]
    if consumerAid == 0:
      r이 Note이면 frontierNotes에 (r, spendId) 추가
      r이 Voucher이면 frontierVouchers에 (r, spendId) 추가
      continue

    consumer <- H_t에서 검증한 auditRecords[consumerAid]
    require producerRecord.auditRecordId < consumerAid
    require consumer의 accepted 공개 입력에 해당 typed spendId가 있음
    G에 consumerAid와 (r -> consumerAid) edge 추가
    if consumerAid not in expandedRecords:
      expandedRecords에 consumerAid 추가
      if consumer.outputRefs is empty:
        require consumer.eventKind == Exit
        terminatedExits에 consumerAid 추가
      else:
        for each output in consumer.outputRefs:
          G에 output과 (consumerAid -> output) edge 추가
          queue에 output 추가

  return (G, frontierNotes, frontierVouchers, terminalClaims, terminatedExits)
```

Private input reference와 공개 spendId의 일치는 accepted proof와 제7장의
binding으로 보장된다. Forward 단계에서 consumer의 모든 다른 parent까지
추가로 복호화해야 하는 것은 아니다. 필요한 자료 조회·복호화·검증이 실패하면
알고리즘은 `fail`하며 추측한 frontier를 반환하지 않는다.

### 12.3 전체 downstream의 의미

`G`는 시작 객체에서 소비 Event와 그 모든 output을 따라 도달하는 graph다.
Merge가 시작 객체와 다른 input을 함께 소비했더라도 그 Merge의 모든 output을
추적한다. Split·Process의 모든 output도 포함하며 WASTE나 질량 0이라는
이유로 경로를 생략하지 않는다. Merge의 다른 input을 upstream으로 역추적해
downstream 집합에 넣지는 않는다.

시작 객체의 미래 식별자를 알아내기 위해 producer record를 읽는 일 자체는
그 producer의 sibling output을 downstream에 추가하는 근거가 아니다.
반면 도달한 consumer의 output은 모두 포함한다. 동일 consumer에 여러 input
경로로 도달하면 각 input edge를 보존하면서 output 확장은 한 번만 수행한다.

- `frontierNotes`: H_t에서 미소비인 도달 Note와 미래 nf.
- `frontierVouchers`: H_t에서 미해결인 도달 Voucher와 미래 rvnf.
- `terminalClaims`: 도달한 공개 ClaimRef.
- `terminatedExits`: 소비 이후 successor 없이 종료한 도달 Exit.

Frontier는 status와 별개다. 미소비 Frozen/Revoked 객체도 frontier에 포함한다.
동일 typed ObjectRef의 여러 도달은 하나로 정규화하고 상충하는 식별자 대응은
오류로 처리한다. Snapshot 뒤의 소비는 같은 trace에 섞지 않는다.

### 12.4 Exact 또는 fail-closed 요구

Output 미래 식별자의 정확한 암호화, 원자적인 spentIn 기록, immutable한
producer·record, 인증된 snapshot 읽기와 외부에서 받은 K개 이상의 유효한 키 조각이 있으면
실제 accepted downstream node·edge와 frontier를 정확히 복원해야 한다.
Record·ciphertext 누락, 키 조각 수량 부족 또는 발견된 검증·복호화 실패를
빈 경로나 미소비로 해석하지 않는다. 잘못된 secret 값의 악의적 전달은
제2.3절의 입력 전제 밖이며 이 조건부 exactness 주장에 포함하지 않는다.
Accepted 순서에서 parent producer는 consumer보다 앞서므로 graph는 비순환이다.

## 13. 감사와 전체 frontier 동결: AuditAndFreeze

### 13.1 목적과 실행 경계

`AuditAndFreeze`는 승인된 문제 시작점에서 downstream을 추적하고, 발견한
미소비 Note·미해결 Voucher 전체의 추가 프로토콜 사용을 차단하는 상위
감사 절차다. 단순히 추적 결과를 반환하는 것만으로 완료되지 않는다.

```text
문제 시작점에 대한 Status Authority의 차단 승인
  -> 확정 snapshot H_t에서 TraceForward
  -> 전체 미소비 frontier를 동결 대상으로 확정
  -> 대상별 current state 확인 및 필요한 Freeze transaction 제출
  -> 공통 확정 시점 H_r에서 전체 대상의 차단 상태 확인
  -> 대상별 결과와 전체 실행의 완료/미완료 보고
```

문제의 실세계 원인 판정 또는 backward 분석을 통해 시작점을 정하는 일은
선행 판단이다. 이 절차는 externally selected start를 입력으로 받으며,
문제를 자동 판정하거나 모든 감사에서 무조건 동결하지 않는다.
Status Authority가 차단 목적의 실행을 승인한 뒤에는 전체 frontier 규칙을
적용한다. 조회만 하는 `TraceForward`도 별도로 사용할 수 있다.

`TraceForward`는 상태를 변경하지 않는 조회 알고리즘으로 유지한다.
Freeze는 그 이후의 별도 온체인 transaction이며, off-chain 감사 client가
둘을 하나의 업무 절차로 연결한다. 이를 하나의 원자적 온체인 transaction,
새 ZK relation 또는 새 공개 AuditRecord 종류로 정의하지 않는다.

### 13.2 입력과 동결 대상의 결정

논리적 입력은 다음과 같다. 승인 기록의 구체 encoding이나 별도 온체인
등록을 요구하는 것은 아니다.

```text
AuditAndFreezeRequest = {
  ledgerContext,          // chain 및 ledger 식별
  start,                  // 유효한 ObjectRef 또는 auditRecordId
  traceSnapshot: H_t,     // 해당 ledger의 확정 immutable prefix
  releasedShares,         // 외부 절차에서 auditor에게 전달된 키 조각
  authorization: Status Authority의 이 범위에 대한 차단 승인
}
```

감사는 제2.3절의 외부 키 전달 전제와 제3.4절의 복호화 interface를 사용한다.
복호화나 record 검증이 실패해 전체 추적 결과를 얻지 못했다면
`TRACE_FAILED`로 종료하고 이 실행에서는 Freeze transaction을 제출하지 않는다.
부분 추적 결과를 전체 대상으로 간주해 상태 변경을 시작하지 않는다.

정상 추적 결과에서 다음 집합을 결정한다.

```text
T = frontierNotes union frontierVouchers
Target = (typed ObjectRef, spendId)
```

다음 규칙은 `AuditAndFreeze`의 필수 조건이다.

1. `T`는 snapshot `H_t`의 전체 미소비 frontier와 정확히 같아야 한다.
   사용자가 일부 target을 제외하거나 결과 목록을 잘라 넣는 선택형 실행은
   이 절차의 정의가 아니다.
2. Merge·Split·Process 분기에는 제12.3절의 모든 output 탐색 규칙을 그대로
   적용한다. 다른 input과 합쳐졌다는 이유로 이후 output을 제외하지 않는다.
3. Note와 Voucher의 타입을 보존한다. 동일한 숫자의 `nf`와 `rvnf`라도
   서로 다른 status mapping의 대상이며 하나로 합치지 않는다.
4. 동일한 typed ObjectRef의 중복 도달은 하나의 target으로 정규화한다.
   동일 객체의 서로 다른 spendId 또는 서로 다른 객체의 동일 typed spendId
   같은 일관성 위반을 단순 중복으로 숨기지 않는다.
5. 현재 이미 Frozen 또는 Revoked인 target도 `T`에서 삭제하지 않는다.
   아래 절차에서 이미 차단된 대상으로 기록한다. WASTE 또는 질량을 이유로
   snapshot frontier를 추가로 필터링하지 않는다.
6. `terminalClaims`와 `terminatedExits`는 감사 결과에 남지만 동결 대상에는
   포함하지 않는다. Claim 상태와 이미 완료된 소비의 rollback을 추가하지 않는다.

`T`는 한 실행 동안 고정된다. 실행 중 소비됐거나 동결에 실패한 target을
목록에서 지워서 전체 완료처럼 보고하지 않는다. `T`가 비어 있으면
`NO_LIVE_TARGETS`로 종료한다. 이는 snapshot에서 동결할 소비 가능 객체가
없다는 뜻이며, 문제가 없거나 최종 제품이 안전하다는 판정이 아니다.

### 13.3 Status Authority interface와 집행

제6.2절의 status interface를 사용한다.

```text
updateNoteStatus(nf, Frozen)
updateVoucherStatus(rvnf, Frozen)
```

Contract는 caller가 Status Authority인지, 해당 spend identifier가 현재
미소비인지, current status에서 요청한 상태 전이가 허용되는지 확인한다.
복호화한 식별자를 올바른 타입의 mapping에 제출하는 것은 감사 client의
책임이다. 이후 소비 Event는 current `spentIn`과 status를 제8.4절에 따라 검사한다.

Status Authority가 제출한 `nf` 또는 `rvnf`가 특정 AuditRecord ciphertext의
복호화 결과인지 Contract가 다시 증명하도록 요구하지 않는다. 올바른 시작점
선택, off-chain 감사 결과 사용과 전체 target 집행은 신뢰된 Status Authority의
책임이며, Contract가 전체 frontier의 포함 여부를 독립적으로 검증하는 것은 아니다.

각 target에 대해 실행 시점의 상태를 읽고 다음과 같이 처리한다.

| 현재 상태 | 이 감사 실행의 처리 |
|---|---|
| 미소비 + Active | 해당 typed spendId를 Frozen으로 변경하는 transaction 제출 |
| 미소비 + Frozen | 중복 Freeze를 보내지 않고 이미 차단된 것으로 기록 |
| 미소비 + Revoked | 상태를 바꾸지 않고 이미 차단된 것으로 기록 |
| 이미 소비됨 | 동결하지 못한 target으로 유지하고 소비 record ID를 기록 |
| 상태 조회 실패 | 결과 미확인으로 유지; Active나 이미 차단됨으로 추정하지 않음 |

이 절차는 Active를 직접 Revoked로 바꾸거나 이미 Frozen/Revoked인 상태를 Active로
되돌리지 않는다. 개별 Freeze 실패 때문에 이전에 성공한 동결을 되돌리지도
않는다. 가능한 다른 target의 처리는 계속하되, 공통 실행 장애 때문에 진행할
수 없는 target은 미시도 사유를 남긴다. 실행은 유한한 시도·확인으로 끝내며,
불명확한 제출 결과를 성공 또는 실패로 임의 판정하지 않는다.

Freeze transaction 자체의 성공 영수증과 target의 최종 차단 상태는 구분해
기록한다. 예를 들어 다른 승인된 실행이 먼저 동결해 이번 transaction이
거부됐어도, 공통 결과 확인에서 이미 차단된 상태로 확인될 수 있다.

### 13.4 결과 확인과 완료 조건

상태 변경 시도 후 같은 ledger의 확정 prefix `H_r`을 공통 결과 확인 시점으로
고정한다. `H_r`은 `H_t` 이후 또는 동일 시점이며, 성공 근거로 삼는 transaction을
포함해야 한다. 모든 target의 `spentIn`과 status를 이 동일한 `H_r`에서 다시
읽는다. 서로 다른 시점에 관찰한 개별 성공을 합쳐 전체 동시 차단처럼 보고하지 않는다.
실행 중 사용한 상태 관찰과 처리 결과보다 과거의 prefix로 되돌아가 완료를
판정하지 않는다. 특히 소비 경합을 관찰한 뒤 그 소비 이전의 상태를 골라
차단 완료라고 보고할 수 없다. 관찰 block과 transaction을 확인할 수 있는
공통 확정 시점을 유한한 확인 안에 확보하지 못하면 결과는 미완료다.

```text
Blocked_H_r(target) :=
  spentIn_H_r[target.type][target.spendId] == 0
  AND status_H_r[target.type][target.spendId] in {Frozen, Revoked}

COMPLETE_AT_CHECKPOINT :=
  TraceForward가 정상 완료됨
  AND T가 비어 있지 않음
  AND H_r이 위 시점·확정 조건을 만족함
  AND targets와 targetResults의 대상 집합이 각각 전체 T와 정확히 같음
  AND 제출했는지 또는 처리됐는지 불명확한 transaction이 없음
  AND 모든 target in T에 대해 Blocked_H_r(target)
```

`Frozen`은 나중에 권한 있는 조치로 해제될 수 있으므로 위 완료 판정은
`H_r`의 상태에 대한 것이다. 차단된 상태가 유지되는 동안 해당 객체의 소비가
거부된다는 제6.2절의 집행 조건을 사용하며 영구 봉쇄를 주장하지 않는다.

결과는 최소 다음 정보를 구분한다.

```text
AuditAndFreezeResult = {
  ledgerContext, start, H_t,
  traceResult,             // graph, Note/Voucher frontier, Claim/Exit terminals
  targets: T,
  targetResults[],         // Target, 시도/영수증, 상태·spentIn 관찰, 실패/미시도 사유
  verificationSnapshot: H_r or unavailable,
  outcome
}

outcome in {
  TRACE_FAILED,
  NO_LIVE_TARGETS,
  COMPLETE_AT_CHECKPOINT,
  INCOMPLETE
}
```

정상 trace와 비어 있지 않은 `T`를 얻었지만 전체 완료 조건을 만족하지
못하면 `INCOMPLETE`다. 소비 경합, 아직 Active인 target, 조회 실패,
확정되지 않은 제출 또는 미시도를 각각 드러낸다. 부분 집행 결과를 버리지
않으며, 단순히 "audit 성공"이라고 반환하지 않는다. 승인 부재·잘못된 ledger
범위는 시작 전 거부하며 위 정상 실행 결과와 섞지 않는다.

이 결과는 감사자의 off-chain 자료다. 전체 graph나 미공개 연결을 새 공개
온체인 로그에 기록하도록 요구하지 않는다. Status transaction에서 발생하는
공개 leakage는 제14장의 범위를 따른다.

### 13.5 AuditAndFreeze 알고리즘

```text
AuditAndFreeze(request, TraceProvider, StatusClient):
  require Status Authority가 ledgerContext, start, H_t의 차단을 승인함
  require ledgerContext와 확정 H_t가 유효함

  trace <- TraceForward(H_t, start, request.releasedShares)
  if trace failed or frontier 일관성 검증 실패:
    return TRACE_FAILED                  // 이 실행의 Freeze 제출은 0개

  T <- trace의 전체 typed frontier를 정규화하고 고정
  if T is empty:
    return NO_LIVE_TARGETS with trace

  for each target in T:
    current <- 현재 typed spentIn/status 조회
    if current 조회 실패:
      target의 미확인 사유 기록
    else if current.spentIn != 0:
      소비 경합 및 consumer 기록
    else if current.status in {Frozen, Revoked}:
      이미 차단된 관찰 기록
    else if current.status == Active:
      target의 Freeze 제출 및 처리 결과 기록
    else:
      유효하지 않은 상태 응답으로 기록하고 성공으로 처리하지 않음
    // 실패한 target도 T에 남는다. 실행 장애로 미시도한 target도 기록한다.

  H_r <- 유한한 확인 후 공통 확정 결과 시점 선택
  finalStates <- H_r에서 전체 T의 typed spentIn/status 조회
  outcome <- 제13.4절 조건이면 COMPLETE_AT_CHECKPOINT, 아니면 INCOMPLETE
  return trace, 전체 T, 대상별 결과, H_r, outcome
```

### 13.6 Snapshot과 실행 시점의 차이

snapshot에서 frontier였던 객체가 Freeze 전에 소비됐다면 Contract의 current
`spentIn` 검사가 그 상태 변경을 거부한다. 해당 target은 `INCOMPLETE` 사유로
남는다. 이는 `H_t`에 대한 trace가 틀렸다는 뜻은 아니지만, 그 감사 실행이
해당 경로의 추가 사용 차단까지 완료했다는 뜻도 아니다.

소비된 target의 새 descendant를 처리하려면 새 snapshot에 대한 후속 감사를
실행해야 한다. 현재 실행의 `T`를 몰래 교체하거나, 자동 재추적을 무한 반복해
완료를 보장하는 절차로 확장하지 않는다. 같은 `T`의 미확인 transaction을
확인하거나 아직 Active인 target을 재시도하는 것과 새 graph를 추적하는 것은
구분한다. 구체 timeout·재시도 한도·확정성 선택은 배포 profile에서 명시한다.

v2는 snapshot 이후 생성된 descendant를 같은 감사 결과에 자동 포함하거나,
모든 post-snapshot descendant가 원자적으로 차단됐다고 주장하지 않는다.

## 14. Privacy와 공개 leakage

### 14.1 정상 운영에서 공개되는 정보

공개 원장은 다음을 드러낼 수 있으며 privacy 주장은 이를 제외한다.

- Event kind, 등록 경로의 p와 Process scope identifier, exact arity,
  transaction caller와 순서·시각. 고정 경로의 record.policyRef는 0이다.
- Public output reference와 같은 Event에서 생성된 sibling output 관계.
- 소비 시의 nf/rvnf, membership root, ciphertext와 길이·배열 수.
- Transfer와 Recall의 공개 absolute deadline.
- Claim handle, producer index, 공개된 DPP 필드·policy·nonce.
- Status transaction의 typed spendId, 변경 상태, 권한 있는 caller와 실행 순서.

Private opening, owner secret, 수치 상태, 실제 parent reference, 비공개
증분·손실·배분과 public commitment에서 미래 spendId로 가는 대응은 정상
운영에서 숨기는 대상이다. 외부 DPP가 일부 정보를 공개하면 그 공개를
지우는 privacy 보장은 없다. 공개 주소·타이밍·정책·arity로 가능한 상관관계도
자동으로 제거되지 않는다.

### 14.2 승인된 공개와 상태 변경

감사자는 승인된 복호화로 actual parent와 미래 spendId의 연결을 배운다.
이는 정상 운영의 ledger privacy에서 제외하는 승인된 disclosure다.
동결에 사용한 미래 spendId가 공개되면 이후 공개 기록 및 이미 알려진 정보와
연결될 수 있다. 한 번 배운 graph와 식별자를 사후에 잊게 만들 수 없다.

전체 graph를 새로운 공개 온체인 감사 로그로 기록할 필요는 없다.
키 조각을 받은 auditor는 그 key에 대응하는 암호문을 로컬에서 복호화할 수 있다.
시작점·snapshot은 감사 계산의 범위이며, 전달된 키의 사용 범위를 암호문별로
제한하는 장치는 아니다. 해당 키로 추가로 해독 가능한 자료의 취급은 신뢰된
auditor의 책임이다. 동일 key의 미래 암호문에 대한 자동 접근 만료도 주장하지 않는다.

감사 요청·수령한 키 조각·plaintext·대상별 보고서의 보관과 접근 통제는 운영
profile에서 정한다. 키 전달과 감사 허용이 모든 감사 자료의 공개 게시 승인까지
의미하지는 않는다.

## 15. 보안 요구사항과 성립 조건

이 절은 충족해야 할 다섯 성질을 기술한다. 구체 scheme에 대한 게임 기반
정의와 증명, circuit·contract conformance가 제시되기 전에는 증명된
정리나 검증 완료로 취급하지 않는다.

### 15.1 Correctness

유효한 profile의 고정 규칙 또는 등록 정책 아래 정직한 witness로 만든 전이는 안정된
pre-state에서 모든 runtime 조건이 충족되면 승인되고 정확한 상태 갱신을
수행해야 한다. 유효한 Issue의 문서와 DPPClaim은 공개 검증을 통과해야 한다.
Proof 생성 후 status·spentness·deadline이 바뀐 경우까지 승인을 보장하지 않는다.

### 15.2 Soundness

공격자가 proof나 제출 자료를 조작해 opening·membership·ownership·권한·정책
검사에 어긋난 전이를 승인시키거나, 동일 입력을 반복 계상·소비하거나,
암호화된 graph와 실제 전이를 다르게 만들거나, 등록되지 않은 Claim을 유효하게
검증시키기 어려워야 한다. Current status에 반하는 소비와 stale deadline의
Recall도 거부해야 한다.

이 요구는 신뢰된 배포 구성의 고정 relation 또는 권한 기관이 등록한 relation과 명시된 초기 입력 가정에
대해서다. 외부 값의 물리적 진실성이나 Policy Authority가 잘못 승인한
정책의 현실적 적합성은 proof soundness의 결과가 아니다.

### 15.3 Explicit-leakage ledger privacy

제14장의 공개 leakage와 승인된 disclosure가 동일한 두 유효 history에 대해,
신뢰된 auditor를 제외한 허용된 공격자가 그 외 private state와 실제 연결을 유의미하게 구별하기
어려워야 한다. Commitment hiding, 다중 proof 환경의 ZK, K명 미만 노출에
대한 threshold secrecy, nullifier의 적절한 의사난수성·unlinkability가 필요하다.
단순한 hash 충돌 저항성만으로 이 전체 성질을 주장하지 않는다.

### 15.4 Authorized backward auditability

유효한 start에 대해 필요한 record와 외부에서 전달된 K개 이상의 유효한 키 조각이 있으면
accepted 실제 parent graph를 정확히 복원해야 한다. Record 누락이나 발견된 입력·
복호화 오류는 잘못된 graph의 성공 반환으로 이어지면 안 된다. 제7장의 실제
입력 binding, immutable producer index, 유효한 키 전달 전제와 복호화 정확성에
의존한다. 외부에서 악의적인 키 조각을 전달해도 반드시 검출한다는 성질은 포함하지 않는다.

### 15.5 Snapshot forward traceability

확정 H_t와 유효한 start에 대해 외부 키 전달 전제가 충족되면 제12장의 정확한
downstream graph, typed frontier와 Claim/Exit 종단을 복원해야 한다.
미래 식별자 ciphertext의 output binding과 소비 시점의 원자적 consumer
index 기록이 필요하다. H_t 이후의 graph까지 포함하는 성질은 아니다.

### 15.6 AuditAndFreeze의 절차적 완료

`COMPLETE_AT_CHECKPOINT`는 제13.4절의 전체 대상·공통 시점·상태 확인 조건이다.
정확한 trace, 올바른 SA 집행과 원장 검증을 연결한 절차적 판정이며 독립적인
여섯 번째 암호학적 정리로 추가하지 않는다. H_r에서 모든 target이 미소비이고
Frozen/Revoked이면 그 차단 상태가 유지되는 동안 원장은 후속 소비를 거부한다.
Race, 불명확한 제출, partial result나 나중의 unfreeze를 이 판정으로 숨길 수 없다.

## 16. 배포 profile의 필수 결정 항목

이 명세는 아래 매개변수를 고정한 배포들에 적용한다. Profile은 public
identifier와 version으로 구분하고 관련 회로·client·verifier가 동일하게
사용해야 한다. 값이 정해지지 않은 항목을 구현자가 transaction마다 임의로
선택하게 해서는 안 된다.

| 항목 | 고정할 값과 충족할 조건 |
|---|---|
| 암호·encoding | 보안 수준, 유한체·곡선·hash, domain encoding, canonical type/tuple/array, hash-to-field, OwnerKey, secret·opening domain과 난수 생성. |
| 정수 회계 | 질량·credit·탄소 단위와 scale, 필드별 bit bound, 합계·교차곱·나머지 범위, no-wraparound 검증 방법. Entry 초기 탄소 입력의 생략은 0, 명시한 유효한 값은 보존한다. |
| ZK | 증명 scheme, setup 신뢰·키 배포, completeness·soundness·ZK 가정, 고정 Event 또는 등록 정책의 proving/verifying key와 공개 입력 순서. |
| Threshold encryption | K·N, key 생성·보관, 수령한 키 조각의 ID·encoding·key 식별, ciphertext/context, 로컬 복호화와 실패 처리, key 고정 또는 명시적인 epoch·과거 복호화 규칙. |
| Membership | 분리된 depth-32 tree의 leaf/node hash, index·path encoding, empty root, append와 accepted root 저장·복구 방법. |
| 정책 | 제5.1절의 세 검증 경로, 고정 Event의 relation·VK 대응, 지원 exact Process arity 집합과 role vector, 참조 상수·Issue threshold, 등록 정책 manifest 인증·배포. |
| 권한 | Entry Issuer, Policy Authority, Status Authority의 인증·역할 관리, submitter/relayer 경로, Process scope credential 및 grant lifecycle. |
| Deadline | block height에서 CurrentEpoch를 계산하는 함수와 Transfer의 허용 D 범위. Transfer `D > current`, Recall `current <= D`를 보존한다. |
| DPP | ProductName·LotID·Unit의 필드 경로, type·문자열 정규화·누락 처리, HashDocumentInfo와 DPPClaim encoding. |
| 원장 조회 | Chain·ledger 식별, 확정 기준, immutable prefix 식별과 인증된 history/state 조회, archive 자료 가용성. |
| 감사 운영 | 유효한 키 조각의 외부 전달 전제, 차단 승인과 query-only 실행 구분, 수령 키·private 감사 자료 접근·보관, status transaction 서명·제출·영수증 해석. 외부 승인·전달 서비스 구현은 audit 범위 밖이다. |
| AuditAndFreeze 종료 | 제출·재시도·확인 한도, unknown transaction 처리와 공통 확정 H_r 선택. 전체 target 보존과 네 outcome 의미를 유지한다. |

Full VK나 record를 저장하는 구체 layout은 달라도 논리 상태와 검증 의미는
같아야 한다. 등록 경로의 normative relation·VK를 바꾸면 새 정책 identity로
등록한다. 고정 경로의 변경은 명시적인 새 배포 profile과 verifier 구성으로 식별한다.
기존 암호문이나 상태의 해석을 바꾸는 변경은 명시적 배포·호환 정책 없이 적용하지 않는다.

## 17. 구현 적합성 확인 기준

아래는 구현을 평가할 때 확인할 의무이며 이 문서가 실행한 테스트 결과가 아니다.

### 17.1 전이·회계·공개 검증

- 아홉 Event의 정상 경로와 exact arity·type·소유권·수치 경계가 일치한다.
- Entry의 0 input, Exit의 0 output, Issue의 Note 소비·Claim 생성이 원자적으로 처리된다.
- Entry의 초기 탄소 생략·명시적 0·양수 입력을 구분해 확인하고 잘못된 명시
  입력은 거부한다. Note commitment와 proof는 실제 사용한 초기 탄소를 결합해야 한다.
- Wrong owner, 변조 opening/path, 동일 input 반복, 중복 소비, stale/frozen input,
  잘못된 verifier, 등록 경로의 잘못된 p와 Process scope, 중복 output 또는 Claim을 거부한다.
- 일곱 고정 Event가 registry·scope 없이 올바른 고정 verifier로 검증되고,
  Process는 policy·grant, Issue는 policy 검사를 적용하는지 확인한다.
- Transfer의 zero Change Note·탄소 증분, Split의 floor/residual,
  Process의 손실·credit 보존·탄소 배분·고정 role을 검사한다.
- WASTE의 모든 비-Exit 소비와 영분모·overflow·음수 encoding을 거부한다.
- Recall의 D 직전·동일·직후 및 proof 생성 이후 지연 제출, Proceed/Recall 경합을 검사한다.
- DPP 필드·p·nonce·h 변조와 미등록 Claim을 거부하고 과거 Claim의 정책 disable 의미를 지킨다.

### 17.2 감사의 정확성과 실패 처리

- Actual parent와 미래 소비 식별자를 바꾸거나 ciphertext 순서·수를 바꾸면 proof가 거부된다.
- Typed reference·spendId, producer/spent index와 record가 일관되고 원자적으로 갱신된다.
- K개 미만·중복 구성원·noncanonical 수령 키 입력을 거부하고, 유효하게 전달된
  K개 조각을 이용한 로컬 복호화를 확인한다. 발견된 record/context·복호화 오류는
  거부한다. 외부 구성원의 원격 partial-response 공격 검증은 요구하지 않는다.
- 분기·합류·중복 도달·다중 output·Claim·Exit에 대해 기대 node/edge/frontier의 정확한 집합을 비교한다.
- Backward의 output 없는 record 시작과 Forward의 output 집합 시작 의미를 구별한다.
- Snapshot 뒤의 Event가 현재 trace에 섞이지 않으며 누락 record·조회 실패가 fail로 끝난다.

### 17.3 AuditAndFreeze 통합

- 승인된 실제 trace 결과의 **전체** typed frontier를 입력 fixture로 대체하지 않고 집행한다.
- Trace 실패 시 제출이 0개이고 target의 삭제·추가·잘못된 dedup을 검출한다.
- Active, 이미 Frozen/Revoked, 소비 경합, 조회 실패, 미시도와 제출 unknown을 구별한다.
- 일부 동결 실패가 다른 대상의 결과를 지우거나 이미 성공한 동결을 rollback하지 않는다.
- Transaction 영수증과 상태 확인을 구별하고 모든 target을 같은 확정 H_r에서 재조회한다.
- 관찰된 소비보다 과거 H_r을 골라 완료로 보고하거나 unresolved transaction을 숨길 수 없다.
- 빈 frontier와 Claim/Exit 종단, 정확한 target/result coverage, 네 outcome을 검증한다.
- 동결된 실제 Note/Voucher의 후속 소비가 거부되고 무관한 정상 객체의 소비는 허용되는지 확인한다.
- Freeze 전 소비된 target은 INCOMPLETE로 남고 새 descendant는 별도 snapshot 감사 대상으로 다룬다.

적합성 판단에는 profile·policy manifest·circuit·contract·감사 client의 대응과
재현 가능한 실행 근거가 필요하다. 문서 정의, 정적 소스 검사와 실행 결과를
서로 구분하여 보고한다.
