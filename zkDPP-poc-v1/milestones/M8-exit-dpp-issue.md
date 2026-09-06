# M8 — Exit한 DPP에 Sustainability Claim을 어떻게 붙이나요?

- 명세 상태: 구현 기준 동결
- 구현 결과: [M8 Result](M8-exit-dpp-issue-result.md)
- 의사결정 배경: [M8 Background](M8-exit-dpp-issue-background.md)
- 이전 결과: [M7 Result](M7-audit-tracing-result.md)
- 검토 기준: [Milestone Checklist](MILESTONE-CHECKLIST.md)
- 작성 범위: 구현 전 자체 완결형 명세입니다. 실제 코드·테스트·측정 결과는 이후 Result에서 확인합니다.
- 원칙: **YAGNI가 최우선입니다.** DPP commitment·Exit·Issue Claim의 구현 가능성, correctness와 비용 확인에 필요한 기능만 만듭니다.

## 0. 이번에는 무엇이 가능해지나요?

**private Note를 Exit하여 공개 DPP commitment로 확정하고, 제품 정보를 공개하지 않은 채 그 DPP가 Sustainability Policy를 만족한다는 Claim을 반복해서 붙입니다.**

| 질문 | M8의 기준 |
|---|---|
| 이전에는 무엇이 있었나요? | M7은 private Note·Voucher의 Event, AuditRecord, 양방향 추적과 nf 기반 동결을 구현했습니다. |
| Exit는 무엇을 추가하나요? | Note를 소비하면서 같은 제품·State를 가진 공개 dppCommitment를 만듭니다. |
| Issue는 무엇을 하나요? | DPP를 소비하거나 바꾸지 않고, 특정 IssuePolicy를 만족했다는 Claim을 추가합니다. |
| Claim은 무엇으로 찾나요? | dppCommitment와 issuePolicyRef의 조합으로 찾습니다. |
| 무엇을 숨기나요? | DocumentHash·AssetRole·State·dppOpening입니다. |
| 제3자는 무엇을 확인하나요? | 특정 DPP에 특정 Policy Claim이 등록됐는지와 현재 상태를 확인합니다. |
| 같은 DPP에 Claim을 여러 개 붙일 수 있나요? | 서로 다른 Policy 또는 version은 가능합니다. 동일 PolicyRef는 한 번만 가능합니다. |
| 무엇은 하지 않나요? | DPP 소유권 이전, 하위 DPP 구성, Re-entry, Claim Tree와 규제 DPP 전체 구현입니다. |

전체 흐름은 다음 두 단계입니다.

$$
\mathrm{private\ Note}
\xrightarrow{\mathrm{Exit}}
\mathrm{Finalized\ DPP}(\mathrm{dppCommitment})
$$

$$
\mathrm{Finalized\ DPP}
\xrightarrow{\mathrm{IssuePolicy}}
\mathrm{같은\ DPP}+\mathrm{Claim}
$$

Exit는 공급망 전이를 종료합니다. Issue는 종료된 DPP의 private State를 공개하지 않고도 Policy 조건 충족을 확인할 수 있게 합니다.

## 1. M7에서 무엇이 달라지나요?

| M7 | M8에서 추가하는 것 |
|---|---|
| Exit는 Note를 소비하고 경로를 종료합니다. | Exit가 같은 제품·State를 dppCommitment로 확정합니다. |
| producerOf는 Note·Voucher 생성 기록을 찾습니다. | DPP 종류를 추가해 Exit 생성 기록을 찾습니다. |
| Policy Registry는 Process Policy를 실행합니다. | 같은 Registry에 Issue Policy와 verifier를 등록합니다. |
| AuditRecord는 숨겨진 부모·출력 소비값을 기록합니다. | Issue는 숨겨진 연결값이 없으므로 암호문 없는 AuditRecord를 허용합니다. |
| nf·rvnf 상태를 집행합니다. | DPP·Policy 조합으로 Claim 상태를 별도 집행합니다. |

새 Main Contract 이름은 ZkDPPClaimLedger입니다. M7의 ZkDPPAuditLedger를 기반으로 하는 별도 M8 배포이며, M7 Contract·Circuit·Result는 비교 가능한 baseline으로 보존하고 상태 migration은 구현하지 않습니다.

M8 Exit가 기존 M7 Exit를 대체하는 이유는 단순합니다. Note를 소비하는 시점에만 private Note와 새 DPP 원문이 같은지 한 번에 증명할 수 있기 때문입니다. Issue는 공개 식별값으로 이미 확정된 dppCommitment를 사용하고, 그 private 원문을 witness로 넣어 Policy 충족을 증명합니다.

## 2. 하나의 예시로 전체 흐름을 보면 어떤가요?

M5의 3-to-2 Process가 만든 ELIGIBLE output을 사용합니다.

| 값 | 실제 단위 값 | scaled integer |
|---|---:|---:|
| $q_{\mathrm{mass}}$ | 270 kg | 270,000,000,000 |
| $a_{\mathrm{rec}}$ | 30 kg | 30,000,000,000 |
| $e$ | 260 kgCO2e | 260,000,000,000 |

이 DPP의 재활용률은 약 11.11%이고 탄소집약도는 약 0.963 kgCO2e/kg입니다. 따라서 아래 두 Policy를 모두 만족합니다.

| Issue Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

대표 실행은 다음 순서입니다.

