# M9 — 최종 Protocol을 하나의 SRS와 Anvil에서 어떻게 재현하나요?

- 명세 상태: 구현 기준 동결
- 구현 결과: [M9 Result](M9-final-integration-result.md)
- 이전 결과: [M8 Result](M8-exit-dpp-issue-result.md)
- 이전 구현 기준: [M8 명세](M8-exit-dpp-issue.md)
- 검토 기준: [Milestone Checklist](MILESTONE-CHECKLIST.md)
- 작성 범위: 최종 통합 구현 명세입니다. 실제 수치와 구현 차이는 M9 Result에서 확인합니다.
- 원칙: **YAGNI가 최우선입니다.** Besu·별도 Router·새 Protocol 기능을 추가하지 않고, M8에서 확정한 기능의 SRS·Contract·시나리오·평가를 통합합니다.

## 0. 이번에는 무엇을 완성하나요?

**M8 Main Protocol의 최종 Circuit 10개를 하나의 $2^{17}$ universal canonical SRS로 Setup하고, ZkDPPClaimLedger 하나에서 다단계 공급망 시나리오를 Anvil로 재현합니다.**

| 질문 | M9의 기준 |
|---|---|
| 새로운 Event를 만드나요? | 아닙니다. M8까지 구현한 Event·Policy만 최종 통합합니다. |
| Main Contract는 무엇인가요? | ZkDPPClaimLedger 하나입니다. 별도 Router는 없습니다. |
| 최종 Circuit은 몇 개인가요? | 고정 Event 7개와 Policy 3개를 합한 10개입니다. |
| Process·Issue는 어떻게 연결하나요? | 별도 verifier Contract를 배포하고 PolicyRecord의 verifierRef로 선택합니다. |
| SRS 크기는 얼마인가요? | 최대 Lagrange domain $2^{17}$, canonical point 131,075개입니다. |
| 왜 $2^{17}$인가요? | 현재 가장 큰 Process Circuit을 지원하는 최소 domain이기 때문입니다. |
| Backend는 무엇인가요? | Docker Anvil·Prague·chain ID 31337만 사용합니다. |
| 몇 번 측정하나요? | 기본 2회이며 시간·메모리 차이가 20%를 넘을 때만 3회차를 추가합니다. |
| 대표 흐름은 무엇인가요? | 원자재 4개가 Transfer·Recall·Proceed·Merge·Process·Split을 거쳐 DPP·Claim이 됩니다. |
| 무엇은 하지 않나요? | Besu·QBFT·외부 baseline·$2^{20}$ SRS·새 Policy·Production Ceremony입니다. |

전체 작업은 다음 순서입니다.

```text
최종 Circuit 10개 compile
  → Circuit별 SRSSize 계산
  → 2^17 universal canonical SRS 생성
  → domain별 Lagrange SRS 생성
  → Circuit별 PK·VK·verifier 생성
  → ZkDPPClaimLedger에 연결
  → Anvil 전체 시나리오 실행
  → SRS·Circuit·gas·E2E·감사 결과 대조
```

Circuit을 SRS 안에 저장하는 구조가 아닙니다. SRS는 polynomial commitment가 지원할 최대 크기를 제공하고, 각 Circuit은 같은 SRS에서 자기 CCS·PK·VK를 별도로 만듭니다.

## 1. M8에서 무엇이 달라지나요?

| M8 | M9에서 정리할 내용 |
|---|---|
| Circuit별 개발 SRS를 사용합니다. | 하나의 universal canonical SRS를 모든 최종 Circuit이 공유합니다. |
| milestone별 독립 fixture·benchmark가 있습니다. | 하나의 다단계 공급망 fixture와 최종 평가 명령을 제공합니다. |
| Process verifier가 constructor와 PolicyRecord에 모두 연결됩니다. | Process도 Issue처럼 PolicyRecord의 verifierRef로만 선택합니다. |
| Claim에서 upstream 감사까지 확인했습니다. | 원자재에서 DPP까지의 forward와 Claim에서 Entry까지의 backward를 함께 확인합니다. |
| 단일 공식 측정값을 주로 기록했습니다. | M9 대상은 기본 2회 측정하고 차이 기준을 적용합니다. |

