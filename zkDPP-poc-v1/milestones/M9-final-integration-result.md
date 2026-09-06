# M9 — 최종 Protocol을 하나의 SRS와 Anvil에서 재현했나요?

- 상태: 완료
- 구현 기준: [M9 명세](M9-final-integration.md)
- 이전 결과: [M8 Result](M8-exit-dpp-issue-result.md)
- Raw: [SRS](../output/m9-srs.json), [Circuit](../output/m9-circuit.json), [Anvil](../output/m9-anvil.json), [감사](../output/m9-audit.json), [checksum](../output/m9-generated-checksums.json)
- 공식 실행: final Setup 1회, 독립 Setup 측정 1회, Anvil 전체 시나리오 2회, 감사 3회입니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

**M8까지 만든 최종 Circuit 10개를 하나의 $2^{17}$ universal SRS로 Setup하고, 원자재 4개가 DPP·Claim이 되는 전체 흐름을 ZkDPPClaimLedger와 Anvil에서 재현했습니다.**

| 질문 | M9 결과 |
|---|---|
| 이전 상태 | Circuit마다 독립 개발 SRS와 milestone별 시나리오를 사용했습니다. |
| 이번 목표 | 하나의 universal SRS·final PK/VK·Main Contract·전체 시나리오를 재현하는 것입니다. |
| 실제 구현 | final SRS runtime, Circuit 10개, Process Policy routing, 21개 Event 기록, 양방향 최종 감사입니다. |
| 이제 가능한 것 | clean Anvil에서 Entry→Transfer·Recall·Proceed→Merge→Process→Split→DPP·Claim을 한 번에 실행할 수 있습니다. |
| 검증 | 전체 Go suite 통과, Foundry 37개 test 통과, Anvil 두 run의 gas·calldata·bytecode가 일치했습니다. |
| 대표 성능 | canonical SRS 생성 평균 약 1.431초, Process Setup 약 0.54초·Prove 약 1.62초입니다. |
| 감사 | backward median 약 8.174초, forward median 약 4.082초입니다. |
| 미포함 | Besu·Router·Production Ceremony·$2^{20}$ SRS·새 Policy·ProductProfile입니다. |
| 다음 단계 | M9은 계획된 POC의 최종 통합 단계이며 이후 작업은 별도 연구·고도화 범위입니다. |

최종 구조는 다음입니다.

```text
Universal canonical SRS 1개
  → domain별 Lagrange SRS 4개
  → final Circuit별 PK·VK 10개
  → verifier Contract 10개
  → ZkDPPClaimLedger 1개
  → Anvil 전체 시나리오
```

Circuit이 SRS 안에 저장되는 것은 아닙니다. 같은 Powers of Tau를 사용해 각 Circuit의 CCS·PK·VK를 별도로 생성했습니다.

## 1. 어떤 Circuit을 최종 대상으로 사용했나요?

| 분류 | Relation | Constraints | Public | Domain |
|---|---|---:|---:|---:|
| 고정 Event | Audit Entry | 19,020 | 4 | $2^{15}$ |
| 고정 Event | Audit Transfer | 53,836 | 11 | $2^{16}$ |
| 고정 Event | Audit Proceed | 39,766 | 7 | $2^{16}$ |
| 고정 Event | Audit Recall | 43,375 | 8 | $2^{16}$ |
| 고정 Event | Audit Merge | 57,551 | 9 | $2^{16}$ |
| 고정 Event | Audit Split | 52,246 | 9 | $2^{16}$ |
| 고정 Event | Exit DPP | 36,930 | 6 | $2^{16}$ |
| Policy | Audit Process 3-to-2 | 93,578 | 15 | $2^{17}$ |
| Policy | Issue Standard V1 | 12,490 | 2 | $2^{14}$ |
| Policy | Issue Strict V2 | 12,490 | 2 | $2^{14}$ |

M7 terminal Exit·M6 Status-aware Circuit·초기 Event·독립 암호화 진단 Circuit은 final Setup에서 제외하고 과거 baseline으로 보존했습니다.

public-input manifest에는 relation별 Field 이름과 순서를 저장했습니다. gnark public witness·generated verifier·Solidity calldata가 같은 순서를 사용하는지 대조했습니다.

## 2. Universal SRS는 어떻게 만들었나요?

