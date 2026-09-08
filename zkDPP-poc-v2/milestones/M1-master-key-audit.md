# M1 — Master Key로 v2 Event와 AuditAndFreeze를 어떻게 검증하나요?

- 상태: 구현 기준 동결
- 구현 계획: [M1 일괄 구현 계획](M1-IMPLEMENTATION-PLAN.md)
- 선택 이유: [M1 Background](M1-master-key-audit-background.md)
- 이전 구현 기준: [`zkDPP-poc-v1` M9](../../zkDPP-poc-v1/milestones/M9-final-integration-result.md)
- 후속 단계: [M2 DKG·M3 Key Rotation Context](FUTURE-MILESTONE-CONTEXT.md)

## 0. 30초 안에 무엇을 구현하나요?

**M1은 v1 M9의 공급망 Event를 v2 lifecycle로 바꾸고, 외부 DKG output으로 가정한 두 Committee share로 Master Key를 복구해 전체 AuditRecord를 감사한 뒤 미소비 frontier를 동결합니다.**

```text
Protocol:
  Exit  = Note → 없음
  Issue = ELIGIBLE Note → ClaimRef(h)

Audit:
  2-of-3 share → SK_A 복구
  → AuditRecord 일괄 복호화
  → Forward Tracing
  → 전체 frontier Freeze
  → H_r에서 결과 확인
```

| 질문 | 구현 기준 |
|---|---|
| Status Authority와 Auditor는 다른가요? | 같은 주체입니다. |
| 실제 DKG를 구현하나요? | 아닙니다. 고정 ExternalKeyPackage를 사용합니다. |
| Audit key는 몇 개인가요? | M1 전체에서 $PK_A,SK_A$ 한 쌍입니다. |
| public key를 회전할 수 있나요? | 없습니다. Circuit 상수로 고정합니다. |
| AuditRecord마다 Committee 응답이 필요한가요? | 없습니다. 두 share로 $SK_A$를 한 번 복구합니다. |
| AuditAndFreeze가 온체인 함수인가요? | 아닙니다. off-chain workflow와 개별 Status transaction입니다. |
| DKG·Rotation·경합 fallback은 어디서 하나요? | M2·M3·후속 검토 범위입니다. |

## 1. M9에서 무엇이 달라지나요?

| 영역 | v1 M9 | v2 M1 |
|---|---|---|
| DocumentInfo | ProductName·LotID | ProductName·LotID·Unit |
| Note nullifier | $H(sk,cm)$ | $H(cm,sk)$ |
| Voucher nullifier | $H(opening,rv)$ | $s_{res}=H(opening)$, $H(rv,s_{res})$ |
| Deadline | timestamp 기반 Epoch | 절대 block number $D$ |
| Transfer 탄소 | 입력 탄소 보존 | Voucher에 운송 탄소 추가 |
| WASTE | Transfer·Merge·Split 가능 | Exit만 가능 |
| Merge·Split 문서 | 새 DocumentHash 허용 | 입력 DocumentHash 보존 |
| Exit | Note→DPP commitment | Note→없음 |
| Issue | DPP에 Claim 추가 | Note를 소비해 ClaimRef 생성 |
| Claim | DPP·Policy composite key와 Status | 공개 $h$, 등록 여부만 존재 |
| 감사 key | Record별 partial decryption | 두 share로 Master Key 복구 |
| 감사 완료 | tracing과 Freeze 분리 | AuditAndFreeze 결과로 연결 |

## 2. 공통 자료형·단위·상수는 무엇인가요?

### 2.1 Field와 Curve

| 항목 | 값 |
|---|---|
| Circuit curve | BLS12-381 |
| Circuit Field modulus | $p$ |
| Audit point curve | Jubjub |
| Jubjub prime subgroup order | $q$ |
| Merkle Tree depth | 32 |
| 질량·탄소 scale | (10^9) |
| 정수 State | uint64 |
| Deadline $D$ | uint64 absolute block number |

State는 다음 세 값입니다.

$$
State=(q_{mass},a_{rec},e)
$$

모든 객체에서 (0\le a_{rec}\le q_{mass})를 확인합니다. WASTE는 (a_{rec}=0,e=0)이어야 합니다.

### 2.2 EventKind와 ObjectType

```text
EventKind:
  Entry=0, Transfer=1, Proceed=2, Recall=3, Merge=4,
  Split=5, Process=6, Exit=7, Issue=8

ObjectType:
  Note=1, Voucher=2, Claim=3
```

Claim은 v1의 DPP ObjectType 위치를 대체합니다. v2 Main Contract에는 DPP ObjectRef를 두지 않습니다.

### 2.3 Hash 관계

사람이 읽는 본문에서는 목적을 드러내는 이름을 사용합니다.

| 이름 | 목적 |
|---|---|
| $H_{note}$ | Note commitment |
| $H_{owner}$ | ZK owner address |
| $H_{nf}$ | Note nullifier |
| $H_{voucher}$ | Voucher commitment |
| $H_{res}$ | Voucher resolution secret |
| $H_{rvnf}$ | Voucher nullifier |
| $H_{policy}$ | PolicyRef |
| $H_{scope}$ | PolicyScopeRef |
| $H_{key}$ | Record masking key |
| $H_{mask}$ | Field 위치별 mask |
| $H_{issue}$ | Claim handle |

기존 구현 식별자는 다음을 유지합니다.

```text
zkDPP:Note:v1
zkDPP:Owner:v1
zkDPP:Nullifier:v1
zkDPP:Voucher:v1
zkDPP:VoucherNullifier:v1
zkDPP:PolicyRef:v1
zkDPP:ScopeRef:v1
zkDPP:AuditKey:v1
zkDPP:AuditMask:v1
```

새 관계에만 다음 목적 식별자를 추가합니다.

```text
zkDPP:VoucherResolutionSecret:v1
zkDPP:Issue:v1
```