M1~M8의 기존 Contract·Artifact·Raw 결과는 당시 구현의 기록으로 보존합니다. M9는 새 final Artifact와 최종 배포용 ZkDPPClaimLedger를 생성하지만 과거 파일을 새 결과로 덮어쓰지 않습니다.

## 2. 최종 Contract는 어떻게 구성하나요?

### 2.1 몇 개의 Contract가 있나요?

상태를 보관하고 Event를 실행하는 Main Contract는 하나입니다.

```text
ZkDPPClaimLedger 1개
```

Hash와 proof 검증은 별도 Contract를 사용합니다.

| 분류 | Contract | 연결 방식 |
|---|---|---|
| Hash | Poseidon2BLS12381 | Main Contract constructor에서 고정 |
| 고정 Event | Entry·Transfer·Proceed·Recall·Merge·Split·Exit verifier 7개 | Main Contract constructor에서 고정 |
| Policy | Process·Issue Standard·Issue Strict verifier 3개 | PolicyRecord의 verifierRef에 등록 |

모든 Circuit은 자기 VK가 들어 있는 verifier Contract를 가집니다. Process·Issue만 별도 상태 원장을 갖는다는 의미가 아닙니다. Note·Voucher·DPP·Claim·AuditRecord와 상태는 ZkDPPClaimLedger가 관리합니다.

### 2.2 별도 Router가 필요한가요?

필요하지 않습니다. ZkDPPClaimLedger가 이미 다음 역할을 수행합니다.

- 고정 Event 함수와 verifier 선택
- PolicyRecord의 verifierRef 선택
- Note·Voucher Tree와 소비 상태 변경
- DPP·Claim 등록과 Claim 상태 변경
- AuditRecord·producerOf·spentIn 갱신

별도 Router를 추가하면 호출 계층과 배포 대상만 늘어나므로 M9에서는 만들지 않습니다.

### 2.3 Process verifier 중복은 어떻게 제거하나요?

M8 constructor의 verifier 배열은 Entry·Transfer·Proceed·Recall·Merge·Split·Process·Exit 순서의 8개입니다. M9에서는 Process를 고정 배열에서 제거합니다.

최종 constructor의 verifier 순서는 다음 7개입니다.

```text
1. Entry
2. Transfer
3. Proceed
4. Recall
5. Merge
6. Split
7. Exit
```

Process Contract 검증은 다음 순서로 변경합니다.

```text
function process(proof, policyRef, policyScopeRef, noteRoot, nf[3], cmOut[2], audit):
    # 핵심: Process verifier를 constructor가 아니라 등록된 PolicyRecord에서 선택합니다.
    # 1. Policy 의미와 사용 권한을 확인합니다.
    policy = policyRecords[policyRef]
    require policy가 존재하고 enabled
    require policy.eventKind == PROCESS
    require policy.inputArity == 3
    require policy.outputArity == 2
    require policyGrants[policyRef][policyScopeRef] == true

    # 2. 기존 Note 소비·출력·Audit 조건을 확인합니다.
    require noteRoot가 accepted root
    require nf 세 개가 서로 다르고 Active·미소비
    require cmOut 두 개가 새 commitment

    # 3. PolicyRecord가 가리키는 verifier로 proof를 검증합니다.
    require policy.verifierRef.Verify(Process public inputs와 audit suffix)

    # 4. 기존 M8 의미대로 상태를 원자적으로 변경합니다.
    record nf 세 개
    append output Note 두 개
    record AuditRecord와 producerOf
```

Issue는 M8 구조를 유지합니다. Issue Policy는 1-to-1이며 PolicyGrant·policyScopeRef를 사용하지 않습니다. Process와 Issue 모두 임의 verifier 주소를 Event 함수 인자로 받지 않습니다.

### 2.4 최종 Policy 목록은 무엇인가요?

| Policy | eventKind | Arity | 핵심 상수 |
|---|---:|---:|---|
| Process 3-to-2 | 6 | 3-to-2 | M5 loss·carbon·allocation rule |
| Issue Standard V1 | 8 | 1-to-1 | 재활용률 10% 이상, 탄소집약도 1.00 이하 |
| Issue Strict V2 | 8 | 1-to-1 | 재활용률 11% 이상, 탄소집약도 0.97 이하 |

