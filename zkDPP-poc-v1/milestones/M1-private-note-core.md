# M1 Private Note Core 구현 명세

- 명세 상태: 구현 기준 동결
- Result: [`M1-private-note-core-result.md`](M1-private-note-core-result.md)
- Checklist: [`MILESTONE-CHECKLIST.md`](MILESTONE-CHECKLIST.md)

이 문서는 M1을 구현하는 데 필요한 데이터, 계산 순서, Circuit pseudo code, fixture, Artifact, test와 측정 Gate를 자체적으로 설명합니다.

## 0. 30초 안에 설계 파악하기

| 질문 | M1 결정 |
|---|---|
| 이전 상태 | zkDPP-poc-v1 구현이 없었습니다. |
| 목표 | 모든 공급망 Event가 공유할 private Note 생성·소비 관계를 검증합니다. |
| 새 기능 | Note Commitment, Private Spend Kernel, Depth-32 membership |
| 핵심 전이 | private Note → public `cm`; private Note·secret·path → public `noteRoot`, `nf` |
| 공개값 | Note Commitment는 `cm`; Private Spend는 `noteRoot`, `nf` |
| 비공개값 | Note 내용, `sk_owner`, 소비 `cm`, leaf index, siblings |
| 구현 | 일반 Go·gnark Circuit·개발용 PLONK Artifact·고정 actor fixture |
| 비범위 | Event, Solidity Contract, duplicate-nullifier 상태, Policy·Status·Audit·DPP |

## 1. 목적·원칙·Canonical 시나리오

M1은 Production 시스템이 아니라 다음 세 가지를 확인하는 POC입니다.

```text
구현 가능성
Correctness
Component 성능
```

현재 기능에 필요하지 않은 Event framework·Contract·Production key generation은 만들지 않습니다. 대신 Hash Domain, 숫자 범위, 공개·비공개 경계와 negative test는 생략하지 않습니다.

Canonical 시나리오:

```text
1. 고정 actor fixture를 읽고 address를 재검산합니다.
2. ProductName·LotID에서 DocumentHash를 만듭니다.
3. ELIGIBLE Note를 만들고 public cm을 계산합니다.
4. cm을 native Depth-32 Tree에 append합니다.
5. 현재 root·private index·sibling 32개를 준비합니다.
6. sk_owner로 address와 nf를 다시 계산합니다.
7. Private Spend Circuit이 Note·소유권·membership·nf를 검증합니다.
8. 별도의 WASTE Note 조건도 확인합니다.
```

## 2. 공통 데이터와 Encoding

### DocumentInfo·DocumentHash

```text
DocumentInfo = (ProductName, LotID)
DocumentHash = HashDocumentInfo(DocumentInfo)
```

각 문자열은 Unicode NFC로 정규화합니다.

```text
uint32 big-endian byte length || UTF-8 ProductName
uint32 big-endian byte length || UTF-8 LotID
```

길이 구분으로 `("ab","c")`와 `("a","bc")`를 구별합니다. 일반 Go에서 BLS12-381 hash-to-field로 DocumentHash를 계산하고 Circuit은 문자열 대신 DocumentHash를 witness로 사용합니다.

### State

$$
\mathrm{State}=(q_{\mathrm{mass}},a_{\mathrm{rec}},e)
$$

| 값 | 의미 | 저장 형식 |
|---|---|---|
| `q_mass` | 전체 질량 | kg × $10^9$, uint64 |
| `a_rec` | 재활용 mass-balance credit 절대 질량 | kg × $10^9$, uint64 |
| `e` | Entry 이전을 포함할 수 있는 누적 탄소 | kgCO2e × $10^9$, uint64 |

```text
MassScale   = 1,000,000,000
CarbonScale = 1,000,000,000
```

두 상수 값은 같지만 물리 단위가 다릅니다. 분배 비율용 `AllocationDenominator`와도 별개입니다. M1은 이미 scale이 적용된 uint64를 직접 입력받고 범용 실수·문자열 변환기는 만들지 않습니다.

모든 Note:

$$
0\leq a_{\mathrm{rec}}\leq q_{\mathrm{mass}}
$$

### AssetRole

```go
type AssetRole uint8

const (
    AssetRoleEligible AssetRole = iota
    AssetRoleWaste
)
```

