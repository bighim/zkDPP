# zkDPP Threshold Partial Decryption 설계

## 0. 문서 목적

이 문서는 zkDPP의 비공개 provenance 감사에서 Threshold Encryption과 Threshold Partial Decryption이 어떻게 사용되는지 설명한다.

핵심 목표는 다음과 같다.

```text
평상시:
    입력과 출력의 provenance edge를 숨긴다.

문제 발생:
    K-out-of-N 감사 승인이 있을 때만 parent edge를 복원한다.

보안 목표:
    Status Authority가 committee master secret key를 얻지 않는다.
```

이 문서에서는 Threshold Decryption을 `위원의 secret share 자체를 전달하지 않고, 특정 ciphertext에 대한 partial decryption만 전달하는 방식`으로 정의한다.

---

# 1. 한 문장으로 이해하기

> AuditRecord는 parent 목록을 담은 잠긴 봉투이고, Threshold Encryption은 봉투를 잠그는 방법이며, Threshold Partial Decryption은 K명의 위원이 자신의 비밀키를 내주지 않고 봉투 하나의 잠금을 함께 푸는 방법이다.

---

# 2. 직관적인 비유

## 2.1 AuditRecord는 감사용 영수증이다

Merge transaction이 다음과 같다고 하자.

```text
실제 입력:
    Note A
    Note B

출력:
    Note C
```

Blockchain에 parent를 그대로 기록하면 누구나 `A+B→C`를 알 수 있다. zkDPP는 parent를 암호화해 AuditRecord에 넣는다.

```text
AuditRecord #17 {
    policyRef: Merge-v1,
    outputRefs: [NoteRef(C)],
    encryptedParents: Encrypt([NoteRef(A), NoteRef(B)])
}
```

## 2.2 Threshold Encryption은 K개의 잠금장치다

5명의 위원 중 3명이 협력해야 열 수 있는 3-of-5 구조를 생각한다.

```text
위원 1명: 열 수 없음
위원 2명: 열 수 없음
위원 3명: 열 수 있음
```

## 2.3 Partial Decryption은 열쇠 조각을 주는 것이 아니다

각 위원은 고정된 secret share를 보관한다.

```text
위원 1: secret share x1
위원 2: secret share x2
위원 3: secret share x3
위원 4: secret share x4
위원 5: secret share x5
```

위원은 `x1`, `x2`, `x3`를 Authority에게 전달하지 않는다. 특정 ciphertext `C17`에 대한 결과만 전달한다.

```text
위원 1: PartialDecrypt(x1, C17) → d1,17
위원 2: PartialDecrypt(x2, C17) → d2,17
위원 3: PartialDecrypt(x3, C17) → d3,17
```

Authority는 `d1,17`, `d2,17`, `d3,17`을 결합해 `C17`의 plaintext만 얻는다.

```text
Combine(d1,17, d2,17, d3,17)
    → [NoteRef(A), NoteRef(B)]
```

이 결과로 다른 ciphertext `C18`을 열 수는 없다. `C18`을 열려면 K명에게 새 partial decryption을 받아야 한다.

---

# 3. 기존 master-key 복구 방식과의 차이

## 3.1 Master secret key를 복구하는 방식

```text
위원 K명이 secret share 자체를 전달
    → Authority가 master sk 복구
    → 모든 ciphertext 복호화 가능
    → Audit 종료 후 sk 폐기가 필요
```

이 방식의 장점은 Authority가 Committee와 한 번만 통신한 뒤 DAG 전체를 빠르게 복호화할 수 있다는 것이다.

하지만 Authority가 master `sk`를 정말 폐기했는지 확인할 수 없다. Authority가 복사본을 보관하면 과거와 미래의 모든 AuditRecord를 K명의 추가 동의 없이 복호화할 수 있다.

## 3.2 Ciphertext별 Threshold Partial Decryption

```text
위원은 secret share를 전달하지 않음
    → 특정 ciphertext에 대한 partial decryption만 전달
    → Authority가 K개를 결합
    → 해당 ciphertext의 plaintext만 복원
```

위원회가 DAG를 직접 복원할 필요는 없다. Authority가 복원된 parent를 받아 `producerOf` 조회와 다음 ciphertext 선택을 수행한다.

## 3.3 비교

