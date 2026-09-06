# zkDPP-poc-v1 아키텍처

이 문서는 milestone과 무관하게 유지되는 시스템 전체 구조를 설명합니다. 진행 상태와 결과는 [`MILESTONES.md`](MILESTONES.md), 미래 기능 논의는 [`milestones/FUTURE-MILESTONE-CONTEXT.md`](milestones/FUTURE-MILESTONE-CONTEXT.md)에서 확인합니다.

Architecture는 배경 지도이며 Milestone 구현 명세를 대체하지 않습니다. 각 Milestone은 필요한 데이터·pseudo code·Interface·Gate를 자체적으로 반복해 설명합니다.

## 큰 그림

```text
private 객체 내용
  → Commitment
  → Membership Tree
  → Private Spend·Nullifier
  → 공급망 Event
  → Contract 상태 변경
```

실제 내용은 숨기고 commitment를 장부에 등록합니다. 소비할 때는 객체 내용·Merkle path·소유자 secret을 공개하지 않고 유효성과 소유권을 증명합니다. Contract는 공개 nullifier로 중복 소비를 막습니다.

## 기본 Note

```text
DocumentInfo = (ProductName, LotID)
DocumentHash = HashDocumentInfo(DocumentInfo)

State = (q_mass, a_rec, e)

Note
  ├─ DocumentHash
  ├─ State
  ├─ AssetRole
  ├─ address
  └─ opening
```

| 값 | 의미 | 저장 형식 |
|---|---|---|
| `q_mass` | 물품 전체 질량 | kg × $10^9$, uint64 |
| `a_rec` | 할당된 재활용 mass-balance credit 절대 질량 | kg × $10^9$, uint64 |
| `e` | Entry 이전을 포함할 수 있는 누적 탄소발자국 | kgCO2e × $10^9$, uint64 |

`Quantity`와 `Unit`은 Note에 두지 않습니다. `AssetRole`은 물질 종류가 아니라 Protocol 사용 역할이며 초기 값은 ELIGIBLE·WASTE입니다.

## 소유자·Commitment·Nullifier

$$
\mathrm{address}=H(\mathrm{OwnerTag},\mathrm{sk}_{\mathrm{owner}})
$$