| Role | 의미 | 추가 조건 |
|---|---|---|
| ELIGIBLE | 후속 공급망 Event에 사용 가능 | 기본 State 조건 |
| WASTE | Exit만 가능한 terminal role | `a_rec=0`, `e=0` |

새 Role 추가는 단순 설정이 아니라 commitment와 Policy 의미를 바꾸는 Protocol 변경입니다.

### 소유자와 opening

$$
\mathrm{address}=H(\mathrm{OwnerTag},\mathrm{sk}_{\mathrm{owner}})
$$

`address`는 Ethereum EOA가 아닌 BLS12-381 field의 ZK 소유자 식별값입니다. PublicKey 계층은 사용하지 않습니다. `opening`은 같은 Document·State·소유자도 서로 다른 commitment로 만드는 비공개 field 값입니다.

### Domain

```text
NoteTag       = "zkDPP:Note:v1"
OwnerTag      = "zkDPP:Owner:v1"
NullifierTag = "zkDPP:Nullifier:v1"
```

Domain 문자열은 일반 Go에서 BLS12-381 hash-to-field로 고정 field constant가 됩니다. Domain은 public input이나 private witness가 아닙니다.

## 3. 고정 테스트 계정과 Native Tree

모든 milestone은 [`../testdata/common/actors-v1.json`](../testdata/common/actors-v1.json)을 테스트 identity의 유일한 원본으로 사용합니다.

```text
actor-1: sk_owner 11
actor-2: sk_owner 22
actor-3: sk_owner 33
actor-4: sk_owner 44
actor-5: sk_owner 55
```

JSON에는 각 `address=H(OwnerTag,sk_owner)` 결과가 함께 들어 있습니다. Loader는 다음을 한 번에 확인하고 하나라도 실패하면 전체 fixture를 거부합니다.

- profile·curve·Hash·OwnerTag metadata
- canonical 10진 field encoding
- 0이 아닌 secret
- ID·secret·address 중복
- 저장 address와 재계산 address 일치

이 secret은 공개 로컬 실험값이며 Production·공개 네트워크에 사용하지 않습니다.

### Native Depth-32 Tree

```text
Append(cm):
    # 핵심: 새 commitment를 leaf로 넣고 root까지 한 경로만 갱신합니다.

    # 1. 새 leaf를 다음 index에 기록합니다.
    leaf index에 cm을 기록합니다.

    # 2. 아래에서 위로 parent를 계산합니다.
    level 0..31에서 left·right를 Poseidon2Compress합니다.

    # 3. 계산한 node와 최신 root를 저장합니다.
    parent node와 새 root를 갱신합니다.

Path(index):
    # 핵심: 현재 root를 재현하는 데 필요한 sibling만 반환합니다.
    현재 Tree의 sibling 32개와 index를 반환합니다.

Verify(root, cm, path):
    # 핵심: cm에서 시작해 path를 따라 public root를 재계산합니다.
    index bit 순서로 compression 32회를 수행합니다.
    계산한 root와 입력 root를 비교합니다.
```

빈 subtree는 level별 zero Hash로 계산합니다.

## 4. Note Commitment

### 목적과 관계

Note의 실제 내용을 공개하지 않고 하나의 field commitment로 묶습니다.

