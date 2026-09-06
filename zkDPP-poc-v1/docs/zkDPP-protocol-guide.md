# zkDPP는 어떻게 동작하나요?

이 문서는 zkDPP 프로토콜의 **목적과 실제 동작**를 설명하는 사람용 안내서입니다. 구현 파일을 읽지 않아도 각 Event의 입력, 검증 내용, 결과와 온체인 변화를 이해할 수 있도록 작성했습니다.

본문은 M9에서 실제로 실행한 최종 POC를 기준으로 합니다. “현재 구현하지 않았습니다”라고 표시한 내용만 범위 밖입니다.

정확한 공개 입력 순서·Contract 함수·Circuit 상수·구현 위치를 확인하거나 수정하려면 [프로토콜 상세 명세](zkDPP-protocol-reference.md)를 사용합니다. AI에게 질문할 때는 이 안내서와 [AI용 프로토콜 YAML](zkDPP-protocol.yaml)을 함께 제공합니다.

이 문서에서 **Event**는 Entry·Transfer·Process처럼 프로토콜 상태를 바꾸는 작업을 뜻합니다. Solidity의 event는 **Event log**라고 구분해서 부릅니다.

| 핵심 용어 | 이 문서에서의 의미 |
|---|---|
| **Circuit** | private witness와 public input의 관계를 검증합니다. |
| **Contract** | 현재 온체인 상태와 proof를 확인하고 상태 변경을 집행합니다. |
| **commitment** | private 객체를 공개하지 않고 나타내는 공개 식별값입니다. Note는 $cm$, Voucher는 $rv$를 사용합니다. |
| **nullifier** | private 객체를 소비할 때 공개하는 중복 소비 방지값입니다. Note는 $nf$, Voucher는 $rvnf$를 사용합니다. |
| **Merkle root·path** | root는 Tree 전체를 대표하고, private path는 특정 commitment의 포함 관계를 증명합니다. |
| **State** | 물품의 Sustainability State를 뜻합니다. Contract의 storage·Status와 구분합니다. |

## 0. 30초 안에 무엇을 기억하면 되나요?

**zkDPP는 공급망 물품의 제품 정보와 Sustainability State를 공개하지 않고도, 올바른 공급망 작업이 실행됐음을 ZK proof로 검증하는 프로토콜입니다.** 이 문서에서는 이후 Sustainability State를 **State**라고 줄여 씁니다.

물품은 공급망 안에서 **Note**로 존재합니다. 전달 중에는 **Voucher**가 됩니다. 공급망 처리가 끝나면 **Exit**를 거쳐 DPP commitment로 남고, 그 DPP에 지속가능성 주장인 **Claim**을 등록할 수 있습니다.

| 단계 | Event | 무엇이 일어나나요? |
|---|---|---|
| 최초 등록 | **Entry** | 최초 비공개 Note를 만듭니다. |
| 소유권 전달 | **Transfer** | Note를 수신자용 Voucher와 송신자의 Change Note로 나눕니다. |
| 전달 완료·회수 | **Proceed·Recall** | Voucher를 수신자 Note 또는 송신자 Return Note로 바꿉니다. |
| 물품 변환 | **Merge·Split·Process** | Note를 합치거나 나누거나 공정 규칙에 따라 변환합니다. |
| 공급망 종료 | **Exit** | Note를 소비하고 DPP commitment를 등록합니다. |
| Claim 등록 | **Issue** | DPP의 비공개 State가 Policy를 만족한다는 Claim을 등록합니다. |

**각 Event는 Circuit 검증과 Contract 검증의 두 단계로 실행합니다.**

1. Circuit은 각 Event에 필요한 비공개 객체·소유권·State 관계를 검증합니다. Issue처럼 소유권을 검증하지 않는 Event도 있습니다.
2. Contract는 현재 온체인 상태와 proof를 확인한 뒤 필요한 상태만 변경합니다.

Contract는 Note나 Voucher의 실제 내용을 보지 않습니다. 대신 **commitment, Merkle root, nullifier와 proof**를 사용합니다.

## 0.1 실제 성능은 어느 정도인가요?

성능은 세 구간으로 나누어 봐야 합니다.

1. **Participant:** proof를 만드는 데 걸리는 시간
2. **온체인:** 트랜잭션이 소비한 gas
3. **Auditor와 Committee:** 기록을 조회·복호화하고 관계를 추적하는 시간

### proof를 만드는 데 얼마나 걸리나요?

M9 로컬 측정에서는 **가장 큰 Process proof가 약 1.63초** 걸렸습니다. Issue는 약 0.25초로 가장 짧았습니다.

| Event | Participant의 proof 생성 시간 |
|---|---:|
| **Entry** | 약 0.44초 |
| **Transfer** | 약 0.85초 |
| **Proceed** | 약 0.83초 |
| **Recall** | 약 0.85초 |
| **Merge** | 약 0.87초 |
| **Split** | 약 0.86초 |
| **Process** | 약 1.63초 |
| **Exit** | 약 0.83초 |
| **Issue** | 약 0.25초 |

준비된 proof를 로컬에서 검증하는 시간은 Event별로 약 2.6~2.7밀리초였습니다. 이 값은 EVM 트랜잭션 시간이 아니라 Go에서 수행한 proof 검증 시간입니다.

### 온체인에서는 gas가 얼마나 필요한가요?

| Event | 대표 gas | 비용이 생기는 주된 이유 |
|---|---:|---|
| **Entry** | 첫 실행 2,258,465<br>이후 약 1.69~1.71M | Note Tree와 감사 기록 생성 |
| **Transfer** | 첫 실행 3,596,789<br>이후 약 3.01~3.06M | Note·Voucher Tree를 함께 변경 |
| **Proceed** | 약 1.78M | Voucher 소비와 Note 생성 |
| **Recall** | 약 1.78M | Voucher 소비와 Return Note 생성 |
| **Merge** | 약 1.85M | Note 두 개 소비와 Note 한 개 생성 |
| **Split** | 약 2.72M | Note 한 개 소비와 Note 두 개 생성 |
| **Process** | 약 2.93M | Note 세 개 소비와 Note 두 개 생성 |
| **Exit** | 약 0.62M | Note 소비와 DPP 등록, Tree 추가 없음 |
| **Issue** | 약 0.46M | Claim 기록 추가, Tree 변경 없음 |

첫 Entry와 첫 Transfer는 비어 있던 저장 영역을 처음 사용하므로 이후 같은 Event보다 gas가 더 큽니다. 출력 객체와 AuditRecord가 많거나 두 Tree를 함께 변경하는 Event도 더 많은 gas를 사용합니다.

### 감사에는 얼마나 걸리나요?

| 감사 방향 | 대표 시간 | 확인한 범위 |
|---|---:|---|
| **역방향 추적** | 약 8.17초 | Standard Claim에서 원자재 Entry 4건까지 |
| **정방향 추적** | 약 4.08초 | Aluminum A에서 DPP 2개와 미소비 Note 3개까지 |

감사 시간의 대부분은 로컬 RPC로 원본 트랜잭션과 AuditRecord를 조회하는 데 사용됐습니다. 역방향 추적은 19개 기록을 확인했고, 정방향 추적은 10개 기록을 확인했기 때문에 더 오래 걸렸습니다.

### 이 숫자는 어떻게 해석해야 하나요?

**이 값은 로컬 Anvil과 개발용 SRS에서 측정한 POC 결과입니다. 운영 환경의 평균 성능이나 비용을 의미하지 않습니다.**

