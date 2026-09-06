# M7 — 감사 기록을 따라가 어떻게 동결하나요?

- 명세 상태: 구현 기준 동결
- 구현 결과: [M7 Result](M7-audit-tracing-result.md)
- 이전 결과: [M6-B1 감사 암호화 코어](M6-B1-audit-encryption-core-result.md)
- 검토 기준: [Milestone Checklist](MILESTONE-CHECKLIST.md)
- 작성 범위: 단일 구현 명세입니다. 실제 코드·테스트·측정 결과는 Result에서 확인합니다.
- 원칙: **YAGNI가 최우선입니다.** 설명·선택 이유·구현 조건을 해당 기능 옆에 두며 별도 Background는 만들지 않습니다.

## 0. 이번에는 무엇이 가능해지나요?

**각 Event의 올바른 감사 정보를 암호화해 기록하고, 위원 두 명의 협조로 부모·자손을 추적한 뒤 선정한 미소비 객체를 nf 기준으로 동결합니다.**

| 질문 | M7의 기준 |
|---|---|
| 이전에는 무엇이 있었나요? | M6의 StatusTree와 M6-B1의 암호화 코어·대표 Process 검증입니다. |
| 무엇을 추가하나요? | 8개 Event의 AuditRecord, 생성·소비 조회, 양방향 감사 프로그램, nf 기반 상태 집행입니다. |
| 대표 예시는 무엇인가요? | Entry로 A를 만들고, A를 B·C로, B를 D·E로 Split합니다. |
| 감사자는 무엇을 찾나요? | D의 부모를 따라 Entry를 찾고, A의 자손을 따라 미소비 C·D·E를 찾습니다. |
| 무엇을 저장하나요? | 기록 메타데이터·outputRefs·공개점·부모 암호문·출력 소비값 암호문입니다. |
| 무엇을 숨기나요? | 실제 소비 cm·rv, 생성된 객체의 평문 소비 nf·rvnf, owner secret입니다. |
| 어떻게 사용을 막나요? | Contract가 공개 소비 nf·rvnf로 미소비 여부와 Active 상태를 조회합니다. |
| 무엇은 하지 않나요? | Claim, 자동 자손 동결, DKG·키 회전, 영구 감사 캐시, 운영용 서버입니다. |

**Public input 전체를 AuditRecord에 복사하지 않습니다. 그러나 암호문과 outputRefs는 AuditRecord에 저장합니다.** 암호문을 calldata에만 두는 설계가 아닙니다.

### 기존 단계와 무엇이 달라지나요?

| 기존 구현 | M7에서 연결할 내용 |
|---|---|
| M1~M5의 Note·Voucher·8개 Event·Policy | 기존 의미를 유지하고 실제 감사 정보의 암호화 검사를 추가합니다. |
| M6의 별도 StatusTree·Active path | nf·rvnf 상태 mapping으로 대체합니다. 기존 membership Tree는 유지합니다. |
| M6-B1의 Process 암호화·실험용 기록 | 모든 대상 Event를 실제 원장에 연결하고 기록·소비를 함께 처리합니다. |
| 암호문 한 건의 복원 | 여러 기록의 조회·복호화·부모/자손 탐색을 연결합니다. |

새 Main Contract 이름은 ZkDPPAuditLedger입니다. 기존 ZkDPPStatusLedger와 M1~M6-B1 명세·코드·결과는 그대로 보존합니다. 새 POC 배포를 사용하며 상태 migration은 구현하지 않습니다.

## 1. 하나의 예시로 전체 흐름을 보면 어떤가요?

**생성 기록만 읽는 것과, 소비 기록을 찾아 다음 단계로 이동하는 것을 같은 예시로 확인합니다.**

$$
\varnothing \xrightarrow{\mathrm{Entry}} cm_A
$$

$$
cm_A \xrightarrow{\mathrm{Split}} (cm_B,cm_C)
$$

$$
cm_B \xrightarrow{\mathrm{Split}} (cm_D,cm_E)
$$

이 절의 A~E는 사람 이름이 아니라 Note 이름입니다. 전부 actor-1이 소유합니다.

| Note | 질량 kg | 재활용 귀속량 kg | 탄소 kgCO2e | 생성 기록 ID |
|---|---:|---:|---:|---:|
| A | 10 | 3 | 5 | 1 — Entry |
| B | 6 | 1.8 | 3 | 2 — 첫 Split |
| C | 4 | 1.2 | 2 | 2 — 첫 Split |
| D | 2 | 0.6 | 1 | 3 — 두 번째 Split |
| E | 4 | 1.2 | 2 | 3 — 두 번째 Split |

모든 State에 $10^9$ scale을 적용한 정수를 사용합니다. Note들은 ELIGIBLE이고, opening은 순서대로 7001~7005입니다. DocumentInfo는 ProductName=M7 Material, LotID=M7-A~M7-E로 고정합니다. 별도 반올림 테스트는 M3·M4의 나머지가 발생하는 값을 사용합니다.

실행 순서는 다음입니다.

1. Admin이 actor-1 시나리오의 EVM account에 Entry 권한을 부여합니다.
2. Entry와 두 Split을 실행합니다. Note leaf는 A·B·C·D·E 순서의 5개, 감사 기록은 3개입니다.
3. 마지막 Split이 확정된 block number·hash를 감사 snapshot으로 고정합니다.
4. D에서 backward tracing하여 B와 A의 Entry까지 복원합니다.
5. A에서 forward tracing하여 미소비 C·D·E를 찾습니다. A·B는 이미 소비됐으므로 leaf 대상에서 제외합니다.
6. Status Authority가 선정한 C·D·E의 nf를 하나씩 Freeze합니다. 각 소비 시도가 실패해야 합니다.
7. C는 Unfreeze 후 Exit가 성공합니다. E는 Frozen에서 Revoked로 바꾸고 소비가 계속 거부돼야 합니다. D는 Frozen으로 남습니다.

마지막 상태는 Note leaf 5개, AuditRecord 4개, 사용된 Note nf 3개(A·B·C)입니다. C의 상태가 Active로 돌아와도 이미 소비됐으므로 다시 사용할 수 없습니다.

## 2. 누가 무엇을 담당하나요?

| 주체 | 역할 | POC 식별 |
|---|---|---|
| System Admin | Entry 권한·Policy Authority 등록을 담당합니다. | Anvil account 0 |
| Participant | 자기 객체의 관계와 올바른 감사 암호화를 증명합니다. | actor-1~3; EVM account 1~3 |
| Policy Authority | M5 방식으로 Process Policy·Grant를 등록합니다. | Anvil account 0을 첫 Authority로 등록 |
| Status Authority | 감사 프로그램을 실행하고 선정한 nf의 상태를 변경합니다. | 배포 시 고정한 Anvil account 4 |
| 위원 1·2·3 | 승인된 원본의 partial decryption을 제공합니다. | B1의 분리된 로컬 share 파일 |
| Contract | proof·권한·소비 상태를 확인하고 원장을 원자적으로 갱신합니다. | ZkDPPAuditLedger |

Auditor는 별도 권한 계정이 아니라 Status Authority의 감사 기능을 가리킵니다. 위원은 그래프를 만들지 않습니다. 운영용 감사 승인 Contract나 위원 네트워크 서버는 만들지 않습니다.

### Entry에서 별도 owner의 참여가 필요한가요?

**승인받은 Participant가 자기 Note를 만들므로 별도 owner와 협조하는 절차는 없습니다.**

관리자는 Participant에게 Entry 권한만 부여합니다. Participant는 자기 secret으로 Note 주소와 소비 nf를 계산합니다. 관리자에게 secret을 전달하지 않습니다.

기존 entryIssuers[account]는 Entry 호출 권한이며 모든 공급망 참여자의 일반 등록 명부가 아닙니다. 기존 Entry Circuit은 secret→주소를 검사하지 않았지만, M7 Entry Circuit은 이 관계와 출력 nf 암호화를 추가로 검사합니다.

EVM 호출 권한과 ZK 소유권은 별개입니다. 시나리오에서 같은 Participant가 둘을 사용하더라도 msg.sender와 ZK address를 암호학적으로 연결하는 추가 검사는 만들지 않습니다.

### 무엇을 신뢰하나요?

신뢰 Setup 생성자·위원회·Auditor는 정직하며 승인된 온체인 원본만 처리합니다. Participant는 신뢰하지 않으므로 실제 부모·올바른 출력 소비값·암호화 관계를 Circuit에서 강제합니다.

원 TDH2의 일반적인 선택 암호문 공격 보안·악의적 위원 응답 증명을 그대로 제공한다고 주장하지 않습니다. 감사자가 복원한 값을 잊도록 강제하지 않으며, Go의 완전한 secret 삭제·Production key 관리는 범위 밖입니다.

## 3. AuditRecord와 공통 검증은 어떻게 구성하나요?

### 3.1 어떤 값을 저장하나요?

**AuditRecord는 무엇을 소비했고 무엇을 만들었으며 이후 어떤 소비값으로 연결되는지 읽는 기록입니다.**

| 필드 | 형식 | 목적 |
|---|---|---|
| eventKind | uint8 | Event별 평문 구조를 해석합니다. |
| policyRef | uint256 Field | Process Policy를 식별합니다. 나머지 Event는 0입니다. |
| outputRefs | ObjectRef 배열 | 생성한 cm·rv와 순서를 기록합니다. |
| r1X, r1Y | 각각 uint256 Field | 공통 암호문 공개점 $R_1$입니다. |
| encryptedParents | uint256 Field 배열 | 실제 부모 식별값을 암호화한 부분입니다. |
| encryptedOutputNfs | uint256 Field 배열 | outputRefs 순서의 소비 nf·rvnf를 암호화한 부분입니다. |

기록 ID는 uint256이고 1부터 증가하는 auditRecords mapping의 key입니다. Record 안에 ID를 다시 저장하지 않습니다. ID 0은 기록 없음입니다. parentCount는 encryptedParents 길이로 알 수 있어 중복 저장하지 않습니다.

ObjectRef는 objectType(uint8)과 rawId(uint256 Field)의 쌍입니다. NOTE=1의 rawId는 cm, VOUCHER=2의 rawId는 rv입니다. rawId는 Tree index가 아닙니다. Claim은 M7에서 허용하지 않습니다.

