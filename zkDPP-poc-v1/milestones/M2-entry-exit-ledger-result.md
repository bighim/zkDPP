# M2 Entry·Exit Ledger Result

- 상태: 완료
- 구현 명세: [`M2-entry-exit-ledger.md`](M2-entry-exit-ledger.md)
- Raw 결과: [`m2-circuit.json`](../output/m2-circuit.json), [`m2-anvil-gas.json`](../output/m2-anvil-gas.json), [`m2-anvil-e2e.json`](../output/m2-anvil-e2e.json)
- 실행 횟수: Setup 1회, gas clean chain 1회, E2E clean chain 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M2 결과 |
|---|---|
| 이전 상태 | M1은 private Note·Commitment·Spend Circuit만 있고 온체인 상태 변경은 없었습니다. |
| 이번 목표 | Note를 실제 EVM Ledger에 Entry하고 private Note를 Exit합니다. |
| 실제 구현 | Entry Circuit, M1 Private Spend Kernel을 사용한 Exit, EntryIssuer 권한, Depth-32 Note Tree, commitment·nullifier 상태, Solidity·Anvil runner |
| 이제 가능한 것 | 승인된 공급자가 `cm`을 Tree에 등록하고, ZK owner의 secret으로 만든 proof를 제출해 대상 `cm`을 공개하지 않고 Exit할 수 있습니다. |
| 검증 | 전체 Go test·Foundry 7개 test 통과, Go·Solidity root·path 일치, 실패 원자성 확인 |
| 대표 성능 | Entry Circuit 5,680 constraints·약 1.49M steady-state gas; Exit가 재사용하는 M1 Private Spend Kernel 16,407 constraints·Exit transaction 398,717 gas |
| 미포함 | Transfer·Voucher·Policy·Status·Audit·Claim·Besu·universal SRS |
| 다음 단계 | M3에서 Voucher를 통한 Transfer·Proceed·Recall 생애주기를 추가합니다. |

```text
Entry:
  ∅ → Note
  commitments[cm] = true
  cm을 Note Tree에 append

Exit:
  Note → ∅
  noteNullifiers[nf] = true
  Tree는 변경하지 않음
```

## 1. M1에서 무엇이 달라졌나요?

| M1 | M2에서 추가 | 현재 가능 |
|---|---|---|
| Note Commitment Circuit | Entry 조건·EntryIssuer·Tree append | 초기 원자재 등록 |
| Private Spend Kernel | accepted root 확인·nullifier 저장 | output 없는 private Exit |
| Go·Circuit | Solidity verifier·Ledger·Anvil | 실제 EVM 실행·gas·E2E |

## 2. 실제 구현한 동작

### Entry

```text
EntryIssuer 권한·중복 cm 확인
  → Entry proof 검증
  → commitments[cm]=true
  → Depth-32 Tree append
```

초기 `e>0`을 허용하며 Entry 이전의 탄소를 포함할 수 있습니다.

### Exit

```text
accepted root·중복 nf 확인
  → M1 Private Spend Kernel proof 검증
  → noteNullifiers[nf]=true
```

여기서 accepted root는 Contract가 과거 Entry를 통해 실제로 생성됐다고 기록한 Note Tree root입니다. Exit는 새 Circuit을 만들지 않고 M1 Private Spend Kernel을 재사용합니다.

