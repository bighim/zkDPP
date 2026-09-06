# zkDPP Protocol Reference

이 문서는 zkDPP-poc-v1에서 **어떤 기능을 구현하려 했고, 실제로 무엇을 구현했으며, 각 Operation이 어떤 값을 증명하고 어떤 온체인 상태를 변경하는지** 한 파일에서 설명합니다.

프로토콜의 목적과 동작을 먼저 이해하려면 [사람용 zkDPP 안내서](zkDPP-protocol-guide.md)를 읽습니다. 이 Reference는 public input·Circuit·Contract storage·실제 구현을 확인하거나 수정할 때 사용합니다.

이 문서에서 **Operation**은 Entry·Transfer·Process처럼 Protocol 상태를 바꾸는 기능을 뜻합니다. **Event log**는 Solidity Contract가 transaction receipt에 남기는 로그를 뜻합니다. 두 용어를 구분해서 사용합니다.

이 문서만 읽어도 Protocol을 이해할 수 있습니다. [`zkDPP-protocol.yaml`](zkDPP-protocol.yaml)은 객체·Circuit·Contract·storage 연결을 기계가 읽을 수 있게 정리한 보조 명세이며, 이 문서를 이해하기 위한 선행 자료가 아닙니다.

- 기준 Protocol: M9 final POC
- Main Contract: ZkDPPClaimLedger
- 상세 명세: [M9 최종 통합 명세](../milestones/M9-final-integration.md)
- 실제 결과: [M9 Result](../milestones/M9-final-integration-result.md)
- 목적: 구현 가능성·correctness·성능 확인
- 원칙: **YAGNI가 최우선입니다.** Production에 필요할 수 있다는 이유만으로 현재 검증하지 않은 기능을 추가하지 않습니다.

## 원하는 정보는 어디에서 찾나요?

| 알고 싶은 것 | 읽을 곳 |
|---|---|
| 전체 Protocol의 목적과 흐름 | 0장 |
| Note·Voucher·DPP·Claim의 의미 | 1장 |
| 누가 proof를 만들고 누가 권한을 행사하는지 | 2장 |
| Contract storage의 형태·기본값·목적 | 3장 |
| Circuit 10개의 공개 입력과 verifier | 4장 |
| Operation 하나의 입력·검증·상태 변화 | 6~14장 |
| AuditRecord·추적·Frozen·Revoked | 15~17장 |
| 최종 SRS·실행 시나리오·실제 결과 | 19~23장 |

## 0. 30초 안에 무엇을 기억하면 되나요?

**zkDPP는 공급망 물품의 제품 정보·수량·Sustainability State를 private 객체로 관리하는 Protocol입니다. 물품은 공급망 안에서 Note로 존재하고, 전달 중에는 Voucher가 되며, Exit 뒤에는 DPP commitment로 남습니다. Participant는 Operation마다 ZK proof를 만들고, Contract는 private 내용을 보지 않은 채 올바른 상태 변화인지 검증해 결과만 기록합니다.**

```text
Entry:                  최초 Note 생성
Transfer:               Note → Voucher + Change Note
Proceed 또는 Recall:    Voucher → 새 Note
Merge·Split·Process:     Note의 Sustainability State 변환
Exit:                   Note → DPP commitment
Issue:                  DPP에 Sustainability Claim 등록
```

| 질문 | 답변 |
|---|---|
| 공급망 물품은 무엇으로 표현하나요? | 공급망 안에서는 private Note로 표현합니다. Transfer가 만든 Voucher는 Proceed 또는 Recall되기 전까지의 전달 상태를 표현합니다. Exit가 끝난 Note는 DPP commitment로 표현합니다. |
| Operation을 실행할 때 무엇을 증명하나요? | 입력을 소비하는 Operation에서는 Circuit이 객체의 존재·소유권과 Sustainability State 변화를 검증합니다. Entry와 Issue는 각 Operation 절에 적힌 별도 관계를 검증합니다. |
| Contract에 보이지 않는 것은 무엇인가요? | Note·Voucher의 실제 내용, owner secret, 소비하는 cm·rv와 Merkle path입니다. |
| Contract에 공개되는 것은 무엇인가요? | Contract가 수용한 현재 또는 과거 membership root, 소비 nullifier인 nf·rvnf, 새 commitment, PolicyRef와 감사 암호문입니다. Operation에 따라 필요한 값만 공개합니다. |
| 온체인에는 무엇을 저장하나요? | Note·Voucher Tree, 생성·소비 기록, Policy, AuditRecord, Status와 Claim을 저장합니다. |
| 중복 소비와 Frozen 상태는 어떻게 막나요? | Contract가 nf·rvnf의 소비 기록과 Status를 확인한 뒤 proof를 검증합니다. |
| 부모와 자손은 어떻게 찾나요? | 암호문이 있는 AuditRecord는 위원 두 명의 응답으로 복호화합니다. Auditor는 복원한 값으로 생성 기록과 소비 기록을 순서대로 따라갑니다. Issue AuditRecord에는 암호문이 없습니다. |
| DPP Claim은 어떻게 만드나요? | Exit가 Note와 같은 private Sustainability State의 DPP commitment를 만들고, Issue가 해당 State가 Policy를 만족한다는 Claim을 등록합니다. |
| 현재 구현하지 않은 것은 무엇인가요? | StatusTree·DPP Tree·Claim Tree·별도 Router·Besu·DPP 소유권 이전입니다. |

### 전체 책임은 어떻게 나뉘나요?

```text
Participant
  private 객체와 secret으로 proof 생성

Circuit
  각 Operation에 필요한 객체·소유권·Sustainability State·감사 암호화 관계 검증

Contract
  Operation마다 필요한 공개값·현재 storage·proof 확인
  → 해당 Operation에 필요한 Tree·생성·소비·Claim·AuditRecord만 변경

위원회·Auditor
  온체인에 기록된 암호문을 복호화해 부모·자손 관계 복원
```

### 이 문서의 핵심 용어는 무엇인가요?

| 용어 | 이 문서에서의 의미 |
|---|---|
| commitment | private data를 공개하지 않고 그 data에 대응하는 공개 식별값입니다. Note는 cm, Voucher는 rv, DPP는 dppCommitment를 사용합니다. |
| nullifier | private 객체를 소비할 때 공개하는 중복 소비 방지 값입니다. Note는 nf, Voucher는 rvnf를 사용합니다. |
| private witness | proof 생성자는 알지만 Circuit verifier와 Contract에는 공개하지 않는 입력입니다. |
| Circuit public input | proof가 어떤 공개값에 대해 유효한지 정하는 Field 값의 순서입니다. |
| transaction calldata | EVM transaction이 Contract 함수에 전달하는 byte data입니다. proof와 public input에 대응하는 값이 들어갑니다. |
| Contract storage | transaction이 끝난 뒤에도 온체인에 남는 Contract 상태입니다. |
| Merkle root | 특정 시점의 Tree 전체를 대표하는 Hash입니다. Contract는 정상 append로 생성한 현재·과거 root를 기록합니다. |
| Merkle path | 한 commitment가 특정 root의 Tree에 포함됐음을 계산하는 sibling 값과 leaf index입니다. zkDPP에서는 private witness입니다. |
| verifier | public input과 proof를 받아 Circuit 관계가 성립하는지 확인하는 Contract입니다. |

같은 값이 여러 위치에 나타날 수 있습니다. 예를 들어 nf는 Circuit public input이면서 transaction calldata에 들어가고, 성공 뒤에는 noteSpentIn의 mapping key가 됩니다. 각 Operation의 입력 표에서 이 위치를 따로 표시합니다.

## 1. 어떤 객체를 사용하나요?

### 1.1 Hash 표기는 무엇을 뜻하나요?

zkDPP는 목적에 따라 Hash 방식을 구분합니다.

| 대상 | 방식 | 이유 |
|---|---|---|
| DocumentHash | ProductName·LotID를 NFC 정규화하고 각 UTF-8 byte 길이를 uint32 big-endian으로 붙인 뒤 BLS12-381 hash-to-field 적용 | 문자열 문서를 Field 값 하나로 바꿉니다. |
| cm·nf·rv·rvnf·dppCommitment·policyRef·policyScopeRef | Domain-tagged Poseidon2 Merkle-Damgard | 일반 계산과 Circuit 안에서 같은 Field 관계를 계산합니다. |
| Note·Voucher Merkle node | Poseidon2 two-input compression | 왼쪽·오른쪽 child에서 parent node를 계산합니다. |
| Artifact·optimized VK checksum | SHA-256 | Circuit 밖의 파일과 byte encoding 동일성을 확인합니다. |

아래 객체 수식의 $H$는 첫 입력에 Domain tag를 넣는 Poseidon2 Field hash입니다. 문자열 Domain은 먼저 BLS12-381 Field 상수로 바꿉니다.

| 관계 | Domain 문자열 |
|---|---|
| Note | zkDPP:Note:v1 |
| ZK owner address | zkDPP:Owner:v1 |
| Note nullifier | zkDPP:Nullifier:v1 |
| Voucher | zkDPP:Voucher:v1 |
| Voucher nullifier | zkDPP:VoucherNullifier:v1 |
| DPP | zkDPP:DPP:v1 |
| PolicyRef | zkDPP:PolicyRef:v1 |
| ScopeRef | zkDPP:ScopeRef:v1 |

### 1.2 공통 Sustainability State는 무엇인가요?

$$
State=(q_{\mathrm{mass}},a_{\mathrm{rec}},e)
$$

| 값 | 의미 | 단위와 값 범위 |
|---|---|---|
| $q_{\mathrm{mass}}$ | 전체 질량 | kg × $10^9$, uint64 |
| $a_{\mathrm{rec}}$ | 할당된 재활용 mass-balance credit | kg × $10^9$, uint64 |
| $e$ | Entry 이전을 포함하는 누적 탄소 | kgCO2e × $10^9$, uint64 |

이 절의 State는 물품의 Sustainability State를 뜻합니다. 뒤에서 설명하는 Contract storage state와는 다른 개념입니다. 세 값은 Note·Voucher·DPP의 private data와 Circuit witness에 들어갑니다. Contract는 세 값을 직접 저장하지 않고 commitment만 저장합니다.

모든 Sustainability State는 다음을 만족해야 합니다.

- $a_{\mathrm{rec}}\le q_{\mathrm{mass}}$입니다.
- AssetRole은 ELIGIBLE=0 또는 WASTE=1입니다.
- WASTE이면 $a_{\mathrm{rec}}=0$, $e=0$입니다.

### 1.3 Note는 무엇인가요?

Note는 공급망 안에 있는 물품의 현재 private 상태입니다.

DocumentInfo는 ProductName과 LotID로 구성하며, 이를 Hash한 DocumentHash만 Note에 넣습니다. Unit과 Quantity 필드는 사용하지 않습니다.

Note의 address는 $sk_{\mathrm{owner}}$에서 계산하는 **ZK owner address**입니다. EVM transaction의 msg.sender와 다른 종류의 값입니다.

```text
Note
  DocumentHash
  AssetRole
  q_mass
  a_rec
  e
  address  # ZK owner address
  opening
```

$$
address=H(OwnerTag,sk_{\mathrm{owner}})
$$

$$
cm=H(NoteTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,address,opening)
$$

$$
nf=H(NullifierTag,sk_{\mathrm{owner}},cm)
$$

| 시점 | 값 | 왜 필요한가요? |
|---|---|---|
| Note를 만들 때 온체인에 공개 | $cm$ | Note 내용을 숨긴 채 존재를 등록합니다. |
| Note 소유자가 보관 | Note의 실제 내용·opening·$sk_{\mathrm{owner}}$ | 나중에 이 Note를 사용할 proof를 만듭니다. |
| Note를 소비할 때 온체인에 공개 | Note root | Circuit이 어떤 Note Tree를 기준으로 membership을 검증하는지 정합니다. |
| Note를 소비할 때 온체인에 공개 | $nf$ | Contract가 같은 Note의 중복 소비를 막을 때 사용하는 값입니다. |
| Note를 소비할 때 proof 안에서만 사용 | $cm$·Note 내용·$sk_{\mathrm{owner}}$·index·Merkle path | 소유권과 Tree membership을 공개하지 않고 검증합니다. |

Note를 만들 때는 $cm$만 온체인에 등록합니다. 소유자는 실제 Sustainability State와 secret을 오프체인에서 보관합니다. $nf$는 $sk_{\mathrm{owner}}$와 $cm$이 정해지면 하나로 결정됩니다. Note를 소비할 때는 $cm$을 transaction에 다시 넣지 않고 이 $nf$를 공개합니다. Circuit은 비공개 $cm$이 Note Tree에 존재하고 소비자가 올바른 소유자인지 함께 검증합니다.

### 1.4 Voucher는 무엇인가요?

Voucher는 Transfer가 생성하는 임시 private 객체입니다. Receiver가 Proceed하거나 Sender가 Recall하면 Voucher의 생애주기가 끝납니다. senderAddress와 receiverAddress는 모두 ZK owner address이며 EVM account가 아닙니다. Voucher opening은 Sender와 Receiver가 공유하므로 두 사람 모두 같은 rvnf를 계산할 수 있습니다. opening을 안전하게 전달하는 방식은 현재 POC에서 구현하지 않았습니다.

