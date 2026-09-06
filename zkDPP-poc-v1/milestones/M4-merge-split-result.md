# M4 Merge·Split Result

- 상태: 완료
- 구현 명세: [`M4-merge-split.md`](M4-merge-split.md)
- Raw 결과: [`m4-circuit.json`](../output/m4-circuit.json), [`m4-anvil-gas.json`](../output/m4-anvil-gas.json), [`m4-anvil-e2e.json`](../output/m4-anvil-e2e.json)
- 실행 횟수: Setup 1회, gas clean chain 1회, E2E clean chain 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M4 결과 |
|---|---|
| 이전 상태 | M3는 한 Note를 Voucher와 Change로 전달하지만 여러 finalized Note를 합치거나 다시 나눌 수 없었습니다. |
| 이번 목표 | private Note의 소유·membership을 숨기면서 Merge State 합과 Split 비례 배분을 검증합니다. |
| 실제 구현 | 공통 mass allocation, Merge·Split Circuit, Ledger API, 연결 fixture, Foundry·Anvil runner |
| 이제 가능한 것 | 같은 owner·같은 AssetRole Note를 Merge하고 State 보존·residual이 적용된 Note 두 개로 Split할 수 있습니다. |
| 검증 | 전체 Go test와 Foundry 14개 test 통과, Go·Solidity final root·path·nullifier 일치 |
| 대표 성능 | Merge 38,204 constraints·1,554,376 gas·1,016.053 ms; Split 34,400·2,381,557 gas·1,081.004 ms |
| 미포함 | ProductProfile·DocumentHash compatibility, 다른 owner Merge, 소유권 이전, Policy·Status·Audit·Besu |
| 다음 단계 | M5에서 exact-arity Process와 Policy별 State delta·allocation을 구현합니다. |

```text
Merge:
  private Note 2개 → public Note 1개

Split:
  private Note 1개 → public Note 2개
```

## 1. M3에서 무엇이 달라졌나요?

| M3 | M4에서 추가 | 현재 가능 |
|---|---|---|
| 한 input Transfer | 두 input Merge | private State 합 |
| Change·Voucher 배분 | finalized Note 두 output Split | private lot 분할 |
| Voucher Tree 사용 가능 | Note Tree만 사용하는 Merge·Split | finalized 객체 산술 비교 |
| M3 전용 allocation | 공통 `allocation` Core | Split·향후 Process 재사용 |

## 2. 실제 구현한 동작

### Merge

```text
같은 root의 private Note 2개 검증
  → 같은 owner·AssetRole 확인
  → q_mass·a_rec·e checked sum
  → input nullifier 2개 저장
  → output Note 1개 append
```

ELIGIBLE+ELIGIBLE과 WASTE+WASTE가 성공하며 ELIGIBLE+WASTE는 실패합니다. Output은 같은 owner·Role을 유지하지만 새 DocumentHash를 사용할 수 있습니다.

### Split

```text
private Note 1개 검증
  → owner가 private q_out1 선택
  → output 2를 내림 계산
  → output 1에 residual 배분
  → input nullifier 저장
  → output Note 2개 순차 append
```

Input은 양의 질량이어야 하지만 output 하나의 zero-State는 허용합니다. 두 output은 input owner·AssetRole을 유지하며 새 DocumentHash를 사용할 수 있습니다.

## 3. 논문 Protocol과 POC 경계

논문에서는 `input1.ProductProfile == input2.ProductProfile`을 요구합니다. M4 POC에는 ProductProfile이 없으며 같은 `sk_owner`와 같은 AssetRole만 검사합니다.

따라서 같은 owner·Role이면 서로 다른 종류의 물품도 POC Merge proof를 만들 수 있습니다. M4 결과는 ProductProfile 검증 성능이 아니라 다중 private membership·State 합·nullifier·Tree update workload입니다.

## 4. 무엇이 공개되고 무엇이 숨겨지나요?

| Event | 공개값 | 비공개값 |
|---|---|---|
| Merge | `noteRoot`, `nf1`, `nf2`, `cmOut` | input Note·`cm`·path 2개, `sk_owner`, output State·DocumentHash·opening |
| Split | `noteRoot`, `nf`, `cmOut1`, `cmOut2` | input Note·`cm`·path, `sk_owner`, output State·DocumentHash·opening·remainder |

Contract ABI와 verifier public witness에는 consumed `cm`과 ProductProfile이 없습니다.

## 5. Circuit과 Contract의 책임

| Event | Circuit | Contract |
|---|---|---|
| Merge | 소유·membership·nf 2개, Role equality, checked State sum, output commitment | accepted root, duplicate·spent nf, output 중복, nullifier 2개·Note 1개 저장 |
| Split | 소유·membership·nf, 같은 Role, 질량 합, floor·residual, output 2개 | accepted root, spent nf, output 중복, nullifier 1개·Note 2개 저장 |

Split의 두 번째 output insert가 실패하면 transaction atomicity로 첫 output과 nullifier 변경도 모두 revert됩니다.