`:v2`로 Domain version을 일괄 변경하지 않습니다. Hash는 v1과 같은 SHA-256 hash-to-field 상수와 Poseidon2 Merkle-Damgard 구현을 사용합니다.

Policy 식도 v1 M9을 유지합니다.

$$
policyRef=H_{policy}(eventKind,authorityId,policyId,version)
$$

$$
policyScopeRef=H_{scope}(sk_{owner},policyRef)
$$

## 3. Document·Note·Voucher·Claim은 어떻게 계산하나요?

### 3.1 DocumentInfo

```text
DocumentInfo:
  ProductName
  LotID
  Unit
```

각 문자열을 NFC UTF-8로 정규화합니다. `ProductName → LotID → Unit` 순서로 각 byte 길이를 uint32 big-endian으로 붙이고 원문 byte를 이어 붙입니다. 전체와 각 Field가 uint32 길이를 넘으면 실패합니다.

$$
DocumentHash=HashToField(Encode(DocumentInfo))
$$

### 3.2 Note

```text
Note:
  DocumentHash, AssetRole, q_mass, a_rec, e,
  ownerAddress, opening
```

$$
ownerAddress=H_{owner}(sk_{owner})
$$

$$
cm=H_{note}(DocumentHash,AssetRole,q_{mass},a_{rec},e,ownerAddress,opening)
$$

v2 nullifier는 commitment를 먼저 넣습니다.

$$
nf=H_{nf}(cm,sk_{owner})
$$

### 3.3 Voucher

```text
Voucher:
  DocumentHash, AssetRole, q_mass, a_rec, e,
  senderAddress, receiverAddress, D, opening
```

$$
rv=H_{voucher}(DocumentHash,AssetRole,q_{mass},a_{rec},e,
senderAddress,receiverAddress,D,opening)
$$

$$
s_{res}=H_{res}(opening)
$$

$$
rvnf=H_{rvnf}(rv,s_{res})
$$

Sender와 Receiver는 같은 Voucher opening을 전달받는다고 가정하므로 같은 $rvnf$를 계산합니다. opening의 안전한 전달 Protocol은 M1 범위가 아닙니다.

### 3.4 Claim

```text
DPPClaim:
  issuePolicyRef
  h
  claimNonce
```

$$
h=H_{issue}(DocumentHash,issuePolicyRef,claimNonce)
$$

ClaimRef는 공개 $h$입니다. Claim에는 Tree·opening·owner·nullifier·Status가 없습니다.

## 4. 누가 무엇을 담당하나요?

| 주체 | 책임 | 신뢰 경계 |
|---|---|---|
| System Admin | Main Contract 배포, Entry Issuer·Policy Authority 등록 | 올바른 verifier·hasher·Status Authority를 배포한다고 가정 |
| Entry Issuer | Entry transaction 승인 | 초기 State와 중복 실물 등록 판단을 신뢰 |
| Participant | private 객체·secret·암호화 난수 보관, proof 생성 | 잘못된 witness는 Circuit이 거부 |
| Policy Authority | Process·Issue Policy와 verifier 등록·disable, Process Grant | 등록 Policy의 현실 적합성을 신뢰 |
| Committee member | 외부 DKG share 보관, 승인 시 share release | M1은 올바른 share 전달을 가정 |
| Status Authority = Auditor | Master Key 복구, 감사, frontier Freeze | 올바른 범위·대상·삭제를 신뢰 |
| Main Contract | current root·spent·Status·Policy·proof 확인과 원자적 상태 변경 | private graph나 Master Key를 알지 못함 |
| DPP verifier | 외부 Product 문서와 Claim의 공개 연결 확인 | 문서 추출 profile을 동일하게 사용 |

M1 대표 실행에서는 Entry Issuer와 Note owner가 같은 업무상 Participant를 사용합니다. Contract는 EVM caller와 ZK owner를 암호학적으로 연결하지 않습니다.

## 5. ExternalKeyPackage와 Master Key 복구는 어떻게 동작하나요?

### 5.1 고정 입력

M1은 DKG를 실행하지 않고 다음 자료를 외부 DKG output으로 간주합니다.

```text
ExternalKeyPackage:
  profile
  sessionId
  curve = Jubjub
  threshold = 2
  memberCount = 3
  publicKey = (PK_X, PK_Y)
  publicShares[3]
  packageChecksum

CommitteePrivateShare:
  profile
  sessionId
  memberId
  scalarShare
  publicShare
```

세 private share 파일은 분리하고 file mode 0600으로 생성합니다. Master Key·fixture polynomial·복구한 secret을 파일·로그·Raw JSON에 기록하지 않습니다. 고정값은 로컬 재현용이며 Production secret이 아닙니다.

ExternalKeyPackage의 public key 좌표와 package checksum은 모든 Circuit manifest에 기록합니다. Setup·evaluate·감사 프로그램은 manifest와 package가 다르면 실행을 거부합니다. 이를 통해 다른 고정 key로 compile한 Circuit과 share package를 섞지 않습니다.

### 5.2 fixture 생성

테스트 전용 생성기는 $SK_A\in[1,q-1]$과 nonzero slope $b$를 사용합니다.

$$
f(t)=SK_A+bt\pmod q
$$

$$
sk_u=f(u),\qquad u\in\{1,2,3\}
$$

$$
PK_A=SK_AG,
\qquad
Q_u=sk_uG
$$

이는 DKG가 아니라 DKG output fixture 생성입니다.

### 5.3 복구