```text
Voucher
  DocumentHash
  AssetRole
  q_mass
  a_rec
  e
  senderAddress
  receiverAddress
  deadlineEpoch
  opening
```

$$
rv=H(VoucherTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,senderAddress,receiverAddress,deadlineEpoch,opening)
$$

$$
rvnf=H(VoucherNullifierTag,opening,rv)
$$

Transfer가 $rv$를 공개해 Voucher Tree에 추가합니다. Proceed·Recall은 소비하는 $rv$를 공개하지 않고 같은 $rvnf$만 공개합니다.

### 1.5 DPP와 Claim은 무엇인가요?

Exit는 Note의 DocumentHash·AssetRole·Sustainability State와 같은 값을 사용해 DPPPrivateData를 만듭니다. Note의 owner address와 opening은 옮기지 않고 새로운 dppOpening을 사용합니다. ELIGIBLE Note와 WASTE Note 모두 Exit할 수 있습니다.

```text
DPPPrivateData
  DocumentHash
  AssetRole
  q_mass
  a_rec
  e
  dppOpening
```

$$
dppCommitment=H(DPPTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,dppOpening)
$$

dppCommitment에는 owner address·Note opening·nf가 들어가지 않습니다. DPP는 소비되지 않으므로 DPP Tree와 nullifier가 없습니다.

dppOpening은 commitment를 숨기고 무작위화하는 값이며 소유권 credential이 아닙니다. Issue는 DPPPrivateData와 dppOpening을 아는 사람이 proof를 만들 수 있습니다. 현재 POC는 그 사람이 현재 DPP owner인지 검증하지 않습니다.

Claim은 별도 Hash나 Claim ID가 아닙니다. Contract가 다음 두 값을 mapping key로 함께 사용해 식별하는 등록 관계입니다.

$$
ClaimKey=(dppCommitment,issuePolicyRef)
$$

위 ClaimKey는 설명을 위한 표기이며 별도 값으로 Hash하거나 저장하지 않습니다. 각 Issue Policy version에는 서로 다른 issuePolicyRef가 있습니다. Contract는 dppCommitment와 issuePolicyRef를 claimRecordOf의 두 mapping key로 사용합니다. 따라서 같은 두 key 조합은 두 번 등록할 수 없습니다. V1과 V2는 issuePolicyRef가 다르므로 각각 한 번씩 등록하고 상태도 따로 관리할 수 있습니다.

## 2. 누가 무엇을 담당하나요?

주체는 **시스템 설정**, **공급망 Operation 실행**, **상태 관리와 감사**의 세 단계에서 등장합니다.

### 2.1 시스템을 누가 설정하나요?

| 주체 | 하는 일 | Contract가 확인하는 것 |
|---|---|---|
| System Admin | Contract를 배포하고 EntryIssuer와 Policy Authority를 등록합니다. | 배포 시 admin에 msg.sender를 저장합니다. 이후 관리 함수는 호출자가 이 admin인지 확인합니다. |
| Policy Authority | Process·Issue Policy를 예약하고 verifier를 등록합니다. Process에 사용할 Grant도 관리합니다. | 호출자의 EVM account가 해당 authorityId에 등록됐는지 확인합니다. |
| Status Authority | 배포될 때 한 account로 고정됩니다. 이후 Note·Voucher·Claim 상태를 변경합니다. | 호출자가 constructor에 저장된 EVM account인지 확인합니다. |

System Admin 권한만으로는 Policy 등록이나 Status 변경을 실행할 수 없습니다. 같은 EVM account가 여러 역할을 맡으려면, 해당 account가 각 역할로 별도로 등록되거나 배포 시 지정되어야 합니다.

### 2.2 공급망 Operation은 누가 실행하나요?

**Participant는 private 객체를 보관하고 proof를 만드는 주체입니다.** Factory는 별도 Contract 역할이 아니라, Process를 실행하는 Participant를 설명하기 위한 업무상 이름입니다.

Entry에서는 두 종류의 확인이 순서대로 일어납니다.

1. 등록된 EntryIssuer EVM account가 entry 함수를 호출해야 합니다. 등록되지 않은 account의 호출은 Contract가 거부합니다.
2. Circuit은 $sk_{\mathrm{owner}}$에서 address를 계산하고, 새 Note에 기록된 owner와 같은지 확인합니다.

EntryIssuer 권한은 **최초 Note를 원장에 추가할 수 있는 권한**입니다. Note를 소유한다는 뜻은 아닙니다. 현재 POC는 EntryIssuer의 EVM account와 Note owner의 ZK address가 같은 사람인지 검증하지 않습니다.

Note나 Voucher를 소비하는 Operation은 다음 순서로 실행됩니다.

| 주체 | 하는 일 |
|---|---|
| Participant | Note·Voucher·DPP의 private data와 secret으로 proof와 공개값을 준비합니다. |
| Circuit | 입력 객체의 membership·소유권·Sustainability State 관계와 output 계산을 검증합니다. |
| ZkDPPClaimLedger | Contract가 수용한 root·미소비·Active·Policy 권한·proof를 확인한 뒤 온체인 상태를 변경합니다. |

현재 POC에서 Note 소유권은 Circuit이 $sk_{\mathrm{owner}}$로 검증합니다. Contract는 공급망 Operation을 실행할 때 transaction의 msg.sender와 Note owner가 같은지 비교하지 않습니다.

Issue는 Note·Voucher 소비 Operation과 다릅니다. Issue proof 생성자는 DPPPrivateData를 알고 있음을 증명하고, Circuit은 그 data가 dppCommitment와 Policy 조건에 맞는지 확인합니다. Issue는 Merkle membership·owner secret·nullifier를 검사하지 않습니다.

### 2.3 상태 관리와 감사는 누가 하나요?

| 주체 | 하는 일 | 현재 POC의 신뢰 경계 |
|---|---|---|
| Status Authority | 감사로 찾은 nf·rvnf 또는 Claim을 Frozen·Active·Revoked로 변경합니다. | 올바른 감사 대상을 선택했다고 가정합니다. |
| 위원 1·2·3 | 각자 가진 key share로 AuditRecord의 partial decryption을 계산합니다. | 서로 다른 두 위원이 정직하게 응답한다고 가정합니다. |
| Auditor | 두 응답을 결합하고 생성·소비 기록을 따라 backward·forward tracing을 수행합니다. | 온체인 원본을 확인하고 올바르게 탐색한다고 가정합니다. |

위원회와 Auditor는 현재 Contract에 account로 등록되지 않습니다. 이들은 온체인 권한 주체가 아니라 off-chain 감사 주체입니다.

## 3. 온체인에는 어떤 상태가 저장되나요?

### 3.1 Main Contract 배포 상태

최종 constructor는 다음 값을 받습니다.

```text
constructor(
  address[7] fixedEventVerifiers,
  address fieldHasher,
  address statusAuthority
)
```

고정 verifier 순서는 Entry, Transfer, Proceed, Recall, Merge, Split, Exit입니다. Process·Issue verifier는 constructor에 넣지 않고 PolicyRecord로 등록합니다.

Constructor는 fieldHasher와 verifier 주소에 Contract bytecode가 존재하는지만 확인합니다. 이 주소들이 M9에서 생성한 올바른 Poseidon2 hasher와 verifier인지 code hash로 대조하지는 않습니다. 배포자가 올바른 주소를 넣는다는 것이 현재 POC의 배포 신뢰 경계입니다.

```text
immutable admin
immutable statusAuthority
immutable fieldHasher
immutable fixed Operation verifier 7개
```

배포 뒤 운영 권한은 다음처럼 저장합니다.

| Solidity 상태 | 형태 | 목적 |
|---|---|---|
| entryIssuers | address → bool | entry 함수를 호출했을 때 Contract가 허용할 EVM account입니다. |
| authorityIdOf | address → uint64 | Policy Authority account를 중복 없는 ID에 연결합니다. |
| policyAuthorities | authorityId → account·enabled | 등록된 Policy Authority를 조회합니다. |
| nextAuthorityId | uint64, 초깃값 1 | 다음 Authority ID를 발급합니다. |

admin 전용 함수는 EntryIssuer와 Policy Authority를 등록합니다. Status 변경 함수는 statusAuthority만 호출할 수 있습니다. Contract는 두 권한을 별도로 확인합니다.

### 3.2 Note·Voucher Tree는 어떤 형태인가요?

Note Tree와 Voucher Tree는 서로 분리된 depth-32 append-only Merkle Tree입니다.

```text
Tree
  acceptedRoots[root] → bool
  nodes[nodeIndex]     → Field
  zeroes[level]        → Field
  leafCount            → uint256
  currentRoot          → Field
```

| 필드 | 목적 |
|---|---|
| nodes | leaf와 갱신한 중간 node를 직접 저장합니다. |
| zeroes | 아직 값이 없는 subtree의 level별 기본 Hash입니다. |
| currentRoot | 가장 최근 root입니다. |
| acceptedRoots | 정상 append로 만들어진 현재·과거 root를 허용합니다. |
| leafCount | 다음 append index입니다. |

getNotePath·getVoucherPath는 현재 Tree 기준 path만 반환합니다. 과거 root별 path snapshot은 저장하지 않습니다.

### 3.3 생성·소비 상태는 무엇인가요?

Solidity의 ObjectRef는 객체 종류를 objectType에, 객체 식별값을 rawId에 저장합니다. objectType 값은 NOTE=1, VOUCHER=2, DPP=3입니다. NOTE의 rawId는 cm, VOUCHER의 rawId는 rv, DPP의 rawId는 dppCommitment입니다. 아래 표에서는 의미를 드러내기 위해 rawId를 objectId라고 적습니다.

| Solidity 형태 | 저장하는 값 | 답하는 질문 |
|---|---|---|
| commitments[cm] → bool | 등록된 Note commitment | 같은 cm이 이미 생성됐나요? |
| voucherCommitments[rv] → bool | 등록된 Voucher commitment | 같은 rv가 이미 생성됐나요? |
| producerOf[objectType][objectId] → audit ID | 객체 생성 기록 | 이 cm·rv·dppCommitment는 어느 Operation에서 생성됐나요? |
| noteSpentIn[nf] → audit ID | Note 소비 기록 | 이 nf는 어느 Operation에서 소비됐나요? |
| voucherSpentIn[rvnf] → audit ID | Voucher 해결 기록 | 이 rvnf를 소비한 AuditRecord는 무엇인가요? AuditRecord의 eventKind를 읽으면 Proceed와 Recall 중 어느 Operation이었는지 알 수 있습니다. |

spentIn 값 0은 소비 기록이 없다는 뜻입니다. 별도 spent boolean을 중복 저장하지 않습니다. noteNullifiers·voucherNullifiers view는 spentIn이 0이 아닌지를 bool로 반환합니다.

AuditRecord ID는 nextAuditRecordId에서 1부터 증가합니다. ID 0을 “기록 없음”으로 남겨 두기 때문에 producerOf·spentIn·claimRecordOf가 별도 exists boolean 없이 동작합니다.

### 3.4 Status는 어떻게 저장하나요?

```text
noteStatusByNf[nf]                 → uint8
voucherStatusByNf[rvnf]            → uint8
claimStatus[dppCommitment][policy] → uint8
```

| 값 | 상태 |
|---:|---|
| 0 | Active |
| 1 | Frozen |
| 2 | Revoked |

mapping에 값이 없으면 기본값 0이므로 Active입니다. Claim은 먼저 claimRecordOf가 0이 아닌지 확인한 뒤 status를 해석해야 합니다.

Note·Voucher 상태 mapping도 nf·rvnf가 실제 output에서 파생됐는지 스스로 증명하지는 않습니다. Status Authority가 감사로 복원한 올바른 소비값만 제출한다는 것이 현재 POC의 신뢰 경계입니다. Contract는 제출된 값이 아직 소비되지 않았는지와 상태 전이만 검사합니다.

### 3.5 Policy Registry는 어떤 값을 저장하나요?

Policy는 다음 순서로 준비합니다.

1. System Admin이 Policy Authority EVM account를 등록하고 authorityId를 발급합니다.
2. 새 Policy family를 만들 때 Authority가 EventKind와 새 policyId를 정하고 version 1을 예약합니다.
3. 기존 family를 개정할 때는 기존 policyId와 다음 version을 예약합니다.
4. 두 예약 모두 eventKind·authorityId·policyId·version에서 policyRef를 계산합니다.
5. Policy Authority가 예약된 policyRef에 vkHash와 verifierRef를 등록합니다.
6. Process Policy라면 Factory가 자기 sk_owner와 policyRef로 policyScopeRef를 계산해 Authority에 전달합니다.
7. Policy Authority가 그 policyScopeRef에 Process Grant를 설정합니다. Issue Policy는 Grant를 사용하지 않습니다.

```text
PolicyRecord
  authorityId   uint64
  policyId      uint64
  version       uint64
  eventKind     uint8
  inputArity    uint8
  outputArity   uint8
  vkHash        bytes32
  verifierRef   address
  enabled       bool
```

주요 mapping은 다음입니다.

