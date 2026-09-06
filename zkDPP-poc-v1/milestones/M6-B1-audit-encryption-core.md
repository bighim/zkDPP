# M6-B1 감사 암호화 코어 — 무엇을 어떻게 구현하나요?

**여러 Event에서 재사용할 감사 암호화 코어를 만들고, 실제 3→2 Process의 올바른 감사 정보를 암호화했다는 PLONK 증명과 2-of-3 원문 복원을 검증합니다.**

- 명세 상태: 구현 기준 동결
- 문서 성격: 자체 완결형 구현 기준입니다. 실제 검증·측정·차이는 [M6-B1 Result](M6-B1-audit-encryption-core-result.md)에서 확인합니다.
- 의사결정 배경: [M6-B1 Background](M6-B1-audit-encryption-core-background.md)
- 이전 결과: [M6 Result](M6-status-enforcement-result.md)
- 최우선 원칙: **YAGNI입니다.** 기능·정확성·성능 확인에 필요한 최소 범위만 구현합니다.

## 0. 30초 안에 무엇을 파악하면 되나요?

| 질문 | 구현 기준 |
|---|---|
| 이번 목표는 무엇인가요? | 감사 메시지를 올바르게 암호화했음을 증명하고, 두 위원의 협조로 정확히 복원합니다. |
| 무엇을 재사용하나요? | 기존 Poseidon2·Note·nullifier·membership·M5 Process·PLONK Artifact 기반입니다. |
| 무엇을 새로 만드나요? | Jubjub Threshold DH·Field 마스킹 코어, 공통 Circuit gadget, 두 검증 Circuit, 작은 측정용 Contract입니다. |
| 대표 사례는 무엇인가요? | 부모 Note 세 개를 소비해 ELIGIBLE·WASTE Note 두 개를 만드는 M5 Process입니다. |
| 암호화 평문은 무엇인가요? | 부모 commitment 세 개와 출력 소비 nullifier 두 개입니다. |
| 암호화 단위는 무엇인가요? | transaction당 키 하나·메시지 벡터 하나·암호문 하나입니다. |
| 누구를 신뢰하나요? | Setup 생성자·위원회·Auditor는 정직합니다. Participant의 관계는 Circuit으로 검증합니다. |
| 무엇을 하지 않나요? | 전체 AuditRecord 원장·다른 Event 연결·그래프 탐색·Freeze·Revoke 전환입니다. |
| 언제 완료인가요? | correctness·세 관점의 비용 측정·Raw 대조·Result가 모두 끝났을 때입니다. |

**기존 M6 Main Contract는 그대로 둡니다.** M6-B1은 새 Main Protocol 배포가 아니라, 이후 전방 추적·상태 집행 전환에서 사용할 코어의 검증 단계입니다.

## 1. 전체 흐름과 대표 데이터는 무엇인가요?

**일반 계산으로 암호문을 만들고 → 같은 관계를 Circuit으로 증명하고 → 검증된 기록을 읽어 두 위원의 결과로 복호화합니다.**

대표 전이는 다음입니다.

$$
(cm_A,cm_B,cm_C)\xrightarrow{\mathrm{Process}}(cm_D,cm_E)
$$

| Note | Owner | 역할 | 질량 kg | 재활용 귀속량 kg | 탄소 kgCO2e |
|---|---|---|---:|---:|---:|
| A | actor-1 | ELIGIBLE | 100 | 20 | 80 |
| B | actor-1 | ELIGIBLE | 120 | 10 | 90 |
| C | actor-1 | ELIGIBLE | 100 | 0 | 60 |
| D | actor-1 | ELIGIBLE | 270 | 30 | 260 |
| E | actor-1 | WASTE | 30 | 0 | 0 |

세 숫자의 Circuit 입력은 모두 위 값에 $10^9$을 곱한 uint64입니다. 기존 M5 fixture의 Note·opening·DocumentHash·owner·path 생성 방식을 재사용합니다. 고정 owner secret의 기준은 [actors-v1.json](../testdata/common/actors-v1.json)입니다.

실제 감사 평문은 State가 아니라 다음 다섯 Field입니다.

$$
\mathbf M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

부모 $cm$은 Participant가 가진 private Note에서 계산합니다. 출력 $nf$는 해당 Note의 owner secret과 commitment로 계산합니다. 입력 소비 $nf_A,nf_B,nf_C$와 출력 $cm_D,cm_E$는 기존 Process public input에 있으므로 평문에 중복해서 넣지 않습니다.

객체 종류와 개수는 이 사례의 PROCESS·3→2 구조에서 알 수 있습니다. 부모 순서는 입력 nullifier 순서, 출력 nullifier 순서는 공개 output commitment 순서입니다. type·길이를 암호화 평문에 반복 저장하지 않습니다.

### 실행 경계는 어디까지인가요?

1. Go fixture가 기존 M5 방식으로 정상 Note·root·path를 준비합니다.
2. 독립 코어 Circuit과 감사 Process Circuit이 같은 5-Field 메시지의 암호화를 검증합니다.
3. 작은 Contract가 감사 Process proof를 검증하고 공개값을 기록합니다.
4. 테스트용 감사 실행기가 기록 ID로 원본을 읽고 두 위원의 결과를 결합합니다.
5. 복원한 다섯 Field를 fixture와 정확히 비교합니다.

이 Contract는 실제 Entry·Process 원장 상태를 변경하지 않습니다. 전체 Ledger에서 부모가 소비되고 새 Note가 append되는 통합 시나리오는 후속 단계입니다.

## 2. 누가 무엇을 책임지나요?

| 주체 | 책임 | 신뢰 경계 |
|---|---|---|
| Setup 생성자 | 암호화 공개키·shares 생성 및 분배 | 정직한 생성과 master secret 비보존을 가정합니다. |
| Participant | 감사 메시지·암호문·PLONK proof 생성 | 잘못된 입력을 제출할 수 있으므로 Circuit으로 검증합니다. |
| Event adapter | 실제 부모·출력 nullifier·공개 문맥 구성 | 암호화 코어가 대신 판단하지 않습니다. |
| 위원 | 승인된 기록의 공개점으로 partial decryption 생성 | 정직하게 계산한다고 가정합니다. |
| Auditor 역할의 실행기 | 기록 읽기·응답 대응·결합·평문 해석 | 임의 암호문을 요청하지 않는다고 가정합니다. |
| 측정용 Contract | proof 확인·동일 공개값 기록 | Main Ledger나 정책 승인 서비스가 아닙니다. |