추가 Policy family·version은 만들지 않습니다. PolicyRef·vkHash·immutable Record·one-way disable과 Process Grant는 기존 M5·M8 의미를 유지합니다.

## 3. 최종 Circuit은 어떤 것인가요?

### 3.1 어떤 Circuit만 포함하나요?

M1~M6의 교체된 baseline Circuit은 final Setup 대상이 아닙니다. M8 Main Protocol이 실제로 호출하는 다음 10개만 compile합니다.

| 분류 | Relation | 기반 | Public input 수 |
|---|---|---|---:|
| 고정 Event | Audit Entry | M7 | 4 |
| 고정 Event | Audit Transfer | M7 | 11 |
| 고정 Event | Audit Proceed | M7 | 7 |
| 고정 Event | Audit Recall | M7 | 8 |
| 고정 Event | Audit Merge | M7 | 9 |
| 고정 Event | Audit Split | M7 | 9 |
| 고정 Event | Exit DPP | M8 | 6 |
| Policy | Audit Process 3-to-2 | M6-B1·M7 | 15 |
| Policy | Issue Standard V1 | M8 | 2 |
| Policy | Issue Strict V2 | M8 | 2 |

제외되는 예시는 초기 Private Spend·초기 Entry·암호화 없는 Event·M6 Status-aware Event·M7 terminal Exit·독립 AuditEncryption 진단 Circuit입니다. 제외는 과거 구현을 삭제한다는 의미가 아니라 final SRS·PK·VK 생성 목록에서 빼는 것입니다.

### 3.2 공개 입력 순서는 어떻게 고정하나요?

final public-input manifest에는 다음 순서를 그대로 기록합니다.

| Relation | 공개 입력 순서 |
|---|---|
| Entry | cm, R1X, R1Y, encryptedOutputNf |
| Transfer | noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch, R1X, R1Y, encryptedParentCM, encryptedVoucherRvnf, encryptedChangeNf |
| Proceed | voucherRoot, rvnf, cmReceiver, R1X, R1Y, encryptedParentRV, encryptedReceiverNf |
| Recall | voucherRoot, rvnf, cmReturn, currentEpoch, R1X, R1Y, encryptedParentRV, encryptedReturnNf |
| Merge | noteRoot, nf1, nf2, cmOut, R1X, R1Y, encryptedParentCM1, encryptedParentCM2, encryptedOutputNf |
| Split | noteRoot, nf, cmOut1, cmOut2, R1X, R1Y, encryptedParentCM, encryptedOutputNf1, encryptedOutputNf2 |
| Process | policyRef, policyScopeRef, noteRoot, nf1, nf2, nf3, cmEligible, cmWaste, R1X, R1Y, encryptedParentCM1, encryptedParentCM2, encryptedParentCM3, encryptedEligibleNf, encryptedWasteNf |
| Exit | noteRoot, nf, dppCommitment, R1X, R1Y, encryptedParentCM |
| Issue Standard | issuePolicyRef, dppCommitment |
| Issue Strict | issuePolicyRef, dppCommitment |

Manifest 순서와 gnark public witness·Solidity verifier calldata가 하나라도 다르면 Setup 완료로 처리하지 않습니다.

## 4. SRS 크기는 어떻게 결정하나요?

### 4.1 무엇을 계산하나요?

각 Circuit을 BLS12-381 SCS로 compile한 뒤 gnark의 `plonk.SRSSize(ccs)`를 호출합니다.

$$
N_{\mathrm{system}}
=
N_{\mathrm{constraints}}+N_{\mathrm{public}}
$$

$$
N_{\mathrm{Lagrange}}
=
\operatorname{NextPowerOfTwo}(N_{\mathrm{system}})
$$

$$
N_{\mathrm{canonical}}
=
N_{\mathrm{Lagrange}}+3
$$

세 개의 추가 canonical point는 blinded polynomial opening에 필요합니다.

현재 완료 Result를 기준으로 예상 domain은 다음입니다. M9 구현은 이 숫자를 복사해 신뢰하지 않고 최종 compile 결과를 다시 계산합니다.

| Relation | 현재 constraints+public | 예상 domain |
|---|---:|---:|
| Issue Standard·Strict | 12,492 | $2^{14}$ |
| Entry | 19,024 | $2^{15}$ |
| Exit | 36,936 | $2^{16}$ |
| Transfer·Proceed·Recall·Merge·Split | 최대 57,560 | $2^{16}$ |
| Process | 93,593 | $2^{17}$ |

