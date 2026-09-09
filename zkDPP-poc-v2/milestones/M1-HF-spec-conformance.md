# M1-HF — 명세와 구현을 어떻게 다시 일치시키나요?

- 상태: 구현 기준 동결
- 실제 결과: [M1-HF Result](M1-HF-spec-conformance-result.md)
- 기준 구현: M1 완료 commit `8560fd3`
- 기존 명세: [M1 Master Key Audit](M1-master-key-audit.md)
- 선택 이유: [M1 Background](M1-master-key-audit-background.md)
- 기존 결과: [M1 Result](M1-master-key-audit-result.md)

## 0. 30초 안에 무엇을 고치나요?

**M1-HF는 M1의 Protocol 구조를 바꾸는 단계가 아닙니다. 이미 확정한 동작을 정확히 구현하고, 실제 실행 결과만을 검증 근거로 남기는 Hotfix입니다.**

```text
Forward Tracing
  → Claim·Exit를 정상 terminal로 구분
  → 전체 Note·Voucher frontier 확정

AuditAndFreeze
  → target별 처리 결과 보존
  → Active만 Freeze
  → Frozen·Revoked를 모두 차단 완료로 인정
  → 하나의 검증된 H_r에서 전체 재확인

검증·측정
  → 실제 Anvil transaction에서 gas 수집
  → manifest 생성과 읽기 전용 check 분리
  → 실제 proof를 모든 Event의 Ledger 경로에 연결
```

함께 수정하는 관계는 두 가지입니다.

- M1에서 사용하는 모든 활성 Hash Domain의 version을 `v1`에서 `v2`로 변경합니다.
- Process는 ELIGIBLE output의 총질량이 0인 경우를 거부합니다.

변경하지 않는 핵심 결정도 명확합니다.

- Committee share 두 개로 한 단계 Master Key $SK_A$를 복구합니다.
- 두 번째 wrapping key를 추가하지 않습니다.
- PolicyRef의 입력 tuple은 기존 `eventKind, authorityId, policyId, version`을 유지합니다.
- DKG·Key Rotation·post-snapshot 자손 fallback은 구현하지 않습니다.

## 1. 왜 별도 Hotfix로 분리하나요?

M1은 Master Key 복구, 10개 Circuit, Ledger, Snapshot RPC와 대표 AuditAndFreeze 실행을 처음 연결했습니다. 그러나 기존 Result가 성공한 대표 경로만으로는 다음 경우를 확인할 수 없었습니다.

- downstream 일부가 Claim 또는 Exit로 끝나는 경우
- frontier에 이미 Revoked된 객체가 있는 경우
- Freeze 제출 또는 receipt 확인 결과가 불명확한 경우
- 결과 확인 중 block hash가 달라지는 경우
- 현재 코드로 benchmark와 check를 다시 실행하는 경우

M1의 코드·Raw 결과·Result는 당시 구현 기록으로 보존합니다. Hotfix는 별도 Artifact·Raw·Result를 만들고, 이전 값을 조용히 덮어쓰지 않습니다.

## 2. 어떤 결정은 유지하나요?

### 2.1 Master Key는 한 단계 구조입니다

M1-HF에서도 다음 관계를 유지합니다.

$$
PK_A=SK_A G
$$

Committee는 $SK_A$의 2-of-3 share를 보관합니다. 승인된 감사에서 Status Authority가 서로 다른 두 share를 받아 $SK_A$를 복구하고 다음을 확인합니다.

$$
SK_A G\stackrel{?}=PK_A
$$

그 뒤 해당 key 기간의 AuditRecord를 로컬에서 복호화합니다. 별도 Committee key로 두 번째 Audit key를 감싸는 구조는 추가하지 않습니다.

### 2.2 AuditAndFreeze는 off-chain workflow입니다

AuditAndFreeze를 단일 Contract 함수로 만들지 않습니다.

```text
Status Authority
  → Snapshot RPC 조회
  → Master Key 복호화
  → graph traversal
  → target별 setStatus transaction
  → 공통 checkpoint 확인
```

