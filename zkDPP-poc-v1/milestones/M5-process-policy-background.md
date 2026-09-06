# M5 Process·Policy Background

이 문서는 M5 구현 방법이 아니라 **왜 현재 Process·Policy 구조를 선택했는지** 보존합니다. 실제 구현 기준은 [M5 Process·Policy 명세](M5-process-policy.md)입니다.

## 0. 30초 안에 Context 복구하기

```text
Factory가 공정 규칙·근거 제출
  → Policy Authority가 Policy ID 예약
  → policyRef 확정
  → Factory가 policyScopeRef 계산
  → Certifier가 Circuit Setup
  → PolicyRecord·PolicyGrant 등록
  → Operator가 같은 Policy로 Process proof 반복 생성
```

| 객체 | 한 줄 역할 |
|---|---|
| `policyRef` | Authority가 승인한 Policy version의 공개 ID |
| `PolicyRecord` | verifier·VK hash·활성 상태를 보관하는 온체인 등록증 |
| `policyScopeRef` | 특정 ZK owner가 특정 Policy를 사용하는 공개 가명 |
| `PolicyGrant` | 그 scope가 그 Policy를 사용해도 된다는 허가 |
| `AuditRecord` | Policy와 달리 실제 Event 한 건을 기록하는 객체 |

PolicyRecord는 Event마다 생기지 않습니다. Policy 하나를 여러 Process transaction이 재사용합니다. AuditRecord는 실제 Event마다 하나씩 생기므로 두 객체의 생애주기가 다릅니다.

## 1. 기존 명세에서 어디까지 정했나요?

### [기존 명세] PolicyRef

기존 명세는 다음 형태만 정했습니다.

$$
policyRef=H(\mathrm{PolicyRefTag},eventKind,policyIdentifier,version)
$$

`policyIdentifier`는 Policy 이름 또는 ID라고만 설명됐으며 순번·UUID·내용 Hash 중 무엇인지는 정하지 않았습니다.

### [기존 명세] PolicyRecord

```text
PolicyRecord:
  eventKind
  vkHash
  verifierRef
  enabled
```

- 등록 시 `enabled=true`입니다.
- Disable은 `true→false`만 허용합니다.
- 규칙·VK 변경은 기존 Record 수정이 아니라 새 PolicyRef 등록입니다.
- Disable은 과거 transition·Claim에 소급하지 않습니다.

### [기존 명세] PolicyGrant

```text
policyGrants[policyScopeRef][policyRef] = allowed
```

Scope는 company·recipe·인증 범위를 직접 노출하지 않는 opaque ID였습니다. 누가 scope를 소유하고 어떤 credential로 증명하는지는 `PROFILE-AUTH`의 미정 사항이었습니다.

### [기존 명세] Process와 Issue

Process와 Issue 모두 Policy를 사용합니다. 따라서 `eventKind`는 같은 Registry에서 두 Event의 Policy를 구분하는 데 필요합니다.

```text
PROCESS Policy:
  private State transition

ISSUE Policy:
  final private State의 Claim threshold
```

## 2. 이번에 무엇을 구체화했나요?

### [이번 결정] Policy Authority namespace

Policy Authority는 공개 책임 주체이므로 EVM account를 사용합니다. System Admin이 account를 등록할 때 `authorityId`를 1부터 증가시켜 발급합니다.

```text
authorityId = 0:
  미등록

authorityId >= 1:
  등록된 Policy Authority
```

Authority ID와 Policy ID는 재사용하지 않습니다. 같은 기관이 Operator 역할도 수행하면 Note 소유용 `sk_owner`는 별도로 사용합니다.

### [이번 결정] Policy family와 version

`policyId`는 각 Authority 내부의 Policy family ID이고 `version`은 그 family의 immutable 개정판입니다.

$$
policyRef=\mathrm{Poseidon2}(\mathrm{PolicyRefTag},eventKind,authorityId,policyId,version)
$$

