# M3 Transfer·Voucher 구현 명세

- 명세 상태: 구현 기준 동결
- 이전 단계: [M2 Entry·Exit Ledger Result](M2-entry-exit-ledger-result.md)
- 미래 Context: [M3 — Transfer와 Voucher 생애주기](FUTURE-MILESTONE-CONTEXT.md#m3)
- 구현 결과: [M3 Transfer·Voucher Result](M3-transfer-voucher-result.md)

이 문서는 M3에서 무엇을 왜, 어떤 관계와 Interface로 구현할지 고정합니다. 구현자는 다른 설계 문서를 오가지 않아도 이 문서만으로 Circuit·Contract·fixture·test와 측정을 작성할 수 있어야 합니다.

## 0. 30초 안에 설계 파악하기

```text
M2까지:
  Entry: ∅ → Note
  Exit:  Note → ∅

M3에서 추가:
  Transfer: Note → Voucher + Change Note
  Proceed:  Voucher → Receiver Note
  Recall:   Voucher → Sender Note
```

Voucher는 아직 끝나지 않은 전달을 나타냅니다. Transfer는 private Note를 소비하고 public Voucher commitment와 public Change Note commitment를 원자적으로 생성합니다. Proceed와 Recall은 소비하는 Voucher commitment를 공개하지 않고, 같은 Voucher nullifier를 사용해 둘 중 하나만 성공합니다.

| 질문 | M3 결정 |
|---|---|
| 전달 질량은 누가 정합니까? | Sender가 private `q_voucher`를 선택합니다. |
| `a_rec`, `e`는 어떻게 나눕니까? | 질량 비례로 나누며 Change를 내림 계산하고 Voucher가 residual을 받습니다. |
| 전량 Transfer는 어떻게 합니까? | 질량 0 Change Note도 생성해 항상 같은 출력 구조를 유지합니다. |
| 운송 탄소를 더합니까? | M3에서는 더하지 않습니다. |
| 어떤 Role을 전달합니까? | ELIGIBLE과 WASTE를 모두 허용하고 Role을 그대로 복사합니다. |
| 새 `rv`는 공개합니까? | Transfer output이므로 공개해 Voucher Tree에 append합니다. |
| 소비하는 `rv`는 공개합니까? | Proceed·Recall에서는 private witness입니다. |
| 어떻게 한 번만 해결합니까? | 공유 opening과 private `rv`로 같은 public `rvnf`를 계산합니다. |
| deadline은 어디서 확인합니까? | private Voucher의 `deadlineEpoch`을 Recall Circuit에서 확인합니다. |

## 1. M2에서 무엇이 달라지나요?

| M2 | M3에서 추가 | 완료 후 가능 |
|---|---|---|
| Note Tree만 존재 | 별도 Voucher Tree | pending Transfer 등록 |
| Note nullifier만 존재 | Voucher nullifier | Proceed·Recall 중 한 번만 해결 |
| Note 생성·종료 | Transfer·Proceed·Recall | 다른 ZK owner에게 Note 전달 |
| Private Spend Kernel | Transfer 안에서 Note 소비 관계 재사용 | input `cm`을 숨긴 Transfer |
| deadline 상태 없음 | Voucher에 private deadline binding | 기한 전 Recall과 기한 무관 Proceed |

M3는 M1 Private Spend Kernel의 계산 관계를 Transfer Circuit 안에서 재사용하지만 M1 PK·VK를 조합하지 않습니다. Transfer는 새로운 하나의 Circuit이므로 별도 CCS·PK·VK를 생성합니다. Proceed와 Recall도 각각 독립 Circuit과 key를 가집니다.

## 2. Canonical 시나리오와 고정 데이터

### 역할

| 참여자 | 역할 |
|---|---|
| Actor 1 | 두 Note의 Sender |
| Actor 2 | 부분 Transfer의 Receiver |
| Actor 3 | 전량 Transfer의 Receiver이지만 완료 전에 Sender가 Recall |

ZK identity는 `testdata/common/actors-v1.json`을 사용합니다. EVM transaction sender와 ZK owner는 암호학적으로 binding하지 않습니다. Voucher와 $o_{rv}$를 Sender·Receiver가 off-chain으로 공유하는 과정은 고정 fixture로만 재현하며 암호화 전달 프로토콜은 구현하지 않습니다.

### 시나리오 A: 부분 Transfer 후 Proceed

입력 ELIGIBLE Note의 State는 다음 scaled integer를 사용합니다.

| 값 | 실제 단위 | scaled integer |
|---|---:|---:|
| `q_mass` | 3 kg | 3,000,000,000 |
| `a_rec` | 1 kg | 1,000,000,000 |
| `e` | 2 kgCO2e | 2,000,000,000 |

Actor 1은 1 kg을 Actor 2에게 전달합니다.

| 출력 | `q_mass` | `a_rec` | `e` |
|---|---:|---:|---:|
| Voucher | 1,000,000,000 | 333,333,334 | 666,666,667 |
| Change Note | 2,000,000,000 | 666,666,666 | 1,333,333,333 |

`a_rec`과 `e`의 나눗셈 residual은 첫 output인 Voucher에 들어갑니다. Actor 2는 Proceed하여 Voucher와 같은 DocumentHash·Role·State를 가진 Receiver Note를 생성합니다.

### 시나리오 B: 전량 Transfer 후 Recall

다른 ELIGIBLE Note 전체를 Actor 3에게 Transfer합니다.

```text
Voucher:
  입력 State 전체

Change Note:
  q_mass = 0
  a_rec  = 0
  e      = 0
```

Actor 1은 deadline 전에 Recall하여 Voucher State 전체를 가진 Sender Note를 다시 생성합니다. Actor 3의 이후 Proceed는 같은 `rvnf`가 이미 사용됐으므로 실패합니다.

### Epoch fixture

```text
EPOCH_SIZE   = 600 seconds
transferEpoch = 100
deltaEpoch    = 6
deadlineEpoch = 106
recallEpoch   = 105
```

별도 negative fixture는 `currentEpoch=106`과 `107`에서 Recall이 실패하고 Proceed는 성공할 수 있음을 검증합니다.

## 3. 공통 Voucher·State·소유 관계

### Note와 State

M3는 기존 Note를 그대로 사용합니다.

```text
Note
  ├─ DocumentHash
  ├─ AssetRole
  ├─ q_mass
  ├─ a_rec
  ├─ e
  ├─ address
  └─ opening
```

| State | 단위·범위 |
|---|---|
| `q_mass` | kg × $10^9$, uint64 |
| `a_rec` | kg × $10^9$, uint64 |
| `e` | kgCO2e × $10^9$, uint64 |

모든 객체는 `a_rec <= q_mass`를 만족합니다. WASTE는 `a_rec=0`, `e=0`을 만족합니다. Transfer는 두 AssetRole을 모두 허용하지만 Voucher·Change·Proceed output·Recall output의 Role을 바꾸지 않습니다.

### Voucher

Voucher Field 순서는 다음으로 고정합니다.

```text
Voucher
  ├─ DocumentHash
  ├─ AssetRole
  ├─ q_mass
  ├─ a_rec
  ├─ e
  ├─ senderAddress
  ├─ receiverAddress
  ├─ deadlineEpoch
  └─ opening
```

```text
VoucherTag          = "zkDPP:Voucher:v1"
VoucherNullifierTag = "zkDPP:VoucherNullifier:v1"
```

두 문자열은 M1 Domain과 같은 hash-to-field 방식으로 Circuit constant가 됩니다. Domain은 public input이나 private witness가 아닙니다.

$$
rv=
H(
\mathrm{VoucherTag},
\mathrm{DocumentHash},
\mathrm{AssetRole},
q_{\mathrm{mass}},
a_{\mathrm{rec}},
e,
\mathrm{senderAddress},
\mathrm{receiverAddress},
\mathrm{deadlineEpoch},
o_{rv}
)
$$

$$
rvnf=
H(
\mathrm{VoucherNullifierTag},
o_{rv},
rv
)
$$

$o_{rv}$는 Sender와 Receiver가 공유하는 Voucher opening입니다. Sender와 Receiver는 서로 다른 `sk_owner`를 가지지만 같은 $o_{rv}$와 `rv`를 사용하므로 같은 `rvnf`를 계산합니다.

### Nullifier와 권한의 차이

| 관계 | 검증 목적 |
|---|---|
| `sk_owner → address` | Proceed는 Receiver, Recall은 Sender가 맞는지 확인 |
| `Voucher payload + o_rv → rv` | 공유 opening이 등록된 Voucher에 binding됐는지 확인 |
| `o_rv + rv → rvnf` | Proceed·Recall이 동일한 해결 ID를 사용하도록 강제 |

$o_{rv}$만 안다고 Proceed·Recall할 수 없습니다. 역할에 맞는 `sk_owner`와 Voucher Tree membership을 함께 증명해야 합니다.

### Voucher Tree

- Note Tree와 독립된 Depth-32 append-only Tree입니다.
- Transfer가 생성한 public `rvNew`를 append합니다.
- Proceed·Recall은 private `rv`와 private path로 public `voucherRoot` membership을 증명합니다.
- 현재 path만 Contract에서 조회하며 과거 root별 path snapshot은 저장하지 않습니다.
- 정상적으로 생성된 과거 Voucher root는 accepted root로 유지합니다.

## 4. Transfer

### 목적과 전이

```text
Transfer:
  Note → Voucher + Change Note
```

Transfer는 Sender가 가진 private Note를 한 번 소비하고, Receiver가 해결할 pending Voucher와 Sender에게 남는 Change Note를 원자적으로 만듭니다.

### 공개·비공개값

Public input 순서는 다음으로 고정합니다.

```text
1. noteRoot
2. nf
3. rvNew
4. cmChange
5. transferEpoch
6. deltaEpoch
```

| 공개값·calldata | 비공개 witness |
|---|---|
| `noteRoot`, `nf` | input Note, `sk_owner`, private input `cm`, index, sibling 32개 |
| `rvNew`, `cmChange` | Voucher·Change Note 전체와 각 opening |
| `transferEpoch`, `deltaEpoch` | `deadlineEpoch`, `q_voucher`, State remainder |

소비하는 input `cm`은 public input과 calldata에 포함하지 않습니다. `receiverAddress`, 전달 질량과 정확한 State 배분도 commitment 안에 숨깁니다.

### State 배분

Sender는 다음 범위의 private `q_voucher`를 선택합니다.

$$
0<q_{voucher}\le q_{input}
$$

$$
q_{change}=q_{input}-q_{voucher}
$$

각 $x\in\{a_{rec},e\}$에 대해 Change를 내림 계산합니다.

$$
x_{change}
=
\left\lfloor
\frac{x_{input}q_{change}}{q_{input}}
\right\rfloor
$$

$$
x_{voucher}=x_{input}-x_{change}
$$

Circuit은 private remainder $r_x$로 내림 관계를 검증합니다.

$$
q_{change}x_{input}
=
q_{input}x_{change}+r_x
$$

$$
0\le r_x<q_{input}
$$

두 uint64 곱은 최대 128-bit이며 BLS12-381 scalar field보다 작습니다. Circuit은 각 operand의 uint64 범위를 별도로 확인합니다.

### Native fixture 수도 코드

```text
# 핵심: Sender가 전송 질량을 정하고 Change를 내림 계산해 Voucher에 residual을 줍니다.
function BuildTransfer(inputNote, senderSecret, receiverAddress,
                       qVoucher, transferEpoch, deltaEpoch,
                       voucherOpening, changeOpening):
    # 1. input 소유자와 전달 질량을 확인합니다.
    assert Address(senderSecret) == inputNote.address
    assert 0 < qVoucher <= inputNote.q_mass
    qChange = inputNote.q_mass - qVoucher

    # 2. a_rec과 e의 Change 몫·remainder를 계산합니다.
    for x in [a_rec, e]:
        xChange, remainder[x] = DivRem(inputNote.x * qChange, inputNote.q_mass)
        xVoucher = inputNote.x - xChange

    # 3. deadline을 계산하고 uint64 범위를 확인합니다.
    deadlineEpoch = transferEpoch + deltaEpoch
    assert deadlineEpoch fits uint64

    # 4. Role과 DocumentHash를 유지한 두 output을 만듭니다.
    voucher = Voucher(
        inputNote.DocumentHash, inputNote.AssetRole,
        qVoucher, aVoucher, eVoucher,
        inputNote.address, receiverAddress,
        deadlineEpoch, voucherOpening,
    )
    change = Note(
        inputNote.DocumentHash, inputNote.AssetRole,
        qChange, aChange, eChange,
        inputNote.address, changeOpening,
    )

    # 5. public output과 private fixture를 반환합니다.
    return VoucherCommitment(voucher), NoteCommitment(change),
           voucher, change, remainder
```

### Transfer Circuit 수도 코드

```text
# 핵심: private input Note를 올바르게 소비하고 비례 배분된 Voucher와 Change를 만듭니다.
function TransferCircuit(public, private):
    # 1. public input 순서와 epoch 범위를 확인합니다.
    [noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch] = public
    assert transferEpoch fits uint64
    assert deltaEpoch fits uint64

    # 2. 기존 Private Spend 관계로 Note 소유·membership·nullifier를 확인합니다.
    ValidateNote(private.inputNote)
    expectedSender = H(OwnerTag, private.senderSecret)
    assert private.inputNote.address == expectedSender
    inputCM = NoteCommitment(private.inputNote)
    assert MerkleRoot(inputCM, private.notePath) == noteRoot
    assert H(NullifierTag, private.senderSecret, inputCM) == nf

    # 3. 질량 분할과 output 기본 조건을 확인합니다.
    assert private.qVoucher > 0
    assert private.qVoucher fits uint64
    assert private.qChange fits uint64
    assert private.inputNote.q_mass == private.qVoucher + private.qChange
    assert private.voucher.DocumentHash == private.inputNote.DocumentHash
    assert private.change.DocumentHash == private.inputNote.DocumentHash
    assert private.voucher.AssetRole == private.inputNote.AssetRole
    assert private.change.AssetRole == private.inputNote.AssetRole
    assert private.voucher.q_mass == private.qVoucher
    assert private.change.q_mass == private.qChange

    # 4. a_rec과 e의 floor·residual·합 보존을 확인합니다.
    for x in [a_rec, e]:
        assert private.inputNote.q_mass * private.change.x
               + private.remainder[x]
               == private.qChange * private.inputNote.x
        assert 0 <= private.remainder[x] < private.inputNote.q_mass
        assert private.inputNote.x == private.voucher.x + private.change.x

    # 5. 두 output State와 소유 관계를 확인합니다.
    ValidateStateAndRole(private.voucher)
    ValidateNote(private.change)
    assert private.voucher.senderAddress == expectedSender
    assert private.change.address == expectedSender

    # 6. deadline 관계를 Voucher에 binding합니다.
    assert private.deadlineEpoch fits uint64
    assert private.deadlineEpoch == transferEpoch + deltaEpoch
    assert private.voucher.deadlineEpoch == private.deadlineEpoch

    # 7. 두 public output commitment를 다시 계산합니다.
    assert VoucherCommitment(private.voucher) == rvNew
    assert NoteCommitment(private.change) == cmChange
```

`qChange=0`이면 floor 식에 따라 Change의 `a_rec=0`, `e=0`이고 Voucher가 입력 State 전체를 받습니다. Change Note 자체는 fresh opening으로 고유한 commitment를 가집니다.

### Transfer Contract 수도 코드와 상태 변화

```text
# 핵심: Note를 한 번 소비하고 Change Note와 Voucher를 서로 다른 Tree에 원자적으로 append합니다.
function transfer(proof, noteRoot, nf, rvNew, cmChange,
                  transferEpoch, deltaEpoch):
    # 1. 저렴한 public precondition을 먼저 확인합니다.
    require acceptedNoteRoots[noteRoot]
    require noteNullifiers[nf] == false
    require commitments[cmChange] == false
    require voucherCommitments[rvNew] == false
    require transferEpoch == currentEpoch()

    # 2. 정해진 public input 순서로 proof를 검증합니다.
    verify TransferProof(
        noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch
    )

    # 3. proof가 성공한 뒤에만 두 객체 상태를 갱신합니다.
    noteNullifiers[nf] = true
    commitments[cmChange] = true
    voucherCommitments[rvNew] = true
    appendNoteTree(cmChange)
    appendVoucherTree(rvNew)
```

Note Tree append가 성공한 뒤 Voucher Tree append가 실패해도 EVM transaction 전체가 revert되어 앞선 mapping·Tree·Event 변경도 남지 않아야 합니다.

### Transfer 실패·Atomicity·측정

다음 입력은 실패해야 합니다.

- 허용되지 않은 Note root, 이미 사용된 `nf`, 중복 `cmChange`·`rvNew`
- 잘못된 sender secret·Note path·input Note
- `q_voucher=0`, `q_voucher>q_input`, uint64 범위 초과
- 잘못된 floor quotient·remainder·State 합
- 변경된 DocumentHash·AssetRole·sender address
- 잘못된 `deadlineEpoch`, `transferEpoch`, `deltaEpoch`
- 잘못된 Voucher·Change opening 또는 public commitment

측정은 Transfer Circuit과 transaction을 분리합니다. Circuit에는 Note membership·State 배분·두 commitment가 포함되며 Contract gas에는 Note Tree와 Voucher Tree append가 모두 포함됩니다.

## 5. Proceed

### 목적과 전이

```text
Proceed:
  Voucher → Receiver Note
```

Receiver는 자신이 대상인 private Voucher를 final Note로 바꿉니다. `rv`는 public input이나 calldata에 포함하지 않습니다.

### 공개·비공개값

Public input 순서는 다음으로 고정합니다.

```text
1. voucherRoot
2. rvnf
3. cmReceiver
```

| 공개값·calldata | 비공개 witness |
|---|---|
| `voucherRoot`, `rvnf`, `cmReceiver` | Voucher 전체, private `rv`, $o_{rv}$, index, sibling 32개 |
| 없음 | Receiver `sk_owner`, Receiver Note와 fresh opening |

Proceed는 deadline을 검사하지 않습니다. deadline 전후 모두 아직 해결되지 않은 Voucher를 Proceed할 수 있습니다.

### Native fixture 수도 코드

```text
# 핵심: Receiver가 private Voucher State를 그대로 새 Note로 확정합니다.
function BuildProceed(voucher, receiverSecret, outputOpening):
    # 1. Receiver 권한과 Voucher commitment를 확인합니다.
    receiverAddress = H(OwnerTag, receiverSecret)
    assert receiverAddress == voucher.receiverAddress
    rv = VoucherCommitment(voucher)

    # 2. 공유 opening으로 Voucher nullifier를 만듭니다.
    rvnf = H(VoucherNullifierTag, voucher.opening, rv)

    # 3. Voucher 내용을 바꾸지 않고 Receiver Note를 만듭니다.
    output = Note(
        voucher.DocumentHash, voucher.AssetRole,
        voucher.q_mass, voucher.a_rec, voucher.e,
        receiverAddress, outputOpening,
    )
    return rvnf, NoteCommitment(output), output
```

### Proceed Circuit 수도 코드

```text
# 핵심: 등록된 private Voucher와 Receiver 권한을 증명하고 같은 State의 Note를 만듭니다.
function ProceedCircuit(public, private):
    # 1. public input을 정해진 순서로 읽습니다.
    [voucherRoot, rvnf, cmReceiver] = public

    # 2. private Voucher commitment와 membership을 확인합니다.
    ValidateStateAndRole(private.voucher)
    rv = VoucherCommitment(private.voucher)
    assert MerkleRoot(rv, private.voucherPath) == voucherRoot

    # 3. Receiver의 역할 권한을 확인합니다.
    receiverAddress = H(OwnerTag, private.receiverSecret)
    assert receiverAddress == private.voucher.receiverAddress

    # 4. 공유 opening과 private rv로 public rvnf를 다시 계산합니다.
    assert H(VoucherNullifierTag, private.voucher.opening, rv) == rvnf

    # 5. Voucher의 제품·Role·State를 그대로 Receiver Note로 옮깁니다.
    ValidateNote(private.outputNote)
    assert private.outputNote.DocumentHash == private.voucher.DocumentHash
    assert private.outputNote.AssetRole == private.voucher.AssetRole
    assert private.outputNote.q_mass == private.voucher.q_mass
    assert private.outputNote.a_rec == private.voucher.a_rec
    assert private.outputNote.e == private.voucher.e
    assert private.outputNote.address == receiverAddress

    # 6. 새 public Note commitment를 확인합니다.
    assert NoteCommitment(private.outputNote) == cmReceiver
```

### Proceed Contract 수도 코드와 상태 변화

```text
# 핵심: private Voucher를 공개하지 않고 한 번 해결한 뒤 Receiver Note를 append합니다.
function proceed(proof, voucherRoot, rvnf, cmReceiver):
    # 1. public precondition만 확인합니다.
    require acceptedVoucherRoots[voucherRoot]
    require voucherNullifiers[rvnf] == false
    require commitments[cmReceiver] == false

    # 2. rv가 없는 public statement로 proof를 검증합니다.
    verify ProceedProof(voucherRoot, rvnf, cmReceiver)

    # 3. 해결 상태와 새 Note를 원자적으로 기록합니다.
    voucherNullifiers[rvnf] = true
    commitments[cmReceiver] = true
    appendNoteTree(cmReceiver)
```

Contract는 `voucherCommitments[rv]` 또는 `deadlineRV[rv]`를 조회하지 않습니다. 등록된 Voucher인지는 Circuit membership이 검증합니다.

### Proceed 실패·Atomicity·측정

다음 입력은 실패해야 합니다.

- 허용되지 않은 Voucher root, 이미 사용된 `rvnf`, 중복 output commitment
- 잘못된 Voucher opening·path·private `rv`
- Receiver가 아닌 `sk_owner`
- 변경된 DocumentHash·AssetRole·State·output address
- 잘못된 public `rvnf` 또는 `cmReceiver`

실패하면 `voucherNullifiers`, `commitments`, Note Tree root와 leaf count가 모두 변하지 않아야 합니다.

## 6. Recall

### 목적과 전이

```text
Recall:
  Voucher → Sender Note
```

Sender는 deadline 이전에 아직 해결되지 않은 private Voucher를 회수합니다. `rv`와 `deadlineEpoch`은 private witness입니다.

### 공개·비공개값

Public input 순서는 다음으로 고정합니다.

```text
1. voucherRoot
2. rvnf
3. cmReturn
4. currentEpoch
```

| 공개값·calldata | 비공개 witness |
|---|---|
| `voucherRoot`, `rvnf`, `cmReturn`, `currentEpoch` | Voucher 전체, private `rv`, private `deadlineEpoch`, $o_{rv}$, path |
| 없음 | Sender `sk_owner`, Return Note와 fresh opening |

### Native fixture 수도 코드

```text
# 핵심: Sender가 deadline 전에 private Voucher State를 그대로 회수합니다.
function BuildRecall(voucher, senderSecret, currentEpoch, outputOpening):
    # 1. Sender 권한과 deadline을 확인합니다.
    senderAddress = H(OwnerTag, senderSecret)
    assert senderAddress == voucher.senderAddress
    assert currentEpoch < voucher.deadlineEpoch

    # 2. private Voucher와 공통 resolution nullifier를 계산합니다.
    rv = VoucherCommitment(voucher)
    rvnf = H(VoucherNullifierTag, voucher.opening, rv)

    # 3. Voucher 내용을 바꾸지 않고 Sender Note를 만듭니다.
    output = Note(
        voucher.DocumentHash, voucher.AssetRole,
        voucher.q_mass, voucher.a_rec, voucher.e,
        senderAddress, outputOpening,
    )
    return rvnf, NoteCommitment(output), output
```

### Recall Circuit 수도 코드

```text
# 핵심: 등록된 private Voucher와 Sender 권한·deadline을 증명하고 같은 State를 회수합니다.
function RecallCircuit(public, private):
    # 1. public input과 currentEpoch 범위를 확인합니다.
    [voucherRoot, rvnf, cmReturn, currentEpoch] = public
    assert currentEpoch fits uint64

    # 2. private Voucher commitment와 membership을 확인합니다.
    ValidateStateAndRole(private.voucher)
    rv = VoucherCommitment(private.voucher)
    assert MerkleRoot(rv, private.voucherPath) == voucherRoot

    # 3. Sender의 역할 권한을 확인합니다.
    senderAddress = H(OwnerTag, private.senderSecret)
    assert senderAddress == private.voucher.senderAddress

    # 4. 공유 opening과 private rv로 public rvnf를 확인합니다.
    assert H(VoucherNullifierTag, private.voucher.opening, rv) == rvnf

    # 5. private deadline과 public current epoch을 비교합니다.
    assert private.voucher.deadlineEpoch fits uint64
    assert currentEpoch < private.voucher.deadlineEpoch

    # 6. Voucher의 제품·Role·State를 Sender Note로 그대로 옮깁니다.
    ValidateNote(private.outputNote)
    assert private.outputNote.DocumentHash == private.voucher.DocumentHash
    assert private.outputNote.AssetRole == private.voucher.AssetRole
    assert private.outputNote.q_mass == private.voucher.q_mass
    assert private.outputNote.a_rec == private.voucher.a_rec
    assert private.outputNote.e == private.voucher.e
    assert private.outputNote.address == senderAddress
    assert NoteCommitment(private.outputNote) == cmReturn
```

### Recall Contract 수도 코드와 상태 변화

```text
# 핵심: block epoch과 public currentEpoch을 고정한 뒤 private deadline proof를 검증합니다.
function recall(proof, voucherRoot, rvnf, cmReturn, currentEpoch):
    # 1. public precondition과 현재 시간을 확인합니다.
    require acceptedVoucherRoots[voucherRoot]
    require voucherNullifiers[rvnf] == false
    require commitments[cmReturn] == false
    require currentEpoch == this.currentEpoch()

    # 2. private rv·deadline을 드러내지 않는 proof를 검증합니다.
    verify RecallProof(voucherRoot, rvnf, cmReturn, currentEpoch)

    # 3. 해결 상태와 Return Note를 원자적으로 기록합니다.
    voucherNullifiers[rvnf] = true
    commitments[cmReturn] = true
    appendNoteTree(cmReturn)
```

`currentEpoch == deadlineEpoch`이면 Circuit의 strict inequality가 실패합니다. Proceed에는 이 검사가 없으므로 deadline 이후에도 가능합니다.

### Recall 실패·Atomicity·측정

Proceed의 공통 실패 조건에 다음을 추가합니다.

- Receiver secret을 사용한 Recall
- `currentEpoch >= deadlineEpoch`
- calldata `currentEpoch`과 실행 block epoch 불일치
- Voucher에 binding되지 않은 private deadline

실패 후 `voucherNullifiers`, output commitment와 Note Tree는 모두 불변이어야 합니다.

## 7. Voucher Tree와 Contract 전체 상태

### 최소 constructor와 verifier

M3 Ledger constructor는 다음 dependency를 받는 방향으로 확장합니다.

```text
constructor(
  entryVerifier,
  privateSpendVerifier,
  transferVerifier,
  proceedVerifier,
  recallVerifier,
  fieldHasher
)
```

Entry와 Exit 의미는 변경하지 않습니다. M3 구현 과정에서 Router 일반화나 verifier registry는 만들지 않습니다.

### 새 Contract 상태

| 상태 | 역할 | 변경 Event |
|---|---|---|
| `voucherCommitments[rv]` | 생성된 Voucher 중복 방지 | Transfer |
| `voucherNullifiers[rvnf]` | Proceed·Recall 이중 해결 방지 | Proceed·Recall |
| `voucherTreeNodes` | 현재 Voucher leaf·중간 node | Transfer |
| `voucherZeroes` | Voucher Tree의 level별 empty Hash | deployment |
| `acceptedVoucherRoots` | 정상 생성된 현재·과거 root | Transfer |
| `currentVoucherRoot` | 최신 Voucher root | Transfer |
| `voucherLeafCount` | 다음 Voucher append index | Transfer |

`deadlineRV[rv]`는 만들지 않습니다. `voucherCommitments`는 Transfer output의 유일성 확인에만 사용하며 resolution에서 private `rv` 조회에 사용하지 않습니다.

### 최소 public Interface

```text
transfer(proof, noteRoot, nf, rvNew, cmChange, transferEpoch, deltaEpoch)
proceed(proof, voucherRoot, rvnf, cmReceiver)
recall(proof, voucherRoot, rvnf, cmReturn, currentEpoch)

getVoucherPath(index)
currentVoucherRoot()
acceptedVoucherRoot(root)
voucherLeafCount()
voucherCommitments(rv)
voucherNullifiers(rvnf)
currentEpoch()
```

### Event

```text
VoucherAppended(rv, index, root)
```

Change·Proceed·Recall output은 기존 `NoteAppended`를 사용합니다. resolution 종류는 transaction function으로 이미 드러나므로 별도 `VoucherResolved` Event는 만들지 않습니다.

### Voucher Tree append 수도 코드

```text
# 핵심: M2 Note Tree와 동일한 규칙으로 public rv를 독립 Tree에 append합니다.
function appendVoucherTree(rv):
    # 1. 다음 leaf 위치를 확인합니다.
    index = voucherLeafCount
    require index < 2^32

    # 2. leaf에서 root까지 현재 node와 sibling을 압축합니다.
    current = rv
    position = index
    store voucherTreeNodes[level=0, position] = current
    for level in 0..31:
        sibling = existing node or voucherZeroes[level]
        current = Poseidon2Compress(left, right according to position bit)
        position = floor(position / 2)
        store voucherTreeNodes[level+1, position] = current

    # 3. 새 root와 leaf count를 기록하고 Event를 발생시킵니다.
    voucherLeafCount = index + 1
    currentVoucherRoot = current
    acceptedVoucherRoots[current] = true
    emit VoucherAppended(rv, index, current)
```

두 번째 on-chain Tree 사용이므로 M2 Tree 코드를 의미 변경 없이 공통 helper로 추출할 수 있습니다. 다만 Note Tree와 Voucher Tree storage는 분리하고, 추가 추상화는 하지 않습니다.

## 8. 구현 구조·Artifact·명령

### 예정 코드 구조

```text
internal/core/voucher/
  Voucher·commitment·nullifier native 계산

features/transfer/
features/proceed/
features/recall/
  README·Circuit·positive/negative test

cmd/setup_m3/
  세 Circuit compile·개발용 Setup·Solidity verifier export

cmd/benchmark_m3/
  canonical fixture·Anvil gas·live E2E
```

M1·M2의 다음 기반을 재사용합니다.

- Go·gnark version과 PLONK-KZG/BLS12-381 runtime
- Poseidon2 Hash·Domain 변환
- Note State·Role·Commitment
- `sk_owner → address`, Note nullifier와 Depth-32 membership
- Solidity verifier export·checksum·Anvil transaction runner
- on-chain Note Tree와 두 번째 사용에 맞춘 Tree helper

다음은 새로 구현합니다.

- Voucher native model·Domain·commitment·nullifier
- Voucher membership witness와 Tree
- Transfer·Proceed·Recall Circuit·PK·VK·verifier
- M3 Ledger 함수와 benchmark scenario

### 개발 Artifact

```text
artifacts/development/m3/
  transfer/
  proceed/
  recall/
```

각 폴더에는 CCS, development canonical·Lagrange SRS, PK, VK, Solidity verifier, fixed proof fixture와 manifest를 둡니다. 재생성 가능한 Artifact는 Git에서 제외합니다. final universal SRS는 M9 범위입니다.

### 예정 명령

```text
make setup-m3
make test-go
make test-contract-m3
make benchmark-m3-gas
make benchmark-m3-e2e
make benchmark-m3
```

## 9. Correctness·측정·완료 Gate

### Native·Circuit Gate

- Voucher native commitment·nullifier와 Circuit 결과가 일치합니다.
- Transfer public variable 수는 6개, Proceed는 3개, Recall은 4개입니다.
- 소비 `cm`과 `rv`는 public witness에 포함되지 않습니다.
- 부분 Transfer의 `a_rec`, `e` residual이 Voucher에 들어가고 합이 보존됩니다.
- 전량 Transfer가 zero-State Change Note를 생성합니다.
- ELIGIBLE·WASTE Transfer가 모두 성공하고 Role 변경은 실패합니다.
- Proceed와 Recall fixture가 같은 Voucher에서 동일한 `rvnf`를 만듭니다.
- 잘못된 $o_{rv}$는 Voucher commitment 또는 `rvnf` 관계를 통과하지 못합니다.
- 잘못된 sender·receiver secret, membership path, State와 output opening이 실패합니다.
- `deltaEpoch=0` Transfer는 성공하지만 Recall은 실패합니다.
- deadline 직전 Recall은 성공하고 deadline과 같거나 이후면 실패합니다.
- Proceed는 deadline 이후에도 성공합니다.

### Foundry·Atomicity Gate

- 부분 Transfer, Proceed, 전량 Transfer와 Recall이 성공합니다.
- 같은 input Note의 재Transfer와 Exit가 nullifier로 실패합니다.
- Proceed 후 Recall, Recall 후 Proceed가 같은 `rvnf`로 실패합니다.
- duplicate Voucher·Change·resolution output이 실패합니다.
- invalid root·proof·epoch가 실패합니다.
- 실패 transaction 후 양 Tree root·leaf count와 모든 mapping이 불변입니다.
- ABI와 verifier public input에 consumed `cm`·`rv`가 없습니다.
- `deadlineRV` mapping과 resolution의 raw `rv` precheck가 없습니다.

### 측정

| 결과 | 포함 범위 | 횟수 | 예정 Raw 파일 |
|---|---|---:|---|
| Circuit | constraints, public inputs, compile·setup·prove·verify, proof bytes | Feature별 1회 | `output/m3-circuit.json` |
| Gas | deployment, Voucher Tree, Transfer·Proceed·Recall receipt, calldata | clean chain 1회 | `output/m3-anvil-gas.json` |
| Live E2E | witness, prove, encode·sign, submit-to-receipt, total | 별도 clean chain 1회 | `output/m3-anvil-e2e.json` |

첫 Voucher append와 이후 append의 SSTORE 차이를 구분합니다. Circuit proving과 on-chain Tree update를 별도 비용으로 해석합니다. 단일 실행값을 평균 성능으로 주장하지 않습니다.

### 명시적 비범위

- 운송 탄소와 Transport Policy
- Voucher encrypted handoff·key exchange
- EVM account와 ZK address binding·relayer
- Status·Audit·Claim·Policy Registry
- 과거 root별 Merkle path archive
- configurable epoch·deadline governance
- final universal SRS·Besu

### 완료 조건

1. 이 명세가 사용자 검토를 통과해 `구현 준비 완료`가 됩니다.
2. 구현 후 Native·Circuit·Foundry Gate를 모두 통과합니다.
3. 공식 Setup·gas·E2E를 정한 횟수만 실행합니다.
4. Raw JSON과 Artifact checksum을 검증합니다.
5. `REUSE.md`에 실제 M3 재사용 근거를 기록합니다.
6. `M3-transfer-voucher-result.md`를 `RESULT-TEMPLATE.md`에 맞춰 생성합니다.
7. Result 없이는 M3를 완료로 표시하지 않습니다.