```text
RecoverMasterKey(package, releasedShares):
    # 핵심: 서로 다른 두 Committee share로 Master Key를 복구하고 공개키와 대조합니다.
    # 1. package와 share의 profile·session·curve가 같은지 확인합니다.
    require package.threshold == 2 and package.memberCount == 3
    require exactly two released shares
    require memberId가 1..3이고 서로 다름

    # 2. 각 scalar와 public share가 일치하는지 확인합니다.
    require 0 < scalarShare < q
    require scalarShare * G == declared publicShare
    require declared publicShare == package.publicShares[memberId]

    # 3. Lagrange interpolation으로 scalar를 복구합니다.
    SK = sum(lambda(memberId) * scalarShare) mod q
    require 0 < SK < q

    # 4. 복구한 scalar가 공통 public key와 같은지 확인합니다.
    require SK * G == package.publicKey
    return SK
```

위원 조합 $\{1,2\}$, $\{1,3\}$, $\{2,3\}$은 모두 같은 public key에 대응하는 $SK_A$를 복구해야 합니다. 한 share, 중복 ID, 다른 session share 혼합과 변조 public share는 실패합니다.

## 6. AuditRecord 암호화는 어떻게 동작하나요?

### 6.1 공통 관계

$PK_A$는 모든 M1 감사 Event Circuit의 상수입니다. Participant는 Record $i$마다 새로운 $r_i\in[1,q-1]$를 선택합니다.

$$
R_i=r_iG,
\qquad
Z_i=r_iPK_A
$$

$$
K_i=H_{key}(Z_{i,X},Z_{i,Y})
$$

$$
k_{i,j}=H_{mask}(K_i,j)
$$

$$
C_{i,j}=M_{i,j}+k_{i,j}\pmod p
$$

$L,n$은 사용하지 않습니다. 위치 $j$는 부모 배열을 먼저, output 미래 소비값 배열을 다음에 이어 붙인 전체 평문 벡터에서 0부터 증가합니다.

### 6.2 Circuit gadget

```text
AssertEncrypted(plaintext[], r, publicR1, publicCiphertext[]):
    # 핵심: 실제 Event 값이 고정 PK_A 아래에서 위치별 mask로 암호화됐는지 확인합니다.
    # 1. 암호화 난수 범위를 확인합니다.
    require 0 < r < q

    # 2. 공개점과 공유점을 계산합니다.
    expectedR1 = r * G
    require expectedR1 == publicR1
    Z = r * fixedPK_A

    # 3. Record key와 위치별 mask를 계산합니다.
    K = H_key(Z.X, Z.Y)
    for j in 0..len(plaintext)-1:
        mask = H_mask(K, j)
        require publicCiphertext[j] == plaintext[j] + mask mod p
```

빈 평문은 Issue·Exit에서도 부모가 하나이므로 발생하지 않습니다. Event별 배열 길이는 compile-time 고정이며 inactive padding을 만들지 않습니다.

### 6.3 Master Key 복호화

```text
DecryptRecord(SK, record, expectedShape):
    # 핵심: 복구한 Master Key로 한 Record의 부모와 output 미래 소비값을 복원합니다.
    # 1. Record 공개점과 배열 shape를 확인합니다.
    require R1이 유효한 Jubjub prime-subgroup point
    require ciphertext 길이가 expectedShape와 같음

    # 2. 같은 공유점과 Record key를 계산합니다.
    Z = SK * R1
    K = H_key(Z.X, Z.Y)

    # 3. 위치별 mask를 빼서 평문을 복원합니다.
    for j in 0..ciphertext.length-1:
        plaintext[j] = ciphertext[j] - H_mask(K, j) mod p
    return plaintext
```

## 7. AuditRecord와 Main Contract는 어떤 상태를 사용하나요?

### 7.1 AuditRecord

```text
AuditRecord:
  eventKind
  policyRef
  outputRefs[]
  R1X
  R1Y
  encryptedParents[]
  encryptedOutputNfs[]
```

AuditRecord ID는 1부터 증가하는 mapping key이며 Record 안에 중복 저장하지 않습니다.

Entry·Transfer·Proceed·Recall·Merge·Split·Exit의 `policyRef`는 0입니다. Process와 Issue만 실제 PolicyRef를 저장합니다.

| Event | 부모 평문 | output 미래 소비값 | outputRefs |
|---|---|---|---|
| Entry | 없음 | output $nf$ | NoteRef |
| Transfer | input $cm$ | Voucher $rvnf$, Change $nf$ | VoucherRef, NoteRef |
| Proceed | input $rv$ | Receiver $nf$ | NoteRef |
| Recall | input $rv$ | Return $nf$ | NoteRef |
| Merge | input $cm$ 2개 | output $nf$ | NoteRef |
| Split | input $cm$ | output $nf$ 2개 | NoteRef 2개 |
| Process | input $cm$ 3개 | ELIGIBLE $nf$, WASTE $nf$ | NoteRef 2개 |
| Exit | input $cm$ | 없음 | 없음 |
| Issue | input $cm$ | 없음 | ClaimRef |

### 7.2 Main Contract

새 Main Contract 이름은 `ZkDPPV2Ledger`로 고정합니다. v1 Contract를 상속하거나 수정하지 않습니다.

Constructor는 다음을 받습니다.

```text
constructor(
  address[7] fixedEventVerifiers,
  address fieldHasher,
  address statusAuthority
)
```

고정 verifier 순서는 Entry, Transfer, Proceed, Recall, Merge, Split, Exit입니다. Process·Issue verifier는 기존 PolicyRecord의 `verifierRef`에서 선택합니다.

M1의 주요 ABI는 다음과 같습니다.

```text
entry(proof, cm, auditCipher) → aid
transfer(proof, noteRoot, nf, rvNew, cmChange, D, auditCipher) → aid
proceed(proof, voucherRoot, rvnf, cmReceiver, auditCipher) → aid
recall(proof, voucherRoot, rvnf, cmReturn, D, auditCipher) → aid
merge(proof, noteRoot, nf1, nf2, cmOut, auditCipher) → aid
split(proof, noteRoot, nf, cmOut1, cmOut2, auditCipher) → aid
process(proof, policyRef, policyScopeRef, noteRoot, nf[3], cmOut[2], auditCipher) → aid
exit(proof, noteRoot, nf, auditCipher) → aid
issue(proof, issuePolicyRef, noteRoot, nf, h, auditCipher) → aid
setStatus(objectType, spendValue, newStatus)
```

