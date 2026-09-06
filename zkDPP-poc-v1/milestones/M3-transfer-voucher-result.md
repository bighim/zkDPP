# M3 Transfer·Voucher Result

- 상태: 완료
- 구현 명세: [`M3-transfer-voucher.md`](M3-transfer-voucher.md)
- Raw 결과: [`m3-circuit.json`](../output/m3-circuit.json), [`m3-anvil-gas.json`](../output/m3-anvil-gas.json), [`m3-anvil-e2e.json`](../output/m3-anvil-e2e.json)
- 실행 횟수: Setup 1회, gas clean chain 1회, E2E clean chain 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M3 결과 |
|---|---|
| 이전 상태 | M2는 Note를 Entry·Exit할 수 있지만 다른 ZK owner에게 전달할 수 없었습니다. |
| 이번 목표 | private Note를 pending Voucher로 전달하고 Proceed·Recall 중 하나로 한 번만 해결합니다. |
| 실제 구현 | Voucher Core, Transfer·Proceed·Recall Circuit, Voucher Tree·nullifier, dual-tree Solidity Ledger, Anvil runner |
| 이제 가능한 것 | 부분·전량 Transfer, Receiver 수령, deadline 전 Sender 회수를 소비 `cm`·`rv` 공개 없이 실행할 수 있습니다. |
| 검증 | 전체 Go test와 Foundry 11개 test 통과, Go·Solidity Note/Voucher root·path·nullifier 일치 |
| 대표 성능 | Transfer 35,388 constraints·3,212,260 gas·1,117.289 ms; Proceed 22,821·1,511,149 gas·678.147 ms; Recall 26,129·1,512,324 gas·687.219 ms |
| 미포함 | Voucher 암호화 전달, 운송 탄소, EVM-ZK binding, Policy·Status·Audit·Claim·Besu |
| 다음 단계 | M4에서 Merge·Split의 compatibility·State 보존·rounding을 결정합니다. |

```text
Transfer:
  private Note → public Voucher + public Change Note

Proceed:
  private Voucher → Receiver Note

Recall:
  private Voucher → Sender Note
```

## 1. M2에서 무엇이 달라졌나요?

| M2 | M3에서 추가 | 현재 가능 |
|---|---|---|
| Note Tree | 독립 Voucher Tree | pending Transfer membership |
| Note nullifier | Voucher nullifier | Proceed·Recall 중 한 번만 성공 |
| Entry·Exit | Transfer·Proceed·Recall | private 소유권 이전·회수 |
| 단일 Tree Contract | 공통 helper를 사용하는 dual-tree Contract | Note와 Voucher 원자적 append |

## 2. 실제 구현한 동작

### Transfer

```text
private input Note 검증
  → Sender가 q_voucher 선택
  → Change State를 내림 계산
  → residual을 Voucher에 배분
  → Note nf 저장
  → Change Note와 Voucher를 각각 Tree에 append
```

ELIGIBLE·WASTE를 모두 허용하고 DocumentHash·AssetRole을 유지합니다. 운송 탄소는 추가하지 않습니다. 전량 Transfer도 `q_mass=a_rec=e=0`인 fresh Change Note를 생성합니다.

부분 Transfer fixture는 다음 결과를 사용했습니다.

| 출력 | `q_mass` | `a_rec` | `e` |
|---|---:|---:|---:|
| Voucher | 1,000,000,000 | 333,333,334 | 666,666,667 |
| Change Note | 2,000,000,000 | 666,666,666 | 1,333,333,333 |

### Proceed·Recall

Proceed는 Receiver secret을, Recall은 Sender secret과 strict deadline을 검증합니다. 두 기능은 같은 Voucher opening으로 다음 값을 계산합니다.

$$
rvnf=H(\mathrm{VoucherNullifierTag},o_{rv},rv)
$$

먼저 성공한 transaction이 `voucherNullifiers[rvnf]=true`를 저장하므로 다른 resolution은 실패합니다. Proceed는 deadline을 검사하지 않고 Recall만 `currentEpoch < deadlineEpoch`을 검증합니다.

## 3. 무엇이 공개되고 무엇이 숨겨지나요?

| Event | 공개값 | 비공개값 |
|---|---|---|
| Transfer | `noteRoot`, `nf`, `rvNew`, `cmChange`, `transferEpoch`, `deltaEpoch` | 소비 Note·`cm`·path·State, Sender secret, Receiver address |
| Proceed | `voucherRoot`, `rvnf`, `cmReceiver` | 소비 Voucher·`rv`·opening·path·State, Receiver secret |
| Recall | `voucherRoot`, `rvnf`, `cmReturn`, `currentEpoch` | 소비 Voucher·`rv`·opening·deadline·path·State, Sender secret |