EventKind는 기존 순서대로 Entry=0, Transfer=1, Proceed=2, Recall=3, Merge=4, Split=5, Process=6, Exit=7입니다. Issue=8은 기존 Policy 구분에만 유지하고 실행 API는 추가하지 않습니다.

암호문의 부모 부분에는 종류를 별도 Field로 반복하지 않습니다. Proceed·Recall의 부모는 Voucher이고 그 외 소비 Event의 부모는 Note라는 고정 구조로 해석합니다. 종류·길이·순서는 Contract 함수와 Circuit에 고정되며 호출자가 선택하지 않습니다.

### 3.2 두 암호화 필드는 다른 키를 쓰나요?

**아닙니다. 하나의 평문 벡터를 키 하나로 암호화하고, 결과를 의미에 따라 두 부분으로 나눠 저장합니다.**

Process의 경우는 다음입니다.

$$
\mathbf M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

$$
\mathbf C=(C_0,C_1,C_2,C_3,C_4)
$$

encryptedParents에는 $C_0,C_1,C_2$, encryptedOutputNfs에는 $C_3,C_4$를 넣습니다. 뒤 배열에서 마스크 index를 다시 0으로 시작하지 않습니다. outputRefs의 첫 출력에는 첫 출력 소비값이 대응합니다.

| Event | 부모 종류·개수 | outputRefs 순서 | 평문 길이 |
|---|---|---|---:|
| Entry | 없음 | Note | 1 |
| Transfer | Note 1개 | Voucher, Change Note | 3 |
| Proceed | Voucher 1개 | Receiver Note | 2 |
| Recall | Voucher 1개 | Sender Note | 2 |
| Merge | Note 2개 | Note | 3 |
| Split | Note 1개 | 첫 Note, 둘째 Note | 3 |
| Process | Note 3개 | ELIGIBLE Note, WASTE Note | 5 |
| Exit | Note 1개 | 없음 | 1 |

### 3.3 무엇은 복사하지 않나요?

noteRoot·voucherRoot·policyScopeRef·소비 nf·epoch·proof·문맥 $L$을 모두 Record에 복사하지 않습니다. 생성 transaction의 검증된 공개 입력으로 읽거나 재계산합니다.

암호문과 outputRefs는 감사 기록의 핵심이므로 **storage에 남깁니다.** 암호문을 calldata에만 두는 변경은 하지 않습니다. 15개 공개 입력 전체 복사를 피하는 것과 모든 중복 storage를 금지하는 것은 다른 주장입니다.

AuditRecorded(uint256 indexed auditRecordId) 로그로 기록 ID와 transaction hash를 연결합니다. Contract가 자기 transaction hash를 계산해 저장하지 않습니다. 조회 방법과 일치 검사는 5장에서 정의합니다. 배열 길이·참조 종류·mapping 관리 비용도 실제 SSTORE에 포함되므로 B1보다 gas가 줄어든다고 미리 단정하지 않습니다.

### 3.4 기존 Note·Voucher 관계는 무엇인가요?

DocumentInfo=(ProductName, LotID)이며 Unit·Quantity는 없습니다. 각 문자열 NFC 정규화·UTF-8 길이 구분과 DocumentHash 계산은 기존 구현을 그대로 재사용합니다. Circuit은 DocumentHash의 원문 metadata 진실성을 확인하지 않습니다.

State 순서는 $(q_{\mathrm{mass}},a_{\mathrm{rec}},e)$이며 각각 uint64입니다. 질량·재활용은 kg, 탄소는 kgCO2e에 $10^9$을 곱합니다. $a_{\mathrm{rec}}\le q_{\mathrm{mass}}$이고 Role은 ELIGIBLE=0 또는 WASTE=1입니다. WASTE이면 재활용·탄소는 모두 0입니다.

$$
address=H(OwnerTag,sk_{\mathrm{owner}})
$$

$$
cm=H(NoteTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,address,opening)
$$

$$
nf=H(NullifierTag,sk_{\mathrm{owner}},cm)
$$

Voucher의 Field 순서는 DocumentHash, AssetRole, 세 State, senderAddress, receiverAddress, deadlineEpoch, opening입니다.

$$
rv=H(VoucherTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,senderAddress,receiverAddress,deadlineEpoch,opening)
$$

$$
rvnf=H(VoucherNullifierTag,opening,rv)
$$

Domain 문자열은 기존 zkDPP:Owner:v1, zkDPP:Note:v1, zkDPP:Nullifier:v1, zkDPP:Voucher:v1, zkDPP:VoucherNullifier:v1을 유지합니다. 기존 MustHashToField로 상수화하고 Native·Circuit은 같은 Poseidon2 Hash 순서를 사용합니다.

### 3.5 일반 계산과 Circuit은 어떻게 암호화를 공유하나요?

**Event가 실제 평문을 구성하고, 공통 코어는 그 벡터를 암호화합니다.**

메시지 Field $p$는 BLS12-381 scalar field이고 Jubjub 좌표의 base field입니다. 암호화 난수·위원 shares의 범위 $q$는 Jubjub 소수 subgroup 차수이며 둘은 다릅니다. B1 라이브러리 상수·canonical 32-byte big-endian encoding·압축하지 않은 $(X,Y)$ 표현을 유지합니다.

기존 암호화 Domain은 zkDPP:AuditContext:v1, zkDPP:AuditKey:v1, zkDPP:AuditMask:v1입니다. Hash·곡선 파라미터를 새로 선택하지 않습니다.

각 Event의 기존 공개 입력 배열을 $P$라고 합니다. 부모와 출력 소비값을 연결한 평문 길이는 $n$입니다.

$$
L=H(AuditContextTag,eventKind,P_0,\ldots,P_{b-1})
$$

$$
R_1=rG,\qquad Z=rPK,\qquad 1\le r<q
$$

$$
K=H(AuditKeyTag,Z_X,Z_Y,L,n)
$$

$$
C_j=M_j+H(AuditMaskTag,K,j)\pmod p,\qquad j=0,\ldots,n-1
$$

PK는 B1 위원회 공개키를 Circuit 상수로 사용합니다. 임의 PK를 witness로 받지 않습니다. 정상 암호화마다 새 crypto/rand 난수를 사용하며 같은 r를 재사용하지 않습니다. Circuit은 난수의 범위와 관계를 확인하지만 진짜 무작위성 자체를 증명하지는 않습니다.

~~~text
PrepareAudit(kind, P, actualParents, actualOutputSpendValues):
    # 핵심: 실제 객체에서 얻은 값들을 한 번에 암호화합니다.
    # 1. Event에 맞는 순서와 길이를 확인합니다.
    M = actualParents || actualOutputSpendValues
    L = H(AuditContextTag, kind, P의 원소들)
    # 2. 같은 PK·새 난수로 하나의 암호문을 만듭니다.
    r = 안전한 새 Jubjub scalar
    CT = Encrypt(fixedCommitteePK, L, M, r)
    # 3. storage의 의미상 두 부분으로 나눕니다.
    audit = (CT.R1.X, CT.R1.Y, CT.C[:부모수], CT.C[부모수:])
    return audit, private r

AssertEventAudit(kind, P, actualParents, actualOutputSpendValues, r, audit):
    # 핵심: 엉뚱한 평문이 아니라 실제 Event의 정보를 암호화했는지 확인합니다.
    # 1. Circuit에 고정한 실제 길이로 벡터를 구성합니다.
    M = actualParents || actualOutputSpendValues
    C = audit.encryptedParents || audit.encryptedOutputNfs
    L = H(AuditContextTag, kind, P의 원소들)
    # 2. B1 gadget으로 난수·공개점·모든 마스크 관계를 확인합니다.
    AssertEncrypted(fixedCommitteePK, L, M, r, audit.R1, C)
~~~

AssertEncrypted는 r의 252-bit 표현·$r<q$·$r\ne0$을 강제하고, 검증된 고정 subgroup 점 G·PK로 계산한 공개점과 좌표가 같은지 확인합니다. 암호문 배열 원소마다 위 덧셈 관계를 강제합니다. 외부 Field·좌표는 canonical 범위를 벗어나면 거부하고 자동 mod 축약하지 않습니다.

### 3.6 공통 소비 검사는 어떤 내용인가요?

**membership은 Circuit이, 현재 소비·동결 여부는 Contract가 확인합니다.**

~~~text
AssertNoteSpend(note, sk, root, path, publicNF):
    # 핵심: 숨긴 Note의 유효성·소유권·Tree 존재·소비값을 확인합니다.
    # 1. Note의 세 State를 64-bit로 제한하고 Role 조건을 검사합니다.
    AssertStateAndRole(note)
    assert note.address == H(OwnerTag, sk)
    cm = NoteCommitment(note)
    # 2. private index의 32개 bit로 좌우 순서를 정합니다.
    bits = ToBinary(path.index, 32)
    node = cm
    for level in 0..31:
        node = bits[level] == 0 ? Compress(node, sibling[level])
                                : Compress(sibling[level], node)
    assert node == root
    # 3. 공개 소비값과 같은지 확인합니다.
    assert H(NullifierTag, sk, cm) == publicNF
    return cm

RequireActiveUnspent(type, spendValue):
    # 핵심: 존재 증명과 별개로 지금 소비 가능한지 확인합니다.
    # 1. 종류에 맞는 소비·상태 mapping을 선택합니다.
    spent = type == NOTE ? noteSpentIn : voucherSpentIn
    status = type == NOTE ? noteStatusByNf : voucherStatusByNf
    # 2. 이미 사용됐거나 Frozen·Revoked이면 실패합니다.
    require spent[spendValue] == 0
    require status[spendValue] == ACTIVE
~~~

Voucher membership도 동일한 32-level 계산을 rv에서 시작합니다. 기존 index·siblings는 private witness이고, 별도 Status path는 없습니다. Contract의 accepted root 확인을 생략하지 않습니다.

### 3.7 공개 입력과 API는 어떤 순서인가요?

audit 인자는 r1X, r1Y, encryptedParents[], encryptedOutputNfs[]로 구성합니다. Contract는 Event별 고정 길이를 확인하고, verifier에는 배열 길이 없이 값만 아래 순서로 전달합니다.