가장 큰 Process의 system size는 다음입니다.

$$
93{,}578+15=93{,}593
$$

따라서 필요한 다음 2의 거듭제곱은 $2^{17}=131{,}072$입니다. PLONK blinding용 point 3개를 더해 canonical SRS는 131,075 points입니다.

| Artifact | Points | 파일 크기 |
|---|---:|---:|
| universal canonical | 131,075 | 6,340,228 B |
| Lagrange $2^{14}$ | 16,384 | 835,060 B |
| Lagrange $2^{15}$ | 32,768 | 1,621,492 B |
| Lagrange $2^{16}$ | 65,536 | 3,194,356 B |
| Lagrange $2^{17}$ | 131,072 | 6,340,084 B |

Run 1 canonical checksum은 `59b67eb470c8df6dbc85afa9ce9bca2da72b73c22f3e2161280346b2a993caff`입니다. 10개 Circuit manifest가 모두 이 checksum을 참조합니다.

Run 1 SRS·PK·VK만 final Artifact로 보존했습니다. Run 2는 독립적인 development $	au$로 생성해 Setup·Prove·Verify가 다시 성공하는지만 확인했습니다.

이 SRS는 `developmentOnly=true`, `productionCeremony=false`입니다. Ethereum EIP-4844 Ceremony 결과나 Production MPC SRS를 사용하지 않았습니다.

## 3. SRS·Setup은 두 실행에서 얼마나 달랐나요?

| 작업 | Run 1 | Run 2 | 차이 판단 |
|---|---:|---:|---|
| canonical SRS 생성 | 1,427.722 ms | 1,434.709 ms | 20% 이하 |
| Lagrange $2^{14}$ | 1,150.750 ms | 1,178.258 ms | 20% 이하 |
| Lagrange $2^{15}$ | 2,436.173 ms | 2,433.069 ms | 20% 이하 |
| Lagrange $2^{16}$ | 5,157.731 ms | 5,195.609 ms | 20% 이하 |
| Lagrange $2^{17}$ | 10,930.549 ms | 10,944.703 ms | 20% 이하 |

SRS·Setup·대표 Prove·Verify에는 20% 초과 항목이 없어 세 번째 Setup run을 실행하지 않았습니다.

대표 Circuit 비용은 다음과 같습니다.

| Relation | Setup Run 1/2 ms | Prove Run 1/2 ms |
|---|---:|---:|
| Entry | 142.672 / 146.027 | 444.396 / 445.490 |
| Transfer | 296.916 / 295.929 | 849.750 / 869.254 |
| Merge | 296.713 / 302.574 | 870.211 / 865.707 |
| Process | 552.792 / 536.107 | 1,626.416 / 1,622.951 |
| Exit DPP | 270.358 / 278.791 | 825.978 / 829.556 |
| Issue Standard | 89.358 / 88.673 | 253.510 / 252.372 |
| Issue Strict | 88.085 / 88.956 | 251.621 / 250.122 |

나머지 relation의 두 값과 Verify·메모리 수치는 Circuit Raw에 있습니다.

## 4. 최종 Contract 구조는 어떻게 달라졌나요?

ZkDPPClaimLedger 하나가 모든 상태를 관리합니다. 별도 Router는 만들지 않았습니다.

constructor에는 고정 Event verifier 7개만 둡니다.

```text
Entry, Transfer, Proceed, Recall, Merge, Split, Exit
```

Process·Issue verifier는 PolicyRecord의 verifierRef로 선택합니다.

| Policy | Grant | verifier 선택 |
|---|---|---|
| Process 3-to-2 | 필요 | PolicyRecord.verifierRef |
| Issue Standard V1 | 사용하지 않음 | PolicyRecord.verifierRef |
| Issue Strict V2 | 사용하지 않음 | PolicyRecord.verifierRef |

M8에서 Process verifier를 constructor와 PolicyRecord에 함께 두던 중복을 제거했습니다. Event 함수의 proof statement·공개 입력·AuditRecord·상태 의미는 바꾸지 않았습니다.

final Ledger runtime은 18,161 B로 EIP-170 24,576 B 이하입니다. 배포 gas는 7,035,065였습니다.

## 5. 전체 시나리오에서는 무슨 일이 있었나요?