$$
cm=H(\mathrm{NoteTag},\mathrm{DocumentHash},\mathrm{AssetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{address},\mathrm{opening})
$$

$$
nf=H(\mathrm{NullifierTag},\mathrm{sk}_{\mathrm{owner}},cm)
$$

`address`는 Ethereum EOA가 아니라 BLS12-381 field의 ZK 소유자 식별값입니다. 별도 PublicKey 계층은 사용하지 않습니다.

| 공개값 | 비공개값 |
|---|---|
| output commitment·membership root·nullifier | 객체 내용·소유자 secret·소비 commitment·Merkle path |

## Membership Tree

- Note Tree와 Voucher Tree는 서로 독립된 Depth-32 append-only 구조입니다.
- Contract는 현재 leaf·중간 node와 level별 zero Hash를 저장할 수 있습니다.
- 정상적으로 생성된 과거 root는 membership proof 수용에 사용할 수 있습니다.
- Contract path 조회는 현재 Tree 기준이며 과거 root별 path snapshot은 기본 범위가 아닙니다.
- Circuit membership과 Contract Tree Update 비용은 별도로 측정합니다.

## Voucher와 private resolution

Voucher는 완료되지 않은 Transfer를 나타내는 private payload입니다.

```text
Transfer:
  private Note → public Voucher commitment + public Change commitment

Proceed·Recall:
  private Voucher + private path
  → public rvnf + public output Note commitment
```

새 `rv`는 Transfer output으로 공개해 Voucher Tree에 등록하지만, Proceed·Recall이 소비하는 `rv`는 private witness입니다.

$$
rvnf=H(\mathrm{VoucherNullifierTag},o_{rv},rv)
$$

Sender와 Receiver가 공유하는 $o_{rv}$로 같은 `rvnf`를 계산합니다. Proceed는 Receiver secret, Recall은 Sender secret을 별도로 검증합니다. Contract는 public `rvnf`로 둘 중 하나만 허용합니다.

`deadlineEpoch`은 Voucher의 private witness이지만 생성 시 공개되는 `transferEpoch+deltaEpoch`로 숫자 자체는 계산할 수 있습니다. 숨기는 것은 resolution transaction이 어느 Voucher의 deadline을 사용했는지에 대한 연결입니다. Recall Circuit은 Contract가 고정한 public `currentEpoch`에 대해 strict inequality를 검증하며 raw `rv`를 key로 하는 deadline mapping은 사용하지 않습니다.

## 객체 생애주기

```text
Entry:     ∅ → Note
Transfer:  Note → Voucher + Change Note
Proceed:   Voucher → Note
Recall:    Voucher → Note
Merge:     Note + Note → Note
Split:     Note → Note + Note
Process:   Note[m] → Note[n]
Exit:      Note → Finalized DPP
Issue:     Finalized DPP → 같은 DPP + Policy Claim
```

### Entry 참여자와 승인

현재 Contract는 Admin이 EVM account에 Entry 호출 권한을 부여하고, 승인된 account가 Entry하는 구조입니다. 기존 이름 entryIssuers는 이 호출 권한을 나타내며, 공급망의 모든 참여자를 등록하는 일반 명부는 아닙니다.

M7의 역할은 **A가 B에게 Entry 권한을 부여하고, 공급망 참여자 B가 자기 최초 Note를 만드는 구조**입니다. A가 B 소유의 Note를 대신 만들거나 B의 owner secret을 받지 않습니다. B가 자기 secret으로 주소와 최초 Note의 소비 nf를 계산하고 올바르게 암호화했다는 관계를 Entry proof가 검증합니다.

M1~M6 baseline Entry에는 이 검사가 없고 M7 감사 Entry에 추가됐습니다. EVM 호출 권한과 ZK 소유권은 별개이며, 동일 Participant 역할이라는 이유만으로 msg.sender와 ZK address가 암호학적으로 연결됐다고 주장하지 않습니다.

### Merge compatibility 경계

논문 Protocol은 서로 다른 Lot을 Merge할 때 동일 ProductProfile을 요구합니다. 현재 POC Note에는 ProductProfile이 없으므로 M4는 같은 owner·같은 AssetRole만 검사합니다. ELIGIBLE+ELIGIBLE과 WASTE+WASTE는 허용하지만 서로 다른 Role은 거부합니다.

M4 POC의 output DocumentHash는 새 private 값이며 input과의 관계를 검증하지 않습니다. 이는 private membership·State 산술·nullifier·EVM 원자성 workload를 확인하기 위한 adaptation이며 논문 ProductProfile compatibility를 구현한 것으로 해석하지 않습니다.

Split은 output 2의 재활용·탄소를 질량 비례로 내림 계산하고 output 1이 나머지를 받습니다. 이 POC 배분 규칙은 확정된 구현이며, ProductProfile의 구체적인 데이터 모델을 구현하지 않은 것과 구분합니다.

## 기능 계층

```text
데이터·암호 핵심
  → Document, State, Role, Owner, Commitment, Nullifier, Merkle

객체 생애주기
  → Note, Voucher, Claim과 Event

정책·권한
  → exact-arity Policy Circuit, VK Registry, Grant

상태 집행
  → Active, Frozen, Revoked

비공개 감사
  → AuditRef, AuditRecord, encrypted parents, partial decryption

통합·실험
  → universal SRS, Anvil, benchmark
```

하위 기능이 실제 두 소비자에서 같은 계산·encoding·Privacy·실패 조건을 가질 때만 Shared Core로 승격합니다.

## SRS lifecycle

개발 milestone은 correctness 확인용 임시 SRS를 사용할 수 있습니다. 모든 최종 Policy Circuit이 확정되면 다음 순서로 통합합니다.

```text
모든 Circuit compile
  → 최대 domain 결정
  → universal canonical SRS 하나
  → domain별 Lagrange SRS
  → Policy별 PK·VK
```

Universal SRS 생성, Lagrange 변환과 Policy Setup 비용을 구분합니다.

M9 final POC는 최대 domain $2^{17}$의 development universal canonical SRS 하나를 사용합니다. $2^{14}$·$2^{15}$·$2^{16}$·$2^{17}$ Lagrange SRS에서 고정 Event 7개와 Policy Circuit 3개의 PK·VK를 생성합니다. 이 SRS는 Production Ceremony 결과가 아니며 더 큰 미래 Circuit은 별도 SRS version이 필요합니다.

## Process Policy

Policy Authority는 공개 EVM account와 `authorityId`를 사용하고 Operator는 private `sk_owner`를 사용합니다. PolicyRef는 Authority의 `(eventKind,policyId,version)`을, ScopeRef는 같은 owner secret과 PolicyRef를 Poseidon2로 binding합니다.

M5 Process는 exact 3-to-2이며 고정 rate·allocation으로 ELIGIBLE Note 3개를 ELIGIBLE·WASTE Note로 변환합니다. 공식 경로는 Policy별 constant verifier이고 optimized VK storage verifier는 architecture ablation입니다.

## 현재 Main Protocol과 nf 기반 Status 집행

M9 final 경로의 ZkDPPClaimLedger가 현재 Main Protocol입니다. M8 의미를 유지하면서 고정 Event verifier 7개는 constructor에, Process·Issue verifier 3개는 PolicyRecord에 연결합니다. 별도 Router와 Process 전용 immutable verifier는 없습니다. M8 Contract와 M7 ZkDPPAuditLedger는 baseline으로 보존합니다.

Contract는 소비 시 이미 공개되는 nf·rvnf로 미소비 여부와 Active 상태를 확인합니다. Note·Voucher membership Tree는 유지하지만 StatusTree·Active path·StatusUpdate proof는 사용하지 않습니다.

상태는 Active=0, Frozen=1, Revoked=2이며 Active→Frozen, Frozen→Active, Frozen→Revoked만 허용합니다. constructor에서 고정한 Status Authority만 변경할 수 있고 이미 소비된 대상은 거부합니다. 동결한 nf와 이후 소비 transaction이 공개적으로 연결되는 Privacy trade-off를 허용합니다.

### M6에 보존된 StatusTree baseline

M6의 `ZkDPPStatusLedger`와 Status-aware Circuit은 비교용으로 보존된 과거 Main 경로입니다. M1~M5 Circuit·`EntryExitLedger`도 구성 요소 검증용 baseline입니다.

```text
Note Tree index i    ↔ NoteStatusTree index i
Voucher Tree index j ↔ VoucherStatusTree index j
```

두 depth-32 Sparse StatusTree는 오프체인 Indexer가 유지하고 Contract는 최신 root만 저장합니다. Active는 empty leaf `0`, Frozen은 `1`, Revoked는 `2`입니다. 정상 output은 Status root를 바꾸지 않습니다.

소비 Circuit은 기존 private object index를 Status path에 재사용하고 Contract가 제공한 current Status root에서 Active를 증명합니다. Status Authority는 private path로 one-leaf StatusUpdate proof를 생성하며 Contract는 proof 검증 후 new root 하나만 SSTORE합니다. Status 대상·index·전이는 공개되고 이후 private 소비와의 직접 연결은 숨겨집니다.

이 방식은 M7 새 경로에서 대체됐지만 M6 구현·명세·결과는 비교 기준으로 보존합니다.

## DPP와 Sustainability Claim

공급망 안의 DPP Object는 private Note로 표현합니다. M8 Exit는 Note를 소비하고 같은 DocumentHash·AssetRole·State를 가진 공개 dppCommitment를 생성합니다. dppCommitment에는 owner address·Note opening·nf가 포함되지 않습니다.

Issue는 DPP를 소비하지 않고 issuePolicyRef Claim을 추가합니다. Claim은 dppCommitment와 issuePolicyRef의 조합이며 같은 조합은 한 번만 등록할 수 있습니다. 제품 정보·State·dppOpening은 private witness이고, 제3자는 Claim 등록 여부와 Active·Frozen·Revoked 상태를 조회합니다.

Issue에는 PolicyGrant·policyScopeRef가 없습니다. dppOpening은 hiding 값이지 현재 소유권 credential이 아니며 DPP ownership transfer는 구현하지 않았습니다. 자세한 경계는 [M8 Result](milestones/M8-exit-dpp-issue-result.md)에 있습니다.

## 감사 암호화 코어

M6-B1은 기존 Main Ledger와 분리된 코어입니다. Jubjub의 2-of-3 Threshold DH와 기존 Poseidon2로 transaction 키 하나를 만들고, 위치별 Field 마스크를 사용합니다. 위원회 공개키는 Circuit 상수이며 부모 참조·출력 소비 nullifier는 Event의 실제 witness에서 계산해야 합니다.

대표 Process의 평문은 $(cm_A,cm_B,cm_C,nf_D,nf_E)$입니다. 독립 암호화 관계 검증과 실제 Event 평문 binding은 별개의 책임입니다. 진단용 AuditEncryptionStore는 proof로 검증한 공개 입력을 그대로 저장할 뿐, 현재 원장의 소비·PolicyGrant·Status를 집행하지 않습니다.

신뢰 Setup·정직한 위원회와 Auditor·검증된 원본만 복호화한다는 경계가 있습니다. 원 TDH2의 일반적인 CCA 보안·응답 증명을 그대로 제공하지 않습니다. 전체 AuditRecord·탐색·nullifier 기반 동결 전환은 [후속 Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m6-b1)에서 별도로 설계합니다.

## 감사 데이터 저장 원칙

M7은 AuditRecord에 encryptedParents·encryptedOutputNfs·공개점·outputRefs와 필요한 기록 메타데이터를 저장합니다. 암호문은 calldata에만 두지 않습니다. public input 전체를 복사하지 않는 것과 감사 기록 자체를 storage에서 제거하는 것은 다른 선택입니다.

noteRoot·policyScopeRef·입력 nf·proof 등은 모두 Record에 복사하지 않습니다. AuditRecorded의 기록 ID로 생성 transaction을 찾고 검증된 공개 입력에서 문맥을 재계산합니다. Event 종류·출력·암호문을 저장 원본과 대조하는 기준은 [M7 단일 명세](milestones/M7-audit-tracing.md)에 있습니다.

감사 프로그램의 과거 transaction 조회와 Contract의 상태 조회는 다릅니다. M7은 producerOf로 생성 기록을, noteSpentIn·voucherSpentIn으로 소비 기록을 찾고 별도 nf·rvnf 상태 mapping으로 사용 가능 여부를 집행합니다. 소비 ID와 spent boolean을 이중 저장하지 않습니다.

M6-B1의 15-Field 저장은 원본 조회를 단순화한 진단 실험입니다. 이 원칙을 적용해 최소화한 AuditRecord나 그 gas를 이미 구현·측정한 것으로 해석하지 않습니다.

## 아직 확정하지 않은 영역

- 논문 ProductProfile 조건의 구체적인 구현·encoding
- M7 감사 원본 조회의 영구 캐시·운영 서버·내부 multicall 지원
- 공급망 중간 Claim·DPP ownership transfer·component DPP 재귀 구성

현재 논의 상태와 폐기 가정은 Future Context에서 관리합니다.