## 6. 계획과 실제 구현의 차이

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Protocol 의미 | M4 명세 | 동일 | 의미 변경 없음 |
| ProductProfile | POC에서 제외 | 제외 | 논문 compatibility 성능 아님 |
| Allocation | M3 관계 공통화 | `internal/core/allocation`으로 승격 | M3 결과 회귀 없음 |
| SRS | Circuit별 development Setup | Split이 Merge와 같은 domain SRS cache 사용 | Split SRS 생성 0 ms |

## 7. Correctness와 최종 상태

공식 전체 Go suite가 통과했습니다.

- Merge·Split public variable 각각 4개
- ELIGIBLE·WASTE 동일 Role Merge
- 서로 다른 Role·owner·duplicate input 실패
- uint64 overflow 실패
- Split residual·zero output 성공과 잘못된 배분 실패
- M3 Voucher allocation 공통화 회귀 없음

Foundry는 M2 7개, M3 4개, M4 3개로 총 14개 test가 통과했습니다.

- Entry→Merge→Split canonical flow
- duplicate input과 invalid proof atomicity
- Split replay 차단
- M2·M3 Contract 기능 회귀 없음

최종 Note Tree leaf는 5개입니다. Merge input nullifier 2개와 Split input nullifier 1개가 기록됐고 Go·Solidity root와 현재 path가 일치했습니다.

## 8. 측정 경계와 환경

| 측정 | 포함 | 제외 | 횟수 |
|---|---|---|---:|
| Circuit | compile·development SRS·Setup·witness·Prove·Verify | Contract·network | Feature별 1회 |
| Gas | fixed proof receipt·calldata | live proof 생성 | clean chain 1회 |
| E2E | witness·prove·tx prepare·submit-to-receipt | compile·Setup·deployment | 별도 clean chain 1회 |

환경은 Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, BLS12-381 PLONK-KZG, Solidity 0.8.30, Foundry·Anvil 1.7.1, Prague와 chain ID 31337입니다.

## 9. Circuit 결과

| Feature | Constraints | Public | Compile | SRS | PLONK Setup | Prove | Verify | Proof |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Merge | 38,204 | 4 | 17 ms | 1,369 ms | 255 ms | 774.927 ms | 2.633 ms | 664 B |
| Split | 34,400 | 4 | 14 ms | 0 ms | 252 ms | 748.769 ms | 2.595 ms | 664 B |

두 Solidity proof serialization은 모두 1,056 B입니다. Merge는 membership 2개와 nullifier 2개를 검증해 Split보다 constraints가 많습니다.

## 10. Deployment·Event gas

Poseidon2, verifier 7개와 Ledger deployment 합계는 17,740,657 gas입니다. Ledger deployment는 4,242,976 gas입니다.

| Event | Receipt gas | Calldata |
|---|---:|---:|
| Entry 1 | 2,053,907 | 1,156 B |
| Entry 2 | 1,487,557 | 1,156 B |
| Merge | 1,554,376 | 1,252 B |
| Split | 2,381,557 | 1,252 B |

Split은 output Note 두 개를 append하므로 한 개를 append하는 Merge보다 827,181 gas 높았습니다.

## 11. Live E2E

| Event | Witness | Prove | Tx prepare | Submit→receipt | E2E |
|---|---:|---:|---:|---:|---:|
| Entry 1 | 0.155 ms | 135.488 ms | 161.086 ms | 53.227 ms | 349.973 ms |
| Entry 2 | 0.106 ms | 156.595 ms | 148.563 ms | 54.188 ms | 359.467 ms |
| Merge | 0.305 ms | 795.658 ms | 165.932 ms | 54.136 ms | 1,016.053 ms |
| Split | 0.200 ms | 775.783 ms | 250.145 ms | 54.759 ms | 1,081.004 ms |

Merge는 proving 시간이 더 길지만 Split은 output 두 개의 gas estimation·Tree update 때문에 transaction 준비와 전체 E2E가 더 길었습니다.

## 12. 재현 명령

```text
make setup-m4
make test-go
make test-contract-m4
make benchmark-m4-gas
make benchmark-m4-e2e
```

## 13. Artifact와 checksum

M4 CCS·SRS·PK·VK, generated verifier와 fixed proof는 재생성 가능하므로 Git에서 제외합니다. [`m4-generated-checksums.json`](../output/m4-generated-checksums.json)이 생성 파일과 M1~M3 Raw 결과 checksum을 검증합니다.

## 14. 제외·한계

- ProductProfile·ProductTypeHash·MergeProfile 없음
- DocumentHash·ProductName·LotID compatibility 없음
- 서로 다른 owner의 공동 Merge 없음
- Merge·Split 중 소유권 이전 없음
- Policy·Status·Audit·Claim·Besu 없음
- development SRS이며 Production setup 아님
- 공식 성능은 case별 단일 실행값이며 평균이 아님

## 15. M5에 무엇을 넘겼나요?

```text
M4가 제공:
  다중 private Note membership·nullifier
  checked uint64 State 합
  공통 질량 비례 allocation·residual
  다중 output 순차 insert·atomicity

M5가 추가:
  exact (m,n) Process Circuit
  Process delta·allocation 규칙
  PolicyRef·VK·Grant
```

M5의 기존 논의는 [Future Context](FUTURE-MILESTONE-CONTEXT.md#m5)에서 확인합니다.
