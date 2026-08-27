# zkDPP Audit 설계에 대한 추가 논의 사항

## 문서 목적

이 문서는 기존 `zkDPP 비공개 provenance 감사와 선택적 동결 설계`를 함께 검토한 후, 제가 추가로 궁금해진 부분과 현재 이해한 내용을 공유하기 위한 메모입니다.

기존 audit design을 다시 설명하는 것은 목적이 아닙니다. 서로 같게 이해하고 있다고 생각하는 내용은 전제로 두고, 구현 전에 추가로 논의해야 할 문제를 정리했습니다.

---

## 1. 먼저 논의하고 싶은 문제

남은 문제는 아래 순서로 연결된다고 생각했습니다.

### Policy·Privacy 문제

**Document 표현 → Circuit의 검증 범위 → 온체인에 노출되는 정보**

1. Process Circuit이 State transition만 검증할지, material·quantity·unit까지 검증할지 정해야 합니다.
2. Circuit의 검증 범위가 정해져야 PolicyRef·VK를 통해 어떤 정보가 노출되는지 판단할 수 있습니다.

### Private provenance·Freeze 문제

**Encrypted DAG 구조 → Decryption 방식 → Frozen/Revoked 조회 방식**

1. 과거 문제 transaction에서 현재 downstream leaf까지 이동할 수 있는 DAG 구조가 필요합니다.
2. DAG를 어떻게 복호화할지 정해야 Auditor의 신뢰 범위와 감사 비용을 판단할 수 있습니다.
3. 현재 leaf를 찾은 후 private Note의 `Active/Frozen/Revoked` 상태를 어떻게 조회할지 정해야 합니다.

### Claim·DPP 문제

**공급망 중간 Claim 필요성 → Issue의 의미 → zkDPP의 DPP 범위**

1. 최종 제품뿐 아니라 공급망 중간 원자재에도 Claim을 발행할 수 있어야 하는지 정해야 합니다.
2. Claim 발행의 역할을 정한 후, 현재 Protocol을 DPP 자체로 볼지 DPP의 cryptographic layer로 볼지 논의해야 합니다.

---

## 2. 제가 생각한 물품 표현

아래는 이후 질문을 이해하기 위해 제가 생각한 물품 표현을 먼저 정리한 것입니다.

$$
Document=(DocumentHash,MergeProfile,quantity,unit,state)
$$

$$
state=(total\_kg,carbon\_footprint\_kgCO2e,recycled\_kg)
$$

| 항목 | 제가 생각한 의미 |
|---|---|
| `DocumentHash` | 물품 metadata를 hash해 binding하는 값입니다. |
| `MergeProfile` | 어떤 종류의 물품인지 나타내고, 기본적으로 같은 값을 가진 물품끼리 Merge할 수 있게 하는 값입니다. |
| `quantity` | 물품의 개수입니다. |
| `unit` | `cell`, `coil`, `container` 같은 quantity의 단위입니다. |
| `state` | Sustainability Claim을 주장하거나 Event 간 consistency를 검증하기 위한 Numeric State입니다. |

예를 들어 `quantity=10`, `unit=cell`이면 cell 10개를 뜻합니다. 이 물품을 Split할 때는 quantity와 함께 State도 각 output에 맞게 분배됩니다.

`MergeProfile`은 아직 추상적인 값입니다. Merge compatibility를 검증하는 데 사용할 수 있고, Process에서도 특정 input/output material을 강제하는 데 사용할 수 있다고 생각했습니다.

이 문서에서는 Carbon과 Recycled 기준을 함께 검증하는 하나의 Claim을 사용한다고 가정합니다. Policy마다 별도 Circuit/VK가 있고, input Note/Voucher commitment는 private witness, output commitment와 nullifier는 public이라고 가정합니다.

---

## 3. Process Circuit은 어디까지 검증해야 하나요?

**논의하고 싶은 내용: Process별 Circuit이 State transition·allocation만 검증하면 되는지, 아니면 input/output material의 종류·quantity·unit까지 강제해야 하는지 논의하고 싶습니다.**

현재 POC의 고정된 Circuit은 input State를 aggregate하고, 선언된 delta와 allocation에 따라 output State가 올바르게 계산되었는지 검증합니다. 이 구조에서는 Certificator가 매 Process마다 State 분배를 직접 확인하지 않아도 Circuit이 계산 일관성을 검증할 수 있습니다.

하지만 Circuit이 input/output material, quantity, unit을 검증하지 않으면 다음과 같은 문제가 생길 수 있다고 생각했습니다.