- proof 생성은 GOMAXPROCS=8인 로컬 환경에서 측정했습니다.
- gas는 이미 만들어진 고정 proof를 Anvil에 제출해 측정했습니다.
- proof 생성 시간과 트랜잭션 처리 시간은 별도로 측정했으므로 하나의 사용자 체감 시간으로 합치지 않습니다.
- 감사 시간은 작은 고정 공급망 그래프의 단일 POC 결과입니다.
- Setup과 SRS 생성은 매 Event 실행 때 반복하는 비용이 아니므로 위 표에서 제외했습니다.

소수점까지 포함한 원시 측정값과 실행 횟수는 [M9 Result](../milestones/M9-final-integration-result.md)에서 확인할 수 있습니다.

## 1. 물품은 어떤 객체로 표현하나요?

### 1.1 Sustainability State는 무엇인가요?

물품의 State는 다음 세 값입니다.

$$
State=(q_{\mathrm{mass}},a_{\mathrm{rec}},e)
$$

| 값 | 의미 | 단위 |
|---|---|---|
| $q_{\mathrm{mass}}$ | 물품의 전체 질량 | kg |
| $a_{\mathrm{rec}}$ | 질량 균형 방식으로 물품에 할당된 재활용량 | kg |
| $e$ | Entry 이전 탄소를 포함하는 누적 탄소 | kgCO2e |

실제 값은 $10^9$ 배율의 uint64로 표현합니다. Contract는 세 값을 직접 저장하지 않고, 이 값들을 포함한 commitment를 저장합니다.

AssetRole은 ELIGIBLE과 WASTE를 구분합니다. 모든 객체에서 $a_{\mathrm{rec}}\le q_{\mathrm{mass}}$여야 합니다. WASTE는 $a_{\mathrm{rec}}=0$과 $e=0$을 만족해야 합니다.

### 1.2 Note는 무엇인가요?

**Note는 공급망 안에 있는 물품의 현재 비공개 상태입니다.**

소유자는 다음 정보를 오프체인에서 보관합니다.

- ProductName과 LotID에서 만든 DocumentHash
- AssetRole과 State
- ZK 소유자 주소
- commitment를 숨기는 opening
- 소유자 비밀값인 $sk_{\mathrm{owner}}$

**Note의 공개 식별값은 $cm$입니다.** Note가 생성되면 $cm$만 Note Tree에 추가됩니다.

Note Tree root는 그 시점까지 추가된 모든 $cm$을 대표합니다. Merkle path는 특정 $cm$이 그 root의 Tree에 들어 있음을 확인할 때 사용하는 형제 노드와 리프 위치입니다. zkDPP는 이 경로를 Circuit 비공개 입력으로 사용합니다.

**Note를 소비할 때는 $cm$을 트랜잭션에 다시 공개하지 않고 $nf$를 공개합니다.** $nf$는 $sk_{\mathrm{owner}}$와 $cm$이 정해지면 하나로 결정됩니다. Contract는 $nf$를 소비 기록의 키로 사용해 같은 Note의 중복 소비를 막습니다.

| 시점 | 공개되는 값 | 비공개로 사용하는 값 |
|---|---|---|
| Note 생성 | $cm$ | Note 내용·opening·$sk_{\mathrm{owner}}$ |
| Note 소비 | Note root·$nf$ | $cm$·Note 내용·Merkle 리프 위치와 경로·$sk_{\mathrm{owner}}$ |

Note의 ZK 소유자 주소는 EVM 계정과 다른 값입니다. Circuit은 $sk_{\mathrm{owner}}$에서 ZK 소유자 주소를 계산합니다. Contract는 트랜잭션의 msg.sender를 Note 소유권 판단에 사용하지 않습니다.

### 1.3 Voucher는 무엇인가요?

**Voucher는 Transfer가 생성하는 임시 비공개 객체입니다.** 수신자가 Proceed하거나 송신자가 Recall하면 Voucher의 생애주기가 끝납니다.

Voucher에는 다음 정보가 들어 있습니다.

- DocumentHash·AssetRole·State
- 송신자와 수신자의 ZK 소유자 주소
- Recall 가능한 마지막 시점을 정하는 deadlineEpoch
- 송신자와 수신자가 공유하는 opening

**Voucher의 공개 식별값은 $rv$이고 nullifier는 $rvnf$입니다.** 송신자와 수신자가 같은 opening을 공유하므로 같은 $rvnf$를 계산할 수 있습니다.

Voucher opening을 안전하게 전달하는 통신 방식은 현재 POC에서 구현하지 않았습니다.

### 1.4 DPP와 Claim은 무엇인가요?

**Exit는 Note의 DocumentHash·AssetRole·State를 그대로 사용해 DPP commitment를 만듭니다.** Note의 ZK 소유자 주소와 Note opening은 DPP로 옮기지 않습니다. DPP에는 새로운 dppOpening을 사용합니다.

dppOpening은 commitment를 숨기는 값이며 소유권을 proof하는 값이 아닙니다. 현재 POC는 DPP 소유권 이전을 검증하지 않습니다.

**DPP는 소비하지 않으므로 DPP Tree와 nullifier가 없습니다.**

**Claim은 별도 객체 ID나 해시가 아닙니다.** Contract는 다음 두 값을 함께 Claim의 키로 사용합니다.

$$
ClaimKey=(dppCommitment,issuePolicyRef)
$$

같은 DPP에 같은 issuePolicyRef를 두 번 등록할 수 없습니다. Policy 버전이 달라지면 issuePolicyRef도 달라지므로, 같은 DPP가 V1과 V2 Claim을 각각 가질 수 있습니다.

## 2. 누가 무엇을 담당하나요?

### 시스템을 설정하는 주체

| 주체 | 하는 일 |
|---|---|
| **System Admin** | Contract를 배포하고 EntryIssuer와 Policy Authority를 등록합니다. |
| **Policy Authority** | Process·Issue Policy와 verifier Contract를 등록하고 Process PolicyGrant를 관리합니다. |
| **Status Authority** | Note·Voucher·Claim을 Frozen·Active·Revoked로 변경합니다. 배포 시 계정이 고정됩니다. |

### 공급망 Event를 실행하는 주체

**Participant는 비공개 Note·Voucher·DPP 데이터를 보관하고 proof를 준비합니다.** Factory는 별도 Contract 역할이 아니라 Process를 실행하는 Participant를 설명하는 업무상 이름입니다.

**M9 대표 시나리오에서는 EntryIssuer와 Note 소유 Participant를 동일한 업무상 주체로 배치했습니다.**

- actor-1이 소유하는 Aluminum Note는 actor-1에 대응하는 ISSUER1 EVM 계정으로 Entry합니다.
- actor-3이 소유하는 Cathode·Anode Note는 actor-3에 대응하는 ISSUER3 EVM 계정으로 Entry합니다.

따라서 대표 시나리오의 흐름은 다음과 같습니다.

1. System Admin이 Participant의 EVM 계정을 EntryIssuer로 등록합니다.
2. 같은 Participant가 자기 소유자 비밀값으로 Note와 Entry proof를 만듭니다.
3. 그 Participant가 EntryIssuer EVM 계정으로 Entry 함수를 호출합니다.

다만 이것은 **POC 시나리오에서 사용한 역할 배치**입니다. Contract는 EntryIssuer EVM 계정과 Note의 ZK 소유자 주소가 같은 사람인지 확인하지 않습니다.

현재 POC가 온체인에서 강제하는 규칙은 다음 두 가지뿐입니다.

1. Entry 함수를 호출한 EVM 계정이 EntryIssuer로 등록돼 있어야 합니다.
2. Circuit이 Note의 소유자 비밀값과 ZK 소유자 주소의 관계를 검증해야 합니다.

EntryIssuer와 Participant를 서로 다른 주체로 분리하는 구조는 현재 구현하거나 검증하지 않았습니다. 향후 분리한다면 Participant가 만든 proof를 EntryIssuer가 승인·제출하는 절차와, EntryIssuer가 무엇을 확인하고 승인할지 추가로 설계해야 합니다.