### 원자재와 전달

1. Aluminum A·B, Cathode, Anode를 Entry했습니다.
2. Aluminum A 첫 Voucher를 Freeze하자 Recall·Proceed가 거부됐습니다.
3. Unfreeze 후 Recall하고 다시 Transfer·Proceed했습니다.
4. 나머지 세 원자재도 Factory로 Transfer·Proceed했습니다.

### 제조

1. Factory가 Aluminum A·B를 Merge해 $(120,20,90)$을 만들었습니다.
2. Merge output·Cathode·Anode의 합은 $(320,30,230)$이었습니다.
3. Process input nf를 Freeze하자 같은 proof가 거부됐고, Unfreeze 후 Process가 성공했습니다.
4. Process는 ELIGIBLE $(270,30,260)$과 WASTE $(30,0,0)$을 만들었습니다.
5. ELIGIBLE output을 Product 1·2로 Split하고 State 합을 보존했습니다.

### DPP와 Claim

1. Product 1을 Exit해 DPP commitment를 만들었습니다.
2. Standard·Strict Claim을 등록했습니다.
3. WASTE도 DPP로 Exit했지만 Issue proof는 실패했습니다.
4. Standard Claim은 Active, Strict Claim은 Revoked로 끝났습니다.
5. Product 2는 미소비 Active Note로 남았습니다.

최종 Note leaf는 19개, Voucher leaf는 5개, AuditRecord는 21개입니다. 각 성공 Anvil run은 deployment·권한·Policy·Event·상태를 합쳐 51개 transaction을 기록했습니다.

## 6. Anvil 두 실행은 같았나요?

두 clean chain의 bytecode hash·transaction 순서·calldata·receipt gas가 모두 일치했습니다. Gas 불일치가 없어 추가 Anvil run은 수행하지 않았습니다.

대표 gas는 다음과 같습니다.

| 실행 | Gas | Calldata | SSTORE |
|---|---:|---:|---:|
| 첫 Entry | 2,258,465 | 1,412 B | 49 |
| 첫 Transfer | 3,596,789 | 1,636 B | 93 |
| Recall | 1,784,864 | 1,540 B | 51 |
| Merge | 1,849,682 | 1,572 B | 53 |
| Process | 2,933,492 | 1,764 B | 97 |
| Split | 2,722,918 | 1,572 B | 93 |
| Product Exit | 618,876 | 1,476 B | 11 |
| Standard Issue | 459,977 | 1,188 B | 4 |

한 Run의 transaction 준비 시간 합은 3,997.132 ms, receipt 대기 합은 2,869.456 ms였습니다. 두 번째 Run은 각각 4,041.426 ms와 2,893.262 ms였습니다.

Anvil은 final Run 1 proof를 사용하는 fixed-proof integration입니다. Participant의 Prove 시간은 SRS·Circuit 측정에서 별도로 기록했습니다. 두 구간을 직접 이어서 측정한 단일 사용자 wall-clock이라고 주장하지 않습니다.

## 7. Backward·Forward 감사는 어떻게 나왔나요?

Backward는 Standard Claim에서 네 Entry까지 이동했습니다.

```text
Claim → Issue → DPP → Exit → Split → Process
      → Merge·Proceed·Transfer·Recall → Entry 4건
```

Forward는 Aluminum A에서 시작해 zero Change·Recall·재Transfer 분기를 지나 Product 1 DPP·WASTE DPP와 Product 2 Note를 찾았습니다.

| Run | Backward ms | Forward ms | Backward 기록 | Forward 객체 | DPP terminal | Note leaf |
|---|---:|---:|---:|---:|---:|---:|
| 1 | 7,290.161 | 4,229.384 | 19 | 12 | 2 | 3 |
| 2 | 8,174.448 | 3,023.382 | 19 | 12 | 2 | 3 |
| 3 | 9,175.491 | 4,081.522 | 19 | 12 | 2 | 3 |

Run 1·2의 forward 시간이 20% 넘게 달라 명세대로 세 번째 감사 run을 실행했습니다. 대표 median은 backward 8,174.448 ms, forward 4,081.522 ms입니다.

각 backward run은 암호문 14개를 복호화하고 위원 응답 28개를 사용했습니다. DPP는 소비 nullifier가 없는 terminal로 처리했으며 미소비 Note leaf로 잘못 분류하지 않았습니다.