1. 기존 Entry 3건과 Process를 실행하여 ELIGIBLE Note와 WASTE Note를 만듭니다.
2. ELIGIBLE Note를 M8 Exit로 소비하고 dppCommitment를 등록합니다.
3. Policy Authority가 등록한 Standard V1 proof로 첫 Claim을 Issue합니다.
4. 같은 DPP·Standard V1 Claim을 다시 만들면 중복으로 거부합니다.
5. Strict V2 proof로 두 번째 Claim을 Issue합니다.
6. DPP 문서의 claims 배열에 두 issuePolicyRef를 담습니다.
7. Standard Claim을 Freeze한 뒤 다시 Active로 돌립니다.
8. Strict Claim은 Freeze한 뒤 Revoke합니다.
9. 제3자가 두 Claim의 등록 여부와 현재 상태를 조회합니다.
10. Claim에서 DPP의 Exit 기록을 찾고, 암호화된 부모 cm을 복원하여 기존 provenance를 거슬러 올라갑니다.

WASTE Note도 Exit하여 DPP commitment를 만들 수 있습니다. 다만 이번 Standard·Strict Policy는 ELIGIBLE만 허용하므로 WASTE DPP에는 Claim을 붙일 수 없습니다.

## 3. 누가 무엇을 담당하나요?

| 주체 | 역할 | 알고 있는 정보 |
|---|---|---|
| Participant | Note를 Exit하고 DPP private 원문으로 Issue proof를 만듭니다. | Note secret·DPPPrivateData |
| Policy Authority | Issue Policy family·version과 verifier를 등록·비활성화합니다. | 공개 Policy 규칙·VK |
| Status Authority | Claim을 Freeze·Unfreeze·Revoke합니다. | 공개 ClaimRef와 외부 판단 근거 |
| 제3자 | DPP 문서에 적힌 Policy Claim을 온체인에서 확인합니다. | dppCommitment·issuePolicyRef |
| 위원회·Auditor | Exit 기록의 부모 cm을 복호화하고 기존 provenance를 추적합니다. | 위원 share 또는 복원 결과 |
| Contract | Exit·Issue proof, 등록 상태, 중복과 상태 전이를 집행합니다. | 공개 입력·온체인 상태 |

Policy Authority는 Claim 내용을 대신 증명하지 않습니다. 어떤 규칙을 공식 Issue Policy로 사용할지 등록합니다. 실제 proof는 dppCommitment의 private 원문을 아는 사람이 생성합니다.

Issue에는 Process의 PolicyGrant와 policyScopeRef를 사용하지 않습니다. Claim은 특정 Factory의 공정 실행 권한이 아니라, 공개 등록된 Policy 조건을 DPP State가 만족한다는 사실이기 때문입니다.

## 4. DPP private data와 commitment는 무엇인가요?

### 4.1 무엇을 commitment하나요?

**공급망 Note에서 제품·Sustainability State만 가져오고 Note 소유권 정보는 제거합니다.**

| Field | 의미 | 범위·단위 |
|---|---|---|
| DocumentHash | ProductName·LotID의 canonical Hash | BLS12-381 scalar field |
| AssetRole | ELIGIBLE 또는 WASTE | enum 0 또는 1 |
| $q_{\mathrm{mass}}$ | 전체 질량 | kg × $10^9$, uint64 |
| $a_{\mathrm{rec}}$ | 재활용 귀속량 | kg × $10^9$, uint64 |
| $e$ | 누적 탄소 | kgCO2e × $10^9$, uint64 |
| dppOpening | commitment hiding·구분 값 | field element |

Field 순서는 다음으로 고정합니다.

~~~text
DPPPrivateData
  1. DocumentHash
  2. AssetRole
  3. q_mass
  4. a_rec
  5. e
  6. dppOpening
~~~

Domain은 다음 문자열을 기존 hash-to-field 방식으로 상수화합니다.

~~~text
DPPTag = "zkDPP:DPP:v1"
~~~

관계식은 다음입니다.

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

### 4.2 무엇을 넣지 않나요?

- owner address는 넣지 않습니다. DPP Claim을 특정 Note owner에 고정하지 않기 위해서입니다.
- Note opening은 넣지 않습니다. 공급망 Note와 finalized DPP의 hiding 값은 서로 분리합니다.
- Note nf는 넣지 않습니다. nf는 공급망 Note를 한 번 소비하는 값이지 DPP identity가 아닙니다.
- 제품 원문은 공개하지 않습니다. DPP owner가 별도 문서로 보여줄 수 있지만 Claim 검증의 필수 입력은 아닙니다.

dppOpening은 소유권 secret이 아닙니다. private 원문을 전달받은 사람이 복사할 수 있으므로 M8은 현재 DPP owner를 암호학적으로 판별하지 않습니다. Issue가 보장하는 것은 **private 원문을 아는 prover가 실제 State에 맞는 Claim을 만들었다는 사실**입니다.

### 4.3 일반 계산은 어떻게 하나요?