Contract는 Master Key·평문·전체 graph를 받지 않습니다.

### 2.3 새로운 기능을 추가하지 않습니다

M1-HF는 다음을 구현하지 않습니다.

- 실제 DKG
- Audit key registry와 rotation
- post-snapshot에서 생성된 새 자손 자동 추적
- Batch Freeze
- Claim Status
- DPP 객체의 온체인 저장
- 운영용 Committee·Auditor server

## 3. Hash Domain은 어떻게 변경하나요?

### 3.1 변경 원칙

Hash에 들어가는 값과 순서는 유지하고 **Domain version만 `v2`로 변경**합니다.

| 목적 | M1 | M1-HF |
|---|---|---|
| Note | `zkDPP:Note:v1` | `zkDPP:Note:v2` |
| Owner address | `zkDPP:Owner:v1` | `zkDPP:Owner:v2` |
| Note nullifier | `zkDPP:Nullifier:v1` | `zkDPP:Nullifier:v2` |
| Voucher | `zkDPP:Voucher:v1` | `zkDPP:Voucher:v2` |
| Voucher nullifier | `zkDPP:VoucherNullifier:v1` | `zkDPP:VoucherNullifier:v2` |
| Voucher resolution secret | `zkDPP:VoucherResolutionSecret:v1` | `zkDPP:VoucherResolutionSecret:v2` |
| PolicyRef | `zkDPP:PolicyRef:v1` | `zkDPP:PolicyRef:v2` |
| PolicyScopeRef | `zkDPP:ScopeRef:v1` | `zkDPP:ScopeRef:v2` |
| Claim handle | `zkDPP:Issue:v1` | `zkDPP:Issue:v2` |
| Record masking key | `zkDPP:AuditKey:v1` | `zkDPP:AuditKey:v2` |
| 위치별 mask | `zkDPP:AuditMask:v1` | `zkDPP:AuditMask:v2` |

M1 활성 관계에서 제거한 Audit Context $L$은 다시 추가하지 않습니다. 따라서 사용하지 않는 `AuditContext` Domain을 Hotfix profile에 만들지 않습니다.

### 3.2 PolicyRef의 입력은 변경하지 않습니다

M1-HF의 PolicyRef는 다음 식입니다.

$$
policyRef=
H_{\mathrm{PolicyRef:v2}}(
eventKind,
authorityId,
policyId,
version
)
$$

inputArity·outputArity를 Hash 입력에 추가하지 않습니다. arity는 계속 PolicyRecord가 저장하고 Contract가 Event 실행 때 확인합니다.

PolicyScopeRef도 입력 관계는 유지합니다.

$$
policyScopeRef=
H_{\mathrm{ScopeRef:v2}}(sk_{\mathrm{owner}},policyRef)
$$

### 3.3 어떤 값이 함께 바뀌나요?

Tag 변경은 단순 문자열 표시 변경이 아닙니다. 다음 값이 모두 새로 계산됩니다.

- ZK owner address
- Note·Voucher commitment
- $nf$·$rvnf$
- PolicyRef·PolicyScopeRef
- Claim $h$
- Audit ciphertext mask
- Note·Voucher Tree root와 path

Tree node의 Poseidon2 compression과 empty-tree zero 값은 변경하지 않습니다. DocumentInfo의 NFC UTF-8·length-prefix encoding과 DocumentHash 계산도 변경하지 않습니다.

OwnerTag가 달라지므로 `actors-v1.json`을 덮어쓰지 않고 `actors-v2.json`을 새로 생성합니다. v1과 기존 M1의 commitment·proof·PolicyRef를 Hotfix 실행에 섞으면 명시적으로 실패해야 합니다.

Audit encryption profile은 `zkDPP-audit-dh-field-v2`로 올리고 새 session ID와 package checksum을 사용합니다. M1-HF는 DKG를 다시 구현하는 단계가 아니므로 동일한 고정 Committee scalar share·public key를 재사용할 수 있지만, v1 profile의 ciphertext·manifest와 섞어 해석하지 않습니다.

