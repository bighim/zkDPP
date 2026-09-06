# M6 Status 집행 구현 명세

- 명세 상태: 구현 기준 동결
- 이전 단계: [M5 Process·Policy Result](M5-process-policy-result.md)
- 의사결정 배경: [M6 Status 집행 Background](M6-status-enforcement-background.md)
- 미래 Context: [M6 — Active·Frozen·Revoked 상태 집행](FUTURE-MILESTONE-CONTEXT.md#m6)
- 구현 결과: [M6 Status 집행 Result](M6-status-enforcement-result.md)

이 문서는 M6의 자체 완결형 구현 기준입니다. Background나 이전 대화를 읽지 않아도 객체·공개범위·Circuit·Contract·Indexer·Gate를 이해하고 구현할 수 있어야 합니다.

## 0. 30초 안에 설계 파악하기

```text
M1~M5:
  Main Protocol을 위한 commitment·private spend·Event·Policy building block 검증

M6 Main Protocol:
  ZkDPPStatusLedger
  + Status-aware 소비 Circuit
  + StatusUpdate Circuit
```

```text
Note Tree index i
  ↔ NoteStatusTree index i

Voucher Tree index j
  ↔ VoucherStatusTree index j
```

| 질문 | M6 결정 |
|---|---|
| Active는 어떻게 표현합니까? | 같은 object index의 Status leaf가 empty `0`임을 증명합니다. |
| StatusTree는 어디에 있습니까? | 전체 node와 path는 오프체인 Indexer, 최신 root만 Contract에 있습니다. |
| 누가 Status를 변경합니까? | 배포 시 고정한 immutable Status Authority입니다. |
| Contract가 Merkle path를 계산합니까? | 계산하지 않고 Status Authority의 StatusUpdate proof를 검증합니다. |
| 소비 시 무엇을 숨깁니까? | `cm`·`rv`, 기존 index, membership path와 Status path입니다. |
| 과거 Status root를 허용합니까? | 허용하지 않고 Contract storage의 현재 root만 verifier에 넣습니다. |
| 여러 leaf를 한 번에 바꿉니까? | Batch를 지원하지 않고 transaction 하나당 leaf 하나만 변경합니다. |
| 기존 Circuit은 어떻게 됩니까? | M1~M5 baseline으로 보존하고 M6 Status-aware Circuit이 Main Protocol이 됩니다. |

## 1. M5에서 무엇이 달라지나요?

| M5까지 가능한 것 | M6에서 추가되는 것 |
|---|---|
| private Note·Voucher membership | 같은 private index의 Active 상태 증명 |
| nullifier로 중복 소비 방지 | Frozen·Revoked 객체의 최초 소비도 차단 |
| Policy 승인 Process | Status-aware Process Policy verifier |
| `EntryExitLedger` building-block Contract | `ZkDPPStatusLedger` Main Protocol Contract |
| 상태 없는 output | 생성 시 별도 write 없이 기본 Active로 해석 |

M6는 기존 nullifier를 대체하지 않습니다. Nullifier는 이미 소비됐는지를 검사하고 Status는 아직 소비되지 않은 객체를 일시 정지하거나 영구 철회합니다.

```text
Nullifier:
  이미 소비했는가?

Status:
  아직 소비할 수 있는가?
```

## 2. 역할과 신뢰 경계

| 역할 | Identity | 책임 |
|---|---|---|
| System Admin | M6 Contract deployer EVM account | Entry Issuer·Policy Authority 관리, Status Authority 변경 권한 없음 |
| Status Authority | constructor에서 고정한 EVM account | live 대상 선택, StatusUpdate proof 생성·제출 |
| Participant | private `sk_owner` | 최신 Active path를 포함한 Event proof 생성 |
| Policy Authority·Certifier | 등록된 EVM account | M6 Status-aware Process Policy·VK 등록 |
| Indexer | POC의 오프체인 Go 객체 | Status Event replay, Tree·path 유지, root 대조 |
| Transaction submitter | 임의 EVM account | Participant proof 제출 가능 |

`statusAuthority`는 zero address를 거부하고 배포 후 교체·해제·multisig 전환을 지원하지 않습니다. Status Authority가 올바른 live 객체를 선택하고 Policy Authority가 Status 검사가 포함된 Process verifier를 등록한다는 것은 M6 신뢰 가정입니다.

## 3. Status 객체·상수·Tree

### ObjectType

```text
NOTE    = 1
VOUCHER = 2
```

Claim은 M8에서 객체와 검증 흐름을 확정한 뒤 추가합니다.

### Status

```text
ACTIVE  = 0
FROZEN  = 1
REVOKED = 2
```

`ACTIVE=0`은 Sparse Merkle Tree의 empty leaf입니다. Status leaf에 `cm`, `rv` 또는 index를 다시 Hash하지 않습니다. object ID와 index의 관계는 기존 Note·Voucher Tree가, 같은 index 사용은 Circuit과 Contract가 강제합니다.

### 허용 전이

| 이전 | 다음 | 의미 |
|---|---|---|
| Active | Frozen | 조사 중 임시 사용 금지 |
| Frozen | Active | 조사 종료 후 사용 재개 |
| Frozen | Revoked | 문제 확정 후 영구 사용 금지 |

다른 모든 전이는 실패합니다. `Revoked`는 terminal입니다.

### Empty root

두 StatusTree의 depth는 기존 object Tree와 같은 32입니다.

$$
z_0=0
$$

$$
z_{\ell+1}=\operatorname{Poseidon2}(z_\ell,z_\ell)
$$

초기값:

```text
noteStatusRoot    = z_32
voucherStatusRoot = z_32
```

Note·Voucher output이 생성돼도 StatusTree에는 Active leaf를 쓰지 않으므로 root는 바뀌지 않습니다.

## 4. 기존 object index를 공유하는 관계

### Note

```text
Note Tree:
  leaf[index] = cm

NoteStatusTree:
  leaf[index] = 0 | 1 | 2
```

### Voucher

```text
Voucher Tree:
  leaf[index] = rv

VoucherStatusTree:
  leaf[index] = 0 | 1 | 2
```

소비 Circuit은 기존 `MerklePath.Index`를 32 bit로 분해해 두 Path의 좌우 순서에 함께 사용합니다. StatusPath에는 별도 index를 두지 않습니다.

```text
# 핵심: 같은 private index가 객체 존재와 Active 상태를 연결합니다.

cm = HashNote(privateNote)

AssertMembership(
    noteRoot,
    cm,
    privateNotePath.Index,
    privateNotePath.Siblings
)

AssertStatusActive(
    noteStatusRoot,
    privateNotePath.Index,
    privateStatusSiblings
)
```

Status update에서는 object ID와 index가 공개됩니다. Contract는 기존 object Tree의 실제 leaf를 조회해 다음을 검사합니다.

```text
NOTE:
  index < noteLeafCount
  noteTreeLeaf(index) == cm

VOUCHER:
  index < voucherLeafCount
  voucherTreeLeaf(index) == rv
```

단순히 `commitments[cm]` 또는 `voucherCommitments[rv]`만 확인해서는 index binding이 성립하지 않으므로 leaf equality를 반드시 검사합니다.

## 5. 오프체인 Status Indexer

### 구현 범위

POC Indexer는 다음 두 Go Tree 인스턴스입니다.

```text
noteStatusTree
voucherStatusTree
```

최소 Interface:

```text
Root() Element
Status(index uint64) Status
Path(index uint64) StatusPath
Apply(index uint64, oldStatus, newStatus Status) (newRoot, error)
Replay(event StatusChanged) error
```

StatusPath:

```text
StatusPath:
  Siblings[32]
```

별도 index를 갖지 않습니다. 호출자가 기존 Note·Voucher index를 제공합니다.

### 저장 방식

- 기본 leaf는 `0`입니다.
- nonzero exception leaf와 필요한 중간 node만 저장합니다.
- 존재하지 않는 node는 level별 zero Hash로 계산합니다.
- Note와 Voucher 생성 Event는 object ID와 index 목록을 복구하는 데 사용하지만 Status root를 변경하지 않습니다.
- `StatusChanged` transaction이 확정된 뒤 로컬 Tree에 같은 전이를 적용합니다.
- 로컬 계산 root와 Event의 `newRoot`가 다르면 동기화 실패로 처리합니다.

### Indexer 수도 코드

```text
function PrepareStatusUpdate(objectType, index, oldStatus, newStatus):

    # 핵심: 현재 확정 root를 기준으로 Path와 변경 후보를 계산합니다.

    # 1. object type에 맞는 Tree를 선택합니다.
    tree = SelectStatusTree(objectType)

    # 2. 로컬 현재 상태가 요청과 일치하는지 확인합니다.
    assert tree.Status(index) == oldStatus

    # 3. 현재 root의 private witness로 사용할 Path를 만듭니다.
    siblings = tree.Path(index)
    oldRoot = tree.Root()

    # 4. 같은 Path에서 newStatus를 적용한 후보 root를 계산합니다.
    newRoot = ComputeRoot(newStatus, index, siblings)

    # 5. 아직 로컬 canonical Tree는 변경하지 않습니다.
    return oldRoot, newRoot, siblings
```

```text
function ConfirmStatusChanged(event):

    # 핵심: 온체인에서 확정된 Event만 로컬 Tree에 반영합니다.

    tree = SelectStatusTree(event.objectType)

    appliedRoot = tree.Apply(
        event.index,
        event.oldStatus,
        event.newStatus
    )

    assert appliedRoot == event.newRoot
```

HTTP API, DB, daemon, 다중 Indexer 합의와 chain reorganization 복구는 구현하지 않습니다.

## 6. 공통 Active Status gadget

### Interface

```text
AssertStatusActive(
    api,
    statusRoot,
    existingObjectIndex,
    statusSiblings[32]
)
```

### Circuit 수도 코드

```text
function AssertStatusActive(statusRoot, index, siblings):

    # 핵심: 기존 object index 위치가 현재 StatusTree에서 empty인지 증명합니다.

    # 1. index를 32 bit로 분해하고 범위를 강제합니다.
    bits = ToBinary(index, 32)

    # 2. Active를 나타내는 empty leaf에서 시작합니다.
    current = ACTIVE  // 0

    # 3. index bit로 각 level의 좌우 순서를 결정합니다.
    for level in 0 .. 31:
        sibling = siblings[level]

        if bits[level] == 0:
            current = Poseidon2(current, sibling)
        else:
            current = Poseidon2(sibling, current)

    # 4. 계산 결과가 proof의 public Status root와 같아야 합니다.
    assert current == statusRoot
```

가짜 객체는 StatusTree에서 Active로 보일 수 있으므로 이 gadget만 독립적으로 소비 권한을 주지 않습니다. 반드시 같은 Circuit에서 실제 Note·Voucher membership과 소유권·nullifier를 함께 검증합니다.

## 7. StatusUpdate Circuit

### 목적

Status Authority가 private Path를 이용해 current Status root에서 leaf 하나만 허용된 상태로 바꿨음을 증명합니다. Contract는 Merkle Path를 받거나 Poseidon2 path를 직접 계산하지 않습니다.

### 공개·비공개값

Public input 순서:

```text
1. objectType
2. oldRoot
3. newRoot
4. index
5. oldStatus
6. newStatus
```

Private witness:

```text
statusSiblings[32]
```

`objectId`는 Circuit input이 아닙니다. Contract가 기존 object Tree의 `leaf[index]`와 직접 대조합니다.

### 전체 Circuit 수도 코드

```text
function DefineStatusUpdateCircuit():

    # 핵심: 같은 index·siblings에서 Status leaf 하나만 바뀌었음을 증명합니다.

    # 1. 공개 입력 범위를 검사합니다.
    assert objectType == NOTE or objectType == VOUCHER
    indexBits = ToBinary(index, 32)

    assert oldStatus == ACTIVE
        or oldStatus == FROZEN
        or oldStatus == REVOKED

    assert newStatus == ACTIVE
        or newStatus == FROZEN
        or newStatus == REVOKED

    # 2. 허용된 전이만 선택합니다.
    allowed =
        (oldStatus == ACTIVE and newStatus == FROZEN)
        or (oldStatus == FROZEN and newStatus == ACTIVE)
        or (oldStatus == FROZEN and newStatus == REVOKED)

    assert allowed

    # 3. oldStatus와 private siblings로 oldRoot를 계산합니다.
    calculatedOldRoot = oldStatus

    for level in 0 .. 31:
        calculatedOldRoot = CompressByIndexBit(
            calculatedOldRoot,
            statusSiblings[level],
            indexBits[level]
        )

    assert calculatedOldRoot == oldRoot

    # 4. 같은 index와 siblings로 newRoot를 계산합니다.
    calculatedNewRoot = newStatus

    for level in 0 .. 31:
        calculatedNewRoot = CompressByIndexBit(
            calculatedNewRoot,
            statusSiblings[level],
            indexBits[level]
        )

    assert calculatedNewRoot == newRoot
```

두 계산이 같은 `statusSiblings`와 `indexBits`를 사용하므로 다른 subtree를 변경할 수 없습니다.

## 8. Status-aware Main Protocol Circuit

### 구현 원칙

기존 M1~M5 Circuit·Artifact는 baseline 재현용으로 보존합니다. 기존 관계를 재사용 가능한 내부 함수로 최소 분리하되 기존 Circuit의 public input·constraints·결과를 바꾸지 않습니다.

새 Circuit:

```text
StatusPrivateSpend
StatusTransfer
StatusProceed
StatusRecall
StatusMerge
StatusSplit
StatusProcess
```

M6 이후 Main Protocol에서는 이 verifier만 사용합니다.

### 공개값·비공개값

| Circuit | Public input 순서 | 추가 private witness |
|---|---|---|
| StatusPrivateSpend | `noteRoot, noteStatusRoot, nf` | Status siblings 1개 |
| StatusTransfer | `noteRoot, noteStatusRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch` | Status siblings 1개 |
| StatusProceed | `voucherRoot, voucherStatusRoot, rvnf, cmReceiver` | Status siblings 1개 |
| StatusRecall | `voucherRoot, voucherStatusRoot, rvnf, cmReturn, currentEpoch` | Status siblings 1개 |
| StatusMerge | `noteRoot, noteStatusRoot, nf1, nf2, cmOut` | Status siblings 2개 |
| StatusSplit | `noteRoot, noteStatusRoot, nf, cmOut1, cmOut2` | Status siblings 1개 |
| StatusProcess | `policyRef, policyScopeRef, noteRoot, noteStatusRoot, nf1, nf2, nf3, cmEligible, cmWaste` | Status siblings 3개 |

각 Status siblings는 32개 field element입니다. 소비 `cm`·`rv`, object index와 두 종류의 Path는 모두 private입니다.

### Note 소비 공통 순서

```text
# 핵심: 기존 Note 소비 관계에 같은 index의 Active 증명을 추가합니다.

1. private Note의 State·AssetRole 조건 검증
2. sk_owner로 address·소유권 검증
3. private Note에서 cm 재계산
4. cm, NotePath.Index, NotePath.Siblings로 noteRoot membership 검증
5. 같은 NotePath.Index와 StatusSiblings로 noteStatusRoot Active 검증
6. sk_owner와 cm으로 public nf 검증
7. 각 Event의 기존 output·State·deadline·Policy 관계 검증
```

### Voucher 소비 공통 순서

```text
# 핵심: 기존 Voucher resolution 관계에 같은 index의 Active 증명을 추가합니다.

1. private Voucher의 State·AssetRole 조건 검증
2. Voucher payload에서 rv 재계산
3. rv, VoucherPath.Index, VoucherPath.Siblings로 voucherRoot membership 검증
4. 같은 VoucherPath.Index와 StatusSiblings로 voucherStatusRoot Active 검증
5. opening과 rv로 public rvnf 검증
6. Proceed는 receiver secret, Recall은 sender secret 검증
7. output Note와 Recall deadline 관계 검증
```

### Event별 상태 추가

| Event | 기존 관계 | M6 추가 |
|---|---|---|
| Exit | Note membership·소유권·nullifier | input Note Active 1개 |
| Transfer | Note 소비, Voucher·Change Note와 비례 배분 | input Note Active 1개 |
| Proceed | Voucher membership·receiver·rvnf | input Voucher Active 1개 |
| Recall | Voucher membership·sender·rvnf·deadline | input Voucher Active 1개 |
| Merge | 같은 owner·Role Note 2개와 State 합 | 두 input Note Active |
| Split | Note 소비와 output 2개 비례 배분 | input Note Active 1개 |
| Process | Policy·Scope·Note 3개·고정 allocation | 세 input Note Active |

Entry는 input 객체가 없으므로 기존 Entry Circuit·verifier를 그대로 사용하고 Status proof를 추가하지 않습니다. 모든 output은 별도 Status write 없이 기본 Active입니다.

## 9. `ZkDPPStatusLedger` Main Contract

### 배포와 기존 baseline

새 독립 Contract 이름은 다음과 같습니다.

```text
ZkDPPStatusLedger
```

기존 `EntryExitLedger`는 수정하지 않습니다. 상속이나 대규모 base-contract refactor 없이 M5의 Entry·Voucher·Tree·Policy Registry 의미를 M6 Contract에 포함합니다. M6는 fresh POC deployment이며 기존 Contract state migration은 구현하지 않습니다.

### Constructor

```text
constructor(
    statusAuthority,
    entryVerifier,
    statusPrivateSpendVerifier,
    statusTransferVerifier,
    statusProceedVerifier,
    statusRecallVerifier,
    statusMergeVerifier,
    statusSplitVerifier,
    statusProcessVerifier,
    statusUpdateVerifier,
    fieldHasher
)
```

- `statusAuthority`는 zero address를 거부하고 immutable로 저장합니다.
- verifier와 hasher는 code가 없는 address를 거부합니다.
- Note·Voucher Tree는 기존처럼 온체인 node를 저장합니다.
- Note·Voucher StatusTree는 node를 저장하지 않고 empty root만 초기화합니다.

### 추가 상태

```text
immutable statusAuthority
immutable statusUpdateVerifier
immutable status-aware verifier 7개

noteStatusRoot
voucherStatusRoot
```

Status root의 accepted-history mapping은 만들지 않습니다.

### Status-aware 소비 API

외부 함수 인자는 기존 M2~M5 API와 동일하게 유지합니다. Status root는 caller가 인자로 제출하지 않고 Contract storage에서 verifier public input에 삽입합니다.

```text
exit(proof, noteRoot, nf)

transfer(
  proof, noteRoot, nf,
  rvNew, cmChange, transferEpoch, deltaEpoch
)

proceed(proof, voucherRoot, rvnf, cmReceiver)

recall(proof, voucherRoot, rvnf, cmReturn, currentEpoch)

merge(proof, noteRoot, nf1, nf2, cmOut)

split(proof, noteRoot, nf, cmOut1, cmOut2)

process(
  proof, policyRef, policyScopeRef,
  noteRoot, nf[3], cmOut[2]
)
```

예를 들어 Exit verifier input은 Contract가 다음처럼 구성합니다.

```text
[
  submittedNoteRoot,
  storage.noteStatusRoot,
  submittedNf
]
```

따라서 Status root가 proof 생성 후 바뀌면 오래된 proof는 검증되지 않습니다.

### Process verifier 우회 방지

M6 POC는 exact 3-to-2 StatusProcess verifier 하나를 constructor에 고정합니다. Status Process용 PolicyRecord를 등록할 때 `verifierRef`와 `vkHash`가 이 M6 verifier와 일치해야 하며 `process`는 immutable StatusProcess verifier를 사용합니다.

기존 M5 legacy Process verifier를 PolicyRecord에 넣거나 Status 검사 없는 verifier를 호출할 수 없습니다. 새로운 복수 Policy의 Status-aware verifier 승인 방식은 M9 통합에서 확장합니다.

### Status update API

```text
updateStatus(
    proof,
    objectType,
    objectId,
    index,
    oldStatus,
    newStatus,
    newRoot
)
```

받지 않는 값:

```text
oldRoot      // Contract storage에서 읽음
statusPath   // Circuit private witness
```

### Contract 수도 코드

```text
function updateStatus(
    proof,
    objectType,
    objectId,
    index,
    oldStatus,
    newStatus,
    newRoot
):

    # 핵심: 공개 대상과 기존 index를 확인하고 ZKP가 증명한 root만 저장합니다.

    # 1. Status Authority 권한을 확인합니다.
    assert msg.sender == statusAuthority

    # 2. 대상 종류와 objectId-index binding을 확인합니다.
    if objectType == NOTE:
        assert index < noteLeafCount
        assert noteTreeLeaf(index) == objectId
        oldRoot = noteStatusRoot

    else if objectType == VOUCHER:
        assert index < voucherLeafCount
        assert voucherTreeLeaf(index) == objectId
        oldRoot = voucherStatusRoot

    else:
        fail InvalidObjectType

    # 3. Contract에서도 허용된 전이인지 확인합니다.
    assert IsAllowedStatusTransition(oldStatus, newStatus)

    # 4. storage current root를 oldRoot public input으로 사용합니다.
    publicInputs = [
        objectType,
        oldRoot,
        newRoot,
        index,
        oldStatus,
        newStatus
    ]

    # 5. Status Authority가 만든 root-update proof를 검증합니다.
    assert statusUpdateVerifier.Verify(proof, publicInputs)

    # 6. 대상 Tree root 하나만 변경합니다.
    if objectType == NOTE:
        noteStatusRoot = newRoot
    else:
        voucherStatusRoot = newRoot

    # 7. 독립 Indexer가 replay할 수 있는 Event를 남깁니다.
    emit StatusChanged(
        objectType,
        objectId,
        index,
        oldStatus,
        newStatus,
        newRoot
    )
```

### Event

```text
StatusChanged(
    objectType,
    objectId,
    index,
    oldStatus,
    newStatus,
    newRoot
)
```

Event에는 Indexer가 empty root부터 현재 Tree를 독립적으로 replay하는 데 필요한 정보가 모두 포함됩니다.

## 10. Canonical 시나리오

Anvil account 역할:

| Index | 역할 |
|---:|---|
| 0 | System Admin·deployer |
| 1~3 | Entry Issuer·일반 transaction submitter |
| 4 | 기존 권한 negative case |
| 5 | immutable Status Authority |
| 6 | 권한 없는 Status updater |

Canonical 흐름:

```text
1. Status-aware verifier, StatusUpdate verifier와 ZkDPPStatusLedger 배포
2. Note A·B·C Entry
3. 최신 empty NoteStatusRoot에서 Note A Exit 성공
4. Note B의 index와 Status path로 Active→Frozen proof 생성·제출
5. Indexer가 StatusChanged를 적용하고 root 일치 확인
6. Freeze 전 Note B proof를 제출해 current Status root 불일치로 실패
7. Frozen Note B에 empty Status path를 사용한 witness 생성 실패
8. Note B Frozen→Active proof 제출
9. 최신 root에서 Note B Transfer 성공
10. 생성한 Voucher V를 Active→Frozen
11. Voucher V Proceed와 Recall 모두 실패
12. Voucher V Frozen→Active 후 Recall 성공
13. Note C를 Active→Frozen→Revoked
14. Note C 소비와 추가 Status 전이 영구 실패
```

Merge·Split·Process는 별도 연결 fixture에서 모든 input이 Active일 때 성공하고 input 하나를 Frozen으로 바꾸면 전체 Event가 실패하도록 검증합니다.

## 11. Correctness Gate

### Native StatusTree

- empty root가 Go·Circuit fixture에서 일치합니다.
- Note·Voucher StatusTree가 같은 numeric index에서도 독립적입니다.
- Active→Frozen→Active와 Active→Frozen→Revoked root가 정확합니다.
- 잘못된 oldStatus·index·path와 terminal Revoked 전이가 실패합니다.
- Event replay 후 Indexer root가 expected on-chain root와 같습니다.

### Circuit

- StatusUpdate public variable 수가 정확히 6개입니다.
- old·new root가 같은 index·siblings로 계산됩니다.
- wrong objectType·index·old/new Status·path·root가 실패합니다.
- Status-aware Event의 consumed `cm`·`rv`, index와 두 Path가 public witness에 없습니다.
- 동일 index 공유를 깨거나 다른 Active 위치의 Status path를 사용하면 실패합니다.
- Status-aware Process는 세 input 각각을 검증합니다.
- 기존 State·owner·nullifier·deadline·Policy negative case도 그대로 실패합니다.

### Contract

- zero Status Authority와 code가 없는 verifier를 배포 시 거부합니다.
- account 6의 Status update를 거부합니다.
- object type·objectId·index 불일치와 존재하지 않는 index를 거부합니다.
- 잘못된·오래된 StatusUpdate proof와 임의 newRoot를 거부합니다.
- Status root history를 소비 proof에 사용할 수 없습니다.
- Frozen Note의 모든 소비 Event와 Frozen Voucher의 Proceed·Recall을 거부합니다.
- Unfreeze 후 최신 proof는 성공합니다.
- Revoked 객체의 소비와 모든 후속 전이를 거부합니다.
- legacy fixed verifier와 legacy Process verifier를 Main Contract에서 호출할 수 없습니다.
- 실패 transaction 후 nullifier·commitment·object Tree·Policy·양 Status root가 불변입니다.
- Batch update ABI가 존재하지 않습니다.

## 12. 구현 구조·Artifact·명령

예정 Feature:

```text
internal/core/status/
features/status_update/
features/status_private_spend/
features/status_transfer/
features/status_proceed/
features/status_recall/
features/status_merge/
features/status_split/
features/status_process/
internal/m6case/
contracts/src/ZkDPPStatusLedger.sol
```

M6 development artifact:

```text
artifacts/development/m6/
  status-update/
  status-private-spend/
  status-transfer/
  status-proceed/
  status-recall/
  status-merge/
  status-split/
  status-process/
```

각 Circuit은 CCS·development SRS·PK·VK·Solidity verifier와 checksum manifest를 가집니다. 기존 M1~M5 Artifact와 Raw JSON은 checksum만 확인하고 다시 생성하거나 덮어쓰지 않습니다.

예정 명령:

```text
make setup-m6
make test-go
make test-contract-m6
make benchmark-m6-gas
make benchmark-m6-e2e
make benchmark-m6
```

## 13. 측정 계획

공식 측정은 case별 한 번 실행합니다.

### Circuit

- StatusUpdate와 Status-aware Event 7개의 constraints·public inputs
- Compile·development SRS·Setup·witness·Prove·Verify
- binary proof·Solidity serialization·CCS·PK·VK 크기
- 기존 baseline과 Status-aware actual value·차이

### Status Authority

- Indexer Path 준비
- StatusUpdate witness
- Prove
- ABI·sign
- submit-to-receipt
- 전체 E2E

### Contract

- `ZkDPPStatusLedger`·verifier deployment gas
- Active→Frozen, Frozen→Active, Frozen→Revoked receipt gas
- StatusUpdate calldata bytes와 root SSTORE
- 7개 Status-aware Event receipt gas·calldata
- 입력 1·2·3개에 따른 Status proof overhead

직접 온체인 Poseidon2 path update와 Batch·multiproof는 구현하거나 측정하지 않습니다.

Raw 결과:

```text
output/m6-circuit.json
output/m6-status-update-gas.json
output/m6-anvil-gas.json
output/m6-anvil-e2e.json
output/m6-generated-checksums.json
```

## 14. 구현 단계와 완료 조건

```text
M6.1 Native StatusTree와 Active gadget
M6.2 StatusUpdate·Status-aware Circuit
M6.3 ZkDPPStatusLedger와 Foundry
M6.4 Indexer 연결 fixture와 Anvil
M6.5 최종 Gate·Result
```

구현·전체 correctness test·공식 단일 측정·Raw 결과 대조 후 [Result Template](RESULT-TEMPLATE.md)에 따라 `M6-status-enforcement-result.md`를 생성합니다. Result가 없으면 M6를 완료로 표시하지 않습니다.

## 15. 명시적 비범위

- Batch Status update·Merkle multiproof
- StatusTree node·Path 온체인 저장
- Contract의 직접 Poseidon2 Merkle update
- Status root history 수용
- 별도 Status index와 Hash 기반 위치
- HTTP Indexer·DB·daemon·reorg 복구
- Claim Status
- 소비된 object ID의 공개 liveness 역조회
- downstream leaf 자동 탐색
- Status Authority 교체·multisig·cryptographic accountability
- Production governance·final universal SRS·Besu