PolicyRef를 Circuit Setup 전에 알아야 하므로 Authority가 ID·version을 먼저 예약합니다. 변경된 규칙은 같은 family의 새 version으로 예약하고 새 Circuit·PK·VK·PolicyRecord를 만듭니다.

### [이번 결정] Policy별 ZK 가명

Factory의 EVM `msg.sender`를 Policy 사용자 identity로 사용하지 않습니다. 같은 `sk_owner`로 Note 소유권과 Policy 사용 scope를 함께 증명합니다.

$$
policyScopeRef=\mathrm{Poseidon2}(\mathrm{ScopeRefTag},sk_{\mathrm{owner}},policyRef)
$$

| 경우 | 공개 연결성 |
|---|---|
| 같은 Factory·같은 Policy version | 같은 scopeRef로 연결됨 |
| 같은 Factory·다른 Policy | 다른 scopeRef이므로 직접 연결 어려움 |
| 같은 Factory·새 version | 새 policyRef·scopeRef·Grant 필요 |

Authority는 `sk_owner`를 모릅니다. Factory가 policyRef를 받은 뒤 scopeRef를 계산해 인증된 off-chain 채널로 전달합니다. Process Circuit이 나중에 같은 secret과의 관계를 검증합니다.

## 3. Policy 등록 Interaction이 왜 두 단계인가요?

```text
1. Factory → Authority
   공정 규칙과 근거 제출

2. Authority
   policyId·version 예약
   policyRef 계산·전달

3. Factory
   policyScopeRef 계산·전달

4. Certifier
   policyRef에 binding된 Circuit Setup

5. Authority
   PolicyRecord와 최초 Grant 등록
```

ScopeRef에 policyRef가 들어가므로 Factory가 policyRef를 받은 뒤 한 번 더 응답해야 합니다. 이 interaction은 Policy 등록·version 변경 시에만 발생하며 매 Process마다 반복하지 않습니다.

POC에서는 Certifier와 Policy Authority를 같은 신뢰 주체·EVM account로 취급합니다. Production 역할 분리·multisig·account rotation은 보류합니다.

## 4. Process 수학은 왜 비율 기반인가요?

절대 `q_loss`와 `delta_e_process`를 Circuit constant로 고정하면 batch 질량이 달라질 때 같은 VK를 재사용하기 어렵습니다. 그래서 손실률과 탄소집약도를 constant로 고정하고 실제 delta는 private input 질량에서 계산합니다.

### 대표 3-to-2 공정

```text
Input 합:
  q_mass = 320 kg
  a_rec  = 30 kg
  e      = 230 kgCO2e

Process:
  질량 손실률 = 6.25%
  추가 탄소   = input 1 kg당 0.09375 kgCO2e

Intermediate:
  q_mass = 300 kg
  a_rec  = 30 kg
  e      = 260 kgCO2e

Output:
  ELIGIBLE = (270, 30, 260)
  WASTE    = ( 30,  0,   0)
```

### 확정 constant

```text
D = 1,000,000,000
inputArity = 3
outputArity = 2
lossRate = 62,500,000
carbonIntensity = 93,750,000
```

$$
q_{\mathrm{loss}}=\left\lfloor\frac{T_{\mathrm{in}}\cdot lossRate}{D}\right\rfloor
$$

$$
\Delta e_{\mathrm{process}}=\left\lfloor\frac{T_{\mathrm{in}}\cdot carbonIntensity}{D}\right\rfloor
$$

`carbonIntensity`의 단위는 kgCO2e/kg input입니다. 실제 산업 공정의 고정비·비선형성·허용 범위는 이번 POC에서 모델링하지 않습니다.

## 5. Allocation Matrix는 설명이고 구현은 원소별 관계입니다

행은 output, 열은 `(q_mass,a_rec,e)`입니다.

$$
A=
\begin{pmatrix}
900{,}000{,}000 & 1{,}000{,}000{,}000 & 1{,}000{,}000{,}000\\
100{,}000{,}000 & 0 & 0
\end{pmatrix}
$$