M1-HF는 fresh POC deployment입니다. 기존 M1 온체인 상태 migration과 객체 변환은 구현하지 않습니다.

## 4. Forward Tracing은 terminal을 어떻게 처리하나요?

### 4.1 반환값

Forward Trace는 frontier만 반환하지 않습니다.

```text
TraceResult

├─ graph
│  └─ parent, child, consumerAuditRecordId
├─ frontierNotes
├─ frontierVouchers
├─ terminalClaims
├─ terminatedExits
├─ visitedObjects
├─ uniqueAuditRecordIds
└─ metrics
```

같은 AuditRecord는 감사 실행 하나에서 한 번만 복호화하고 memory cache에서 재사용합니다.

### 4.2 Claim은 정상 terminal입니다

Claim에는 미래 소비 nullifier가 없습니다. 따라서 Claim에 `FutureSpendPosition`을 요청하지 않습니다.

```text
for each child in consumer.outputRefs:
    add graph edge parent → child

    if child.type == CLAIM:
        require consumer.eventKind == ISSUE
        require producerOf[CLAIM][child.h] == consumerAuditRecordId
        add child to terminalClaims
        continue

    add child to traversal queue
```

Claim과 미소비 Note가 같은 분기에 있어도 Claim은 정상 종료하고 Note 탐색은 계속해야 합니다.

### 4.3 Exit도 정상 terminal입니다

소비 AuditRecord의 outputRefs가 비어 있으면 다음을 확인합니다.

```text
require consumer.eventKind == EXIT
add consumed object to terminatedExits
do not add a new traversal target
```

Issue 이외의 Event가 Claim을 출력하거나 Exit 이외의 소비 Event가 output 없이 끝나면 Trace를 실패시킵니다.

### 4.4 Trace 실패는 frontier로 해석하지 않습니다

다음 오류가 하나라도 있으면 `TRACE_FAILED`이며 Freeze transaction은 0개입니다.

- Snapshot·chain·Ledger identity 불일치
- producerOf 누락 또는 잘못된 object type
- AuditRecord shape·암호문 오류
- parent와 consumer record 불일치
- spentIn이 가리키는 consumer가 실제 spend를 포함하지 않음
- 비인과적 record 연결
- 알 수 없는 terminal 형태

## 5. Claim에서 Entry까지 어떻게 거슬러 올라가나요?

Master Key 기반 Backward Trace를 M1-HF의 공식 경로로 추가합니다.

```text
TraceBackward(H_t, ClaimRef(h), SK_A):
    # 1. Claim producer를 확인합니다.
    issueAid = producerOf[CLAIM][h] at H_t
    issueRecord = LoadAndVerifyOriginal(issueAid, H_t)
    require issueRecord.eventKind == ISSUE
    require issueRecord.outputRefs == [ClaimRef(h)]

    # 2. Issue가 암호화한 부모 cm을 복원합니다.
    parentCM = Decrypt(issueRecord.encryptedParents[0], SK_A)

    # 3. 부모 Note의 producer를 따라갑니다.
    queue = [NoteRef(parentCM)]
    repeat producer lookup, record decryption and parent expansion

    # 4. Entry를 정상 시작점으로 기록합니다.
    if producerRecord.eventKind == ENTRY:
        add Entry terminal

    return graph, entries, visited records and metrics
```

Merge·Process처럼 부모가 여러 개인 Event에서는 모든 부모로 분기합니다. 이미 방문한 object와 AuditRecord는 다시 확장하지 않습니다.

## 6. AuditAndFreeze 결과는 무엇을 보존하나요?

### 6.1 target은 Note와 Voucher만 포함합니다

$$
T=\operatorname{DeduplicateTyped}(
frontierNotes\cup frontierVouchers
)
$$

Claim·Exit terminal은 TraceResult에는 남지만 Freeze target에는 들어가지 않습니다.

### 6.2 target별 결과

모든 target은 다음 결과를 하나씩 가집니다.