public input 수는 다음으로 고정합니다.

| Event Circuit | public input 수 |
|---|---:|
| Entry | 4 |
| Transfer | 10 |
| Proceed | 7 |
| Recall | 8 |
| Merge | 9 |
| Split | 9 |
| Process | 15 |
| Exit | 5 |
| Issue Standard | 7 |
| Issue Strict | 7 |

핵심 storage:

```text
NoteTree, VoucherTree
commitments[cm]
voucherCommitments[rv]
producerOf[objectType][rawId]
noteSpentIn[nf]
voucherSpentIn[rvnf]
noteStatusByNf[nf]
voucherStatusByNf[rvnf]
claimRegistered[h]
auditRecords[aid]
nextAuditRecordId
Policy Registry·Process Grant
```

v2 Main Contract에는 `dppCommitment`, DPP producer, `claimRecordOf`, `claimStatus`, `setClaimStatus`가 없습니다.

### 7.3 공통 상태 변경

```text
CompleteEvent(kind, proof, publicInputs, spends, outputRefs, auditCipher):
    # 핵심: proof·소비·output·AuditRecord를 한 transaction에서 원자적으로 기록합니다.
    # 1. Event별 권한·root·Status·Policy·shape·중복을 확인합니다.
    require all current checks pass

    # 2. 정확한 고정 또는 Policy verifier로 proof를 확인합니다.
    require verifier.Verify(proof, publicInputs)

    # 3. 새로운 AuditRecord ID를 배정합니다.
    aid = nextAuditRecordId

    # 4. input 소비와 output을 기록합니다.
    write noteSpentIn 또는 voucherSpentIn
    append Note·Voucher output to matching Tree
    write Claim registration when output is Claim
    write producerOf for every outputRef

    # 5. AuditRecord를 저장하고 ID를 증가시킵니다.
    auditRecords[aid] = immutable record
    nextAuditRecordId = aid + 1
```

한 조건이라도 실패하면 spent·Tree·commitment·Claim·producer·AuditRecord ID가 모두 이전 상태로 돌아갑니다.

## 8. Entry는 어떻게 동작하나요?

```text
전이: 없음 → ELIGIBLE Note
public input: cm, R1X, R1Y, encryptedOutputNf
private witness: DocumentInfo, Note, sk_owner, r
```

입력 API에서 생략한 $e_{initial}$은 0으로 정규화합니다. 명시적 0과 유효한 양수는 그대로 사용하며 음수·uint64 초과 입력을 0으로 바꾸지 않습니다.

```text
EntryCircuit:
    # 핵심: 승인받은 Participant의 초기 Note와 미래 nf 암호화를 검증합니다.
    # 1. Unit을 포함한 DocumentHash와 ELIGIBLE State를 확인합니다.
    require q_mass > 0 and 0 <= a_rec <= q_mass

    # 2. owner secret과 commitment를 확인합니다.
    require ownerAddress == H_owner(sk_owner)
    cm = CommitNote(private Note)
    require cm == public.cm

    # 3. 미래 소비값을 계산하고 암호화를 확인합니다.
    nf = H_nf(cm, sk_owner)
    AssertEncrypted([nf], r, public.R1, [public.encryptedOutputNf])
```

Contract는 `entryIssuers[msg.sender]`, 새 $cm$, Field 범위와 proof를 확인합니다. 성공하면 Note Tree·commitments·producerOf·Entry AuditRecord를 기록합니다.

## 9. Transfer는 어떻게 동작하나요?

```text
전이: ELIGIBLE Note → ELIGIBLE Voucher + ELIGIBLE Change Note
public input:
  noteRoot, nf, rvNew, cmChange, D,
  R1X, R1Y, encryptedParentCM, encryptedVoucherRvnf, encryptedChangeNf
private witness:
  input Note·index·path·sk_owner, receiverAddress,
  Voucher·Change Note, allocation remainder,
  delta_e_transport, r
```

질량과 재활용량:

$$
q_{in}=q_T+q_C,
\qquad q_T>0
$$

$$
a_C=\left\lfloor\frac{a_{in}q_C}{q_{in}}\right\rfloor,
\qquad a_T=a_{in}-a_C
$$

탄소:

$$
e_C=\left\lfloor\frac{e_{in}q_C}{q_{in}}\right\rfloor
$$

$$
e_T=e_{in}-e_C+\Delta e_{transport}
$$

$\Delta e_{transport}$는 private uint64이고 $e_T$ overflow를 거부합니다. 전량 Transfer는 $q_C=a_C=e_C=0$인 무작위-opening Change Note를 만듭니다.

```text
TransferCircuit:
    # 핵심: ELIGIBLE Note를 보존된 문서와 운송 탄소가 반영된 Voucher·Change로 나눕니다.
    # 1. input membership·owner·nf와 ELIGIBLE role을 확인합니다.
    require input cm in noteRoot
    require input owner == H_owner(sk_owner)
    require public.nf == H_nf(input cm, sk_owner)
    require input.role == ELIGIBLE

    # 2. 질량·재활용량·기존 탄소의 floor·residual 배분을 확인합니다.
    require allocation equations and remainder bounds

    # 3. 운송 탄소를 Voucher에만 추가합니다.
    require voucher.e == input.e - change.e + delta_e_transport
    require all State values fit uint64

    # 4. 문서·소유자·deadline·output commitment를 확인합니다.
    require output DocumentHash == input DocumentHash
    require Voucher.sender == Change.owner == input.owner
    require Voucher.D == public.D
    require rvNew and cmChange are correct

    # 5. 부모와 두 output 미래 소비값의 암호화를 확인합니다.
    rvnf = VoucherNullifier(Voucher)
    changeNf = NoteNullifier(Change)
    AssertEncrypted([inputCM, rvnf, changeNf], r, public.R1, public.C)
```