| Event | 기존 공개 입력 $P$ | 감사 suffix 포함 총개수 |
|---|---|---:|
| Entry | cm | 4 |
| Transfer | noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch | 11 |
| Proceed | voucherRoot, rvnf, cmReceiver | 7 |
| Recall | voucherRoot, rvnf, cmReturn, currentEpoch | 8 |
| Merge | noteRoot, nf1, nf2, cmOut | 9 |
| Split | noteRoot, nf, cmOut1, cmOut2 | 9 |
| Process | policyRef, policyScopeRef, noteRoot, nf[3], cmOut[2] | 15 |
| Exit | noteRoot, nf | 5 |

suffix는 항상 r1X, r1Y, encryptedParents 원소들, encryptedOutputNfs 원소들입니다. proof·ABI offset·배열 길이는 Circuit public input에 들어가지 않습니다.

~~~text
entry(proof, cm, audit)
exit(proof, noteRoot, nf, audit)
transfer(proof, noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch, audit)
proceed(proof, voucherRoot, rvnf, cmReceiver, audit)
recall(proof, voucherRoot, rvnf, cmReturn, currentEpoch, audit)
merge(proof, noteRoot, nf1, nf2, cmOut, audit)
split(proof, noteRoot, nf, cmOut1, cmOut2, audit)
process(proof, policyRef, policyScopeRef, noteRoot, nf[3], cmOut[2], audit)
~~~

각 함수는 새 auditRecordId를 반환합니다. outputRefs·eventKind·Record의 policyRef는 Contract가 해당 함수의 검증 입력에서 구성하며 별도 비검증 인자로 받지 않습니다.

### 3.8 Contract는 기록과 소비를 어떻게 함께 처리하나요?

새 Contract는 기존 membership Tree·commitments·voucherCommitments·Entry 권한·Policy Registry를 유지합니다. 생성 중복은 commitments/voucherCommitments, 소비는 noteSpentIn/voucherSpentIn으로 구분합니다.

| 추가 상태·조회 | 역할 |
|---|---|
| nextAuditRecordId | 1부터 증가하는 다음 ID입니다. |
| auditRecords[id], getAuditRecord(id) | 변경 불가능한 감사 기록입니다. 0·없는 ID 조회는 거부합니다. |
| producerOf[objectType][rawId] | 객체를 생성한 AuditRecord ID입니다. |
| noteSpentIn[nf] | Note를 소비한 AuditRecord ID입니다. |
| voucherSpentIn[rvnf] | Proceed·Recall 중 성공한 소비 기록 ID입니다. |
| noteStatusByNf[nf], voucherStatusByNf[rvnf] | 종류별 사용 가능 상태입니다. 없는 항목은 Active입니다. |

기존 noteNullifiers(nf)·voucherNullifiers(rvnf) 조회는 해당 spentIn 값이 0이 아닌지 반환하는 view로 제공합니다. 같은 사실을 bool mapping에 다시 SSTORE하지 않습니다.

~~~text
VerifyAndRecord(proof, kind, policyRef, P, consumedSpendValues, outputRefs, audit):
    # 핵심: 검증한 Event의 소비·출력·감사 기록을 하나의 transaction으로 확정합니다.
    # 1. 각 Event 함수가 먼저 권한·root·미소비·Active·출력 중복을 검사합니다.
    require 감사 배열 길이와 출력 종류·개수가 해당 Event와 일치
    require 모든 Field 값이 p 미만
    require outputRefs가 함수의 공개 출력과 정확히 일치하고 새 객체임
    # 2. 암호문까지 포함한 공개 입력을 고정 verifier로 검증합니다.
    Verify(selectedAuditVerifier, proof, P || audit의 Field들)
    # 3. proof 이후에만 ID와 소비 상태를 기록합니다.
    aid = nextAuditRecordId
    nextAuditRecordId = aid + 1
    각 consumedSpendValue에 대해 해당 spentIn[value] = aid
    # 4. 공개 출력과 생성 기록을 함께 갱신합니다.
    outputRefs 순서대로:
        commitments 또는 voucherCommitments에 등록
        해당 membership Tree에 append
        producerOf[type][rawId] = aid
    # 5. 검증한 암호문·출력 참조를 저장하고 ID 로그를 남깁니다.
    auditRecords[aid] = (kind, policyRef, outputRefs, audit)
    Emit AuditRecorded(aid)
    return aid
~~~

이 의사 코드는 기능 관계를 표현합니다. selectedAuditVerifier·권한·소비 목록은 내부에서 결정하며 사용자가 임의의 verifier나 consumedSpendValues를 제출하는 public 함수는 만들지 않습니다.

NoteSpend(nf)·VoucherSpend(rvnf)는 해당 종류의 소비 mapping을 선택하기 위한 내부 표시입니다. Record의 평문 부모가 아니며 public API에 별도 객체로 추가하지 않습니다. 공통 함수의 proof는 해당 Event 호출이 받은 proof입니다.

Proof 실패·verifier revert·두 번째 출력 중복·Tree 가득 참 등 어떤 실패에서도 ID·spentIn·commitment·Tree·producerOf·AuditRecord가 모두 불변이어야 합니다. verifier 오류는 기존 InvalidProof 처리 관례로 정규화합니다.

NoteAppended·VoucherAppended·NoteExited와 권한·Policy 로그는 기존 목적을 유지합니다. AuditRecorded는 감사 대상 8개 Event 성공 때만 발생합니다. 권한 변경·동결 transaction은 공급망 Event의 AuditRecord를 추가 생성하지 않습니다.

## 4. 각 Event는 어떤 감사 정보를 만드나요?

**각 절의 준비는 Participant의 일반 계산, Circuit은 proof 안의 검사, Contract는 현재 원장 상태의 검사·변경입니다.** 공통 함수의 전체 의미는 3장에서 정의했으며 아래에서는 실제 인자를 명시합니다.

각 Event 측정은 일반 암호화·witness·Prove·Verify·proof 크기와 실제 원장 transaction gas를 분리합니다. 구체적인 실행 횟수·환경은 8장의 동일 기준을 적용합니다.

### 4.1 Entry는 최초 Note를 어떻게 남기나요?

**Participant가 자기 Note를 생성하면서 그 Note의 소비 nf를 암호화합니다. nf를 사용 처리하는 것은 아닙니다.**

$$
\varnothing \xrightarrow{\mathrm{Entry}} cm_A,\qquad \mathbf M=(nf_A)
$$

공개값은 cm, $R_1$ 좌표, 암호화된 nf 한 개입니다. private witness는 Note 전체·owner secret·암호화 난수입니다. encryptedParents는 빈 배열이고 outputRefs는 NoteRef(cm) 하나입니다.

~~~text
PrepareEntry:
    # 핵심: 권한을 받은 소유자가 자기 최초 Note와 감사 암호문을 준비합니다.
    address = H(OwnerTag, skOwner)
    note = DocumentHash·State·ELIGIBLE·address·새 opening으로 생성
    cm = NoteCommitment(note)
    nfOut = H(NullifierTag, skOwner, cm)
    audit, r = PrepareAudit(ENTRY, [cm], [], [nfOut])

EntryCircuit:
    # 1. 기존 Entry의 숫자·Role 조건을 확인합니다.
    AssertStateAndRole(note)
    assert note.q_mass > 0 and note.AssetRole == ELIGIBLE
    # 2. 주소·commitment·출력 소비값을 실제 Note에서 계산합니다.
    assert note.address == H(OwnerTag, skOwner)
    assert NoteCommitment(note) == public.cm
    nfOut = H(NullifierTag, skOwner, public.cm)
    # 3. 올바른 nf가 같은 암호문에 들어 있는지 확인합니다.
    AssertEventAudit(ENTRY, [public.cm], [], [nfOut], r, audit)

entry Contract:
    # 1. Entry 호출 권한과 output 중복을 확인합니다.
    require entryIssuers[msg.sender] and commitments[cm] == false
    # 2. proof 검증·새 Note·생성 기록만 함께 처리합니다.
    return VerifyAndRecord(proof, ENTRY, 0, [cm], [], [NoteRef(cm)], audit)
~~~

초기 e는 0 이상 uint64이며 Entry 이전 탄소를 허용합니다. 현실 원자재·문서의 진실성은 기존 Entry 승인 가정에 둡니다. q_mass=0, WASTE, 범위 초과, 다른 secret·주소·nf 암호문, 미승인·권한 해제된 호출자는 실패합니다.

성공하면 Note Tree와 commitment·producerOf·AuditRecord만 늘고 noteSpentIn은 바뀌지 않습니다. 측정에는 최초 append와 후속 append를 구분합니다.

### 4.2 Split은 자손 두 개를 어떻게 연결하나요?

**실제로 소비한 부모 하나와, 두 출력이 나중에 사용할 nf를 같은 기록으로 연결합니다.**

$$
cm_A \xrightarrow{\mathrm{Split}} (cm_B,cm_C),\qquad \mathbf M=(cm_A,nf_B,nf_C)
$$

공개값은 noteRoot, nf, cmOut1, cmOut2와 감사 suffix 5개입니다. private witness는 입력·출력 Note, owner secret, 입력 index·siblings, 배분 나머지와 암호화 난수입니다.

소유자가 첫 출력의 질량을 선택합니다. 입력 질량은 양수이고 출력 하나의 질량 0은 허용합니다. 각 $x\in\{a_{\mathrm{rec}},e\}$의 배분은 다음입니다.

$$
q_1+q_2=q_{\mathrm{in}},\qquad
x_2=\left\lfloor\frac{q_2x_{\mathrm{in}}}{q_{\mathrm{in}}}\right\rfloor,\qquad
x_1=x_{\mathrm{in}}-x_2
$$

~~~text
PrepareSplit:
    # 핵심: 두 출력의 State를 계산한 뒤 그 출력의 nf까지 암호화합니다.
    q2 = input.q_mass - 선택한 q1
    각 x = a_rec, e에 대해 x2, remainder = divmod(q2 * input.x, input.q_mass)
    x1 = input.x - x2
    output1, output2 = 같은 owner·Role과 새 DocumentHash·opening으로 생성
    parents = [NoteCommitment(input)]
    outNfs = [NoteNullifier(sk, cmOut1), NoteNullifier(sk, cmOut2)]
    audit, r = PrepareAudit(SPLIT, [root, nf, cmOut1, cmOut2], parents, outNfs)

