# M1 Background — 왜 Master Key 기반 감사로 변경하나요?

- 상태: 구현 기준 동결
- 구현 명세: [M1 Master Key Audit](M1-master-key-audit.md)
- 이전 기준: [`zkDPP-poc-v1` M9 Result](../../zkDPP-poc-v1/milestones/M9-final-integration-result.md)
- 원본 논의: [`zkdpp-v2-protocol.md`](../../Conversation%20History/zkdpp-v2-protocol.md), [`zkdpp-v2-implementation-changes.md`](../../Conversation%20History/zkdpp-v2-implementation-changes.md)

## 0. 30초 안에 무엇을 기억하면 되나요?

**M1은 Committee와 AuditRecord마다 상호작용하던 v1 M9을, 승인된 감사에서 한 key 기간의 Master Key를 복구해 전체 기록을 읽는 구조로 바꿉니다.** 동시에 Exit·Issue·Claim lifecycle과 v2 회계·식별자 규칙을 적용하고, Forward Tracing 결과 전체를 실제 Freeze와 공통 결과 확인까지 연결합니다.

```text
기존 M9:
  AuditRecord마다 Committee partial decryption

V2-M1:
  외부 DKG 결과로 가정한 2-of-3 share
  → Status Authority가 Master Key 복구
  → 해당 key 기간 전체 AuditRecord 복호화
  → Forward Tracing
  → 전체 frontier Freeze
  → 공통 블록에서 결과 확인
```

| 질문 | 결정 |
|---|---|
| Auditor와 Status Authority는 다른 주체인가요? | 아닙니다. 같은 주체의 감사 기능과 온체인 집행 기능입니다. |
| DKG를 구현하나요? | 아닙니다. 외부 DKG output을 고정 fixture로 재현합니다. |
| key를 몇 계층 사용하나요? | 한 단계 Audit key $PK_A,SK_A$만 사용합니다. |
| AuditRecord마다 Committee를 다시 호출하나요? | M1에서는 아닙니다. 두 share로 $SK_A$를 한 번 복구합니다. |
| 같은 마스크를 모든 Field에 사용하나요? | 아닙니다. Record별 $K_i$, 위치별 $k_{i,j}$를 사용합니다. |
| $L,n$을 사용하나요? | 사용하지 않습니다. Event Circuit이 문맥·순서·길이를 고정합니다. |
| AuditAndFreeze는 온체인 함수인가요? | 아닙니다. 복호화·추적·개별 Freeze·결과 확인을 연결한 off-chain workflow입니다. |
| post-snapshot 자손을 자동 추적하나요? | 아닙니다. M1에서는 `INCOMPLETE`로 끝냅니다. |
| Key Rotation을 구현하나요? | 아닙니다. M3에서 다룹니다. |

## 1. 어떤 문제에서 시작했나요?

### [기존 M9] Record별 복호화

v1 M9에서는 AuditRecord마다 서로 다른 공개점 $R_i$가 있습니다. Committee member $u$는 자기 share $sk_u$로 다음 partial decryption을 계산합니다.

$$
D_{u,i}=sk_uR_i
$$

Status Authority는 Record마다 서로 다른 두 위원의 응답을 결합합니다. 이 방식은 Committee가 어떤 Record를 열었는지 알 수 있고, Status Authority는 승인받은 Record 외의 복호화 능력을 얻지 않는다는 장점이 있습니다.

반면 graph가 길어지면 Committee interaction이 Record 수에 비례합니다. 100개 Record를 복호화하려면 두 위원이 각각 100개의 partial decryption을 만들어야 합니다. Audit은 자주 실행하지 않더라도 실제 downstream이 길면 latency와 운영 복잡도가 커집니다.

### [이번 결정] key 기간 전체를 여는 Master Key

Committee가 외부 DKG로 $SK_A$의 share를 분배받았다고 가정합니다. 감사 승인 시 서로 다른 두 위원이 자기 secret share를 Status Authority에게 전달하고, Status Authority는 Lagrange interpolation으로 $SK_A$를 복구합니다.

$$
SK_A=sum_{u\in Q}\lambda_u sk_u\pmod q,
\qquad |Q|=2
$$