~~~text
function CommitDPP(data):
    # 핵심: private 제품·State를 owner와 분리된 DPP commitment로 만듭니다.
    # 1. 기존 Note와 같은 State 범위를 확인합니다.
    require data.q_mass, data.a_rec, data.e가 uint64 범위
    require data.a_rec <= data.q_mass
    require data.AssetRole이 ELIGIBLE 또는 WASTE

    # 2. WASTE의 기존 attribution 규칙을 유지합니다.
    if data.AssetRole == WASTE:
        require data.a_rec == 0
        require data.e == 0

    # 3. 정해진 Field 순서로 DPP commitment를 계산합니다.
    return Poseidon2(
        DPPTag,
        data.DocumentHash,
        data.AssetRole,
        data.q_mass,
        data.a_rec,
        data.e,
        data.dppOpening
    )
~~~

Native 계산과 Circuit gadget은 같은 Domain·Field 순서·Poseidon2 구현을 공유합니다.

## 5. Exit는 Note를 어떻게 DPP로 확정하나요?

### 5.1 목적과 공개 범위는 무엇인가요?

**Exit는 private Note를 한 번 소비하고, 같은 제품·State를 가진 공개 dppCommitment를 등록합니다.**

Public input 순서는 다음 여섯 개입니다.

~~~text
1. noteRoot
2. nf
3. dppCommitment
4. R1X
5. R1Y
6. encryptedParentCM
~~~

Private witness는 다음입니다.

- 기존 Note 전체와 owner secret
- 소비 Note의 private cm·leaf index·Depth-32 sibling path
- DPPPrivateData
- 감사 암호화 난수

noteRoot는 membership을, nf는 중복 소비와 nf 상태를, dppCommitment는 finalized DPP를 나타냅니다. $R_{1X},R_{1Y}$와 encryptedParentCM은 실제 소비 cm을 숨긴 채 backward tracing을 가능하게 합니다.

### 5.2 Circuit은 무엇을 검증하나요?

~~~text
circuit M8Exit(public, private):
    # 핵심: 소비한 Note와 동일한 제품·State만 finalized DPP로 바꿉니다.
    # 1. 기존 M7 방식으로 private Note가 Tree에 있는지 확인합니다.
    cm = CommitNote(private.note)
    require MerkleRoot(cm, private.index, private.noteSiblings) == public.noteRoot

    # 2. prover가 Note owner secret을 아는지 확인합니다.
    ownerAddress = Hash(OwnerTag, private.sk_owner)
    require ownerAddress == private.note.address

    # 3. 공개 nf가 바로 이 private Note의 소비값인지 확인합니다.
    expectedNf = Hash(NullifierTag, private.sk_owner, cm)
    require expectedNf == public.nf

    # 4. Note와 DPP가 같은 제품·Role·State를 가리키는지 확인합니다.
    require private.dpp.DocumentHash == private.note.DocumentHash
    require private.dpp.AssetRole   == private.note.AssetRole
    require private.dpp.q_mass      == private.note.q_mass
    require private.dpp.a_rec       == private.note.a_rec
    require private.dpp.e           == private.note.e

    # 5. 공개 dppCommitment를 다시 계산합니다.
    expectedDPP = CommitDPP(private.dpp)
    require expectedDPP == public.dppCommitment

    # 6. 실제 부모 cm 하나를 M7 감사 형식으로 암호화했는지 확인합니다.
    L = HashAuditContext(EXIT, public.noteRoot, public.nf, public.dppCommitment)
    ciphertext = EncryptAudit(PK, L, [cm], private.auditRandomness)
    require ciphertext.R1 == (public.R1X, public.R1Y)
    require ciphertext.C[0] == public.encryptedParentCM
~~~

M7 Note의 State·Role·소유권·membership·nullifier 검사를 그대로 유지합니다. StatusTree path는 사용하지 않습니다. Contract가 public nf로 Active·미소비 상태를 확인합니다.

### 5.3 Contract는 무엇을 기록하나요?

새 객체 종류는 다음 값을 사용합니다.

~~~text
ObjectType.DPP = 3
~~~

Exit 성공 시 생성하는 AuditRecord의 의미는 다음입니다.

| 필드 | 값 |
|---|---|
| eventKind | EXIT=7 |
| policyRef | 0 |
| outputRefs | DPPRef(dppCommitment) 하나 |
| 공개점 | $(R_{1X},R_{1Y})$ |
| encryptedParents | encryptedParentCM 하나 |
| encryptedOutputNfs | 빈 배열 |

DPP는 이후 Issue에서 소비되지 않습니다. 따라서 DPP Tree·DPP nullifier·미래 소비값 암호문을 만들지 않습니다.

Contract 의사 코드는 다음입니다.

~~~text
function exit(proof, noteRoot, nf, dppCommitment, auditCiphertext):
    # 핵심: Note 소비·DPP 생성·AuditRecord를 한 transaction으로 확정합니다.
    # 1. 기존 Note 원장 상태를 확인합니다.
    require noteRoot가 accepted Note root
    require noteSpentIn[nf] == 0
    require noteStatusByNf[nf] == ACTIVE

    # 2. 같은 DPP가 이미 등록됐는지 확인합니다.
    require producerOf[DPP][dppCommitment] == 0

    # 3. Contract가 고정한 M8 Exit verifier로 proof를 검증합니다.
    publicInputs = [noteRoot, nf, dppCommitment,
                    auditCiphertext.R1X, auditCiphertext.R1Y,
                    auditCiphertext.encryptedParentCM]
    require m8ExitVerifier.Verify(proof, publicInputs)

    # 4. 소비·DPP 생성·AuditRecord를 하나의 transaction에서 기록합니다.
    auditRecordId = nextAuditRecordId
    noteSpentIn[nf] = auditRecordId
    producerOf[DPP][dppCommitment] = auditRecordId
    auditRecords[auditRecordId] = Exit AuditRecord
    nextAuditRecordId += 1

    emit AuditRecorded(auditRecordId)
    emit DPPFinalized(dppCommitment, auditRecordId)
    return auditRecordId