```text
TargetResult

├─ target
│  └─ objectType, objectRef, spendValue
├─ initialObservation
│  └─ block, spentIn, status, error
├─ action
│  ├─ NOT_NEEDED_ALREADY_BLOCKED
│  ├─ NOT_ATTEMPTED_ALREADY_SPENT
│  ├─ FREEZE_SUBMITTED
│  ├─ FREEZE_REJECTED
│  └─ TRANSACTION_UNCERTAIN
├─ transaction
│  └─ hash, receiptStatus, receiptBlock, gas, error
├─ finalObservation
│  └─ H_r, spentIn, status, error
└─ blockedAtCheckpoint
```

조회·제출·receipt 오류를 버리지 않습니다. 어떤 target의 오류가 다른 target 처리를 안전하게 막지 않는다면 나머지 target 처리를 계속하되, 전체 결과에는 모든 오류를 남깁니다.

### 6.3 이미 차단된 상태

초기 상태가 다음과 같으면 새 transaction을 보내지 않습니다.

| 상태 | 처리 |
|---|---|
| 미소비 + Active | Freeze 제출 |
| 미소비 + Frozen | 이미 차단됨으로 기록 |
| 미소비 + Revoked | 이미 차단됨으로 기록 |
| 이미 소비됨 | 동결 실패 target으로 기록 |
| 조회 실패 | 결과 불명확으로 기록 |

### 6.4 완료 판정

공통 결과 Snapshot $H_r$에서 target은 다음일 때 차단된 상태입니다.

$$
Blocked_{H_r}(target):=
spentIn_{H_r}(target)=0
\land
status_{H_r}(target)\in\{Frozen,Revoked\}
$$

`COMPLETE_AT_CHECKPOINT`는 다음 조건을 모두 만족해야 합니다.

- Trace가 정상 완료됐습니다.
- $T$가 비어 있지 않습니다.
- targetResults가 $T$의 모든 target과 정확히 일대일 대응합니다.
- 제출·receipt 결과가 불명확한 transaction이 없습니다.
- 모든 target이 같은 $H_r$에서 차단됐습니다.

Trace는 성공했지만 $T$가 비어 있으면 `NO_LIVE_TARGETS`입니다. 나머지 미완료 상태는 `INCOMPLETE`입니다.

## 7. Snapshot RPC는 무엇을 검증하나요?

### 7.1 Ledger Context

```text
LedgerContext

├─ expectedChainId
├─ ledgerAddress
├─ expectedRuntimeCodeHash
├─ deploymentBlock
└─ confirmationPolicy
```

모든 감사 시작 전에 chain ID·Ledger 주소·runtime code hash를 확인합니다.

### 7.2 감사 Snapshot

$$
H_t=(blockNumber_t,blockHash_t)
$$

가능하면 EIP-1898 block-hash selector 또는 동등한 block-hash 고정 RPC를 사용합니다. node가 block number 조회만 지원하면 전체 조회 전후에 같은 번호의 block hash를 다시 확인하고, 달라졌으면 Trace를 실패시킵니다.

다음 조회는 모두 $H_t$에 고정합니다.

- producerOf
- noteSpentIn·voucherSpentIn
- AuditRecord
- Claim 등록 상태
- PolicyRecord
- Event log·transaction·receipt 원본

AuditRecord storage만 신뢰하지 않습니다. Event log에서 record가 생성된 transaction을 찾고 다음을 대조합니다.

- transaction.to가 기대 Ledger인지
- function selector와 eventKind가 같은지
- receipt가 성공했는지
- public output·spend·PolicyRef·Audit ciphertext가 storage record와 같은지
- record 생성 block이 $H_t$에 포함되는지

### 7.3 결과 Snapshot

모든 Freeze 시도가 끝난 뒤 다음 조건을 만족하는 $H_r$을 선택합니다.

$$
H_r=(blockNumber_r,blockHash_r)
$$

- $blockNumber_r\ge blockNumber_t$
- 모든 성공 receipt의 block number 이상
- 실행 중 소비·상태 관찰에 사용한 block 이상
- confirmationPolicy를 만족