복구한 scalar가 공개키와 일치하는지 확인합니다.

$$
PK_A\stackrel{?}=SK_AG
$$

그 뒤에는 Committee와 다시 통신하지 않고 해당 key로 암호화된 모든 AuditRecord를 로컬에서 복호화합니다.

### 왜 이 선택이 허용되나요?

이번 신뢰 모델은 Status Authority가 승인된 감사 범위 안에서 올바르게 행동한다고 가정합니다. 대신 한 번 $SK_A$를 얻으면 해당 key 기간의 모든 기록을 읽을 수 있다는 Privacy 비용을 명시적으로 받아들입니다.

이 구조가 Committee 동의를 암호학적으로 강제하려면 실제 key를 Auditor가 생성하면 안 됩니다. M1은 DKG를 구현하지 않으므로 이를 증명하지 않고, **외부 DKG가 올바르게 분배한 output을 fixture로 받았다**고만 가정합니다. 실제 dealerless 생성은 M2가 검증합니다.

## 2. 한 단계 key를 선택한 이유는 무엇인가요?

### [폐기] Threshold key가 별도 Audit key를 감싸는 구조

검토한 두 단계 구조는 다음과 같습니다.

```text
고정 Committee Threshold key PK_T
  → Enc(PK_T, SK_Audit)
  → AuditRecord는 PK_Audit로 암호화
```

이 구조는 Committee의 장기 share를 유지할 수 있지만, 어떤 단일 주체도 $SK_{Audit}$을 모르게 생성하고 감싸려면 DKG 외에 분산 key wrapping 또는 MPC가 필요합니다. 현재 POC에는 불필요한 두 번째 key lifecycle과 registry가 생깁니다.

### [이번 결정] Audit key 자체를 Threshold share로 분배

M1은 하나의 key pair만 사용합니다.

$$
PK_A=SK_AG
$$

Participant는 $PK_A$로 Record를 암호화하고, Committee는 $SK_A$의 share를 보관합니다. 감사 전에는 한 위원이나 Status Authority가 $SK_A$를 알지 못하고, 감사 승인 뒤 두 share를 결합하면 해당 key 기간 전체를 열 수 있습니다.

M3에서는 key 기간마다 새로운 DKG key를 사용합니다. 이전 key를 공개하기 전에 다음 key를 활성화해 과거 key가 미래 Record를 열지 못하게 합니다.

## 3. 왜 Record마다 새로운 $K_i$가 필요한가요?

AuditRecord $i$마다 Participant가 새로운 nonzero scalar $r_i$를 고릅니다.

$$
R_i=r_iG,
\qquad
Z_i=r_iPK_A
$$

Status Authority는 $SK_A$로 같은 공유점을 계산합니다.

$$
Z_i=SK_AR_i
$$

Record별 masking key는 공유점 좌표에서 파생합니다.

$$
K_i=H_{\mathrm{key}}(Z_{i,X},Z_{i,Y})
$$

새로운 $r_i$를 사용하면 Record마다 $R_i,Z_i,K_i$가 달라집니다. 같은 $r_i$를 재사용하면 같은 위치의 mask도 재사용되므로 평문 관계가 노출될 수 있습니다. Circuit은 $r_i\ne0$과 암호화 관계를 확인하지만 난수의 진짜 무작위성과 전역 재사용 여부를 증명하지는 않습니다. 안전한 random source 사용을 Participant의 구현 책임으로 둡니다.

## 4. 왜 위치별 마스크가 필요한가요?

하나의 Record가 두 평문 $M_0,M_1$을 가진다고 하겠습니다. 같은 $K_i$를 두 값에 직접 더하면 다음과 같습니다.

$$
C_0=M_0+K_i
$$

$$
C_1=M_1+K_i
$$

누구나 두 암호문을 빼서 마스크를 제거할 수 있습니다.

$$
C_0-C_1=M_0-M_1
$$

평문 하나를 알고 있다면 $K_i=C_0-M_0$을 계산해 나머지 값도 읽을 수 있습니다. 따라서 $K_i$는 Record의 기준값으로만 사용하고 위치 $j$마다 다른 마스크를 파생합니다.