| 상태 | 역할 |
|---|---|
| authorityIdOf[account] | Policy Authority EVM account를 ID로 바꿉니다. |
| nextPolicyId[authorityId] | 해당 Authority가 다음에 예약할 Policy family ID입니다. |
| nextPolicyVersion[authorityId][policyId] | 같은 family에서 다음에 예약할 version 번호입니다. |
| policyFamilies[authorityId][policyId] | 같은 Policy family의 EventKind를 고정합니다. |
| policyReservations[policyRef] | 등록 전에 예약한 family·version입니다. |
| policyRecords[policyRef] | 고정된 identity·arity·vkHash·verifierRef와 현재 enabled 상태를 저장합니다. |
| policyGrants[policyRef][policyScopeRef] | 특정 private owner scope의 Process 사용 권한입니다. |

PolicyRecord의 identity·arity·vkHash·verifierRef는 수정하지 않습니다. enabled만 true에서 false로 바뀌며 재활성화하지 않습니다. Grant는 Process에만 사용하며 revoke·regrant가 가능합니다.

Contract는 vkHash와 verifierRef가 실제로 같은 VK를 나타내는지 다시 계산하지 않습니다. Policy Authority가 올바른 두 값을 등록한다는 것이 현재 POC의 신뢰 경계입니다.

### 3.6 AuditRecord는 무엇을 저장하나요?

```text
ObjectRef
  objectType uint8
  rawId      uint256

AuditRecord
  eventKind
  policyRef
  outputRefs[]
  r1X
  r1Y
  encryptedParents[]
  encryptedOutputNfs[]
```

| 필드 | 목적 |
|---|---|
| eventKind | 암호문의 부모·출력 순서를 해석합니다. |
| policyRef | Process·Issue에 사용한 Policy를 식별합니다. |
| outputRefs | 생성한 Note·Voucher·DPP와 순서를 기록합니다. |
| $R_1$ | 위원들이 partial decryption을 계산할 때 사용하는 공개점입니다. 두 응답을 결합하면 private shared point $Z$를 얻고, $Z$에서 private Field masking key $K$를 파생합니다. |
| encryptedParents | 실제 소비 cm·rv를 숨겨 저장합니다. |
| encryptedOutputNfs | 새 cm·rv가 미래에 사용할 nf·rvnf를 숨겨 저장합니다. |

AuditRecord는 transaction의 public input 전체를 복사하지 않습니다. noteRoot·proof·policyScopeRef는 원본 transaction에서 다시 읽습니다. 입력 nf·rvnf는 원본 transaction에도 있고, 성공 후 noteSpentIn·voucherSpentIn의 key로도 남습니다. Process와 Issue의 policyRef처럼 감사 기록을 해석하는 데 필요한 값은 AuditRecord에도 저장합니다.

outputRefs·eventKind·policyRef는 감사 프로그램이 Record의 종류와 다음 탐색 대상을 바로 해석할 수 있도록 저장합니다. root·proof처럼 원본 transaction에서 확인하면 되는 값은 AuditRecord에 다시 저장하지 않습니다.

Issue는 공개 DPP·Policy 관계만 기록하므로 암호화를 사용하지 않습니다. Issue AuditRecord의 $R_{1X}$와 $R_{1Y}$는 기본값 0이고, 두 암호문 배열은 비어 있습니다.

transaction이 받는 AuditCipher와 저장되는 AuditRecord의 차이는 다음과 같습니다.

| 값 | AuditCipher calldata | AuditRecord storage |
|---|---:|---:|
| $R_{1X},R_{1Y}$ | 포함 | 포함 |
| encryptedParents | 포함 | 포함 |
| encryptedOutputNfs | 포함 | 포함 |
| eventKind | 함수 종류로 결정 | 저장 |
| policyRef | Process·Issue 함수 인자 | 필요한 Operation에 저장 |
| outputRefs | 새 commitment 함수 인자에서 결정 | 저장 |

Participant가 eventKind나 outputRefs를 임의의 AuditCipher 필드로 보내는 구조가 아닙니다. Contract가 호출된 함수와 검증된 output을 기준으로 두 값을 구성합니다.

### 3.7 Claim은 어떻게 저장하나요?

```text
claimRecordOf[dppCommitment][issuePolicyRef]
  → Issue AuditRecord ID
```

값이 0이면 Claim이 없습니다. 별도 Claim ID·nonce·Tree·registered boolean을 만들지 않습니다.

### 3.8 Event log는 무엇을 알려주나요?

Event log는 storage를 대체하지 않습니다. off-chain 감사 프로그램이 관련 transaction과 AuditRecord ID를 빠르게 찾게 하는 색인입니다.

| Event log | 알려주는 내용 |
|---|---|
| NoteAppended | 새 cm, Note index, 갱신된 Note root |
| VoucherAppended | 새 rv, Voucher index, 갱신된 Voucher root |
| AuditRecorded | 새 AuditRecord ID |
| DPPFinalized | dppCommitment와 Exit AuditRecord ID |
| ClaimIssued | dppCommitment·issuePolicyRef·Issue AuditRecord ID |
| NullifierStatusChanged | Note·Voucher의 nf·rvnf 상태 전이 |
| ClaimStatusChanged | 특정 DPP·Policy Claim의 상태 전이 |
| PolicyReserved·PolicyRegistered·PolicyDisabled | Policy lifecycle |
| PolicyGrantUpdated | Process scope의 Grant 변경 |
| EntryIssuerUpdated | EntryIssuer 허용·해제 |
| PolicyAuthorityRegistered | 새 Policy Authority와 authorityId |
| NoteExited | Exit가 소비한 Note nf |

Transfer·Merge·Process마다 별도 이름의 lifecycle Event log를 추가하지 않았습니다. NoteAppended·VoucherAppended는 새 output을 알려줍니다. spentIn mapping은 소비 transaction의 AuditRecord ID를 알려줍니다. 실제 private 부모·자손 연결은 해당 AuditRecord의 암호문을 복호화해 얻습니다.

### 3.9 공급망 Operation 외에는 어떤 함수를 사용하나요?

관리 함수는 권한과 Registry를 변경합니다.

| 함수 | 호출할 수 있는 주체 | 변경하는 상태 |
|---|---|---|
| setEntryIssuer | System Admin | allowed=true로 EntryIssuer를 허용하고 false로 해제 |
| registerPolicyAuthority | System Admin | authorityIdOf·policyAuthorities·ID counter |
| reservePolicy·reservePolicyVersion | 등록된 Policy Authority | family·version reservation |
| registerPolicy | reservation을 소유한 Policy Authority | PolicyRecord의 identity·arity·vkHash·verifierRef |
| setPolicyGrant | 해당 Process Policy의 Authority | policyGrants |
| disablePolicy | 해당 Policy의 Authority | enabled를 false로 변경 |
| setStatus | Status Authority | Note·Voucher status mapping |
| setClaimStatus | Status Authority | Claim status mapping |

조회 함수는 상태를 변경하지 않습니다.

Policy를 disable하면 이후의 새 Process·Issue만 거부합니다. 이미 성공한 Process output과 기존 Claim은 그대로 유지됩니다.

| 조회 목적 | 함수 |
|---|---|
| 현재 Tree 상태 | currentNoteRoot·currentVoucherRoot·leafCount·leaf·getPath |
| root 수용 여부 | acceptedNoteRoot·acceptedVoucherRoot |
| 소비 여부와 기록 | noteNullifiers·voucherNullifiers·noteSpentIn·voucherSpentIn |
| 생성 기록과 감사 저장값 | producerOf·getAuditRecord; getAuditRecord는 암호문을 반환하며 평문은 위원 응답으로 복원 |
| Policy | computePolicyRef·policyRecords·policyGrants |
| Claim | verifyClaim·claimRecordOf·claimStatus |

## 4. 어떤 Circuit과 verifier를 사용하나요?

Circuit은 private witness와 public input 사이의 관계를 증명합니다. Contract는 이 proof를 검증한 뒤 온체인 상태를 변경합니다. **Circuit은 Contract storage를 직접 읽지 않습니다.**

| Circuit | 증명하는 객체 전이 | Public input 수 | Verifier 선택 | 실제 구현 |
|---|---|---:|---|---|
| Audit Entry | $\varnothing\rightarrow Note$ | 4 | constructor 고정 | features/audit_entry |
| Audit Transfer | $Note\rightarrow Voucher+Note$ | 11 | constructor 고정 | features/audit_transfer |
| Audit Proceed | $Voucher\rightarrow Note$ | 7 | constructor 고정 | features/audit_proceed |
| Audit Recall | $Voucher\rightarrow Note$ | 8 | constructor 고정 | features/audit_recall |
| Audit Merge | $Note+Note\rightarrow Note$ | 9 | constructor 고정 | features/audit_merge |
| Audit Split | $Note\rightarrow Note+Note$ | 9 | constructor 고정 | features/audit_split |
| Audit Process 3-to-2 | $Note^3\rightarrow ELIGIBLE+WASTE$ | 15 | PolicyRecord | features/audit_process_3_2 |
| Exit DPP | $Note\rightarrow DPP$ | 6 | constructor 고정 | features/m8_exit_dpp |
| Issue Standard V1 | DPP private data가 Standard 조건을 만족함을 증명 | 2 | PolicyRecord | features/issue_claim |
| Issue Strict V2 | DPP private data가 Strict 조건을 만족함을 증명 | 2 | PolicyRecord | features/issue_claim |

고정 Operation verifier 7개는 Contract 배포 시 정해집니다. Process와 Issue verifier는 Policy Authority가 PolicyRecord에 등록하며, 실행할 때 policyRef로 선택합니다.

### 4.1 의사 코드의 공통 함수는 무엇을 뜻하나요?

뒤의 Operation 의사 코드는 다음 공통 기능을 이름으로 호출합니다. 이 이름은 Solidity 함수가 아니라 Circuit 관계를 짧게 표현한 것입니다.

| 의사 코드 이름 | 입력 | 검증하거나 계산하는 내용 |
|---|---|---|
| AssertStateAndRole | Note 또는 output Sustainability State | uint64 범위, $a_{\mathrm{rec}}\le q_{\mathrm{mass}}$, AssetRole과 WASTE 규칙을 확인합니다. |
| CommitNote | Note 전체 | Note field를 정해진 순서와 Domain으로 Hash하여 cm을 계산합니다. |
| CommitVoucher | Voucher 전체 | Voucher field를 정해진 순서와 Domain으로 Hash하여 rv를 계산합니다. |
| CommitDPP | DPPPrivateData | DPP field를 정해진 순서와 Domain으로 Hash하여 dppCommitment를 계산합니다. |
| NoteNullifier | sk_owner·cm | 해당 Note가 소비될 때 공개할 nf를 계산합니다. |
| VoucherNullifier | Voucher opening·rv | 해당 Voucher가 소비될 때 공개할 rvnf를 계산합니다. |
| AssertNoteSpend | Note·sk_owner·root·index·path·public nf | cm·owner address·Merkle membership·nf가 하나의 Note와 일치하는지 확인합니다. |
| AssertVoucherMembership | Voucher·root·index·path | rv를 다시 계산하고 Voucher Tree membership을 확인합니다. Sender·Receiver 권한은 호출 Operation이 별도로 확인합니다. |
| AssertEncrypted | Audit Context $L$·평문 배열·AuditCipher·private randomness | 실제 부모와 output 소비값을 같은 private Field masking key $K$로 암호화한 결과가 public AuditCipher와 같은지 확인합니다. |
| AssertMassProportionalAllocation | input·output State·remainder | Transfer의 $a_{\mathrm{rec}}$·$e$가 질량 비례 floor와 residual 규칙을 따르는지 확인합니다. |
| AssertOutput2FloorAndOutput1Residual | input·두 output State·remainder | Split output 2의 floor 값과 output 1의 residual을 확인합니다. |
| ApplyLossAndCarbon·ApplyAllocation | Process input 합·private delta·remainder | Circuit에 고정된 Process 비율로 intermediate와 ELIGIBLE·WASTE State를 계산합니다. |

AuditCipher는 $R_{1X}$, $R_{1Y}$, encryptedParents와 encryptedOutputNfs를 묶은 calldata입니다. 현재 구현의 Audit Context $L$은 EventKind와 해당 Operation의 감사 필드를 제외한 public input을 Hash한 값입니다. 따라서 다른 Operation이나 다른 public input을 위해 만든 암호문을 그대로 재사용할 수 없습니다. 암호화 수식과 복호화 과정은 16장에서 설명합니다.

각 Operation 절은 다음 순서로 읽습니다.

1. 무엇을 구현하려 했고 실제로 무엇을 구현했는지 확인합니다.
2. 실행 전 어떤 상태가 필요한지 확인합니다.
3. calldata·Circuit public input·private witness·기존 storage를 구분합니다.
4. Circuit이 검증하는 관계를 읽습니다.
5. Contract가 확인하는 현재 상태를 읽습니다.
6. 성공 후 storage와 Event log가 어떻게 바뀌는지 확인합니다.
7. 실패하면 어떤 상태도 남지 않는지 확인합니다.

## 5. 모든 Operation은 어떤 공통 순서로 실행되나요?