### 감사를 수행하는 주체

| 주체 | 하는 일 |
|---|---|
| **Committee member 1·2·3** | 각자 가진 secret share로 AuditRecord의 partial decryption point를 계산합니다. |
| **Auditor** | 서로 다른 위원 두 명의 응답을 결합하고 부모·자손 관계를 탐색합니다. |

현재 POC는 Committee member와 Auditor가 정직하게 동작한다고 가정합니다. 이들은 Contract에 EVM 계정으로 등록되지 않습니다.

## 3. 온체인에는 무엇이 저장되나요?

**Contract는 비공개 State를 직접 저장하지 않습니다.** 다음 공개 식별값과 관계를 저장합니다.

### 객체의 존재와 Tree

| 온체인 상태 | 답하는 질문 |
|---|---|
| **Note Tree** | 어떤 Note commitment들이 생성됐나요? |
| **Voucher Tree** | 어떤 Voucher commitment들이 생성됐나요? |
| **commitments**[$cm$] | 같은 Note commitment가 이미 생성됐나요? |
| **voucherCommitments**[$rv$] | 같은 Voucher commitment가 이미 생성됐나요? |

Note Tree와 Voucher Tree는 서로 분리된 깊이 32 추가 전용 Merkle Tree입니다. Contract는 정상 추가로 만든 현재·과거 root를 기록합니다. Circuit은 비공개 Merkle path를 사용해 입력 commitment가 공개 root에 포함됐음을 proof합니다.

### 생성·소비·Claim은 감사 기록에 어떻게 연결되나요?

**성공한 공급망 Event는 AuditRecord를 하나씩 생성합니다.** Contract는 nextAuditRecordId에서 1부터 증가하는 새 ID를 발급합니다.

생성·소비·Claim 매핑은 AuditRecord를 다시 복사하지 않고, 관련된 AuditRecord ID만 저장합니다.

| 온체인 상태 | 키 | 저장하는 값 | 답하는 질문 |
|---|---|---|---|
| **producerOf** | $cm$·$rv$·dppCommitment | 생성 AuditRecord ID | 이 객체는 어느 Event에서 생성됐나요? |
| **noteSpentIn** | $nf$ | 소비 AuditRecord ID | 이 Note는 소비됐나요? 어디에서 소비됐나요? |
| **voucherSpentIn** | $rvnf$ | 소비 AuditRecord ID | Proceed와 Recall 중 어느 경로가 성공했나요? |
| **claimRecordOf** | dppCommitment·issuePolicyRef | Issue AuditRecord ID | 이 DPP에 해당 Policy Claim이 등록됐나요? |

값이 0이면 연결된 AuditRecord가 없다는 뜻입니다. 하나의 Event에서 생성·소비한 객체는 모두 같은 AuditRecord ID를 사용합니다.

### AuditRecord 본체에는 무엇을 저장하나요?

auditRecords[ID]에는 다음 내용이 들어 있습니다.

| 저장 내용 | 목적 |
|---|---|
| **Event 종류·Policy** | 어떤 Event와 Policy의 기록인지 구분합니다. |
| **생성 객체 목록** | 새로 만든 Note·Voucher·DPP를 찾습니다. |
| **public point $R_1$** | 위원들이 partial decryption을 계산할 때 사용합니다. |
| **부모 암호문** | 소비한 부모 $cm$·$rv$를 숨겨 저장합니다. |
| **출력 소비값 암호문** | 새 $cm$·$rv$가 나중에 사용할 $nf$·$rvnf$를 숨겨 저장합니다. |

트랜잭션의 공개값을 모두 복사하지는 않습니다. root·proof·입력 $nf$처럼 원본 트랜잭션이나 다른 매핑에서 확인할 수 있는 값은 다시 저장하지 않습니다.

Issue도 AuditRecord를 만들지만 암호화할 부모와 출력 소비값은 없습니다. 따라서 Issue AuditRecord에는 Event 종류와 policyRef만 기록하고 암호문은 비워 둡니다.

### Policy와 상태

| 온체인 상태 | 의미 |
|---|---|
| **policyRecords** | Policy 버전, verifier Contract, VK 해시와 활성 상태를 저장합니다. |
| **policyGrants** | 특정 policyScopeRef가 Process Policy를 사용할 수 있는지 저장합니다. |
| **noteStatusByNf** | Note의 Active·Frozen·Revoked 상태를 저장합니다. |
| **voucherStatusByNf** | Voucher의 Active·Frozen·Revoked 상태를 저장합니다. |
| **claimStatus** | DPP·Policy Claim의 Active·Frozen·Revoked 상태를 저장합니다. |

**감사 암호문**은 소비한 부모 $cm$·$rv$와 새 출력이 나중에 사용할 $nf$·$rvnf$를 숨겨 기록한 값입니다. Circuit은 실제 객체에서 계산한 값이 이 암호문에 들어 있는지 검증합니다. 4장에서는 이 저장값을 복호화해 관계를 추적하는 방법을 설명합니다.

## 4. 감사용 Threshold Encryption·Decryption은 어떻게 동작하나요?

**현재 POC는 원 TDH2 전체가 아닙니다.** 2-of-3 Threshold Diffie–Hellman 복원과 Poseidon2 Field masking을 결합한 감사 전용 POC입니다.

공개 트랜잭션만으로는 소비한 $cm$·$rv$를 알 수 없습니다. 각 Event는 자신에게 존재하는 부모 $cm$·$rv$ 또는 새 출력의 $nf$·$rvnf$만 암호화해 AuditRecord에 저장합니다. Entry에는 부모가 없고 Exit에는 출력 nullifier가 없으며, Issue는 암호화를 사용하지 않습니다.

현재 POC는 **위원 3명 중 서로 다른 2명이 협력하면 복호화할 수 있는 구조**를 사용합니다. 한 명만으로는 복호화할 수 없습니다.

전체 흐름은 다음과 같습니다.

| 순서 | 주체 | 하는 일 |
|---|---|---|
| **1. 키 설정** | 신뢰하는 생성자 | Committee public key $PK$와 위원별 secret share 3개를 만듭니다. |
| **2. 암호화** | Participant | 실제 부모·출력 소비값의 암호문을 계산합니다. |
| **3. 암호화 검증** | Event Circuit | Participant가 실제 값을 올바르게 암호화했는지 검증합니다. |
| **4. 온체인 기록** | Contract | proof를 검증한 뒤 $R_1$과 암호문을 AuditRecord에 저장합니다. |
| **5. Partial Decryption** | 서로 다른 위원 2명 | 각자 secret share로 응답 $D_i$를 하나씩 만듭니다. |
| **6. 원문 복원** | Auditor | 두 응답을 결합하고 부모 $cm$·$rv$와 출력 $nf$·$rvnf$를 복원합니다. |

### 4.1 무엇을 암호화하나요?

하나의 Event에서 감사에 필요한 값은 순서가 정해진 **BLS12-381 Circuit Field 원소 배열 $M$**입니다.

예를 들어 Note 3개를 출력 Note 2개로 바꾸는 **Process Event**는 다음 다섯 값을 암호화합니다.

$$
M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

앞의 세 값은 소비한 부모 Note의 commitment입니다. 뒤의 두 값은 새 출력 Note가 나중에 소비될 때 사용할 $nf$입니다.

**출력 $cm$은 이미 공개되므로 다시 암호화하지 않습니다.** AuditRecord의 생성 객체 목록에서 확인할 수 있습니다.

### 4.2 2-of-3 키는 어떻게 준비하나요?