| 항목 | Master sk 복구 | Partial Decryption |
|---|---|---|
| Authority가 master sk를 얻는가? | 예 | 아니오 |
| 위원이 secret share를 보내는가? | 예 | 아니오 |
| 다른 ciphertext 복호화 | Authority 혼자 가능 | 매번 K명 협력 필요 |
| Committee 통신 | Audit 시작 시 1회 | Ciphertext batch/graph level별 |
| 키 폐기 신뢰 가정 | 필요 | 불필요 |
| 감사 속도 | 빠름 | 상대적으로 느림 |
| Privacy 경계 | Authority 신뢰 | K-of-N 유지 |

---

# 4. zkDPP에서 AuditRecord가 사용되는 전체 흐름

## 4.1 Setup

감사 Committee를 고정하고 Threshold key setup/DKG를 한 번 수행한다.

```text
Public:
    committeeKeyId
    committee public key PK
    threshold K
    member count N

Private:
    member 1 share x1
    member 2 share x2
    ...
    member N share xN
```

Committee public key와 version은 Registry에 등록한다. 각 AuditRecord는 어떤 committee key를 사용했는지 `committeeKeyId`를 binding해야 한다.

## 4.2 정상 Event 실행

Merge `A+B→C`를 예로 든다.

```text
Private witness:
    Note A
    Note B
    Merkle paths
    owner secret
    encryption randomness

Public:
    accepted root
    nullifiers
    output cm_C
    committeeKeyId
    encryptedParents
```

Operator/Prover는 parent를 Committee public key로 암호화하고 transition proof를 생성한다.

```text
encryptedParents = TE.Enc(PK, [NoteRef(A), NoteRef(B)]; r)
```

Circuit은 다음을 함께 검사한다.

```text
1. A, B가 실제 Note opening과 일치한다.
2. A, B가 accepted root에 속한다.
3. nullifier가 올바르다.
4. State transition A+B→C가 올바르다.
5. encryptedParents의 plaintext가 실제 A,B이다.
```

Contract는 proof를 검증한 후 AuditRecord를 저장한다.

```text
AuditRecord #17 {
    committeeKeyId: 1,
    policyRef: Merge-v1,
    outputRefs: [NoteRef(C)],
    encryptedParents: ciphertext17
}
```

## 4.3 Audit 시작

Authority가 문제가 의심되는 `AuditRecord #17`을 식별한다.

```text
DecryptionRequest {
    caseId,
    auditRecordId: 17,
    ciphertextHash,
    committeeKeyId,
    authoritySignature
}
```

각 위원은 요청이 자신의 감사 Policy에 맞는지 확인한 후 partial decryption을 만든다.

```text
PartialDecryption {
    caseId,
    auditRecordId,
    memberId,
    shareValue,
    correctnessProof
}
```

`correctnessProof`는 위원이 등록된 자신의 secret share를 사용해 올바른 partial decryption을 만들었다는 증거다.

## 4.4 Authority가 parent 복원

Authority는 올바른 partial decryption K개를 결합한다.

```text
Combine(d1,17, d2,17, d3,17)
    → [NoteRef(A), NoteRef(B)]
```

Authority는 parent 각각의 producer를 조회한다.

```text
producerOf[NoteRef(A)] = AuditRecord #3
producerOf[NoteRef(B)] = AuditRecord #7
```

그런 다음 `AuditRecord #3`, `#7`의 ciphertext를 다시 Committee에 요청한다. Entry에 도달할 때까지 반복하면 backward provenance DAG가 복원된다.

---

# 5. Threshold ElGamal 기반 구체적 스킴

## 5.1 기본 설정

위수 $q$ 위의 group $\mathbb{G}$, generator $G$를 사용한다.

Committee의 master secret은 $x$, public key는 다음과 같다.

\[
PK=xG.
\]

Shamir polynomial $f$를 사용해:

\[
f(0)=x,
\qquad
x_i=f(i)
\]

로 각 위원 (i)의 secret share (x_i)를 만든다. 위원의 public verification key는:

\[
VK_i=x_iG
\]

로 둘 수 있다.

## 5.2 ElGamal Encryption

Operator는 암호화 randomness (r)을 새로 선택한다.

Parent plaintext를 group message (M)으로 encoding했다고 가정하면:

\[
C_1=rG,
\]

\[
C_2=M+rPK.
\]

Ciphertext는:

\[
C=(C_1,C_2)
\]

이다.

Circuit은 private (M,r)과 public (C_1,C_2,PK)에 대해 위 두 식이 성립함을 증명한다.