가장 큰 Process를 기준으로 다음 크기를 사용합니다.

```text
maximum Lagrange domain = 131,072
universal canonical SRS = 131,075 G1 points
```

M9에서 compile한 어떤 Circuit이라도 $2^{17}$을 초과하면 임의로 SRS를 키우지 않고 명세 불일치로 실패합니다. 현재 기능에 필요한 최소 크기를 검증하는 것이 이번 POC의 범위입니다.

### 4.2 하나의 canonical SRS를 어떻게 공유하나요?

첫 Setup run에서 임의의 development용 $τ$로 BLS12-381 canonical SRS 131,075 points를 한 번 생성합니다. $τ$는 파일·로그·manifest에 저장하지 않습니다.

필요한 Lagrange domain은 다음 네 개입니다.

```text
2^14 = 16,384
2^15 = 32,768
2^16 = 65,536
2^17 = 131,072
```

각 domain $N$에 대해 universal canonical SRS의 첫 $N$개 G1 point를 복사하고 gnark-crypto BLS12-381 KZG의 `ToLagrangeG1`을 적용합니다. 생성한 Lagrange SRS는 universal SRS와 같은 VK를 사용하며 G1 길이가 정확히 $N$이어야 합니다.

Circuit별 `plonk.Setup`에는 다음을 전달합니다.

- canonical: 131,075-point universal SRS 전체 또는 필요한 $N+3$ prefix
- Lagrange: 해당 Circuit domain과 정확히 같은 $N$-point SRS

gnark Setup이 canonical 길이 $N+3$ 이상과 Lagrange 길이 $N$ 일치를 다시 확인합니다. 모든 Circuit이 Setup·Prove·Verify에 성공해야 같은 universal SRS를 사용했다고 판단합니다.

### 4.3 Ethereum Ceremony 결과를 사용하나요?

사용하지 않습니다. Ethereum EIP-4844 KZG Ceremony는 blob commitment를 위한 것이며 현재 Process가 요구하는 $2^{17}$보다 작습니다.

M9 SRS manifest에는 다음을 명시합니다.

```text
developmentOnly = true
source = local-unsafe-benchmark
curve = BLS12-381
maxDomain = 131072
canonicalPoints = 131075
productionCeremony = false
```

M9 결과를 Production trusted setup으로 주장하지 않습니다. Production에서는 같은 curve·충분한 G1 powers·검증 가능한 transcript를 가진 별도 MPC Ceremony가 필요합니다.

### 4.4 향후 더 큰 Circuit이 생기면 어떻게 하나요?

새 Circuit의 `constraints + public inputs`가 131,072 이하라면 같은 canonical SRS에서 새 domain Lagrange SRS와 새 PK·VK를 만들 수 있습니다.

이를 초과하면 다음 중 하나가 필요합니다.

- 더 큰 universal SRS와 새 version의 전체 key 생성
- 기존 SRS version을 유지하고 큰 Circuit만 별도 SRS family로 분리

M9에서는 어느 확장 방식을 미리 구현하지 않습니다. $2^{20}$은 미래 후보일 뿐 현재 Artifact 크기가 아닙니다.

## 5. 최종 Artifact는 어떻게 저장하나요?

### 5.1 폴더 구조는 무엇인가요?

```text
artifacts/final/
  srs/
    universal-canonical.bin
    manifest.json
    lagrange/
      domain-2^14.bin
      domain-2^15.bin
      domain-2^16.bin
      domain-2^17.bin

  circuits/
    audit-entry/
    audit-transfer/
    audit-proceed/
    audit-recall/
    audit-merge/
    audit-split/
    exit-dpp/
    audit-process-3-2/
    issue-standard-v1/
    issue-strict-v2/

  verifiers/
    public-input-manifest.json
    checksums.json
```

각 Circuit 폴더에는 `ccs.bin`, `proving.key`, `verifying.key`, `manifest.json`을 둡니다. canonical·Lagrange SRS를 Circuit 폴더에 중복 복사하지 않습니다.

### 5.2 SRS manifest에는 무엇을 기록하나요?