~~~

proof가 실패하거나 dppCommitment가 중복이면 nf·producerOf·AuditRecord ID가 모두 바뀌지 않아야 합니다.

## 6. IssuePolicy는 어떻게 준비하나요?

### 6.1 PolicyRef는 어떻게 정해지나요?

M5의 Authority·Policy family·version·immutable PolicyRecord·one-way disable을 그대로 사용합니다.

$$
\mathrm{issuePolicyRef}
=
H(
\mathrm{PolicyRefTag},
\mathrm{ISSUE},
\mathrm{authorityId},
\mathrm{policyId},
\mathrm{version}
)
$$

Encoding은 M5와 같습니다.

| 값 | encoding |
|---|---|
| eventKind | uint8, ISSUE=8 |
| authorityId | uint64, 0은 미등록 |
| policyId | Authority 내부 uint64 family ID |
| version | uint64, 1부터 시작 |

Canonical 등록 순서는 다음입니다.

1. System Admin이 Policy Authority account를 등록하여 authorityId=1을 발급합니다.
2. Authority는 기존 Process family를 policyId=1로 유지합니다.
3. Authority가 ISSUE family를 예약하여 policyId=2, version=1의 policyRef를 얻습니다.
4. Certifier가 Standard V1 Circuit을 해당 policyRef 상수로 Setup합니다.
5. Authority가 ISSUE·1-to-1·verifierRef·vkHash를 immutable PolicyRecord로 등록합니다.
6. Version 1 등록 후 version=2를 예약하고 Strict V2도 같은 순서로 등록합니다.

Circuit Setup은 policyRef 예약 뒤에 수행합니다. policyRef가 Circuit constant이므로 같은 Policy version의 여러 DPP proof는 같은 PK·VK를 재사용할 수 있지만 다른 version은 별도 Circuit·PK·VK를 사용합니다.

### 6.2 두 Policy는 어떤 조건인가요?

기존 비율 분모를 재사용합니다.

$$
D=1{,}000{,}000{,}000
$$

| 상수 | Standard V1 | Strict V2 |
|---|---:|---:|
| inputArity | 1 | 1 |
| outputArity | 1 | 1 |
| minRecycledRate | 100,000,000 | 110,000,000 |
| maxCarbonIntensity | 1,000,000,000 | 970,000,000 |
| 허용 Role | ELIGIBLE | ELIGIBLE |

두 조건은 나눗셈 없이 cross multiplication으로 검증합니다.

$$
a_{\mathrm{rec}}\cdot D
\ge
q_{\mathrm{mass}}\cdot\mathrm{minRecycledRate}
$$

$$
e\cdot D
\le
q_{\mathrm{mass}}\cdot\mathrm{maxCarbonIntensity}
$$

양변의 곱셈은 uint64 결과로 제한하지 않습니다. 먼저 입력 값이 uint64임을 확인하고 Circuit field에서 정확한 정수 곱셈·비교를 수행합니다. 현재 최대 곱은 BLS12-381 scalar field modulus보다 작으므로 modular wraparound를 정수 비교로 오해하지 않도록 bound를 함께 테스트합니다.

### 6.3 PolicyRecord는 무엇을 의미하나요?

M5 구조를 유지합니다.

~~~text
PolicyRecord
  authorityId
  policyId
  version
  eventKind = ISSUE
  inputArity = 1
  outputArity = 1
  vkHash
  verifierRef
  enabled
~~~

Policy disable은 이후의 Issue만 막습니다. 이미 만들어진 Claim의 등록 여부나 상태는 바꾸지 않습니다. 같은 family의 version도 서로 다른 PolicyRef·verifier·Claim 상태를 가집니다.

## 7. Issue는 Claim을 어떻게 추가하나요?

### 7.1 Claim을 무엇으로 식별하나요?

**Claim은 하나의 DPP와 하나의 Issue Policy가 맺는 관계입니다.**

$$
\mathrm{ClaimRef}
=
(\mathrm{dppCommitment},\mathrm{issuePolicyRef})
$$

별도 식별 Hash나 nonce를 만들지 않습니다. Contract mapping의 두 key와 DPP 문서의 두 값이 이미 같은 Claim을 유일하게 가리킵니다.

~~~text
DPP
  dppCommitment
  claims[]
    issuePolicyRef = Standard V1
    issuePolicyRef = Strict V2
~~~

claims 배열은 DPP 문서가 보여주는 off-chain 목록입니다. Contract에 모든 Claim을 모은 동적 배열을 중복 저장하지 않습니다. 각 항목의 실제 등록 여부와 상태는 dppCommitment·issuePolicyRef로 claimRecordOf와 claimStatus를 조회해 확인합니다.

### 7.2 공개값과 비공개값은 무엇인가요?

Issue Circuit의 Public input은 두 개뿐입니다.

~~~text
1. issuePolicyRef
2. dppCommitment
~~~

Private witness는 다음입니다.

