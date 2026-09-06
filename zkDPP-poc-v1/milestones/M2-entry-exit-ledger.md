# M2 Entry·Exit Ledger 구현 명세

- 명세 상태: 구현 기준 동결
- 선행 Result: [`M1-private-note-core-result.md`](M1-private-note-core-result.md)
- Result: [`M2-entry-exit-ledger-result.md`](M2-entry-exit-ledger-result.md)
- Checklist: [`MILESTONE-CHECKLIST.md`](MILESTONE-CHECKLIST.md)

이 문서는 M2의 Entry·Exit Circuit, Solidity Ledger, Note Tree, 권한, fixture, Foundry·Anvil test와 측정 구현에 필요한 내용을 자체적으로 설명합니다.

## 0. 30초 안에 설계 파악하기

| 질문 | M2 결정 |
|---|---|
| 이전 상태 | M1은 `cm`, Private Spend·`nf` Circuit만 있고 Contract 상태 변경은 없었습니다. |
| 목표 | Note를 EVM Ledger에 Entry하고 private Note를 Exit합니다. |
| Entry | 신규 Circuit, EntryIssuer 권한, `commitments[cm]`, Tree append |
| Exit | M1 Private Spend 재사용, `noteNullifiers[nf]`, output 없음 |
| 공개값 | Entry는 `cm`; Exit는 `noteRoot`, `nf` |
| Tree | Depth 32, 현재 node 직접 저장, 현재 path 조회 |
| 구현 | Go·gnark·Solidity·Foundry·clean Anvil gas와 live E2E |
| 비범위 | Voucher·Policy·Status·Audit·Claim·Besu·universal SRS |

## 1. M1에서 무엇이 달라지나요?

| M1 | M2 추가 | 완료 후 가능 |
|---|---|---|
| private Note → `cm` | Entry 조건·권한·Tree append | 초기 원자재 등록 |
| Private Spend → root·`nf` | accepted root·nullifier mapping | output 없는 private Exit |
| native Tree | Solidity node storage·path query | 실제 EVM membership 기반 |
| Go PLONK | Solidity verifier·Anvil runner | gas·transaction E2E |

$$
\varnothing\rightarrow\mathrm{Note}\rightarrow\varnothing
$$

## 2. Canonical 시나리오와 고정 데이터

### EVM 참여자

| Anvil account | 역할 |
|---:|---|
| 0 | Contract admin·배포자 |
| 1 | EntryIssuer A |
| 2 | EntryIssuer B |
| 3 | EntryIssuer C |
| 4 | 권한 없는 Entry negative test |

별도 Exit 역할·Relayer·allowlist는 없습니다. Actor-1 Note를 가진 참여자가 직접 Exit transaction을 제출합니다. EVM account와 ZK address는 암호학적으로 binding하지 않습니다.

### ZK Note fixture

| 원자재 | ZK owner | `q_mass` | `a_rec` | `e` | opening |
|---|---|---:|---:|---:|---:|
| Raw Material A | actor-1 | 100 kg | 20 kg | 80 kgCO2e | 2001 |
| Raw Material B | actor-2 | 50 kg | 0 kg | 40 kgCO2e | 2002 |
| Raw Material C | actor-3 | 30 kg | 10 kg | 25 kgCO2e | 2003 |

State는 $10^9$ scale의 uint64로 저장합니다. ProductName·LotID와 actor secret·address는 고정 fixture에서 재현합니다.

### 실행 흐름

```text
1. Poseidon2·Entry verifier·Private Spend verifier·Ledger를 배포합니다.
2. Admin이 EntryIssuer A·B·C를 승인합니다.
3. A·B·C가 원자재 Note를 하나씩 Entry합니다.
4. Contract 현재 root·Raw Material A path를 조회합니다.
5. actor-1 Note·sk_owner·path로 Private Spend proof를 만듭니다.
6. actor-1 EVM 참여자가 Raw Material A를 Exit합니다.
7. commitments·root·leafCount·noteNullifiers를 검증합니다.
```