| 필드 | 의미 |
|---|---|
| version | SRS Artifact format version |
| curve·proofSystem | BLS12-381·PLONK-KZG |
| developmentOnly·source | Production Ceremony가 아님을 표시 |
| maxDomain·canonicalPoints | $2^{17}$·131,075 |
| canonicalChecksum·bytes | universal file identity |
| lagrangeDomains | 네 domain·checksum·bytes |
| generationRuns | 두 실행 시간·메모리·선택 여부 |
| selectedRun | 항상 첫 번째 run |

두 번째 run이 더 빨라도 selectedRun을 바꾸지 않습니다.

### 5.3 Circuit manifest에는 무엇을 기록하나요?

| 필드 | 의미 |
|---|---|
| relation·eventKind·policyRef | Circuit 역할 |
| constraints·publicInputs | compile 결과와 순서 |
| systemSize·domainSize | SRS 필요 크기 |
| universalSRSChecksum | 모든 Circuit에서 같은 값이어야 함 |
| lagrangeSRSChecksum | domain별 파일 identity |
| ccs·PK·VK checksum·bytes | Circuit별 Artifact |
| verifierSourceChecksum | generated Solidity source |
| verifierRuntimeCodeHash | 실제 배포 code identity |

public-input-manifest는 10개 relation의 ordered field name·개수·domain·PolicyRef·verifier source를 한 파일에서 찾을 수 있게 합니다.

## 6. 대표 시나리오는 어떻게 진행하나요?

### 6.1 누가 참여하나요?

| 역할 | ZK identity | Anvil account |
|---|---|---:|
| System Admin·Policy Authority | 해당 없음 | 0 |
| Aluminum Supplier | actor-1 | 1 |
| Factory | actor-2 | 2 |
| Component Supplier | actor-3 | 3 |
| Status Authority | 해당 없음 | 4 |

EVM account와 ZK owner address는 암호학적으로 binding하지 않습니다. 같은 참여자 역할에 배치한 실험용 identity일 뿐입니다.

Admin은 account 1·3에 Entry 권한을 부여하고 account 0을 Policy Authority로 등록합니다. Status Authority는 constructor에서 account 4로 고정합니다.

### 6.2 어떤 원자재로 시작하나요?

모든 값은 기존처럼 $10^9$ scale의 uint64로 Circuit에 입력합니다.

| 원자재 | 초기 owner | $q_{\mathrm{mass}}$ | $a_{\mathrm{rec}}$ | $e$ |
|---|---|---:|---:|---:|
| Aluminum A | actor-1 | 60 kg | 10 kg | 40 kgCO2e |
| Aluminum B | actor-1 | 60 kg | 10 kg | 50 kgCO2e |
| Cathode | actor-3 | 100 kg | 10 kg | 70 kgCO2e |
| Anode | actor-3 | 100 kg | 0 kg | 70 kgCO2e |

네 입력의 합은 다음입니다.

$$
(q_{\mathrm{mass}},a_{\mathrm{rec}},e)=(320,30,230)
$$

전부 ELIGIBLE입니다. DocumentInfo의 ProductName·LotID와 Note·Voucher opening은 고정 테스트 데이터에서 서로 다른 값으로 지정합니다.

### 6.3 Transfer·Recall·Proceed는 어떻게 연결하나요?

1. 네 원자재를 각각 Entry합니다.
2. Aluminum A 전량을 actor-2에게 Transfer합니다. 질량 0 Change Note도 생성합니다.
3. 첫 Voucher의 rvnf를 Freeze하고 Proceed·Recall이 모두 실패하는지 확인합니다.
4. Voucher를 Active로 되돌리고 epoch 105에서 Recall합니다.
5. 반환 Note를 epoch 106에서 다시 전량 Transfer하고 actor-2가 Proceed합니다.
6. Aluminum B·Cathode·Anode도 전량 Transfer하고 actor-2가 Proceed합니다.

첫 Aluminum Transfer는 transferEpoch=100, deltaEpoch=10, deadlineEpoch=110입니다. Recall은 $105<110$에서 성공합니다. 재Transfer와 나머지 Transfer는 각 실행 block의 currentEpoch을 public 입력으로 사용합니다.

모든 전량 Transfer는 무작위 opening의 zero-State Change Note를 Note Tree에 append합니다. 이를 생략하거나 최종 제품 질량에 다시 더하지 않습니다.

