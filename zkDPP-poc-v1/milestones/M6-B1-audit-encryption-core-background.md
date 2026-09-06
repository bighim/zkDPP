# M6-B1 감사 암호화 코어 — 왜 이 구조인가요?

**이번 목표는 재사용 가능한 감사 암호화 코어를 만들고, 실제 Event의 올바른 정보를 암호화했음을 증명하며, 위원 두 명의 협조로 정확히 복원할 수 있는지 확인하는 것입니다.**

- 문서 성격: 구현 전 의사결정 배경입니다. 구현 결과가 아닙니다.
- 구현 기준: [M6-B1 구현 명세](M6-B1-audit-encryption-core.md)
- 이전 결과: [M6 Status 집행 Result](M6-status-enforcement-result.md)
- 작성 원칙: **YAGNI가 최우선입니다.** 구현 가능성·정확성·성능 확인에 필요한 기능만 다룹니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

| 질문 | 이번 결정 |
|---|---|
| 어디에서 출발했나요? | M6는 별도 StatusTree로 private 객체의 Active 상태를 검증했습니다. |
| 전체 설계 변경의 목적은 무엇인가요? | 감사 때 소비 nullifier를 복원해 downstream을 찾고, 해당 nullifier로 상태를 집행하는 것입니다. |
| 이번 범위는 어디까지인가요? | 암호화·암호화 관계의 Circuit 검증·2-of-3 복호화 코어를 검증합니다. |
| 대표 사례는 무엇인가요? | 기존 M5의 3→2 Process입니다. |
| 어떤 정보를 숨기나요? | 실제 부모 참조 세 개와 출력의 소비 nullifier 두 개입니다. |
| 암호화 단위는 무엇인가요? | transaction당 메시지 하나·키 하나·암호문 하나입니다. |
| 누구를 신뢰하나요? | Setup 생성자·위원회·Auditor를 신뢰합니다. Participant의 입력은 Circuit으로 검증합니다. |
| 무엇은 아직 하지 않나요? | 전체 AuditRecord 원장, 모든 Event 적용, 그래프 탐색, Freeze·Revoke 전환입니다. |

**M6는 보존된 StatusTree baseline이고, M6-B1은 그 이후 설계 변경을 준비하는 첫 코어 검증 단계입니다.** M6-B1 완료가 곧 새로운 Main Protocol이나 전방 추적 전체의 완료를 뜻하지 않습니다.

## 1. 어떤 문제에서 출발했나요?

**소비되는 commitment는 숨기지만, 감사할 때는 그 객체가 이후 어디에서 사용됐는지 알아야 합니다.**

Note를 생성하면 공개 commitment $cm$이 등록됩니다. 소비할 때는 실제 $cm$을 private witness로 사용하고, 대신 nullifier $nf$를 공개합니다.

$$
nf=H(\mathrm{NullifierTag},sk_{\mathrm{owner}},cm)
$$

소유자는 Note를 만들 때 이미 $cm$과 자기 secret을 알고 있으므로, 그 Note가 나중에 소비될 때 사용할 $nf$를 계산할 수 있습니다. 다음 Event가 Transfer인지 Process인지 예측하는 작업이 아닙니다.

이 값을 평문으로 미리 공개하면 생성과 소비가 연결됩니다. 그래서 **소비 nullifier를 암호화해 두고, 승인된 감사에서만 복원하는 방향**을 선택했습니다.

복원한 $nf$로 소비 기록을 찾는 mapping과 상태 집행은 후속 통합 작업입니다. M6-B1에서는 먼저 그 연결의 기반인 암호화를 정확히 검증합니다.

## 2. 무엇을 한 번에 암호화하나요?

**부모 참조와 출력 소비 nullifier를 함께 숨깁니다. 이미 공개된 출력 commitment는 중복해서 암호화하지 않습니다.**

대표 Process는 다음입니다.