- 실제로 존재하지 않거나 의미 없는 물품 commitment를 생성할 수 있습니다.
- input material이 필요한 material type인지 확인하지 않고 Process할 수 있습니다.
- input/output quantity를 실제 공정 규칙과 다르게 부풀릴 수 있습니다.
- Carbon/Recycled State를 의미 없는 output에 배정할 수 있습니다.

Entry와 Process는 둘 다 새로운 물품 commitment를 생성합니다. 따라서 물품의 종류와 생성 규칙을 누가 승인하는지, Certificator의 신뢰가 어떤 범위까지 필요한지도 같이 정해야 합니다.

제가 이해한 Process별 VK의 역할은 다음과 같습니다.

- Prover가 필요한 Process Policy를 선택합니다.
- Registry에 승인된 Policy/Circuit/VK가 있으면 그 VK를 사용합니다.
- 필요한 Policy가 없다면 Certificator가 Circuit을 검토하고 승인한 뒤 Registry에 추가합니다.

어떤 값을 Circuit이 검증하고 어떤 값을 Certificator·Operator의 신뢰에 맡길지 정해야 Process Circuit의 constraint와 Policy Registry의 역할을 확정할 수 있습니다.

---

## 4. 무엇을 숨겨야 하나요?

**논의하고 싶은 내용: Policy/Circuit이 material 종류와 Recipe를 검증할 때, 온체인 관찰자에게 어느 수준까지의 정보 노출을 허용할지 논의하고 싶습니다.**

제가 이해한 Policy에는 다음과 같은 규칙이 들어갈 수 있다고 생각했습니다.

| 구분 | Policy가 검증할 수 있는 내용 |
|---|---|
| Input material | A, B, C |
| Input ratio | 1 : 2 : 1 |
| Output material | D, E |
| Output ratio | 2 : 3 |

이 Policy와 Circuit/VK가 1:1로 대응하고 Policy의 의미가 알려져 있다면, 온체인에 공개되는 output commitment가 D/E 계열의 물품이라는 정보를 추론할 수 있어 보입니다.

Policy 이름을 무작위 숫자나 hash로 바꾸더라도, Policy별로 다른 verifier address/VK를 사용한다면 동일한 verifier를 사용한 transaction들을 동일한 Policy/Recipe family로 묶을 수 있는지도 궁금합니다.

제가 구분하고 싶은 Privacy 범위는 다음과 같습니다.

| Privacy 범위 | 숨기고자 하는 정보 |
|---|---|
| Numeric value | quantity, carbon, recycled value |
| Business relation | 정확한 batch와 거래 edge |
| Material type | input/output 물품 종류 |
| Recipe/Policy | 공정·혼합·분배 규칙 |

Circuit이 어느 정보까지 검증할지 정한 후, 이 중 어느 범위까지 숨길 수 있는지와 어느 정보는 Policy fingerprint로 남는지 확인해야 합니다.

---

## 5. Encrypted DAG 전체를 온체인에 저장하면 forward tracing이 가능한가요?

**논의하고 싶은 내용: Private provenance를 유지하면서 과거 문제 transaction으로부터 현재 사용 가능한 downstream leaf Note를 찾으려면, 어떤 forward 정보를 암호화해 온체인에 저장해야 하는지 논의하고 싶습니다.**

기존 `encryptedParents`는 child AuditRecord에서 parent AuditRef를 복원하는 방향의 정보를 제공합니다. 복원한 parent AuditRef로 `producerOf` Index를 조회하면 이전 AuditRecord로 거슬러 올라갈 수 있습니다.

하지만 우리가 Freeze하고자 하는 대상은 과거의 이미 소비된 output이 아니라, 그 output에서 파생된 현재 unspent downstream leaf입니다. 따라서 parent에서 시작해 어느 transaction이 그 parent를 소비했는지, 그 transaction이 어떤 child를 만들었는지 알 수 있는 forward 정보가 추가로 필요해 보입니다.

이때 encrypted DAG 전체를 온체인에 저장하는 구조를 생각할 수 있다고 보았습니다. Parent relation뿐 아니라 향후 Authority가 consumer/child relation을 복원할 수 있는 정보도 AuditRecord 또는 별도 암호화 필드로 남길 수 있는지 궁금합니다.

소비하는 input `cm`은 private witness입니다. Contract가 `consumerOf[cm]` 같은 mapping을 직접 기록하려면 input `cm`을 알아야 하므로, private input을 유지하면서 forward relation을 어떤 형태로 binding하고 조회할 수 있는지 확인하고 싶습니다.

---

