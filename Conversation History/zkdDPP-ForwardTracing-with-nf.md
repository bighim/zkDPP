# AuditRecord 확장 제안: downstream leaf를 찾아 동결하기

**$cm$이 소비될 때 사용할 $nf$를 암호화해 AuditRecord에 저장하면, 감사 시 관련 downstream만 따라가 아직 소비되지 않은 $cm$을 찾아 동결할 수 있습니다.**

이 문서는 기존 audit design에 대한 **동료 검토용 변경 제안**입니다. 먼저 Note의 $cm$과 $nf$를 설명하고, 4장에서 소유권 전달 구간을 다룹니다. 이어서 downstream 탐색과 동결 방법을 설명합니다. Auditor는 별도 역할이 아니라 Status Authority의 감사 기능을 가리킵니다.

## 1. 무엇을 바꾸려는 건가요?

**과거를 찾는 기존 기능은 유지하고, 이후 소비를 따라가는 기능을 추가합니다.**

- **기존 backward tracing:** 현재 $cm$이 어떤 부모에서 만들어졌는지 Entry까지 거슬러 올라갑니다.
- **추가할 forward tracing:** 문제 $cm$이 어디에서 소비됐는지 따라가 현재 미소비 leaf들을 찾고 Freeze합니다.

핵심은 **$cm$과 소비 $nf$를 감사 시에만 연결하는 것**입니다. 암호문을 추가하는 것뿐 아니라, 그 $nf$로 소비 기록을 찾는 온체인 mapping도 필요합니다.

## 2. $cm$이 소비될 때 사용할 $nf$를 어떻게 알 수 있나요?

**$cm$과 소유자의 secret이 정해지면 $nf$도 결정됩니다. 다음 Event를 예측할 필요는 없습니다.**

$$
nf=H(sk_{\mathrm{owner}},cm)
$$

$sk_{\mathrm{owner}}$는 Note 소유자의 secret입니다. 설명을 위해 Hash 용도 구분 상수는 생략했습니다.

같은 Note를 Transfer·Merge·Split·Process·Exit 중 어디에서 소비하더라도 같은 $nf$를 사용합니다. 다음 transaction에서 생성될 값을 미리 계산하는 것이 아닙니다.

1. **$cm$ 생성 시:** 그 $cm$이 소비될 때 사용할 $nf$를 계산하고, 암호화해 AuditRecord에 저장합니다.
2. **$cm$ 소비 시:** 같은 $nf$를 공개하고 사용 처리합니다.

**암호문 저장은 소비가 아닙니다.** 생성 시에는 평문 $nf$를 공개하거나 spent 상태를 변경하지 않습니다. 아래의 $\operatorname{Encrypt}(nf)$는 위원회 협조로 복호화하는 암호문을 뜻하며, 구체적인 암호 방식은 여기서 정하지 않습니다.

## 3. AuditRecord에는 무엇이 담기나요?

**부모 참조, 생성된 참조, 암호화된 소비 $nf$를 구분합니다.**

| 필드 | 담긴 내용 | 답하는 질문 |
|---|---|---|
| encryptedParents | 이번 Event가 소비한 부모 참조의 암호문 | 무엇을 소비했나요? |
| outputRefs | 이번 Event가 생성한 객체의 공개 참조 | 무엇을 만들었나요? |
| **encryptedOutputNfs — 추가** | 생성된 $cm$들이 소비될 때 사용할 $nf$의 암호문 | 이후 어디에서 소비되나요? |

**encryptedParents에는 transaction이 들어 있지 않습니다.** $\operatorname{NoteRef}(cm_A)$처럼 **객체 종류와 식별값으로 구성된 참조**가 암호화되어 있습니다.

### Split 예시에서는 어떻게 보이나요?

$$
cm_A \xrightarrow{\mathrm{Split}} (cm_B,cm_C)
$$

이 Split의 AuditRecord는 다음과 같습니다.

| 필드 | 예시 값 |
|---|---|
| encryptedParents | $\operatorname{Encrypt}([\operatorname{NoteRef}(cm_A)])$ |
| outputRefs | $[\operatorname{NoteRef}(cm_B),\operatorname{NoteRef}(cm_C)]$ |
| encryptedOutputNfs | $[\operatorname{Encrypt}(nf_B),\operatorname{Encrypt}(nf_C)]$ |

$\operatorname{Encrypt}(nf_B)$는 $cm_{B}$에, $\operatorname{Encrypt}(nf_C)$는 $cm_{C}$에 대응합니다. **이 기록은 $cm_{A}$를 소비했지만, 암호화해 추가하는 값은 새로 생성한 $cm_{B}$·$cm_{C}$의 소비 $nf$입니다.**

### 다른 nf를 암호화해 넣는 것은 어떻게 막나요?

**$cm_B$를 만드는 Event의 Circuit이, 함께 공개한 암호문에 바로 그 $cm_B$의 소비 $nf_B$가 들어 있음을 증명해야 합니다.**