Contract는 accepted root, $nf$의 미소비·Active, `block.number < D`, 새 output과 proof를 확인합니다. 성공하면 Note 소비·Voucher Tree·Note Tree·producerOf·AuditRecord를 원자적으로 기록합니다.

## 10. Proceed는 어떻게 동작하나요?

```text
전이: ELIGIBLE Voucher → Receiver ELIGIBLE Note
public input:
  voucherRoot, rvnf, cmReceiver,
  R1X, R1Y, encryptedParentRV, encryptedReceiverNf
private witness:
  Voucher·index·path, receiver sk_owner, Receiver Note, r
```

Circuit은 Voucher membership, $rvnf$, receiver secret, DocumentHash·State 보존과 부모·output 미래 $nf$ 암호화를 확인합니다. Contract는 accepted root, $rvnf$의 미해결·Active, 새 $cm$과 proof를 확인합니다. Proceed는 $D$ 전후 모두 가능합니다.

성공하면 voucherSpentIn, Receiver Note Tree append, producerOf와 AuditRecord가 같은 ID를 사용합니다.

## 11. Recall은 어떻게 동작하나요?

```text
전이: ELIGIBLE Voucher → Sender ELIGIBLE Return Note
public input:
  voucherRoot, rvnf, cmReturn, D,
  R1X, R1Y, encryptedParentRV, encryptedReturnNf
private witness:
  Voucher·index·path, sender sk_owner, Return Note, r
```

Circuit은 Voucher의 private $D$가 public $D$와 같고 sender secret·문서·State·$rvnf$·암호문이 올바른지 확인합니다. Contract는 `block.number <= D`를 직접 확인합니다.

Proceed와 Recall은 같은 $rvnf$를 사용하며 먼저 성공한 하나만 voucherSpentIn을 기록합니다. $D+1$ 블록부터 Recall은 실패하고 Proceed는 계속 가능합니다.

## 12. Merge는 어떻게 동작하나요?

```text
전이: ELIGIBLE Note 2개 → ELIGIBLE Note 1개
public input:
  noteRoot, nf1, nf2, cmOut,
  R1X, R1Y, encryptedParentCM1, encryptedParentCM2, encryptedOutputNf
private witness:
  input Note·index·path 2개, common sk_owner, output Note, r
```

Circuit은 두 입력이 서로 다르고 같은 owner·DocumentHash·ELIGIBLE role인지 확인합니다. Output은 같은 owner·DocumentHash·role을 유지하고 세 State를 checked uint64 addition으로 더합니다. 부모 $cm$ 두 개와 output $nf$ 암호화를 확인합니다.

Contract는 두 $nf$의 distinct·미소비·Active, 새 output과 proof를 확인하고 소비 2개·Note append 1개·AuditRecord를 원자적으로 기록합니다.

## 13. Split은 어떻게 동작하나요?

```text
전이: ELIGIBLE Note 1개 → ELIGIBLE Note 2개
public input:
  noteRoot, nf, cmOut1, cmOut2,
  R1X, R1Y, encryptedParentCM, encryptedOutputNf1, encryptedOutputNf2
private witness:
  input Note·index·path·sk_owner, output Note 2개,
  allocation remainder, r
```

Output 2가 floor를 받고 Output 1이 residual을 받습니다. 두 output은 입력 owner·DocumentHash·ELIGIBLE role을 유지합니다. 질량 0 output 하나를 허용하지만 그 output의 $a_{rec},e$도 0이어야 합니다.

Circuit은 State 합·floor·remainder·두 output distinctness와 감사 암호화를 확인합니다. Contract는 input 상태, 두 새 output과 proof를 확인하고 Note 소비 1개·append 2개를 원자적으로 기록합니다.

## 14. Process는 어떻게 동작하나요?

```text
전이: ELIGIBLE Note 3개 → ELIGIBLE Note + WASTE Note
public input:
  policyRef, policyScopeRef, noteRoot, nf1, nf2, nf3,
  cmEligible, cmWaste,
  R1X, R1Y,
  encryptedParentCM1, encryptedParentCM2, encryptedParentCM3,
  encryptedEligibleNf, encryptedWasteNf
private witness:
  input Note·index·path 3개, common sk_owner,
  intermediate·output State, remainder, output Note 2개, r
```

M1은 v1 M9의 Process Policy를 유지합니다.

| 상수 | 값 |
|---|---:|
| input/output arity | 3-to-2 |
| 질량 손실률 | 6.25% |
| 탄소 추가 | input 1 kg당 0.09375 kgCO2e |
| WASTE 질량 | intermediate 질량의 10% |
| output roles | ELIGIBLE, WASTE |

입력은 같은 owner의 서로 다른 ELIGIBLE Note입니다. Process는 새로운 output DocumentHash를 허용합니다. 모든 $a_{rec}$과 탄소는 ELIGIBLE output에 귀속하고 WASTE는 질량만 가집니다.

Contract는 PROCESS·3-to-2 PolicyRecord, enabled, PolicyGrant, root, 세 $nf$의 distinct·미소비·Active, output과 proof를 확인합니다.

## 15. Exit는 어떻게 동작하나요?

```text
전이: ELIGIBLE 또는 WASTE Note → 없음
public input:
  noteRoot, nf, R1X, R1Y, encryptedParentCM
private witness:
  input Note·index·path·sk_owner, r
```

```text
ExitCircuit:
    # 핵심: Note를 한 번 소비하고 successor 없이 종료하며 실제 부모만 암호화합니다.
    # 1. Note·owner·membership·nf를 확인합니다.
    require input Note is valid ELIGIBLE or WASTE
    require input cm in noteRoot
    require owner == H_owner(sk_owner)
    require public.nf == H_nf(input cm, sk_owner)

    # 2. 부모 cm 암호화를 확인합니다.
    AssertEncrypted([inputCM], r, public.R1, [public.encryptedParentCM])
```