### 6.4 Merge·Process·Split 결과는 무엇인가요?

Factory가 받은 Aluminum 두 개를 Merge합니다.

$$
(60,10,40)+(60,10,50)=(120,20,90)
$$

Process 입력은 Merge output·Cathode·Anode입니다.

$$
(120,20,90)+(100,10,70)+(100,0,70)=(320,30,230)
$$

Process 직전에 첫 input nf를 Frozen으로 바꿔 proof가 같아도 transaction이 실패하는지 확인합니다. Active로 되돌린 뒤 exact 3-to-2 Process를 실행합니다.

M5 Policy 계산 결과는 다음입니다.

```text
Input aggregate:       (320, 30, 230)
6.25% mass loss:        20
Process carbon add:     30
Intermediate:          (300, 30, 260)
ELIGIBLE output:       (270, 30, 260)
WASTE output:           (30,  0,   0)
```

ELIGIBLE output은 200 kg과 70 kg으로 Split합니다. output 2가 floor를 받고 output 1이 conservation residual을 받습니다.

| Split output | $q_{\mathrm{mass}}$ | $a_{\mathrm{rec}}$ | $e$ |
|---|---:|---:|---:|
| Product 1 | 200,000,000,000 | 22,222,222,223 | 192,592,592,593 |
| Product 2 | 70,000,000,000 | 7,777,777,777 | 67,407,407,407 |

두 output의 세 State 합은 Process ELIGIBLE output과 정확히 같아야 합니다.

### 6.5 Exit·DPP·Issue는 어떻게 마무리하나요?

1. Product 1을 Exit하고 dppOpening=9901로 DPP commitment를 만듭니다.
2. 같은 DPP에 Standard V1 Claim을 Issue합니다.
3. 같은 Standard Claim 재Issue가 실패하는지 확인합니다.
4. Strict V2 Claim도 Issue합니다.
5. WASTE output을 dppOpening=9902로 Exit합니다.
6. WASTE DPP는 두 Issue Circuit 모두 만족하지 못하는지 확인합니다.
7. Standard Claim은 Active→Frozen→Active로 변경합니다.
8. Strict Claim은 Active→Frozen→Revoked로 변경합니다.
9. verifyClaim이 Standard=Active, Strict=Revoked를 반환하는지 확인합니다.

Product 2는 미소비 Active Note로 남깁니다. WASTE DPP는 정상 종료된 객체이지만 Sustainability Claim은 없습니다.

### 6.6 감사에서는 무엇을 찾나요?

Backward 감사는 Standard Claim에서 시작합니다.

```text
Standard Claim
  → Issue AuditRecord
  → Product 1 DPP
  → Exit AuditRecord
  → Split
  → Process
  → Merge·Proceed·Transfer·Recall
  → Entry 4건
```

Forward 감사는 Aluminum A의 Entry cm에서 시작합니다. zero Change·Recall·재Transfer 분기를 포함해 소비 기록을 따라가고 Product 1 DPP와 Product 2 Note를 구분합니다.

M9 감사 decoder는 M8 Exit의 DPPRef를 정상 terminal output으로 처리합니다. DPP는 소비 nullifier가 없으므로 더 이상 spentIn을 조회하지 않습니다. 알려진 Policy 목록으로 Claim 등록·상태를 조회해 terminal DPP 결과에 붙입니다. 자동 동결이나 DPP 내부 재귀 탐색은 추가하지 않습니다.

각 감사는 별도의 고정 block number·hash snapshot과 빈 in-memory cache로 시작합니다.

## 7. 두 번의 Setup과 측정은 어떻게 진행하나요?

### 7.1 SRS·Setup 두 번은 무엇이 다른가요?

Run 1은 final Artifact를 생성합니다.

```text
random development tau 1
  → universal canonical SRS 1
  → domain Lagrange SRS 4개
  → Circuit별 PK·VK
  → Prove·Verify
  → artifacts/final에 보존
```

Run 2는 독립적인 임시 측정입니다.

```text
random development tau 2
  → universal canonical SRS 2
  → domain Lagrange SRS 4개
  → Circuit별 임시 PK·VK
  → 같은 witness로 Prove·Verify
  → 시간·메모리·성공 결과만 보존
```