```text
Output 0 ELIGIBLE:
  질량 90%, a_rec 100%, 탄소 100%

Output 1 WASTE:
  질량 10%, a_rec 0%, 탄소 0%
```

Circuit은 범 일반 matrix library를 만들지 않습니다. 각 State 원소에 대해 constant multiplication·floor·remainder 관계를 검증하고 coefficient 0은 output 0 equality로 단순화합니다.

모든 비율은 floor합니다. WASTE output을 먼저 floor 계산하고 ELIGIBLE output은 intermediate State에서 WASTE를 빼 conservation residual을 받습니다. 이 규칙은 M3·M4의 `output 2 floor, output 1 residual`과 같습니다.

## 6. 왜 탄소와 a_rec을 ELIGIBLE에 모두 배분하나요?

### [이번 결정] 탄소

경제적 가치가 없는 true waste에는 공통 공정 emissions를 배분하지 않는 [GHG Protocol Product Life Cycle Standard](https://ghgprotocol.org/sites/default/files/ghgp/standards/Product-Life-Cycle-Accounting-Reporting-Standard-EReader_041613_0.pdf)의 방향을 POC 근거로 사용합니다. Input 탄소와 Process 추가 탄소를 ELIGIBLE output에 전부 귀속합니다.

이 결정은 판매 가능한 co-product에는 적용하지 않습니다. Waste treatment·소각·매립의 후속 배출이 사라진다는 뜻도 아니며 별도 처리 단계나 system boundary에서 회계해야 합니다. [EU Environmental Footprint 방법](https://environment.ec.europa.eu/system/files/2021-12/Annexes%201%20to%202.pdf)처럼 disposal·recycling burden을 별도로 모델링하는 체계도 있으므로 보편 규칙으로 주장하지 않습니다.

### [POC adaptation] recycled attribution

`a_rec`은 폐기물의 물리적 재활용 물질 조성값이 아니라 ELIGIBLE 제품에 귀속되는 recycled-material attribution입니다. WASTE에는 Claim을 귀속하지 않고 ELIGIBLE에 전부 상속합니다. 이는 보편적인 규제 규칙이 아니라 이번 Policy 선택입니다.

### [이번 결정] Output Role

Factory가 제안한 output 의미와 근거를 Certifier가 검토하고 다음 Role을 Circuit constant로 고정합니다.

```text
output 0 = ELIGIBLE
output 1 = WASTE
```

Factory가 transaction마다 Role을 바꿀 수 없습니다. Product Type과 근거 문서 진실성은 Circuit 밖에서 Certifier가 판단합니다.

## 7. PolicyRef 공개는 무엇을 노출하나요?

PolicyRef는 stable fingerprint입니다. 같은 public policyRef를 사용하면 어떤 승인 규칙을 통과했는지와 같은 Policy version을 사용했다는 사실이 드러납니다.

이번 POC는 다음 Privacy 경계를 채택합니다.

```text
숨김:
  input Note·State·DocumentHash·소유 address·정확한 output State

공개:
  policyRef
  policyScopeRef
  root·nullifier·output commitments
```

Policy 의미·Role·recipe fingerprint까지 숨기는 generic private-policy proof는 구현하지 않습니다.

## 8. VK 표현 세 가지를 구분합니다

```text
Serialized VK:
  gnark Go artifact
  현재 약 49,144 B

On-chain optimized VK:
  Solidity verification에 실제 필요한 Policy-specific word
  기존 verifier 관찰 기준 약 38 words

Verifier runtime:
  검증 코드 + 공통 SRS + optimized VK
  현재 약 7.1KB
```

49KB serialized artifact 전체를 SSTORE하는 비교는 잘못된 기준입니다. 같은 logical VK를 Solidity용 canonical optimized representation으로 추출해 비교합니다.

## 9. Verifier 방식 A와 B

### 방식 A: Policy별 constant verifier

```text
Policy별 Verifier Contract:
  공통 verifier logic
  공통 SRS constant
  Policy VK constant

PolicyRecord:
  verifierRef
```

현재 gnark exporter와 M1~M4가 사용하는 방식입니다. [`poc-v2` 평가 보고서](../../poc-v2/POC%20Evaluation%20Report.tex)에서는 verifier마다 약 1.60M gas의 deployment를 실제 측정했습니다. 장점은 VK SLOAD가 없고 generated source를 그대로 사용할 수 있다는 점입니다. 단점은 Policy마다 검증 코드·공통 SRS가 반복 배포된다는 점입니다.

### 방식 B: Storage VK와 generic verifier

```text
Generic Verifier Contract:
  공통 verifier logic
  공통 SRS

Main storage:
  policyRef → optimized VK words
```

Generated verifier의 constant 참조를 memory VK field 참조로 바꿉니다. Policy 등록비용은 낮아질 수 있지만 proof마다 VK SLOAD·memory 구성이 추가됩니다.

두 방식은 같은 proof·public input·logical VK를 검증해야 합니다. 방식 A를 공식 Protocol 경로로 사용하고 B는 architecture ablation으로 분리합니다.

### vkHash

$$
vkHash=\mathrm{SHA256}(\mathrm{CanonicalOnchainVKEncoding})
$$

같은 canonical optimized VK encoding을 A의 constant extractor와 B의 storage loader가 공유합니다. Serialized Go VK checksum과 별도 필드로 혼동하지 않습니다.

## 10. Hash 선택 원칙

| 목적 | Hash |
|---|---|
| Circuit에서 관계를 검증 | Poseidon2 |
| PolicyRef·ScopeRef | Poseidon2 |
| Note·Voucher·nullifier·Merkle | Poseidon2 |
| PolicySpec·근거 문서·Artifact checksum | SHA-256 |
| canonical optimized VK의 `vkHash` | SHA-256 |
| Domain 문자열을 field constant로 변환 | SHA-256 기반 hash-to-field |

DocumentHash와 Domain hash-to-field는 off-chain에서 계산됩니다. Circuit은 이미 계산된 field 값을 사용하므로 SHA-256 compression constraints를 추가하지 않습니다.

## 11. Policy lifecycle을 왜 immutable하게 두나요?

같은 policyRef가 다른 verifier·규칙을 가리키면 과거 Process와 Claim이 어떤 규칙을 통과했는지 모호해집니다.

```text
같은 policyRef 재등록: 금지
Record·verifier·vkHash 수정: 금지
enabled true→false: 허용
disabled 재활성화: 금지
새 규칙: 같은 policyId의 새 version
```

Grant는 Policy 규칙이 아니라 Factory의 현재 인증 상태이므로 revoke·regrant를 허용합니다. Disable과 Grant 변경은 과거 accepted Process에 소급하지 않습니다.

## 12. 폐기하거나 보류한 대안

### [폐기]

- Factory를 EVM `msg.sender`로 식별
- Policy Authority를 ZK proof로 인증
- raw 49KB serialized VK 전체 SSTORE
- mutable PolicyRecord·verifier replacement
- Chameleon Hash로 같은 PolicyRef 유지
- `M_max`, `N_max`, inactive padding
- Product Type 현실 진실성을 Circuit에서 검증

### [보류]

- Policy 간·동일 Policy 내 완전 unlinkability
- private Policy credential·Grant Tree
- range Policy·비선형 delta·고정비 탄소
- co-product allocation과 waste-treatment stage
- Certifier·Policy Authority 역할 분리와 multisig
- cross-chain authority namespace

## 13. 최종 결정 요약

```text
Identity:
  Authority = EVM account + authorityId
  Operator  = sk_owner

Policy:
  exact 3-to-2
  immutable family/version
  rate·allocation·Role constant

Authorization:
  scopeRef = H(ScopeRefTag, sk_owner, policyRef)
  policyGrants[policyRef][scopeRef]

Verifier:
  A = constant verifier, 공식 경로
  B = storage optimized VK, 비교 실험

Privacy:
  input cm·State 숨김
  policyRef·scopeRef 공개
```