~~~text
DocumentHash
AssetRole
q_mass
a_rec
e
dppOpening
~~~

제품 정보와 State는 숨습니다. 제3자는 공개 PolicyRef를 통해 조건을 알고, 공개 dppCommitment에 그 조건을 만족한다는 Claim이 등록됐는지만 확인합니다.

### 7.3 Circuit은 무엇을 검증하나요?

~~~text
circuit IssuePolicy(public, private):
    # 핵심: 숨겨진 DPP State가 이 Circuit의 Sustainability 기준을 만족하는지 증명합니다.
    # 1. 다른 Policy verifier로 바꿔치기하지 못하게 PolicyRef를 고정합니다.
    require public.issuePolicyRef == CIRCUIT_POLICY_REF

    # 2. State 형식과 이번 Policy가 허용하는 DPP 종류를 확인합니다.
    require private.q_mass, private.a_rec, private.e가 uint64 범위
    require private.q_mass > 0
    require private.a_rec <= private.q_mass
    require private.AssetRole == ELIGIBLE

    # 3. private 원문이 공개 DPP commitment의 원문인지 확인합니다.
    expectedDPP = CommitDPP(private)
    require expectedDPP == public.dppCommitment

    # 4. 재활용률이 최소 기준 이상인지 나눗셈 없이 확인합니다.
    require private.a_rec * D
            >= private.q_mass * MIN_RECYCLED_RATE

    # 5. 탄소집약도가 최대 기준 이하인지 나눗셈 없이 확인합니다.
    require private.e * D
            <= private.q_mass * MAX_CARBON_INTENSITY
~~~

Issue Circuit은 Note membership·Note nf·owner secret을 다시 검사하지 않습니다. Exit Circuit이 Note와 DPP 원문의 동일성을 검증했고 Contract가 이 dppCommitment의 producer가 Exit인지 확인하기 때문입니다.

### 7.4 Contract는 무엇을 확인하나요?

~~~text
function issue(proof, issuePolicyRef, dppCommitment):
    # 핵심: Exit된 DPP에 검증된 Policy Claim 하나를 원자적으로 추가합니다.
    # 1. DPP가 M8 Exit에서 실제로 만들어졌는지 확인합니다.
    exitRecordId = producerOf[DPP][dppCommitment]
    require exitRecordId != 0
    require auditRecords[exitRecordId].eventKind == EXIT

    # 2. Issue Policy의 존재·활성 상태·종류를 확인합니다.
    policy = policyRecords[issuePolicyRef]
    require policy가 존재
    require policy.enabled
    require policy.eventKind == ISSUE
    require policy.inputArity == 1
    require policy.outputArity == 1

    # 3. 동일 DPP·Policy Claim의 중복을 막습니다.
    require claimRecordOf[dppCommitment][issuePolicyRef] == 0

    # 4. PolicyRecord가 지정한 verifier로 두 공개값을 검증합니다.
    require policy.verifierRef.Verify(
        proof,
        [issuePolicyRef, dppCommitment]
    )

    # 5. Claim 생성 기록을 원자적으로 남깁니다.
    auditRecordId = nextAuditRecordId
    auditRecords[auditRecordId] = Issue AuditRecord
    claimRecordOf[dppCommitment][issuePolicyRef] = auditRecordId
    nextAuditRecordId += 1

    emit AuditRecorded(auditRecordId)
    emit ClaimIssued(dppCommitment, issuePolicyRef, auditRecordId)
    return auditRecordId
~~~

누구나 transaction을 relay할 수 있습니다. Contract는 msg.sender를 DPP owner로 해석하지 않습니다. proof 생성에는 DPP private 원문이 필요하지만 제출 account와 ZK identity를 binding하지 않습니다.

### 7.5 Main Contract의 최소 Interface는 무엇인가요?

M8 Main Contract는 M7의 Entry·Transfer·Proceed·Recall·Merge·Split·Process와 조회 API를 유지하고, 기존 Exit를 DPP output이 있는 형태로 교체합니다. 추가·변경되는 최소 Interface는 다음입니다.

~~~text
exit(
    proof,
    noteRoot,
    nf,
    dppCommitment,
    auditCiphertext
) → auditRecordId

issue(
    proof,
    issuePolicyRef,
    dppCommitment
) → auditRecordId

verifyClaim(
    dppCommitment,
    issuePolicyRef
) → registered, status, issueAuditRecordId

setClaimStatus(
    dppCommitment,
    issuePolicyRef,
    newStatus
)
~~~

추가 상태와 Event는 다음으로 제한합니다.

| 상태·Event | 역할 |
|---|---|
| producerOf[DPP][dppCommitment] | 해당 DPP를 만든 Exit AuditRecord를 찾습니다. |
| claimRecordOf[dppCommitment][issuePolicyRef] | Claim을 만든 Issue AuditRecord를 찾고 중복을 막습니다. |
| claimStatus[dppCommitment][issuePolicyRef] | Claim의 Active·Frozen·Revoked 상태입니다. |
| DPPFinalized | 공개 DPP commitment와 Exit 기록 ID를 알립니다. |
| ClaimIssued | DPP·Policy·Issue 기록 ID를 알립니다. |
| ClaimStatusChanged | 한 Claim의 이전·새 상태를 알립니다. |

