# M6 Status 집행 Background

이 문서는 M6 구현 방법이 아니라 **왜 현재 Status 구조를 선택했는지** 보존합니다. 실제 구현 기준은 [M6 Status 집행 명세](M6-status-enforcement.md)입니다.

## 0. 30초 안에 Context 복구하기

```text
공개 output:
  Note cm 또는 Voucher rv와 insertion index 생성

비공개 소비:
  cm·rv, index와 Merkle path를 private witness로 사용

문제:
  Contract는 private cm·rv로 public mapping을 조회할 수 없음

해결:
  기존 object index를 별도 StatusTree index로 재사용
  → empty leaf로 Active 증명
  → Status Authority가 StatusUpdate ZKP로 root 변경 증명
```

| 구성요소 | 한 줄 역할 |
|---|---|
| Note·Voucher Tree | 객체가 실제로 생성됐음을 증명합니다. |
| Note·Voucher StatusTree | 같은 index의 객체가 현재 Active인지 증명합니다. |
| Participant | private 객체와 두 Merkle path로 Status-aware Event proof를 생성합니다. |
| Status Authority | 공개 대상을 Freeze·Unfreeze·Revoke하고 root 변경 proof를 생성합니다. |
| Indexer | 두 StatusTree 전체와 최신 path를 오프체인에서 유지합니다. |
| `ZkDPPStatusLedger` | 최신 Status root와 상태 전이를 집행하는 M6 이후 Main Protocol Contract입니다. |

M1~M5는 Main Protocol에 필요한 commitment, private spend, Event, Tree와 Policy를 단계적으로 확인한 building-block milestone입니다. M6부터는 Status 검사를 생략할 수 없는 별도 Main Contract를 사용합니다.

## 1. 무엇이 문제였나요?

Note commitment `cm`과 Voucher commitment `rv`는 생성 transaction에서 공개됩니다. 그러나 소비할 때는 어떤 과거 객체를 사용했는지 숨기기 위해 `cm`과 `rv`를 Circuit private witness로 둡니다.

```text
생성:
  NoteAppended(cm_A, index=5)

소비:
  public  = root, nullifier, output
  private = cm_A, index 5, path
```

Contract가 소비 `cm_A`를 보지 못하므로 다음 일반 mapping 조회를 수행할 수 없습니다.

```text
status[cm_A] == Active?
```

소비 시 `cm_A` 또는 index 5를 다시 공개하면 생성 Event와 소비 transaction이 직접 연결됩니다. 따라서 Contract가 hidden key를 조회하는 대신, Circuit이 현재 공개 root에 대해 private 객체의 Active 상태를 증명해야 합니다.

## 2. 왜 별도 StatusTree를 사용하나요?

### [폐기] 소비할 때 object ID를 공개하는 mapping

```text
require(status[cm] == Active)
```

구현은 가장 단순하지만 소비 transaction이 과거 output `cm`을 공개하므로 private provenance 목표와 맞지 않습니다.

### [폐기] 기존 Note·Voucher Tree를 mutable하게 변경

기존 leaf를 `H(cm,status)` 형태로 바꾸면 하나의 path로 존재와 Active를 함께 검증할 수 있습니다. 하지만 Status 변경 전의 과거 membership root를 허용하면 Freeze를 우회할 수 있고, M1~M5 Circuit·PK·VK·Result 전체의 의미를 다시 바꿔야 합니다.

### [이번 결정] Membership Tree와 StatusTree 분리

```text
Note Tree:
  객체 존재와 insertion index

Note StatusTree:
  같은 index의 현재 Status
```

기존 Note·Voucher membership root는 append-only 특성에 따라 허용된 과거 root를 사용할 수 있습니다. 반면 Status proof는 반드시 Contract의 최신 Status root를 사용합니다. 이 분리로 정상 append의 동시성과 Freeze의 즉시 집행을 함께 유지합니다.

## 3. 왜 기존 insertion index를 재사용하나요?

### [폐기] `Hash(cm)` 일부를 Status index로 사용

Depth-32 Tree에서 `Hash(cm)`의 32 bit를 위치로 사용하면 서로 다른 객체가 같은 위치에 충돌할 수 있습니다. 객체 수가 늘수록 birthday bound에 따라 충돌 가능성이 빠르게 증가합니다.