1. **Note와 소유자 확인:** 비공개 Note를 Hash한 값이 공개된 $cm_B$와 같고, 사용한 $sk_{\mathrm{owner}}$가 그 Note의 소유자 secret인지 확인합니다.
2. **소비 nf 계산:** 같은 secret과 $cm_B$로 $nf_B=H(sk_{\mathrm{owner}},cm_B)$를 계산합니다.
3. **암호문 확인:** 방금 계산한 $nf_B$를 암호화한 결과가 encryptedOutputNfs의 $cm_B$에 대응하는 공개 암호문과 같은지 확인합니다.

Circuit이 암호문을 복호화하는 것은 아닙니다. **직접 계산한 $nf_B$를 암호화한 결과와 공개 암호문을 비교**합니다. 이 과정을 영지식으로 증명하므로 소유자 secret과 평문 $nf_B$는 공개되지 않습니다.

이 검사가 없으면 참가자가 엉뚱한 값을 암호화해 넣어, 감사자가 나중에 소비 기록을 찾지 못하게 만들 수 있습니다.

기존 필드와 producerOf는 유지합니다. 암호문 구성, 직렬화와 proof 입력 형식은 후속 설계로 남깁니다.

## 4. Transfer의 Voucher는 어떻게 이어지나요?

**Note의 $cm$은 $nf$로, Voucher의 $rv$는 $rvnf$로 소비됩니다.**

Transfer는 기존 $cm_{\mathrm{input}}$을 $nf_{\mathrm{input}}$으로 소비하고, Sender의 Change Note와 Voucher를 만듭니다.

$$
cm_{\mathrm{input}} \xrightarrow{\mathrm{Transfer}} (cm_{\mathrm{change}},rv)
$$

- **Change Note:** Sender가 자신의 secret으로 $nf_{\mathrm{change}}$를 계산해 encryptedOutputNfs에 암호화해 저장합니다.
- **Voucher:** Sender와 Receiver가 공유하는 opening으로 소비 $rvnf$를 계산합니다.

$$
rvnf=H(o_{\mathrm{rv}},rv)
$$

$o_{\mathrm{rv}}$는 공유 opening이며 위 식도 용도 구분 상수를 생략했습니다. Sender는 Voucher 생성 시 $rv$와 $o_{\mathrm{rv}}$를 알므로 $\operatorname{Encrypt}(rvnf)$를 준비할 수 있습니다. 이 암호문도 AuditRecord에서 해당 Voucher 참조와 대응되도록 저장합니다.

**Transfer Circuit도 같은 검사를 수행합니다.** 공개된 $rv$가 해당 Voucher의 commitment인지 확인하고, 그 Voucher의 공유 opening으로 $rvnf$를 계산한 뒤, 바로 그 $rvnf$를 암호화한 결과가 공개 암호문과 같은지 확인합니다.

### Proceed와 Recall 중 어느 방향으로 이동하나요?

- **Proceed:** $rv$를 소비하고 Receiver의 새로운 Note를 만듭니다.
- **Recall:** 같은 $rv$를 소비하고 Sender의 새로운 Note를 만듭니다.

두 Event는 **같은 $rvnf$**를 사용합니다. 먼저 성공한 Event의 AuditRecord ID를 voucherSpentIn에 기록하며, 조회 key는 $rvnf$입니다. Auditor는 이 mapping으로 실제 선택된 방향을 찾습니다.

Voucher 상태는 Note와 분리한 voucherStatusByNf로 관리합니다. 소비 시 공개된 $rvnf$로 미소비 여부와 Active 상태를 검사합니다.

| 생성된 값 | 소비할 때 사용할 값 | 계산 주체 |
|---|---|---|
| Change Note $cm_{\mathrm{change}}$ | $nf_{\mathrm{change}}$ | Sender |
| Voucher $rv$ | $rvnf$ | 공유 opening을 아는 Sender |
| Proceed의 $cm_{\mathrm{receiver}}$ | $nf_{\mathrm{receiver}}$ | Receiver |
| Recall의 $cm_{\mathrm{return}}$ | $nf_{\mathrm{return}}$ | Sender |

**Sender는 Receiver의 소유자 secret을 몰라도 $rvnf$를 준비할 수 있습니다.** 그 뒤 Proceed에서는 Receiver가, Recall에서는 Sender가 새 $cm$의 소비 $nf$를 준비하므로 소유권 전달 구간에서도 downstream 연결을 이어갈 수 있습니다.

## 5. 어떻게 마지막 leaf까지 따라가나요?

**생성 기록은 producerOf로, 소비 기록은 noteSpentIn으로 찾습니다.**

| Mapping | Key | 찾는 값 |
|---|---|---|
| producerOf — 기존 | 객체 참조 key | 그 객체를 생성한 AuditRecord ID |
| **noteSpentIn — 추가** | 소비 $nf$ | 그 $nf$를 소비한 AuditRecord ID |

두 mapping 모두 사람이나 회사가 아니라 **transaction에 대응하는 AuditRecord**를 가리킵니다. 소비 기록 mapping의 이름은 noteSpentIn으로 줄여 사용합니다.

다음과 같이 두 번 Split했다고 하겠습니다.