constructor는 M7의 hasher·Entry 및 Event verifier·immutable Status Authority를 유지하고 M8 Exit verifier를 받습니다. Issue verifier는 constructor 배열로 고정하지 않고 각 immutable PolicyRecord의 verifierRef에서 선택합니다. 임의 verifier 주소를 issue 호출 인자로 받지 않습니다.

## 8. Claim 상태와 제3자 검증은 어떻게 동작하나요?

### 8.1 어떤 상태를 저장하나요?

~~~text
claimRecordOf[dppCommitment][issuePolicyRef]
    = Issue AuditRecord ID

claimStatus[dppCommitment][issuePolicyRef]
    = ACTIVE(0) | FROZEN(1) | REVOKED(2)
~~~

claimRecordOf가 0이면 Claim이 등록되지 않은 것입니다. 별도의 claimRegistered boolean은 저장하지 않습니다.

상태 전이는 M7과 동일합니다.

~~~text
ACTIVE → FROZEN
FROZEN → ACTIVE
FROZEN → REVOKED
~~~

REVOKED는 terminal입니다. ACTIVE에서 바로 REVOKED하거나 REVOKED를 되돌릴 수 없습니다. constructor에서 지정한 immutable Status Authority만 상태를 변경합니다.

~~~text
function setClaimStatus(dppCommitment, issuePolicyRef, newStatus):
    # 핵심: Status Authority만 특정 DPP·Policy Claim의 상태를 바꿉니다.
    # 1. 공개 책임 주체와 Claim 존재 여부를 확인합니다.
    require msg.sender == statusAuthority
    require claimRecordOf[dppCommitment][issuePolicyRef] != 0

    # 2. 현재 상태에서 허용된 전이인지 확인합니다.
    oldStatus = claimStatus[dppCommitment][issuePolicyRef]
    require (oldStatus, newStatus)가 허용된 전이

    # 3. 해당 Policy version의 Claim 상태만 변경합니다.
    claimStatus[dppCommitment][issuePolicyRef] = newStatus
    emit ClaimStatusChanged(
        dppCommitment,
        issuePolicyRef,
        oldStatus,
        newStatus
    )
~~~

한 Claim의 상태를 바꿔도 같은 DPP의 다른 Policy Claim은 변하지 않습니다. PolicyRecord disable과 Claim status도 별개입니다.

### 8.2 제3자는 무엇을 확인하나요?

DPP owner가 제3자에게 반드시 보여줄 값은 두 개입니다.

1. dppCommitment
2. 확인하려는 issuePolicyRef

제3자는 다음 view를 호출합니다.

~~~text
verifyClaim(dppCommitment, issuePolicyRef)
    → registered
    → status
    → issueAuditRecordId
~~~

registered가 false이면 status의 기본값 0을 Active Claim으로 해석하면 안 됩니다. **먼저 registered를 확인한 뒤 status를 해석해야 합니다.**

registered가 true이고 status가 ACTIVE이면 현재 유효한 Claim입니다. FROZEN은 일시 중지, REVOKED는 영구 철회입니다. Policy가 나중에 disable되어도 과거 Claim은 상태가 ACTIVE인 한 등록 사실을 유지합니다.

eth_call은 gas를 실제 지불하지 않는 조회입니다. 성능 측정에서는 같은 view를 transaction으로 실행한 gas와 eth_call latency를 구분합니다.

### 8.3 제품 정보는 무엇을 보여줘야 하나요?

Claim 확인에 ProductName·LotID·정확한 State는 필요하지 않습니다. DPP owner는 필요한 경우 DPPPrivateData와 연결된 제품 문서를 공개해 dppCommitment 원문을 보여줄 수 있지만, M8의 공개 Claim 확인은 이를 요구하지 않습니다.

이 기능은 규제상 DPP 전체 검증이 아닙니다. 제3자는 **이 commitment가 등록된 Policy 기준을 만족했다는 사실과 현재 Claim 상태**를 확인합니다.

## 9. Issue AuditRecord와 backward tracing은 어떻게 연결되나요?

### 9.1 Issue는 왜 암호화하지 않나요?

Issue가 참조하는 dppCommitment와 issuePolicyRef는 transaction에 공개됩니다. DPP를 소비하지 않고 새 private 객체도 만들지 않으므로 숨겨야 할 부모 참조나 미래 nf가 없습니다.

Issue AuditRecord는 다음 의미만 가집니다.

| 필드 | 값 |
|---|---|
| eventKind | ISSUE=8 |
| policyRef | issuePolicyRef |
| outputRefs | 빈 배열 |
| r1X, r1Y | 사용하지 않음, storage 기본값 0 |
| encryptedParents | 빈 배열 |
| encryptedOutputNfs | 빈 배열 |

R1=(0,0)을 정상 Jubjub ciphertext point라고 해석하지 않습니다. eventKind가 ISSUE이면 **암호문이 없는 Record 형식**으로 해석합니다. Issue public input에도 R1 좌표를 넣지 않습니다.

AuditRecord에 dppCommitment를 다시 저장하지 않습니다. claimRecordOf의 composite key와 ClaimIssued Event가 Issue transaction을 찾게 하고 transaction calldata에서 공개 dppCommitment를 읽습니다.

### 9.2 Claim에서 공급망 부모까지 어떻게 이동하나요?