$$
cm=H(\mathrm{NoteTag},\mathrm{DocumentHash},\mathrm{AssetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{address},\mathrm{opening})
$$

Hash 입력 순서는 고정이며 일반 Go와 Circuit이 동일해야 합니다.

### 공개·비공개값

| 공개값 | 비공개 witness |
|---|---|
| `cm` | DocumentHash, AssetRole, `q_mass`, `a_rec`, `e`, address, opening |

### Native 수도 코드

```text
NoteCommitment(Note):
    # 핵심: Note의 모든 필드를 고정 순서로 Hash해 cm 하나로 묶습니다.

    # 1. State와 Role이 Protocol 조건을 만족하는지 확인합니다.
    ValidateState(Note.State, Note.AssetRole)

    # 2. Domain Tag부터 opening까지 순서를 바꾸지 않고 Hash합니다.
    return Poseidon2Hash(
        NoteTag,
        Note.DocumentHash,
        Note.AssetRole,
        Note.q_mass,
        Note.a_rec,
        Note.e,
        Note.address,
        Note.opening
    )
```

### Circuit 수도 코드

```text
NoteCommitmentCircuit(public cm, private Note):

    # 핵심: 숨겨진 Note가 올바르고 public cm의 원문임을 증명합니다.

    # 1. 세 State가 64-bit 범위인지 확인합니다.
    ToBinary(Note.q_mass, 64)
    ToBinary(Note.a_rec, 64)
    ToBinary(Note.e, 64)

    # 2. Note 공통 State·Role 조건을 확인합니다.
    Assert(Note.a_rec <= Note.q_mass)
    Assert(Note.AssetRole == ELIGIBLE or WASTE)

    # 3. WASTE에는 credit과 탄소를 귀속하지 않습니다.
    if Note.AssetRole == WASTE:
        Assert(Note.a_rec == 0)
        Assert(Note.e == 0)

    # 4. 숨겨진 Note로 commitment를 다시 계산합니다.
    expectedCM = Poseidon2Hash(
        NoteTag,
        Note.DocumentHash,
        Note.AssetRole,
        Note.q_mass,
        Note.a_rec,
        Note.e,
        Note.address,
        Note.opening
    )

    # 5. 재계산한 값이 공개된 cm과 같은지 확인합니다.
    Assert(expectedCM == cm)
```

### 구현 위치·실패 조건·측정

```text
features/note_commitment/
  README.md
  circuit.go
  circuit_test.go
```

실패해야 하는 경우:

- State uint64 초과
- `a_rec>q_mass`
- 정의되지 않은 Role
- WASTE의 non-zero `a_rec`·`e`
- 변조된 DocumentHash·State·address·opening·public cm

측정: constraints, public inputs, Compile·Setup·Prove·Verify, binary proof, CCS·SRS·PK·VK 크기입니다. Tree append·Contract·EVM은 포함하지 않습니다.

## 5. Private Spend Kernel

### 목적과 관계

어떤 `cm`을 소비하는지 공개하지 않고 다음 사실을 함께 증명합니다.

```text
유효한 Note를 알고 있음
Note의 sk_owner를 알고 있음
Note가 public noteRoot에 포함됨
올바른 public nf를 생성함
```

$$
nf=H(\mathrm{NullifierTag},\mathrm{sk}_{\mathrm{owner}},cm)
$$

### 공개·비공개값

| 공개값 | 비공개 witness |
|---|---|
| `noteRoot`, `nf` | Note 전체, `sk_owner`, 소비 `cm`, leaf index, sibling 32개 |

### Native 수도 코드

```text
PrivateSpend(Note, sk_owner, path):

    # 핵심: Note를 공개하지 않고 소유권·Tree 포함·nullifier를 확인합니다.

    # 1. 소비하려는 Note 자체가 유효한지 확인합니다.
    ValidateState(Note.State, Note.AssetRole)

    # 2. secret으로 소유자 address를 다시 만들어 비교합니다.
    expectedAddress = H(OwnerTag, sk_owner)
    Require(expectedAddress == Note.address)

    # 3. Note commitment를 복원하고 현재 Tree에 있는지 확인합니다.
    cm = NoteCommitment(Note)
    Require(MerkleVerify(noteRoot, cm, path))

    # 4. 같은 Note의 재사용을 식별할 nullifier를 계산합니다.
    expectedNF = H(NullifierTag, sk_owner, cm)
    Require(expectedNF == public nf)
```

### Circuit 수도 코드

```text
PrivateSpendCircuit(
    public noteRoot,
    public nf,
    private Note,
    private sk_owner,
    private leafIndex,
    private siblings[32]
):

    # 핵심: 어떤 cm을 소비하는지 숨긴 채 유효한 Note 소비를 증명합니다.

    # 1. Note의 숫자 범위·credit·Role·WASTE 조건을 확인합니다.
    Note의 uint64·a_rec·Role·WASTE 조건을 확인합니다.

    # 2. sk_owner가 Note에 binding된 소유자 secret인지 확인합니다.
    expectedAddress = Poseidon2Hash(OwnerTag, sk_owner)
    Assert(expectedAddress == Note.address)

    # 3. 숨겨진 Note에서 소비 commitment를 다시 계산합니다.
    cm = NoteCommitment(Note)

    # 4. private index·siblings로 public noteRoot를 재계산합니다.
    bits = ToBinary(leafIndex, 32)
    calculatedRoot = cm

    for level = 0..31:
        sibling = siblings[level]

        if bits[level] == 0:
            calculatedRoot = Compress(calculatedRoot, sibling)
        else:
            calculatedRoot = Compress(sibling, calculatedRoot)

    # 5. 계산된 root가 공개 statement와 같은지 확인합니다.
    Assert(calculatedRoot == noteRoot)

    # 6. secret과 private cm에 binding된 public nf를 확인합니다.
    expectedNF = Poseidon2Hash(NullifierTag, sk_owner, cm)
    Assert(expectedNF == nf)
```

### 구현 위치·실패 조건·측정

```text
features/private_spend/
  README.md
  circuit.go
  circuit_test.go
```

실패해야 하는 경우:

- 잘못된 Note State·Role·address·opening
- 잘못된 `sk_owner`
- 변조된 leaf index·sibling·root
- 잘못된 public nullifier
- public input에 소비 `cm` 또는 index가 추가된 회로

Circuit은 같은 `nf`가 과거에 사용됐는지 알 수 없습니다. Contract duplicate-nullifier mapping은 M2에서 구현합니다.

측정: constraints, public inputs, witness·Prove·Verify, proof·artifact 크기입니다. Contract root lookup·SSTORE·transaction E2E는 포함하지 않습니다.

## 6. 공통 구현 구조와 재사용

```text
internal/core/
  hash/       Poseidon2·Domain
  document/   DocumentInfo encoding·hash
  owner/      sk_owner·address
  note/       State·Role·commitment·nullifier
  merkle/     native Depth-32 Tree

internal/circuitutil/
  Note validation·Poseidon2·membership gadgets

internal/testkit/
  actor fixture loader

internal/artifact/
  compile·SRS·Setup·serialization·checksum
```

M1에서 확인할 재사용 후보:

- DocumentInfo encoding
- State·Role validation
- owner address
- Note commitment
- nullifier
- Depth-32 membership
- Artifact runtime

실제 두 번째 소비자에서 계산·encoding·Domain·공개범위·실패 조건이 같을 때만 [`../REUSE.md`](../REUSE.md)의 `재사용 확정`으로 승격합니다.

## 7. SRS·Artifact·명령

### 개발 단계

```text
Circuit compile
  → unsafekzg.NewSRS
  → canonical·Lagrange SRS
  → plonk.Setup
  → PK·VK
  → Prove·Verify
```

```text
artifacts/development/m1/note-commitment/
artifacts/development/m1/private-spend/

각 폴더:
  ccs.bin
  srs-canonical.bin
  srs-lagrange.bin
  proving.key
  verifying.key
  manifest.json
```

Manifest는 relation, curve, proof system, constraints, public inputs, 시간, 파일 크기와 SHA-256을 기록합니다. 개발 Artifact는 Git과 Production에서 제외합니다.

### 최종 통합과의 관계

M1 SRS는 최종 universal SRS가 아닙니다. 모든 Policy Circuit이 완성된 뒤 최대 domain을 정하고 universal canonical SRS, domain별 Lagrange SRS와 Policy별 PK·VK를 생성합니다.

### 명령

```text
make test-go
make setup-m1
make evaluate-m1
```

Output:

```text
output/m1-core.json
milestones/M1-private-note-core-result.md
```

## 8. 통합 Correctness·측정·완료 Gate

### Correctness

- actor fixture valid·invalid case
- NFC equivalent DocumentInfo와 length-delimited collision 방지
- 일반 Go와 Circuit의 address·cm·nf·root 일치
- Note Commitment와 Private Spend positive·negative case
- public input 수와 Privacy boundary 확인
- 저장 Artifact checksum·reload·PLONK Verify

### 측정

| Feature | 측정 경계 | 횟수 |
|---|---|---:|
| Note Commitment | Compile·Setup·Prove·Verify·artifact bytes | 1회 |
| Private Spend | Compile·Setup·Prove·Verify·artifact bytes | 1회 |

환경, Go·gnark version, CPU, GOMAXPROCS와 run count를 Raw JSON에 기록합니다.

### 비범위

- Entry·Exit·Transfer 등 Event relation
- Solidity Contract와 duplicate-nullifier state
- Policy Registry·Status·AuditRecord·Claim
- Production key generation·trusted setup
- 미래 Event를 위한 범용 framework

### 완료 조건

1. 일반 Go·Circuit positive·negative test 통과
2. 공개·비공개 경계 확인
3. 실제 PLONK Setup·Prove·Verify 성공
4. Artifact checksum·재현 명령 기록
5. [`RESULT-TEMPLATE.md`](RESULT-TEMPLATE.md)에 따른 Result 생성