SplitCircuit:
    # 1. 실제 private input을 소비하는 관계를 확인합니다.
    cmIn = AssertNoteSpend(input, sk, root, path, nf)
    assert input.q_mass > 0
    # 2. 두 출력의 범위·owner·Role·State 합과 배분을 확인합니다.
    각 output에 대해 AssertStateAndRole(output)
    assert 두 output.address == input.address
    assert 두 output.AssetRole == input.AssetRole
    assert q1 + q2 == input.q_mass
    각 x = a_rec, e에 대해:
        assert x1 + x2 == input.x
        assert q2 * input.x == input.q_mass * x2 + remainder
        assert 0 <= remainder < input.q_mass; remainder는 uint64
    assert 두 output commitment가 공개값과 일치하고 서로 다름
    # 3. 실제 부모·두 output의 nf를 암호화했는지 확인합니다.
    outNfs = [NoteNullifier(sk, cmOut1), NoteNullifier(sk, cmOut2)]
    AssertEventAudit(SPLIT, P, [cmIn], outNfs, r, audit)

split Contract:
    # 1. 현재 root·미소비·Active·두 출력 중복을 확인합니다.
    require acceptedNoteRoot(root)
    RequireActiveUnspent(NOTE, nf)
    require cmOut1 != cmOut2 and 두 commitment가 미등록
    # 2. 소비 기록 하나와 두 output을 순서대로 함께 처리합니다.
    return VerifyAndRecord(proof, SPLIT, 0, P, [NoteSpend(nf)],
                           [NoteRef(cmOut1), NoteRef(cmOut2)], audit)
~~~

출력 DocumentHash와 입력 DocumentHash의 관계는 기존 M4처럼 검사하지 않습니다. WASTE도 같은 Role로 Split할 수 있으며 재활용·탄소 0을 유지합니다.

잘못된 비례 배분·나머지·Role·owner·부모 암호문·output nf·동일 output·Frozen input은 실패합니다. 두 번째 append가 실패하면 첫 output과 nf·기록 ID도 되돌아가야 합니다. 두 출력 저장·append 비용을 함께 측정합니다.

### 4.3 Transfer는 소유권 전달 구간을 어떻게 연결하나요?

**Sender는 Receiver의 secret 없이 공유 opening으로 Voucher의 소비 rvnf를 준비합니다.**

$$
cm_{\mathrm{in}}\xrightarrow{\mathrm{Transfer}}(rv_{\mathrm{new}},cm_{\mathrm{change}})
$$

$$
\mathbf M=(cm_{\mathrm{in}},rvnf_{\mathrm{new}},nf_{\mathrm{change}})
$$

outputRefs 순서는 기존 공개 입력과 맞춰 **Voucher, Change Note**입니다. Note Tree와 Voucher Tree는 독립되어 있으므로 이 참조 순서가 두 Tree의 전역 index를 하나로 합친다는 뜻은 아닙니다.

공개값은 noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch와 감사 suffix 5개입니다. private witness는 Sender secret, 입력 Note·path, Voucher·Change Note, 배분 나머지·opening·암호화 난수입니다.

Sender가 $0<q_v\le q_{\mathrm{in}}$을 선택하고 $q_c=q_{\mathrm{in}}-q_v$로 계산합니다. 재활용·탄소는 Change를 내림 계산하고 Voucher가 나머지를 받습니다.

$$
x_c=\left\lfloor\frac{q_cx_{\mathrm{in}}}{q_{\mathrm{in}}}\right\rfloor,\qquad
x_v=x_{\mathrm{in}}-x_c,\qquad x\in\{a_{\mathrm{rec}},e\}
$$

~~~text
PrepareTransfer:
    # 핵심: Sender가 Change의 nf와 Voucher의 rvnf를 모두 계산합니다.
    Voucher·Change State = 위 비례 배분; 두 출력은 input의 DocumentHash·Role 유지
    Voucher.senderAddress = Sender 주소; receiverAddress = 선택한 Receiver 주소
    Change.address = Sender 주소
    Voucher.deadlineEpoch = transferEpoch + deltaEpoch
    두 출력에 새 opening을 사용하고 Voucher opening은 Receiver와 공유
    outNfs = [H(VoucherNullifierTag, voucher.opening, rvNew),
              H(NullifierTag, senderSecret, cmChange)]
    audit, r = PrepareAudit(TRANSFER, P, [cmIn], outNfs)

TransferCircuit:
    # 1. private Note 소비·Sender 소유권을 확인합니다.
    cmIn = AssertNoteSpend(input, senderSecret, noteRoot, path, nf)
    # 2. Voucher·Change의 State·Role·소유자·배분을 확인합니다.
    AssertStateAndRole(Voucher); AssertStateAndRole(Change)
    assert Voucher.q_mass > 0 and Voucher.q_mass + Change.q_mass == input.q_mass
    assert 두 출력의 DocumentHash·Role == input의 값
    assert Voucher.senderAddress == Change.address == H(OwnerTag, senderSecret)
    각 x = a_rec, e에 대해:
        assert Change.q_mass * input.x == input.q_mass * Change.x + remainder
        assert 0 <= remainder < input.q_mass; remainder는 uint64
        assert Voucher.x + Change.x == input.x
    # 3. epoch overflow 없이 deadline과 공개 출력을 확인합니다.
    transferEpoch, deltaEpoch, Voucher.deadlineEpoch는 uint64
    assert Voucher.deadlineEpoch == transferEpoch + deltaEpoch
    assert VoucherCommitment(Voucher) == rvNew
    assert NoteCommitment(Change) == cmChange
    # 4. 실제 부모와 output의 소비값을 암호문에 연결합니다.
    outNfs = [VoucherNullifier(Voucher.opening, rvNew),
              NoteNullifier(senderSecret, cmChange)]
    AssertEventAudit(TRANSFER, P, [cmIn], outNfs, r, audit)

transfer Contract:
    # 1. 현재 소비·전송 시점·중복 출력을 확인합니다.
    require acceptedNoteRoot(noteRoot)
    RequireActiveUnspent(NOTE, nf)
    require transferEpoch == floor(block.timestamp / 600)
    require rvNew와 cmChange가 각 Tree의 신규 commitment
    # 2. 한 Note 소비, Voucher·Change 생성, 감사 기록을 함께 처리합니다.
    return VerifyAndRecord(proof, TRANSFER, 0, P, [NoteSpend(nf)],
                           [VoucherRef(rvNew), NoteRef(cmChange)], audit)
~~~

전량 Transfer도 질량·재활용·탄소가 모두 0인 Change Note를 생성합니다. 그 Note도 실제 등록 객체이므로 추적에서 임의로 생략하지 않습니다. 운송 탄소는 추가하지 않습니다. ELIGIBLE·WASTE 모두 허용합니다.

deltaEpoch=0은 허용하며 이 Voucher의 Recall은 불가능합니다. opening 전달은 고정 로컬 테스트 데이터로만 재현합니다. 잘못된 shared opening·Receiver/Sender 관계·배분·epoch·Frozen input·중복 출력은 실패합니다. 두 Tree 중 하나의 갱신이 실패해도 모든 기록이 되돌아가야 합니다.

### 4.4 Proceed는 Receiver의 Note로 어떻게 이어지나요?

**공유 opening의 rvnf로 Voucher를 소비하고, Receiver secret으로 새 Note의 nf를 준비합니다.**

$$
rv\xrightarrow{\mathrm{Proceed}}cm_{\mathrm{receiver}},\qquad
\mathbf M=(rv,nf_{\mathrm{receiver}})
$$

공개값은 voucherRoot, rvnf, cmReceiver와 감사 suffix 4개입니다. private witness는 Voucher·path·Receiver secret·출력 Note·암호화 난수입니다.

~~~text
PrepareProceed:
    # 핵심: Receiver가 Voucher의 State를 자기 Note로 옮깁니다.
    output = 같은 DocumentHash·Role·State, Receiver 주소, 새 opening의 Note
    rvnf = VoucherNullifier(voucher.opening, rv)
    nfOut = NoteNullifier(receiverSecret, cmReceiver)
    audit, r = PrepareAudit(PROCEED, P, [rv], [nfOut])

ProceedCircuit:
    # 1. private Voucher의 유효성과 membership을 확인합니다.
    AssertStateAndRole(voucher); deadlineEpoch는 uint64
    rv = VoucherCommitment(voucher)
    assert MerkleRoot(rv, private index·siblings 32개) == voucherRoot
    # 2. Receiver와 공유 opening의 소비값을 확인합니다.
    assert voucher.receiverAddress == H(OwnerTag, receiverSecret)
    assert VoucherNullifier(voucher.opening, rv) == public.rvnf
    # 3. 새 Note와 감사 정보가 실제 Voucher에서 이어지는지 확인합니다.
    AssertStateAndRole(output)
    assert output의 DocumentHash·Role·State == voucher의 값
    assert output.address == voucher.receiverAddress
    assert NoteCommitment(output) == cmReceiver
    AssertEventAudit(PROCEED, P, [rv], [NoteNullifier(receiverSecret, cmReceiver)], r, audit)

proceed Contract:
    # 1. 현재 Voucher 소비·상태와 출력 중복을 확인합니다.
    require acceptedVoucherRoot(voucherRoot)
    RequireActiveUnspent(VOUCHER, rvnf)
    require commitments[cmReceiver] == false
    # 2. Voucher 소비 기록과 Receiver Note를 함께 남깁니다.
    return VerifyAndRecord(proof, PROCEED, 0, P, [VoucherSpend(rvnf)],
                           [NoteRef(cmReceiver)], audit)
~~~

Proceed는 deadline 전후 모두 가능합니다. 소비 rv는 calldata에 공개하지 않으며 Contract가 voucherCommitments[rv]를 조회하지 않습니다.

Receiver가 아닌 secret, 잘못된 opening·path·State·nf 암호문, Frozen·Revoked·이미 해결된 Voucher는 실패합니다. 실패 후 voucherSpentIn과 출력·Tree·감사 기록은 모두 불변입니다.

### 4.5 Recall은 Sender에게 어떻게 돌아가나요?

**Proceed와 같은 rvnf를 소비하지만, Sender secret과 deadline 조건으로 반환 Note를 만듭니다.**

$$
rv\xrightarrow{\mathrm{Recall}}cm_{\mathrm{return}},\qquad
\mathbf M=(rv,nf_{\mathrm{return}})
$$

공개값은 voucherRoot, rvnf, cmReturn, currentEpoch와 감사 suffix 4개입니다. private witness는 Voucher·path·Sender secret·반환 Note·암호화 난수입니다.

