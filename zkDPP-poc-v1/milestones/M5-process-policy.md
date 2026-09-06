# M5 Process·Policy 구현 명세

- 명세 상태: 구현 기준 동결
- 이전 단계: [M4 Merge·Split Result](M4-merge-split-result.md)
- 의사결정 배경: [M5 Process·Policy Background](M5-process-policy-background.md)
- 미래 Context: [M5 — Process와 Policy Circuit](FUTURE-MILESTONE-CONTEXT.md#m5)
- 구현 결과: [M5 Process·Policy Result](M5-process-policy-result.md)

이 문서는 M5의 자체 완결형 구현 기준입니다. Background를 읽지 않아도 구현할 수 있도록 객체·수식·공개범위·수도 코드·Contract API·Gate를 모두 포함합니다.

## 0. 30초 안에 설계 파악하기

```text
Policy 등록:
  Admin이 Authority 등록
  → Authority가 Policy ID·version 예약
  → policyRef 확정
  → Factory가 scopeRef 계산
  → Certifier가 exact 3-to-2 Circuit Setup
  → PolicyRecord·Grant 등록

Process 실행:
  private Note 3개
  → 비율 기반 손실·탄소 추가
  → ELIGIBLE Note + WASTE Note
  → Policy verifier로 proof 검증
```

| 질문 | M5 결정 |
|---|---|
| Policy는 무엇입니까? | Certifier가 승인하고 Circuit constant로 고정한 공정 규칙입니다. |
| 누가 Policy를 등록합니까? | 등록된 Policy Authority EVM account입니다. |
| Factory 권한은 무엇으로 확인합니까? | 같은 `sk_owner`로 만든 public `policyScopeRef`와 PolicyGrant입니다. |
| Process 크기는 얼마입니까? | exact 3-input 2-output입니다. |
| Product Type을 검사합니까? | 검사하지 않고 Certifier의 외부 검토에 둡니다. |
| VK는 재사용됩니까? | 같은 Policy version의 모든 Process에서 재사용됩니다. |
| 공식 verifier 방식은 무엇입니까? | Policy별 constant verifier Contract입니다. |
| Storage VK도 구현합니까? | 같은 optimized VK를 사용하는 architecture ablation으로 구현·측정합니다. |

## 1. M4에서 무엇이 달라지나요?

| M4에서 이어받는 기능 | M5 사용 |
|---|---|
| 같은 root의 private Note 2개 소비 | 같은 root의 private Note 3개 소비 |
| checked uint64 State 합 | 세 input의 단계별 checked sum |
| constant 비율·floor·residual | Process loss·carbon·allocation 계산 |
| 다중 output atomic insert | ELIGIBLE·WASTE Note 2개 append |
| Policy 없는 고정 verifier | PolicyRecord가 선택하는 verifier |

M5는 처음으로 Policy Authority·PolicyRef·ScopeRef·Grant·Policy lifecycle을 구현합니다. Status·Audit·Issue는 이후 milestone입니다.

**왜 이렇게 정했나요?** M1~M4는 ZK 객체 전이 자체를 검증했습니다. M5는 같은 전이라도 Certifier가 승인한 공정 규칙과 Factory 사용 권한을 함께 강제합니다. 상세 배경은 [Policy 객체의 역할](M5-process-policy-background.md#0-30초-안에-context-복구하기)에 있습니다.

## 2. 역할과 canonical interaction

### 역할

| 역할 | Identity | 책임 |
|---|---|---|
| System Admin | EVM deployer account | Policy Authority 등록 |
| Policy Authority·Certifier | EVM account + `authorityId` | Policy ID 예약, 근거 검토, Setup, Record·Grant·Disable |
| Operator·Factory | private `sk_owner` | ScopeRef 계산, input 소유권·Process proof 생성 |
| Transaction submitter | 임의 EVM account | 고정된 proof·public input 제출 가능 |

Entry Issuer와 Policy Authority는 공개 책임 주체이므로 EVM account를 사용합니다. Operator의 EVM account는 Factory identity로 사용하지 않습니다.

### 등록 interaction

```text
# 핵심: policyRef가 scope와 Circuit보다 먼저 확정됩니다.
1. Factory가 Process 규칙·근거를 Authority에 제출합니다.
2. Authority가 policyId·version을 예약하고 policyRef를 공개합니다.
3. Factory가 H(ScopeRefTag, sk_owner, policyRef)를 계산해 전달합니다.
4. Certifier가 policyRef에 binding된 Circuit을 Setup합니다.
5. Authority가 PolicyRecord와 최초 PolicyGrant를 등록합니다.
6. Factory가 같은 Policy로 Process proof를 반복 생성합니다.
```

Authority는 Factory의 `sk_owner`를 알지 못합니다. 잘못된 scopeRef를 제출한 Factory는 이후 Circuit의 scope relation을 만족할 수 없습니다. 별도 scope 등록 proof는 만들지 않습니다.

**왜 interaction이 한 번 더 필요합니까?** ScopeRef에 policyRef가 포함돼 cross-policy linking을 줄이기 때문입니다. 이 추가 왕복은 Policy 등록·version 변경에만 필요합니다. 상세 배경은 [등록 Interaction](M5-process-policy-background.md#3-policy-등록-interaction이-왜-두-단계인가요)에 있습니다.

## 3. Domain·encoding·Policy identity

### Domain

```text
PolicyRefTag = "zkDPP:PolicyRef:v1"
ScopeRefTag  = "zkDPP:ScopeRef:v1"
```

문자열은 기존 Domain과 같은 SHA-256 기반 hash-to-field로 BLS12-381 field constant가 됩니다. PolicyRef·ScopeRef 계산 자체는 Poseidon2를 사용합니다.

### 정수 encoding

| 값 | 범위 |
|---|---|
| `eventKind` | uint8 |
| `authorityId` | uint64, 1부터 시작 |
| `policyId` | uint64, Authority 내부에서 1부터 시작 |
| `version` | uint64, 1부터 시작 |

`eventKind`는 최소 PROCESS와 ISSUE를 구분합니다. M5에서는 PROCESS만 실행하지만 Registry는 Issue Policy를 구분할 수 있도록 필드를 유지합니다.

### PolicyRef

$$
policyRef=\mathrm{Poseidon2}(\mathrm{PolicyRefTag},eventKind,authorityId,policyId,version)
$$

`(authorityId,policyId)`가 Policy family이고 `version`이 immutable 개정판입니다. 같은 numeric policyId라도 Authority가 다르면 다른 Policy입니다.

### PolicyScopeRef

$$
policyScopeRef=\mathrm{Poseidon2}(\mathrm{ScopeRefTag},sk_{\mathrm{owner}},policyRef)
$$

Process Circuit은 input Note ownership과 scopeRef를 같은 secret으로 검증합니다. 같은 Policy version의 Process는 linkable하며 다른 Policy·version에서는 다른 scopeRef를 사용합니다.

**왜 EVM address를 사용하지 않나요?** `msg.sender`는 transaction 제출자일 뿐 private Note owner와 연결되지 않습니다. ScopeRef는 Note address를 공개하지 않고 같은 `sk_owner`와 Policy 권한을 binding합니다. 상세 배경은 [Policy별 ZK 가명](M5-process-policy-background.md#이번-결정-policy별-zk-가명)에 있습니다.

## 4. ProcessPolicy constant

### 구조

```text
ProcessPolicy:
  eventKind       = PROCESS
  inputArity      = 3
  outputArity     = 2
  denominator     = 1,000,000,000
  lossRate        = 62,500,000
  carbonIntensity = 93,750,000
  outputRoles     = [ELIGIBLE, WASTE]
  allocation      = fixed 2×3 coefficient table
  rounding        = floor, output 0 residual
```

Rate·allocation·Role은 Circuit constant입니다. Factory가 transaction마다 normative 값을 선택할 수 없습니다.

### Canonical input

같은 owner의 ELIGIBLE Note 세 개를 사용합니다.

| Input | `q_mass` | `a_rec` | `e` |
|---|---:|---:|---:|
| A | 100,000,000,000 | 20,000,000,000 | 80,000,000,000 |
| B | 120,000,000,000 | 10,000,000,000 | 90,000,000,000 |
| C | 100,000,000,000 | 0 | 60,000,000,000 |
| 합 | 320,000,000,000 | 30,000,000,000 | 230,000,000,000 |

모든 수치는 kg 또는 kgCO2e에 $10^9$ scale을 적용한 uint64입니다.

### Input checked sum

$$
T_{\mathrm{in}}=\sum_{i=1}^{3}q_{\mathrm{mass},i}
$$

$$
R_{\mathrm{in}}=\sum_{i=1}^{3}a_{\mathrm{rec},i}
$$

$$
C_{\mathrm{in}}=\sum_{i=1}^{3}e_i
$$

두 번의 덧셈 중간값과 최종값을 모두 uint64로 range check하여 field wraparound와 uint64 overflow를 막습니다.

### Process delta

$$
q_{\mathrm{loss}}=\left\lfloor\frac{T_{\mathrm{in}}\cdot62{,}500{,}000}{10^9}\right\rfloor
$$

$$
\Delta e_{\mathrm{process}}=\left\lfloor\frac{T_{\mathrm{in}}\cdot93{,}750{,}000}{10^9}\right\rfloor
$$

Circuit은 private remainder로 각 floor를 검증합니다.

$$
T_{\mathrm{in}}\cdot rate=D\cdot quotient+remainder,\qquad 0\le remainder<D
$$

Intermediate State:

$$
I_T=T_{\mathrm{in}}-q_{\mathrm{loss}},\qquad I_R=R_{\mathrm{in}},\qquad I_C=C_{\mathrm{in}}+\Delta e_{\mathrm{process}}
$$

`q_loss<=T_in`과 `I_C` uint64 overflow를 검증합니다.

### Allocation Rule

행은 `[ELIGIBLE,WASTE]`, 열은 `[q_mass,a_rec,e]`입니다.

$$
A=
\begin{pmatrix}
900{,}000{,}000 & 1{,}000{,}000{,}000 & 1{,}000{,}000{,}000\\
100{,}000{,}000 & 0 & 0
\end{pmatrix}
$$

WASTE를 floor 계산합니다.

$$
q_W=\left\lfloor\frac{I_T\cdot100{,}000{,}000}{10^9}\right\rfloor
$$

$$
a_{\mathrm{rec},W}=0,\qquad e_W=0
$$

ELIGIBLE은 conservation residual을 받습니다.

$$
q_E=I_T-q_W,\qquad a_{\mathrm{rec},E}=I_R,\qquad e_E=I_C
$$

Canonical output은 ELIGIBLE `(270,30,260)`, WASTE `(30,0,0)`입니다.

**왜 generic Matrix multiplication을 만들지 않나요?** Coefficient가 constant이고 대부분 0 또는 1이므로 원소별 constant multiplication·equality가 더 단순합니다. Matrix는 Policy를 읽기 위한 표현입니다. 상세 배경은 [Allocation Matrix](M5-process-policy-background.md#5-allocation-matrix는-설명이고-구현은-원소별-관계입니다)에 있습니다.

## 5. Product·WASTE·외부 신뢰 경계

- Input 세 개는 ELIGIBLE입니다.
- Output 0은 ELIGIBLE, output 1은 WASTE로 고정합니다.
- Output은 같은 owner를 유지합니다.
- Output DocumentHash는 새 private 값을 허용합니다.
- Circuit은 ProductName·LotID·Product Type·recipe 현실 진실성을 검사하지 않습니다.
- Factory가 제출한 공정·제품·폐기물 설명과 근거 문서는 Certifier가 off-chain에서 검토합니다.

WASTE에는 `q_mass`만 남기고 탄소와 recycled attribution을 모두 ELIGIBLE에 귀속합니다. 판매 가능한 부산물은 WASTE가 아니라 별도 co-product Policy가 필요합니다. Waste treatment 후속 배출은 이번 Process에서 제거되는 것이 아니라 별도 system boundary의 책임입니다.

**왜 이렇게 정했나요?** True waste에는 공통 공정 탄소를 배분하지 않는 회계 방향과 DPP Claim을 ELIGIBLE product에 귀속하는 POC 정책을 사용합니다. 이 규칙의 적용 한계는 [탄소와 a_rec 결정](M5-process-policy-background.md#6-왜-탄소와-a_rec을-eligible에-모두-배분하나요)에 있습니다.

## 6. Process Circuit

### 공개·비공개값

Public input 순서:

```text
1. policyRef
2. policyScopeRef
3. noteRoot
4. nf1
5. nf2
6. nf3
7. cmEligible
8. cmWaste
```

Private witness:

- input Note·private `cm`·Depth-32 path 3개
- 공통 `sk_owner`
- input·intermediate·output State
- `q_loss`, `delta_e_process`
- loss·carbon·waste-mass remainder
- output DocumentHash·opening

Consumed `cm`과 정확한 State·owner address는 public input이나 calldata에 포함하지 않습니다.

### Native fixture 수도 코드

```text
# 핵심: 같은 owner의 세 Note에 고정 Policy rate·allocation을 적용합니다.
function BuildProcess(policy, inputs[3], ownerSecret, outputDocuments, openings):
    # 1. Policy identity와 owner를 준비합니다.
    assert policy == CanonicalProcessPolicy
    ownerAddress = H(OwnerTag, ownerSecret)
    scopeRef = H(ScopeRefTag, ownerSecret, policy.policyRef)

    # 2. 세 ELIGIBLE input State를 overflow 없이 합합니다.
    for input in inputs:
        assert input.address == ownerAddress
        assert input.AssetRole == ELIGIBLE
    total = CheckedStateSum(inputs)

    # 3. 고정 비율로 private Process delta를 계산합니다.
    qLoss, remLoss = DivRem(total.q_mass * lossRate, D)
    carbonAdd, remCarbon = DivRem(total.q_mass * carbonIntensity, D)
    intermediate = State(total.q_mass-qLoss, total.a_rec, total.e+carbonAdd)

    # 4. WASTE 질량을 floor하고 나머지를 ELIGIBLE에 줍니다.
    wasteMass, remWaste = DivRem(intermediate.q_mass * wasteMassRate, D)
    waste = State(wasteMass, 0, 0)
    eligible = State(intermediate.q_mass-wasteMass, intermediate.a_rec, intermediate.e)

    # 5. 같은 owner의 output Note 두 개를 만듭니다.
    outEligible = Note(outputDocuments[0], ELIGIBLE, eligible, ownerAddress, openings[0])
    outWaste = Note(outputDocuments[1], WASTE, waste, ownerAddress, openings[1])
    return scopeRef, outEligible, outWaste, remainders
```

### Circuit 수도 코드

```text
# 핵심: Policy identity·Factory 권한·세 private input·공정 수학·두 output을 함께 검증합니다.
function ProcessCircuit(public, private):
    # 1. public Policy statement를 고정합니다.
    [policyRef, scopeRef, noteRoot, nf1, nf2, nf3, cmEligible, cmWaste] = public
    assert policyRef == ConstantPolicyRef

    # 2. 같은 owner secret으로 scope와 Note address를 계산합니다.
    ownerAddress = H(OwnerTag, private.ownerSecret)
    assert H(ScopeRefTag, private.ownerSecret, policyRef) == scopeRef

    # 3. 세 input의 Note·ownership·membership·nullifier를 검증합니다.
    for i in [0,1,2]:
        ValidateNote(private.inputs[i])
        assert private.inputs[i].AssetRole == ELIGIBLE
        assert private.inputs[i].address == ownerAddress
        cm[i] = NoteCommitment(private.inputs[i])
        assert MerkleRoot(cm[i], private.paths[i]) == noteRoot
        assert H(NullifierTag, private.ownerSecret, cm[i]) == public.nf[i]
    assert all cm[i] distinct
    assert all nf[i] distinct

    # 4. input State를 단계별 uint64 checked sum합니다.
    total = CheckedStateSum(private.inputs)

    # 5. loss·carbon rate의 floor relation을 검증합니다.
    AssertFloor(total.q_mass, lossRate, D, private.qLoss, private.remLoss)
    AssertFloor(total.q_mass, carbonIntensity, D, private.carbonAdd, private.remCarbon)
    assert private.qLoss <= total.q_mass
    intermediate = CheckedState(total.q_mass-private.qLoss, total.a_rec, total.e+private.carbonAdd)

    # 6. WASTE 질량 floor와 ELIGIBLE residual을 검증합니다.
    AssertFloor(intermediate.q_mass, wasteMassRate, D, private.outputWaste.q_mass, private.remWaste)
    assert private.outputWaste.AssetRole == WASTE
    assert private.outputWaste.a_rec == 0
    assert private.outputWaste.e == 0
    assert private.outputEligible.AssetRole == ELIGIBLE
    assert private.outputEligible.q_mass == intermediate.q_mass-private.outputWaste.q_mass
    assert private.outputEligible.a_rec == intermediate.a_rec
    assert private.outputEligible.e == intermediate.e

    # 7. 두 output owner·commitment를 확인합니다.
    assert private.outputEligible.address == ownerAddress
    assert private.outputWaste.address == ownerAddress
    assert NoteCommitment(private.outputEligible) == cmEligible
    assert NoteCommitment(private.outputWaste) == cmWaste
```

### Circuit 실패 조건

- wrong policyRef·scopeRef·owner secret
- input Note·path·root·nullifier 변조와 duplicate input
- WASTE input 또는 input owner 불일치
- State 합·uint64 overflow·underflow
- loss·carbon rate, floor quotient·remainder 변조
- allocation·output Role·WASTE zero-attribution 변조
- output owner·commitment·opening 변조

## 7. Authority·Policy Registry

### 상태

```text
nextAuthorityId
authorityIdOf[account]
policyAuthorities[authorityId]

nextPolicyId[authorityId]
nextVersion[authorityId][policyId]
policyFamilies[authorityId][policyId]
policyReservations[policyRef]

policyRecords[policyRef]
policyGrants[policyRef][policyScopeRef]
```

`authorityId`, `policyId`, version은 1부터 시작하고 재사용하지 않습니다.

`policyFamilies`는 family 존재 여부와 고정 `eventKind`를 보관합니다. `policyReservations`는 Setup 전에 확정한 `(authorityId,policyId,version,eventKind)`을 보관하며 등록 후에도 과거 예약 증거로 유지합니다.

### PolicyRecord

```text
PolicyRecord:
  authorityId: uint64
  policyId: uint64
  version: uint64
  eventKind: uint8
  inputArity: uint8
  outputArity: uint8
  vkHash: bytes32
  verifierRef: address
  enabled: bool
```

`vkHash`는 SHA-256 canonical optimized VK encoding입니다. `verifierRef`는 방식 A의 immutable verifier Contract입니다.

### Authority 등록

```text
# 핵심: System Admin이 공개 EVM account에 고유 authorityId를 발급합니다.
function registerPolicyAuthority(account):
    require msg.sender == systemAdmin
    require account != zero
    require authorityIdOf[account] == 0
    authorityId = nextAuthorityId
    nextAuthorityId += 1
    authorityIdOf[account] = authorityId
    policyAuthorities[authorityId] = {account, enabled:true}
    emit PolicyAuthorityRegistered(authorityId, account)
```

Authority disable·rotation은 M5에서 구현하지 않습니다.

### Policy family·version 예약

```text
# 핵심: Setup 전에 policyRef를 확정합니다.
function reservePolicy(eventKind):
    authorityId = requireRegisteredAuthority(msg.sender)
    policyId = nextPolicyId[authorityId]
    nextPolicyId[authorityId] += 1
    version = 1
    policyFamilies[authorityId][policyId] = {eventKind, exists:true}
    nextVersion[authorityId][policyId] = 2
    policyRef = H(PolicyRefTag, eventKind, authorityId, policyId, version)
    policyReservations[policyRef] = reservation
    emit PolicyReserved(policyRef, authorityId, policyId, version, eventKind)
```

```text
# 핵심: 같은 family의 새 immutable version을 예약합니다.
function reservePolicyVersion(policyId):
    authorityId = requireRegisteredAuthority(msg.sender)
    require policyId belongs to authorityId
    version = nextVersion[authorityId][policyId]
    nextVersion[authorityId][policyId] += 1
    eventKind = familyEventKind[authorityId][policyId]
    policyRef = H(PolicyRefTag, eventKind, authorityId, policyId, version)
    policyReservations[policyRef] = reservation
    emit PolicyReserved(...)
```

### PolicyRecord 등록

```text
# 핵심: 예약한 Authority만 immutable verifier와 VK hash를 등록합니다.
function registerPolicy(policyRef, inputArity, outputArity, vkHash, verifierRef):
    reservation = policyReservations[policyRef]
    require reservation.authority account == msg.sender
    require policyRecords[policyRef] does not exist
    require verifierRef != zero and verifierRef has code
    require vkHash != zero
    policyRecords[policyRef] = immutable metadata with enabled=true
    emit PolicyRegistered(...)
```

같은 policyRef 재등록과 Record·verifier·vkHash 수정 API는 만들지 않습니다.

### PolicyGrant

```text
# 핵심: Policy를 등록한 Authority만 ZK scope의 사용 권한을 변경합니다.
function setPolicyGrant(policyRef, policyScopeRef, allowed):
    record = policyRecords[policyRef]
    require record.authority account == msg.sender
    require policyScopeRef != 0
    policyGrants[policyRef][policyScopeRef] = allowed
    emit PolicyGrantUpdated(policyRef, policyScopeRef, allowed)
```

Grant는 revoke 후 regrant를 허용합니다. Factory가 scopeRef를 계산해 Authority에 전달하는 off-chain 인증 과정은 신뢰 가정입니다.

### Policy disable

```text
# 핵심: Policy를 등록한 Authority가 신규 사용을 영구 중단합니다.
function disablePolicy(policyRef):
    record = policyRecords[policyRef]
    require record.authority account == msg.sender
    require record.enabled == true
    record.enabled = false
    emit PolicyDisabled(policyRef)
```

Disabled Policy는 다시 활성화할 수 없습니다. 과거 Process·Claim은 그대로 유지합니다.

## 8. Process Contract

### Interface

```text
process(
  proof,
  policyRef,
  policyScopeRef,
  noteRoot,
  nf[3],
  cmOut[2]
)
```

### Contract 수도 코드

```text
# 핵심: 승인·Grant·proof를 확인한 뒤 세 Note를 소비하고 두 Note를 원자적으로 append합니다.
function process(proof, policyRef, scopeRef, noteRoot, nf[3], cmOut[2]):
    # 1. PolicyRecord와 사용 권한을 확인합니다.
    record = policyRecords[policyRef]
    require record exists and record.enabled
    require record.eventKind == PROCESS
    require record.inputArity == 3 and record.outputArity == 2
    require policyGrants[policyRef][scopeRef] == true

    # 2. Note root·input·output precondition을 확인합니다.
    require acceptedNoteRoots[noteRoot]
    require nf[0], nf[1], nf[2] are pairwise distinct and unspent
    require cmOut[0] != cmOut[1]
    require both output commitments are new

    # 3. ordered public statement와 Policy verifier로 proof를 검증합니다.
    inputs = [policyRef, scopeRef, noteRoot, nf[0], nf[1], nf[2], cmOut[0], cmOut[1]]
    verify record.verifierRef with proof and inputs

    # 4. proof 이후에만 input과 output 상태를 반영합니다.
    mark three note nullifiers spent
    insert cmOut[0] and append Note Tree
    insert cmOut[1] and append Note Tree
```

누구나 valid proof transaction을 relay할 수 있지만 scopeRef에 대응하는 `sk_owner`와 input Note를 모르면 proof를 새로 만들 수 없습니다.

### Atomicity

Policy·Grant·root·proof·output 중 하나라도 실패하면 nullifier·commitment·Tree가 모두 불변이어야 합니다. 두 번째 output insert가 실패해도 첫 output과 세 nullifier가 EVM transaction 전체와 함께 revert되어야 합니다.

## 9. Verifier A/B architecture ablation

### 공통 canonical optimized VK

Setup 도구는 gnark VK에서 Solidity verification에 필요한 field를 고정 순서로 추출합니다.

```text
CanonicalOnchainVK:
  public input count
  domain size·inverse·omega
  selector commitments
  permutation commitments
  custom-gate metadata가 있다면 그 commitment
```

공통 KZG SRS는 generic verifier code에 고정하고 Policy별 encoding에는 포함하지 않습니다. 실제 Process VK를 생성한 뒤 word 수와 byte 수를 manifest에 기록합니다.

$$
vkHash=\mathrm{SHA256}(\mathrm{CanonicalOnchainVKEncoding})
$$

### 방식 A

- gnark generated verifier 또는 동일 constant template을 배포합니다.
- PolicyRecord의 `verifierRef`가 이 Contract를 가리킵니다.
- 공식 Process transaction은 이 경로를 사용합니다.

### 방식 B

- 공통 verifier logic·SRS를 가진 generic verifier를 한 번 배포합니다.
- canonical optimized VK를 PolicyRef별 storage에 기록합니다.
- storage VK를 memory 구조로 읽어 같은 Process proof를 검증합니다.
- 방식 A와 같은 `vkHash`, proof, ordered public input과 성공 결과를 요구합니다.
- B는 공식 PolicyRecord를 대체하지 않는 별도 diagnostic Contract로 둡니다.

### 비교 경계

| 측정 | 방식 A | 방식 B |
|---|---|---|
| 공통·Policy별 deployment | 측정 | 측정 |
| Policy-specific VK 등록 | verifier deployment | SSTORE |
| Process verification gas | 측정 | 측정 |
| VK load | bytecode constant | SLOAD·memory |
| Policy 수 1·2·5·10 누적 | raw 결과로 계산 | raw 결과로 계산 |

Serialized 49KB Go VK 전체 SSTORE는 비교하지 않습니다. 두 방식은 동일한 optimized VK를 사용해야 합니다.

**왜 두 방식을 비교하나요?** 방식 A는 per-policy deployment가 크고 runtime load가 작으며, 방식 B는 공통 코드를 재사용하지만 등록 SSTORE와 매 proof SLOAD가 생깁니다. 상세 배경은 [Verifier A/B](M5-process-policy-background.md#9-verifier-방식-a와-b)에 있습니다.

## 10. Canonical 시나리오

```text
1. System Admin이 Authority EVM account 등록
2. Authority가 PROCESS Policy family version 1 예약
3. Actor 1이 policyScopeRef 계산
4. Certifier가 3-to-2 Process Circuit Setup
5. Authority가 PolicyRecord와 Actor 1 Grant 등록
6. EntryIssuer가 Actor 1 소유 ELIGIBLE Note A·B·C 등록
7. Actor 1이 Process proof 생성
8. 임의 EVM submitter가 Process transaction 제출
9. ELIGIBLE·WASTE commitment와 nullifier·root 검증
10. 동일 proof를 방식 B diagnostic verifier에서도 검증
11. Authority가 Policy disable
12. 신규 Process가 거부되고 과거 output은 유지됨을 확인
```

Grant revoke·regrant는 canonical 외 별도 lifecycle test로 검증합니다.

## 11. 구현 구조·Artifact·명령

### 예정 기능

```text
internal/core/policy/
  Domain·PolicyRef·ScopeRef·Process constant·optimized VK encoding

features/process_policy_3_2/
  README·Circuit·test

internal/m5case/
  interaction·Policy·Process fixture

contracts:
  Policy Registry가 포함된 Ledger
  constant Process verifier
  storage-VK diagnostic verifier

cmd/setup_m5/
cmd/benchmark_m5/
```

M1~M4의 Note·owner·commitment·nullifier·Merkle·checked sum·allocation·artifact·Anvil runner를 재사용합니다. M1~M4 Raw 결과와 Artifact를 덮어쓰지 않습니다.

### 개발 Artifact·Raw 결과

```text
artifacts/development/m5/process-policy-3-2/
output/m5-circuit.json
output/m5-policy-registry-gas.json
output/m5-verifier-ablation.json
output/m5-anvil-gas.json
output/m5-anvil-e2e.json
output/m5-generated-checksums.json
```

### 예정 명령

```text
make setup-m5
make test-go
make test-contract-m5
make benchmark-m5-gas
make benchmark-m5-e2e
make benchmark-m5-verifier-ablation
make benchmark-m5
```

공식 측정은 case별 한 번입니다. Constant verifier 공식 경로와 storage verifier ablation을 결과에서 분리합니다.

## 12. Correctness·완료 Gate

### Identity·Registry

- Authority ID는 1부터 증가하고 account·ID를 재사용하지 않습니다.
- 미등록 Authority, 다른 Authority의 family/version·Record·Grant·Disable 조작이 실패합니다.
- PolicyRef가 Native·Circuit·Solidity에서 일치합니다.
- 같은 `sk_owner`·policyRef의 scopeRef가 일치하고 다른 Policy에서는 달라집니다.
- 같은 policyRef 재등록·Record 수정·disabled 재활성화가 불가능합니다.
- Grant revoke는 신규 Process를 막고 regrant 후 다시 허용합니다.

### Process Circuit

- public variable 수는 정확히 8개입니다.
- consumed `cm`, path, owner address와 State가 public witness에 없습니다.
- canonical `(320,30,230)→(270,30,260)+(30,0,0)`이 성공합니다.
- 다른 batch 질량에서도 같은 rate·VK가 올바른 비율을 계산합니다.
- wrong PolicyRef·ScopeRef·owner·input·delta·allocation·Role·remainder가 실패합니다.
- duplicate input, overflow·underflow와 WASTE attribution 변조가 실패합니다.

### Contract·Atomicity

- Policy 등록·Grant 후 Process가 성공합니다.
- missing/revoked Grant, disabled Policy, wrong EventKind·arity·root·proof를 거부합니다.
- input nullifier 재사용과 output 중복을 거부합니다.
- 실패 후 Policy 상태·nullifier·commitment·Tree가 부분 변경되지 않습니다.
- 기존 M2~M4 Event가 회귀하지 않습니다.

### Verifier A/B

- A·B의 canonical optimized VK word·encoding·vkHash가 같습니다.
- 같은 proof·public input이 두 방식에서 모두 성공합니다.
- 한 word 변조와 vkHash mismatch가 실패합니다.
- deployment·VK 등록·SLOAD·Verify gas를 분리합니다.
- 49KB serialized VK와 optimized VK 값을 같은 열에 섞지 않습니다.

### 완료 조건

1. Background와 이 명세가 사용자 검토를 통과해 `구현 준비 완료`가 됩니다.
2. Native·Circuit·Foundry·A/B Gate를 통과합니다.
3. Setup·gas·E2E·ablation을 정한 횟수만 실행합니다.
4. M1~M4 Raw 결과 checksum이 변하지 않습니다.
5. `REUSE.md`와 `ARCHITECTURE.md`를 실제 결과에 맞게 갱신합니다.
6. `M5-process-policy-result.md`를 Result Template에 맞춰 생성합니다.
7. Result와 Raw JSON이 일치하지 않으면 M5를 완료로 표시하지 않습니다.

## 13. 명시적 비범위

- Policy Authority ZK credential·privacy
- Factory를 EVM `msg.sender`로 식별하는 방식
- Policy 간·동일 Policy 내 완전 unlinkability
- ProductProfile·Product Type 현실 검증
- range·비선형·고정비 Process Policy
- 판매 가능한 co-product allocation
- raw 49KB serialized VK 전체 SSTORE
- mutable verifier·Chameleon Hash·proxy upgrade
- Status·Audit·Issue 실행
- final universal SRS·Besu