Transfer에서 새 `rvNew`는 public Tree leaf이지만 Proceed·Recall에서 어떤 `rv`를 소비했는지는 공개하지 않습니다. Contract ABI와 generated verifier public witness에서 consumed `cm`·`rv`가 제거됐음을 확인했습니다.

## 4. Circuit과 Contract의 책임

| 기능 | Circuit이 확인하는 것 | Contract가 확인·변경하는 것 |
|---|---|---|
| Transfer | Note 소유·membership·nf, State 배분, Voucher·Change, deadline binding | accepted Note root, 중복·epoch, Note nf, 두 Tree append |
| Proceed | private Voucher membership, Receiver 권한, rvnf, output Note | accepted Voucher root, 중복 rvnf·output, Note append |
| Recall | private Voucher membership, Sender 권한, rvnf, strict deadline, output Note | accepted Voucher root, 중복 rvnf·output, 실행 epoch, Note append |

Contract는 `deadlineRV[rv]`를 저장하거나 resolution에서 raw `rv`를 조회하지 않습니다. Voucher의 존재와 deadline 관계는 Circuit에서 증명합니다. `deadlineEpoch` 숫자는 Transfer의 public `transferEpoch+deltaEpoch`로 계산할 수 있지만, resolution이 어느 Voucher를 소비했는지는 숨겨집니다.

## 5. 저장 상태

| 상태 | 역할 | 변경 Event |
|---|---|---|
| `commitments` | Note output 중복 방지 | Entry·Transfer·Proceed·Recall |
| `noteNullifiers` | Note 재소비 방지 | Exit·Transfer |
| `voucherCommitments` | 새 Voucher 중복 방지 | Transfer |
| `voucherNullifiers` | Proceed·Recall 이중 해결 방지 | Proceed·Recall |
| Note Tree | finalized·Change Note | Entry·Transfer·Proceed·Recall |
| Voucher Tree | pending Voucher | Transfer |

최종 상태는 Note leaf 6개, Voucher leaf 2개입니다. 두 Note nullifier와 두 Voucher nullifier가 모두 기록됐고 Go·Solidity root와 현재 path가 일치했습니다.

## 6. 계획과 실제 구현의 차이

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Protocol 의미 | M3 동결 명세 | 동일 | 의미 변경 없음 |
| Ledger 이름 | 기존 이름 유지 | `EntryExitLedger` 확장 | M9 Router 전까지 rename 없음 |
| SRS | Circuit별 development SRS | Recall은 Proceed와 같은 domain SRS cache 사용 | Recall SRS 생성시간 0 ms, final universal SRS 아님 |
| Docker | 실행 가능 환경 | 처음 compile 확인 전에 Docker Desktop 시작 필요 | 공식 test·측정 결과 영향 없음 |

## 7. Correctness와 최종 상태

공식 전체 Go suite가 통과했습니다.

- Native Voucher commitment·nullifier와 Circuit 일치
- 부분 residual·전량 zero Change·WASTE Transfer
- public variable 수 Transfer 6, Proceed 3, Recall 4
- 잘못된 remainder·path·opening·deadline 실패

Foundry는 기존 M2 7개와 M3 4개, 총 11개 test가 모두 통과했습니다.

- canonical Transfer·Proceed·Recall과 양 Tree root·path
- deadline 이후 Proceed 성공, deadline 경계 Recall 실패
- Proceed·Recall single-use Voucher nullifier
- invalid epoch·proof 이후 두 Tree·mapping 불변
- 기존 Entry·Exit 회귀 없음

## 8. 측정 경계와 환경

| 측정 | 포함 | 제외 | 횟수 |
|---|---|---|---:|
| Circuit | compile·development SRS·Setup·witness·Prove·Verify | Contract·network | Feature별 1회 |
| Gas | fixed proof transaction receipt·calldata | live proof 생성 | clean chain 1회 |
| E2E | witness·prove·tx prepare·submit-to-receipt | compile·Setup·deployment | 별도 clean chain 1회 |

환경은 Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, BLS12-381 PLONK-KZG, Solidity 0.8.30, Foundry·Anvil 1.7.1, Prague와 chain ID 31337입니다.

## 9. Circuit 결과