### [이번 결정] 충돌 없는 기존 index 재사용

```text
Note Tree index 5
  ↔ Note StatusTree index 5

Voucher Tree index 2
  ↔ Voucher StatusTree index 2
```

Note와 Voucher는 서로 다른 StatusTree를 사용하므로 양쪽에 같은 numeric index가 있어도 충돌하지 않습니다. index는 생성 시 `cm`·`rv`와 함께 공개되지만 소비 Circuit에서는 private witness로 유지됩니다.

같은 index를 두 증명에서 공유합니다.

```text
Note membership:
  cm + index + notePath → noteRoot

Active status:
  0 + 같은 index + statusPath → noteStatusRoot
```

별도 Status index나 `cm→statusIndex` mapping을 만들지 않습니다.

## 4. 왜 Active를 empty leaf로 표현하나요?

Status 값은 다음 고정 field 값입니다.

| Status | Leaf |
|---|---:|
| Active | `0` |
| Frozen | `1` |
| Revoked | `2` |

새 객체는 기본 Active이므로 Note·Voucher output이 생길 때 Status leaf를 별도로 쓰지 않습니다.

```text
Entry·Transfer·Proceed·Recall·Merge·Split·Process:
  Status root 변경 없음

Freeze·Unfreeze·Revoke:
  Status root 변경
```

이 방식에서 Active proof는 임의 key가 Tree 전체에 없다는 검색이 아닙니다. 기존 object index로 정해진 정확한 위치의 leaf가 empty `0`이라는 Merkle membership입니다. 객체 자체의 존재는 같은 Circuit의 Note·Voucher membership이 보장합니다.

명시적 Active leaf를 모든 output에 추가하는 방식은 proof는 단순하지만 정상 Event마다 Status root를 바꾸고 모든 pending Status proof를 낡게 만듭니다. 이번 설계는 예외만 기록해 Status root가 실제 상태 변경 시에만 움직이도록 합니다.

## 5. 왜 Tree 전체를 오프체인에 두나요?

Contract가 StatusTree node를 저장하면 leaf 한 건을 변경할 때 32단계 ancestor node SSTORE가 필요합니다. 이번 POC는 다음처럼 역할을 나눕니다.

```text
Indexer:
  NoteStatusTree·VoucherStatusTree node와 path 유지

Contract:
  noteStatusRoot·voucherStatusRoot만 저장
```

Indexer가 제공한 path는 그대로 신뢰하지 않습니다. Participant나 Status Authority가 path를 witness로 사용하고 Circuit이 온체인 root와의 관계를 검증합니다. 거짓·오래된 path는 root 불일치로 proof를 만들 수 없거나 검증에 실패합니다.

Indexer가 잘못된 path를 제공하면 안전하게 실패하지만 path 제공을 거부하면 proof 생성이 지연될 수 있습니다. 따라서 Indexer는 correctness가 아니라 availability에 영향을 줍니다.

POC에서는 Go 자료구조가 Indexer 역할을 하며 HTTP API, DB, daemon과 chain reorganization 복구는 구현하지 않습니다.

## 6. 왜 StatusUpdate도 ZKP로 검증하나요?

Status leaf 한 건을 바꿀 때 old root와 new root를 모두 확인해야 합니다.

```text
oldStatus + path → oldRoot   // Poseidon2 32회
newStatus + path → newRoot   // Poseidon2 32회
```

EVM에는 현재 POC가 사용하는 BLS12-381 Poseidon2 전용 precompile이 없습니다. Solidity에서 64회의 Poseidon2 compression을 직접 실행하지 않고, Status Authority가 이 계산을 Circuit에서 수행해 proof를 제출합니다.

```text
Status Authority:
  Merkle 계산 + PLONK proof 생성

Contract:
  current oldRoot 제공
  proof 검증
  newRoot SSTORE 한 번
```

Status update path는 proof의 private witness이며 calldata에 넣지 않습니다. 이 선택의 주된 목적은 Path Privacy가 아니라 온체인 Poseidon2 계산을 Status Authority의 오프체인 proving으로 옮기는 것입니다.