$$
k_{i,j}=H_{\mathrm{mask}}(K_i,j)
$$

$$
C_{i,j}=M_{i,j}+k_{i,j}\pmod p
$$

비밀키가 여러 개 생기는 것은 아닙니다. 장기 비밀은 $SK_A$ 하나이며, $K_i$와 $k_{i,j}$는 공개 $R_i$, Record 위치와 함께 자동으로 재계산하는 일시 값입니다.

## 5. 왜 $L,n$을 제거하나요?

### [기존 M9]

v1 M9은 다음처럼 Audit Context $L$과 평문 길이 $n$을 masking key에 넣었습니다.

$$
K=H(Z_X,Z_Y,L,n)
$$

두 값은 M6-B1 구현 과정에서 AI가 추가 binding을 위해 제안한 보조값이며 Threshold DH 복호화 자체에 필수적이지 않습니다.

### [이번 결정]

v2에서는 다음으로 단순화합니다.

$$
K_i=H_{\mathrm{key}}(Z_{i,X},Z_{i,Y})
$$

- Event 종류는 고정 verifier 또는 Policy verifier 선택으로 결합됩니다.
- public input은 proof statement 자체에 결합됩니다.
- 실제 부모·미래 소비값은 Event Circuit이 private 객체에서 다시 계산합니다.
- 평문 순서와 길이는 각 Event Circuit의 고정 shape입니다.

따라서 $L,n$을 다시 넣어도 핵심 관계가 새로 생기지 않습니다. M1의 Circuit·Artifact는 처음부터 두 값 없이 생성합니다.

## 6. Exit·Issue·Claim은 왜 바뀌나요?

### [기존 M9]

```text
Note → Exit → 온체인 DPP commitment → Issue Claim
```

Exit가 DPP commitment를 만들고, Issue는 그 DPP에 여러 Policy Claim을 붙였습니다. Claim은 별도 Status도 가졌습니다.

### [이번 결정]

DPP는 온체인 객체가 아니라 외부 Product와 그 Passport 문서입니다. 공급망의 final Note는 두 방식 중 하나로 종료합니다.

```text
Exit:  Note → 없음
Issue: Note → ClaimRef(h)
```

Exit는 successor 없는 종료입니다. Issue는 final ELIGIBLE Note를 소비하고, 외부 Product DPP에 첨부할 Claim handle을 생성합니다.

$$
h=H_{\mathrm{issue}}(DocumentHash,issuePolicyRef,claimNonce)
$$

$$
DPPClaim=(issuePolicyRef,h,claimNonce)
$$

Claim은 terminal이며 소비·소유권 이전·Status를 갖지 않습니다. 한 Note는 한 번만 소비할 수 있으므로 같은 Note로 nonce나 Policy만 바꿔 재발급할 수 없습니다.

## 7. 문서·회계 규칙은 왜 함께 바뀌나요?

### [이번 결정] DocumentInfo

외부 Product DPP와 Claim을 연결하려면 ProductName·LotID뿐 아니라 Unit도 DocumentHash에 포함합니다.

$$
DocumentInfo=(ProductName,LotID,Unit)
$$

Unit은 문서 필드이고 State의 정수 scale을 대신하지 않습니다.

### [이번 결정] Merge·Split

Merge와 Split은 같은 DocumentHash의 물량을 합치고 나누는 Event로 제한합니다. 서로 다른 DocumentHash를 변환하는 작업은 Process가 담당합니다.

### [이번 결정] 운송 탄소

Transfer는 기존 탄소를 질량 비례로 나눈 뒤 Voucher 쪽에 private 운송 탄소를 더합니다. 실제 운송량·배출량의 진실성은 외부 입력과 운영 검토의 책임입니다.

### [이번 결정] WASTE

Process는 WASTE를 생성할 수 있지만 WASTE의 소비 경로는 Exit뿐입니다. WASTE도 graph·frontier·Freeze 대상에서는 제외하지 않습니다.

## 8. $D$는 무엇이고 왜 Epoch을 제거하나요?