## 3. 공통 Note·State·소유자 관계

```text
Note
  ├─ DocumentHash
  ├─ State = (q_mass, a_rec, e)
  ├─ AssetRole
  ├─ address
  └─ opening
```

| 값 | 조건 |
|---|---|
| `q_mass`, `a_rec`, `e` | uint64, $10^9$ scale |
| `a_rec` | `0 <= a_rec <= q_mass` |
| ELIGIBLE | 후속 Event 사용 가능 |
| WASTE | `a_rec=0`, `e=0`, Exit만 가능 |

Entry의 `e`는 Entry 이전 채굴·정제·운송을 포함할 수 있습니다. `e=0`을 강제하지 않으며 초기 State의 현실 진실성은 EntryIssuer 신뢰 가정입니다.

$$
\mathrm{address}=H(\mathrm{OwnerTag},\mathrm{sk}_{\mathrm{owner}})
$$

$$
cm=H(\mathrm{NoteTag},\mathrm{DocumentHash},\mathrm{AssetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{address},\mathrm{opening})
$$

$$
nf=H(\mathrm{NullifierTag},\mathrm{sk}_{\mathrm{owner}},cm)
$$

## 4. Entry

### 목적과 전이

$$
\varnothing\rightarrow\mathrm{Note}
$$

승인된 EntryIssuer가 외부 확인을 거친 ELIGIBLE 초기 Note를 증명하고 public `cm`을 Ledger Tree에 등록합니다.

### 공개·비공개값

| 공개값·calldata | 비공개 witness |
|---|---|
| Entry proof, `cm` | DocumentHash, AssetRole, `q_mass`, `a_rec`, `e`, address, opening |

### Native Note 생성 수도 코드

```text
BuildEntryNote(DocumentInfo, State, ownerAddress, opening):

    # 핵심: 외부에서 확인한 초기 원자재 정보를 ELIGIBLE Note 하나로 만듭니다.

    # 1. 사람이 읽는 제품 설명을 고정된 field 값으로 바꿉니다.
    DocumentHash = HashDocumentInfo(DocumentInfo)

    # 2. Entry에 필요한 최소 State 조건을 확인합니다.
    Require(State.q_mass > 0)
    Require(State.a_rec <= State.q_mass)

    # 3. 공급망에서 사용할 수 있는 초기 Note를 구성합니다.
    Note = {
        DocumentHash,
        AssetRole = ELIGIBLE,
        State,
        address = ownerAddress,
        opening
    }

    # 4. Note 전체를 public identifier인 cm으로 묶습니다.
    cm = NoteCommitment(Note)
    return Note, cm
```

### Entry Circuit 수도 코드

```text
EntryCircuit(public cm, private Note):

    # 핵심: 숨겨진 초기 Note가 Entry 규칙과 public cm을 만족함을 증명합니다.

    # 1. State 세 값이 64-bit 범위인지 확인합니다.
    ToBinary(Note.q_mass, 64)
    ToBinary(Note.a_rec, 64)
    ToBinary(Note.e, 64)

    # 2. Entry 전용 State·Role 조건을 확인합니다.
    Assert(Note.q_mass != 0)
    Assert(Note.a_rec <= Note.q_mass)
    Assert(Note.AssetRole == ELIGIBLE)

    # 3. 숨겨진 Note로 commitment를 다시 계산합니다.
    expectedCM = Poseidon2Hash(
        NoteTag,
        Note.DocumentHash,
        Note.AssetRole,
        Note.q_mass,
        Note.a_rec,
        Note.e,
        Note.address,
        Note.opening
    )

    # 4. 재계산한 commitment를 public cm에 binding합니다.
    Assert(expectedCM == cm)
```

M1의 State validation·Commitment gadget을 재사용하고 `q_mass>0`, ELIGIBLE을 추가합니다.

### Prover 수도 코드

```text
ProveEntry(Note, cm):
    # 핵심: cm만 공개하고 Note 전체는 witness로 숨깁니다.

    # 1. 공개 statement와 private witness를 분리합니다.
    witness.public  = [cm]
    witness.private = [Note]

    # 2. Entry 전용 PK로 proof를 생성합니다.
    proof = PLONK.Prove(EntryCCS, EntryPK, witness)

    # 3. Contract가 읽을 Solidity 형식으로 반환합니다.
    return SolidityMarshal(proof), [cm]
```

### EntryIssuer 권한

| 상태 | 역할 |
|---|---|
| immutable `admin` | EntryIssuer 승인·해제 |
| `entryIssuers[account]` | 해당 EVM account의 현재 Entry 권한 |

```text
setEntryIssuer(account, allowed):
    # 핵심: Admin만 초기 원자재 등록 권한을 부여하거나 해제합니다.

    # 1. 권한 변경 호출자와 대상 account를 확인합니다.
    Require(msg.sender == admin)
    Require(account != zero address)

    # 2. 권한 상태를 갱신하고 공개 Event를 남깁니다.
    entryIssuers[account] = allowed
    Emit EntryIssuerUpdated(account, allowed)
```

권한 해제는 이후 Entry만 막고 과거 Note에는 영향을 주지 않습니다. Admin 교체·다중서명·Production governance는 M2 범위가 아닙니다.

### Entry Contract 수도 코드와 상태 변화

```text
entry(proof, cm):

    # 핵심: 유효한 신규 cm만 한 번 등록하고 Note Tree를 갱신합니다.

    # 1. 호출 권한과 동일 commitment 중복을 먼저 차단합니다.
    Require(entryIssuers[msg.sender] == true)
    Require(commitments[cm] == false)

    # 2. Circuit proof가 public cm을 승인했는지 확인합니다.
    Verify(EntryVerifier, proof, publicInputs=[cm])

    # 3. cm의 등록 사실을 기록하고 Tree에 append합니다.
    commitments[cm] = true
    index, newRoot = AppendNoteTree(cm)

    # 4. 새 index와 root를 Event로 공개합니다.
    Emit NoteAppended(cm, index, newRoot)
```

| 변경 상태 | 의미 |
|---|---|
| `commitments[cm]=true` | 같은 암호학적 output 중복 방지 |
| `noteTreeNodes` | leaf·ancestor node 갱신 |
| `currentNoteRoot` | 최신 root |
| `acceptedRoots[newRoot]=true` | 과거·현재 root 수용 |
| `noteLeafCount++` | 다음 leaf 위치 |

`commitments`는 ever-registered 표시이며 unspent 상태가 아닙니다. 다른 opening을 사용한 현실 물품 중복 발행은 EntryIssuer 책임입니다.

### Entry 실패·Atomicity·측정

실패:

- 비관리자의 issuer 변경, zero account 승인
- 권한 없는·해제된 issuer
- 중복 `cm`
- `q_mass=0`, `a_rec>q_mass`, WASTE, uint64 초과, 변조 proof

Proof 또는 Tree Update가 실패하면 commitment·node·root·count가 전부 revert해야 합니다.

측정:

- Circuit constraints·public inputs·Compile·Setup·Prove·Verify
- binary·Solidity proof bytes
- 첫 append와 이후 append receipt gas·calldata
- witness·prove·tx prepare·submit-to-receipt E2E

## 5. Exit

### 목적과 전이

$$
\mathrm{Note}\rightarrow\varnothing
$$

M1 Private Spend Circuit·CCS·PK·VK를 재사용해 소비 `cm`을 공개하지 않고 Note를 terminal 소비합니다. ELIGIBLE과 WASTE 모두 Exit할 수 있습니다.

### 공개·비공개값

| 공개값·calldata | 비공개 witness |
|---|---|
| proof, `noteRoot`, `nf` | Note, `sk_owner`, 소비 `cm`, leaf index, sibling 32개 |

### Native Exit fixture 수도 코드

```text
BuildExitWitness(Note, sk_owner, currentTree):

    # 핵심: 소비할 Note를 공개하지 않고 Spend proof에 필요한 witness를 준비합니다.

    # 1. Note에서 소비 commitment를 다시 계산합니다.
    cm = NoteCommitment(Note)

    # 2. 현재 Tree 기준 membership path와 root를 가져옵니다.
    path = currentTree.Path(indexOf(cm))
    noteRoot = currentTree.Root()
    # 3. 이 Note에만 대응하는 nullifier를 만듭니다.
    nf = H(NullifierTag, sk_owner, cm)

    return Note, sk_owner, path, noteRoot, nf
```

### Exit Circuit 전체 수도 코드

```text
PrivateSpendCircuit(
    public noteRoot,
    public nf,
    private Note,
    private sk_owner,
    private leafIndex,
    private siblings[32]
):

    # 핵심: 소비 cm을 숨긴 채 Note 유효성·소유권·membership·nf를 증명합니다.

    # 1. Note의 숫자 범위를 확인합니다.
    ToBinary(Note.q_mass, 64)
    ToBinary(Note.a_rec, 64)
    ToBinary(Note.e, 64)

    # 2. 공통 State·Role 조건을 확인합니다.
    Assert(Note.a_rec <= Note.q_mass)
    Assert(Note.AssetRole == ELIGIBLE or WASTE)

    # 3. WASTE에는 credit과 탄소를 귀속하지 않습니다.
    if Note.AssetRole == WASTE:
        Assert(Note.a_rec == 0)
        Assert(Note.e == 0)

    # 4. sk_owner가 Note의 실제 소유자 secret인지 확인합니다.
    expectedAddress = Poseidon2Hash(OwnerTag, sk_owner)
    Assert(expectedAddress == Note.address)

    # 5. 숨겨진 Note에서 소비 commitment를 계산합니다.
    cm = Poseidon2Hash(
        NoteTag,
        Note.DocumentHash,
        Note.AssetRole,
        Note.q_mass,
        Note.a_rec,
        Note.e,
        Note.address,
        Note.opening
    )

    # 6. private path로 public noteRoot를 재계산합니다.
    bits = ToBinary(leafIndex, 32)
    calculatedRoot = cm

    for level = 0..31:
        sibling = siblings[level]

        if bits[level] == 0:
            calculatedRoot = Compress(calculatedRoot, sibling)
        else:
            calculatedRoot = Compress(sibling, calculatedRoot)

    # 7. 계산된 root를 공개 statement에 binding합니다.
    Assert(calculatedRoot == noteRoot)

    # 8. secret과 private cm으로 public nullifier를 확인합니다.
    expectedNF = Poseidon2Hash(NullifierTag, sk_owner, cm)
    Assert(expectedNF == nf)
```

### Prover 수도 코드

```text
ProveExit(Note, sk_owner, path, noteRoot, nf):
    # 핵심: noteRoot와 nf만 공개하는 Private Spend proof를 만듭니다.

    # 1. 공개 statement와 private witness를 분리합니다.
    witness.public  = [noteRoot, nf]
    witness.private = [Note, sk_owner, path.index, path.siblings]

    # 2. M1에서 만든 Private Spend PK를 재사용합니다.
    proof = PLONK.Prove(PrivateSpendCCS, PrivateSpendPK, witness)

    # 3. Contract calldata용 proof를 반환합니다.
    return SolidityMarshal(proof), [noteRoot, nf]
```

### Exit Contract 수도 코드와 상태 변화

```text
exit(proof, noteRoot, nf):

    # 핵심: 유효한 private Note를 output 없이 한 번만 소비합니다.

    # 1. root와 nullifier의 현재 Ledger 상태를 확인합니다.
    Require(acceptedRoots[noteRoot] == true)
    Require(noteNullifiers[nf] == false)

    # 2. M1 Private Spend proof를 그대로 검증합니다.
    Verify(PrivateSpendVerifier, proof, [noteRoot, nf])

    # 3. nullifier만 spent로 기록하고 Tree는 변경하지 않습니다.
    noteNullifiers[nf] = true
    Emit NoteExited(nf)
```

| 변경 상태 | Exit 전 | Exit 후 |
|---|---|---|
| `noteNullifiers[nf]` | false | true |
| `commitments` | 변경 없음 | 변경 없음 |
| `currentNoteRoot` | root | 동일 root |
| `noteLeafCount` | count | 동일 count |

Exit에는 output commitment·Tree append·EVM caller allowlist가 없습니다. 시나리오의 Note 소유자가 직접 호출하지만 실제 소유권은 ZK proof가 검증합니다.

### Exit 실패·Atomicity·측정

실패:

- accepted 상태가 아닌 root
- 이미 사용된 `nf`
- 잘못된 Note·`sk_owner`·index·sibling·root·nf·proof
- Generated verifier의 false 또는 내부 revert

Verifier false·revert는 Contract `InvalidProof`로 정규화합니다. 실패하면 nullifier·Tree 상태가 변하지 않아야 합니다.

측정:

- 재사용 Private Spend constraints·Prove·Verify·Solidity proof bytes
- Exit receipt gas·calldata
- witness·prove·tx prepare·submit-to-receipt E2E

## 6. Note Tree와 Contract 전체 상태

### Tree append 수도 코드

```text
AppendNoteTree(leaf):

    # 핵심: 새 cm 경로의 node만 갱신해 최신 root를 만듭니다.

    # 1. Tree 용량을 확인하고 새 leaf 위치를 정합니다.
    Require(noteLeafCount < 2^32)

    # 2. level 0에 새 leaf를 저장합니다.
    index = noteLeafCount
    position = index
    current = leaf
    Store(level=0, position, current)

    # 3. leaf에서 root까지 parent를 순서대로 계산합니다.
    for level = 0..31:
        if position is even:
            left = current
            right = NodeOrZero(level, position+1)
        else:
            left = NodeOrZero(level, position-1)
            right = current

        current = Poseidon2Compress(left, right)
        position = position >> 1
        Store(level+1, position, current)

    # 4. 새 Tree 상태와 accepted root를 원자적으로 기록합니다.
    noteLeafCount = index + 1
    currentNoteRoot = current
    acceptedRoots[current] = true
```

빈 node는 `zeroes[level]`을 사용하고 실제 leaf·parent만 `noteTreeNodes`에 저장합니다. Constructor는 zero Hash 32단계와 초기 accepted root를 계산합니다.

`getNotePath(index)`는 현재 root와 sibling 32개를 반환합니다. 과거 root bool은 보존하지만 과거 path snapshot은 저장하지 않습니다.

### 전체 상태와 Interface

| 상태 | 역할 | 변경 함수 |
|---|---|---|
| `admin` | issuer 관리 | constructor |
| `entryIssuers` | Entry 권한 | `setEntryIssuer` |
| `commitments` | 등록된 output `cm` | `entry` |
| `noteNullifiers` | 소비된 private Note | `exit` |
| `noteTreeNodes` | 현재 leaf·parent | `entry` |
| `zeroes` | 빈 subtree Hash | constructor |
| `acceptedRoots` | 유효 root | constructor·`entry` |
| `currentNoteRoot` | 최신 root | constructor·`entry` |
| `noteLeafCount` | leaf 개수 | `entry` |

```text
constructor(entryVerifier, privateSpendVerifier, fieldHasher)

setEntryIssuer(account, allowed)
entry(proof, cm)
exit(proof, noteRoot, nf)

getNotePath(index)
currentNoteRoot()
acceptedNoteRoot(root)
noteLeafCount()
commitments(cm)
noteNullifiers(nf)
```

Event:

```text
EntryIssuerUpdated(account, allowed)
NoteAppended(cm, index, root)
NoteExited(nf)
```

## 7. 구현 구조·Artifact·명령

### 코드 구조

```text
features/entry/
  README.md
  circuit.go
  circuit_test.go

features/private_spend/
  M1 Circuit 재사용

contracts/src/
  EntryExitLedger.sol
  EntryVerifier.sol
  PrivateSpendVerifier.sol
  IFieldHasher.sol
  IZkVerifier.sol

contracts/test/
  EntryExitLedger.t.sol

cmd/setup_m2/
cmd/benchmark_m2/
internal/m2case/
internal/solgen/
```

### Artifact 생성

```text
Entry Circuit compile
  → artifacts/development/m2/entry/{CCS,SRS,PK,VK,manifest}

M1 Private Spend manifest·checksum 검증
  → 기존 CCS·PK·VK 로드

Entry VK·Private Spend VK
  → Solidity verifier export

Poseidon2 parameters
  → Solidity compression contract 생성

Canonical assignments
  → fixed Solidity proof fixture 생성
```

Binary proof와 Solidity proof bytes를 구분합니다. Generated verifier·Poseidon·fixture checksum과 M1 output checksum을 기록합니다.

### 실행 환경·명령

```text
Go 1.25.7
gnark 0.15.0, gnark-crypto 0.20.1
Solidity 0.8.30, Prague EVM
Foundry Docker 1.7.1
Anvil chain ID 31337, block gas limit 30M
```

```text
make test-go
make setup-m2
make test-contract-m2
make benchmark-m2-gas
make benchmark-m2-e2e
```

결과:

```text
output/m2-circuit.json
output/m2-anvil-gas.json
output/m2-anvil-e2e.json
output/m2-generated-checksums.json
milestones/M2-entry-exit-ledger-result.md
```

Gas와 E2E는 서로 다른 clean Anvil chain에서 실행합니다. M2 기본 RPC port는 local 충돌을 피하기 위해 18545이며 chain ID는 31337입니다.

## 8. Correctness·측정·완료 Gate

### Go Circuit

- 초기 `e=0`, `e>0` Entry 성공
- `q_mass=0`, WASTE Entry, `a_rec>q_mass`, uint64 초과·변조 `cm` 실패
- Entry public input 1개
- Private Spend public input 2개·M1 Artifact 재사용

### Foundry·Atomicity

- Admin 승인·해제·비관리자 거부
- EntryIssuer 3개의 Entry
- 권한 없는·해제된 issuer와 중복 commitment 거부
- Go·Solidity root·path 32개 일치
- 올바른 Exit, invalid root·proof·중복 nullifier 거부
- Entry 실패 후 commitment·root·count 불변
- Exit 실패 후 nullifier·Tree 불변
- Exit 성공 후 root·count 불변

### Anvil 측정

| 계층 | 측정 | 횟수 |
|---|---|---:|
| Circuit | Compile·Setup·Prove·Verify·proof·artifact bytes | Feature별 1회 |
| Deployment | Poseidon2·verifier 2개·Ledger gas | clean gas run 1회 |
| Entry | 첫 append·이후 append gas·calldata | case별 1회 |
| Exit | gas·calldata | 1회 |
| Live E2E | witness·prove·tx prepare·submit-to-receipt | 별도 clean run, case별 1회 |

`tx prepare`에는 ABI encoding, nonce·gas price 조회, `eth_estimateGas`와 서명이 포함됩니다.

### 비범위

- Voucher·Transfer·Proceed·Recall
- Merge·Split·Process
- PolicyRef·Status·AuditRecord·Claim
- 과거 root별 path archive
- Besu·permissioning·final universal SRS
- Admin 교체·다중서명·Production governance

### 완료 조건

1. Go·Foundry positive·negative·atomicity Gate 통과
2. Go·Solidity Tree differential test 통과
3. Entry 3건·Exit 1건 Anvil 성공
4. Gas·live E2E case별 1회 기록
5. Generated·M1 checksum 확인
6. [`RESULT-TEMPLATE.md`](RESULT-TEMPLATE.md)에 따른 Result 생성