모든 target 상태를 $H_r$에 고정해 읽고, 조회가 끝난 뒤 같은 block number의 hash가 여전히 $blockHash_r$인지 다시 확인합니다. 확인할 수 없으면 `INCOMPLETE`입니다.

Anvil POC에서는 transaction이 포함된 블록을 즉시 확인 대상으로 사용할 수 있습니다. Production confirmation depth는 M1-HF에서 일반화하지 않습니다.

## 8. Process의 0 질량을 어떻게 막나요?

M1-HF는 개별 input을 모두 양수로 제한하지 않습니다. zero-State Note가 입력 중 일부에 있더라도 전체 Process가 유효한 제품을 만들면 허용할 수 있습니다.

대신 최종 ELIGIBLE output의 질량을 반드시 양수로 제한합니다.

$$
q_{\mathrm{eligible}}>0
$$

Native `Policy.Apply`와 Process Circuit이 같은 조건을 확인합니다.

```text
Process:
    calculate total, loss, intermediate and WASTE
    calculate ELIGIBLE residual
    require eligible.q_mass > 0
    continue existing conservation and audit checks
```

검증 사례:

- input 세 개와 output 두 개가 모두 zero인 Process는 실패합니다.
- 일부 input이 zero이지만 ELIGIBLE output이 양수인 경우는 성공합니다.
- 기존 `(320,30,231) → (270,30,261)+(30,0,0)` 사례는 그대로 성공합니다.

## 9. 외부 DPP Claim 검증은 어떻게 RPC와 연결하나요?

Issue Circuit은 Note에 이미 결합된 DocumentHash를 사용해 Claim $h$를 계산합니다. ProductName·LotID·Unit 문자열의 preimage를 Circuit에서 다시 Hash하지 않습니다.

이 경계를 원래 M1 명세에도 명확히 반영합니다.

- Circuit 책임: 소비 Note의 DocumentHash·State·Policy 조건과 $h$ 결합
- 외부 DPP verifier 책임: 공개받은 ProductName·LotID·Unit으로 DocumentHash와 $h$ 재계산

RPC Source에 다음 조회를 추가합니다.

```text
ClaimRegistered(h, H_t)
PolicyRecord(issuePolicyRef, H_t)
Producer(CLAIM, h, H_t)
Record(issueAid, H_t)
LoadAndVerifyOriginal(issueAid, H_t)
```

외부 DPP 검증은 하나의 Snapshot에서 다음을 모두 확인합니다.

1. 공개받은 ProductName·LotID·Unit으로 DocumentHash 재계산
2. `h = H_issue:v2(DocumentHash, issuePolicyRef, claimNonce)` 재계산
3. `claimRegistered[h] == true`
4. `producerOf[CLAIM][h]`가 Issue AuditRecord를 가리킴
5. Issue Record의 policyRef와 outputRefs가 Claim과 일치
6. PolicyRecord가 ISSUE·1-to-1이며 해당 발급 시점에 유효했음
7. transaction·receipt 원본과 storage record가 일치

제품 정보는 Contract storage에 저장하지 않습니다.

## 10. 실제 proof는 어디까지 연결하나요?

Mock verifier 테스트는 상태 전이·원자성 단위 테스트로 유지합니다. 하지만 공식 통합 Gate는 generated verifier와 실제 proof를 사용합니다.

다음 10개 relation을 모두 포함합니다.

| 분류 | Relation | Public input 수 |
|---|---|---:|
| 고정 Event | Entry | 4 |
| 고정 Event | Transfer | 10 |
| 고정 Event | Proceed | 7 |
| 고정 Event | Recall | 8 |
| 고정 Event | Merge | 9 |
| 고정 Event | Split | 9 |
| 고정 Event | Exit | 5 |
| Policy | Process 3-to-2 | 15 |
| Policy | Issue Standard | 7 |
| Policy | Issue Strict | 7 |

공식 Foundry·Anvil 시나리오는 모든 Event에서 다음 연결을 확인합니다.

```text
실제 private witness
  → final PK로 proof 생성
  → public input·calldata 조립
  → ZkDPPV2Ledger
  → PolicyRecord 또는 고정 verifier 선택
  → generated Solidity verifier 성공
  → AuditRecord·Tree·mapping 변경
```