~~~text
function TraceClaimBackward(dppCommitment, issuePolicyRef, snapshot):
    # 핵심: 공개 Claim에서 Exit의 private parent cm을 복원해 기존 provenance로 이동합니다.
    # 1. Claim의 Issue 기록을 찾습니다.
    issueRecordId = claimRecordOf[dppCommitment][issuePolicyRef] at snapshot
    require issueRecordId != 0

    # 2. 원본 transaction과 암호문 없는 Issue Record를 대조합니다.
    issueTx = FindTransaction(AuditRecorded(issueRecordId))
    require issueTx가 같은 dppCommitment와 issuePolicyRef를 사용
    require issueRecord의 eventKind == ISSUE

    # 3. DPP를 만든 Exit 기록으로 이동합니다.
    exitRecordId = producerOf[DPP][dppCommitment] at snapshot
    require exitRecordId != 0
    require exitRecord의 eventKind == EXIT

    # 4. Exit의 암호문 하나를 위원 둘의 협조로 복호화합니다.
    parentCM = DecryptExitParent(exitRecordId)

    # 5. M7 backward tracing으로 기존 공급망 provenance를 계속 따라갑니다.
    return TraceNoteBackward(parentCM, snapshot)
~~~

Issue 기록 자체는 threshold decryption이 필요 없습니다. Exit 기록에서 private parent cm으로 넘어갈 때부터 기존 위원회 복호화가 필요합니다.

감사는 M7과 같이 block number·hash snapshot을 먼저 고정합니다. transaction·storage·Event가 일치하지 않으면 기록 없음으로 처리하지 않고 명시적으로 실패합니다.

## 10. 무엇을 재사용하고 어디에 구현하나요?

### 10.1 기존 기능과 신규 기능은 무엇인가요?

| 구분 | 기능 |
|---|---|
| 그대로 재사용 | Note commitment·ownership·membership·nf, M7 감사 암호화, AuditRecord 원본 조회, Policy identity·Registry lifecycle |
| 수정 후 재사용 | M7 Exit를 DPP output이 있는 M8 Exit로 확장, ObjectRef decoder에 DPP 추가, AuditRecord에 암호문 없는 Issue 형식 추가 |
| 신규 구현 | DPP commitment, Standard·Strict Issue Circuit, Claim composite mapping·상태·조회, Claim backward adapter |
| 구현하지 않음 | DPP Tree·nullifier, Issue Grant·scope, ownership transfer, Claim Tree, component DPP·Re-entry |

M7 Exit Artifact를 M8 Exit에 재사용하지 않습니다. public input과 statement가 바뀌므로 별도 CCS·PK·VK가 필요합니다. Standard V1과 Strict V2도 Policy constant가 다르므로 각각 별도 Setup을 수행합니다.

### 10.2 예정 폴더 구조는 어떻게 되나요?

~~~text
internal/core/dpp/
  model.go
  commitment.go

features/m8_exit_dpp/
  README.md
  circuit.go
  circuit_test.go

features/issue_standard/
  README.md
  circuit.go
  circuit_test.go

features/issue_strict/
  README.md
  circuit.go
  circuit_test.go

internal/m8case/
cmd/setup_m8/
cmd/evaluate_m8/
cmd/benchmark_m8/

contracts/src/ZkDPPClaimLedger.sol
contracts/test/ZkDPPClaimLedger.t.sol
contracts/test/fixtures/m8-proofs.json

artifacts/development/m8/
  exit-dpp/
  issue-standard-v1/
  issue-strict-v2/
~~~

이 구조는 구현 방향이며 실제 파일명은 기존 package convention과 충돌할 때 최소한으로 조정할 수 있습니다. Protocol 관계·public input 순서는 변경하지 않습니다.

### 10.3 어떤 명령과 결과를 제공하나요?

~~~text
make setup-m8
make evaluate-m8
make test-go
make test-contract-m8
make benchmark-m8-gas
make benchmark-m8-audit
make benchmark-m8
~~~

Raw 결과 예정 위치는 다음입니다.

| 파일 | 내용 |
|---|---|
| output/m8-circuit.json | M8 Exit·Issue V1·V2 constraints, public input, Setup·Prove·Verify·proof 크기 |
| output/m8-anvil-gas.json | 배포·Exit·Issue·상태 변경·조회 gas, calldata와 SSTORE |
| output/m8-audit.json | Claim→Issue→Exit→기존 provenance 조회·복호화·전체 시간 |
| output/m8-generated-checksums.json | Artifact·verifier·fixture checksum과 실행 이력 |

개발용 SRS만 사용합니다. 최종 universal SRS는 M9에서 모든 최종 Circuit 크기를 확인한 뒤 생성합니다.

## 11. 무엇을 확인하면 M8이 완료되나요?

### 11.1 DPP·Exit correctness는 무엇인가요?

- ELIGIBLE·WASTE Note가 모두 Exit하여 DPP commitment를 만들 수 있어야 합니다.
- Note와 DPPPrivateData의 DocumentHash·AssetRole·State가 모두 같아야 합니다.
- dppOpening 또는 dppCommitment를 바꾸면 proof가 실패해야 합니다.
- 잘못된 owner secret·Note path·root·nf·감사 암호문은 실패해야 합니다.
- 이미 소비한 nf, Frozen·Revoked nf와 중복 dppCommitment는 거부해야 합니다.
- producerOf[DPP]가 M8 Exit AuditRecord를 정확히 가리켜야 합니다.
- 실패한 Exit 뒤에는 nf·producerOf·AuditRecord count가 모두 불변이어야 합니다.

