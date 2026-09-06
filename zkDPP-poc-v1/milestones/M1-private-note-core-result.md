# M1 Private Note Core Result

- 상태: 완료
- 구현 명세: [`M1-private-note-core.md`](M1-private-note-core.md)
- Raw 결과: [`../output/m1-core.json`](../output/m1-core.json)
- 실행 횟수: Setup 1회, Feature별 Prove·Verify 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M1 결과 |
|---|---|
| 이전 상태 | zkDPP-poc-v1 코드와 공통 Note 암호 관계가 없었습니다. |
| 이번 목표 | 모든 공급망 Event가 공유할 private Note 생성과 소비 검증 기반을 만듭니다. |
| 실제 구현 | Note·State·owner, Note Commitment, Private Spend Kernel, Depth-32 Merkle, 고정 actor, PLONK 실행 환경 |
| 이제 가능한 것 | Note 내용을 숨겨 `cm`을 만들고, 특정 `cm`을 밝히지 않은 채 소유권·Merkle membership·`nf` 관계를 증명할 수 있습니다. |
| 검증 | 일반 Go·Circuit 결과 일치, positive·negative test와 실제 PLONK Prove·Verify 성공 |
| 대표 성능 | Note Commitment 5,678 constraints·136 ms; Private Spend Kernel 16,407 constraints·407 ms |
| 미포함 | Entry·Exit Event, Contract Tree Update, nullifier 중복 상태, Policy·Status·Audit·DPP |
| 다음 단계 | M2에서 `cm`을 Contract Tree에 Entry하고 `nf`를 온체인에 기록합니다. |

```text
Note Commitment:
  private Note → public cm

Private Spend Kernel:
  private Note + sk_owner + private path
  → public noteRoot + nf
```

**Private Spend Kernel은 Note를 실제로 소비하는 함수가 아닙니다.** Note의 소유권·Merkle membership·nullifier 관계를 검증하는 재사용 가능한 Circuit입니다. 실제 소비는 후속 Event Contract가 이 proof를 검증하고 `noteNullifiers[nf]=true`를 저장할 때 발생합니다.

## 1. 이전 상태에서 무엇이 달라졌나요?

| 이전 | M1에서 추가 | 현재 가능 |
|---|---|---|
| Note schema 없음 | DocumentHash·State·Role·address·opening | 공통 private Note 표현 |
| 소유 관계 없음 | `address=H(OwnerTag,sk_owner)` | secret 기반 소유권 검증 |
| 소비 검증 기반 없음 | Commitment·membership·nullifier 관계 | 후속 Event가 재사용할 private 소비 proof |
| Artifact 기준 없음 | 개발용 CCS·SRS·PK·VK·manifest | 재현 가능한 PLONK 실험 |

## 2. 실제 구현한 동작

### Note

| 구성 | 의미 |
|---|---|
| `DocumentHash` | ProductName·LotID binding |
| `q_mass` | kg × $10^9$ 전체 질량 |
| `a_rec` | kg × $10^9$ 재활용 credit |
| `e` | kgCO2e × $10^9$ 누적 탄소 |
| `AssetRole` | ELIGIBLE 또는 WASTE |
| `address` | ZK 소유자 식별값 |
| `opening` | commitment randomizer |

### Commitment와 소비 관계 검증

$$
cm=H(\mathrm{NoteTag},\mathrm{DocumentHash},\mathrm{AssetRole},q_{\mathrm{mass}},a_{\mathrm{rec}},e,\mathrm{address},\mathrm{opening})
$$

$$
nf=H(\mathrm{NullifierTag},\mathrm{sk}_{\mathrm{owner}},cm)
$$