Run 2 Artifact가 더 빠르거나 작아도 final로 교체하지 않습니다. 두 SRS byte·PK·VK가 다른 것은 정상이며 constraints·domain·public input·검증 의미는 같아야 합니다.

### 7.2 언제 세 번째로 측정하나요?

wall-clock과 측정 메모리는 다음 차이를 계산합니다.

$$
\mathrm{differenceRate}
=
\frac{|run_1-run_2|}{\min(run_1,run_2)}
$$

$\mathrm{differenceRate}>0.20$인 항목만 동일 조건의 세 번째 측정을 실행합니다. 두 번이면 두 값을 모두 표시하고 평균을 보조값으로 사용합니다. 세 번이면 세 값을 모두 표시하고 median을 대표값으로 사용합니다.

Gas·calldata·constraints·public input·Artifact schema는 같은 입력에서 결정적이어야 합니다. 두 Anvil run의 gas가 다르면 세 번째로 평균내지 않고 transaction 순서·state·calldata·bytecode hash를 먼저 대조합니다. 원인을 수정한 뒤 전체 scenario를 새 clean chain에서 다시 실행하고 실패 이력을 보존합니다.

### 7.3 무엇을 측정하나요?

| 관점 | 측정 대상 |
|---|---|
| SRS | canonical 생성, domain별 Lagrange 변환, 파일 크기·checksum |
| Certifier·Setup | Circuit별 compile·PLONK Setup, PK·VK·verifier 크기 |
| Participant | Event별 witness·Prove·Native Verify, 할당량·heap snapshot |
| Contract | hasher·verifier·Main 배포, Policy 등록, Event·상태 gas·calldata·SSTORE |
| E2E | proof 준비, ABI·transaction 준비, submit-to-receipt, 전체 시간 |
| Auditor·위원회 | 원본 조회, partial decryption, 결합, backward·forward traversal |

할당량을 peak RSS로 표현하지 않습니다. 전체 receipt gas를 순수 SSTORE 비용으로 표현하지 않습니다. 작은 로컬 Anvil 그래프를 Production 공급망 throughput으로 일반화하지 않습니다.

## 8. 명령과 Raw 결과는 무엇인가요?

### 8.1 구현할 명령은 무엇인가요?

```text
make setup-m9-final
make evaluate-m9
make test-go
make test-contract-m9
make benchmark-m9-setup
make benchmark-m9-anvil
make benchmark-m9-audit
make benchmark-m9
make check-m9
```

`setup-m9-final`은 Run 1 final Artifact를 생성합니다. `benchmark-m9-setup`은 Run 2와 필요한 20% 초과 재측정을 관리합니다. `benchmark-m9-anvil`과 `benchmark-m9-audit`은 각 case의 두 clean-chain run을 내부에서 수행합니다.

Aggregate 명령과 개별 benchmark를 중복 실행하지 않습니다. 공식 Raw가 존재하면 덮어쓰지 않고 명시적으로 실패합니다.

### 8.2 어떤 Raw를 생성하나요?

```text
output/m9-srs.json
output/m9-circuit.json
output/m9-anvil.json
output/m9-audit.json
output/m9-generated-checksums.json
```

- m9-srs는 두 SRS 생성·Lagrange 변환·Setup 시간과 재측정 판단을 기록합니다.
- m9-circuit은 relation별 domain·Prove·Verify·Artifact 크기를 기록합니다.
- m9-anvil은 두 전체 scenario의 transaction·gas·E2E와 최종 상태를 기록합니다.
- m9-audit은 두 backward·forward 감사의 방문 객체·기록·복호화·RPC·시간을 기록합니다.
- m9-generated-checksums는 final Artifact·verifier·fixture·Raw와 M1~M8 보호 파일을 대조합니다.

## 9. 무엇을 확인하면 M9가 완료되나요?

### 9.1 SRS·Artifact Gate