> 주의: 긴 parent byte list를 group message (M)으로 직접 encoding하는 방식은 실제 구현에서 제약이 많다. 실제 POC에서는 6장의 hybrid encryption을 우선 검토한다.

## 5.3 Partial Decryption 생성

Ciphertext (C=(C_1,C_2))에 대해 위원 (i)는:

\[
D_i=x_iC_1
\]

를 계산한다.

위원은 (x_i)를 보내지 않고 (D_i)만 보낸다. 또한 다음 두 discrete-log 관계가 같은 (x_i)를 사용했다는 Chaum–Pedersen 계열의 correctness proof를 함께 제공할 수 있다.

\[
VK_i=x_iG,
\qquad
D_i=x_iC_1.
\]

## 5.4 K개 partial decryption 결합

선택한 위원 집합을 (S), 각 위원의 Lagrange coefficient를 λᵢ라고 하면:

\[
D=\sum_{i\in S}\lambda_iD_i.
\]

그러면:

\[
D=xC_1=xrG=rPK.
\]

Authority는:

\[
M=C_2-D
\]

로 해당 ciphertext의 plaintext (M)만 복원한다.

Master secret (x)는 복원되지 않는다.

## 5.5 다른 ciphertext에는 재사용할 수 없다

다른 ciphertext (C'=(C'_1,C'_2))는 다른 randomness (r')을 사용한다.

\[
C'_1=r'G.
\]

그러므로 위원은 새로운 partial decryption을 계산해야 한다.

\[
D'_i=x_iC'_1.
\]