### 11.2 Issue Policy boundary는 무엇인가요?

- Standard의 정확한 10%·1.00 boundary는 성공해야 합니다.
- Strict의 정확한 11%·0.97 boundary는 성공해야 합니다.
- 최소 재활용률보다 작은 값과 최대 탄소집약도보다 큰 값은 실패해야 합니다.
- $q_{\mathrm{mass}}=0$, uint64 범위 초과와 WASTE Role은 실패해야 합니다.
- M5 대표 output $(270,30,260)$은 두 Policy에서 모두 성공해야 합니다.
- 같은 Policy VK가 다른 허용 input mass에서도 재사용돼야 합니다.
- public input은 IssuePolicyRef·dppCommitment 두 개만 있어야 합니다.

### 11.3 Registry·Claim·상태는 무엇인가요?

- 미등록·disabled·잘못된 EventKind·arity·verifier Policy는 Issue를 거부해야 합니다.
- producerOf가 없거나 EXIT가 아닌 기록의 dppCommitment는 거부해야 합니다.
- 같은 DPP·같은 policyRef의 두 번째 Issue는 실패해야 합니다.
- 같은 DPP의 Standard·Strict Claim은 각각 등록 가능해야 합니다.
- Policy disable은 신규 Issue만 막고 기존 Claim을 바꾸지 않아야 합니다.
- Status Authority가 아닌 account의 상태 변경과 금지 전이는 실패해야 합니다.
- 한 Policy version의 Freeze·Revoke가 다른 version Claim에 영향을 주지 않아야 합니다.
- verifyClaim은 미등록과 기본 상태값 0을 구분해 반환해야 합니다.
- Issue 실패 뒤 Claim mapping·AuditRecord count·다른 원장 상태는 불변이어야 합니다.

### 11.4 Audit·회귀는 무엇인가요?

- Issue AuditRecord는 빈 암호문을 정상 형식으로 처리해야 합니다.
- Claim에서 Issue transaction·Exit AuditRecord·private parent cm·기존 provenance로 이동해야 합니다.
- Exit parent 복호화는 M7과 같은 위원 조합에서 정확해야 합니다.
- 저장 원본과 transaction input이 다르면 감사가 실패해야 합니다.
- M7의 Entry·Transfer·Proceed·Recall·Merge·Split·Process와 양방향 추적·nf 상태가 회귀하지 않아야 합니다.
- DPP에는 Note·Voucher의 spent mapping을 잘못 적용하지 않아야 합니다.

### 11.5 비용은 어떤 경계로 측정하나요?

| 관점 | 측정 내용 |
|---|---|
| Participant | M8 Exit·Issue V1·V2의 witness·Prove·Verify, constraints, proof·key 크기 |
| Contract | verifier·Main Contract deployment, Exit·Issue·중복 거부·Claim 상태 변경 gas, calldata·SSTORE |
| 제3자 | verifyClaim transaction gas와 eth_call latency를 분리 |
| Auditor·위원회 | Claim 조회, Issue·Exit 원본 대조, Exit parent partial decryption·결합, upstream tracing 전체 시간 |

검증·저장 gas 차이를 순수 SSTORE 비용으로 표현하지 않습니다. 단일 측정값을 평균이나 대규모 DPP 성능으로 일반화하지 않습니다. 기존 M1~M7 Raw 결과는 다시 측정하거나 덮어쓰지 않습니다.

공식 측정은 case별 한 번입니다. 실패·재실행이 있으면 횟수와 원인을 Raw·Result에 함께 기록합니다.

### 11.6 완료 조건은 무엇인가요?

M8은 다음 조건을 모두 만족해야 완료입니다.

1. DPP commitment·M8 Exit·Issue V1·V2의 Native·Circuit 결과가 일치합니다.
2. Go correctness와 기존 회귀 test가 통과합니다.
3. Foundry positive·negative·atomicity test가 통과합니다.
4. 대표 DPP에 Standard·Strict Claim을 등록하고 독립 상태를 조회합니다.
5. Claim에서 Exit와 기존 upstream provenance까지 backward tracing합니다.
6. Artifact 재로딩·checksum·public input 순서를 대조합니다.
7. Raw JSON과 `M8-exit-dpp-issue-result.md`의 수치가 일치합니다.
8. Result가 생성된 뒤에만 MILESTONES의 M8을 완료로 바꿉니다.

## 12. 어디에서 멈추나요?

M8에는 다음을 포함하지 않습니다.

- DPP ownership token·current owner 인증·ownership rotation
- DPP 안의 component DPP 재귀 구조와 Re-entry
- 공급망 중간 단계의 Claim
- Claim Tree·Claim nullifier·Claim 소비
- Issue의 PolicyGrant·owner scope
- 제품 원문을 반드시 공개하는 검증 절차
- QR·NFC·external credential·규제상 DPP 전체 데이터 모델
- DKG·위원회 key rotation·악의적 위원 응답 증명
- 영구 Audit cache·운영 서버
- Besu·permissioning·final universal SRS

이 범위는 dppCommitment에 private Sustainability State를 고정하고, Exit에서 생성한 DPP에 여러 Policy Claim을 붙여 제3자가 상태까지 확인할 수 있는지를 검증하는 데 충분합니다.