개발용 신뢰 설정 과정에서 하나의 **master secret $x$**를 만들고, 이를 위원별 secret share $x_1,x_2,x_3$으로 나눕니다.

$G$는 프로토콜이 고정한 Jubjub 기준점입니다. $q$는 Jubjub prime-order subgroup의 order이며, $x$, $x_i$, $r$과 Lagrange 계수는 mod $q$로 계산합니다.

| 값 | 누가 가지나요? | 역할 |
|---|---|---|
| **Committee public key $PK=xG$** | 모든 Participant에게 공개하고 Event Circuit에 상수로 고정 | 감사 정보를 암호화할 때 사용합니다. |
| **secret share $x_i$** | 각 위원 한 명 | partial decryption을 계산할 때 사용합니다. |
| **master secret $x$** | 설정 시점에만 존재 | 조각을 만든 뒤 저장하지 않습니다. |

어느 위원도 master secret 전체를 가지고 있지 않습니다. Auditor는 위원들의 응답을 결합하지만 master secret 자체를 복원하지도 않습니다.

현재 POC는 신뢰하는 생성자가 키를 만들고 조각으로 나누는 방식을 사용합니다. 위원들이 함께 키를 만드는 **분산 키 생성(DKG)**과 키 교체는 구현하지 않았습니다.

### 4.3 Participant는 어떻게 암호화하나요?

Participant는 Event마다 새로운 비공개 난수 $r$을 선택합니다.

$$
R_1=rG,
\qquad
Z=rPK
$$

$R_1$은 AuditRecord에 공개합니다. 공유점 $Z$는 Participant와 Circuit이 계산에 사용하지만 공개하지 않습니다.

**Committee public key는 $PK$입니다. $K$는 public key가 아닙니다.** $K$는 감사 평문을 가리기 위해 공유점 $Z$에서 파생하는 **private Field masking key**입니다. 이 문서에서는 $PK$와 $K$를 서로 다른 값으로 일관되게 구분합니다.

Audit Context $L$은 Event 종류와 해당 Event의 public input을 묶은 해시입니다. $n$은 이 Event가 암호화하는 평문 Field 원소의 개수입니다.

| 값 | 공개 여부와 저장 위치 |
|---|---|
| **$PK$** | 공개값이며 Event Circuit에 상수로 고정합니다. |
| **$r$** | Participant가 보관하는 private witness입니다. |
| **$R_1$** | public input이며 AuditRecord에도 저장합니다. |
| **$Z$** | 비공개 계산값이며 저장하지 않습니다. |
| **$L$** | public input에서 다시 계산할 수 있으며 별도 저장하지 않습니다. |
| **$n$** | Event별 평문 배열 길이에서 결정하며 별도 저장하지 않습니다. |
| **$K,k_j$** | private masking key와 위치별 마스크이며 저장하지 않습니다. |
| **$C_j$** | public input이며 AuditRecord에 암호문으로 저장합니다. |
| **$D_i$** | Committee member가 Auditor에게 off-chain으로 전달하며 온체인에 저장하지 않습니다. |

Participant는 $Z$, $L$, $n$을 Poseidon2로 해시해 Field masking key $K$를 만듭니다.

$$
K=H(AuditKeyTag,Z_X,Z_Y,L,n)
$$

하나의 $K$를 그대로 모든 값에 더하지는 않습니다. 배열의 위치 $j$마다 다른 마스크 $k_j$를 만듭니다.

$$
k_j=H(AuditMaskTag,K,j)
$$

각 평문 $M_j$에는 같은 위치의 마스크를 더합니다. $p$는 BLS12-381 scalar field이자 Circuit Field의 modulus입니다. $M_j$, $C_j$, $K$와 $k_j$의 덧셈·뺄셈은 모두 mod $p$로 계산합니다. 계산 결과가 $p$에 도달하면 0부터 다시 이어지는 Field 연산입니다.

$$
C_j=M_j+k_j\pmod p
$$

따라서 감사 암호문 한 묶음은 하나의 $K$를 사용하지만, 각 위치 $j$에는 서로 다른 $k_j$를 사용합니다.

### 4.4 $L$과 $n$은 필수값인가요?

**$L$과 $n$은 Threshold DH의 정확한 암호화·복호화에 필수값이 아닙니다.** $R_1$, $Z$와 위치 $j$만으로도 복원 가능한 Field masking을 만들 수 있습니다. 현재처럼 Event별 평문 순서와 개수가 고정된 POC에서는 두 값이 주는 실익도 작습니다.

두 값은 기존 zkDPP 프로토콜이나 원 TDH2가 요구한 값이 아닙니다. **M6-B1 POC 구현 과정에서 AI가 추가 연결을 위해 자동 제안해 넣은 보조값입니다.**

| 값 | AI가 추가한 목적 | 현재 필요성 |
|---|---|---|
| **Audit Context $L$** | Event 종류와 public input을 $K$ 계산에 한 번 더 연결 | Event Circuit이 이미 public input과 암호문의 관계를 검증하므로 핵심 암호화 관계에는 필수적이지 않음 |
| **평문 길이 $n$** | 감사 암호문의 Field 원소 개수를 $K$ 계산에 연결 | Event마다 평문 순서와 길이가 고정돼 있으므로 중복 정보 |

따라서 $L$과 $n$은 **현재 구현을 단순화할 때 제거를 검토할 수 있는 보조값**입니다. 다만 현재 M6-B1~M9 Circuit·PK·VK·proof는 두 값을 포함해 생성했습니다. 이 문서는 실제 구현을 설명하기 위해 두 값을 표시하지만, 프로토콜의 본질적인 필수값으로 취급하지 않습니다. 실제로 제거하면 감사 Circuit과 모든 PK·VK·proof를 다시 생성해야 합니다.

### 4.5 Event Circuit은 무엇을 검증하나요?

Participant가 임의의 값을 암호화하면 감사 연결이 끊깁니다. 이를 막기 위해 공급망 Event의 Circuit이 암호화를 함께 검증합니다.

1. Circuit이 실제 비공개 입력에서 부모 $cm$·$rv$를 다시 계산합니다.
2. Circuit이 실제 출력과 소유자 비밀값에서 출력 $nf$·$rvnf$를 계산합니다.
3. Circuit이 이 값들을 Event별 순서로 평문 배열에 넣습니다.
4. Circuit이 비공개 난수 $r$과 고정된 Committee public key $PK$로 $R_1$, $Z$, $K$와 위치별 마스크를 다시 계산합니다.
5. 계산한 암호문이 트랜잭션에 공개된 암호문과 같은지 확인합니다.

**proof가 성공하면 AuditRecord의 암호문이 실제 부모와 출력 소비값을 담고 있음을 알 수 있습니다.** 평문의 실제 값은 공개되지 않습니다.

Audit Context $L$은 이 연결을 $K$ 계산에도 추가로 반영합니다. 하지만 암호문과 public input의 기본 관계는 Event Circuit proof 자체가 이미 검증합니다.

### 4.6 Committee와 Auditor는 어떻게 복호화하나요?

Auditor는 복호화할 AuditRecord ID를 정합니다. 각 Committee member는 같은 온체인 원본을 확인한 뒤 자기 secret share와 public point $R_1$으로 partial decryption point $D_i$를 계산합니다.

$$
D_i=x_iR_1
$$

**partial decryption은 평문의 일부를 복호화한다는 뜻이 아닙니다.** 각 Committee member가 DH shared point $Z$를 복원하는 데 필요한 점 하나를 계산한다는 뜻입니다. $D_i$는 온체인에 저장하지 않고 Auditor에게 off-chain으로 전달합니다.

Auditor는 서로 다른 위원 두 명의 $D_i$를 Lagrange 계수로 결합합니다. 그러면 master secret을 직접 복원하지 않고도 암호화 때 사용한 것과 같은 $Z$를 얻습니다.