Contract는 root, $nf$의 미소비·Active와 proof를 확인합니다. 성공하면 noteSpentIn과 Exit AuditRecord만 기록합니다. Tree·commitment·producerOf·Claim은 변경하지 않습니다. Record의 outputRefs와 encryptedOutputNfs는 비어 있습니다.

## 16. Issue는 어떻게 동작하나요?

```text
전이: ELIGIBLE Note → ClaimRef(h)
public input:
  issuePolicyRef, noteRoot, nf, h,
  R1X, R1Y, encryptedParentCM
private witness:
  DocumentInfo, input Note·index·path·sk_owner,
  claimNonce, r
```

Standard·Strict 기준은 v1 M9을 유지합니다.

| Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

```text
IssueCircuit:
    # 핵심: final ELIGIBLE Note를 소비하고 외부 Product DPP에 붙일 Claim h를 생성합니다.
    # 1. PolicyRef와 Note membership·owner·nf를 확인합니다.
    require public.issuePolicyRef == fixed policyRef
    require input role == ELIGIBLE and q_mass > 0
    require input cm in public.noteRoot
    require owner == H_owner(sk_owner)
    require public.nf == H_nf(input cm, sk_owner)

    # 2. Unit을 포함한 DocumentHash와 Note를 연결합니다.
    documentHash = HashDocumentInfo(ProductName, LotID, Unit)
    require documentHash == input.DocumentHash

    # 3. Policy threshold를 정수 교차곱으로 확인합니다.
    require a_rec * denominator_rec >= q_mass * numerator_rec
    require e * denominator_carbon <= q_mass * numerator_carbon
    require no integer overflow

    # 4. 공개 Claim handle을 계산합니다.
    expectedH = H_issue(documentHash, issuePolicyRef, claimNonce)
    require expectedH == public.h

    # 5. 실제 부모 cm 암호화를 확인합니다.
    AssertEncrypted([inputCM], r, public.R1, [public.encryptedParentCM])
```

```text
issue(proof, issuePolicyRef, noteRoot, nf, h, auditCipher):
    # 핵심: Note 소비·Claim 등록·producer·AuditRecord를 원자적으로 기록합니다.
    # 1. Issue Policy와 input 상태를 확인합니다.
    require PolicyRecord exists and enabled
    require eventKind == ISSUE and arity == 1-to-1
    require accepted noteRoot
    require noteSpentIn[nf] == 0 and noteStatusByNf[nf] == Active
    require claimRegistered[h] == false

    # 2. Policy verifier로 정확한 public input의 proof를 확인합니다.
    require policy.verifierRef.Verify(proof, publicInputs)

    # 3. 소비·Claim·AuditRecord를 같은 aid에 연결합니다.
    aid = nextAuditRecordId
    noteSpentIn[nf] = aid
    claimRegistered[h] = true
    producerOf[CLAIM][h] = aid
    auditRecords[aid] = Issue record with outputRefs=[ClaimRef(h)]
    nextAuditRecordId = aid + 1
```

같은 Note는 이미 소비되므로 다른 nonce나 Policy로 재Issue할 수 없습니다. Claim에는 미래 소비 암호문과 Status가 없습니다.

## 17. 외부 DPP는 Claim을 어떻게 확인하나요?

```text
VerifyDPPClaim(ledger, snapshot, DPP, DPPClaim):
    # 핵심: 외부 Product 문서에서 계산한 Claim이 실제 Issue 결과인지 확인합니다.
    # 1. DPP에서 세 문서 필드를 정해진 profile로 추출합니다.
    info = (ProductName, LotID, Unit)
    require canonical field types and encoding

    # 2. 공개 Claim handle을 다시 계산합니다.
    documentHash = HashDocumentInfo(info)
    expectedH = H_issue(documentHash, issuePolicyRef, claimNonce)
    require expectedH == h

    # 3. Snapshot에서 Claim 등록과 producer를 확인합니다.
    require claimRegistered[h] == true
    aid = producerOf[CLAIM][h]
    require aid != 0

    # 4. Issue AuditRecord를 확인합니다.
    record = LoadAndVerifyOriginal(aid, snapshot)
    require record.eventKind == ISSUE
    require record.policyRef == issuePolicyRef
    require record.outputRefs == [ClaimRef(h)]

    # 5. Claim이 의미하는 공개 Policy를 확인합니다.
    policy = policyRecords[issuePolicyRef]
    require policy.eventKind == ISSUE and policy.arity == 1-to-1
    require policy manifest와 verifier hash가 알려진 Issue Policy와 같음
    return valid
```

현재 Policy가 disable됐다는 이유로 과거 accepted Claim을 무효화하지 않습니다. 추가 Product 필드와 물리 제품 결합, 전체 규제 준수는 이 결과로 증명하지 않습니다.

## 18. Status는 어떻게 집행하나요?

M1은 Note·Voucher에만 기존 상태 전이를 사용합니다.

$$
Active\rightarrow Frozen,
\qquad
Frozen\rightarrow Active,
\qquad
Frozen\rightarrow Revoked
$$

```text
setStatus(objectType, spendValue, newStatus):
    # 핵심: Status Authority만 미소비 Note·Voucher의 공개 소비값 상태를 변경합니다.
    require msg.sender == statusAuthority
    require objectType is NOTE or VOUCHER
    require spendValue is a canonical Field value
    require matching spentIn[spendValue] == 0
    require currentStatus → newStatus is allowed
    write matching status mapping
```

Revoked는 terminal입니다. Status mapping 기본값 Active는 실제 객체 존재 증명이 아니며 Status Authority가 감사에서 얻은 올바른 $nf$·$rvnf$를 제출한다고 신뢰합니다.

## 19. Forward Tracing은 어떻게 동작하나요?