$$
(cm_A,cm_B,cm_C)
\xrightarrow{\mathrm{Process}}
(cm_D,cm_E)
$$

| 정보 | 의미 | 처리 |
|---|---|---|
| $cm_A,cm_B,cm_C$ | 실제 소비한 부모 Note | 암호화합니다. |
| $nf_D,nf_E$ | 새 출력이 나중에 사용할 소비 nullifier | 암호화합니다. |
| $cm_D,cm_E$ | 공개된 출력 Note commitment | 기존 공개 입력으로 유지합니다. |
| $nf_A,nf_B,nf_C$ | 이번 transaction에서 소비한 입력 nullifier | 기존 공개 입력으로 유지합니다. |

따라서 평문은 다음 다섯 Field입니다.

$$
\mathbf M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

**부모 $cm$이 private이라는 것은 Contract와 일반 관찰자에게 숨긴다는 뜻입니다.** Participant는 Note와 membership witness를 알고 있으므로 그 실제 부모를 암호화할 수 있습니다.

Process라는 공개 Event 구조에서 부모가 Note 세 개, 출력이 Note 두 개라는 것을 알 수 있습니다. 따라서 이 사례에서는 각 값에 type·개수를 반복해서 붙이지 않습니다. 부모 순서는 입력 nullifier 순서에, 출력 소비값 순서는 공개 output commitment 순서에 대응합니다.

### 같은 키를 쓰면 전체가 함께 공개되나요?

**네. 감사 승인 단위를 transaction으로 정했습니다.**

하나의 키를 복원하면 해당 transaction의 부모와 출력 소비값을 함께 읽을 수 있습니다. $nf_D$만 공개하고 같은 묶음의 $nf_E$는 계속 숨기는 별도 권한 체계는 만들지 않습니다.

이는 의도한 공개 범위입니다. 후속 감사에서는 복호화한 transaction을 저장해 두고 재사용할 수 있지만, 그 캐시와 그래프 탐색 자체는 이번 범위가 아닙니다.

## 3. 키 하나와 여러 마스크는 무엇이 다른가요?

**암호화는 한 번이지만, 같은 숫자를 모든 원소에 더하는 것은 아닙니다.**

전체 동작을 다음처럼 표현합니다.

$$
\mathbf C=\operatorname{Encrypt}(K,\mathbf M)
$$

내부에서는 같은 키 $K$와 위치 $j$로 각 위치의 마스크 $k_j$를 파생합니다.

$$
k_j=H(\mathrm{AuditMaskTag},K,j)
$$

$$
C_j=M_j+k_j\pmod p
$$

$p$는 메시지 Field의 크기입니다. 같은 마스크를 재현한 뒤 빼면 원문이 돌아옵니다.

$$
M_j=C_j-k_j\pmod p
$$

작은 숫자 예시에서 $p=101$, $M_j=80$, $k_j=30$이면 암호문은 9이고, 복호화는 $(9-30)\bmod101=80$입니다. 실제 보안 파라미터가 아니라 정보가 손실되지 않는다는 계산 예시입니다.

### 왜 모든 값에 같은 마스크를 더하지 않나요?

같은 마스크 $k$를 반복하면 다음 관계가 공개됩니다.

$$
C_1-C_2=(M_1+k)-(M_2+k)=M_1-M_2
$$

따라서 **키 하나에서 위치별 마스크를 파생**합니다. 위치는 공개여도 되지만 키는 비밀이어야 합니다. 마스크를 독립 난수로 뽑고 복원 정보를 남기지 않으면 Auditor가 이를 재현할 수 없습니다.

한 번의 함수 호출과 원소별 계산은 서로 다른 암호 방식이 아닙니다. 동일한 벡터 암호화의 바깥 표현과 내부 구현입니다. 이미 하나의 키를 공유하는 설계끼리 비교하면서 위원 응답이 열 개에서 두 개로 줄어든다고 설명하지 않습니다.

### 기존 XOR와는 무엇이 달라졌나요?