(D_i)는 (C')에 사용할 수 없다.

---

# 6. 실제 parent list를 위한 Hybrid Encryption

## 6.1 왜 Hybrid Encryption이 필요한가?

ElGamal 같은 public-key encryption으로 긴 parent list 전체를 암호화하는 것은 비싸고 message encoding이 불편하다.

그래서 AuditRecord마다 작은 random symmetric key (k)를 만든다.

```text
Operator/Prover:
    k ← random 256-bit key
```

그런 다음:

```text
encryptedParentList = SymmetricEncrypt(k, Encode(actualParents))
encryptedKey        = ThresholdEncrypt(PK, k)
```

AuditRecord에는 다음을 저장한다.

```text
AuditRecord {
    ...
    encryptedKey,
    encryptedParentList,
    encryptionNonce,
    committeeKeyId
}
```

## 6.2 직관적인 비유

```text
Parent 목록       = 큰 택배 상자
Symmetric key k    = 택배 상자 열쇠
Threshold Encrypt = 작은 열쇠를 보관하는 K-of-N 공동금고
```

K명은 큰 상자를 직접 여는 것이 아니라, 상자 열쇠 (k)를 공동으로 복원한다. Authority가 (k)로 parent list를 복호화한다.

## 6.3 Circuit이 검증할 관계

Circuit private witness:

```text
actualParents
k
threshold-encryption randomness r
symmetric-encryption nonce/randomness
```

Circuit public input/output:

```text
committeeKeyId
encryptedKey
encryptedParentList
encryptionNonce
```

Circuit은 다음을 증명한다.

\[
encryptedParentList
=
SymmetricEncrypt(k,Encode(actualParents)),
\]

\[
encryptedKey
=
ThresholdEncrypt(PK,k;r).
\]

또한 `actualParents`는 해당 transition에서 실제로 소비한 private Note/Voucher와 같아야 한다.

## 6.4 Audit 시

```text
K명의 partial decryption
    → encryptedKey에서 k 복원
    → k로 encryptedParentList 복호화
    → actualParents 확인
```

> 중요: `SymmetricEncrypt` 방식은 아직 미확정이다. AES-GCM과 ChaCha20-Poly1305는 일반 시스템에서 효율적이지만 ZK Circuit에서는 비싼 편이다. Poseidon/GMiMC 계열의 circuit-friendly authenticated encryption 또는 별도 proof 구조를 검토해야 한다.

---

# 7. Circuit이 Encryption을 검증해야 하는 이유

## 7.1 Decryption 성공은 provenance 정확성을 뜻하지 않는다

악의적 Operator가 실제로 A,B를 소비하고 X,Y를 암호화할 수 있다.

```text
실제 transition:
    A+B → C

AuditRecord:
    encryptedParents = Encrypt([X,Y])
```

나중에 ciphertext를 올바른 key로 복호화하면 X,Y가 정상적으로 나온다. 하지만 X,Y는 실제 input이 아니다.

```text
Decryption 성공
    ≠
암호화한 parent가 실제 input과 같음
```

## 7.2 Transition proof가 둘을 묶어야 한다

```text
실제 private input
    ==
암호문의 private plaintext
```

이 관계를 증명하는 것이 Circuit의 역할이다. Circuit은 복호화를 수행하지 않는다. 암호화가 올바르게 수행되었는지를 검증한다.

---

# 8. DAG 복원 시 Interaction과 성능

다음을 정의한다.

- (V): 방문하는 AuditRecord 수
- (K): 복호화에 필요한 위원 수
- (L): DAG의 최대 깊이

## 8.1 전체 계산·통신량

총 partial decryption 개수는:

\[
K\cdot V
\]

에 비례한다.

전체 통신 데이터도:

\[
O(KV)
\]

이다.

## 8.2 Level별 Batch

같은 DAG level에서 알게 된 ciphertext는 한 번에 요청할 수 있다.

```text
Round 1:
    [C1]

Round 2:
    [C2, C3]

Round 3:
    [C4, C5, C6]
```

Batch를 사용하면 network round-trip은 대략:

\[
O(L)

\]

로 줄일 수 있다. 계산량은 여전히 (K\cdot V)에 비례한다.

## 8.3 Committee가 DAG를 복원하지 않는다

```text
Committee:
    요청받은 ciphertext batch에 대한 partial decryption 생성

Authority:
    K개 share 검증/결합
    plaintext parent 복원
    producerOf 조회
    다음 ciphertext batch 선택
    DAG 구성
```

---

# 9. Security·Governance 조건

## 9.1 감사 요청은 사건에 binding되어야 한다

위원이 Authority의 모든 요청에 무조건 응답하면 Authority는 사실상 전체 원장을 순차적으로 복호화할 수 있다.

따라서 partial decryption은 다음 요청에 binding해야 한다.

```text
caseId
auditRecordId
ciphertextHash
committeeKeyId
Authority authorization
request timestamp/nonce
```

각 위원은 요청과 응답을 감사 log에 남겨야 한다.

## 9.2 잘못된 partial decryption을 막아야 한다

위원은 partial decryption과 함께 correctness proof를 제공해야 한다. Authority는 등록된 member verification key로 이 proof를 검증한다.

## 9.3 K명 담합은 막을 수 없다

K명 이상의 위원이 담합하면 ciphertext를 복호화할 수 있다. 이것은 Threshold Encryption의 기본 trust assumption이다.

## 9.4 Availability도 필요하다

K명이 응답하지 않으면 감사를 진행할 수 없다. N과 K는 보안뿐 아니라 가용성을 고려해 정해야 한다.

## 9.5 Key rotation

AuditRecord는 사용한 `committeeKeyId`를 저장해야 한다. 위원 교체 후 과거 ciphertext를 어떻게 복호화할지 다음 중 하나를 정해야 한다.

```text
과거 committee share 보관
ciphertext re-encryption
epoch별 key 유지
key resharing/refresh
```

---

# 10. 검토 가능한 성능 근거

현재 zkDPP와 완전히 같은 `gnark 0.15 + PLONK-KZG + BLS12-381 + Threshold ElGamal + encryptedParents`의 공개 benchmark는 확인하기 어렵다. 따라서 정확한 비용은 반드시 POC microbenchmark로 측정해야 한다.

다만 비교 근거로 사용할 수 있는 결과는 다음과 같다.

## 10.1 Circuit 내 ElGamal relation

Jubjub curve/R1CS 기준의 한 연구는 ElGamal 암호화 일관성 관계에 최대 약 1,768 constraints가 필요하다고 분석했다. 이 결과는 현재 gnark PLONK constraint와 직접 동일하지 않지만 public-key encryption relation의 대략적인 규모를 보여준다.

- <https://eprint.iacr.org/2023/097.pdf>

## 10.2 Circuit 내 symmetric encryption

같은 연구의 1KB plaintext 기준 constraint는 다음과 같다.

| 방식 | 1KB constraints |
|---|---:|
| AES-128 CTR | 748,694 |
| Poseidon sponge | 4,020 |
| GMiMC CTR | 1,128 |
| GMiMC sponge | 4,512 |

따라서 일반 AES를 Circuit에 직접 넣는 것보다 circuit-friendly primitive를 검토해야 한다. Parent 목록은 보통 1–3개로 1KB보다 작지만, 정확한 constraint는 직접 측정해야 한다.

## 10.3 Native Threshold Encryption 참고값

`Threshold Encryption with Silent Setup`의 구현 평가는 다음을 보고한다.

```text
Encryption: < 7 ms
Partial decryption: < 1 ms
1,024 partial-decryption aggregation: < 200 ms
Ciphertext size: ElGamal의 약 8배
```

이 스킴은 pairing 기반이고 zkDPP Circuit 내 Threshold ElGamal과 동일하지 않다. Audit-side native 비용이 실용적일 수 있다는 참고로만 사용한다.

- <https://eprint.iacr.org/2024/263.pdf>

## 10.4 재현 가능한 구현 참고

- Threshold ElGamal 개념 및 3-of-5 사용 예: <https://threshold-elgamal.readthedocs.io/en/latest/index.html>
- Verifiable distributed decryption 구현 참고: <https://github.com/slowli/elastic-elgamal>
- ElGamal 기반 threshold decryption/hybrid encryption 참고: <https://github.com/tompetersen/threshold-crypto>

위 repository들은 생산 환경용으로 보안 검증되지 않은 경우가 있으므로 API와 실험 구조 참고용으로만 사용한다.

---

# 11. zkDPP POC 권장 실험

## 11.1 기능 조합

```text
Committee:
    고정 3-of-5

Encryption:
    BLS12-381 scalar field와 맞는 native twisted-Edwards curve 후보
    AuditRecord별 hybrid encryption

Decryption:
    ciphertext별 partial decryption
    DAG level별 batch request

DAG reconstruction:
    Status Authority가 수행
```

## 11.2 Component benchmark

```text
Native:
    Key setup/DKG
    Encryption
    PartialDecrypt 1개
    Partial share proof verification
    K-share combine
    Symmetric decrypt

Circuit:
    EC ElGamal relation constraints
    Parent-list encryption constraints
    Full Event constraints 증가량
    Prove/verify time
    Proof/public input/ciphertext bytes

EVM:
    Ciphertext calldata bytes
    AuditRecord storage/Event gas
    Event total gas

Audit E2E:
    K = 2,3,4
    V = 10,100,1000
    DAG depth L = 3,10,30
    batch/no-batch latency
```

## 11.3 비교 실험

```text
Baseline:
    기존 Event Circuit

Variant A:
    Event + direct parent encryption relation

Variant B:
    Event + hybrid encryption relation

Audit mode 1:
    master sk reconstruction

Audit mode 2:
    ciphertext-level partial decryption
```

Master-sk mode는 성능 상한선이지만 키 폐기 신뢰 가정이 필요하다. Partial-decryption mode는 추가 interaction을 지불하고 threshold privacy를 유지한다.

---

# 12. 미확정 항목

구현 전 다음을 확정해야 한다.

- [ ] Threshold scheme과 security model
- [ ] DKG / centralized test setup / key resharing
- [ ] Committee (K,N)
- [ ] Curve: Jubjub/BLS12-381 twisted Edwards/Bandersnatch 기타
- [ ] Parent AuditRef encoding과 최대 parent 수
- [ ] Direct ElGamal 또는 hybrid encryption
- [ ] Circuit-friendly symmetric authenticated encryption
- [ ] KDF, nonce, domain separation
- [ ] Partial decryption correctness proof
- [ ] Decryption request authorization과 audit log
- [ ] Batch partial-decryption API
- [ ] Committee key rotation과 과거 ciphertext 처리
- [ ] Ciphertext storage 또는 Event log encoding
- [ ] Status Authority가 복원한 plaintext의 보관/폐기 정책

---

# 13. 최종 정리

```text
Threshold Encryption:
    Event 실행 시 parent를 잠그는 기능

Circuit binding:
    잠긴 parent가 실제 transition input과 같음을 증명

Threshold Partial Decryption:
    Audit 시 K명이 master sk를 내주지 않고
    특정 ciphertext의 잠금만 함께 푸는 기능

Status Authority:
    partial decryption을 결합해 parent를 복원하고
    producerOf를 따라 DAG를 구성하는 주체
```

이 설계의 가장 중요한 보안 차이는 다음이다.

```text
Master-sk reconstruction:
    Authority가 모든 ciphertext를 열 수 있는 장기 키를 얻음

Partial decryption:
    Authority는 K명이 승인한 ciphertext의 plaintext만 얻음
```