~~~text
PrepareRecall:
    # 핵심: Sender가 기한 전에 같은 Voucher State를 자기 Note로 회수합니다.
    output = 같은 DocumentHash·Role·State, Sender 주소, 새 opening의 Note
    rvnf = VoucherNullifier(voucher.opening, rv)
    nfOut = NoteNullifier(senderSecret, cmReturn)
    audit, r = PrepareAudit(RECALL, P, [rv], [nfOut])

RecallCircuit:
    # 1. private Voucher 존재·Sender·공유 소비값을 확인합니다.
    AssertStateAndRole(voucher); deadlineEpoch와 currentEpoch는 uint64
    rv = VoucherCommitment(voucher)
    assert MerkleRoot(rv, private index·siblings 32개) == voucherRoot
    assert voucher.senderAddress == H(OwnerTag, senderSecret)
    assert VoucherNullifier(voucher.opening, rv) == public.rvnf
    # 2. deadline 전인지와 반환 Note가 같은 내용인지 확인합니다.
    assert currentEpoch < voucher.deadlineEpoch
    AssertStateAndRole(output)
    assert output의 DocumentHash·Role·State == voucher의 값
    assert output.address == voucher.senderAddress
    assert NoteCommitment(output) == cmReturn
    # 3. 실제 rv와 반환 Note의 nf를 암호문에 연결합니다.
    AssertEventAudit(RECALL, P, [rv], [NoteNullifier(senderSecret, cmReturn)], r, audit)

recall Contract:
    # 1. 최신 block epoch·소비·상태·출력을 확인합니다.
    require currentEpoch == floor(block.timestamp / 600)
    require acceptedVoucherRoot(voucherRoot)
    RequireActiveUnspent(VOUCHER, rvnf)
    require commitments[cmReturn] == false
    # 2. Proceed와 동일한 소비 mapping에 기록합니다.
    return VerifyAndRecord(proof, RECALL, 0, P, [VoucherSpend(rvnf)],
                           [NoteRef(cmReturn)], audit)
~~~

deadline과 같은 epoch부터 Recall은 실패합니다. deadlineRV mapping은 만들지 않습니다. 기한 전에는 Proceed·Recall이 경쟁하며 먼저 기록된 rvnf만 성공합니다.

잘못된 Sender·epoch·deadline·암호문·Frozen Voucher는 실패합니다. Recall 성공 뒤 Proceed와 반대 순서 모두 재소비를 거부해야 합니다.

### 4.6 Merge는 두 부모를 어떻게 기록하나요?

**같은 owner·Role의 두 Note를 소비하고, 두 부모와 새 Note의 nf를 기록합니다.**

$$
(cm_1,cm_2)\xrightarrow{\mathrm{Merge}}cm_{\mathrm{out}},\qquad
\mathbf M=(cm_1,cm_2,nf_{\mathrm{out}})
$$

공개값은 noteRoot, nf1, nf2, cmOut와 감사 suffix 5개입니다. private witness는 두 입력·path, 공통 owner secret, 출력 Note·암호화 난수입니다.

~~~text
PrepareMerge:
    # 핵심: State를 overflow 없이 더하고 실제 두 부모를 암호화합니다.
    output.State = 각 State의 checked uint64 합
    output = 같은 owner·Role, 새 DocumentHash·opening
    audit, r = PrepareAudit(MERGE, P, [cm1, cm2], [NoteNullifier(sk, cmOut)])

MergeCircuit:
    # 1. 같은 secret으로 두 private input 소비를 확인합니다.
    cm1 = AssertNoteSpend(input1, sk, root, path1, nf1)
    cm2 = AssertNoteSpend(input2, sk, root, path2, nf2)
    assert cm1 != cm2 and nf1 != nf2
    # 2. 같은 Role·owner와 State 합을 확인합니다.
    AssertStateAndRole(output)
    assert input1.Role == input2.Role == output.Role
    assert output.address == H(OwnerTag, sk)
    assert output.State == input1.State + input2.State
    assert NoteCommitment(output) == cmOut
    # 3. 부모·새 output의 소비값을 연결합니다.
    AssertEventAudit(MERGE, P, [cm1, cm2], [NoteNullifier(sk, cmOut)], r, audit)

merge Contract:
    # 1. 두 소비값과 신규 output을 확인합니다.
    require acceptedNoteRoot(root) and nf1 != nf2
    RequireActiveUnspent(NOTE, nf1); RequireActiveUnspent(NOTE, nf2)
    require commitments[cmOut] == false
    # 2. 두 nf가 같은 소비 기록 ID를 가리키게 합니다.
    return VerifyAndRecord(proof, MERGE, 0, P, [NoteSpend(nf1), NoteSpend(nf2)],
                           [NoteRef(cmOut)], audit)
~~~

입력·출력 DocumentHash 관계와 ProductProfile은 검사하지 않습니다. ELIGIBLE끼리 또는 WASTE끼리만 허용합니다. 같은 입력 두 번 사용·다른 owner·다른 Role·overflow·Frozen input·부모 순서나 output nf 변조는 실패합니다.

두 소비 nf가 같은 AuditRecord ID를 가리키므로, 감사 프로그램이 합쳐진 경로를 반복 방문해도 한 기록을 한 번만 복호화하는지 확인합니다.

### 4.7 Process는 기존 Policy와 어떻게 함께 검증하나요?

**기존 M5의 공정 규칙을 유지하고, B1에서 검증한 실제 부모·출력 nf 암호화를 그대로 사용합니다.**

공개 입력은 policyRef, policyScopeRef, noteRoot, nf 3개, cm 2개와 감사 suffix 7개입니다. private witness는 같은 owner의 입력 3개·path·출력 2개·owner secret·공정 계산 나머지·암호화 난수입니다.

PolicyRef와 ScopeRef는 다음 관계를 유지합니다.

$$
policyRef=H(PolicyRefTag,6,authorityId,policyId,version)
$$

$$
policyScopeRef=H(ScopeRefTag,sk_{\mathrm{owner}},policyRef)
$$

POC는 authorityId=policyId=version=1, exact 3→2입니다. 분모 $D=10^9$, lossRate=62,500,000, carbonIntensity=93,750,000, wasteMassRate=100,000,000을 Circuit 상수로 유지합니다.

입력 합을 $(Q,A,E)$로 표시하면 다음입니다.

$$
q_{\mathrm{loss}}=\left\lfloor\frac{Q\cdot lossRate}{D}\right\rfloor,\qquad
\Delta e=\left\lfloor\frac{Q\cdot carbonIntensity}{D}\right\rfloor
$$

$$
Q_I=Q-q_{\mathrm{loss}},\quad A_I=A,\quad E_I=E+\Delta e,\qquad
q_W=\left\lfloor\frac{Q_I\cdot wasteMassRate}{D}\right\rfloor
$$

ELIGIBLE output은 $(Q_I-q_W,A_I,E_I)$, WASTE output은 $(q_W,0,0)$입니다. State scale과 비율 분모는 값이 같지만 별개 개념입니다.

~~~text
PrepareProcess:
    # 핵심: M5의 공정 계산과 B1의 감사 평문 구성을 그대로 사용합니다.
    total = 세 input의 checked uint64 State 합
    loss·추가 탄소·WASTE 질량 = 위 floor 계산과 나머지
    두 output = 같은 owner, [ELIGIBLE, WASTE], 새 DocumentHash·opening
    M = [실제 부모 cm 3개, NoteNullifier(sk, cmEligible), NoteNullifier(sk, cmWaste)]
    audit, r = PrepareAudit(PROCESS, P, M[:3], M[3:])

ProcessCircuit:
    # 1. 고정 Policy와 같은 owner secret의 scope를 확인합니다.
    assert public.policyRef == CanonicalPolicyRef
    assert public.policyScopeRef == H(ScopeRefTag, sk, public.policyRef)
    # 2. ELIGIBLE 입력 3개의 실제 소비를 확인합니다.
    각 i에 대해 cm[i] = AssertNoteSpend(input[i], sk, root, path[i], nf[i])
    assert 세 input.Role == ELIGIBLE
    assert cm들이 서로 다르고 nf들도 서로 다름
    # 3. 기존 공정 수학·범위·floor를 확인합니다.
    total = 단계별 checked uint64 합
    각 floor에 대해 value * rate == D * quotient + remainder
    quotient·remainder는 uint64, 0 <= remainder < D
    assert loss <= total.q_mass
    intermediate의 차감·탄소 합은 uint64 범위
    assert 두 output.State == 위 공정식의 결과
    AssertStateAndRole(두 output); owner == H(OwnerTag, sk)
    assert 두 Role == [ELIGIBLE, WASTE]
    assert 두 commitment가 공개 출력과 같고 서로 다름
    # 4. B1과 동일한 평문·문맥·암호화를 검증합니다.
    AssertEventAudit(PROCESS, P, cm[0:3],
                     [NoteNullifier(sk, cmEligible), NoteNullifier(sk, cmWaste)], r, audit)

process Contract:
    # 1. 해당 Policy·Grant·감사 verifier를 확인합니다.
    require PolicyRecord 존재·enabled·PROCESS·3→2
    require record.verifierRef == 배포 시 고정한 auditProcessVerifier
    require policyGrants[policyRef][policyScopeRef]
    # 2. 세 현재 소비 상태와 두 출력을 확인합니다.
    require acceptedNoteRoot(root) and nf 세 개가 서로 다름
    각 nf에 대해 RequireActiveUnspent(NOTE, nf)
    require 두 출력이 서로 다르고 미등록
    # 3. 공정 proof와 감사 기록을 함께 확정합니다.
    return VerifyAndRecord(proof, PROCESS, policyRef, P, 세 NoteSpend,
                           [NoteRef(cmEligible), NoteRef(cmWaste)], audit)
~~~

실제 원자재 종류·DocumentHash 관계·근거 문서 진실성은 Circuit에서 검사하지 않습니다. WASTE의 탄소·재활용 0은 이 POC attribution Policy이며 물리적 폐기물 조성의 보편 규칙으로 주장하지 않습니다.

Policy lifecycle은 기존과 같습니다. Admin이 Authority를 등록하고 Authority가 자기 family/version을 예약·등록합니다. Record·VK·verifier는 변경 불가, disable은 단방향이며 Grant만 revoke·regrant할 수 있습니다. 비활성 Policy나 Grant 해제는 과거 기록의 감사·복호화를 막지 않습니다.