$$
Z=xR_1=rPK
$$

Auditor는 이 $Z$에서 같은 Field masking key $K$와 위치별 마스크를 다시 만들고 암호문에서 마스크를 뺍니다.

$$
M_j=C_j-k_j\pmod p
$$

그 결과 부모 $cm$·$rv$와 출력 $nf$·$rvnf$를 원래 순서대로 복원합니다.

### 4.7 현재 무엇을 신뢰하나요?

현재 POC는 다음을 가정합니다.

- 위원회와 Auditor는 Contract와 Circuit이 검증해 저장한 AuditRecord만 처리합니다.
- 서로 다른 위원 두 명은 올바른 partial decryption 값을 제공합니다.
- Auditor는 온체인 원본과 응답을 올바르게 결합합니다.

악의적인 위원 응답을 판별하는 별도 proof는 구현하지 않았습니다. 원 TDH2의 인증 암호문 전체도 구현하지 않았으므로, 원 TDH2의 모든 보안 성질을 그대로 주장하지 않습니다.

### 4.8 역방향 추적은 무엇인가요?

역방향 추적은 현재 객체를 만든 AuditRecord에서 부모를 복원하고 Entry 방향으로 이동합니다.

Claim에서 시작할 때는 다음 순서를 사용합니다.

1. claimRecordOf에서 Issue AuditRecord를 찾습니다.
2. Issue 트랜잭션에서 dppCommitment를 확인합니다.
3. producerOf에서 DPP를 만든 Exit AuditRecord를 찾습니다.
4. Exit의 부모 암호문을 복호화해 부모 $cm$을 얻습니다.
5. 부모 Note의 producerOf를 따라가며 Entry까지 반복합니다.

### 4.9 정방향 추적은 무엇인가요?

정방향 추적은 현재 $cm$·$rv$가 소비될 때 사용할 $nf$·$rvnf$를 복원하고, spentIn 매핑에서 다음 Event를 찾습니다.

1. producerOf에서 현재 객체를 만든 AuditRecord를 찾습니다.
2. 그 기록의 출력 소비값 암호문을 복호화합니다.
3. 현재 출력과 같은 위치의 $nf$·$rvnf$를 선택합니다.
4. noteSpentIn 또는 voucherSpentIn에서 소비 AuditRecord를 찾습니다.
5. 소비 기록이 있으면 그 Event의 생성 객체 목록으로 이동합니다.
6. 소비 기록이 없으면 감사 기준 시점의 미소비 리프입니다.

DPP는 소비값이 없는 최종 출력입니다. DPP에는 spentIn을 조회하지 않습니다.

Auditor는 감사 시작 시 **블록 번호와 블록 해시를 감사 기준 시점(snapshot)**으로 고정합니다. 이 시점 이후 대상이 먼저 소비되면 Freeze 트랜잭션은 실패합니다. Contract는 하위 객체를 자동으로 다시 추적하거나 모든 리프를 자동 동결하지 않습니다.

## 5. Frozen·Revoked는 어떻게 동작하나요?

**Status Authority는 감사에서 복원한 $nf$·$rvnf$ 또는 Claim 키를 사용해 상태를 변경합니다.**

허용하는 상태 전이는 다음 세 가지입니다.

$$
Active\rightarrow Frozen,
\qquad
Frozen\rightarrow Active,
\qquad
Frozen\rightarrow Revoked
$$

Revoked는 최종 상태이므로 다시 Active나 Frozen으로 바꿀 수 없습니다.

| 대상 | 상태를 찾는 키 | Frozen일 때의 결과 |
|---|---|---|
| **Note** | $nf$ | Transfer·Merge·Split·Process·Exit가 실패합니다. |
| **Voucher** | $rvnf$ | Proceed·Recall이 모두 실패합니다. |
| **Claim** | dppCommitment·issuePolicyRef | Claim 조회 결과가 Frozen으로 표시됩니다. |

이미 소비된 Note·Voucher는 상태를 변경할 수 없습니다. Claim 상태는 Policy 버전별로 독립적으로 관리합니다.

상태 매핑의 기본값 Active는 해당 $nf$·$rvnf$가 실제 객체에서 파생됐다는 증거가 아닙니다. Status Authority가 감사로 복원한 올바른 값을 제출한다고 가정합니다.

**최종 프로토콜은 별도 StatusTree를 사용하지 않습니다.** 소비 트랜잭션이 공개하는 $nf$·$rvnf$를 매핑 키로 직접 조회합니다. 그 대신 동결된 nullifier와 이후 소비 트랜잭션이 공개적으로 연결되는 프라이버시 상충 관계를 허용합니다.

## 6. Entry는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Entry는 최초 비공개 Note를 원장에 추가합니다.

$$
\varnothing\rightarrow Note
$$

| 구분 | 내용 |
|---|---|
| **private witness** | 새 Note의 데이터·$sk_{\mathrm{owner}}$·암호화 난수 $r$ |
| **public input** | Note commitment $cm$·감사 암호문 |
| **성공 후 output** | Note 1개 |

### 무엇을 검증하나요?

**Circuit은 다음 관계를 검증합니다.**

1. Note의 질량이 0보다 크고 State가 유효한지 확인합니다.
2. 소유자 비밀값에서 계산한 ZK 주소와 Note 소유자가 같은지 확인합니다.
3. Note commitment와 Note가 소비될 때 사용할 $nf$를 계산합니다.
4. 실제 $nf$가 감사 암호문에 들어 있는지 확인합니다.

**Contract는 호출자가 등록된 EntryIssuer인지, $cm$이 새 값인지와 proof가 유효한지 확인합니다.**

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **commitments** | 새 $cm$을 등록합니다. |
| **Note Tree** | $cm$을 리프로 추가합니다. |
| **producerOf** | 새 Note를 Entry AuditRecord에 연결합니다. |
| **AuditRecord** | 출력 Note와 암호화된 출력 $nf$를 기록합니다. |

**실패 조건:** 등록되지 않은 EntryIssuer, 중복 $cm$, 유효하지 않은 Note State·소유자·암호문은 실패합니다. 실패하면 어떤 상태도 변경되지 않습니다.

## 7. Transfer는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Transfer는 송신자의 Note를 수신자가 받을 Voucher와 송신자에게 남는 Change Note로 나눕니다.

$$
Note_{\mathrm{input}}\rightarrow Voucher_{\mathrm{receiver}}+Note_{\mathrm{change}}
$$

| 구분 | 내용 |
|---|---|
| **private witness** | 입력 Note·$sk_{\mathrm{owner}}$·Merkle path·수신자 ZK 주소·두 출력의 데이터·배분 나머지·deadlineEpoch·암호화 난수 $r$ |
| **public input** | Note root·입력 $nf$·새 $rv$·새 Change $cm$·transferEpoch·deltaEpoch·감사 암호문 |
| **성공 후 output** | Voucher 1개·Change Note 1개 |

### State는 어떻게 나누나요?

송신자가 Voucher에 보낼 질량을 선택합니다. 이 질량은 0보다 크고 입력 질량보다 클 수 없습니다. Change 질량은 입력에서 Voucher 질량을 뺀 값입니다.

$a_{\mathrm{rec}}$과 $e$는 질량 비례로 나눕니다. Change 값을 내림 계산하고, 나머지는 Voucher에 배정합니다. 따라서 두 출력의 합은 입력 State와 정확히 같습니다.

전량 Transfer도 Change Note를 생략하지 않습니다. 이 경우 무작위 opening을 가진 질량 0 Change Note를 만듭니다.

### 무엇을 검증하나요?

**Circuit은 다음 관계를 검증합니다.**