```text
1. proof 생성자는 해당 Operation에 필요한 private data를 준비합니다.
   입력 객체를 소비하는 Operation이라면 secret과 Merkle path도 준비합니다.

2. proof 생성자는 output commitment·nullifier·감사 암호문 중
   해당 Operation에 필요한 공개값을 계산합니다.

3. proof 생성자는 private witness와 public input으로 PLONK proof를 만듭니다.

4. EVM transaction은 proof와 public input에 대응하는 calldata를 Contract에 전달합니다.

5. Contract는 해당 Operation에 필요한 권한·root·소비 여부·Status·Policy를 확인합니다.

6. 고정 Operation verifier 또는 PolicyRecord의 verifier가 proof를 검증합니다.

7. proof가 성공하면 Contract는 nextAuditRecordId에서 새 ID를 발급합니다.
   spentIn·producerOf·AuditRecord는 같은 ID로 연결합니다.
   Note·Voucher Tree에는 AuditRecord ID가 아니라 새 commitment를 append합니다.

8. 어느 단계든 실패하면 transaction 전체가 되돌아갑니다.
```

위 순서는 공통 골격이며, 모든 Operation이 모든 단계를 사용하는 것은 아닙니다.

| Operation | 적용하지 않는 단계 |
|---|---|
| Entry | input 객체 소비·Merkle membership·spentIn 변경이 없습니다. |
| Issue | input 객체 소비·Merkle membership·감사 암호화·Tree 변경이 없습니다. |
| Exit | Note를 소비하지만 새 Note·Voucher를 만들지 않으므로 Tree append가 없습니다. |

Contract는 proof를 검증한 뒤에만 storage를 변경합니다. 두 output 중 하나가 중복이거나 Tree append가 실패해도 첫 output만 남지 않습니다.

### 공개된 값은 모두 storage에 복사되나요?

아닙니다. **transaction에서 공개되는 값과 Contract storage에 남는 값은 서로 다릅니다.**

| 위치 | 무엇이 들어가나요? | 나중에 어떻게 확인하나요? |
|---|---|---|
| transaction calldata | Operation에 필요한 proof, root, nf·rvnf, 새 commitment, PolicyRef, 감사 암호문 | 원본 transaction을 다시 읽습니다. |
| Event log | 새 leaf·root·AuditRecord ID·상태 변경의 색인 | receipt와 topic으로 찾습니다. |
| Contract storage | 현재 Tree, 생성·소비 관계, 필요한 감사 암호문, Policy·Claim·Status | view 함수와 mapping으로 조회합니다. |

예를 들어 Process의 public input은 15개이지만, AuditRecord가 이 15개를 그대로 복사하지는 않습니다. root와 policyScopeRef는 원본 transaction과 관련 Contract mapping에서 확인합니다. nf 3개는 원본 transaction에 공개되고 noteSpentIn의 key로도 남습니다. AuditRecord에는 **공개 transaction만으로 알 수 없는 부모 cm 3개와 output nf 2개의 암호문**, outputRefs, eventKind와 policyRef를 저장합니다.

## 6. Operation: Entry

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | 등록된 EntryIssuer가 최초 Note 등록을 요청하고, Circuit이 그 Note의 owner secret 관계를 검증합니다. |
| 실제 구현 | 등록된 EntryIssuer account의 호출만 허용합니다. Circuit은 별도로 $sk_{\mathrm{owner}}$에서 계산한 ZK owner address와 출력 nf 암호화를 확인합니다. |
| 객체 전이 | $\varnothing\rightarrow Note$ |
| proof 생성자 | 새 Note의 private data와 sk_owner를 가진 Participant |
| EVM 호출 권한 | entryIssuers[msg.sender]가 true인 account |
| verifier | constructor에 고정된 Audit Entry verifier |

### 실행 전에 무엇이 필요하나요?

Entry를 호출하는 EVM account가 entryIssuers에 등록되어 있어야 합니다. 새 cm은 commitments에 아직 등록되지 않은 값이어야 합니다. 최초 Note이므로 Note root나 Merkle path는 필요하지 않습니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** cm → R1X → R1Y → encryptedOutputNf

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| cm | ✓ | — | entry calldata로 받고 commitments에서 중복 확인 |
| Note 전체·sk_owner·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·encryptedOutputNf | ✓ | — | AuditCipher calldata로 받음 |
| msg.sender | — | — | transaction sender이며 entryIssuers에서 권한 확인 |

### Circuit은 무엇을 검증하나요?

```text
EntryCircuit:
    # 핵심: Note owner secret으로 만든 유효한 Note와 미래 nf를 함께 증명합니다.
    # 1. Note 형식을 확인합니다.
    AssertStateAndRole(Note)
    require Note.q_mass > 0
    require Note.AssetRole == ELIGIBLE

    # 2. secret과 Note owner를 연결합니다.
    address = H(OwnerTag, sk_owner)
    require Note.address == address

    # 3. 공개 commitment를 확인합니다.
    cm = CommitNote(Note)
    require public.cm == cm

    # 4. 이 Note가 소비될 때 사용할 nf를 계산합니다.
    nf_out = H(NullifierTag, sk_owner, cm)

    # 5. 실제 nf_out이 감사 암호문에 들어 있는지 확인합니다.
    AssertEncrypted(context, [nf_out], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

```text
entry(proof, cm, audit):
    # 1. 호출 권한과 중복을 확인합니다.
    require entryIssuers[msg.sender]
    require commitments[cm] == false

    # 2. Entry verifier로 proof를 확인합니다.
    verify Entry public inputs

    # 3. Note와 감사 기록을 함께 저장합니다.
    commitments[cm] = true
    append cm to Note Tree
    producerOf[NOTE][cm] = auditRecordId
    store AuditRecord(outputRefs=[NoteRef(cm)])
```

### 온체인 상태 변화

| 상태 | 변화 |
|---|---|
| commitments | cm을 true로 등록 |
| Note Tree | leaf 1개 append |
| producerOf | Note 생성 AuditRecord 연결 |
| AuditRecord | outputRefs=[Note], encryptedOutputNfs=[output nf]로 1개 추가 |
| Event log | NoteAppended·AuditRecorded |

EntryIssuer의 EVM account와 Note owner의 ZK address는 서로 다른 검사 대상입니다. 등록되지 않은 caller, 중복 cm, 잘못된 owner·Sustainability State·commitment·암호문은 실패합니다. 실패하면 Note Tree와 AuditRecord가 모두 그대로 유지됩니다.

## 7. Operation: Transfer

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | Sender Note의 일부 또는 전부를 Receiver가 받을 Voucher로 바꿉니다. |
| 실제 구현 | Voucher와 Sender Change Note를 항상 함께 만들며 전량 Transfer도 zero-State Change Note를 append합니다. |
| 객체 전이 | $Note\rightarrow Voucher+Change\ Note$ |
| proof 생성자 | input Note의 sk_owner를 가진 Sender |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Audit Transfer verifier |

Sender가 $q_{\mathrm{voucher}}$를 선택합니다. $a_{\mathrm{rec}}$과 $e$는 질량 비례로 나누며 Change를 floor 계산하고 Voucher가 residual을 받습니다.

$$
0<q_{\mathrm{voucher}}\le q_{\mathrm{input}},\qquad
q_{\mathrm{change}}=q_{\mathrm{input}}-q_{\mathrm{voucher}}
$$

$x\in\{a_{\mathrm{rec}},e\}$에 대해 다음 관계를 사용합니다.

$$
x_{\mathrm{change}}=
\left\lfloor\frac{x_{\mathrm{input}}q_{\mathrm{change}}}{q_{\mathrm{input}}}\right\rfloor,
\qquad
x_{\mathrm{voucher}}=x_{\mathrm{input}}-x_{\mathrm{change}}
$$

ELIGIBLE Note와 WASTE Note 모두 Transfer할 수 있습니다. 두 output은 input의 AssetRole을 그대로 유지합니다. deltaEpoch=0도 허용합니다. 이 경우 deadlineEpoch=transferEpoch이므로 Recall 조건은 처음부터 만족할 수 없지만 Proceed는 가능합니다.

### 실행 전에 무엇이 필요하나요?

Contract가 noteRoot를 acceptedNoteRoot로 수용해야 합니다. Circuit은 private input cm이 이 noteRoot에 포함됐음을 증명해야 합니다. input nf는 미소비·Active여야 하고, 새 rvNew과 cmChange는 아직 등록되지 않아야 합니다. public transferEpoch은 transaction이 실행되는 block의 epoch와 같아야 합니다.

### 값은 어디에서 사용하나요?

```text
Circuit public input 순서:
  noteRoot, nf, rvNew, cmChange,
  transferEpoch, deltaEpoch,
  R1X, R1Y,
  encryptedParentCM,
  encryptedVoucherRvnf,
  encryptedChangeNf

Circuit private witness:
  input Note·cm·index·path·sender secret
  receiverAddress
  Voucher·Change Note·openings
  allocation remainder·deadlineEpoch·audit randomness

Contract calldata:
  proof, noteRoot, nf, rvNew, cmChange,
  transferEpoch, deltaEpoch, AuditCipher
