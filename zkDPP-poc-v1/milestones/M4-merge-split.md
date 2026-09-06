# M4 Merge·Split 구현 명세

- 명세 상태: 구현 기준 동결
- 이전 단계: [M3 Transfer·Voucher Result](M3-transfer-voucher-result.md)
- 미래 Context: [M4 — Merge와 Split](FUTURE-MILESTONE-CONTEXT.md#m4)
- 구현 결과: [M4 Merge·Split Result](M4-merge-split-result.md)

이 문서는 M4 POC에서 Merge·Split을 어떤 관계와 Interface로 구현할지 고정합니다. 논문 Protocol의 ProductProfile compatibility와 이번 POC에서 실제로 검증하는 조건을 분리해 설명합니다.

## 0. 30초 안에 설계 파악하기

```text
Merge:
  private Note 2개 → public Note 1개

Split:
  private Note 1개 → public Note 2개
```

| 질문 | M4 POC 결정 |
|---|---|
| Merge compatibility는 무엇입니까? | 같은 owner와 같은 AssetRole만 검사합니다. ProductProfile은 POC에서 제외합니다. |
| 어떤 Role을 Merge할 수 있습니까? | ELIGIBLE+ELIGIBLE, WASTE+WASTE를 허용합니다. 서로 다른 Role은 거부합니다. |
| Merge State는 어떻게 계산합니까? | 세 State를 항목별로 더하고 uint64 overflow를 거부합니다. |
| Split 질량은 누가 정합니까? | owner가 private `q_out1`을 선택합니다. |
| Split `a_rec`, `e`는 어떻게 나눕니까? | M3와 같이 output 2를 내림 계산하고 output 1이 residual을 받습니다. |
| 질량 0 Split output을 허용합니까? | 허용합니다. 단, input `q_mass>0`이므로 두 output이 동시에 0일 수는 없습니다. |
| DocumentHash 관계를 검사합니까? | POC에서는 검사하지 않고 각 output의 새 DocumentHash를 허용합니다. |
| 소비 Note가 공개됩니까? | input `cm`과 path는 private witness입니다. |

## 1. 논문 Protocol과 POC의 경계

논문에서는 다음 조건을 사용합니다.

```text
Merge 가능:
  input1.ProductProfile == input2.ProductProfile
```

ProductProfile은 서로 다른 LotID를 가진 물품이 같은 제품군·단위·Merge 규칙을 사용한다는 사실을 나타냅니다.

현재 POC Note에는 ProductProfile이 없습니다.

```text
Note
  ├─ DocumentHash
  ├─ State
  ├─ AssetRole
  ├─ address
  └─ opening
```

M4에서 ProductProfile을 추가하면 Note commitment와 M1~M3 Circuit·Artifact가 모두 바뀝니다. 이번 POC의 목적은 private 다중 input membership, State 합·배분, nullifier와 EVM 원자성 비용을 확인하는 것이므로 ProductProfile을 추가하지 않습니다.

따라서 M4 POC는 다음만 compatibility 조건으로 사용합니다.

```text
input1.address   == input2.address
input1.AssetRole == input2.AssetRole
output.address   == input1.address
output.AssetRole == input1.AssetRole
```

DocumentHash·ProductName·LotID·ProductProfile 관계는 Circuit이 검사하지 않습니다. 이 결과를 논문 Protocol의 완전한 compatibility 검증으로 주장하지 않습니다.

## 2. M3에서 무엇이 달라지나요?

| M3에서 이어받는 기능 | M4 사용 |
|---|---|
| private Note commitment·membership | Merge 2개, Split 1개 input 검증 |
| `sk_owner → address` | 같은 owner의 Merge·Split 강제 |
| Note nullifier | input Note별 한 번만 소비 |
| uint64 State와 Role 조건 | Merge 합·Split 비례 배분 |
| floor·residual relation | Split에서 그대로 재사용 |
| output 순차 insert·transaction atomicity | Merge 1개, Split 2개 output append |
| clean-chain Anvil runner | M4 gas·live E2E 측정 |

M4는 Voucher Tree와 Voucher nullifier를 사용하지 않습니다. Note Tree만 읽고 갱신합니다.

## 3. Canonical 시나리오와 고정 데이터

### 참여자

| 참여자 | 역할 |
|---|---|
| Actor 1 | Merge input 두 개와 Split input의 ZK owner |
| EntryIssuer | 초기 Note를 등록하는 EVM account |

EVM transaction sender와 ZK owner는 binding하지 않습니다.

### 시나리오 A: ELIGIBLE Merge

서로 다른 DocumentHash를 가진 두 ELIGIBLE Note를 같은 owner가 Merge합니다.

| Note | `q_mass` | `a_rec` | `e` | AssetRole |
|---|---:|---:|---:|---|
| Input 1 | 3,000,000,000 | 1,000,000,000 | 2,000,000,000 | ELIGIBLE |
| Input 2 | 2,000,000,000 | 500,000,000 | 1,000,000,000 | ELIGIBLE |
| Output | 5,000,000,000 | 1,500,000,000 | 3,000,000,000 | ELIGIBLE |

Output은 새 Lot을 나타내는 새 DocumentHash와 opening을 사용합니다. Circuit은 이 DocumentHash가 두 input과 어떤 관계인지 검사하지 않습니다.

### 시나리오 B: Merge output Split

Merge output을 두 Note로 Split합니다. Owner가 private `q_out1`을 선택하고 `q_out2`는 차이로 계산합니다.

```text
Merge output
  → Split output 1
  → Split output 2
```

두 output은 input과 같은 owner·AssetRole을 유지하지만 새 DocumentHash와 opening을 사용할 수 있습니다.

### 독립 residual fixture

M3와 같은 nonzero residual 값을 사용해 floor 규칙을 확인합니다.

| 객체 | `q_mass` | `a_rec` | `e` |
|---|---:|---:|---:|
| Input | 3,000,000,000 | 1,000,000,000 | 2,000,000,000 |
| Output 1 | 1,000,000,000 | 333,333,334 | 666,666,667 |
| Output 2 | 2,000,000,000 | 666,666,666 | 1,333,333,333 |

Output 2를 내림 계산하고 output 1이 최소 단위 residual을 받습니다.

### 독립 Role fixture

- WASTE+WASTE Merge는 성공합니다.
- WASTE State는 `a_rec=0`, `e=0`을 유지합니다.
- ELIGIBLE+WASTE Merge는 실패합니다.

## 4. 공통 Note·State 관계

```text
State = (q_mass, a_rec, e)
```

| State | 단위·범위 |
|---|---|
| `q_mass` | kg × $10^9$, uint64 |
| `a_rec` | kg × $10^9$, uint64 |
| `e` | kgCO2e × $10^9$, uint64 |

모든 Note는 다음 기존 조건을 만족합니다.

```text
a_rec <= q_mass
AssetRole ∈ {ELIGIBLE, WASTE}
WASTE이면 a_rec=0, e=0
```

Merge·Split은 새로운 Domain을 만들지 않습니다. 기존 `NoteTag`, `OwnerTag`, `NullifierTag`와 Note commitment·nullifier를 재사용합니다.

### 공통 Privacy 경계

| 공개 | 비공개 |
|---|---|
| accepted Note root | input Note·`cm`·path·index |
| input별 `nf` | `sk_owner`와 정확한 input State |
| output `cm` | output DocumentHash·State·opening |

입력과 output commitment가 같은 transaction calldata에 직접 연결되지 않도록 consumed `cm`은 public input에 포함하지 않습니다.

## 5. Merge

### 목적과 전이

```text
Merge:
  Note 1 + Note 2 → Note Output
```

같은 owner와 같은 AssetRole의 private Note 두 개를 소비하고 세 State의 합을 가진 새 Note를 생성합니다.

### 공개·비공개값

Public input 순서는 다음으로 고정합니다.

```text
1. noteRoot
2. nf1
3. nf2
4. cmOut
```

| 공개값·calldata | 비공개 witness |
|---|---|
| `noteRoot`, `nf1`, `nf2`, `cmOut` | input Note 2개, private `cm1`·`cm2`, path 2개 |
| 없음 | 공통 `sk_owner`, output Note와 opening |

두 input은 같은 현재 Note root에서 증명합니다. 과거 root별 path snapshot은 사용하지 않습니다.

### Merge State 관계

각 $x\in\{q_{mass},a_{rec},e\}$에 대해 다음을 검증합니다.

$$
x_{out}=x_1+x_2
$$

세 input·output 값을 모두 uint64로 range check합니다. 두 input의 합이 $2^{64}-1$을 넘으면 uint64 output과 equality를 동시에 만족할 수 없으므로 proof가 실패합니다.

### Native fixture 수도 코드

```text
# 핵심: 같은 owner·Role의 두 Note State를 overflow 없이 더해 새 Note를 만듭니다.
function BuildMerge(input1, input2, ownerSecret, outputDocumentHash, outputOpening):
    # 1. 두 input 소유자와 compatibility adaptation을 확인합니다.
    ownerAddress = H(OwnerTag, ownerSecret)
    assert input1.address == ownerAddress
    assert input2.address == ownerAddress
    assert input1.AssetRole == input2.AssetRole
    assert input1.cm != input2.cm

    # 2. 세 State를 overflow 없이 더합니다.
    qOut = CheckedUint64Add(input1.q_mass, input2.q_mass)
    aOut = CheckedUint64Add(input1.a_rec, input2.a_rec)
    eOut = CheckedUint64Add(input1.e, input2.e)

    # 3. 새 DocumentHash와 같은 owner·Role의 output을 만듭니다.
    output = Note(
        outputDocumentHash,
        input1.AssetRole,
        State(qOut, aOut, eOut),
        ownerAddress,
        outputOpening,
    )
    return output
```

### Merge Circuit 수도 코드

```text
# 핵심: 두 private Note의 소유·membership·nullifier와 State 합을 한 proof에서 검증합니다.
function MergeCircuit(public, private):
    # 1. public statement를 정해진 순서로 읽습니다.
    [noteRoot, nf1, nf2, cmOut] = public

    # 2. 공통 owner secret에서 address를 계산합니다.
    ownerAddress = H(OwnerTag, private.ownerSecret)

    # 3. 각 input의 Note·소유·membership·nullifier를 검증합니다.
    for i in [1, 2]:
        ValidateNote(private.input[i])
        assert private.input[i].address == ownerAddress
        cm[i] = NoteCommitment(private.input[i])
        assert MerkleRoot(cm[i], private.path[i]) == noteRoot
        assert H(NullifierTag, private.ownerSecret, cm[i]) == public.nf[i]

    # 4. 같은 Note를 두 번 사용하지 못하게 합니다.
    assert cm[1] != cm[2]
    assert nf1 != nf2

    # 5. POC compatibility인 같은 AssetRole을 확인합니다.
    assert private.input[1].AssetRole == private.input[2].AssetRole
    assert private.output.AssetRole == private.input[1].AssetRole
    assert private.output.address == ownerAddress

    # 6. State 합과 uint64 범위를 확인합니다.
    ValidateNote(private.output)
    for x in [q_mass, a_rec, e]:
        assert private.output.x == private.input[1].x + private.input[2].x

    # 7. 새 public output commitment를 확인합니다.
    assert NoteCommitment(private.output) == cmOut
```

Input·output DocumentHash 사이에는 equality나 compatibility 조건을 넣지 않습니다.

### Merge Contract 수도 코드와 상태 변화

```text
# 핵심: 두 private Note를 한 번씩 소비하고 output Note 하나를 원자적으로 append합니다.
function merge(proof, noteRoot, nf1, nf2, cmOut):
    # 1. public precondition을 확인합니다.
    require acceptedNoteRoots[noteRoot]
    require nf1 != nf2
    require noteNullifiers[nf1] == false
    require noteNullifiers[nf2] == false
    require commitments[cmOut] == false

    # 2. 정해진 public statement로 proof를 검증합니다.
    verify MergeProof(noteRoot, nf1, nf2, cmOut)

    # 3. proof 이후에만 두 input과 output 상태를 반영합니다.
    noteNullifiers[nf1] = true
    noteNullifiers[nf2] = true
    commitments[cmOut] = true
    appendNoteTree(cmOut)
```

### Merge 실패·Atomicity·측정

다음은 실패해야 합니다.

- 허용되지 않은 root, 이미 사용된 `nf`, 같은 `nf1==nf2`
- 같은 private Note를 두 input slot에 사용
- 서로 다른 owner 또는 잘못된 owner secret
- ELIGIBLE+WASTE처럼 서로 다른 AssetRole
- output Role·owner 변조
- `q_mass`, `a_rec`, `e` 합 오류와 uint64 overflow
- 잘못된 path·opening·output commitment

실패하면 두 nullifier, output commitment, Note Tree root·leaf count가 모두 불변이어야 합니다.

## 6. Split

### 목적과 전이

```text
Split:
  Note Input → Note Output 1 + Note Output 2
```

Owner가 private 질량을 선택하고 input State를 질량 비례로 나눈 두 Note를 원자적으로 생성합니다.

### 공개·비공개값

Public input 순서는 다음으로 고정합니다.

```text
1. noteRoot
2. nf
3. cmOut1
4. cmOut2
```

| 공개값·calldata | 비공개 witness |
|---|---|
| `noteRoot`, `nf`, `cmOut1`, `cmOut2` | input Note·private `cm`·path·owner secret |
| 없음 | `q_out1`, 두 output Note·opening, `a_rec`·`e` remainder |

### Split State 관계

Owner가 다음 범위의 `q_out1`을 선택합니다.

$$
0\le q_{out1}\le q_{input}
$$

$$
q_{out2}=q_{input}-q_{out1}
$$

Input은 `q_mass>0`이어야 합니다. 따라서 두 output이 동시에 zero-State가 될 수 없습니다.

각 $x\in\{a_{rec},e\}$에 대해 output 2를 내림 계산합니다.

$$
x_{out2}
=
\left\lfloor
\frac{x_{input}q_{out2}}{q_{input}}
\right\rfloor
$$

$$
x_{out1}=x_{input}-x_{out2}
$$

Circuit은 private remainder $r_x$로 floor 관계를 검증합니다.

$$
q_{out2}x_{input}=q_{input}x_{out2}+r_x
$$

$$
0\le r_x<q_{input}
$$

### Native fixture 수도 코드

```text
# 핵심: output 2를 내림 계산하고 output 1에 residual을 주어 State를 정확히 보존합니다.
function BuildSplit(input, ownerSecret, qOut1,
                    documentHash1, documentHash2, opening1, opening2):
    # 1. input 소유권과 질량 범위를 확인합니다.
    ownerAddress = H(OwnerTag, ownerSecret)
    assert input.address == ownerAddress
    assert input.q_mass > 0
    assert 0 <= qOut1 <= input.q_mass
    qOut2 = input.q_mass - qOut1

    # 2. output 2의 State 몫과 remainder를 계산합니다.
    for x in [a_rec, e]:
        xOut2, remainder[x] = DivRem(input.x * qOut2, input.q_mass)
        xOut1 = input.x - xOut2

    # 3. 같은 owner·Role의 두 새 Note를 만듭니다.
    output1 = Note(documentHash1, input.AssetRole,
                   State(qOut1, aOut1, eOut1), ownerAddress, opening1)
    output2 = Note(documentHash2, input.AssetRole,
                   State(qOut2, aOut2, eOut2), ownerAddress, opening2)
    return output1, output2, remainder
```

### Split Circuit 수도 코드

```text
# 핵심: private Note를 한 번 소비하고 State 보존·floor가 적용된 output 두 개를 만듭니다.
function SplitCircuit(public, private):
    # 1. public statement를 읽고 input Note를 검증합니다.
    [noteRoot, nf, cmOut1, cmOut2] = public
    ValidateNote(private.input)
    assert private.input.q_mass > 0

    # 2. owner·membership·nullifier를 검증합니다.
    ownerAddress = H(OwnerTag, private.ownerSecret)
    assert private.input.address == ownerAddress
    inputCM = NoteCommitment(private.input)
    assert MerkleRoot(inputCM, private.path) == noteRoot
    assert H(NullifierTag, private.ownerSecret, inputCM) == nf

    # 3. 두 output의 owner·Role·질량 합을 확인합니다.
    ValidateNote(private.output1)
    ValidateNote(private.output2)
    assert private.output1.address == ownerAddress
    assert private.output2.address == ownerAddress
    assert private.output1.AssetRole == private.input.AssetRole
    assert private.output2.AssetRole == private.input.AssetRole
    assert private.input.q_mass == private.output1.q_mass + private.output2.q_mass

    # 4. output 2 floor·remainder와 State 합을 확인합니다.
    for x in [a_rec, e]:
        assert private.output2.q_mass * private.input.x
               == private.input.q_mass * private.output2.x + private.remainder[x]
        assert 0 <= private.remainder[x] < private.input.q_mass
        assert private.input.x == private.output1.x + private.output2.x

    # 5. 두 public output commitment를 확인합니다.
    assert NoteCommitment(private.output1) == cmOut1
    assert NoteCommitment(private.output2) == cmOut2
```

Input·output DocumentHash 사이에는 equality를 넣지 않습니다. 두 output이 같은 DocumentHash를 사용하는 것도 허용하지만 fresh opening으로 서로 다른 commitment를 만들어야 합니다.

### Split Contract 수도 코드와 상태 변화

```text
# 핵심: private Note를 한 번 소비하고 output Note 두 개를 순서대로 원자적으로 append합니다.
function split(proof, noteRoot, nf, cmOut1, cmOut2):
    # 1. public precondition을 확인합니다.
    require acceptedNoteRoots[noteRoot]
    require noteNullifiers[nf] == false
    require commitments[cmOut1] == false
    require commitments[cmOut2] == false
    require cmOut1 != cmOut2

    # 2. proof를 검증합니다.
    verify SplitProof(noteRoot, nf, cmOut1, cmOut2)

    # 3. input과 두 output을 순서대로 기록합니다.
    noteNullifiers[nf] = true
    commitments[cmOut1] = true
    appendNoteTree(cmOut1)
    require commitments[cmOut2] == false
    commitments[cmOut2] = true
    appendNoteTree(cmOut2)
```

두 번째 output insert나 append가 실패하면 EVM atomicity로 첫 output과 nullifier 변경도 모두 revert되어야 합니다.

### Split 실패·Atomicity·측정

다음은 실패해야 합니다.

- 허용되지 않은 root, 이미 사용된 `nf`
- input `q_mass=0`, output 질량 합 오류
- 서로 다른 output owner·Role 또는 input Role 변경
- 잘못된 비례 배분·remainder·State 합
- 중복 output commitment·기존 commitment 재사용
- 잘못된 input path·owner secret·output opening

질량 0 output 자체는 실패 조건이 아닙니다. zero-State output도 fresh commitment로 Tree에 append합니다.

## 7. Contract Interface와 상태

기존 `EntryExitLedger`를 계속 확장하며 M9 전까지 이름을 바꾸지 않습니다.

### Constructor

```text
constructor(
  entryVerifier,
  privateSpendVerifier,
  transferVerifier,
  proceedVerifier,
  recallVerifier,
  mergeVerifier,
  splitVerifier,
  fieldHasher
)
```

### 새 Interface

```text
merge(proof, noteRoot, nf1, nf2, cmOut)
split(proof, noteRoot, nf, cmOut1, cmOut2)
```

새 mapping이나 Tree는 만들지 않습니다. 기존 Note Tree, `commitments`, `noteNullifiers`를 재사용합니다.

### 상태 변화

| Event | input 상태 | output 상태 | Note Tree 증가 |
|---|---|---|---:|
| Merge | nullifier 2개 기록 | commitment 1개 등록 | 1 |
| Split | nullifier 1개 기록 | commitment 2개 등록 | 2 |

## 8. 구현 구조·Artifact·명령

### 예정 Feature

```text
features/merge/
  README.md
  circuit.go
  circuit_test.go

features/split/
  README.md
  circuit.go
  circuit_test.go

internal/m4case/
  connected Merge·Split fixture

cmd/setup_m4/
cmd/benchmark_m4/
```

### 재사용

- Note·State·Role·owner·commitment·nullifier
- Depth-32 private membership
- M3 allocation·remainder native 계산과 Circuit relation
- development Artifact·Solidity verifier export
- Note Tree와 output 순차 insert
- Foundry·Anvil runner

M3의 Voucher model·Voucher Tree·deadline은 M4 Circuit에서 사용하지 않지만 기존 Contract 기능은 회귀 없이 유지합니다.

### 개발 Artifact

```text
artifacts/development/m4/
  merge/
  split/
```

Merge·Split은 별도 CCS·PK·VK·Solidity verifier를 가집니다. M1~M3 Artifact와 Raw 결과는 다시 측정하거나 덮어쓰지 않습니다.

### 예정 명령

```text
make setup-m4
make test-go
make test-contract-m4
make benchmark-m4-gas
make benchmark-m4-e2e
make benchmark-m4
```

## 9. Correctness·측정·완료 Gate

### Native·Circuit Gate

- Merge·Split native 결과와 Circuit output commitment가 일치합니다.
- Merge public variable 수는 4개, Split은 4개입니다.
- consumed `cm`과 path가 public witness에 없습니다.
- ELIGIBLE+ELIGIBLE과 WASTE+WASTE Merge가 성공합니다.
- ELIGIBLE+WASTE, 다른 owner, 같은 input 두 번 사용은 실패합니다.
- Merge State 합과 uint64 overflow가 검증됩니다.
- Split partial·nonzero residual·zero output이 성공합니다.
- Split의 잘못된 floor·remainder·State 합·Role 변경은 실패합니다.
- arbitrary output DocumentHash가 허용되고 ProductProfile 검사가 없음을 test 이름과 Result에 명시합니다.

### Foundry·Atomicity Gate

- 기존 M2·M3 test가 회귀하지 않습니다.
- 연결된 Entry→Merge→Split 시나리오가 성공합니다.
- Go·Solidity Note root·path와 최종 leaf count가 일치합니다.
- invalid root·proof·spent nullifier·duplicate input·output을 거부합니다.
- Merge와 Split 실패 후 mapping·root·leaf count가 모두 불변입니다.
- ABI와 verifier public inputs에 input `cm`이 없습니다.

### Canonical 최종 상태

연결 시나리오는 다음 순서를 사용합니다.

```text
Entry Note 1
Entry Note 2
Merge → Note 3
Split Note 3 → Note 4 + Note 5
```

최종 Note Tree leaf count는 5입니다. Merge input nullifier 2개와 Split input nullifier 1개가 기록됩니다.

### 측정

| 결과 | 포함 | 횟수 | Raw 파일 |
|---|---|---:|---|
| Circuit | constraints, public inputs, compile·SRS·Setup·Prove·Verify, proof bytes | Feature별 1회 | `output/m4-circuit.json` |
| Gas | verifier·Ledger deployment, Merge·Split receipt·calldata | clean chain 1회 | `output/m4-anvil-gas.json` |
| Live E2E | witness·prove·tx prepare·submit-to-receipt·total | 별도 clean chain 1회 | `output/m4-anvil-e2e.json` |

Gas와 E2E는 각각 한 번만 측정합니다. Circuit membership proving 비용과 Contract Note Tree append·SSTORE 비용을 분리해 해석합니다.

### 명시적 비범위

- ProductProfile·ProductTypeHash·MergeProfile
- DocumentHash·ProductName·LotID compatibility
- 서로 다른 owner의 공동 Merge
- Merge·Split 중 소유권 이전
- 운송·Process 탄소 delta
- Policy·Status·Audit·Claim
- final universal SRS·Besu

### 완료 조건

1. 이 명세가 사용자 검토를 통과해 `구현 준비 완료`가 됩니다.
2. Native·Circuit·Foundry Gate를 통과합니다.
3. Setup·gas·E2E를 정한 횟수만 실행합니다.
4. M1~M3 Raw 결과 checksum이 변하지 않습니다.
5. `REUSE.md`에 Merge·Split에서 실제 재사용한 관계를 기록합니다.
6. `M4-merge-split-result.md`를 `RESULT-TEMPLATE.md`에 맞춰 생성합니다.
7. Result 없이는 M4를 완료로 표시하지 않습니다.