wrong verifier·old v1 Tag proof·public input 순서 변경·ciphertext 변경은 실패해야 합니다.

## 11. Benchmark는 어떻게 다시 만드나요?

### 11.1 고정 수치 출력을 금지합니다

`benchmark-m1-hotfix-anvil`은 source에 저장한 gas 상수를 JSON으로 복사하지 않습니다.

각 결과는 같은 실행에서 얻은 다음 근거를 가져야 합니다.

- transaction hash
- block number·block hash
- receipt status·gasUsed
- calldata bytes
- deployed address·runtime code hash·runtime bytes
- 사용한 verifier·proof fixture checksum

Anvil이 없거나 transaction이 실패하면 benchmark도 실패하며 이전 수치를 새 결과로 출력하지 않습니다.

### 11.2 측정 대상

- verifier 10개와 Ledger deployment
- EntryIssuer·Policy Authority·PolicyRecord·Grant
- 9개 Event transaction
- Note·Voucher Freeze·Unfreeze·Revoke
- Master Key 복구
- AuditRecord 1·10·100·1,000개 복호화
- Claim·Exit terminal을 포함한 forward·backward tracing
- AuditAndFreeze target별 transaction과 $H_r$ 확인
- 전체 lifecycle E2E

기존 M1 Raw 숫자는 비교 기준으로 읽기만 하고 덮어쓰지 않습니다.

## 12. Manifest 생성과 검증을 어떻게 분리하나요?

### 12.1 생성 명령

다음 명령만 manifest와 checksum 기준을 씁니다.

```text
make finalize-m1-hotfix
```

이 명령은 Setup·evaluate·benchmark가 모두 성공한 뒤 한 번 실행합니다. 기존 파일이 있으면 기본적으로 덮어쓰지 않습니다.

### 12.2 읽기 전용 검사

```text
make check-m1-hotfix
```

`check-m1-hotfix`는 다음 파일을 수정하거나 다시 생성하지 않습니다.

- Raw JSON
- public-input manifest
- relation manifest
- generated checksum
- proof·VK·verifier

검사 순서:

1. 검사 전 대상 파일 checksum 캡처
2. 저장된 manifest와 현재 Artifact·source·Raw 비교
3. public input 이름·순서·개수 비교
4. verifier code hash·VK hash·proof fixture 연결 비교
5. 기존 M1 Raw·Result 불변 확인
6. 검사 후 대상 파일 checksum 재확인

하나라도 다르거나 검사 중 파일이 변경되면 실패합니다.

## 13. Artifact와 Raw 결과는 어디에 두나요?

기존 M1 경로를 덮어쓰지 않습니다.

```text
artifacts/development/m1-hotfix/
  baseline.json
  key-package/
  srs/
  circuits/<relation>/
  manifests/

contracts/src/generated/v2-m1-hotfix/<relation>/
contracts/test/fixtures/v2-m1-hotfix-proofs.json
```

Raw 결과:

```text
output/m1-hotfix-circuit.json
output/m1-hotfix-key-recovery.json
output/m1-hotfix-anvil.json
output/m1-hotfix-audit.json
output/m1-hotfix-generated-checksums.json
```

Result:

```text
milestones/M1-HF-spec-conformance-result.md
```

SRS·PK·VK·proof·generated verifier·secret share는 기존과 같이 Git에 포함하지 않습니다.

## 14. 구현 순서는 어떻게 되나요?

```text
M1-HF.0  기존 M1 보존 checksum·Domain migration preflight
M1-HF.1  v2 Domain·Process 양수 조건
M1-HF.2  terminal-aware forward·backward tracing
M1-HF.3  target result·Snapshot RPC 강화
M1-HF.4  DPP RPC·실제 proof Ledger 통합
M1-HF.5  실제 Anvil benchmark·읽기 전용 check
M1-HF.6  최종 회귀·Result
```

예정 명령:

```text
make setup-m1-hotfix
make evaluate-m1-hotfix
make test-go
make test-contract-m1-hotfix
make benchmark-m1-hotfix-key-recovery
make benchmark-m1-hotfix-decrypt
make benchmark-m1-hotfix-anvil
make benchmark-m1-hotfix-audit
make benchmark-m1-hotfix
make finalize-m1-hotfix
make check-m1-hotfix
```

## 15. 어떤 correctness Gate를 통과해야 하나요?

### Domain·Circuit

- 모든 활성 Domain 문자열이 `:v2`이고 `:v1`과 섞이지 않습니다.
- PolicyRef 입력 tuple은 기존 네 값으로 유지됩니다.
- actors-v2의 address가 OwnerTag v2 일반 계산과 일치합니다.
- 기존 v1·M1 proof와 Hotfix verifier를 섞으면 실패합니다.
- all-zero Process가 실패하고 positive ELIGIBLE Process가 성공합니다.
- Event별 public input 개수와 순서는 M1과 같습니다.

### Trace

- Claim-only downstream은 정상 Trace와 `NO_LIVE_TARGETS`로 끝납니다.
- Claim과 미소비 Note가 함께 있으면 Note만 frontier에 남습니다.
- Exit path는 terminatedExits에 남고 오류가 아닙니다.
- Claim에서 Issue·Process·Entry까지 backward tracing이 성공합니다.
- Merge의 중복 조상을 한 번만 확장합니다.
- 잘못된 terminal·parent·consumer·record는 `TRACE_FAILED`입니다.

### Status·결과

- Active target은 Freeze transaction을 제출합니다.
- Frozen·Revoked target은 재제출 없이 차단 완료로 인정합니다.
- 소비된 target은 `INCOMPLETE` 원인으로 남습니다.
- 조회·제출·receipt 오류가 targetResults에서 사라지지 않습니다.
- transaction 결과가 불명확하면 완료로 표시하지 않습니다.
- Trace 실패 시 Freeze transaction은 0개입니다.

### Snapshot·RPC

- wrong chain ID·Ledger address·runtime code hash를 거부합니다.
- $H_t$와 $H_r$의 block hash 변경을 거부합니다.
- $H_r<H_t$를 거부합니다.
- $H_r$이 성공 receipt block보다 과거이면 거부합니다.
- 모든 final status가 같은 $H_r$에서 조회됩니다.
- transaction·receipt·AuditRecord 원본 불일치를 거부합니다.

### Contract·통합

- 실제 proof를 사용한 9개 Event lifecycle이 성공합니다.
- Process·Issue가 PolicyRecord verifierRef를 실제로 사용합니다.
- Exit는 successor 없이 종료하고 Issue는 Claim 하나를 생성합니다.
- 실패 뒤 Tree·spentIn·producerOf·Claim·AuditRecord가 불변입니다.
- runtime bytecode가 EIP-170 제한 이내입니다.

### 검증 명령

- benchmark 명령이 Anvil 없이 성공하지 않습니다.
- gas 값마다 실제 receipt가 연결됩니다.
- read-only check 전후 파일 checksum이 같습니다.
- 기존 M1 Raw·Result와 v1 M9 파일이 변경되지 않습니다.

## 16. 완료 조건은 무엇인가요?

- 위 correctness Gate가 모두 통과합니다.
- Go 전체 suite와 Hotfix Foundry suite가 통과합니다.
- 10개 proof가 일반 계산·gnark·Solidity verifier에서 일치합니다.
- 실제 Anvil lifecycle과 AuditAndFreeze가 성공합니다.
- 새 Raw·manifest·Artifact가 서로 일치합니다.
- `check-m1-hotfix`가 읽기 전용으로 성공합니다.
- M1-HF Result가 실제 수치와 한계를 설명합니다.
- 기존 M1과 v1 M9 checksum이 유지됩니다.

전체 조건이 충족된 뒤에만 M1-HF를 `완료`로 변경합니다. 한 단계 Master Key가 Production key release 안전성을 제공하거나, Hotfix가 DKG·Rotation까지 구현했다고 주장하지 않습니다.