### [폐기] 직접 온체인 Merkle update

Contract가 path를 받아 old·new root를 직접 계산하는 경로는 구현하지 않습니다.

### [폐기] Batch와 Merkle multiproof

M6는 transaction 하나에서 object 하나만 변경합니다. 여러 대상은 현재 root 순서에 맞춰 개별 transaction으로 처리합니다.

## 7. 누가 무엇을 공개하나요?

### Status 변경

Status Authority의 update transaction에서는 다음이 공개됩니다.

```text
object type
cm 또는 rv
기존 insertion index
oldStatus와 newStatus
oldRoot와 newRoot
StatusChanged Event
```

동결·철회 대상과 상태 전이는 공개 집행 정보입니다. Status path만 proof witness로 숨깁니다.

### 정상 소비

```text
공개:
  membership root
  current Status root
  nullifier와 output

비공개:
  소비 cm·rv
  기존 index
  membership path
  Status path
```

따라서 어떤 객체가 Frozen·Revoked됐는지는 공개되지만, 이후의 private 소비 transaction이 과거 어떤 `cm`·`rv`와 연결되는지는 proof가 직접 공개하지 않습니다. 상태 변경 직후 발생한 transaction을 이용한 timing 추론은 범위 밖입니다.

## 8. 역할과 신뢰 경계

| 역할 | 책임 | 신뢰·한계 |
|---|---|---|
| Participant | Active path를 포함한 Status-aware Event proof 생성 | 최신 Indexer path가 필요합니다. |
| Status Authority | 조사 대상을 선택하고 StatusUpdate proof·transaction 생성 | 잘못된 대상을 동결·철회할 수 있는 신뢰 주체입니다. |
| Indexer | Tree 재구성, path 제공, 온체인 root 대조 | 거짓 path는 검증 실패하지만 서비스 거부는 가능합니다. |
| Contract | 권한·대상/index·proof·현재 root 검증 후 new root 저장 | object의 private 소비 이력을 역으로 해석하지 않습니다. |

Status Authority는 Contract 배포 시 immutable EVM account로 고정하며 System Admin이 변경하지 못합니다.

## 9. 소비된 객체와 M7 Audit의 관계

Contract는 `cm`·`rv`가 생성됐다는 것은 알지만, private nullifier로 소비됐기 때문에 공개 object ID만으로 이미 소비됐는지 역조회할 수 없습니다.

M6 운영에서는 Status Authority가 외부 조사 결과를 이용해 live Note 또는 unresolved Voucher만 상태 변경 대상으로 선택합니다. 소비된 객체에는 Status update를 요청하지 않습니다.

이 조건은 M6 Contract가 암호학적으로 강제하지 않습니다. M7에서 성공 Event의 AuditRecord와 provenance를 추가하면 Authority가 문제 transaction과 live downstream 객체를 식별하는 근거가 보강됩니다.

## 10. M6 이후 Main Protocol

기존 `EntryExitLedger`와 M1~M5 Circuit은 이전 building block과 성능 baseline을 재현하기 위해 보존합니다.

```text
M1~M5:
  Commitment·private spend·Event·Policy의 독립 검증 단계

M6 이후:
  ZkDPPStatusLedger
  + Status-aware 소비 Circuit
  + StatusUpdate Circuit
```

`ZkDPPStatusLedger`는 Status 검사가 없는 소비 verifier나 우회 함수를 제공하지 않습니다. M6는 migration 실험이 아니라 fresh POC deployment이며, 최종 M9 통합은 M6 Main Protocol 경로를 기준으로 합니다.

## 11. 이번 단계의 비범위

- Batch Status update와 Merkle multiproof
- 온체인 StatusTree node·Path 저장
- Contract의 직접 Poseidon2 Merkle update
- 별도 Status index와 `Hash(cm)` 기반 위치
- HTTP Indexer·DB·daemon·reorg 처리
- Claim Status
- 소비된 객체의 공개 liveness 역조회
- downstream leaf 자동 탐색
- 악의적인 Status Authority의 cryptographic accountability
- Status Authority 교체·multisig
- accumulator·온라인 Active 서명·FHE·TEE