## 6. Auditor를 신뢰하지 않는 Threshold Decryption 구조가 가능한가요?

**논의하고 싶은 내용: K개의 share로 master secret key를 복구한 Auditor가 그 key를 폐기했다고 신뢰하는 방식 대신, 누구도 master key를 복구하지 않고 승인된 ciphertext만 복호화하는 구조가 가능한지 논의하고 싶습니다.**

### 기존 AuditRecord의 Threshold Encryption

Committee는 public key $PK$와 N개의 secret share를 가집니다. Event를 실행하는 Operator는 실제 parent AuditRef 목록을 Committee public key로 암호화해 AuditRecord에 넣습니다.

Transition Circuit은 암호화된 parent가 실제로 소비한 private input과 같음을 검증합니다. Circuit은 ciphertext를 복호화하는 것이 아니라, 올바른 plaintext가 올바른 key로 암호화되었는지 검증합니다.

### 기존에 생각한 master-key 복구

기존에는 K명의 위원이 자신의 secret share를 Auditor에게 전달하고, Auditor가 master secret key를 복구해 DAG를 연다고 생각했습니다. Audit이 종료되면 Auditor가 master key를 폐기한다고 가정했습니다.

하지만 Auditor가 master key를 정말 폐기했는지 확인할 수 없습니다. 한 번 master key를 얻으면 같은 public key로 암호화된 과거·미래 ciphertext를 K명의 추가 동의 없이 복호화할 수 있어 보입니다.

### Ciphertext별 Threshold Partial Decryption

Threshold ElGamal을 예로 들면 Committee의 master secret을 $x$, public key를 $PK$라고 할 때 다음이 성립합니다.

$$
PK=xG
$$

Operator는 plaintext $M$과 매번 새로 선택한 randomness $r$을 이용해 ciphertext를 만듭니다.

$$
C_1=rG,\qquad C_2=M+rPK
$$

각 위원 $i$는 master key의 고정된 share $x_i$를 보관합니다. 위원은 $x_i$를 Auditor에게 전달하지 않고, 특정 ciphertext에 대한 partial decryption $D_i$만 전달합니다.

$$
D_i=x_iC_1
$$

$D_i$는 plaintext의 일부도 아니고, secret share $x_i$도 아닙니다. 해당 ciphertext의 잠금을 일부만 푸는 암호학적 중간값입니다.

K명의 위원 집합을 $S$, 각 위원의 Lagrange coefficient를 $\lambda_i$라고 하면 Auditor는 K개의 partial decryption을 다음과 같이 결합합니다.

$$
D=\sum_{i\in S}\lambda_iD_i=xC_1
$$

이 ciphertext의 plaintext는 다음과 같이 복원됩니다.

$$
M=C_2-D
$$

Auditor는 $M$만 얻고 master secret $x$는 얻지 못합니다. 다른 ciphertext는 다른 $C_1$을 사용하므로 복호화하려면 K명에게 새 partial decryption을 받아야 합니다.

### DAG 복원 주체와 비용

Committee는 DAG를 직접 복원하지 않습니다. 각 위원은 Auditor가 요청한 ciphertext batch에 대한 partial decryption만 제공합니다. Auditor가 K개를 결합해 parent를 얻고, `producerOf` Index를 조회해 다음 ciphertext를 선택하며 DAG를 구성합니다.

방문하는 AuditRecord 수를 $V$, 복호화에 필요한 위원 수를 $K$, DAG의 최대 깊이를 $L$이라고 하면 partial-decryption 계산·통신량은 대략 $K\times V$에 비례합니다. 같은 DAG level의 ciphertext를 batch로 요청하면 network round-trip은 대략 $L$에 비례할 수 있습니다.

이 방식은 master key 폐기에 대한 신뢰 가정을 없애는 대신, 복호화할 ciphertext의 수와 DAG 깊이에 따라 Committee와의 interaction 비용이 증가합니다.

---

## 7. Frozen/Revoked key-value map은 linkability를 만드나요?

**논의하고 싶은 내용: 항목이 없으면 Active로 보고 Frozen과 Revoked만 기록하는 key-value map으로 private object의 사용 가능 상태를 검증할 수 있는지, 그 lookup key가 input-output linkability를 만들 수 있는지 논의하고 싶습니다.**

제가 생각한 기본 상태 구조는 다음과 같습니다.

| Key 상태 | 의미 |
|---|---|
| 항목 없음 | Active |
| `Frozen` | 조사 중이므로 임시 사용 금지 |
| `Revoked` | 문제가 확인되어 영구 사용 금지 |