| Feature | Constraints | Public | Compile | SRS | PLONK Setup | Prove | Verify | Proof |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Transfer | 35,388 | 6 | 16 ms | 1,479 ms | 279 ms | 866.076 ms | 2.729 ms | 664 B |
| Proceed | 22,821 | 3 | 9 ms | 814 ms | 156 ms | 463.241 ms | 2.675 ms | 664 B |
| Recall | 26,129 | 4 | 10 ms | 0 ms | 167 ms | 466.870 ms | 2.735 ms | 664 B |

Solidity proof serialization은 세 기능 모두 1,056 B입니다. Transfer는 Note membership, 배분과 두 output commitment를 모두 포함해 가장 큽니다. Recall은 Proceed에 uint64 current epoch과 strict comparison이 추가됩니다.

## 10. Deployment·Event gas

Verifier 5개, Poseidon2와 dual-tree Ledger의 deployment 합계는 14,373,310 gas입니다. Ledger deployment 4,080,349 gas에는 Note·Voucher zero Hash 각 32단계 초기화가 포함됩니다.

| Event | Case | Receipt gas | Calldata |
|---|---|---:|---:|
| Entry | 첫 Note | 2,053,875 | 1,156 B |
| Entry | 두 번째 Note | 1,487,489 | 1,156 B |
| Transfer | 부분·첫 Voucher append | 3,212,260 | 1,316 B |
| Proceed | 부분 Voucher | 1,511,149 | 1,220 B |
| Transfer | 전량·두 번째 Voucher append | 2,662,914 | 1,316 B |
| Recall | 전량 Voucher | 1,512,324 | 1,252 B |

Transfer는 Note Tree와 Voucher Tree를 함께 갱신하므로 가장 비쌉니다. 첫 Voucher append는 비어 있던 Voucher storage를 처음 기록해 두 번째 Transfer보다 549,346 gas 높았습니다.

## 11. Live E2E

| Event | Case | Witness | Prove | Tx prepare | Submit→receipt | E2E |
|---|---|---:|---:|---:|---:|---:|
| Entry | 첫 Note | 0.093 ms | 139.551 ms | 167.620 ms | 53.529 ms | 360.812 ms |
| Entry | 두 번째 Note | 0.062 ms | 152.025 ms | 167.299 ms | 52.727 ms | 372.125 ms |
| Transfer | 부분 | 0.070 ms | 802.923 ms | 261.680 ms | 52.593 ms | 1,117.289 ms |
| Proceed | 부분 | 0.090 ms | 461.323 ms | 162.324 ms | 54.393 ms | 678.147 ms |
| Transfer | 전량 | 0.055 ms | 822.165 ms | 236.663 ms | 55.419 ms | 1,114.326 ms |
| Recall | 전량 | 2.172 ms | 466.009 ms | 166.239 ms | 52.785 ms | 687.219 ms |

Transfer E2E는 35,388-constraint proof 생성이 지배합니다. Proceed·Recall은 gas가 거의 같고 Recall의 추가 deadline 관계만 Circuit 규모에 반영됩니다.

## 12. 재현 명령

```text
make setup-m3
make test-go
make test-contract-m3
make benchmark-m3-gas
make benchmark-m3-e2e
```

## 13. Artifact와 checksum

개발용 CCS·SRS·PK·VK, generated verifier와 fixed proof는 Git에서 제외하고 재생성합니다. 각 manifest와 [`m3-generated-checksums.json`](../output/m3-generated-checksums.json)이 파일 checksum을 검증합니다. M1·M2 Raw JSON checksum도 M3 Setup 전후 동일했습니다.

## 14. 제외·한계

- $o_{rv}$의 암호화 전달·key exchange 없음
- 운송 탄소·Transfer Policy 없음
- EVM account와 ZK owner binding·relayer 없음
- Status·Audit·Claim·Policy 없음
- 과거 root별 path snapshot 없음
- development SRS이며 Production setup 아님
- 공식 성능은 case별 단일 실행값이며 평균이 아님

## 15. M4에 무엇을 넘겼나요?

```text
M3가 제공:
  private Note 소비와 output Note append
  uint64 State 비례 배분·residual
  두 output을 원자적으로 생성하는 Contract
  dual-tree helper·M3 Anvil runner

M4가 결정:
  Merge compatibility
  Split output 순서와 residual
  질량 0 Split output 허용 여부
```

M4의 기존 논의는 [Future Context](FUTURE-MILESTONE-CONTEXT.md#m4)에서 확인합니다.