```

Contract는 acceptedNoteRoot·noteSpentIn·noteStatusByNf·commitments·voucherCommitments와 현재 block epoch를 읽습니다. private input cm·Sustainability State·path는 Contract가 읽지 않습니다.

### Circuit은 무엇을 검증하나요?

```text
TransferCircuit:
    # 핵심: 하나의 Note를 보존 법칙에 맞는 Voucher와 Change Note로 나눕니다.
    # 1. private input Note의 membership·owner·nf를 확인합니다.
    senderAddress = H(OwnerTag, sk_sender)
    cm_in = AssertNoteSpend(Note, sk_sender, noteRoot, index, path, public.nf)

    # 2. 질량과 State 배분을 확인합니다.
    require Voucher.q_mass > 0
    require q_input == q_voucher + q_change
    AssertMassProportionalAllocation(a_rec, e, remainder)

    # 3. owner와 output 종류를 확인합니다.
    require Change.address == senderAddress
    require Voucher.senderAddress == senderAddress
    require Voucher.DocumentHash·AssetRole == input Note
    require Change.DocumentHash·AssetRole == input Note

    # 4. deadline과 공개 output을 확인합니다.
    require deadlineEpoch == transferEpoch + deltaEpoch
    require public.rvNew == CommitVoucher(Voucher)
    require public.cmChange == CommitNote(Change)

    # 5. 부모와 output 소비값을 암호화했는지 확인합니다.
    rvnf_out = VoucherNullifier(Voucher.opening, rvNew)
    nf_change = NoteNullifier(sk_sender, cmChange)
    AssertEncrypted(context, [cm_in, rvnf_out, nf_change], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

```text
transfer(proof, root, nf, rvNew, cmChange, transferEpoch, deltaEpoch, audit):
    # 1. 현재 사용할 수 있는 Note인지 확인합니다.
    require acceptedNoteRoot(root)
    require noteSpentIn[nf] == 0
    require noteStatusByNf[nf] == Active
    require transferEpoch == currentEpoch()

    # 2. 두 output이 모두 새 값인지 확인하고 proof를 검증합니다.
    require voucherCommitments[rvNew] == false
    require commitments[cmChange] == false
    verify Transfer public inputs

    # 3. 소비·두 output·감사를 원자적으로 기록합니다.
    noteSpentIn[nf] = auditRecordId
    append rvNew to Voucher Tree
    append cmChange to Note Tree
    producerOf[VOUCHER][rvNew] = auditRecordId
    producerOf[NOTE][cmChange] = auditRecordId
    store AuditRecord(outputRefs=[VoucherRef(rvNew), NoteRef(cmChange)])
```

### 온체인 상태 변화

| 상태 | 변화 |
|---|---|
| noteSpentIn | input nf에 AuditRecord ID 기록 |
| voucherCommitments | rvNew을 true로 등록 |
| commitments | cmChange를 true로 등록 |
| Voucher Tree | rvNew 1개 append |
| Note Tree | cmChange 1개 append |
| producerOf | Voucher·Change 생성 기록 2개 연결 |
| AuditRecord | 부모 cm, Voucher rvnf, Change nf의 암호문 저장 |
| Event log | VoucherAppended·NoteAppended·AuditRecorded |

잘못된 owner·membership·Sustainability State 배분·deadline·output·암호문은 proof를 실패시킵니다. 소비되거나 Frozen인 nf, acceptedNoteRoot가 아닌 root, 실행 block과 다른 transferEpoch, 중복 output은 Contract가 거부합니다. 실패하면 Note 소비와 두 Tree append가 모두 취소됩니다.

## 8. Operation: Proceed

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | Receiver가 Voucher를 자기 Note로 바꿉니다. |
| 실제 구현 | Circuit이 Receiver secret과 output State를 검증합니다. Proceed는 deadline 전후 모두 허용합니다. |
| 객체 전이 | $Voucher\rightarrow Receiver\ Note$ |
| proof 생성자 | Voucher의 receiver secret을 아는 Participant |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Audit Proceed verifier |

### 실행 전에 무엇이 필요하나요?

- voucherRoot가 Contract가 수용한 Voucher Tree root여야 합니다.
- rvnf가 아직 voucherSpentIn에 기록되지 않아야 합니다.
- voucherStatusByNf[rvnf]가 Active여야 합니다.
- cmReceiver가 아직 commitments에 등록되지 않은 값이어야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** voucherRoot → rvnf → cmReceiver → R1X → R1Y → encryptedParentRV → encryptedReceiverNf

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| voucherRoot | ✓ | — | proceed calldata로 받고 acceptedVoucherRoot 확인 |
| rvnf | ✓ | — | proceed calldata로 받고 voucherSpentIn·voucherStatusByNf 조회 |
| cmReceiver | ✓ | — | proceed calldata로 받고 commitments 중복 확인 |
| Voucher·rv·index·path | — | ✓ | Contract에 전달하지 않음 |
| receiver sk_owner·Receiver Note·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·감사 암호문 2개 | ✓ | — | AuditCipher calldata로 받음 |

### Circuit은 무엇을 검증하나요?

```text
ProceedCircuit:
    # 핵심: Receiver만 Voucher를 같은 State의 Note로 바꿀 수 있습니다.
    # 1. Voucher가 Voucher Tree에 있는지 확인합니다.
    rv = AssertVoucherMembership(Voucher, voucherRoot, index, path)

    # 2. Receiver secret과 Voucher의 receiverAddress를 연결합니다.
    require H(OwnerTag, sk_receiver) == Voucher.receiverAddress
    require public.rvnf == H(VoucherNullifierTag, Voucher.opening, rv)

    # 3. Voucher의 제품·Role·State를 Receiver Note에 그대로 전달합니다.
    require ReceiverNote.DocumentHash == Voucher.DocumentHash
    require ReceiverNote.AssetRole == Voucher.AssetRole
    require ReceiverNote.State == Voucher.State
    require ReceiverNote.address == Voucher.receiverAddress
    require public.cmReceiver == CommitNote(ReceiverNote)

    # 4. 실제 부모 rv와 Receiver Note의 미래 nf를 암호화했는지 확인합니다.
    nf_out = NoteNullifier(sk_receiver, cmReceiver)
    AssertEncrypted(context, [rv, nf_out], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

```text
proceed(proof, voucherRoot, rvnf, cmReceiver, audit):
    require voucherRoot is accepted
    require voucherSpentIn[rvnf] == 0
    require voucherStatusByNf[rvnf] == Active
    require commitments[cmReceiver] == false
    verify Audit Proceed proof

    voucherSpentIn[rvnf] = auditRecordId
    append cmReceiver to Note Tree
    producerOf[NOTE][cmReceiver] = auditRecordId
    store AuditRecord
```

| 성공 후 상태 | 변화 |
|---|---|
| voucherSpentIn[rvnf] | 새 AuditRecord ID 기록 |
| commitments[cmReceiver] | true |
| Note Tree | Receiver Note 1개 append |
| producerOf | Receiver Note 생성 기록 연결 |
| AuditRecord | encryptedParents=[rv], encryptedOutputNfs=[receiver nf] |
| Event log | NoteAppended·AuditRecorded |

잘못된 Receiver secret·Voucher path·output Sustainability State·암호문은 Circuit proof를 실패시킵니다. 이미 해결됐거나 Frozen인 rvnf와 중복 output은 Contract가 거부합니다. 실패하면 어떤 상태도 변경되지 않습니다.

## 9. Operation: Recall

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | Sender가 deadline 전에 Voucher를 회수합니다. |
| 실제 구현 | Circuit이 Sender secret과 $currentEpoch<deadlineEpoch$를 검증합니다. Contract는 public currentEpoch가 실행 block의 epoch와 같은지도 확인합니다. |
| 객체 전이 | $Voucher\rightarrow Sender\ Return\ Note$ |
| proof 생성자 | Voucher의 sender secret을 아는 Participant |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Audit Recall verifier |

### 실행 전에 무엇이 필요하나요?

voucherRoot가 Contract가 수용한 Voucher Tree root여야 합니다. rvnf는 아직 voucherSpentIn에 기록되지 않았고 Active여야 합니다. cmReturn은 아직 commitments에 등록되지 않은 값이어야 합니다. 추가로 Recall transaction의 currentEpoch가 Contract의 currentEpoch()와 같아야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** voucherRoot → rvnf → cmReturn → currentEpoch → R1X → R1Y → encryptedParentRV → encryptedReturnNf

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| voucherRoot | ✓ | — | recall calldata로 받고 acceptedVoucherRoot 확인 |
| rvnf | ✓ | — | recall calldata로 받고 voucherSpentIn·voucherStatusByNf 조회 |
| cmReturn | ✓ | — | recall calldata로 받고 commitments 중복 확인 |
| currentEpoch | ✓ | — | recall calldata로 받고 block.timestamp/600과 비교 |
| Voucher·rv·index·path·deadlineEpoch | — | ✓ | Contract에 전달하지 않음 |
| sender sk_owner·Return Note·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·감사 암호문 2개 | ✓ | — | AuditCipher calldata로 받음 |

### Circuit은 무엇을 검증하나요?

```text
RecallCircuit:
    # 핵심: Sender만 deadline 전에 Voucher를 반환 Note로 바꿀 수 있습니다.
    rv = AssertVoucherMembership(Voucher, voucherRoot, index, path)
    require H(OwnerTag, sk_sender) == Voucher.senderAddress
    require public.rvnf == H(VoucherNullifierTag, Voucher.opening, rv)
    require public.currentEpoch < Voucher.deadlineEpoch
    require ReturnNote.DocumentHash == Voucher.DocumentHash
    require ReturnNote.AssetRole == Voucher.AssetRole
    require ReturnNote.State == Voucher.State
    require ReturnNote.address == Voucher.senderAddress
    require public.cmReturn == CommitNote(ReturnNote)
    nf_out = NoteNullifier(sk_sender, cmReturn)
    AssertEncrypted(context, [rv, nf_out], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

```text
recall(proof, voucherRoot, rvnf, cmReturn, currentEpoch, audit):
    require voucherRoot is accepted
    require voucherSpentIn[rvnf] == 0
    require voucherStatusByNf[rvnf] == Active
    require currentEpoch == block.timestamp / 600
    require commitments[cmReturn] == false
    verify Audit Recall proof

    voucherSpentIn[rvnf] = auditRecordId
    append cmReturn to Note Tree
    producerOf[NOTE][cmReturn] = auditRecordId
    store AuditRecord
```

| 성공 후 상태 | 변화 |
|---|---|
| voucherSpentIn[rvnf] | 새 AuditRecord ID 기록 |
| commitments[cmReturn] | true |
| Note Tree | Sender Return Note 1개 append |
| producerOf | Return Note 생성 기록 연결 |
| AuditRecord | encryptedParents=[rv], encryptedOutputNfs=[return nf] |
| Event log | NoteAppended·AuditRecorded |

Proceed와 Recall은 동일한 rvnf를 사용합니다. 먼저 성공한 Operation이 voucherSpentIn[rvnf]를 기록하므로 다른 해결 경로는 실패합니다. deadline 이후에는 Recall만 실패하며 Proceed는 계속 가능합니다. deltaEpoch=0이면 deadlineEpoch와 transferEpoch이 같으므로 Recall은 처음부터 불가능합니다.

## 10. Operation: Merge

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | 같은 owner가 가진 Note 두 개를 하나로 합칩니다. |
| 실제 구현 | 두 Note는 같은 owner와 AssetRole을 가져야 합니다. 물질 compatibility 조건으로는 AssetRole만 사용하며 ProductProfile과 DocumentHash 관계는 검사하지 않습니다. |
| 객체 전이 | $Note_A+Note_B\rightarrow Note_C$ |
| proof 생성자 | 두 Note의 공통 sk_owner를 아는 Participant |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Audit Merge verifier |

ELIGIBLE 두 개 또는 WASTE 두 개를 Merge할 수 있습니다. 서로 다른 AssetRole의 조합은 허용하지 않습니다.

### 실행 전에 무엇이 필요하나요?

Contract가 noteRoot를 acceptedNoteRoot로 수용해야 합니다. Circuit은 두 private input cm이 모두 이 noteRoot에 포함됐음을 증명해야 합니다. nf1과 nf2는 서로 다르고 모두 미소비·Active여야 하며, cmOut은 아직 등록되지 않아야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** noteRoot → nf1 → nf2 → cmOut → R1X → R1Y → encryptedParentCM1 → encryptedParentCM2 → encryptedOutputNf

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| noteRoot | ✓ | — | merge calldata로 받고 acceptedNoteRoot 확인 |
| nf1·nf2 | ✓ | — | merge calldata로 받고 noteSpentIn·noteStatusByNf 조회 |
| cmOut | ✓ | — | merge calldata로 받고 commitments 중복 확인 |
| input Note·cm·index·path 2개 | — | ✓ | Contract에 전달하지 않음 |
| 공통 sk_owner·output Note·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·부모 암호문 2개·output nf 암호문 | ✓ | — | AuditCipher calldata로 받음 |

### Circuit은 무엇을 검증하나요?

```text
MergeCircuit:
    # 핵심: 서로 다른 Note 두 개의 State를 overflow 없이 합칩니다.
    cm1 = AssertNoteSpend(Note1, sk_owner, noteRoot, index1, path1, nf1)
    cm2 = AssertNoteSpend(Note2, sk_owner, noteRoot, index2, path2, nf2)
    require cm1 != cm2 and nf1 != nf2
    require Note1.AssetRole == Note2.AssetRole
    require Output.AssetRole == Note1.AssetRole
    require Output.address == H(OwnerTag, sk_owner)
    require Output.State == checkedAdd(Note1.State, Note2.State)
    require public.cmOut == CommitNote(Output)
    nf_out = NoteNullifier(sk_owner, cmOut)
    AssertEncrypted(context, [cm1, cm2, nf_out], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

Contract는 accepted noteRoot, 서로 다른 두 nf, 두 nf의 미소비·Active 상태와 새 cmOut을 확인한 뒤 proof를 검증합니다.

```text
merge(proof, noteRoot, nf1, nf2, cmOut, audit):
    require noteRoot is accepted
    require nf1 != nf2
    require nf1 and nf2 are unspent and Active
    require commitments[cmOut] == false
    verify Audit Merge proof

    noteSpentIn[nf1] = auditRecordId
    noteSpentIn[nf2] = auditRecordId
    append cmOut to Note Tree
    producerOf[NOTE][cmOut] = auditRecordId
    store AuditRecord
```

| 성공 후 상태 | 변화 |
|---|---|
| noteSpentIn[nf1·nf2] | 같은 AuditRecord ID 기록 |
| commitments[cmOut] | true |
| Note Tree | output Note 1개 append |
| producerOf | output Note 생성 기록 연결 |
| AuditRecord | encryptedParents 2개·encryptedOutputNfs 1개 |
| Event log | NoteAppended·AuditRecorded |

Role·owner·Sustainability State 합·암호문이 틀리면 proof가 실패합니다. 두 input이 같거나 이미 소비·동결됐거나 output이 중복이면 Contract가 거부합니다. 실패하면 두 input 중 하나만 소비되는 상태는 남지 않습니다.

## 11. Operation: Split

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | Note 하나를 같은 owner·AssetRole의 Note 두 개로 나눕니다. |
| 실제 구현 | Participant가 두 output의 $q_{\mathrm{mass}}$를 정합니다. Circuit은 output 2의 $a_{\mathrm{rec}}$·$e$를 floor 계산하고 output 1에 residual을 배정합니다. 질량 0 output 하나를 허용합니다. |
| 객체 전이 | $Note_A\rightarrow Note_B+Note_C$ |
| proof 생성자 | input Note의 sk_owner를 아는 Participant |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Audit Split verifier |

ELIGIBLE과 WASTE를 모두 Split할 수 있으며, 두 output은 input의 AssetRole을 유지합니다.

$q_1+q_2=q_{\mathrm{input}}$이어야 합니다. $x\in\{a_{\mathrm{rec}},e\}$에 대해 output 2와 output 1은 다음과 같습니다.

$$
x_2=\left\lfloor\frac{x_{\mathrm{input}}q_2}{q_{\mathrm{input}}}\right\rfloor,
\qquad
x_1=x_{\mathrm{input}}-x_2
$$

### 실행 전에 무엇이 필요하나요?

Contract가 noteRoot를 acceptedNoteRoot로 수용해야 합니다. Circuit은 private input cm이 이 noteRoot에 포함됐음을 증명해야 합니다. input nf는 미소비·Active여야 하고, cmOut1과 cmOut2는 서로 다르며 아직 등록되지 않아야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** noteRoot → nf → cmOut1 → cmOut2 → R1X → R1Y → encryptedParentCM → encryptedOutputNf1 → encryptedOutputNf2

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| noteRoot | ✓ | — | split calldata로 받고 acceptedNoteRoot 확인 |
| nf | ✓ | — | split calldata로 받고 noteSpentIn·noteStatusByNf 조회 |
| cmOut1·cmOut2 | ✓ | — | split calldata로 받고 commitments 중복 확인 |
| input Note·cm·index·path·sk_owner | — | ✓ | Contract에 전달하지 않음 |
| output Note 2개·allocation remainder·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·부모 암호문·output nf 암호문 2개 | ✓ | — | AuditCipher calldata로 받음 |

### Circuit은 무엇을 검증하나요?

```text
SplitCircuit:
    # 핵심: 선택한 질량과 일관된 비례 규칙으로 State를 나눕니다.
    cm_in = AssertNoteSpend(Input, sk_owner, noteRoot, index, path, nf)
    require Input.q_mass > 0
    require q_input == q_out1 + q_out2
    require Output1.address == Input.address
    require Output2.address == Input.address
    require Output1.AssetRole == Input.AssetRole
    require Output2.AssetRole == Input.AssetRole
    AssertOutput2FloorAndOutput1Residual(a_rec, e)
    require public.cmOut1 == CommitNote(Output1)
    require public.cmOut2 == CommitNote(Output2)
    nf1 = NoteNullifier(sk_owner, cmOut1)
    nf2 = NoteNullifier(sk_owner, cmOut2)
    AssertEncrypted(context, [cm_in, nf1, nf2], auditCipher)
```

Circuit은 두 output의 DocumentHash와 input DocumentHash 사이의 관계를 검사하지 않습니다. 각 output은 새로운 private DocumentHash를 사용할 수 있습니다.

### Contract는 무엇을 확인하고 저장하나요?

Contract는 acceptedNoteRoot, input nf의 미소비·Active 상태, 서로 다른 새 output 두 개와 proof를 확인합니다.

```text
split(proof, noteRoot, nf, cmOut1, cmOut2, audit):
    require noteRoot is accepted
    require nf is unspent and Active
    require cmOut1 and cmOut2 are distinct and new
    verify Audit Split proof

    noteSpentIn[nf] = auditRecordId
    append cmOut1 then cmOut2 to Note Tree
    link both outputs in producerOf
    store AuditRecord
```

| 성공 후 상태 | 변화 |
|---|---|
| noteSpentIn[nf] | 새 AuditRecord ID 기록 |
| commitments[cmOut1·cmOut2] | 모두 true |
| Note Tree | output 1과 output 2를 순서대로 append |
| producerOf | 두 output을 같은 생성 기록에 연결 |
| AuditRecord | encryptedParents 1개·encryptedOutputNfs 2개 |
| Event log | NoteAppended 2개·AuditRecorded |

Sustainability State 합·floor·residual·owner·Role·암호문이 틀리면 proof가 실패합니다. 두 output 중 하나라도 중복이면 Contract가 전체 transaction을 되돌립니다.

## 12. Operation: Process

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | Policy Authority가 등록한 공정 verifier만 Process에 사용할 수 있게 합니다. |
| 실제 구현 | exact 3-to-2 Circuit에 공정 비율을 constant로 넣고 PolicyRecord·ScopeRef·Grant로 사용 권한을 확인합니다. |
| 객체 전이 | $Note_1+Note_2+Note_3\rightarrow ELIGIBLE\ Note+WASTE\ Note$ |
| proof 생성자 | 같은 owner의 input Note 3개와 sk_owner를 가진 Factory Participant |
| EVM 호출 권한 | 특정 msg.sender 제한 없음 |
| ZK Policy 권한 | Circuit이 policyScopeRef를 sk_owner와 policyRef에 연결하고, Contract가 그 policyScopeRef의 Grant를 확인합니다. |
| verifier | policyRecords[policyRef].verifierRef |

### Policy identity

$$
policyRef=H(PolicyRefTag,eventKind,authorityId,policyId,version)
$$

$$
policyScopeRef=H(ScopeRefTag,sk_{\mathrm{owner}},policyRef)
$$

PolicyRef는 공개되어 공정 Policy를 식별합니다. 같은 Policy를 반복 사용하는 owner는 같은 policyScopeRef로 연결됩니다. 다른 Policy에서는 다른 scope가 됩니다.

Policy Authority는 sk_owner를 알지 못합니다. Factory는 Authority가 공개한 policyRef를 받은 뒤 자기 sk_owner로 policyScopeRef를 계산하고 Authority에 전달합니다. Authority는 이 공개 policyScopeRef에 Grant를 설정합니다.

### 실행 전에 무엇이 필요하나요?

PolicyRecord가 존재하고 enabled여야 하며 EventKind는 PROCESS, arity는 3-to-2여야 합니다. 위 식으로 계산한 policyScopeRef에 Grant가 있어야 합니다. 세 input nf는 서로 다르고 모두 미소비·Active여야 하며 두 output commitment는 새 값이어야 합니다.

### 공정 상수와 결과

```text
inputArity      = 3
outputArity     = 2
D               = 1,000,000,000
lossRate        = 6.25%
carbonIntensity = input 1 kg당 0.09375 kgCO2e
WASTE mass      = intermediate mass의 10%
```

총 input을 $(q,a,e)$라고 할 때 Circuit은 다음 값을 floor 계산하고 각 remainder가 $0\le r<D$인지 확인합니다.

$$
q_{\mathrm{loss}}=\left\lfloor\frac{q\cdot 62{,}500{,}000}{D}\right\rfloor,
\qquad
\Delta e_{\mathrm{process}}=\left\lfloor\frac{q\cdot 93{,}750{,}000}{D}\right\rfloor
$$

$$
q_{\mathrm{intermediate}}=q-q_{\mathrm{loss}},
\qquad
e_{\mathrm{intermediate}}=e+\Delta e_{\mathrm{process}}
$$

$$
q_{\mathrm{waste}}=\left\lfloor\frac{q_{\mathrm{intermediate}}\cdot100{,}000{,}000}{D}\right\rfloor
$$

WASTE는 $(q_{\mathrm{waste}},0,0)$이고, ELIGIBLE은 남은 질량과 $a$, $e_{\mathrm{intermediate}}$ 전부를 받습니다.

M9 대표 입력은 다음 결과를 만듭니다. 아래 숫자는 읽기 쉽도록 $10^9$ scale을 제거한 kg·kgCO2e 값입니다. 실제 Circuit witness는 각 값을 $10^9$ 배율의 uint64로 사용합니다.

```text
Input aggregate  (320, 30, 230)
Intermediate     (300, 30, 260)
ELIGIBLE output  (270, 30, 260)
WASTE output      (30,  0,   0)
```

### 값은 어디에서 사용하나요?

```text
Circuit public input 순서:
  policyRef, policyScopeRef, noteRoot,
  nf1, nf2, nf3,
  cmEligible, cmWaste,
  R1X, R1Y,
  encrypted parent cm 3개,
  encrypted output nf 2개

Circuit private witness:
  input Note 3개·cm·path·sk_owner
  intermediate·output State
  loss·carbon·allocation remainder
  output DocumentHash·opening·audit randomness

Contract calldata:
  proof, policyRef, policyScopeRef, noteRoot,
  nf 3개, output cm 2개, AuditCipher
```

Contract는 policyRecords·policyGrants·acceptedNoteRoot·noteSpentIn·noteStatusByNf·commitments를 실행 전에 읽습니다. private input cm·Sustainability State·path는 Contract가 읽지 않습니다.

### Circuit은 무엇을 검증하나요?

```text
ProcessCircuit:
    # 핵심: 같은 owner의 Note 3개를 Policy constant에 맞는 두 output으로 변환합니다.
    require public.policyRef == CIRCUIT_POLICY_REF
    require public.policyScopeRef == H(ScopeRefTag, sk_owner, policyRef)

    for input 1..3:
        cm[i] = AssertNoteSpend(input[i], sk_owner, noteRoot, path[i], nf[i])
        require input[i].AssetRole == ELIGIBLE

    require 모든 cm·nf가 서로 다름
    total = checked uint64 sum(inputs.State)
    intermediate = ApplyLossAndCarbon(total, Policy constants)
    outputs = ApplyAllocation(intermediate)

    require output 0 Role == ELIGIBLE
    require output 1 Role == WASTE
    require output 0 and output 1 address == owner
    require WASTE.a_rec == 0 and WASTE.e == 0
    require public cmEligible·cmWaste == output commitments

    nfEligible = NoteNullifier(sk_owner, cmEligible)
    nfWaste = NoteNullifier(sk_owner, cmWaste)
    AssertEncrypted(context, [cm1, cm2, cm3, nfEligible, nfWaste], auditCipher)
```

Circuit은 input과 output의 DocumentHash 관계를 검사하지 않습니다. ELIGIBLE과 WASTE output은 새로운 private DocumentHash를 사용할 수 있습니다.

### Contract는 무엇을 확인하고 저장하나요?

```text
process(proof, policyRef, scopeRef, root, nf[3], cmOut[2], audit):
    # 핵심: final Contract는 Process verifier를 PolicyRecord에서 선택합니다.
    # 1. Policy와 권한을 확인합니다.
    policy = policyRecords[policyRef]
    require policy exists and enabled
    require policy.eventKind == PROCESS
    require policy.inputArity == 3 and policy.outputArity == 2
    require policyGrants[policyRef][scopeRef]

    # 2. Note 상태를 확인합니다.
    require acceptedNoteRoot(root)
    require nf 3개가 distinct·unspent·Active
    require output commitment 2개가 새 값

    # 3. 등록된 verifier로 proof를 검증합니다.
    verify policy.verifierRef with Process public inputs

    # 4. 원자적으로 기록합니다.
    noteSpentIn[nf1..nf3] = auditRecordId
    append cmEligible and cmWaste to Note Tree
    record producerOf and AuditRecord
```

### 온체인 상태 변화

| 상태 | 변화 |
|---|---|
| noteSpentIn | nf 3개 소비 기록 |
| commitments | cmEligible·cmWaste를 모두 true로 등록 |
| Note Tree | ELIGIBLE·WASTE 2개 append |
| producerOf | output 2개 생성 기록 |
| AuditRecord | 부모 3개·출력 소비값 2개 암호문 기록 |
| Event log | NoteAppended 2개·AuditRecorded |

잘못된 PolicyRef·ScopeRef·owner·membership·Sustainability State 계산·Role·암호문은 proof를 실패시킵니다. 비활성 Policy, 없는 Grant, 소비되거나 Frozen인 nf, 중복 output은 Contract가 거부합니다. 실패하면 nf 3개와 output 2개 중 일부만 기록되는 상태는 남지 않습니다.

## 13. Operation: Exit

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | 공급망 Note를 종료하고 Claim을 연결할 DPP commitment를 만듭니다. |
| 실제 구현 | Circuit이 Note와 DPPPrivateData의 DocumentHash·AssetRole·State가 같은지 검증합니다. |
| 객체 전이 | Note를 소비하고 dppCommitment를 등록 |
| proof 생성자 | input Note와 sk_owner, DPP private data를 가진 Participant |
| EVM 호출 권한 | 별도 msg.sender 제한 없음 |
| verifier | constructor에 고정된 Exit DPP verifier |

### 실행 전에 무엇이 필요하나요?

Contract가 noteRoot를 acceptedNoteRoot로 수용해야 합니다. Circuit은 private input cm이 이 noteRoot에 포함됐음을 증명해야 합니다. nf는 미소비·Active여야 하며, 같은 dppCommitment를 producerOf[DPP]에 이미 등록하지 않았어야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** noteRoot → nf → dppCommitment → R1X → R1Y → encryptedParentCM

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| noteRoot | ✓ | — | exit calldata로 받고 acceptedNoteRoot 확인 |
| nf | ✓ | — | exit calldata로 받고 noteSpentIn·noteStatusByNf 조회 |
| dppCommitment | ✓ | — | exit calldata로 받고 producerOf 중복 확인 |
| input Note·cm·index·path·sk_owner | — | ✓ | Contract에 전달하지 않음 |
| DPPPrivateData·audit randomness | — | ✓ | Contract에 전달하지 않음 |
| $R_1$·encryptedParentCM | ✓ | — | AuditCipher calldata로 받음 |

### Circuit은 무엇을 검증하나요?

```text
ExitCircuit:
    # 핵심: 소비한 Note와 같은 제품·State만 DPP로 확정합니다.
    cm = AssertNoteSpend(Note, sk_owner, noteRoot, index, path, nf)
    require DPP.DocumentHash == Note.DocumentHash
    require DPP.AssetRole == Note.AssetRole
    require DPP.State == Note.State
    require public.dppCommitment == CommitDPP(DPPPrivateData)
    AssertEncrypted(context, [cm], auditCipher)
```

### Contract는 무엇을 확인하고 저장하나요?

```text
exit(proof, noteRoot, nf, dppCommitment, audit):
    require noteRoot is accepted
    require noteSpentIn[nf] == 0
    require noteStatusByNf[nf] == Active
    require producerOf[DPP][dppCommitment] == 0
    verify fixed Exit verifier

    noteSpentIn[nf] = auditRecordId
    producerOf[DPP][dppCommitment] = auditRecordId
    store Exit AuditRecord(outputRefs=[DPPRef])
```

| 성공 후 상태 | 변화 |
|---|---|
| noteSpentIn[nf] | Exit AuditRecord ID 기록 |
| producerOf[DPP][dppCommitment] | 같은 AuditRecord ID 기록 |
| AuditRecord | outputRefs=[DPP], encryptedParents=[input cm] |
| Note·Voucher Tree | 변경 없음 |
| Event log | AuditRecorded·NoteExited·DPPFinalized |

ELIGIBLE과 WASTE 모두 Exit할 수 있습니다. DPP는 소비하지 않으므로 DPP Tree와 nullifier를 만들지 않습니다. 잘못된 Note·DPP 대응 관계와 암호문은 proof를 실패시키고, 이미 소비·동결된 nf나 중복 dppCommitment는 Contract가 거부합니다.

## 14. Operation: Issue

### 무엇을 구현했나요?

| 항목 | 내용 |
|---|---|
| 구현 목표 | 제품 정보를 공개하지 않고 DPP State가 Sustainability Policy를 만족한다는 Claim을 등록합니다. |
| 실제 구현 | Standard V1·Strict V2 Policy Circuit과 DPP·Policy 조합의 Claim mapping을 구현했습니다. |
| 객체 전이 | dppCommitment는 바꾸지 않고 claimRecordOf에 Claim 기록 1개를 추가 |
| proof 생성자 | dppCommitment의 private 원문을 아는 사람 |
| EVM 호출 권한 | 별도 msg.sender 제한과 PolicyGrant 없음 |
| verifier | policyRecords[issuePolicyRef].verifierRef |

Issue Policy는 다음입니다.

두 Policy는 비율 분모 $D=1{,}000{,}000{,}000$을 사용합니다.

| Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

### 실행 전에 무엇이 필요하나요?

producerOf[DPP][dppCommitment]가 Exit AuditRecord를 가리켜야 합니다. Issue PolicyRecord가 존재하고 enabled이며 EventKind는 ISSUE, arity는 1-to-1이어야 합니다. 같은 DPP·issuePolicyRef Claim은 아직 없어야 합니다.

### 값은 어디에서 사용하나요?

**Circuit public input 순서:** issuePolicyRef → dppCommitment

| 값 | Circuit public input | Circuit private witness | Contract에서의 위치 |
|---|---:|---:|---:|
| issuePolicyRef | ✓ | — | issue calldata로 받고 policyRecords 조회 |
| dppCommitment | ✓ | — | issue calldata로 받고 producerOf·claimRecordOf 조회 |
| DocumentHash·AssetRole·State·dppOpening | — | ✓ | Contract에 전달하지 않음 |

### Circuit은 무엇을 검증하나요?

```text
IssueCircuit:
    # 핵심: private DPP State가 Circuit에 고정된 Sustainability 기준을 만족합니다.
    require public.issuePolicyRef == CIRCUIT_POLICY_REF
    AssertDPPFieldRangesAndRoleRules(private DPP data)
    require q_mass > 0
    require a_rec <= q_mass
    require AssetRole == ELIGIBLE
    require public.dppCommitment == CommitDPP(private DPP data)
    recycledHave = a_rec * D
    recycledNeed = q_mass * minRecycledRate
    carbonHave = e * D
    carbonLimit = q_mass * maxCarbonIntensity
    # Field 비교를 정수 비교로 해석할 수 있게 네 곱의 범위를 고정합니다.
    require recycledHave fits in 94 bits
    require recycledNeed fits in 94 bits
    require carbonHave fits in 94 bits
    require carbonLimit fits in 94 bits
    require recycledHave >= recycledNeed
    require carbonHave <= carbonLimit
```

### Contract는 무엇을 확인하고 저장하나요?

```text
issue(proof, issuePolicyRef, dppCommitment):
    exitRecordId = producerOf[DPP][dppCommitment]
    require exitRecordId != 0
    require auditRecords[exitRecordId].eventKind == EXIT

    policy = policyRecords[issuePolicyRef]
    require policy exists and enabled
    require policy.eventKind == ISSUE and arity == 1-to-1
    require claimRecordOf[dppCommitment][issuePolicyRef] == 0
    verify policy.verifierRef

    claimRecordOf[dppCommitment][issuePolicyRef] = auditRecordId
    store ciphertext-free Issue AuditRecord
```

| 성공 후 상태 | 변화 |
|---|---|
| claimRecordOf[dppCommitment][issuePolicyRef] | Issue AuditRecord ID 기록 |
| claimStatus | 별도 SSTORE 없음. mapping 기본값 0을 Active로 해석 |
| AuditRecord | eventKind·policyRef만 저장하고 암호문과 outputRefs는 비움 |
| Tree·spentIn·producerOf | 변경 없음 |
| Event log | AuditRecorded·ClaimIssued |

Issue에는 owner secret·Note membership·nf·PolicyGrant·policyScopeRef가 없습니다. 미등록 DPP, Exit가 만들지 않은 DPP, 비활성·잘못된 Issue Policy, 중복 Claim과 잘못된 proof는 실패하며 어떤 Claim 기록도 남지 않습니다.

## 15. Operation마다 어떤 AuditRecord를 남기나요?

감사 암호문을 사용하는 Operation에서는 부모 참조와 output이 나중에 소비될 때 사용할 값을 하나의 private Field masking key $K$로 함께 암호화합니다. $K$는 Committee public key $PK$와 다른 값입니다. storage에서는 암호문을 의미에 따라 두 배열로 나눕니다. Issue는 암호화를 사용하지 않습니다.

| Operation | encryptedParents 평문 | encryptedOutputNfs 평문 | outputRefs 순서 |
|---|---|---|---|
| Entry | 없음 | output nf | Note |
| Transfer | input cm | Voucher rvnf, Change nf | Voucher, Change Note |
| Proceed | input rv | Receiver Note nf | Receiver Note |
| Recall | input rv | Return Note nf | Return Note |
| Merge | input cm 2개 | output nf | Note |
| Split | input cm | output nf 2개 | Note 1, Note 2 |
| Process | input cm 3개 | ELIGIBLE nf, WASTE nf | ELIGIBLE, WASTE |
| Exit | input cm | 없음 | DPP |
| Issue | 없음 | 없음 | 없음 |

Circuit은 임의 평문 배열을 그대로 암호화하지 않습니다. 실제 private input commitment와 실제 output이 소비될 nf·rvnf를 계산한 뒤 그 값이 암호문에 들어 있는지 검증합니다.

## 16. 감사 암호화는 어떻게 동작하나요?

M6-B1에서 만든 2-of-3 Threshold DH 기반 Field 암호화를 Entry부터 Exit까지의 Operation이 재사용합니다. Issue는 private 부모나 output 소비값이 없으므로 이 암호화를 사용하지 않습니다.

$$
R_1=rG,\qquad Z=rPK
$$

$$
K=H(AuditKeyTag,Z_X,Z_Y,L,n)
$$

$$
C_j=M_j+H(AuditMaskTag,K,j)\pmod p
$$

- $PK$는 Committee public key입니다. $K$는 $Z$에서 파생한 private Field masking key이며 public key가 아닙니다.
- $L$은 EventKind와 감사 필드를 제외한 해당 Operation의 public input을 Hash한 Audit Context입니다.
- $n$은 평문 Field 원소의 개수입니다.
- 하나의 private Field masking key $K$에서 위치별 마스크를 만듭니다.
- 부모와 output 소비값의 mask index는 하나의 연속 벡터입니다.
- 위원회는 검증된 온체인 원본에 대해서만 partial decryption을 제공합니다.

$r$은 proof 생성자가 고른 비공개 암호화 난수이고, $G$는 고정 Jubjub generator입니다. $PK$는 2-of-3 Committee public key입니다. $R_1=rG$는 AuditRecord에 공개하고, $Z=rPK$는 $K$를 파생하는 private shared point입니다. $j$는 0부터 시작하는 평문 위치입니다. $p$는 BLS12-381 scalar field이자 Circuit Field의 modulus로 $M_j$, $C_j$, $K$, 마스크를 계산할 때 사용합니다. Jubjub scalar $r$과 Committee secret share는 별도의 subgroup order $q$에 대해 계산합니다.

**$L$과 $n$은 Threshold DH 암호화·복호화의 correctness에 필수값이 아닙니다.** 두 값은 기존 zkDPP Protocol이나 원 TDH2가 요구한 값이 아니라, M6-B1 POC 구현 과정에서 AI가 추가 binding을 위해 자동 제안한 보조값입니다. 현재처럼 Operation별 평문 순서와 길이가 고정된 POC에서는 $L$의 실익이 작고 $n$은 중복 정보입니다. 다만 현재 M6-B1~M9 Circuit·PK·VK·proof에는 두 값이 포함돼 있으므로, 제거하려면 해당 Artifact를 다시 생성해야 합니다.

이 POC 암호화 코어는 원 TDH2의 인증 암호문 전체를 구현하지 않습니다. Contract와 Circuit이 검증해 저장한 원본만 위원회가 처리하고, 위원들이 올바른 partial decryption을 반환한다고 가정합니다. 악의적인 위원 응답을 판별하는 proof는 구현하지 않았습니다.

Auditor는 감사 시작 시 block number와 block hash를 snapshot으로 고정합니다. 이후의 producerOf·spentIn 조회는 이 시점의 상태를 기준으로 해석합니다. LoadAndVerifyOriginal은 snapshot 안의 transaction·receipt와 Contract AuditRecord가 같은 Operation·outputRefs·암호문을 나타내는지 확인하는 절차입니다.

### Backward tracing

Claim은 producerOf에 등록되지 않으므로 먼저 Claim 전용 시작 단계를 사용합니다.

encryptedParents를 복호화하면 cm 또는 rv Field 값이 나옵니다. Auditor는 AuditRecord의 eventKind로 각 부모가 Note인지 Voucher인지와 부모 개수를 판단합니다.

```text
TraceBackwardFromClaim(dppCommitment, issuePolicyRef, snapshot):
    # 1. Claim을 만든 Issue 기록과 원본 transaction을 확인합니다.
    issueId = claimRecordOf[dppCommitment][issuePolicyRef]
    issueRecord = LoadAndVerifyOriginal(issueId, snapshot)
    require issueRecord.eventKind == ISSUE

    # 2. 같은 DPP를 만든 Exit 기록으로 이동합니다.
    exitId = producerOf[DPP][dppCommitment]
    return TraceBackwardObject(DPPRef(dppCommitment), exitId, snapshot)
```

```text
TraceBackwardObject(startRef, producerId, snapshot):
    # 핵심: 객체를 만든 기록에서 private 부모를 복원해 Entry 방향으로 이동합니다.
    record = LoadAndVerifyOriginal(producerId, snapshot)

    if record is Entry:
        return Entry

    plaintext = DecryptOnce(record)
    for each parent in plaintext.parents:
        parentProducerId = producerOf[parent.objectType][parent.objectId]
        TraceBackwardObject(parent, parentProducerId, snapshot)
```

M9에서는 Standard Claim에서 DPP·Exit·Split·Process·Merge·Transfer를 거쳐 Entry 4건을 복원했습니다.

### Forward tracing

```text
TraceForward(startRef, snapshot):
    # 핵심: 현재 cm·rv가 소비될 때 사용할 nf·rvnf를 복원해 다음 Operation을 찾습니다.
    producerId = producerOf[startRef.objectType][startRef.objectId]
    producerRecord = LoadAndVerifyOriginal(producerId, snapshot)
    plaintext = DecryptOnce(producerRecord)
    spendValue = producerRecord.outputRefs에서 startRef와 같은 위치의 plaintext output 소비값
    if startRef is Note:
        consumerId = noteSpentIn[spendValue]
    else if startRef is Voucher:
        consumerId = voucherSpentIn[spendValue]

    if consumerId == 0:
        return unspent leaf

    consumerRecord = LoadAndVerifyOriginal(consumerId, snapshot)
    for each outputRef in consumerRecord:
        if outputRef is DPP:
            return terminal DPP with Claims
        TraceForward(outputRef, snapshot)
```

M9에서는 Aluminum A에서 시작해 zero Change 분기·Recall·재Transfer·Merge·Process·Split을 지나 Product DPP와 미소비 Note를 찾았습니다.

복호화 결과는 한 감사 실행의 memory cache에만 둡니다. 다음 감사는 빈 cache로 시작합니다. 조회 오류를 미소비 leaf로 처리하지 않습니다.

Note·Voucher output은 outputRefs와 encryptedOutputNfs가 같은 순서로 대응합니다. DPP는 소비값이 없으므로 encryptedOutputNfs가 없고 forward tracing의 terminal로 처리합니다.

감사 snapshot 이후 대상 Note·Voucher가 먼저 소비되면 Status Authority의 Freeze transaction은 현재 spentIn 검사에서 실패합니다. Contract는 downstream을 자동으로 재추적하거나 모든 leaf를 재귀적으로 동결하지 않습니다. Auditor가 snapshot 결과에서 동결 대상을 선택하고 각 값을 별도로 제출합니다.

## 17. Frozen·Revoked는 어떻게 집행하나요?

### Note·Voucher 상태

```text
setStatus(objectType, spendValue, newStatus):
    # 핵심: 공개 nf·rvnf 상태를 proof 검증 전에 조회할 수 있게 합니다.
    require msg.sender == statusAuthority
    require objectType is NOTE or VOUCHER
    require spendValue < BLS12-381 scalar field modulus
    if objectType == NOTE:
        require noteSpentIn[spendValue] == 0
        oldStatus = noteStatusByNf[spendValue]
        require transition(oldStatus, newStatus) is allowed
        noteStatusByNf[spendValue] = newStatus

    if objectType == VOUCHER:
        require voucherSpentIn[spendValue] == 0
        oldStatus = voucherStatusByNf[spendValue]
        require transition(oldStatus, newStatus) is allowed
        voucherStatusByNf[spendValue] = newStatus
```

### Claim 상태

```text
setClaimStatus(dppCommitment, policyRef, newStatus):
    require msg.sender == statusAuthority
    require claimRecordOf[dppCommitment][policyRef] != 0
    require transition is allowed
    claimStatus[dppCommitment][policyRef] = newStatus
```

허용 전이는 다음입니다.

```text
Active → Frozen
Frozen → Active
Frozen → Revoked
```

Revoked는 terminal입니다. 이미 소비된 Note·Voucher의 상태 변경은 거부합니다. Claim은 Policy version별로 독립 상태를 가집니다.

status mapping의 기본값 0은 “이 nf가 실제 객체에서 파생됐다”는 증거가 아닙니다. 유효한 객체와 nf의 관계는 소비 Operation proof가 검증합니다. setStatus는 객체 존재를 별도로 증명하지 않으므로, Status Authority가 감사로 복원한 올바른 nf·rvnf만 제출한다고 가정합니다.

### 왜 StatusTree가 없나요?

최종 경로에서는 소비 transaction이 이미 공개하는 nf·rvnf를 mapping key로 사용합니다. 따라서 StatusTree path·StatusUpdate proof·외부 Status Indexer가 필요하지 않습니다.

대신 동결된 nf와 이후 소비 transaction이 공개적으로 연결되는 Privacy trade-off를 허용합니다.

## 18. Operation별 온체인 상태 변화는 어떻게 다른가요?

먼저 새 객체 등록과 Tree 변화를 비교합니다.

| Operation | commitments | voucherCommitments | Note Tree | Voucher Tree |
|---|---:|---:|---:|---:|
| Entry | +1 | — | +1 | — |
| Transfer | +1 | +1 | +1 | +1 |
| Proceed | +1 | — | +1 | — |
| Recall | +1 | — | +1 | — |
| Merge | +1 | — | +1 | — |
| Split | +2 | — | +2 | — |
| Process | +2 | — | +2 | — |
| Exit | — | — | — | — |
| Issue | — | — | — | — |

다음 표는 소비·생성 관계와 감사·Claim 기록을 보여줍니다.

| Operation | noteSpentIn | voucherSpentIn | producerOf | AuditRecord | claimRecordOf |
|---|---:|---:|---:|---:|---:|
| Entry | — | — | Note +1 | +1 | — |
| Transfer | 1개 | — | Voucher·Note +2 | +1 | — |
| Proceed | — | 1개 | Note +1 | +1 | — |
| Recall | — | 1개 | Note +1 | +1 | — |
| Merge | 2개 | — | Note +1 | +1 | — |
| Split | 1개 | — | Note +2 | +1 | — |
| Process | 3개 | — | Note +2 | +1 | — |
| Exit | 1개 | — | DPP +1 | +1 | — |
| Issue | — | — | — | +1 | Claim +1 |

표의 `+1`은 Tree에 추가한 객체 leaf 개수입니다. 실제 append는 leaf뿐 아니라 depth-32 경로의 중간 node도 갱신하므로 SSTORE 1회를 뜻하지 않습니다.

## 19. 최종 SRS와 verifier는 어떻게 구성했나요?

최종 대상은 고정 Operation Circuit 7개와 Policy Circuit 3개입니다.

| Relation | Constraints | Public | Domain |
|---|---:|---:|---:|
| Audit Entry | 19,020 | 4 | $2^{15}$ |
| Audit Transfer | 53,836 | 11 | $2^{16}$ |
| Audit Proceed | 39,766 | 7 | $2^{16}$ |
| Audit Recall | 43,375 | 8 | $2^{16}$ |
| Audit Merge | 57,551 | 9 | $2^{16}$ |
| Audit Split | 52,246 | 9 | $2^{16}$ |
| Exit DPP | 36,930 | 6 | $2^{16}$ |
| Audit Process 3-to-2 | 93,578 | 15 | $2^{17}$ |
| Issue Standard | 12,490 | 2 | $2^{14}$ |
| Issue Strict | 12,490 | 2 | $2^{14}$ |

가장 큰 Process를 기준으로 universal canonical SRS를 만들었습니다.

```text
maximum Lagrange domain = 131,072
universal canonical SRS = 131,075 points
```

하나의 canonical SRS에서 $2^{14}$·$2^{15}$·$2^{16}$·$2^{17}$ Lagrange SRS를 만들고, relation별 PK·VK를 별도로 생성했습니다.

```text
Universal SRS
  ├─ fixed Operation verifier 7개
  └─ Policy verifier 3개
        ↓
  ZkDPPClaimLedger
```

이 SRS는 local unsafe development SRS입니다. Production Ceremony 결과가 아닙니다. $2^{17}$보다 큰 미래 Circuit은 더 큰 SRS 또는 별도 SRS version이 필요합니다.

## 20. 최종 대표 시나리오는 어떻게 실행했나요?

### 시작 State

아래 값은 읽기 쉽도록 $10^9$ scale을 제거한 kg·kgCO2e 단위입니다. 실제 Note와 Circuit witness에는 각 값을 $10^9$ 배율의 uint64로 넣습니다.

| 원자재 | Owner | State $(q,a,e)$ |
|---|---|---|
| Aluminum A | actor-1 | $(60,10,40)$ |
| Aluminum B | actor-1 | $(60,10,50)$ |
| Cathode | actor-3 | $(100,10,70)$ |
| Anode | actor-3 | $(100,0,70)$ |

### 전체 흐름

1. 네 원자재를 Entry했습니다.
2. Aluminum A를 Transfer해 첫 Voucher를 만든 뒤 Freeze하고 Proceed·Recall 거부를 확인했습니다.
3. Unfreeze 후 Recall하고 다시 Transfer·Proceed했습니다.
4. 나머지 원자재를 Factory actor-2에게 Transfer·Proceed했습니다.
5. Aluminum A·B를 Merge했습니다.
6. Merge output·Cathode·Anode를 3-to-2 Process에 넣었습니다.
7. ELIGIBLE $(270,30,260)$과 WASTE $(30,0,0)$을 만들었습니다.
8. ELIGIBLE output을 Product 1·2로 Split했습니다.
9. Product 1을 Exit해 DPP를 만들고 Standard·Strict Claim을 Issue했습니다.
10. WASTE도 DPP로 Exit했습니다. WASTE는 Issue Circuit의 ELIGIBLE 조건을 만족하지 않아 Issue proof를 만들 수 없음을 확인했습니다.
11. Standard Claim은 Active, Strict Claim은 Revoked로 끝났습니다.
12. backward·forward 감사를 실행했습니다.

최종 상태는 Note leaf 19개, Voucher leaf 5개, AuditRecord 21개입니다. Product 2는 미소비 Active Note로 남았습니다.

## 21. 무엇을 구현하려 했고 실제로는 어떻게 되었나요?

| 영역 | 구현하려던 기능 | 실제 구현 | 남은 한계 |
|---|---|---|---|
| Private Note | 내용·owner·path를 숨긴 소비 | private cm·path와 public root·nf | Contract가 msg.sender와 ZK owner를 비교하지 않음 |
| Transfer | private 소유권 전달 | shared Voucher opening과 Proceed·Recall 경쟁 | opening 전달 암호화 미구현 |
| Merge | compatible 객체 결합 | 같은 owner·AssetRole만 검사 | ProductProfile 미검증 |
| Split | State 보존 분할 | output 2 floor·output 1 residual | 현실별 rounding Policy 한 종류 |
| Process | 등록된 Policy verifier가 강제하는 공정 | exact 3-to-2 constant Policy | 다른 arity·공정 Policy 없음 |
| Status | 객체 Freeze·Revoke | nf·rvnf mapping 직접 조회 | 동결 후 소비 transaction 연결 공개 |
| Audit | backward·forward provenance | 2-of-3 복호화와 snapshot traversal | 정직한 위원·Auditor 가정 |
| DPP | 공급망 종료 제품 commitment | Exit에서 Note와 DPP State 동일성 증명 | DPP ownership transfer 없음 |
| Claim | State를 숨긴 Sustainability 주장 | Standard·Strict Policy Claim | 규제 DPP 전체를 대체하지 않음 |
| SRS | 모든 final Circuit의 공통 기반 | $2^{17}$ development universal SRS | Production Ceremony 아님 |
| Contract | 하나의 Main Protocol 원장 | 고정 verifier 7개·Policy verifier 3개 | upgrade·proxy·Router 없음 |

### 계획과 실제 측정의 차이

- Setup은 2회 실행했고 20% 초과 항목이 없어 3회차를 실행하지 않았습니다.
- Anvil 전체 시나리오는 2회 실행했으며 gas·calldata·bytecode가 일치했습니다.
- Forward 감사 시간이 20% 이상 달라 감사만 3회 실행하고 median을 사용했습니다.
- Anvil은 final fixed proof를 제출했습니다. Prove 시간과 transaction 시간은 별도 측정이므로 하나의 live 사용자 지연으로 해석하지 않습니다.
- 첫 Anvil 시도는 fixture Field 문자열 형식 문제로 실패했고 canonical 32-byte hex로 수정했습니다.

## 22. 실제 비용은 어느 정도였나요?

### 대표 Circuit

Setup과 Prove는 GOMAXPROCS=8인 로컬 환경에서 relation별로 수행한 wall-clock 시간입니다. Setup은 CCS와 SRS에서 PK·VK를 만드는 구간이고, Prove는 준비된 PK와 witness에서 proof를 만드는 구간입니다. 표의 두 값은 독립된 Run 1과 Run 2입니다.

| Relation | Setup Run 1/2 ms | Prove Run 1/2 ms |
|---|---:|---:|
| Entry | 142.672 / 146.027 | 444.396 / 445.490 |
| Transfer | 296.916 / 295.929 | 849.750 / 869.254 |
| Process | 552.792 / 536.107 | 1,626.416 / 1,622.951 |
| Exit DPP | 270.358 / 278.791 | 825.978 / 829.556 |
| Issue Standard | 89.358 / 88.673 | 253.510 / 252.372 |

### 대표 Contract transaction

Gas와 calldata는 final fixed proof를 Anvil transaction으로 제출해 receipt에서 측정했습니다. Prove 시간은 포함하지 않습니다. SSTORE는 transaction trace에서 센 opcode 횟수입니다.

| 실행 | Gas | Calldata | SSTORE |
|---|---:|---:|---:|
| 첫 Entry | 2,258,465 | 1,412 B | 49 |
| 첫 Transfer | 3,596,789 | 1,636 B | 93 |
| Recall | 1,784,864 | 1,540 B | 51 |
| Merge | 1,849,682 | 1,572 B | 53 |
| Process | 2,933,492 | 1,764 B | 97 |
| Split | 2,722,918 | 1,572 B | 93 |
| Product Exit | 618,876 | 1,476 B | 11 |
| Standard Issue | 459,977 | 1,188 B | 4 |

SSTORE 횟수는 transaction 전체 trace의 opcode 수입니다. 이를 순수 Tree 비용이나 순수 AuditRecord 비용으로 해석하지 않습니다.

### 감사

감사 시간은 고정 snapshot에서 RPC로 원본 transaction·AuditRecord를 조회하고, 위원 두 명의 응답을 결합해 graph를 탐색한 전체 wall-clock입니다. 세 실행의 median을 대표값으로 표시합니다.

| 방향 | 대표 median | 방문 범위 |
|---|---:|---|
| Backward | 8,174.448 ms | Claim에서 Entry 4건까지 |
| Forward | 4,081.522 ms | Aluminum A에서 DPP 2개·Note leaf 3개까지 |

감사 시간 대부분은 로컬 RPC 원본 조회입니다. 이 값은 대규모 공급망 throughput이 아닙니다.

## 23. 어디에 구현되어 있나요?

| 목적 | 위치 |
|---|---|
| Final Main Contract | `contracts/src/final/ZkDPPClaimLedger.sol` |
| Final Circuit·SRS runtime | `internal/finalsrs`, `internal/m9run` |
| 전체 공급망 고정 데이터 | `internal/m9case` |
| Forward audit adapter | `internal/m9audit` |
| Final SRS·PK·VK | `artifacts/final` |
| Public input manifest | `artifacts/final/verifiers/public-input-manifest.json` |
| Raw 측정값 | `output/m9-*.json` |

재현 명령은 다음입니다.

```text
make setup-m9-final
make evaluate-m9
make benchmark-m9-setup
make test-go
make test-contract-m9
make benchmark-m9-anvil
make benchmark-m9-audit
make check-m9
```

## 24. 이 문서를 어떻게 읽으면 되나요?

- Protocol의 큰 그림만 필요하면 0~5장을 읽습니다.
- 특정 Operation의 동작이 필요하면 6~14장을 읽습니다.
- Contract storage가 궁금하면 3장과 18장을 읽습니다.
- Audit·Freeze가 궁금하면 15~17장을 읽습니다.
- SRS·최종 시나리오·측정이 궁금하면 19~22장을 읽습니다.
- 정확한 public input 순서와 구현 Gate는 [M9 명세](../milestones/M9-final-integration.md)를 확인합니다.
- 실제 실행값과 실패·재실행 이력은 [M9 Result](../milestones/M9-final-integration-result.md)를 확인합니다.

이 문서가 설명하는 완료 범위는 **현재 zkDPP POC가 private 객체·Policy·감사·상태·DPP Claim을 하나의 universal SRS와 Main Contract에서 실행할 수 있다는 것**입니다. Production 거버넌스·규제 DPP 전체·실제 trusted setup이 완료됐다는 의미는 아닙니다.