이 map은 `cm_A→cm_B`같은 provenance edge를 저장하지 않고, `cm_A=Frozen`과 같은 상태만 저장합니다. 따라서 key-value map 자체가 provenance graph라고 생각하지는 않습니다.

Note가 생성될 때 output `cm_A`가 공개되어도, 나중에 어느 transaction이 `cm_A`를 소비했는지를 숨기는 것은 별도의 문제라고 이해했습니다.

Authority가 public output `cm_A`를 key로 Frozen/Revoked를 기록하는 것은 가능해 보입니다. 하지만 나중에 `cm_A`를 소비하는 transaction에서 `cm_A`는 private witness이므로 Contract와 Circuit이 어떤 key로 현재 status를 조회할지 확인해야 합니다.

만약 소비 transaction에서 생성 시점과 같은 lookup key를 public input으로 제출한다면, 관찰자가 output의 생성 transaction과 현재 소비 transaction을 연결할 수 있는지 검토해야 합니다. 따라서 key-value 구조가 반드시 linkability를 만든다고 단정하기보다, lookup key의 내용과 공개 시점에 따라 linkability가 생길 수 있는지 확인하고 싶습니다.

---

## 8. Issue는 terminal로만 사용해야 하나요?

**논의하고 싶은 내용: 최종 제품이 아닌 공급망 중간 원자재의 구매자도 Carbon/Recycled Claim을 검증할 수 있어야 한다면, Claim 발행 Event가 반드시 terminal이어야 하는지 논의하고 싶습니다.**

현재 Issue는 input Note를 소비하고 Claim을 만드는 Event로 정의되어 있습니다.

$$
Note\rightarrow Claim
$$

새로운 Note를 생성하지 않으므로 Issue 이후에는 해당 물품을 Transfer, Merge, Split, Process할 수 없습니다. 따라서 현재 Issue는 최종 Claim을 발행하는 terminal Event입니다.

하지만 공급망 중간의 원자재를 구매하는 사람도 해당 원자재가 Carbon/Recycled 기준을 만족하는지 확인하고 싶을 수 있습니다. 이 경우에는 Claim을 발행하면서도 이후 Event에서 계속 소비할 수 있는 새 Note를 함께 만드는 구조가 필요한지 궁금합니다.

$$
Note\rightarrow Note'+Claim
$$

이 중간 Claim Event의 이름과 정확한 lifecycle은 아직 정하지 않았습니다. 중간 Claim이 필요한지, 필요하다면 기존 Issue와 어떻게 구분할지 우선 논의하고 싶습니다.

---

## 9. 현재 Protocol 자체를 DPP라고 정의할 수 있나요?

**논의하고 싶은 내용: 현재 설계한 zkDPP Protocol 자체를 논문에서 `DPP`라고 정의하고 사용해도 되는지 논의하고 싶습니다.**

Issue 또는 중간 Claim Event가 제공하는 결과는 Claim handle, ZK proof, PolicyRef/version, Claim status입니다.

제가 궁금한 것은 별도의 전체 DPP Platform과 기능을 비교하는 문제가 아닙니다. 논문에서 제안하는 현재 Protocol 자체를 `DPP`라고 부를 수 있는지, 그렇게 부르려면 DPP의 범위를 논문에서 어떻게 정의해야 하는지가 핵심입니다.

따라서 현재 Protocol이 제공하는 private State transition, Claim, provenance audit 및 상태 관리 기능을 근거로 논문에서 `DPP`라는 용어를 사용할 수 있는지 먼저 정리하고 싶습니다.

---

## 10. 회의에서 먼저 논의하고 싶은 질문

1. **Process별 Circuit은 State transition·allocation 외에 input/output material·quantity·unit까지 강제해야 하나요?**
2. **Policy/Circuit이 material·Recipe를 검증할 때 온체인 관찰자에게 어느 정보까지 노출되는 것을 허용할 수 있나요?**
3. **Encrypted DAG를 온체인에 저장하면서 private provenance를 유지하고 current downstream leaf를 찾으려면 어떤 forward 정보가 필요한가요?**
4. **Master key를 복구한 Auditor를 신뢰하지 않고, ciphertext별 Partial Decryption으로 audit할 수 있나요?**
5. **Frozen/Revoked key-value map으로 private input의 status를 검증하면서 input-output linkability를 막을 수 있나요?**
6. **공급망 중간 원자재에도 Claim을 발행할 수 있어야 하나요?**
7. **현재 zkDPP Protocol 자체를 논문에서 DPP라고 정의하고 사용해도 되나요?**