Exit는 새 commitment·Tree node·root·leafCount를 만들지 않습니다. Private Spend proof를 검증한 뒤 `nf`를 저장하는 시점에 실제 Ledger 소비가 완료됩니다. 상세 수도 코드는 [M2 명세](M2-entry-exit-ledger.md#5-exit)에 있습니다.

### Circuit과 Contract의 책임

| 기능 | Circuit이 확인하는 것 | Contract가 확인·변경하는 것 |
|---|---|---|
| Entry | Note 조건, `q_mass>0`, ELIGIBLE, private Note와 public `cm`의 관계 | EntryIssuer 권한, 중복 `cm`, proof, `commitments[cm]=true`, Tree append |
| Exit | 소유권, 비공개 `cm`의 Merkle membership, public `nf` 관계 | accepted root, 중복 `nf`, proof, `noteNullifiers[nf]=true` |

## 3. 공개값과 비공개값

| Event | 공개값 | 비공개값 |
|---|---|---|
| Entry | `cm` | DocumentHash, State, AssetRole, address, opening |
| Exit | `noteRoot`, `nf` | Note, `sk_owner`, 소비 `cm`, leaf index, sibling 32개 |

Entry에서 EVM account는 EntryIssuer 권한을 나타내고, ZK address는 Note 소유자를 나타냅니다. 두 값은 암호학적으로 binding하지 않습니다.

Exit 권한은 ZK owner의 `sk_owner`로 만든 proof가 증명합니다. Exit transaction을 제출하는 EVM account 자체는 ZK owner와 binding되지 않습니다. Canonical 시나리오에서는 Note 소유자가 직접 제출했지만 Contract가 이를 강제하는 것은 아닙니다.

## 4. 저장 상태와 변경 Event

| 상태 | 역할 | 변경 함수 |
|---|---|---|
| `entryIssuers` | Entry 호출 권한 | `setEntryIssuer` |
| `commitments` | 과거에 등록된 output `cm` | `entry` |
| `noteNullifiers` | Exit에 사용된 `nf`를 기록하여 같은 Note의 재소비 방지 | `exit` |
| `noteTreeNodes` | 현재 leaf·중간 node | `entry` |
| `acceptedRoots` | 유효한 현재·과거 root | `entry` |
| `currentNoteRoot` | 최신 root | `entry` |
| `noteLeafCount` | leaf 개수 | `entry` |

`commitments[cm]=true`는 해당 `cm`이 등록됐다는 뜻이지 unspent 상태라는 뜻이 아닙니다. Exit는 사용된 `nf`를 `noteNullifiers`에 저장하여 같은 Note의 재소비를 막습니다. 소비된 `cm`은 비공개이므로 Contract에서 특정 `cm`의 소비 여부를 직접 조회할 수는 없습니다.

## 5. 계획과 실제 구현의 차이

| 항목 | 계획 | 실제 | 이유·영향 |
|---|---|---|---|
| Protocol 의미 | M2 동결 명세 | 동일 | 의미 변경 없음 |
| Anvil RPC | 8545 | 18545 | 다른 local service와 포트 충돌, Protocol 영향 없음 |
| Invalid proof | verifier가 false 반환 가능 | 내부 curve 연산에서 revert 가능 | Contract가 모두 `InvalidProof`로 정규화 |
| Exit Circuit | 별도 Circuit 없음 | M1 Private Spend Kernel CCS·PK·VK 재사용 | 중복 Setup 제거 |

## 6. Correctness와 최종 상태

Go test:

- 초기 `e=0`, `e>0` Entry 성공
- `q_mass=0`, WASTE Entry, `a_rec>q_mass`, uint64 초과, 변조 commitment 실패
- Entry public input 1개, Private Spend Kernel public input 2개
- M1 artifact 재사용

Foundry 7개 test:

- Admin 승인·해제와 비관리자 거부
- EntryIssuer 3개의 Entry와 Tree root·path 일치
- 권한 없는·해제된 issuer, 중복 commitment 거부
- 올바른 Exit, invalid root·proof와 중복 nullifier 처리
- 실패 후 commitment·root·leafCount·nullifier 불변
- Exit 후 root·leafCount 불변

최종 상태:

```text
Entry 3건 후 leafCount = 3
Exit 후 leafCount      = 3
Exit 후 root           = Entry 후 root와 동일
noteNullifiers[nf]     = true
Go path == Contract path 32개
```

## 7. 측정 경계와 환경

| 측정 | 포함 | 제외 | 횟수 |
|---|---|---|---:|
| Circuit | Compile·Setup·Prove·Verify, proof bytes | Contract·network | Feature별 1회 |
| Gas | fixed proof transaction receipt·calldata | live proof 생성 | clean chain 1회 |
| E2E | witness·prove·tx prepare·submit-to-receipt | Compile·Setup·deployment | 별도 clean chain 1회 |

환경: Docker Anvil, chain ID 31337, RPC 18545, Prague, block gas limit 30M, Solidity 0.8.30, Foundry 1.7.1, Go 1.25.7입니다.

`tx prepare`에는 ABI encoding, nonce·gas price 조회, `eth_estimateGas`와 서명이 포함됩니다.

## 8. Circuit 성능

| Feature | Constraints | Public inputs | Compile | Setup | Prove | Verify | Binary | Solidity |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Entry | 5,680 | 1 | 6 ms | 285 ms | 131.620 ms | 2.653 ms | 664 B | 1,056 B |
| M1 Private Spend Kernel, Exit에서 재사용 | 16,407 | 2 | 8 ms | 855 ms | 433.439 ms | 2.571 ms | 664 B | 1,056 B |

Exit는 M1 Private Spend Kernel artifact를 재사용했으며 별도 Exit Circuit·Setup을 만들지 않았습니다.

## 9. Deployment·권한 gas

| Transaction | Receipt gas |
|---|---:|
| Poseidon2 deployment | 2,282,769 |
| Entry verifier deployment | 1,601,844 |
| Private Spend verifier deployment | 1,601,592 |
| EntryExitLedger deployment | 2,115,680 |
| EntryIssuer 승인 A | 45,490 |
| EntryIssuer 승인 B | 45,478 |
| EntryIssuer 승인 C | 45,490 |

Contract deployment 합계는 7,601,885 gas입니다. Ledger deployment는 zero Hash 32단계 초기화를 포함합니다.

## 10. Entry·Exit gas

| Event | Case | Receipt gas | Calldata |
|---|---|---:|---:|
| Entry | Raw Material A, 첫 append | 2,053,300 | 1,156 B |
| Entry | Raw Material B | 1,486,891 | 1,156 B |
| Entry | Raw Material C | 1,503,979 | 1,156 B |
| Exit | Raw Material A | 398,717 | 1,188 B |

첫 Entry는 비어 있던 Tree storage를 처음 기록하므로 이후 Entry보다 비쌉니다. Exit는 Circuit membership과 Contract nullifier 저장만 수행해 Tree append가 없습니다.

## 11. Live E2E

| Event | Case | Witness | Prove | Tx prepare | Submit→receipt | E2E |
|---|---|---:|---:|---:|---:|---:|
| Entry | A | 0.125 ms | 133.213 ms | 183.565 ms | 54.104 ms | 371.029 ms |
| Entry | B | 0.068 ms | 151.429 ms | 159.299 ms | 54.157 ms | 364.974 ms |
| Entry | C | 0.059 ms | 156.471 ms | 193.668 ms | 53.850 ms | 404.068 ms |
| Exit | A | 0.048 ms | 468.772 ms | 84.174 ms | 53.741 ms | 606.757 ms |

Exit는 Tree append가 없어 Entry보다 gas가 낮지만, Depth-32 membership proof를 생성하므로 이 실행에서는 Prove 시간과 전체 E2E가 가장 컸습니다.

## 12. 재현 명령

```text
make test-go
make setup-m2
make test-contract-m2
make benchmark-m2-gas
make benchmark-m2-e2e
```

## 13. Artifact와 checksum

Entry 개발용 CCS·SRS·PK·VK, Solidity verifier, Poseidon2 source와 고정 proof fixture는 재생성 가능하므로 Git에서 제외합니다. Generated checksum은 [`m2-generated-checksums.json`](../output/m2-generated-checksums.json), 실제 수치는 `output/m2-*.json`에 기록합니다. M1 raw checksum도 함께 확인했습니다.

## 14. 제외·한계

- Voucher·Transfer·Proceed·Recall 없음
- Merge·Split·Process 없음
- Policy·Status·Audit·Claim 없음
- 과거 root별 path snapshot 없음
- Besu·final universal SRS 없음
- 단일 POC 실행값이며 반복 평균이나 Production 성능이 아님

## 15. M3에 무엇을 넘겼나요?

```text
M2가 제공:
  Note Tree·현재 path
  Entry output 등록
  Private Spend Kernel·Note nullifier
  Solidity verifier·Anvil runner

M3가 추가:
  Voucher Tree
  Transfer + Change Note
  Proceed·Recall
  Voucher nullifier·deadline
```

M3의 기존 논의와 미결정 항목은 [Future Context](FUTURE-MILESTONE-CONTEXT.md#m3)에서 확인합니다.