wrong Policy·scope·Grant·legacy verifier, 공정 비율·나머지·Role·overflow·부모·output nf 변조·Frozen input은 실패합니다. Process는 B1의 검증 관계와 public 순서가 같으므로 고정 PK·CCS·VK를 checksum 확인 후 재사용하며 새 SRS를 만들지 않습니다.

### 4.8 Exit는 종료된 경로를 어떻게 남기나요?

**Note는 소비됐지만 다음 출력이 없다는 사실도 감사 기록으로 남깁니다.**

$$
cm\xrightarrow{\mathrm{Exit}}\varnothing,\qquad \mathbf M=(cm)
$$

공개값은 noteRoot, nf와 감사 suffix 3개입니다. private witness는 Note·owner secret·path·암호화 난수입니다. outputRefs와 encryptedOutputNfs는 빈 배열입니다.

~~~text
PrepareExit:
    # 핵심: 실제로 종료할 부모 Note만 암호화합니다.
    cm = NoteCommitment(note)
    nf = NoteNullifier(sk, cm)
    audit, r = PrepareAudit(EXIT, [root, nf], [cm], [])

ExitCircuit:
    # 1. M1 Private Spend와 동일한 소비 관계를 확인합니다.
    cm = AssertNoteSpend(note, sk, root, path, nf)
    # 2. 실제 종료한 부모를 암호화했는지 확인합니다.
    AssertEventAudit(EXIT, [root, nf], [cm], [], r, audit)

exit Contract:
    # 1. 현재 사용 가능한 등록 Note인지 확인합니다.
    require acceptedNoteRoot(root)
    RequireActiveUnspent(NOTE, nf)
    # 2. 소비 기록과 부모 암호문을 남기고 Tree append는 하지 않습니다.
    aid = VerifyAndRecord(proof, EXIT, 0, [root, nf], [NoteSpend(nf)], [], audit)
    Emit NoteExited(nf)
    return aid
~~~

ELIGIBLE·WASTE 모두 Exit할 수 있으며 zero-State Note도 기존 조건을 유지합니다. 별도 Exit 역할·allowlist는 없습니다.

잘못된 secret·path·nf·암호문, Frozen·Revoked·이미 소비된 Note는 실패합니다. 성공 후 Tree root·leaf count는 그대로이고 noteSpentIn과 AuditRecord만 추가됩니다. Auditor는 이 경로를 미소비 leaf로 보고하지 않습니다.

## 5. Auditor는 부모와 자손을 어떻게 따라가나요?

### 5.1 어떤 시점을 기준으로 읽나요?

**감사 전체가 같은 snapshot을 읽어야 소비 여부를 일관되게 판단할 수 있습니다.**

감사 시작 시 chain ID·Ledger 주소·block number·block hash를 고정합니다. producerOf·spentIn·AuditRecord 조회는 같은 block number를 사용하고, 시작·종료 시 해당 block hash가 바뀌지 않았는지 확인합니다. RPC가 과거 상태 조회를 지원하지 않거나 block hash가 달라지면 불완전한 감사로 종료합니다. 최신 상태로 조용히 대체하지 않습니다.

순회는 outputRefs·부모 배열 순서의 결정적인 BFS를 사용합니다. 같은 객체는 (objectType, rawId)로 방문 여부를 구분합니다. 복호화 캐시는 해당 Ledger·snapshot으로 한정된 감사 실행 내부에서만 사용합니다. RPC 조회는 실제 호출 횟수를 별도로 기록합니다.

### 5.2 AuditRecord ID만으로 문맥을 어떻게 얻나요?

**암호문은 Record에서 읽고, 암호화 문맥은 해당 기록을 만든 transaction의 공개 입력으로 재계산합니다.**

~~~text
LoadRecord(aid, snapshot):
    # 핵심: 같은 기록의 storage와 성공한 transaction 원본을 대조합니다.
    # 1. snapshot에서 존재하는 기록인지 확인합니다.
    record = getAuditRecord(aid, block=snapshot.number)
    require aid > 0, Event 종류·배열 길이·출력 종류가 명세와 일치
    # 2. Ledger 주소와 indexed aid로 원본 transaction을 찾습니다.
    logs = 배포 block부터 snapshot까지의 AuditRecorded(aid)
    require 일치하는 로그가 정확히 하나
    tx, receipt = 로그의 transaction hash로 조회
    require receipt 성공, 같은 block hash·Ledger 로그·snapshot 이하의 block
    # 3. 직접 호출한 Event의 ABI를 해석하고 storage와 비교합니다.
    require tx.to == 대상 Ledger
    kind, P, audit = 허용된 함수 selector와 calldata를 decode
    require kind·PolicyRef·outputRefs·암호문 == record의 대응값
    L = H(AuditContextTag, kind, P의 원소들)
    return record, L, P
~~~

M7 POC는 Ledger 직접 호출 transaction을 사용합니다. 외부 multicall·내부 contract 호출의 trace 해석은 구현하지 않습니다. 지원하지 않는 호출 원본은 명시적으로 실패시키며, 이를 정상 감사나 미소비로 처리하지 않습니다. 배포 설정에는 Ledger·verifier 주소, bytecode checksum, B1 위원회 공개키 checksum을 기록하고 위원·Auditor가 같은 설정을 사용합니다.

getAuditRecord 조회 실패, 원본 없음, 중복 로그, 잘못된 selector·배열, storage와 다른 ciphertext는 모두 실패입니다. 잘못된 암호문을 다른 문맥으로 복호화하고 계속 진행하지 않습니다.

### 5.3 누가 복호화하고 무엇을 보관하나요?

**위원은 share 자체가 아니라 해당 공개점에 대한 응답을 제공하고, Auditor는 원문만 실행 중 메모리에 보관합니다.**

위원 수 3·threshold 2, 곡선·키·마스크는 B1과 동일합니다. 위원은 서로 다른 ID 두 개를 사용하며 공식 시나리오는 1·2입니다.

$$
D_i=x_iR_1,\qquad
Z=\sum_{i\in S}\lambda_iD_i,\qquad
\lambda_i=\prod_{\substack{h\in S\\h\ne i}}\frac{-h}{i-h}\pmod q
$$

$$
M_j=C_j-H(AuditMaskTag,K,j)\pmod p
$$

K는 3.5절과 같은 Z·L·평문 길이로 유도합니다. 실제 응답 함수는 B1 PartialDecrypt, 결합은 CombineAndDecrypt를 재사용합니다.

~~~text
DecryptRecord(aid, snapshot, cache):
    # 핵심: 한 감사 실행에서 같은 기록을 두 번 복호화하지 않습니다.
    # 1. 메모리의 기존 복원 결과를 확인합니다.
    if cache에 aid가 있으면 return 복호화 원문
    record, L, P = LoadRecord(aid, snapshot)
    # 2. 각 위원이 동일 원본을 확인하고 자기 share로 응답합니다.
    각 위원 ID 1, 2:
        위원 역할 함수가 aid·snapshot으로 원본을 독립 확인
        response[i] = PartialDecrypt(share[i], record.R1)
    # 3. 하나의 암호문으로 다시 이어 원문을 복원합니다.
    C = record.encryptedParents || record.encryptedOutputNfs
    M = CombineAndDecrypt(L, (record.R1, C), 서로 다른 두 response)
    parents = Event의 고정 부모 종류와 M[:부모수]를 결합한 ObjectRef 목록
    outputSpendValues = M[부모수:]
    require 길이·순서가 record.outputRefs와 일치
    # 4. 원문·해석 결과를 이번 감사의 메모리에만 보관합니다.
    cache[aid] = (parents, outputSpendValues)
    return cache[aid]
~~~

정직한 위원회·Auditor 가정이므로 응답 증명은 추가하지 않습니다. 부족한 응답·중복 ID·잘못된 점·형식은 B1의 검사를 유지합니다. 같은 key의 원문 묶음을 복원하면 부모와 모든 출력 소비값을 함께 읽으며 필드별 감사 승인은 만들지 않습니다.

캐시는 DB·파일·다음 감사로 유지하지 않습니다. 그렇다고 이미 원문을 읽은 Auditor가 반드시 잊는다는 보안 보장은 아닙니다.

### 5.4 Backward tracing은 어떻게 동작하나요?

**현재 객체를 만든 기록의 부모를 복원하고, 그 부모의 생성 기록을 반복해서 찾습니다.**

~~~text
TraceBackward(startRef, snapshot):
    # 핵심: 생성 기록의 암호화된 부모를 따라 Entry까지 이동합니다.
    # 1. 감사 실행의 빈 캐시·방문 집합을 준비합니다.
    queue = [startRef]; visitedObjects = {}; expandedRecords = {}; cache = {}
    while queue가 비어 있지 않으면:
        ref = 앞에서 꺼내고, 이미 방문했으면 건너뜁니다.
        aid = producerOf[ref.type][ref.rawId] at snapshot
        require aid가 존재
        record = LoadRecord(aid, snapshot)
        require ref가 record.outputRefs에 정확히 한 번 존재
        # 2. 같은 생성 기록의 부모는 한 번만 확장합니다.
        if aid가 expandedRecords에 있으면 continue
        expandedRecords에 aid 추가
        if record.eventKind == ENTRY:
            Entry 도달로 기록; continue  # 부모가 없으므로 복호화 불필요
        # 3. 실제 부모를 복원해 다음 생성 기록으로 이동합니다.
        parents, _ = DecryptRecord(aid, snapshot, cache)
        각 parent에 대해:
            parentAid = producerOf[parent.type][parent.rawId] at snapshot
            require 0 < parentAid < aid
            parent → record.outputRefs의 관계를 결과에 기록
            queue에 parent 추가
    return 도달한 객체·기록·부모 관계·Entry 목록
~~~

대표 D→B→A 사례는 객체 3개·고유 기록 3개를 방문합니다. Entry는 부모가 없으므로 복호화는 Split 기록 2개, 위원 응답은 4개입니다. 이 숫자는 실제 성능값이 아니라 시나리오 correctness 기대값입니다.

부모가 없는 임의 Event, 존재하지 않는 producer, 순서가 뒤집힌 생성 ID 등 불일치는 실패입니다. 정상적인 Merge의 중복 조상은 방문 집합으로 재사용하며 오류로 간주하지 않습니다.

### 5.5 Forward tracing은 어떻게 동작하나요?

**생성 기록에서 해당 객체의 소비값을 복원하고, 그 소비값이 가리키는 Event의 출력으로 이동합니다.**