Auditor는 전체 설계에서 Status Authority의 감사 기능을 가리킵니다. 이번에는 네트워크 서비스나 별도 조직 관리 기능을 만들지 않고 로컬 실행기로 재현합니다.

원래 TDH2의 $\Gamma,R_2,e,f$, 위원별 응답 증명과 이를 위한 Hash-to-scalar는 구현하지 않습니다. 공개키 $PK$, $R_1$, partial decryption과 Field 마스킹은 유지합니다. [선택 이유](M6-B1-audit-encryption-core-background.md#5-원래-tdh2에서-무엇을-남기고-무엇을-제외하나요)

**온체인에서 검증·등록된 암호문만 읽는다는 가정은 배포 환경의 조건입니다.** 저수준 CombineAndDecrypt 함수 자체에 블록체인 조회나 승인 기능을 넣지 않습니다. 형식이 올바른 임의 암호문을 그 함수에 직접 넣었을 때 일반적인 인증 암호처럼 변조를 판별한다고 주장하지 않습니다.

## 3. 숫자·Hash·암호문은 어떻게 표현하나요?

### Field와 곡선은 무엇인가요?

| 항목 | 고정 기준 |
|---|---|
| 증명 시스템 | Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, PLONK-KZG/BLS12-381 |
| 메시지 Field | BLS12-381 scalar field $\mathbb F_p$ |
| 암호화 곡선 | gnark-crypto의 BLS12-381 twisted-Edwards, 즉 Jubjub |
| 암호화 그룹 | 라이브러리 기본 기준점 $G$가 생성하는 소수 차수 $q$ subgroup |
| Cofactor | 8 |
| 점 표현 | affine $(X,Y)$, 두 좌표 모두 $\mathbb F_p$ |
| 암호화 scalar | 정수 $0\le v<q$; master secret·생성 난수는 아래 규칙으로 0을 제외합니다. |

라이브러리 상수의 정확한 값은 다음입니다.

$$
p=52435875175126190479447740508185965837690552500527637822603658699938581184513
$$

$$
q=6554484396890773809930967563523245729705921265872317281365359162392183254199
$$

이 값은 코드에서 새로 설계하지 않고 해당 버전의 라이브러리 상수와 대조합니다. $p$는 좌표·메시지·Hash용이고 $q$는 shares·점에 곱하는 scalar·결합 계수용입니다.

### 기존 Poseidon2는 어떻게 재사용하나요?

[Native Hash](../internal/core/hash/poseidon.go)의 여러 Field 입력 Hash와 [Circuit Hash](../internal/circuitutil/gadgets.go)의 동일 기능을 사용합니다. 기존 Merkle compression을 새 대칭암호 함수처럼 직접 호출하지 않습니다.

- 기존 기본 permutation: width 2, full rounds 6, partial rounds 50, S-box degree 5입니다.
- 여러 Field를 처리하는 기존 Merkle–Damgård Hash 구성과 초기값을 그대로 재사용합니다.
- Hash 결과와 입력 Field는 canonical representation을 사용합니다.
- 기존 Note·Owner·Nullifier·Policy Domain은 변경하지 않습니다.

| 새 Domain | 문자열 |
|---|---|
| AuditKeyTag | zkDPP:AuditKey:v1 |
| AuditMaskTag | zkDPP:AuditMask:v1 |
| AuditContextTag | zkDPP:AuditContext:v1 |

각 문자열은 기존 MustHashToField와 같은 방식으로 Field 상수가 됩니다. 이 변환은 Circuit 밖에서 수행하며 Domain은 witness나 추가 public input이 아닙니다.

### 외부 값은 어떤 순서로 읽나요?

| 데이터 | 표현 |
|---|---|
| Field 값 | 32-byte big-endian, $0\le v<p$만 허용 |
| Jubjub scalar | 32-byte big-endian, $0\le v<q$만 허용 |
| 점 | $X$ 다음 $Y$, 총 두 Field. 점 압축 없음 |
| 코어 암호문 | $R_{1X},R_{1Y},C_0,\ldots,C_{n-1}$ |
| 진단 JSON의 Field·scalar | 0x 접두사와 64자리 소문자 hex |
| 위원 ID | 1, 2, 3의 정수 |

큰 정수를 읽고 자동으로 mod $p$ 또는 mod $q$로 줄여 입력 오류를 숨기지 않습니다. 범위 초과·잘못된 길이·잘못된 점은 거부합니다. 낮은 수준의 점 decoding 함수가 모든 검사를 수행한다고 가정하지 않습니다.

코어 API의 최소 데이터는 Point, PublicKey, Share, Ciphertext, Partial입니다. Share는 위원 ID와 비밀 scalar를, Partial은 위원 ID와 점 $D_i$를 가집니다. Ciphertext는 점 $R_1$과 Field 배열만 가집니다. Event 정보·원장 ID는 호출 계층이 보관합니다.

### 새 코어가 공통으로 사용하는 관계는 무엇인가요?

$$
R_1=rG,\qquad Z=rPK
$$

$$
K=H(\mathrm{AuditKeyTag},Z_X,Z_Y,L,n)
$$

$$
k_j=H(\mathrm{AuditMaskTag},K,j),\qquad 0\le j<n
$$

$$
C_j=M_j+k_j\pmod p
$$

$L$은 호출 계층이 결정·검증하는 공개 문맥이고, $n$은 실제 평문 길이입니다. 대칭키 하나에서 위치별 마스크를 파생합니다. 같은 수를 모든 원소에 더하지 않습니다.

코어는 비어 있지 않은 Field 배열을 처리합니다. Native에는 임의의 실제 길이를 전달할 수 있지만 Circuit에서는 compile 전에 길이를 고정합니다. 가변 최대 slot·inactive padding·범용 직렬화 프레임워크는 만들지 않습니다.

## 4. TrustedSetup은 어떻게 구현하나요?

**암호화 키 생성과 PLONK SRS·회로 Setup은 서로 다른 작업입니다.**

암호화 Setup은 위원회 공개키를 정합니다. 그 뒤 해당 공개키를 고정한 두 Circuit을 compile하고 개발용 PLONK Setup을 수행합니다.

### 입력·출력과 관계

입력은 안전한 난수원이며 위원 수 3·threshold 2는 고정입니다. 반환값은 공개키와 ID 1·2·3의 shares입니다. master secret을 반환값이나 저장 파일에 포함하지 않습니다.

$$
F(t)=x+at\pmod q,\qquad x_i=F(i),\qquad PK=xG
$$

$x,a$는 $1\le v<q$에서 생성합니다. 어느 share가 0이면 다시 생성합니다. $a\ne0$이므로 서로 다른 위원 ID의 share가 같은 값이 되는 상수 다항식을 사용하지 않습니다.

~~~text
TrustedSetup():
    # 핵심: 하나의 공개키와, 서로 다른 두 위원이 협력할 shares를 만듭니다.

    # 1. 라이브러리의 고정 곡선·기준점을 확인합니다.
    p, q, G = 고정된 Jubjub 설정
    G가 곡선 위에 있고, 영점이 아니며, q*G가 영점인지 확인합니다.

    # 2. 안전한 난수로 master secret과 공유용 계수를 만듭니다.
    x, a = 각각 [1, q-1]에서 균등하게 생성
    shares[i] = (i, (x + a*i) mod q), i = 1, 2, 3
    0인 share가 있으면 다시 생성합니다.

    # 3. 공개키를 만들고 비밀 자료를 분리합니다.
    PK = x*G
    PK의 곡선·subgroup·비영점 조건을 확인합니다.
    공개 파일에는 곡선 정보·PK·공개 checksum만 기록합니다.
    각 위원 파일에는 해당 위원의 ID·share·공개 설정 식별정보만 기록합니다.

    # 4. master secret과 공유용 계수는 영속 저장하지 않습니다.
    x와 a의 불필요한 보관을 끝내고 PK, shares만 반환합니다.
~~~

### 실패 조건·재현·측정

난수원 실패, 잘못된 고정 곡선·기준점, 허용되지 않은 공개키는 실패입니다. 실패 후 부분적인 설정을 정상 Artifact로 취급하지 않습니다.

실제 생성에는 crypto/rand를 사용합니다. 테스트에서만 고정 $x,a$와 재현값을 주입할 수 있으며, 이를 Production 또는 공개 네트워크용으로 제시하지 않습니다. Node 간 DKG·share refresh·키 회전 CLI는 만들지 않습니다.

private shares는 Git과 Raw 결과에서 제외하고 위원별로 분리합니다. 생성자가 남긴 사본이 없다는 가정과 Go의 완전한 메모리 삭제를 보장하지 못한다는 한계를 함께 기록합니다.

암호화 Setup 시간과 공개키·share 파일 크기는 PLONK Setup 시간·SRS 크기와 분리합니다. 공유한 공개키는 이 milestone의 모든 Circuit과 측정에서 동일하게 사용합니다.

## 5. Encrypt는 어떻게 계산하나요?

**코어는 평문의 의미를 판단하지 않고 Field 벡터를 암호화합니다.**

### Interface와 범위

입력은 PublicKey, $L$, 비어 있지 않은 Field 배열 $\mathbf M$, 난수 $r$입니다. 출력은 Ciphertext입니다. $r$은 호출자가 안전하게 생성하고 proof witness에도 같은 값을 사용합니다.

난수는 $1\le r<q$입니다. 정상 암호화 호출은 새 난수를 사용합니다. 결정적 테스트 fixture를 제외하고 서로 다른 암호화에 같은 난수를 재사용하지 않습니다. Circuit이 난수의 진짜 무작위성을 증명하는 것은 아닙니다.

~~~text
Encrypt(PK, L, M, r):
    # 핵심: 키 하나를 만들고, 각 위치의 다른 마스크로 벡터 전체를 숨깁니다.

    # 1. 타입·길이·범위를 확인합니다.
    M이 비어 있으면 실패합니다.
    PK가 유효한 비영점 subgroup 점인지 확인합니다.
    L과 M의 모든 원소가 canonical Field인지 확인합니다.
    1 <= r < q인지 확인합니다.

    # 2. 같은 난수로 공개점과 비밀점을 계산합니다.
    R1 = r*G
    Z = r*PK
    n = len(M)
    K = H(AuditKeyTag, Z.X, Z.Y, L, n)

    # 3. 같은 키에서 위치별 마스크를 파생합니다.
    각 j = 0 .. n-1에 대해:
        mask = H(AuditMaskTag, K, j)
        C[j] = M[j] + mask   # 메시지 Field에서 계산합니다.

    # 4. 공개 암호문만 반환합니다.
    return Ciphertext(R1, C)
~~~

공개값은 $L$과 Ciphertext입니다. $\mathbf M,r,Z,K,k_j$는 비공개입니다. 평문을 로그·Raw 결과에 무심코 출력하지 않습니다. 재현 fixture의 공개된 값은 테스트 자료임을 구분합니다.

길이·범위·곡선·subgroup 조건 위반은 실패합니다. 다른 정상 공개키로 암호화하는 수학적 연산 자체는 가능하지만, 해당 암호문은 고정 공개키의 Circuit에서 통과할 수 없어야 합니다.

Field 0과 $p-1$은 유효한 평문입니다. $M_j+k_j$가 $p$를 넘는 것은 정상적인 Field wraparound이며 정보 손실이 아닙니다.

일반 암호화 시간·할당 메모리는 proof 생성 시간과 별도 기록합니다.

## 6. AssertEncrypted와 독립 Circuit은 무엇을 증명하나요?

**주어진 private 평문을 주어진 공개 문맥에서 올바르게 암호화했는지 증명합니다. Event의 진실성은 아직 판단하지 않습니다.**

공통 gadget 입력은 고정 PublicKey, $L$, private $\mathbf M$, private $r$, 공개 Ciphertext입니다. gadget은 값을 암호화해서 반환하기보다 공개 암호문과의 관계를 강제합니다.

~~~text
AssertEncrypted(api, fixedPK, L, M, r, CT):
    # 핵심: Native Encrypt와 같은 관계를 constraints로 강제합니다.

    # 1. compile 시 실제 벡터 길이와 공개키를 고정합니다.
    n = len(M)
    n > 0이고 len(CT.C) == n인지 확인합니다.
    fixedPK와 G는 compile 이전에 곡선·subgroup·비영점 검증을 마친 상수입니다.

    # 2. private scalar가 허용 범위인지 확인합니다.
    r를 252비트로 분해하고 r <= q-1, r != 0을 강제합니다.

    # 3. 공개점과 키 생성 관계를 확인합니다.
    expectedR1 = JubjubScalarMul(G, r)
    CT.R1.X == expectedR1.X, CT.R1.Y == expectedR1.Y를 강제합니다.
    Z = JubjubScalarMul(fixedPK, r)
    K = H(AuditKeyTag, Z.X, Z.Y, L, n)

    # 4. 각 위치의 평문과 공개 암호문을 연결합니다.
    각 j = 0 .. n-1에 대해:
        mask = H(AuditMaskTag, K, j)
        CT.C[j] == M[j] + mask를 강제합니다.
~~~

정상 subgroup 기준점과 $1\le r<q$에서 계산한 expectedR1과 좌표가 같으므로, 공개점의 유효성을 임의 witness에 맡기지 않습니다. 점 gadget의 API 이름만으로 임의 점의 subgroup 검사가 끝났다고 가정하지 않습니다.

### 독립 5-Field Circuit의 공개·비공개값

| 구분 | 순서·내용 |
|---|---|
| Public input | $L,R_{1X},R_{1Y},C_0,C_1,C_2,C_3,C_4$ — 8개 |
| Private witness | $M_0,\ldots,M_4,r$ |
| Circuit constant | 위원회 PK, 곡선·Domain·벡터 길이 |

대표 fixture는 Process에서 얻은 같은 $L,\mathbf M,r,CT$를 사용합니다. 독립 Circuit이 통과했다는 사실만으로 실제 부모·출력 nullifier가 맞다고 주장하지 않습니다.

같은 gadget이 1-Field에서도 동작하는지는 correctness test에서 확인합니다. 1-Field 전용 배포·공식 Setup·성능 표를 추가하지 않습니다.

잘못된 ciphertext 좌표·원소, 변조한 평문·문맥, 범위 밖 난수와 다른 공개키로 생성한 암호문은 실패해야 합니다. 공개값 개수와 순서는 witness를 추출해 확인합니다.

## 7. PartialDecrypt와 CombineAndDecrypt는 어떻게 복원하나요?

**위원은 자기 share로 점 하나를 만들고, Auditor는 서로 다른 두 점을 결합해 같은 키를 얻습니다.**

### PartialDecrypt

입력은 위원 Share와 해당 기록의 $R_1$입니다. 출력은 ID와 점 $D_i$입니다.

$$
D_i=x_iR_1
$$

~~~text
PartialDecrypt(share, R1):
    # 핵심: secret share 자체를 보내지 않고 해당 공개점에 대한 결과만 만듭니다.

    # 1. 로컬 설정과 입력 형식을 확인합니다.
    share.ID가 1, 2, 3 중 하나인지 확인합니다.
    1 <= share.Value < q인지 확인합니다.
    R1의 canonical 좌표·곡선·subgroup·비영점 조건을 확인합니다.

    # 2. 자신의 share로 계산합니다.
    D = share.Value * R1

    # 3. 위원 ID와 점만 반환합니다.
    return Partial(share.ID, D)
~~~

어떤 기록을 처리할지 승인하고, 온체인 원본을 읽는 것은 호출 계층의 책임입니다. 임의 암호문을 받는 HTTP API는 만들지 않습니다. 위원이 거짓 결과를 보내는 것을 검증하는 추가 증명은 반환하지 않습니다.

### CombineAndDecrypt

입력은 공개 문맥 $L$, Ciphertext, 서로 다른 두 Partial입니다. 출력은 Field 평문 배열입니다. 코어는 정확히 두 응답을 받습니다. 더 많은 응답을 수집한 호출 계층은 사용할 두 위원을 선택합니다.

$$
\lambda_i=\prod_{\substack{h\in S\\h\ne i}}\frac{-h}{i-h}\pmod q
$$

$$
Z=\sum_{i\in S}\lambda_iD_i
$$

분수는 실수 나눗셈이 아니라 mod $q$ 역원을 이용합니다. ID 1·2 조합에서는 $\lambda_1=2,\lambda_2=-1$이며, 다른 조합에서도 해당 ID의 계수를 계산합니다.

$$
K=H(\mathrm{AuditKeyTag},Z_X,Z_Y,L,n)
$$

$$
M_j=C_j-H(\mathrm{AuditMaskTag},K,j)\pmod p
$$

~~~text
CombineAndDecrypt(L, CT, partials):
    # 핵심: 두 응답으로 비밀점을 복원하고 같은 마스크를 빼서 원문을 얻습니다.

    # 1. 응답 수·ID·형식을 확인합니다.
    정확히 두 응답이며 ID가 서로 다르고 1..3 범위인지 확인합니다.
    L, CT와 응답 점들의 canonical 표현·필요한 곡선 조건을 확인합니다.
    CT.C가 비어 있으면 실패합니다.
    두 응답이 같은 기록과 공개 설정에 속하는지는 호출 계층에서 확인합니다.

    # 2. 위원 번호에 맞는 계수로 점들을 결합합니다.
    lambda_i = 다른 위원 h에 대한 (-h)/(i-h) mod q
    Z = lambda_1 * D_1 + lambda_2 * D_2

    # 3. 같은 문맥과 실제 길이로 키와 마스크를 재현합니다.
    n = len(CT.C)
    K = H(AuditKeyTag, Z.X, Z.Y, L, n)
    각 j = 0 .. n-1에 대해:
        M[j] = CT.C[j] - H(AuditMaskTag, K, j)

    # 4. 해석 전의 Field 배열을 반환합니다.
    return M
~~~

부족한 응답·중복 ID·잘못된 길이·허용되지 않은 점은 실패합니다. 정직한 위원회 가정이므로 **형식이 올바른 거짓 응답의 암호학적 판별**은 요구하지 않습니다.

또한 이 함수만으로는 올바른 형식의 변조된 암호문·다른 문맥을 인증할 수 없습니다. 잘못된 원문이 반환될 수 있습니다. 변경된 ciphertext를 거부하는 책임은 Event proof·동일 기록 확인에 있습니다. 테스트에서도 이를 임의 암호문에 대한 인증 복호화 실패로 잘못 표현하지 않습니다.

원문을 보고 추가 Field 축약이나 손실 있는 mod 변환을 하지 않습니다. 반환된 Field의 순서 그대로 원래 메시지와 비교합니다.

## 8. 실제 Process의 감사 정보는 어떻게 연결하나요?

**M5 Process의 의미를 보존하고, 그 private Note에서 감사 평문을 직접 구성합니다.**

[기존 Process Circuit](../features/process_policy_3_2/circuit.go)은 수정하지 않습니다. 별도 adapter Circuit에서 기존 Define 관계를 재사용하고 암호화 관계를 추가합니다. 기존 CCS·PK·VK로 변경된 Circuit을 검증할 수 있다고 가정하지 않습니다.

### 기존 Process에서 유지할 조건

| 영역 | 유지할 관계 |
|---|---|
| Policy | 고정 canonical PolicyRef, owner secret과 ScopeRef의 Poseidon2 관계 |
| 입력 | 같은 owner의 서로 다른 ELIGIBLE Note 3개 |
| membership | 같은 Note root, private index와 sibling 32개, 기존 compression |
| 소비값 | 입력마다 기존 Note nullifier 계산·공개값 일치·중복 거부 |
| 숫자 | State uint64, $a_{\mathrm{rec}}\le q_{\mathrm{mass}}$, 단계별 덧셈 overflow 거부 |
| Process | 기존 고정 lossRate·carbonIntensity·rounding |
| 출력 | 같은 owner, ELIGIBLE·WASTE 순서, WASTE의 재활용·탄소 0 |
| commitment | 공개 output과 계산 결과 일치, output 중복 거부 |

숫자 scale과 비율 분모 $D$의 값은 모두 $10^9$이지만 의미는 구분합니다.

$$
\mathrm{lossRate}=62{,}500{,}000,\quad
\mathrm{carbonIntensity}=93{,}750{,}000,\quad
\mathrm{wasteMassRate}=100{,}000{,}000
$$

입력 합계를 $(Q,A,E)$라고 하면 다음 관계를 유지합니다.

$$
q_{\mathrm{loss}}=\left\lfloor\frac{Q\cdot\mathrm{lossRate}}{D}\right\rfloor,\qquad
\Delta e=\left\lfloor\frac{Q\cdot\mathrm{carbonIntensity}}{D}\right\rfloor
$$

$$
Q_I=Q-q_{\mathrm{loss}},\quad A_I=A,\quad E_I=E+\Delta e
$$

$$
Q_E=\left\lfloor\frac{Q_I\cdot\mathrm{wasteMassRate}}{D}\right\rfloor
$$

출력 D의 State는 $(Q_I-Q_E,A_I,E_I)$, 출력 E의 State는 $(Q_E,0,0)$입니다. 각 floor는 몫·나머지 관계와 $0\le remainder<D$를 검증합니다. Product Type과 input/output DocumentHash의 관계는 기존 M5처럼 검사하지 않습니다.

### 공개 입력과 문맥

공개 입력은 아래 순서를 고정합니다. 기존 M5의 8개 public input을 앞부분에 그대로 유지합니다.

| 위치 | 값 |
|---:|---|
| 0 | policyRef |
| 1 | policyScopeRef |
| 2 | noteRoot |
| 3, 4, 5 | 입력 nf A, B, C |
| 6, 7 | 출력 cm D, E |
| 8, 9 | $R_{1X},R_{1Y}$ |
| 10, 11, 12, 13, 14 | $C_0,\ldots,C_4$ |

PROCESS는 기존 EventKind 6입니다. 공개 문맥은 Circuit 안에서 다음 순서로 계산하며 별도 public input으로 추가하지 않습니다.

$$
L=H(\mathrm{AuditContextTag},6,
\mathrm{policyRef},\mathrm{policyScopeRef},\mathrm{noteRoot},
nf_A,nf_B,nf_C,cm_D,cm_E)
$$

이 문맥은 이 POC의 고정 deployment·공개 설정을 전제로 합니다. production용 cross-chain·key-rotation replay 정책을 구현한 것으로 주장하지 않습니다.

### 전체 adapter 수도 코드

~~~text
AuditProcessCircuit(publicInputs, privateWitness):
    # 핵심: 실제 Process witness에서 감사 평문을 만들고 같은 공개 암호문을 검증합니다.

    # 1. 기존 M5 관계를 그대로 검증합니다.
    M5Process.Define(기존 public input 8개와 기존 private witness)
    # 위 표의 owner·membership·nullifier·State·Policy·output 조건을 모두 포함합니다.

    # 2. 실제 입력 Note의 commitment를 같은 순서로 다시 얻습니다.
    각 i = 0, 1, 2에 대해:
        parentCM[i] = 기존 NoteCommitment(Inputs[i])
    # 임의의 parent 배열을 별도 witness로 받아 신뢰하지 않습니다.

    # 3. 실제 출력의 미래 소비값을 계산합니다.
    outNF[0] = H(NullifierTag, SKOwner, CMOut[0])
    outNF[1] = H(NullifierTag, SKOwner, CMOut[1])
    M = [parentCM[0], parentCM[1], parentCM[2], outNF[0], outNF[1]]

    # 4. 공개 Event 문맥을 계산합니다.
    L = H(AuditContextTag, PROCESS, 기존 public input 8개를 기존 순서로)

    # 5. 실제 평문과 공개 암호문을 공통 gadget에 연결합니다.
    AssertEncrypted(api, fixedCommitteePK, L, M, private r, public CT)
~~~

기존 M5 Define을 호출한 후 입력 commitment를 재계산하는 작은 중복은 허용합니다. 기존 구현을 변경하는 대규모 공통화는 이번에 하지 않습니다. Source를 재사용하는 것과 기존 proving key를 재사용하는 것을 구분합니다.

### 실패 조건과 측정

기존 Process 실패 조건에 더해, 순서를 바꾼 부모·잘못된 출력 nullifier로 만든 암호문, 다른 공개키·난수 관계·문맥을 사용한 암호문이 실패해야 합니다.

공통 코어는 같은 잘못된 평문을 일관되게 암호화하면 성공할 수 있지만, Process adapter는 실제 witness에서 평문을 구성하므로 실패해야 합니다. 이 차이를 별도 positive/negative pair로 확인합니다.

독립 코어와 adapter는 각각 constraints·public inputs·prove/verify·할당 메모리를 측정합니다. 기존 M5 Raw 값은 재실행 없이 읽어 constraints 차이와 회로 구성 차이를 설명하는 데만 사용합니다. 서로 다른 실행 시점의 시간을 안정적인 성능 비율로 일반화하지 않습니다.

## 9. 측정용 Contract는 무엇을 저장하나요?

**Main Ledger를 변경하지 않고 proof 검증·기록 비용만 확인하는 진단용 Contract를 만듭니다.**

후속 구현 이름은 AuditEncryptionStore이며 constructor에서 감사 Process verifier 하나를 고정합니다. 독립 코어 Circuit은 Native proof 검증에 사용하고, 그 전용 Contract를 추가 배포하지 않습니다.

### Interface와 상태

| Interface·상태 | 역할 |
|---|---|
| constructor(processVerifier) | 15-public-input 감사 Process verifier를 고정합니다. |
| verifyOnly(proof, inputs[15]) | proof만 확인하고 상태를 바꾸지 않습니다. |
| verifyAndStore(proof, inputs[15]) | proof 성공 후 같은 배열을 신규 ID에 기록합니다. |
| getRecord(id) | 저장된 공개 입력 15개를 반환합니다. |
| nextRecordId | 1부터 증가하는 진단용 기록 ID입니다. |
| records[id] | 변경 불가능한 uint256[15] 배열입니다. |

각 공개값은 $p$ 미만이어야 합니다. 0인 verifier 주소·코드 없는 주소, 실패 또는 revert한 proof, 없는 기록 ID를 거부합니다. 기존 verifier 호출 관례처럼 실패를 하나의 진단용 proof 오류로 정규화합니다.

기존 public input 8개가 저장된 **문맥 원자료**입니다. Auditor는 이를 읽어 같은 $L$을 Go에서 계산합니다. $L$을 중복 저장하거나 이를 계산하는 온체인 Poseidon2 Contract를 추가하지 않습니다.

~~~text
verifyAndStore(proof, inputs[15]):
    # 핵심: 검증한 바로 그 공개 입력 배열을 기록합니다.

    # 1. 공개값의 기본 형식과 proof를 확인합니다.
    모든 inputs 원소가 p보다 작은지 확인합니다.
    고정 verifier.Verify(proof, inputs)가 성공하지 않으면 revert합니다.

    # 2. 신규 ID에 동일한 배열을 원자적으로 저장합니다.
    id = nextRecordId
    records[id] = inputs
    nextRecordId = id + 1

    # 3. 조회할 ID만 반환하고 알립니다.
    RecordStored(id)를 발생시키고 id를 반환합니다.
~~~

verifyOnly는 같은 기본 검사와 verifier 호출을 수행하되 기록·ID·Event를 변경하지 않습니다. 측정을 위해 view 함수도 실제 transaction으로 제출해 receipt gas를 얻습니다.

공개키를 바꾸거나 저장된 배열을 수정하는 함수는 만들지 않습니다. 별도 storeOnly 우회 함수도 만들지 않습니다.

### Main Protocol과 무엇이 다른가요?

이 Contract는 이미 사용된 Note를 판별하지 않고 accepted root·PolicyGrant·Status도 집행하지 않습니다. 같은 유효 proof를 여러 ID에 기록할 수 있습니다. 이는 측정용 도구의 범위이며 production에서 허용할 동작이라는 뜻이 아닙니다.

실제 PolicyRef·ScopeRef 관계는 Process Circuit에서 확인하지만, 온체인 권한 Registry까지 구현한 것으로 표시하지 않습니다. State·membership 관계는 fixture와 Circuit에서 검증되고 Main Ledger의 현재 상태와는 연결하지 않습니다.

### 저장된 원본에서 복원하는 대표 흐름

1. 정상 감사 Process proof로 verifyAndStore를 한 번 성공시킵니다.
2. 테스트 실행기가 반환된 ID를 위원 역할의 함수들에 전달합니다.
3. 각 위원은 getRecord로 동일한 배열을 읽고, 같은 verifier·공개키 설정과 연결된 기록인지 확인합니다.
4. 입력 0..7로 $L$을 재계산하고, 8..14에서 Ciphertext를 읽습니다.
5. 위원 1·2의 Partial을 결합해 다섯 Field를 복원합니다.
6. fixture의 실제 부모·출력 nullifier와 순서까지 비교합니다.

네트워크 서버·RPC 권한 정책·감사 승인 Contract는 구현하지 않습니다. 로컬 역할 분리로 승인된 원본만 처리한다는 정상 흐름을 재현합니다.

## 10. 무엇이 통과해야 하나요?

| Gate | 성공·실패 기준 |
|---|---|
| Setup | 같은 곡선·PK로 세 shares 생성, 잘못된 곡선·영점 PK 거부 |
| Native 왕복 | 세 위원 쌍 모두 같은 원문, 1-Field·5-Field 재사용 |
| Encoding | 범위 초과·짧거나 긴 고정 원소·잘못된 hex·잘못된 점 거부 |
| Field 경계 | 0·$p-1$·wraparound 이후 원문 보존 |
| 독립 Circuit | 정상 암호화 통과, 변조한 메시지·공개점·암호문·문맥·난수 실패 |
| Process binding | 실제 부모·출력 nf가 아닌 평문으로 만든 정상 암호문도 실패 |
| 기존 Process | owner·path·nf·State·Role·Policy 관계를 변경하지 않음 |
| 응답 처리 | ID 범위·중복·부족한 응답 거부, 세 유효한 쌍 복원 |
| Contract | 15개 공개 입력·저장 배열 일치, 실패 후 ID·기록 불변 |
| 원본 복원 | getRecord 값만으로 문맥·암호문을 재구성하고 원문 일치 |
| 공개범위 | 부모 cm·출력 nf·owner secret·r·K가 public witness에 없음 |
| Artifact | 저장한 CCS·keys 재로딩, 공개 설정·checksum과 일치 |

형식이 올바른 잘못된 $D_i$를 판별하는 응답 증명이나 임의 ciphertext의 인증 복호화 Gate는 만들지 않습니다. 한 개의 응답을 거부하는 테스트가 암호학적 2-of-3 보안 증명 자체는 아니라는 점도 기록합니다.

기존 M1~M6의 코드·Artifact·Raw 결과 checksum을 보존합니다. 실제 실행 결과는 Result에서 관리합니다.

## 11. 비용은 어떤 경계에서 측정하나요?

**모든 공식 case는 한 번입니다. workload 크기와 반복 측정 횟수를 혼동하지 않습니다.**

### 환경과 공통 원칙

- 기존 Go·gnark·Foundry/Anvil 도구 버전을 유지합니다.
- 공식 Go 측정은 GOMAXPROCS=8로 고정하고 요청값·실제값·CPU·OS·아키텍처를 기록합니다.
- proof 호출과 복호화 workload는 순차 실행합니다. 별도 ST 비교·GPU·위원 네트워크 병렬화는 추가하지 않습니다.
- Anvil은 기존 Prague, chain ID 31337, port 18545, 30M block gas limit을 사용합니다.
- 소스·Artifact·생성 verifier·fixture checksum을 기록합니다.
- 실패가 발생하면 실패 항목을 지우거나 성공한 것으로 표시하지 않습니다. 재실행이 필요하면 사유와 시도 횟수를 기록하고 결과를 평균처럼 제시하지 않습니다.

### Participant와 Circuit

각 두 Circuit에 대해 다음을 기록합니다.

- constraints·public inputs·compile 시간
- 개발 SRS 생성과 PLONK Setup 시간
- 일반 암호화·witness 준비·prove·native verify 시간
- binary proof bytes와 Solidity serialization bytes 구분
- CCS·canonical/Lagrange SRS·PK·VK 크기
- runtime.TotalAlloc 증가량과 구간 전후 HeapAlloc

할당 메모리·heap snapshot은 peak RSS가 아닙니다. prove 구간의 메모리 통계를 전체 시스템 최대 메모리라고 표시하지 않습니다. Setup·파일 로딩·JSON 기록은 prove 시간에 포함하지 않습니다.

### 온체인

한 clean chain에서 Process verifier·AuditEncryptionStore를 배포하고 동일 fixed proof의 verifyOnly와 verifyAndStore를 각각 한 번 제출합니다.

- 두 deployment의 receipt gas
- 두 호출의 receipt gas와 ABI calldata bytes
- 기록된 문맥 원자료 8 Field와 암호문 7 Field의 저장 범위
- transaction trace에서 기록 경로의 SSTORE 횟수·해당 opcode gas
- verifyAndStore와 verifyOnly의 gas 차이

전체 기록은 public input 15개이며 논리 데이터 480 B입니다. 그중 암호문 핵심은 $R_1$ 좌표 2개와 암호화된 값 5개, 즉 224 B입니다. ABI header·mapping·ID 관리 비용과 구분합니다.

두 함수의 총 gas 차이에는 storage 외의 제어·Event·calldata 차이도 포함됩니다. 이를 순수 SSTORE gas라고 표시하지 않습니다. 독립 암호문을 반환하는 것과 Main Process Event 전체 gas는 서로 다른 측정입니다.

### 위원회와 Auditor

서로 독립적인 5-Field 암호문 1·10·100·1,000개를 각각 하나의 workload로 측정합니다. 각 암호문은 새 $r$을 사용하며 workload 안의 $R_1$ 중복이 없어야 합니다.

암호문 준비·Setup·파일 로딩은 시간 구간 밖에 둡니다. 각 workload에서 위원 1·2의 partial decryption, 두 응답 결합·키/마스크 생성·평문 복원, 전체 합계와 할당 메모리를 기록합니다.

correctness에서 모든 쌍을 확인하되 공식 처리량은 ID 1·2 쌍으로 고정합니다. 암호문 $V$개에 대한 위원 응답은 $2V$개이고 평문 Field 수와는 별개입니다.

이 workload는 정상 Native Encrypt로 준비한 자료의 **오프라인 코어 처리량**입니다. 1,000개 암호문 각각을 PLONK로 등록하고 실제 그래프를 탐색한 결과가 아닙니다. 실제 온체인 원본 흐름은 앞의 대표 기록 한 건으로 별도 검증합니다.

## 12. 구현 위치·명령·Artifact는 어떻게 나누나요?

**아래는 구현 위치와 실행 경계입니다.**

| 영역 | 위치·역할 |
|---|---|
| 재사용 코어 | internal/core/auditcrypto: Setup, Encrypt, PartialDecrypt, CombineAndDecrypt, encoding |
| 공통 gadget | internal/circuitutil/audit_encryption.go: AssertEncrypted |
| 독립 feature | features/audit_encryption: 5-Field Circuit과 README |
| Process adapter | features/audit_process_3_2: 기존 M5 관계에 감사 암호화 추가 |
| fixture | internal/m6b1case: M5 자료로 평문·문맥·정상/실패 사례 구성 |
| 진단용 Contract | contracts/src/AuditEncryptionStore.sol와 해당 test |
| 실행기 | cmd/setup_m6_b1, evaluate_m6_b1, benchmark_m6_b1 |

각 feature README는 목적·관계·공개/비공개·실행 방법·실패 사례·한계를 설명합니다. 새 코어는 기존 프로젝트를 import하지 않습니다. 기존 M5 Define·Note·Hash·Merkle 기반은 현재 module 안에서 재사용합니다.

### 명령과 실행 순서

| 명령 | 수행 내용 |
|---|---|
| make setup-m6-b1 | 암호화 Setup·두 Circuit compile·개발 SRS·PK/VK·Process verifier export |
| make evaluate-m6-b1 | 두 Circuit의 실제 Prove/Verify·메모리·proof 크기 측정과 fixed proof fixture 생성 |
| make test-go | 전체 correctness suite를 최종 한 번 실행 |
| make test-contract-m6-b1 | 대표 proof·실패·기록 원자성·원본 복원 연결 확인 |
| make benchmark-m6-b1-gas | clean Anvil의 배포·verify-only·verify-and-store 측정 |
| make benchmark-m6-b1-decrypt | 네 가지 오프라인 복호화 workload 측정 |
| make benchmark-m6-b1 | gas와 decrypt를 순서대로 실행하는 aggregate |
| make check-m6-b1 | 실행 전 baseline과 신규 생성물의 checksum 대조 |

setup 명령에서 성능용 Prove를 미리 반복하지 않습니다. evaluate에서 생성한 fixed proof를 Contract test·gas 측정에 재사용합니다. aggregate와 개별 benchmark를 모두 실행해 같은 공식 결과를 중복 생성하지 않습니다.

개발용 Artifact는 artifacts/development/m6-b1 아래 committee, audit-encryption, audit-process-3-2로 분리합니다. committee에는 공개 설정과 위원별 private share 파일을 구분하고, Circuit별로 CCS·SRS·PK·VK·manifest를 저장합니다.

기존 artifact package의 milestone 경로·checksum 기반을 재사용합니다. 암호화 공개키를 바꾸면 고정 상수가 달라지므로 두 Circuit의 Setup·keys·verifier도 다시 생성해야 합니다. 이번에는 키 회전을 지원하지 않습니다.

Raw 결과 파일은 다음입니다.

- output/m6-b1-circuit.json
- output/m6-b1-anvil-gas.json
- output/m6-b1-decryption.json
- output/m6-b1-generated-checksums.json

각 결과에는 profile·곡선·Hash 설정·공개키 식별·도구 버전·실행 경계·runCount=1을 기록합니다. master secret·shares·비밀 대칭키는 Raw에 넣지 않습니다. 생성물과 private 자료는 Git에서 제외하고, 결과 JSON은 기존 경향에 맞춰 보존합니다.

## 13. 어느 순서로 개발하고 무엇으로 완료하나요?

| 내부 단계 | 결과 |
|---|---|
| M6-B1.1 일반 Go 코어 | Native Setup·벡터 암호화·2-of-3 복원·형식 검증 |
| M6-B1.2 Circuit·Process 연결 | 공통 gadget·독립 5-Field 관계·실제 Note에서 감사 평문 구성·기존 M5 의미 보존 |
| M6-B1.3 Setup·Artifact | 두 Circuit의 개발 keys·공식 proof·Process verifier·고정 fixture |
| M6-B1.4 EVM 진단 | 검증·기록 Contract·원본 조회와 복원 |
| M6-B1.5 검증·측정 | 전체 correctness·gas·네 복호화 workload |
| M6-B1.6 결과 대조·문서 | 보존 기준·checksum·Result·활성 문서 동기화 |

이 표는 기능 의존 순서입니다. 승인된 일괄 구현은 Native 코어 → gadget·Process 연결 → Setup·proof → 진단 Contract → correctness·측정 → Result의 여섯 단계로 진행했습니다. 구체적인 실행 이력은 Result에서 관리합니다.

완료할 때는 [Result Template](RESULT-TEMPLATE.md)에 따라 M6-B1-audit-encryption-core-result.md를 생성합니다. 첫 화면에는 코어와 Event adapter의 차이, 이번에 가능한 동작, 대표 correctness·측정값·비범위를 표시합니다.

다음 조건을 모두 만족해야 MILESTONES의 상태를 완료로 바꿀 수 있습니다.

1. Native·Circuit·저장값·복호화 평문이 일치합니다.
2. 실제 Event와 무관한 평문을 암호화한 경우를 Process adapter가 거부합니다.
3. 정한 correctness Gate와 공식 case별 측정이 끝났습니다.
4. Result의 값과 Raw JSON을 대조했습니다.
5. 기존 M1~M6 코드·Artifact·Raw 결과가 보존됐습니다.
6. 결과를 전체 provenance 감사나 새로운 Main Protocol 구현으로 과장하지 않았습니다.

## 14. 무엇을 다음 단계로 남기나요?

이번에는 전체 Event 적용, Voucher 감사 회로, 실제 AuditRecord Registry, producerOf·noteSpentIn·voucherSpentIn, 그래프 탐색·캐시·snapshot, nullifier 기반 Status mapping과 Freeze·Revoke를 구현하지 않습니다.

기존 M6 StatusTree·Main Contract도 수정하거나 제거하지 않습니다. B1은 후속 통합의 기반이며 현재 Main 경로의 교체가 아닙니다.

Claim·DPP·DKG·키 refresh/rotation·위원 네트워크·Production secret 보관·Besu·최종 universal SRS·범용 암호 프레임워크도 제외합니다.

M7 등 후속 단계는 이 코어의 결과를 읽은 뒤 원장·Event·감사·동결을 연결하는 명세를 별도로 작성해야 합니다. 여기서 그 전체 구현 순서나 새 milestone 번호까지 임의로 확정하지 않습니다.

**이 명세는 구현과 측정의 기준이지, 정식 암호학적 보안 증명이나 Production 배포 승인 문서가 아닙니다.** 신뢰 Setup·정직한 위원회·검증된 기록 전용이라는 가정과 변형된 암호 구성의 한계를 결과에도 유지합니다.