Snapshot은 다음 두 값입니다.

$$
H_t=(blockNumber_t,blockHash_t)
$$

RPC source는 chain ID·Ledger 주소·배포 code hash·Snapshot block hash를 확인하고 모든 `eth_call`, Event log, transaction·receipt 조회를 $blockNumber_t$에 고정합니다.

```text
TraceForward(H_t, start, SK_A):
    # 핵심: output의 미래 nf·rvnf를 Master Key로 복원해 모든 downstream을 탐색합니다.
    require Snapshot and deployment identity are valid
    queue = start ObjectRef 또는 start AuditRecord의 모든 outputRefs

    while queue is not empty:
        object = pop(queue)
        require object and producer record agree at H_t

        if object is ClaimRef:
            add terminal Claim
            continue

        plaintext = DecryptRecord(SK_A, producerRecord)
        spendValue = object 위치의 output 미래 소비값
        consumerAid = typed spentIn[spendValue] at H_t

        if consumerAid == 0:
            add object and spendValue to typed frontier
            continue

        consumer = LoadAndVerifyOriginal(consumerAid, H_t)
        require public consumed spendValue and record agree

        if consumer.outputRefs is empty:
            require consumer is Exit
            add terminated Exit
            continue

        append every consumer output to queue

    return graph, frontierNotes, frontierVouchers,
           terminalClaims, terminatedExits
```

조회·원본·암호문·shape 오류를 미소비 frontier로 처리하지 않습니다. 같은 Record 복호화는 한 감사 실행의 memory cache에서 재사용합니다.

## 20. AuditAndFreeze는 어떻게 동작하나요?

AuditAndFreeze는 새 Contract 함수가 아니라 Status Authority가 실행하는 off-chain workflow입니다.

```text
AuditAndFreeze(start, H_t, releasedShares):
    # 핵심: 전체 downstream frontier를 찾아 개별 Freeze하고 공통 H_r에서 완료를 판정합니다.
    # 1. Master Key를 복구하고 public key를 대조합니다.
    SK_A = RecoverMasterKey(fixedPackage, releasedShares)

    # 2. 고정 Snapshot에서 전체 graph와 frontier를 복원합니다.
    trace = TraceForward(H_t, start, SK_A)
    if trace fails:
        zero Freeze transactions
        return TRACE_FAILED

    # 3. 전체 typed frontier를 고정합니다.
    T = DeduplicateTyped(trace.frontierNotes ∪ trace.frontierVouchers)
    if T is empty:
        return NO_LIVE_TARGETS

    # 4. target별 현재 상태를 읽고 필요한 Freeze를 제출합니다.
    for target in T:
        current = ReadCurrentSpentAndStatus(target)
        if current is unspent and Active:
            submit setStatus(target, Frozen)
        else:
            record already-blocked, consumed, query-failed or invalid status
        keep every target in result

    # 5. 모든 시도 뒤 공통 결과 Snapshot을 고정합니다.
    H_r = CaptureConfirmedBlockNumberAndHash()

    # 6. H_r에서 전체 T를 다시 확인합니다.
    for target in T:
        final = ReadSpentAndStatusAt(H_r, target)
        blocked = final.unspent and final.status in {Frozen, Revoked}

    # 7. 전체 대상이 차단됐을 때만 완료로 판정합니다.
    if every target is confirmed blocked at H_r:
        outcome = COMPLETE_AT_CHECKPOINT
    else:
        outcome = INCOMPLETE

    # 8. key material은 세션 종료 뒤 best-effort zeroize합니다.
    Zeroize(SK_A and released share copies)
    return trace, T, targetResults, H_r, outcome
```

`COMPLETE_AT_CHECKPOINT`는 $H_r$ 시점의 결과이며 영구 차단을 뜻하지 않습니다. M1은 post-snapshot consumer를 따라가지 않습니다. 먼저 소비된 target은 그대로 `INCOMPLETE` 원인으로 남깁니다.

## 21. 대표 실행 예시는 무엇인가요?

### 21.1 전체 Event lifecycle

M9의 원자재 State를 재사용하고 Aluminum A의 첫 Transfer에만 1 kgCO2e 운송 탄소를 추가합니다. 다른 Transfer의 증분은 0입니다.

```text
원자재 Entry
  → Transfer·Recall·재Transfer·Proceed
  → Merge
  → Process
  → Split
  ├─ Product 1 Note → Standard Issue → Claim 1
  ├─ Product 2 Note → Strict Issue → Claim 2
  └─ WASTE Note → Exit → 종료
```

운송 탄소를 포함한 Process input 합은 ((320,30,231))이고 Process carbon 30을 추가한 output은 다음입니다.

```text
ELIGIBLE = (270, 30, 261)
WASTE    = (30, 0, 0)
```

ELIGIBLE을 200 kg과 70 kg으로 Split하면 두 output 모두 재활용률 약 11.11%, 탄소집약도 약 0.967이므로 Standard·Strict 경계를 통과합니다. 같은 Note를 두 Policy에 재사용하지 않고 서로 다른 Product Note를 각각 Issue합니다.

### 21.2 AuditAndFreeze

```text
Entry A
  → Split
  ├─ B → Split → D, E
  └─ C
```

Event 생성이 끝난 뒤 두 share로 $SK_A$를 복구합니다. A에서 Forward Tracing하면 미소비 C·D·E와 각 $nf$를 얻습니다. 세 값을 개별 Freeze하고 $H_r$에서 모두 미소비·Frozen인지 확인해 `COMPLETE_AT_CHECKPOINT`를 반환합니다.

### 21.3 post-snapshot 경합

별도 negative case에서는 $H_t$ 이후 target 하나를 먼저 소비합니다. Freeze가 거부되고 최종 결과는 `INCOMPLETE`입니다. 새 자손 partial decryption fallback은 호출하지 않습니다.

## 22. 무엇을 반드시 검증하나요?