~~~text
TraceForward(startRef, snapshot):
    # 핵심: 소비 기록이 없을 때만 snapshot의 미소비 leaf로 분류합니다.
    # 1. 이 방향만의 빈 캐시·방문 집합을 준비합니다.
    queue = [startRef]; visitedObjects = {}; cache = {}
    while queue가 비어 있지 않으면:
        ref = 앞에서 꺼내고, 이미 방문했으면 건너뜁니다.
        producerAid = producerOf[ref.type][ref.rawId] at snapshot
        require producerAid가 존재
        record = LoadRecord(producerAid, snapshot)
        index = outputRefs에서 ref의 유일한 위치
        # 2. 해당 출력이 소비될 때 사용할 값을 복원합니다.
        _, spends = DecryptRecord(producerAid, snapshot, cache)
        s = spends[index]
        consumerAid = 해당 종류의 spentIn[s] at snapshot
        # 3. 성공한 조회값이 0인 경우에만 미소비로 분류합니다.
        if consumerAid == 0:
            leaves에 (ref, s) 추가; continue
        # 4. 실제 소비 기록의 다음 출력으로 이동합니다.
        require consumerAid > producerAid
        consumer, L, P = LoadRecord(consumerAid, snapshot)
        require 해당 종류의 공개 input 소비값 목록에 s가 존재
        if consumer.outputRefs가 비어 있으면:
            require consumer.eventKind == EXIT
            종료 경로로 기록; continue
        ref → consumer.outputRefs의 관계를 기록
        queue에 consumer.outputRefs를 순서대로 추가
    return 미소비 leaves·종료 경로·조회한 관계
~~~

대표 A→B·C→D·E 사례는 객체 5개·고유 기록 3개를 방문하고 C·D·E를 반환합니다. 복호화는 Entry와 두 Split의 3개 기록, 위원 응답은 6개입니다. C나 E가 같은 생성 기록을 다시 만나도 복호화는 반복하지 않습니다.

기록 수와 실제 RPC 호출 수는 다릅니다. 위원별 독립 원본 조회를 포함한 요청 횟수도 측정합니다. 단순 조회 오류를 spentIn=0으로 대체하는 것은 금지합니다.

Merge는 여러 경로가 같은 소비 기록·출력에 도달할 수 있으므로 중복 객체를 제거합니다. Transfer는 공개 outputRefs의 종류에 따라 Note·Voucher mapping을 선택합니다. Voucher는 같은 rvnf를 Proceed·Recall이 공유하므로 실제 성공한 한 기록으로만 이어집니다.

## 6. nf로 어떻게 Freeze·Unfreeze·Revoke하나요?

**Contract는 비공개 cm·rv를 역으로 찾지 않고, 소비 시 이미 공개되는 nf·rvnf의 상태를 확인합니다.**

| 저장값 | 의미 |
|---|---|
| 항목 없음 또는 ACTIVE=0 | 아직 소비되지 않았다면 사용할 수 있습니다. |
| FROZEN=1 | 임시 사용 금지입니다. |
| REVOKED=2 | 영구 사용 금지입니다. |

허용 전이는 Active→Frozen, Frozen→Active, Frozen→Revoked입니다. Active→Revoked, 같은 상태 반복, Revoked에서 나오는 모든 전이는 거부합니다.

API는 setStatus(uint8 objectType, uint256 spendValue, uint8 newStatus)입니다. 기존 M6처럼 constructor에서 지정한 immutable statusAuthority만 호출합니다. admin이 이를 교체하거나 대신 호출하는 우회 기능은 없습니다.

~~~text
setStatus(objectType, spendValue, newStatus):
    # 핵심: 현재 미소비인 nf의 허용된 상태 전이만 실행합니다.
    # 1. 권한·종류·Field 범위를 확인합니다.
    require msg.sender == statusAuthority
    require objectType in {NOTE, VOUCHER} and spendValue < p
    require newStatus in {ACTIVE, FROZEN, REVOKED}
    # 2. 조회 후 먼저 소비된 대상은 이 transaction에서 거부합니다.
    require 해당 spentIn[spendValue] == 0
    oldStatus = 해당 statusByNf[spendValue]
    require (oldStatus, newStatus)가 허용 전이
    # 3. 상태 하나만 변경하고 공개 로그를 남깁니다.
    해당 statusByNf[spendValue] = newStatus
    Emit NullifierStatusChanged(objectType, spendValue, oldStatus, newStatus)
~~~

Status 변경은 Note/Voucher Tree·AuditRecord·spentIn을 바꾸지 않습니다. Unfreeze는 과거 소비를 취소하지 않습니다.

Contract는 이 nf가 특정 cm의 암호문을 복호화한 값인지, 아직 생성되지 않은 임의 nf인지 독립적으로 증명하지 않습니다. 유효한 live 객체를 선정하는 것은 정직한 Authority의 책임입니다. 이를 위해 cm·index나 별도 복호화 proof를 상태 변경 API에 추가하지 않습니다.

감사 프로그램은 snapshot 결과 중 Authority가 선택한 대상을 하나씩 제출합니다. 먼저 소비된 경우 해당 실패를 보고하고 멈추며 다음 자손을 자동으로 재추적·동결하지 않습니다. 다른 대상에 대한 순차 처리 여부와 개별 결과를 기록하며 Batch 원자성을 주장하지 않습니다.

동결 때 공개된 nf와 Unfreeze 후 소비 transaction은 누구나 연결할 수 있습니다. 다음 자손의 소비값까지 자동으로 공개되는 것은 아니며 추가 기록의 복호화가 필요합니다. StatusTree 제거는 모든 오프체인 기능 제거가 아니라 Status path 제공 기능의 제거입니다.

## 7. 무엇을 재사용하고 어디에 구현하나요?

### 기존 의미와 새 구현은 어떻게 나누나요?

| 영역 | 기준 |
|---|---|
| Note·Voucher·Hash·membership·State 배분 | 현재 module의 M1~M5 관계를 그대로 재사용합니다. |
| 감사 암호화·좌표·scalar 검사 | B1 auditcrypto와 AssertEncrypted를 재사용합니다. |
| Process Circuit·keys | B1 감사 Process의 관계·공개 입력 15개·고정 PK가 같으므로 기존 Artifact를 검증 후 사용합니다. |
| 나머지 7개 감사 Circuit | 기존 Define 관계를 호출하는 별도 feature를 만들고 감사 관계를 추가합니다. Entry만 owner secret 검사를 추가합니다. |
| Main Contract | ZkDPPAuditLedger를 새로 작성하고 기존 M6 Contract를 수정하지 않습니다. |
| StatusTree 관련 코드 | 과거 baseline으로 보존하며 새 경로에서 호출하지 않습니다. |
| 감사 프로그램 | Go 함수·CLI와 실행 중 메모리만 사용합니다. |

Source를 재사용하는 것과 기존 PK·VK를 재사용하는 것은 다릅니다. Entry·Exit·Transfer·Proceed·Recall·Merge·Split은 회로 관계가 달라지므로 새 keys가 필요합니다. old verifier를 새 Main Contract에 등록하는 우회를 차단합니다.

### Contract 구성과 기존 API는 무엇을 유지하나요?

constructor에는 Entry·Exit·Transfer·Proceed·Recall·Merge·Split·Process의 감사 verifier 8개, Poseidon2 hasher, statusAuthority를 전달합니다. Admin은 배포자입니다. verifier·hasher는 0 또는 코드 없는 주소를 거부하고 배포 후 고정합니다.

EntryIssuer 관리, Policy Authority 등록·family/version 예약·Record 등록·Grant·disable API는 M5의 의미와 이름을 유지합니다. Process는 감사 Process verifier만 선택할 수 있습니다. M5의 storage-VK 비교 실험을 M7에서 다시 구현·측정하지 않습니다.

Note/Voucher Tree는 depth 32, 현재 leaf·중간 node·zero hash·leaf count·current root·accepted roots를 각각 유지합니다. 빈 leaf는 0이고 zero hash는 level마다 기존 Compress를 적용합니다. 정상 생성 output만 append하고 모든 과거 accepted root는 기존처럼 수용합니다.

~~~text
AppendObject(tree, leaf):
    # 핵심: 공개 output의 현재 leaf·ancestor node를 직접 저장합니다.
    # 1. 용량과 삽입 위치를 확인합니다.
    require tree.leafCount < 2^32
    index = tree.leafCount; position = index; node = leaf
    tree.nodes[0][position] = node
    # 2. 현재 sibling 또는 해당 level의 zero hash로 위까지 계산합니다.
    for level in 0..31:
        sibling = nodes[level][position XOR 1], 없으면 zeroes[level]
        node = position이 짝수 ? Compress(node, sibling) : Compress(sibling, node)
        position = floor(position / 2)
        tree.nodes[level+1][position] = node
    # 3. root·count·accepted 상태를 반영합니다.
    tree.currentRoot = node; tree.acceptedRoots[node] = true
    tree.leafCount += 1
    return index, node
~~~

currentNoteRoot/currentVoucherRoot, acceptedNoteRoot/acceptedVoucherRoot, noteLeafCount/voucherLeafCount, getNotePath/getVoucherPath, commitments/voucherCommitments를 유지합니다. Path는 현재 Tree 기준으로 32개 sibling을 반환하고 과거 path archive는 만들지 않습니다. membership path와 감사 snapshot의 기록 조회는 다른 기능입니다.

### 예정 폴더와 명령은 무엇인가요?

아래는 구현 예정 위치입니다. 이 문서를 작성하면서 코드·폴더·생성물을 만들지는 않습니다.

| 위치 | 구현 목적 |
|---|---|
| internal/core/auditcrypto, internal/circuitutil/audit_encryption.go | B1 암호화·복호화·공통 gadget을 사용합니다. |
| features/audit_entry, audit_exit, audit_transfer, audit_proceed, audit_recall, audit_merge, audit_split | Event별 회로와 설명 README입니다. |
| features/audit_process_3_2 | 기존 B1 감사 Process를 사용합니다. |
| internal/audit | 기록 decoding·snapshot·캐시·부모/자손 탐색입니다. |
| internal/m7case | Note·Voucher·Merge·Process의 연결 테스트 데이터입니다. |
| contracts/src/ZkDPPAuditLedger.sol | 감사 기록·상태 집행이 연결된 새 원장입니다. |
| cmd/setup_m7, evaluate_m7, benchmark_m7, audit_m7 | 개발 keys·proof·측정·단일 감사 실행 명령입니다. |