## 8. Correctness는 무엇을 확인했나요?

- final relation이 정확히 10개이고 public input 순서가 manifest와 일치했습니다.
- 같은 universal SRS checksum이 10개 Circuit manifest에 연결됐습니다.
- 네 Lagrange SRS 길이가 relation domain과 일치했습니다.
- Run 1·2의 전체 Setup·Prove·Verify가 성공했습니다.
- final Artifact를 다시 읽어 proof를 생성·검증했습니다.
- Process immutable verifier가 제거되고 PolicyRecord routing이 동작했습니다.
- Process는 Grant를 요구하고 Issue는 Grant 설정을 거부했습니다.
- Voucher·Note Freeze와 실패 transaction 원자성을 확인했습니다.
- 네 원자재 State, Merge·Process·Split 합이 일치했습니다.
- WASTE Issue와 중복 Claim을 거부했습니다.
- 전체 Go suite가 통과했습니다.
- Foundry는 기존 suite와 M9을 합쳐 37개 test가 모두 통과했습니다.
- M1~M8 보호 파일 checksum이 유지됐습니다.

## 9. 계획과 실제 구현이 달랐나요?

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Setup 반복 | 2회, 20% 초과 시 3회 | 2회로 종료 | 초과 항목이 없었습니다. |
| Anvil 반복 | 2회 | 2회, gas 완전 일치 | 추가 실행이 필요하지 않았습니다. |
| 감사 반복 | 2회, 20% 초과 시 3회 | forward 차이로 3회 | 세 값과 median을 기록했습니다. |
| E2E | Prove와 transaction 구간 분리 | fixed proof Anvil과 Circuit Prove를 별도 측정 | 단일 live wall-clock으로 해석하지 않습니다. |
| Protocol 의미 | M8 유지 | 변경 없음 | Process verifier routing만 중복 제거했습니다. |

첫 Anvil 시도는 fixture metadata가 decimal Field 문자열이어서 canonical hex decoder가 거부했습니다. metadata를 기존 32-byte lowercase hex 형식으로 고치고 fixture proof를 다시 생성했습니다. 실패 이력과 두 evaluate 실행을 checksum report에 남겼습니다.

## 10. Artifact와 재현 명령은 무엇인가요?

final Artifact는 다음 위치에 있습니다.

```text
artifacts/final/srs
artifacts/final/circuits/<relation>
artifacts/final/verifiers
```

각 Policy Circuit에는 canonical optimized VK와 SHA-256 vkHash가 있습니다. verifier checksum·runtime code hash·public-input manifest도 final Artifact에 포함됩니다.

재현 명령은 다음입니다.

```text
make setup-m9-final
make evaluate-m9
make benchmark-m9-setup
make test-go
make test-contract-m9
make benchmark-m9-anvil
make benchmark-m9-audit
make check-m9
```

Aggregate인 `make benchmark-m9`과 개별 benchmark를 중복 실행하지 않습니다. 공식 Artifact·Raw가 존재하면 덮어쓰기를 거부합니다.

`m9-generated-checksums.json`은 M9 생성물과 M1~M8 보호 파일을 대조합니다. final universal SRS는 구현·성능 확인용이며 Git에 포함하지 않습니다.

## 11. 무엇은 아직 보장하지 않나요?

- $2^{17}$보다 큰 미래 Circuit은 같은 SRS로 Setup할 수 없습니다.
- Production MPC Ceremony·Ethereum Ceremony SRS를 사용하지 않았습니다.
- Besu·QBFT·별도 Router를 구현하지 않았습니다.
- ProductProfile·운송 탄소·새 Process·Issue Policy를 추가하지 않았습니다.
- DPP ownership transfer·component DPP·중간 Claim을 구현하지 않았습니다.
- 감사 시간은 로컬 RPC의 작은 그래프 결과이며 Production throughput이 아닙니다.
- Anvil transaction 측정과 Participant proof 생성을 하나의 live 사용자 지연으로 측정하지 않았습니다.

M9는 **현재 확정된 zkDPP POC를 하나의 universal SRS·final PK/VK·Main Contract·전체 공급망 시나리오로 재현할 수 있음**을 확인한 최종 통합 단계입니다.