$$
cm_A \xrightarrow{\mathrm{Split}} (cm_B,cm_C)
$$

$$
cm_B \xrightarrow{\mathrm{Split}} (cm_D,cm_E)
$$

$cm_{C}$, $cm_{D}$, $cm_{E}$는 아직 소비되지 않았다고 가정합니다. 그러면 이 세 Note가 동결 대상 leaf입니다.

### $cm_{A}$에서 시작하는 조회 순서

1. **생성 기록 찾기:** producerOf로 $cm_{A}$가 생성된 AuditRecord를 찾습니다.
2. **소비 $nf$ 복원:** 그 기록의 encryptedOutputNfs에서 $cm_{A}$에 대응하는 암호문을 위원회 협조로 복호화하여 $nf_{A}$를 얻습니다.
3. **소비 기록 찾기:** noteSpentIn에 $nf_A$를 넣어 $cm_A$를 소비한 AuditRecord를 찾습니다.
4. **다음 $cm$으로 이동:** 해당 기록의 outputRefs에서 $cm_{B}$·$cm_{C}$를 읽고 반복합니다.

조회 결과에 따라 두 가지로 나뉩니다.

- **소비 기록 없음:** 조회 시점에 미소비인 $cm$입니다. 복원한 $nf$를 동결 대상으로 삼습니다.
- **소비 기록 있음:** 그 Event가 만든 다음 $cm$들을 따라갑니다.

AuditRecord ID는 1부터 사용합니다. noteSpentIn에서 $nf$로 조회한 값이 0이면 소비 기록이 없다는 뜻입니다. 이를 신뢰하려면 모든 정상 소비가 **같은 transaction에서 소비 기록 mapping을 빠짐없이 갱신**해야 합니다.

Exit처럼 소비 기록은 있지만 다음 $cm$이 없는 경우는 **종료된 경로**입니다. 동결할 미소비 leaf가 아닙니다.

## 6. $nf$로 동결하면 무엇이 단순해지나요?

**Status Authority가 복호화한 $nf$를 공개해 동결하면, Contract는 소비 시 이미 공개되는 $nf$로 상태를 직접 검사합니다.**

| noteStatusByNf의 조회값 | 의미 |
|---|---|
| 항목 없음 / 0 | Active |
| 1 | Frozen |
| 2 | Revoked |

소비 Contract는 **미소비 여부 → Active 여부 → Event proof**를 검사합니다. Active여도 이미 소비됐다면 거부합니다. 동결 해제는 소비 기록을 지우는 행위가 아닙니다.

상태 변경 권한은 Status Authority에게만 있습니다. 기존의 **Active→Frozen, Frozen→Active, Frozen→Revoked** 전이는 유지합니다.

| 제거 가능 | 계속 필요 |
|---|---|
| 별도 StatusTree와 Active Status path | 기존 객체 membership Tree와 소유권·nullifier 검증 |
| StatusUpdate ZKP | 생성 Circuit에서 해당 $cm$의 소비 $nf$를 올바르게 암호화했음을 증명 |
| Status path 제공용 Indexer | 감사 프로그램과 위원회 복호화 처리 |

StatusTree용 외부 서비스는 제거할 수 있지만 모든 오프체인 기능이 사라지는 것은 아닙니다. 생성 Circuit에서 위 계산을 증명하는 비용과 암호문 저장 비용이 추가되므로 **전체 비용이 반드시 줄어든다고 단정하지 않습니다.**

### 어떤 Privacy를 허용하나요?

- **일반 관찰자:** 동결 때 공개된 $nf$가 Unfreeze 후 소비되면, 그 상태 변경과 소비 transaction을 연결할 수 있습니다. 대상 $cm$까지 알려졌다면 $cm$과 소비의 연결도 드러납니다.
- **다음 $cm$:** 그 다음 $cm$들의 소비 연결은 자동으로 드러나지 않습니다. 각각의 암호문을 추가 복호화해야 합니다.
- **Auditor:** 새 암호문을 복호화할 때는 해당 암호문에 대한 **$K$개의 partial decryption**이 필요합니다. Master secret key를 받지는 않습니다.
- **이미 복원한 $nf$:** 소비 여부를 계속 관찰할 때는 추가 복호화가 필요 없습니다. 감사 종료 후에도 이미 알게 된 $nf$를 잊도록 강제할 수는 없습니다.

### 무엇을 더 논의해야 하나요?

1. **Entry 소유자 참여:** 생성자가 소유자 secret을 모르는 경우, 소유자가 $nf$ 암호문과 proof 생성에 참여하는 절차가 필요합니다.
2. **조회와 Freeze의 경쟁:** 조회 후 먼저 소비될 수 있습니다. 미소비 확인과 동결을 원자적으로 처리하고, 소비가 먼저 성공하면 다음 기록으로 이어가는 절차를 정해야 합니다.
3. **감사 대상 확인:** Status Authority가 제출한 $nf$가 지정한 $cm$에 대응하는 암호문을 복호화한 값인지 Contract까지 검증할지, Authority의 책임으로 둘지 정해야 합니다.