각 feature README는 목적·공개/비공개·검사·실패 조건·실행 방법·한계를 함께 설명합니다.

| 명령 | 예정 동작 |
|---|---|
| make setup-m7 | B1 위원회·Process Artifact checksum 확인, 새 7개 Circuit compile·개발 Setup·Solidity verifier 준비 |
| make evaluate-m7 | Event별 대표 proof 1회 생성·검증, 연결 사례의 추가 correctness proof 준비·실행 횟수 기록 |
| make test-go | 기존 테스트를 포함한 전체 Go correctness |
| make test-contract-m7 | 기존 회귀와 새 원장의 실제 proof·실패·원자성 확인 |
| make benchmark-m7-gas | 고정 proof로 Event·기록·상태 변경 gas·SSTORE 측정 |
| make benchmark-m7-audit | 원장을 새로 구성하고 두 방향의 실제 원본 조회·복호화·탐색 측정 |
| make benchmark-m7 | gas·audit 개별 명령을 중복 없이 순서대로 실행 |

audit_m7은 RPC URL, Ledger 주소, 배포 manifest, snapshot block, 방향, 시작 객체 종류·rawId를 입력받는 단일 실행 CLI입니다. snapshot block hash를 조회·고정하며 결과·실패를 반환합니다. 복호화 원문은 메모리에서만 보관하고 공개 성능 JSON에는 secret·share·전체 원문을 쓰지 않습니다. 선택 대상의 ref·nf 출력은 승인된 감사용 로컬 결과이며 자동 온체인 공개가 아닙니다.

### Setup·Artifact는 어떻게 보존하나요?

B1 위원회 공개 설정과 세 share를 재사용합니다. 없거나 checksum이 맞지 않으면 명시적으로 실패하고 M7이 임의의 새 위원회를 만들지 않습니다. shares는 기존 제한 권한·Git 제외 규칙을 유지하며 master secret을 복구하지 않습니다.

B1 감사 Process의 CCS·PK·VK·manifest를 다시 읽고 checksum·위원회 PK·공개 입력 순서를 확인합니다. 새 일곱 회로는 artifacts/development/m7의 Feature별 폴더에 개발 CCS·canonical/Lagrange SRS·PK·VK·manifest를 둡니다. Setup은 proof를 생성하지 않습니다.

추가 시나리오 때문에 같은 Circuit을 여러 번 Prove한 경우 대표 성능 1회와 correctness용 proof 횟수를 구분합니다. 성공한 측정 파일을 묵시적으로 덮어쓰거나 실패 시도를 감추지 않습니다.

예정 Raw는 output/m7-circuit.json, m7-anvil-gas.json, m7-audit.json, m7-generated-checksums.json입니다. keys·proof·generated verifier·고정 proof fixture는 Git 제외, Raw는 보존합니다.

## 8. 무엇을 확인하면 M7이 완료되나요?

### 8.1 대표 흐름 외에 어떤 테스트가 필요한가요?

**정상 복원뿐 아니라 추적이 끊기거나 잘못된 동결이 성공하는 경우를 검사합니다.**

| 범위 | 성공·실패 조건 |
|---|---|
| Entry | 승인된 자기 Note 생성, e=0·e>0 성공; 다른 secret·잘못된 nf 암호문·미승인 실패 |
| 암호화 | Native·Circuit·storage 동일, 잘못된 부모·nf·순서·문맥·PK·점·난수 실패 |
| 소비 상태 | 모든 Note 소비 Event가 Frozen·Revoked·이미 소비된 nf 거부 |
| Voucher | Proceed·Recall이 같은 rvnf를 공유하고 다른 secret·opening·deadline 위반·이중 해결 거부 |
| 배분 | Split·Transfer 나머지·전량 전달 zero Change, Merge overflow·같은 Role, Process 기존 수학 유지 |
| 기록 | Event당 ID 하나, 출력 순서·producer·spentIn 일치, Exit 빈 출력·Entry 빈 부모 허용 |
| 원자성 | proof·길이·중복 output·두 번째 append 실패 후 모든 관련 상태 불변 |
| 원본 조회 | 없는 ID·로그·잘못된 receipt·selector·암호문·snapshot 변경은 감사 실패 |
| 추적 | Note·Voucher 종류 분리, Merge의 중복 조상 처리, Exit 종료, 잘못된 producer/consumer 관계 거부 |
| 동결 | 비Authority·금지 전이·기소비 대상 거부; snapshot 이후 소비 경쟁 실패 보고 |
| 보존 | 과거 코드·keys·Raw 유지, 새 경로에 StatusTree·legacy verifier 소비 우회·Batch 없음 |

Voucher 보조 사례는 actor-1→actor-2로 부분 Transfer 후 Proceed, 별도 Note의 전량 Transfer 후 Recall입니다. 기존 600초 epoch 기준 transferEpoch=100, deltaEpoch=6, Recall=105를 사용합니다. 106·107 Recall 실패, 이후 Proceed 허용, deltaEpoch=0 Recall 불가도 검사합니다.

Voucher를 Freeze한 상태에서는 Proceed·Recall이 모두 실패해야 합니다. Unfreeze 후 한 분기가 성공하면 다른 분기는 같은 rvnf 때문에 실패합니다. 전량 Transfer의 zero Change도 추적 결과에 남습니다.

Merge는 같은 owner의 서로 다른 두 Note를 합친 뒤 backward에서 양쪽 Entry, forward에서 합쳐진 output을 중복 없이 찾는지 확인합니다. Process는 M5/B1의 (320,30,230)→(270,30,260)+(30,0,0) 연결 자료를 사용해 양 출력과 세 부모를 복원합니다.

### 8.2 비용은 어떤 단위로 나누나요?

**Participant의 증명, Contract의 기록, 위원회·Auditor의 실제 추적을 구분합니다.**

| 관점 | 측정 구간 | 함께 기록할 내용 |
|---|---|---|
| Participant | 일반 암호화·witness·Prove·Native Verify를 각각 측정 | constraints·public 개수·proof binary/Solidity 크기·compile/Setup·할당량 |
| Contract | 승인·Policy·배포와 8개 Event·상태 변경 receipt | gas·calldata·storage layout·SSTORE count/opcode gas |
| 위원회 | 원본 확인과 각 partial decryption을 분리 | ID별 응답 수·계산 시간·원본 조회 시간 |
| Auditor | Record 조회·응답 결합·키/마스크 복원·탐색·전체 | 방문 객체·고유 기록·실제 복호화·캐시 hit·RPC 횟수·오류 |

Forward와 backward는 서로 다른 빈 메모리 캐시로 측정합니다. 한 방향 내부의 캐시 재사용은 허용하고 횟수를 기록합니다. 위원 원본 조회와 Auditor 조회를 포함한 실제 전체 시간도 기록하되 겹치는 시간을 두 번 더하지 않습니다.

대표 그래프의 기대값은 backward 고유 기록 3·복호화 2·응답 4, forward 고유 기록 3·복호화 3·응답 6입니다. 논리 방문 수와 RPC 호출 횟수는 구분합니다. 작은 그래프의 단일 값으로 큰 공급망의 확장 성능을 단정하지 않습니다.

저장 gas는 AuditRecord의 metadata·두 암호문 배열·outputRefs·producer·spentIn·Tree를 구분해 설명합니다. trace의 SSTORE 합과 전체 receipt gas는 다릅니다. B1의 15개 입력 저장 gas를 빼거나 Field 개수에 비례시켜 M7 절감량으로 주장하지 않습니다.

### 8.3 환경과 실행 횟수는 어떻게 고정하나요?

- Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, PLONK-KZG/BLS12-381·Jubjub·기존 Poseidon2입니다.
- GOMAXPROCS=8이며 proof·위원 연산은 순차 실행합니다. CPU·OS·버전·요청/실제 thread 수를 기록합니다.
- Foundry·Anvil 1.7.1, Prague, chain ID 31337, 초기 timestamp 60000, 30M block gas limit, port 18545입니다.
- 별도 Compose project zkdpp-m7만 생성·정리합니다. 이미 점유된 port나 다른 프로젝트를 중단하지 않습니다. gas trace는 B1에서 확인한 steps-tracing을 사용합니다.
- 고정 proof gas와 감사 추적은 서로 다른 clean chain에서 각각 수행합니다. 측정 시작 전 배포·원장 준비를 끝내며 감사 측정에는 실제 RPC 조회·복호화·탐색을 포함합니다.
- 공식 성능은 case별 1회입니다. 같은 Circuit의 연결용 추가 proof·실패·재시도는 별도 횟수로 남깁니다. B1의 1·10·100·1,000개 오프라인 복호화 실험은 반복하지 않습니다.
- 기존 코드·Artifact·Raw의 checksum을 실행 전에 캡처하고 종료 시 대조합니다. 문서 작성 단계에서는 측정을 실행하지 않습니다.

### 8.4 무엇을 결과 문서에 남기나요?

구현 완료 후 M7-audit-tracing-result.md를 [Result Template](RESULT-TEMPLATE.md)에 따라 작성합니다. 첫 화면에서 다음을 알 수 있어야 합니다.

1. 실제로 연결한 8개 Event와 AuditRecord 저장 범위
2. Entry→Split→Split에서 복원한 부모·자손·미소비 leaf
3. Freeze·Unfreeze·Revoke와 소비 결과
4. Participant·Contract·위원회·Auditor의 실제 비용
5. 실패·재시도·한계·기존 M6와 달라진 점

정확한 원문 복원·public input 순서·저장 원본·생성/소비 mapping·상태 변경을 모두 대조하고 실제 값이 Raw와 일치해야 완료입니다. Result가 없거나 하나의 Event라도 암호화·상태 검사를 우회하면 M7을 완료로 표시하지 않습니다.

### 8.5 어디에서 멈추나요?

Claim·DPP, ProductProfile 확장, 원자재 현실 진실성 증명, DKG·키 회전, 영구 감사 캐시·DB·위원 HTTP 서버, 자동 자손 동결·Batch, M6 상태 migration, Besu·최종 universal SRS는 제외합니다.

이번 파일은 **구현 명세**입니다. 완료된 구현·검증·측정의 실제 값과 차이는 Result에서 관리합니다. 자동 커밋하지 않습니다.