Private Spend Kernel은 Note 조건, 소유자 address, commitment, Depth-32 membership과 nullifier를 하나의 proof에서 확인합니다. 이 단계에서는 Contract 상태를 변경하지 않으므로 같은 proof 관계를 검증하는 것만으로 Note가 소비되지는 않습니다. 상세 관계는 [M1 명세](M1-private-note-core.md#5-private-spend-kernel)에 있습니다.

## 3. 공개값과 비공개값

| 기능 | 공개값 | 비공개값 |
|---|---|---|
| Note Commitment | `cm` | Note 전체 |
| Private Spend Kernel | `noteRoot`, `nf` | Note, `sk_owner`, 증명 대상 `cm`, leaf index, sibling 32개 |

증명 대상 `cm`과 Tree 위치는 공개하지 않습니다.

## 4. 저장 상태

M1에는 EVM Ledger 상태가 없습니다. 다음 개발용 Artifact만 파일로 생성했습니다.

| Artifact | 역할 |
|---|---|
| CCS | compile된 Circuit |
| canonical·Lagrange SRS | 개발용 PLONK Setup |
| PK·VK | proof 생성·검증 |
| manifest | checksum·크기·환경 |

Contract Tree와 중복 nullifier mapping은 M2로 넘겼습니다.

## 5. 계획과 실제 구현의 차이

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Protocol 관계 | 동결된 M1 명세 | 동일 | 의미 변경 없음 |
| Owner | `sk_owner → address` | 동일 | PublicKey 계층 없음 |
| 측정 | Feature별 1회 | Feature별 1회 | 계획 준수 |

초기 탄소 `e`의 설명은 이후 M2 논의에서 Entry 이전 탄소를 포함할 수 있도록 명확해졌지만, M1 Circuit은 처음부터 ELIGIBLE Note의 `e>0`을 허용했으므로 Circuit 결과는 바뀌지 않았습니다.

## 6. Correctness와 최종 상태

성공:

- 고정 actor 5개의 address 파생
- ELIGIBLE·유효 WASTE Note Commitment
- 올바른 `sk_owner`와 Depth-32 path의 Private Spend Kernel
- 저장한 CCS·PK·VK 재로딩 후 PLONK proof 검증

실패:

- 잘못된 actor metadata·중복·0 secret·address
- `a_rec>q_mass`, uint64 초과, unknown role, 잘못된 WASTE State
- 변조된 DocumentHash·State·address·opening
- 잘못된 secret·index·sibling·root·nullifier

Private Spend Kernel의 public variable 수가 2개이며 증명 대상 `cm`이 포함되지 않음을 compile 결과로 확인했습니다.

## 7. 측정 경계와 환경

| 측정 | 포함 | 제외 | 횟수 |
|---|---|---|---:|
| Compile | gnark SCS compile | 파일 로딩 | 1회 |
| Setup | 개발용 SRS와 `plonk.Setup` | final universal SRS | 1회 |
| Prove·Verify | native witness와 PLONK | Contract·network | Feature별 1회 |
| Proof bytes | gnark binary serialization | Solidity calldata | Feature별 1회 |

환경: macOS arm64, Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, 10 logical CPUs, `GOMAXPROCS=10`입니다.

## 8. Circuit 성능

| Feature | Constraints | Public inputs | Compile | Setup | Prove | Verify | Binary proof |
|---|---:|---:|---:|---:|---:|---:|---:|
| Note Commitment | 5,678 | 1 | 3 ms | 279 ms | 136 ms | 2 ms | 664 B |
| Private Spend Kernel | 16,407 | 2 | 8 ms | 855 ms | 407 ms | 2 ms | 664 B |

Private Spend Kernel의 추가 10,729 constraints에는 address 검증, nullifier 계산과 Depth-32 Merkle membership 검증을 위한 Poseidon2 compression 32단계가 포함됩니다. Contract Tree append와 nullifier 저장 비용은 포함되지 않습니다.

## 9. 재현 명령

```text
make test-go
make setup-m1
make evaluate-m1
```

Raw 수치는 `output/m1-core.json`에 기록됩니다.

## 10. Artifact와 checksum

소스·test·Result·Raw JSON은 Git에 포함할 수 있습니다. `artifacts/development/m1/`의 CCS·SRS·PK·VK는 재생성 가능하므로 Git에서 제외합니다. Manifest가 각 파일의 SHA-256과 크기를 검증합니다.

## 11. 제외·한계

- Event와 Contract 상태 변경 없음
- 중복 nullifier 차단 없음
- EntryIssuer·Policy·Status·Audit·Claim 없음
- 개발용 SRS를 Production setup으로 주장하지 않음
- 단일 실행값을 안정된 평균 성능으로 주장하지 않음

## 12. M2에 무엇을 넘겼나요?

```text
M1이 제공:
  Note schema
  Note Commitment
  Private Spend Kernel
  Depth-32 native·Circuit Merkle
  Artifact runtime

M2가 추가:
  Entry Circuit
  Note Tree append
  commitments[cm]
  noteNullifiers[nf]
  Solidity·Anvil
```

## 부록 A. 선택적 과거 해석

M1 개발 당시 기존 State-3 Entry·Exit와 constraints 범위를 확인했습니다.

| 참고 대상 | Constraints | Public inputs | Membership |
|---|---:|---:|---|
| 기존 Entry | 2,621 | 1 | 없음 |
| M1 Note Commitment | 5,678 | 1 | 없음 |
| 기존 Exit | 13,136 | 3 | Depth 32 |
| M1 Private Spend Kernel | 16,407 | 2 | Depth 32 |

이 비교는 M1 완료 조건이나 현재 Protocol 명세가 아니며, 과거 constraints 질문을 복구할 때만 참고합니다.