1. Circuit이 송신자가 입력 Note의 소유자인지 확인합니다.
2. Circuit이 비공개 입력 $cm$이 공개 Note root에 포함됐는지 확인합니다.
3. Circuit이 공개 $nf$가 이 Note와 소유자 비밀값에서 계산됐는지 확인합니다.
4. Circuit이 Voucher와 Change의 질량·재활용량·탄소 배분을 확인합니다.
5. Circuit이 두 출력의 DocumentHash와 AssetRole이 입력과 같은지 확인합니다.
6. Circuit이 마감 시점과 두 출력 commitment를 확인합니다.
7. Circuit이 부모 $cm$, Voucher의 $rvnf$, Change Note의 $nf$가 감사 암호문에 들어 있는지 확인합니다.

**Contract는 수용된 Note root, 입력 $nf$의 미소비·Active 상태, 실행 epoch, 출력 중복과 proof를 확인합니다.**

**ELIGIBLE과 WASTE Note를 모두 Transfer할 수 있습니다.** 두 출력은 입력의 AssetRole을 유지합니다.

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **noteSpentIn** | 입력 $nf$에 AuditRecord ID를 기록합니다. |
| **voucherCommitments**·Voucher Tree | 새 $rv$를 등록하고 추가합니다. |
| **commitments**·Note Tree | Change $cm$을 등록하고 추가합니다. |
| **producerOf** | Voucher와 Change Note를 같은 AuditRecord에 연결합니다. |
| **AuditRecord** | 부모 $cm$과 두 출력 소비값의 암호문을 기록합니다. |

**deltaEpoch=0도 허용합니다.** 이 경우 Recall은 처음부터 불가능하지만 Proceed는 가능합니다.

**실패 조건:** 잘못된 소유자·포함 관계·State 배분·마감 시점·암호문, 소비되거나 Frozen인 $nf$, 중복 출력은 실패합니다. 실패하면 Note 소비와 두 Tree 추가가 모두 취소됩니다.

## 8. Proceed는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Proceed는 수신자가 Voucher를 같은 제품·State의 자기 Note로 바꾸는 Event입니다.

$$
Voucher\rightarrow Receiver\ Note
$$

| 구분 | 내용 |
|---|---|
| **private witness** | Voucher·수신자 $sk_{\mathrm{owner}}$·Voucher Merkle path·수신자 Note·암호화 난수 $r$ |
| **public input** | Voucher root·$rvnf$·수신자 Note $cm$·감사 암호문 |
| **성공 후 output** | 수신자 Note 1개 |

### 무엇을 검증하나요?

**Circuit**은 Voucher가 Voucher Tree에 있고, 수신자 비밀값이 Voucher의 receiverAddress와 같은 ZK 소유자를 나타내는지 확인합니다. 수신자 Note는 Voucher의 DocumentHash·AssetRole·State를 그대로 이어받아야 합니다.

**Contract**는 Voucher root가 수용된 root인지, $rvnf$가 미소비·Active인지, 출력 $cm$이 새 값인지와 proof가 유효한지 확인합니다. **Proceed는 마감 시점 전후 모두 가능합니다.**

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **voucherSpentIn** | $rvnf$에 AuditRecord ID를 기록합니다. |
| **commitments**·Note Tree | 수신자 Note $cm$을 등록하고 추가합니다. |
| **producerOf** | 수신자 Note를 AuditRecord에 연결합니다. |
| **AuditRecord** | 부모 $rv$와 수신자 Note $nf$의 암호문을 기록합니다. |

**실패 조건:** 잘못된 수신자 비밀값·Voucher 포함 관계·출력 State·암호문, 이미 해결됐거나 Frozen인 $rvnf$와 중복 출력은 실패합니다.

## 9. Recall은 어떻게 동작하나요?

### 목적과 결과

**핵심:** Recall은 송신자가 마감 시점 전에 Voucher를 송신자의 Return Note로 되돌리는 Event입니다.

$$
Voucher\rightarrow Sender\ Return\ Note
$$

| 구분 | 내용 |
|---|---|
| **private witness** | Voucher·송신자 $sk_{\mathrm{owner}}$·Voucher Merkle path·Return Note·deadlineEpoch·암호화 난수 $r$ |
| **public input** | Voucher root·$rvnf$·Return Note $cm$·currentEpoch·감사 암호문 |
| **성공 후 output** | 송신자 Return Note 1개 |

### 무엇을 검증하나요?

**Circuit**은 Voucher membership, 송신자 비밀값, $rvnf$와 Return Note를 확인합니다. Return Note는 Voucher의 DocumentHash·AssetRole·State를 그대로 이어받아야 합니다. 또한 public currentEpoch이 private deadlineEpoch보다 작아야 합니다.

**Contract**는 public currentEpoch이 트랜잭션 블록의 $\lfloor timestamp/600\rfloor$과 같은지 확인합니다. Voucher가 미소비·Active이고 proof와 출력이 유효한지도 확인합니다.

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **voucherSpentIn** | $rvnf$에 AuditRecord ID를 기록합니다. |
| **commitments**·Note Tree | Return Note $cm$을 등록하고 추가합니다. |
| **producerOf** | Return Note를 AuditRecord에 연결합니다. |
| **AuditRecord** | 부모 $rv$와 Return Note $nf$의 암호문을 기록합니다. |

**Proceed와 Recall은 같은 $rvnf$를 사용합니다.** 먼저 성공한 경로가 voucherSpentIn을 기록하므로 다른 경로는 실패합니다. 마감 시점 이후에는 Recall만 실패하며 Proceed는 계속 가능합니다.

**실패 조건:** 잘못된 송신자 비밀값·Voucher 포함 관계·마감 시점·Return Note·암호문, 이미 해결됐거나 Frozen인 $rvnf$와 중복 출력은 실패합니다.

## 10. Merge는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Merge는 같은 소유자와 AssetRole을 가진 Note 두 개를 하나로 합칩니다.

$$
Note_A+Note_B\rightarrow Note_C
$$

| 구분 | 내용 |
|---|---|
| **private witness** | Note 두 개·공통 $sk_{\mathrm{owner}}$·두 Merkle path·출력 Note·암호화 난수 $r$ |
| **public input** | Note root·입력 $nf$ 두 개·출력 $cm$·감사 암호문 |
| **성공 후 output** | 합쳐진 Note 1개 |

### 무엇을 검증하나요?

**Circuit**은 다음을 확인합니다.

1. 두 비공개 입력 $cm$이 같은 공개 Note root에 포함됩니다.
2. 두 Note는 같은 소유자 비밀값으로 소비됩니다.
3. 두 입력과 출력의 AssetRole이 같습니다.
4. 출력의 $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$는 두 입력을 더한 값이며 uint64 범위를 넘지 않습니다.
5. 부모 $cm$ 두 개와 출력 $nf$가 감사 암호문에 들어 있습니다.

**Contract**는 두 $nf$가 서로 다르고 미소비·Active인지, 출력 $cm$이 새 값인지와 proof를 확인합니다.

### 온체인에서는 무엇이 바뀌나요?

**noteSpentIn**에 입력 두 개를 소비한 AuditRecord ID를 기록합니다. 출력 $cm$ 하나를 **commitments**와 Note Tree에 추가하고 **producerOf**에 연결합니다. **AuditRecord**에는 부모 두 개와 출력 소비값의 암호문을 저장합니다.

**ELIGIBLE 두 개 또는 WASTE 두 개를 Merge할 수 있습니다.** ELIGIBLE과 WASTE를 서로 합칠 수는 없습니다. 현재 POC는 ProductProfile이나 입력·출력 DocumentHash 관계를 검증하지 않습니다.