$D$는 Recall을 허용하는 마지막 절대 block number입니다.

$$
Transfer:\quad block.number<D
$$

$$
Recall:\quad block.number\le D
$$

v1의 `block.timestamp/600`, `transferEpoch`, `deltaEpoch`, `currentEpoch`을 제거합니다. 실제 시간과 블록 수를 혼동하지 않고 Contract가 현재 `block.number`를 직접 확인합니다. Proceed는 $D$ 이후에도 가능합니다.

## 9. AuditAndFreeze는 무엇을 추가하나요?

### [기존 M9]

Forward Tracing은 미소비 leaf를 반환하고 Status Authority가 별도로 대상을 Freeze했습니다. 하나의 실행에서 전체 frontier를 빠짐없이 처리했는지 공통 시점으로 판정하지 않았습니다.

### [이번 결정]

AuditAndFreeze는 다음을 연결한 off-chain 업무 절차입니다.

```text
Master Key 복구
  → 고정 Snapshot에서 Forward Tracing
  → 전체 frontier 확정
  → 개별 Freeze transaction
  → 공통 결과 Snapshot에서 전체 재확인
```

Contract가 graph를 읽거나 Master Key를 받지 않습니다. Status Authority가 기존 Status 함수를 대상별로 호출합니다. Trace가 완성되지 않으면 일부 결과만으로 Freeze를 시작하지 않습니다.

결과는 다음을 구분합니다.

- `TRACE_FAILED`
- `NO_LIVE_TARGETS`
- `COMPLETE_AT_CHECKPOINT`
- `INCOMPLETE`

## 10. Snapshot은 무엇을 고정하나요?

감사 Snapshot은 블록 번호와 해당 블록 Hash의 쌍입니다.

$$
H_t=(blockNumber_t,blockHash_t)
$$

모든 producer·spent·AuditRecord·transaction·receipt 조회는 $H_t$에 고정합니다. 같은 block number의 Hash가 바뀌면 Snapshot을 실패시킵니다.

Freeze 뒤에는 별도의 공통 결과 Snapshot을 사용합니다.

$$
H_r=(blockNumber_r,blockHash_r)
$$

여러 시점의 성공을 합쳐 전체 동시 차단처럼 보고하지 않습니다.

## 11. 무엇을 신뢰하고 무엇을 보장하지 않나요?

### 신뢰 가정

- 외부 DKG가 올바른 2-of-3 Key Package를 생성했습니다.
- Committee member는 자기 share를 승인된 Status Authority에게만 전달합니다.
- Status Authority는 승인 범위에서 올바르게 감사·집행합니다.
- Status Authority는 세션 종료 후 Master Key·share·불필요한 원문을 폐기합니다.
- Entry Issuer와 Policy Authority는 기존 책임을 올바르게 수행합니다.

### 보장하지 않는 것

- M1 fixture가 dealerless DKG를 구현했다는 주장
- Master Key와 이미 본 평문의 삭제를 암호학적으로 증명
- Active key 공개 뒤에도 미래 Record Privacy가 유지된다는 주장
- post-snapshot descendant의 자동·원자적 차단
- 실제 물리량·탄소·문서 내용의 진실성
- 운영 Committee network와 share 보관 안전성

## 12. 후속 Milestone은 무엇을 해결하나요?

### M2

Jubjub 2-of-3 DKG를 로컬 Committee 프로세스로 구현하고 M1 fixture를 실제 output으로 교체합니다. Coinbase cb-mpc는 구조·검증 참고자료일 뿐 Curve·runtime을 직접 사용하지 않습니다.

### M3

새 key를 먼저 활성화한 뒤 이전 key를 release하는 Rotation을 구현합니다. AuditRecord에 key ID를 넣고 registry와 Circuit public key binding을 추가합니다. M1에서 선택한 fixed public-key Circuit은 의도적으로 다시 Setup합니다.

### [보류] post-snapshot fallback

M9의 Record별 partial decryption을 이용하면 Active key 전체를 공개하지 않고 경합 consumer 한 건만 따라갈 수 있습니다. 이는 가능한 보완책이지만 M1 구현·측정에 포함하지 않습니다.