원래 TDH2는 Hash로 만든 비트열과 메시지를 XOR합니다. 우리는 이미 Field 값인 $cm,nf$를 그대로 두고 Field 마스킹을 사용합니다.

- 여러 값을 하나의 Field로 압축하지 않습니다. 다섯 Field는 다섯 암호화된 Field로 남습니다.
- 메시지 마스킹을 위한 비트 XOR는 없어집니다.
- 키와 마스크를 만드는 Hash, 타원곡선 계산, scalar 범위 검증은 남습니다.
- 실제 constraints·시간·저장 gas 개선은 측정 전에는 단정하지 않습니다.

[Blanksquare의 대칭암호 설명](https://docs.blanksquare.io/protocol-details/cryptography/snark-friendly-symmetric-encryption)은 Field 마스크 파생과 덧셈·뺄셈 접근의 참고 자료입니다. 이 문서가 우리 Threshold 구성 전체의 보안을 증명하는 것은 아닙니다.

## 4. 왜 Jubjub과 기존 Poseidon2를 사용하나요?

**proof를 만드는 곡선과, proof로 검증할 암호화 계산의 곡선을 구분합니다.**

| 역할 | 선택 |
|---|---|
| PLONK·KZG proof 생성과 검증 | 기존 BLS12-381 |
| TDH 점 연산 | Jubjub의 소수 차수 subgroup |
| 대칭키·마스크·문맥 Hash | 기존 Poseidon2 |

### Base·Scalar·Circuit Field는 어떻게 연결되나요?

Base Field는 곡선점의 좌표를 계산하는 숫자 체계입니다. Scalar Field는 점에 곱하는 숫자를 계산하는 체계입니다. Circuit Field는 constraints를 표현하는 기본 숫자 체계입니다.

우리 구성에서는 다음이 같습니다.

**BLS12-381의 Scalar Field = 현재 Circuit Field = Jubjub의 Base Field**

그러므로 Jubjub 좌표 계산을 Circuit의 기본 Field 연산으로 표현할 수 있습니다. 다른 Field의 좌표 계산을 여러 조각으로 흉내 내는 부담을 피하는 방향입니다. 다만 Jubjub scalar 범위까지 같은 것은 아닙니다.

[gnark의 곡선 설명](https://github.com/Consensys/gnark/blob/v0.15.0/std/algebra/native/twistededwards/doc.go)과 [Jubjub 정의](https://github.com/zkcrypto/jubjub#curve-description)가 이 관계를 설명합니다.

### Native와 Circuit에 모두 Jubjub이 필요한가요?

**네. 일반 Go 계산은 암호문을 만들고 복호화하며, Circuit은 같은 암호화가 올바르게 수행됐음을 증명합니다.**

온체인 verifier가 Jubjub 암호화를 다시 실행하지는 않습니다. 온체인에서는 기존처럼 BLS12-381 기반 PLONK proof를 검증합니다.

Poseidon2의 기본 파라미터도 새로 선택하지 않습니다. [기존 Native Hash](../internal/core/hash/poseidon.go)와 [Circuit Hash](../internal/circuitutil/gadgets.go)의 동일한 구성을 재사용하고, 새 용도에 맞는 Domain만 구분합니다.

## 5. 원래 TDH2에서 무엇을 남기고 무엇을 제외하나요?

**일반적인 복호화 서비스를 그대로 만드는 것이 아니라, 등록 검증을 통과한 암호문만 읽는 감사 기능을 만듭니다.**

원래 TDH2와 [Coinbase 구현](https://github.com/coinbase/cb-mpc/blob/master/src/cbmpc/crypto/tdh2.cpp)은 암호문과 partial decryption의 독립적인 검증을 포함합니다. 원 논문은 요청자가 선택한 암호문을 복호화 서비스에 제출할 수 있는 환경을 분석합니다. [TDH2 원 논문](https://www.shoup.net/papers/thresh1.pdf)

우리의 가정은 다음입니다.

| 주체 | 가정과 책임 |
|---|---|
| Participant | 신뢰하지 않습니다. 실제 부모·출력 nullifier·암호화 관계를 Circuit으로 검증합니다. |
| Setup 생성자 | 올바른 키를 생성·분배하고 master secret을 영속 저장하지 않는다고 신뢰합니다. |
| 위원회 | 승인된 기록의 원본만 읽고 올바른 partial decryption을 제공합니다. |
| Auditor | 올바른 기록을 요청하고 응답을 정상적으로 결합합니다. |
| 등록 경로 | 성공한 proof의 공개값과 실제 기록한 값이 정확히 같습니다. |

| 수식·구성 | 원래 역할 | M6-B1 판단 |
|---|---|---|
| $PK=xG$와 2-of-3 shares | 한 위원만으로 복호화하지 못하게 합니다. | 유지합니다. |
| $R_1=rG,\ Z=rPK$ | 암호화자와 위원회가 같은 비밀점을 얻습니다. | 유지합니다. |
| 키·위치별 마스크 | 감사 메시지를 숨깁니다. | Poseidon2·Field 방식으로 구체화합니다. |
| $\Gamma,\ R_2=r\Gamma$ | TDH2의 독립적인 암호문 검증에 사용합니다. | 제외합니다. |
| $W_1=sG,\ W_2=s\Gamma,\ e,f=s+re$ | 암호문과 같은 난수 사용에 대한 검증값입니다. | Event proof와 원본 확인에 책임을 둡니다. |
| $D_i=x_iR_1$ | 위원별 복호화 기여값입니다. | 유지합니다. |
| 위원별 추가 난수·응답 증명 | 거짓 partial decryption을 판별합니다. | 정직한 위원회 가정에 따라 제외합니다. |
| $\sum_i\lambda_iD_i$ | master secret 없이 비밀점을 복원합니다. | 유지합니다. |

**따라서 Zcash 방식의 $\Gamma$ 생성, hash-to-curve, TDH2 검증 Hash의 scalar 변환은 이번 구현에 넣지 않습니다.** 이전 논의는 독립적인 TDH2 검증을 유지한다는 전제였고, 최신 신뢰 모델에서는 적용하지 않습니다.

Jubjub scalar 연산 자체가 없어지는 것은 아닙니다. 키·shares·암호화 난수·결합 계수는 여전히 곡선의 scalar 범위를 사용합니다.

## 6. 어떤 보안을 주장하고, 무엇은 주장하지 않나요?

**이번 목표는 합의한 환경에서 기능·정확성·비용을 확인하는 POC입니다. 원 TDH2의 보안 증명을 그대로 상속한다고 주장하지 않습니다.**

유지해야 할 핵심 연결은 다음입니다.

1. 실제 입력 Note에서 부모 commitment를 계산합니다.
2. 실제 출력 Note와 owner secret에서 출력 nullifier를 계산합니다.
3. 고정된 위원회 공개키와 같은 난수로 공개점·암호문을 계산합니다.
4. PLONK proof가 검증한 공개값과 저장된 값이 같습니다.
5. 복호화 대상은 승인된 기록의 원본입니다.

독립 코어 Circuit은 임의의 평문을 올바르게 암호화했음을 검증할 뿐, 그 평문이 유효한 공급망 Event의 정보인지 판단하지 않습니다. **이 차이를 확인하기 위해 실제 M5 Process에 코어를 연결한 별도 Circuit도 검증합니다.**

필요한 암호학적 가정은 Jubjub의 Diffie–Hellman 관련 난제, 선택한 Poseidon2 구성의 키·마스크 유도에 필요한 성질, PLONK의 영지식·지식 건전성입니다. 기존 Hash가 commitment에 쓰였다는 사실만으로 암호화 전체의 보안 검토가 끝나는 것은 아닙니다.

다음은 보장하지 않습니다.

- 임의 암호문을 처리하는 일반적인 선택 암호문 공격 보안
- 악의적인 위원회·Auditor 또는 기준 이상의 share 유출에 대한 보호
- Go 런타임의 secret 완전 삭제나 상수 시간 실행
- 공개된 테스트용 secret으로 실제 개인정보를 보호하는 기능
- 감사 종료 후 Auditor가 이미 읽은 값을 잊도록 강제하는 기능

기본 형식·길이·곡선점·subgroup·scalar 검사는 생략하지 않습니다. 이는 거짓 응답 증명을 구현하는 것과 다르며, 정상 동작과 Participant 입력 검증에 필요합니다.

## 7. 무엇을 측정하고, 어디에서 멈추나요?

**메시지 한 건의 생성 비용과 많은 암호문의 복원 비용을 따로 봅니다.**

| 관점 | 질문 | 측정 범위 |
|---|---|---|
| Participant | 암호화의 올바름을 얼마나 효율적으로 증명하나요? | 독립 코어·감사 Process의 constraints, prove/verify 시간, 메모리, proof 크기 |
| 온체인 | 검증하고 기록하는 데 얼마나 드나요? | 진단용 verifier·기록 Contract의 deployment, 검증 gas, calldata, 저장 gas |
| 위원회·Auditor | 많은 암호문을 얼마나 효율적으로 복원하나요? | 1·10·100·1,000개 암호문의 partial decryption·결합·복원 |

2-of-3에서 암호문 한 묶음은 두 위원의 응답으로 복원합니다. Field 다섯 개라고 응답 열 개를 요구하지 않습니다. 함수 호출이 하나라고 Field 처리 비용이 메시지 길이와 무관한 것도 아닙니다.

모든 공식 측정은 case별 한 번입니다. 암호문 1,000개는 하나의 workload이며 같은 실험을 1,000회 반복해 평균을 냈다는 뜻이 아닙니다.

### 왜 전체 Ledger를 지금 바꾸지 않나요?

가장 먼저 확인할 불확실성은 실제 암호화 Circuit의 크기와 비용, 그리고 동일 원문 복원입니다. 전체 원장을 먼저 바꾸면 실패 원인이 암호화인지 Event 연결인지 상태 집행인지 구분하기 어려워집니다.

따라서 이번에는 작은 검증·저장 Contract와 Process 사례만 사용합니다. 저장된 원본을 읽는 대표 흐름은 확인하지만, 그것을 Main Ledger의 소비·PolicyGrant·Tree·Status 집행으로 해석하지 않습니다.

후속 단계로 넘기는 것은 다음입니다.

- 모든 Event와 Voucher의 감사 정보 연결
- AuditRecord ID와 producerOf·소비 기록 mapping의 전체 통합
- backward/forward 그래프 탐색과 transaction별 복호화 캐시
- snapshot의 미소비 leaf 선정과 nullifier 기반 Freeze·Revoke
- Claim, DKG·키 회전, 네트워크 서버, Besu와 최종 universal SRS

**기존 M6 결과는 수정하거나 없애지 않습니다. 이번 코어가 완료되어도 기존 Main Contract를 대체하지 않습니다.** 이후 통합 명세에서 변경을 별도로 진행합니다.

## 8. 무엇을 구현 명세에서 확인하면 되나요?

[구현 명세](M6-B1-audit-encryption-core.md)는 Background 없이도 다음을 확인할 수 있게 작성합니다.

- 상수·Domain·Field 순서·공개값과 witness
- 다섯 공통 기능의 관계식·입출력·실패 조건·한글 수도 코드
- 독립 코어와 Process 연결의 차이
- 진단용 Contract가 검증·저장하는 정확한 값
- correctness·측정·Artifact·Result 생성 조건

현재 문서는 선택 이유를 보존합니다. **구현 완료·측정 완료·재사용 검증 완료 여부는 향후 Result와 Milestones에서 관리합니다.**