**실패 조건:** 서로 다른 소유자·AssetRole, 잘못된 State 합·포함 관계·암호문, 같거나 이미 소비·동결된 입력과 중복 출력은 실패합니다. 실패하면 입력 하나만 소비되는 상태는 남지 않습니다.

## 11. Split은 어떻게 동작하나요?

### 목적과 결과

**핵심:** Split은 Note 하나를 같은 소유자·AssetRole의 Note 두 개로 나눕니다.

$$
Note_A\rightarrow Note_B+Note_C
$$

| 구분 | 내용 |
|---|---|
| **private witness** | 입력 Note·$sk_{\mathrm{owner}}$·Merkle path·출력 Note 두 개·배분 나머지·암호화 난수 $r$ |
| **public input** | Note root·입력 $nf$·출력 $cm$ 두 개·감사 암호문 |
| **성공 후 output** | 분할된 Note 2개 |

Participant는 두 출력의 질량을 정합니다. 두 질량의 합은 입력 질량과 같아야 합니다. Circuit은 출력 2의 $a_{\mathrm{rec}}$과 $e$를 질량 비례로 내림 계산하고, 나머지를 출력 1에 배정합니다.

### 무엇을 검증하나요?

**Circuit**은 입력 membership·소유자·$nf$, 두 출력의 소유자·AssetRole·State 합과 감사 암호문을 확인합니다. 입력 질량은 0보다 커야 합니다. 출력 하나는 질량 0일 수 있지만 두 출력이 모두 0일 수는 없습니다.

**Contract**는 입력 $nf$가 미소비·Active인지, 두 출력 $cm$이 서로 다르고 새 값인지와 proof를 확인합니다.

### 온체인에서는 무엇이 바뀌나요?

**noteSpentIn**에 입력 소비 기록을 남깁니다. 두 출력 $cm$을 **commitments**와 Note Tree에 순서대로 추가합니다. **producerOf**는 두 출력을 같은 AuditRecord에 연결합니다.

**ELIGIBLE과 WASTE를 모두 Split할 수 있습니다.** 두 출력은 입력 AssetRole을 유지합니다. 출력 DocumentHash와 입력 DocumentHash의 관계는 검증하지 않습니다.

**실패 조건:** 잘못된 소유자·포함 관계·질량 합·비례 배분·AssetRole·암호문, 소비되거나 Frozen인 입력과 중복 출력은 실패합니다. 두 출력 중 하나라도 기록할 수 없으면 트랜잭션 전체가 되돌아갑니다.

## 12. Process는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Process는 Policy Authority가 등록한 Process verifier Contract에 따라 Note 세 개를 ELIGIBLE Note와 WASTE Note로 변환합니다.

$$
Note_1+Note_2+Note_3\rightarrow ELIGIBLE\ Note+WASTE\ Note
$$

| 구분 | 내용 |
|---|---|
| **private witness** | Note 세 개·공통 $sk_{\mathrm{owner}}$·세 Merkle path·공정 중간값·배분 나머지·출력 Note 두 개·암호화 난수 $r$ |
| **public input** | policyRef·policyScopeRef·Note root·입력 $nf$ 세 개·출력 $cm$ 두 개·감사 암호문 |
| **성공 후 output** | ELIGIBLE Note 1개·WASTE Note 1개 |

Factory는 Process를 수행하는 Participant입니다. Process 사용 권한은 Factory의 EVM 계정이 아니라 policyScopeRef에 부여합니다.

1. Policy Authority가 policyRef를 공개합니다.
2. Factory가 자기 $sk_{\mathrm{owner}}$와 policyRef로 policyScopeRef를 계산합니다.
3. Factory가 policyScopeRef를 Policy Authority에 전달합니다. Policy Authority는 $sk_{\mathrm{owner}}$를 알지 못합니다.
4. Policy Authority가 이 policyScopeRef에 사용 권한을 기록합니다.
5. Process Circuit이 proof 안에서 같은 policyScopeRef를 다시 계산합니다.

### 현재 Process Policy는 무엇인가요?

현재 POC는 입력 Note 3개를 출력 Note 2개로 바꾸는 공정 하나를 구현했습니다.

| 규칙 | 값 |
|---|---:|
| 입력 | ELIGIBLE Note 3개 |
| 질량 손실 | 입력 총질량의 6.25% 내림 |
| 공정 탄소 추가 | 입력 1 kg당 0.09375 kgCO2e 내림 |
| WASTE 질량 | 중간 상태 질량의 10%를 내림 계산 |
| 출력 | ELIGIBLE Note 1개·WASTE Note 1개 |

아래 숫자는 $10^9$ 배율을 제거한 kg·kgCO2e 값입니다. 세 입력의 합이 $(320,30,230)$이면 중간 상태는 $(300,30,260)$입니다. 최종 출력은 ELIGIBLE $(270,30,260)$과 WASTE $(30,0,0)$입니다.

**WASTE에는 질량만 배정합니다.** 재활용량과 공정 후 탄소는 모두 ELIGIBLE에 배정합니다.

### 무엇을 검증하나요?

**Circuit**은 다음을 확인합니다.

1. 공개 policyRef가 Circuit에 고정된 Policy와 같습니다.
2. policyScopeRef가 Factory의 소유자 비밀값과 policyRef에서 계산됐습니다.
3. 입력 Note 세 개가 같은 소유자의 서로 다른 ELIGIBLE Note입니다.
4. 세 Note가 공개 Note root에 포함되고 각 $nf$가 올바릅니다.
5. 질량 손실·공정 탄소·WASTE 질량이 Policy 비율과 내림 규칙을 따릅니다.
6. 두 출력이 같은 소유자에게 속하고 ELIGIBLE·WASTE 규칙을 만족합니다.
7. 부모 $cm$ 세 개와 출력 $nf$ 두 개가 감사 암호문에 들어 있습니다.

**Contract**는 PolicyRecord가 활성인지, 입력 3개·출력 2개의 PROCESS Policy인지, policyScopeRef에 PolicyGrant가 있는지 확인합니다. 세 입력 $nf$의 미소비·Active 상태, 두 출력 중복과 proof도 확인합니다.

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **noteSpentIn** | 입력 $nf$ 세 개에 같은 AuditRecord ID를 기록합니다. |
| **commitments**·Note Tree | ELIGIBLE·WASTE 출력 두 개를 추가합니다. |
| **producerOf** | 두 출력을 같은 Process AuditRecord에 연결합니다. |
| **AuditRecord** | 부모 세 개와 출력 소비값 두 개의 암호문을 저장합니다. |

현재 POC는 다른 입력·출력 개수, 비선형 공정, ProductProfile, 입력과 출력의 DocumentHash 관계를 검증하지 않습니다.

**실패 조건:** 잘못된 Policy·policyScopeRef·사용 권한·소유자·포함 관계·공정 계산·AssetRole·암호문, 소비되거나 Frozen인 입력과 중복 출력은 실패합니다. 실패하면 세 입력이나 두 출력의 일부만 기록되는 상태는 남지 않습니다.

## 13. Exit는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Exit는 공급망 Note를 소비하고 공개 DPP commitment를 등록합니다.

| 구분 | 내용 |
|---|---|
| **private witness** | Note·$sk_{\mathrm{owner}}$·Merkle path·DPPPrivateData·암호화 난수 $r$ |
| **public input** | Note root·$nf$·dppCommitment·부모 $cm$의 감사 암호문 |
| **성공 후 output** | DPP commitment 1개 |

### 무엇을 검증하나요?

**Circuit**은 입력 Note의 membership·소유자·$nf$를 확인합니다. DPPPrivateData의 DocumentHash·AssetRole·State는 Note와 같아야 합니다. Circuit은 dppCommitment와 실제 부모 $cm$의 감사 암호문도 확인합니다.