### Master Key

- 세 두-위원 조합이 같은 public key에 대응하는 Master Key를 복구합니다.
- share 한 개·중복 ID·다른 session·변조 scalar/public share를 거부합니다.
- 잘못된 Master Key로 AuditRecord 복호화 결과가 원문과 일치하지 않습니다.
- Master Key·share가 로그·Raw JSON·Git 대상에 나타나지 않습니다.

### 객체·Event

- Unit만 바꾸면 DocumentHash·Note commitment·Claim $h$가 달라집니다.
- Merge의 서로 다른 DocumentHash와 Split의 변조 output DocumentHash를 거부합니다.
- 생성 때 암호화한 $nf$·$rvnf$가 실제 소비값과 같습니다.
- 운송 탄소 0·양수·overflow, 부분·전량 Transfer와 remainder를 확인합니다.
- WASTE의 Transfer·Merge·Split·Process·Issue를 거부하고 Exit를 허용합니다.
- Transfer는 `block.number == D`에서 거부하고 Recall은 같은 블록에서 허용합니다.
- Recall은 (D+1)에서 실패하고 Proceed는 성공할 수 있습니다.
- Exit는 output을 만들지 않고 Issue는 Claim 하나를 만듭니다.
- 같은 Note의 재Issue·Issue 후 Exit·Exit 후 Issue를 중복 소비로 거부합니다.
- Claim status 함수와 DPP commitment 상태가 존재하지 않습니다.

### 암호화

- 일반 계산과 Circuit의 $R_i,K_i,k_{i,j},C_{i,j}$가 같습니다.
- Record마다 다른 $r_i,R_i,K_i$를 사용합니다.
- 같은 Record의 위치별 mask가 서로 다릅니다.
- 잘못된 부모·output 미래 소비값·순서·암호문·공개점을 거부합니다.
- $L,n$이 Circuit·witness·public input·Artifact metadata에 없습니다.

### Contract·감사

- 모든 Event 실패 뒤 Tree·spent·producer·Claim·AuditRecord가 불변입니다.
- Claim에서 Issue·upstream Entry까지 backward tracing이 가능합니다.
- Master Key Forward Tracing이 정확한 전체 graph·frontier·terminal을 반환합니다.
- Trace 실패 시 Freeze transaction 수가 0입니다.
- Active·Frozen·Revoked·소비됨·조회 실패를 구분합니다.
- 모든 target을 같은 $H_r$에서 확인합니다.
- 일부 실패를 `COMPLETE_AT_CHECKPOINT`로 보고하지 않습니다.

## 23. 무엇을 측정하나요?

공식 case는 각각 한 번 실행하며 평균으로 주장하지 않습니다.

| 관점 | 측정 |
|---|---|
| Participant | Event별 native encryption·constraints·witness·Prove·Verify·proof 크기·할당 메모리 |
| Master Key | 조합별 복구·public-key 대조 시간 |
| Bulk decrypt | 독립 Record 1·10·100·1,000개 전체 복호화 시간과 원문 일치 |
| Contract | verifier·Ledger deployment, Event·Claim·Status gas·calldata·SSTORE |
| Audit | Snapshot·RPC·복호화·traversal·Freeze submission·$H_r$ 확인·전체 시간 |
| 비교 | v1 M9 Raw의 Record별 partial-decryption 응답 수·시간과 v2 M1 결과 |

할당 메모리를 peak RSS로 표현하지 않습니다. AuditRecord workload를 실제 대규모 공급망 감사 성능으로 일반화하지 않습니다. v1 결과는 재측정하거나 덮어쓰지 않습니다.

## 24. 구현·Artifact·명령은 어떻게 구성하나요?

예정 source 경계:

```text
internal/core/auditcrypto
internal/core/document·note·voucher·claim
internal/audit
features/audit_<event>
features/issue_claim
contracts/src/ZkDPPV2Ledger.sol
internal/m1case
cmd/setup_m1·evaluate_m1·benchmark_m1
```

개발 Artifact:

```text
artifacts/development/m1/key-package
artifacts/development/m1/circuits/<relation>
contracts/src/generated/m1-*/PlonkVerifier.sol
contracts/test/fixtures/m1-proofs.json
```

Git에는 source·문서·Raw JSON만 포함합니다. private shares·Master Key·SRS·PK·VK·proof·generated verifier·fixture는 ignore합니다.

예정 명령:

```text
make setup-m1
make evaluate-m1
make test-go
make test-contract-m1
make benchmark-m1-key-recovery
make benchmark-m1-decrypt
make benchmark-m1-anvil
make benchmark-m1-audit
make benchmark-m1
make check-m1
```

Raw 결과:

```text
output/m1-circuit.json
output/m1-key-recovery.json
output/m1-anvil.json
output/m1-audit.json
output/m1-generated-checksums.json
```

Aggregate와 개별 benchmark를 중복 실행하지 않습니다. `check-m1`은 proof나 benchmark를 다시 만들지 않고 public input·Artifact·checksum·Raw 일치만 확인합니다.

## 25. 완료 조건은 무엇인가요?

- 위 positive·negative·atomicity Gate가 모두 통과합니다.
- 저장한 Artifact를 checksum 확인 후 다시 읽어 모든 proof를 검증합니다.
- Event별 public input 순서와 Solidity verifier·fixture가 같습니다.
- 일반 계산·Circuit·Contract·복호화 평문·graph·Status 결과가 일치합니다.
- 기존 `zkDPP-poc-v1`과 Conversation History checksum이 변하지 않습니다.
- 실제 DKG·Rotation·post-snapshot fallback을 구현했다고 주장하지 않습니다.
- [`RESULT-TEMPLATE.md`](RESULT-TEMPLATE.md)에 따른 `M1-master-key-audit-result.md`를 생성합니다.
- Result와 Raw JSON을 대조한 뒤에만 M1을 완료로 표시합니다.