- 최종 Circuit이 정확히 10개여야 합니다.
- 모든 Circuit의 `plonk.SRSSize`가 manifest와 일치해야 합니다.
- 최대 domain은 $2^{17}$, canonical point 수는 131,075여야 합니다.
- 10개 Circuit의 universalSRSChecksum이 같아야 합니다.
- Lagrange SRS는 relation domain과 정확히 같은 길이여야 합니다.
- Run 1·Run 2 모두 10개 Circuit Setup·Prove·Verify에 성공해야 합니다.
- final PK·VK·verifier는 Run 1 SRS에서만 생성돼야 합니다.
- serialized Artifact를 다시 읽어 proof를 검증해야 합니다.
- public-input manifest·gnark witness·Solidity calldata 순서가 같아야 합니다.

### 9.2 Contract·Policy Gate

- constructor 고정 verifier가 7개여야 합니다.
- Process 전용 immutable verifier와 별도 Router가 없어야 합니다.
- Process·Issue가 각 PolicyRecord verifierRef를 사용해야 합니다.
- Process는 Grant를 요구하고 Issue는 Grant를 사용하지 않아야 합니다.
- wrong EventKind·arity·PolicyRef·verifier·Grant가 실패해야 합니다.
- 기존 M8 Event·AuditRecord·nf 상태·Claim 상태와 원자성이 회귀하지 않아야 합니다.
- Main Contract runtime이 EIP-170 24,576 B 이하여야 합니다.

### 9.3 대표 시나리오 Gate

- 네 Entry State의 합과 Merge·Process·Split 결과가 정확해야 합니다.
- Freeze한 Voucher는 Proceed·Recall이, Freeze한 Note는 Process가 실패해야 합니다.
- Recall 후 재Transfer·Proceed가 성공해야 합니다.
- Process output은 ELIGIBLE $(270,30,260)$과 WASTE $(30,0,0)$이어야 합니다.
- Split output 두 개의 합이 ELIGIBLE output과 정확히 같아야 합니다.
- Product 1 DPP에는 Standard·Strict Claim이 등록돼야 합니다.
- WASTE DPP Issue는 실패해야 합니다.
- Standard=Active, Strict=Revoked와 Product 2=미소비 Active가 최종 상태여야 합니다.
- 실패 transaction 뒤 Tree·spentIn·producerOf·Claim·AuditRecord가 불변이어야 합니다.

### 9.4 감사·측정 Gate

- Claim에서 네 Entry까지 backward tracing이 성공해야 합니다.
- Aluminum A에서 Product 1 DPP·Product 2 Note와 zero Change 분기를 forward tracing해야 합니다.
- DPP를 미소비 Note leaf로 잘못 분류하지 않아야 합니다.
- 각 복호화 원문이 Event fixture와 일치해야 합니다.
- 공식 측정이 기본 2회이고 20% 초과 항목만 3회여야 합니다.
- Anvil 두 run의 gas가 다르면 원인 설명 없이 대표값을 만들지 않아야 합니다.
- M1~M8 Raw·Artifact·Result와 Conversation History checksum이 유지돼야 합니다.

### 9.5 완료 문서는 무엇인가요?

`milestones/M9-final-integration-result.md`를 Result Template에 맞춰 생성합니다. 첫 화면에는 다음을 표시합니다.

- final Contract·Circuit·Policy 구성
- $2^{17}$ universal SRS와 실제 domain 분포
- 전체 시나리오에서 가능해진 흐름
- SRS·Setup·Prove·gas·E2E·감사 대표값
- 두 실행의 차이와 추가 측정 여부
- 개발용 SRS·ProductProfile·ownership 한계

Result와 Raw가 일치하지 않으면 M9를 완료로 표시하지 않습니다.

## 10. 어디에서 멈추나요?

M9에는 다음을 포함하지 않습니다.

- Besu·QBFT·validator·RPC node 구성
- 별도 Router·proxy·upgrade migration
- $2^{20}$ 선제 SRS와 자동 SRS 확장
- Ethereum EIP-4844 Ceremony SRS 사용
- Production MPC Ceremony·contribution client
- 새로운 Process·Issue Policy
- ProductProfile·운송 탄소·중간 Claim
- DPP ownership transfer·component DPP·Re-entry
- DKG·위원회 key rotation·운영 서버
- 외부 프로젝트와의 성능 순위 비교

M9의 완료는 **M8까지 구현한 Protocol을 하나의 개발용 universal SRS·최종 Contract·Anvil 시나리오로 재현하고 비용 경계를 측정했다는 의미**입니다. Production 배포 준비가 끝났다는 의미는 아닙니다.