**Contract**는 Note root, $nf$의 미소비·Active 상태, dppCommitment 중복과 proof를 확인합니다.

### 온체인에서는 무엇이 바뀌나요?

| 상태 | 변경 내용 |
|---|---|
| **noteSpentIn** | 입력 $nf$에 Exit AuditRecord ID를 기록합니다. |
| **producerOf** | dppCommitment를 Exit AuditRecord에 연결합니다. |
| **AuditRecord** | DPP 출력과 부모 $cm$의 암호문을 기록합니다. |
| Note·Voucher Tree | 변경하지 않습니다. |

**ELIGIBLE과 WASTE 모두 Exit할 수 있습니다.** DPP는 소비하지 않으므로 DPP Tree와 nullifier를 만들지 않습니다.

**실패 조건:** 잘못된 소유자·포함 관계·Note와 DPP의 대응 관계·암호문, 소비되거나 Frozen인 Note와 중복 dppCommitment는 실패합니다.

## 14. Issue는 어떻게 동작하나요?

### 목적과 결과

**핵심:** Issue는 DPP의 비공개 State가 Issue Policy를 만족한다는 Claim을 등록합니다. DPP commitment 자체는 변경하지 않습니다.

| 구분 | 내용 |
|---|---|
| **private witness** | DocumentHash·AssetRole·State·dppOpening |
| **public input** | issuePolicyRef·dppCommitment |
| **성공 후 output** | **claimRecordOf**에 Claim 기록 1개 |

### 현재 Issue Policy는 무엇인가요?

| Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

두 Policy는 ELIGIBLE DPP만 허용합니다.

### 무엇을 검증하나요?

**Circuit**은 private witness에서 dppCommitment를 다시 계산합니다. 그다음 재활용률과 탄소집약도가 Circuit에 고정된 Policy 경계를 만족하는지 확인합니다.

**Contract**는 다음을 확인합니다.

1. dppCommitment가 Exit에서 생성됐습니다.
2. Issue PolicyRecord가 존재하고 활성입니다.
3. Policy 종류가 ISSUE이고 입력·출력 개수가 각각 1개입니다.
4. 같은 DPP·issuePolicyRef Claim이 아직 없습니다.
5. PolicyRecord에 등록된 verifier Contract에서 proof가 성공합니다.

### 온체인에서는 무엇이 바뀌나요?

claimRecordOf에 Issue AuditRecord ID를 기록합니다. Claim 상태는 별도로 저장하지 않아도 매핑 기본값 0을 Active로 해석합니다. Issue AuditRecord에는 eventKind와 policyRef만 기록하고 암호문과 outputRefs는 남기지 않습니다.

**Issue는 Note를 다시 소비하지 않습니다.** 소유자 비밀값·Merkle 포함 관계·nullifier·Process 사용 권한도 확인하지 않습니다. DPPPrivateData와 dppOpening을 아는 사람이 proof를 만들 수 있으며, 현재 POC는 그 사람이 현재 DPP 소유자인지 확인하지 않습니다.

**실패 조건:** WASTE DPP, Policy 경계를 만족하지 못하는 DPP, 비활성 Policy와 중복 Claim은 실패합니다.

## 15. 전체 공급망은 어떻게 이어지나요?

M9에서는 다음 네 원자재로 전체 흐름을 실행했습니다. 표의 숫자는 $10^9$ 배율을 제거한 kg·kgCO2e 값입니다.

| 원자재 | 초기 소유자 | State $(q,a,e)$ |
|---|---|---|
| Aluminum A | actor-1 | $(60,10,40)$ |
| Aluminum B | actor-1 | $(60,10,50)$ |
| Cathode | actor-3 | $(100,10,70)$ |
| Anode | actor-3 | $(100,0,70)$ |

실행 흐름은 다음과 같습니다.

1. 네 원자재를 Entry했습니다.
2. Aluminum A를 Transfer해 Voucher를 만든 뒤 Freeze했습니다. Frozen 상태에서 Proceed·Recall이 모두 실패했습니다.
3. Voucher를 Active로 되돌린 뒤 Recall하고, 다시 Transfer·Proceed했습니다.
4. Aluminum B·Cathode·Anode도 Factory에게 Transfer·Proceed했습니다.
5. Factory가 Aluminum A·B를 Merge했습니다.
6. Merge 출력·Cathode·Anode를 Process했습니다.
7. Process는 ELIGIBLE $(270,30,260)$과 WASTE $(30,0,0)$을 만들었습니다.
8. ELIGIBLE 출력을 Product 1과 Product 2로 Split했습니다.
9. Product 1을 Exit해 DPP commitment를 만들었습니다.
10. Product 1 DPP에 Standard V1과 Strict V2 Claim을 등록했습니다.
11. WASTE도 Exit해 DPP commitment를 만들었지만 Issue proof는 만들 수 없었습니다.
12. Standard Claim은 Active, Strict Claim은 Revoked 상태로 끝났습니다.
13. Claim에서 네 Entry까지 역방향 추적했습니다.
14. Aluminum A에서 Product DPP와 미소비 Note까지 정방향 추적했습니다.

최종 온체인 상태에는 Note 리프 19개, Voucher 리프 5개와 AuditRecord 21개가 남았습니다. Product 2는 미소비 Active Note이고, WASTE DPP에는 Claim이 없습니다.

## 16. 실제로 구현한 범위와 남은 한계는 무엇인가요?

| 영역 | 실제 구현 | 현재 구현하지 않은 것 |
|---|---|---|
| Note | 비공개 포함 관계·소유자·nullifier 소비 | EVM 호출자와 ZK 소유자의 연결 검증 |
| Transfer | Voucher·Change Note와 Proceed·Recall 경쟁 | Voucher opening 전달 암호화 |
| Merge | 같은 소유자·AssetRole Note 결합 | ProductProfile 호환성 검증 |
| Split | 질량 비례 내림·나머지 배분 | 다른 반올림 Policy |
| Process | 입력 3개·출력 2개로 고정한 Policy 공정 | 다른 입력·출력 개수·비선형 공정 |
| 감사 | 2-of-3 복호화·양방향 추적 | 악의적인 위원 응답 proof·운영 서버 |
| 상태 관리 | $nf$·$rvnf$·Claim 매핑 집행 | StatusTree·하위 객체 자동 동결 |
| DPP·Claim | Exit DPP와 Standard·Strict Claim | DPP 소유권 이전·중간 Claim |
| 배포 | 최종 메인 Contract·verifier Contract 10개·개발용 범용 SRS | 운영용 다자간 신뢰 설정식(Ceremony)·Besu·별도 Router |

이 POC는 구현 가능성·정확성·대표 성능을 확인합니다. 실제 운영 거버넌스, 규제상 DPP 전체와 안전한 신뢰 설정이 완료됐다는 의미는 아닙니다.

## 17. 구현을 수정하려면 무엇을 읽나요?

평소에는 이 안내서만 읽으면 됩니다.

구현을 변경할 때는 다음 순서로 이동합니다.

1. 이 안내서에서 바꾸려는 Event의 목적과 프로토콜 의미를 확인합니다.
2. [프로토콜 상세 명세](zkDPP-protocol-reference.md)에서 공개 입력·비공개 입력·Circuit·Contract 저장 상태를 확인합니다.
3. [프로토콜 YAML](zkDPP-protocol.yaml)에서 정확한 이름·순서·구현 위치를 확인합니다.
4. 실제 Circuit·Contract 코드를 수정합니다.
5. Milestone 명세와 Result를 갱신합니다.

사람용 안내서와 YAML의 의미가 다르면 한쪽을 임의로 정답으로 선택하지 않습니다. 실제 코드와 최신 동결 명세를 대조해 불일치를 먼저 해결합니다.
