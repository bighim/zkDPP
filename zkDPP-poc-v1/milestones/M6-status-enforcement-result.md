# M6 Status 집행 Result

- 상태: 완료
- 구현 명세: [`M6-status-enforcement.md`](M6-status-enforcement.md)
- Background: [`M6-status-enforcement-background.md`](M6-status-enforcement-background.md)
- Raw 결과: [`m6-circuit.json`](../output/m6-circuit.json), [`m6-anvil-gas.json`](../output/m6-anvil-gas.json), [`m6-anvil-e2e.json`](../output/m6-anvil-e2e.json)
- 실행 횟수: Setup 1회, 최종 gas clean chain 1회, 최종 E2E clean chain 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M6 결과 |
|---|---|
| 이전 상태 | M5는 private Event와 Policy가 있었지만 Frozen·Revoked 객체도 소비할 수 있었습니다. |
| 이번 목표 | private `cm`·`rv`를 공개하지 않고 현재 Active인 객체만 소비하게 합니다. |
| 실제 구현 | 두 오프체인 StatusTree, StatusUpdate ZKP, Status-aware Circuit 7개, `ZkDPPStatusLedger` |
| 이제 가능한 것 | Note·Voucher를 공개적으로 Freeze·Unfreeze·Revoke하고 private 소비에서 최신 상태를 집행할 수 있습니다. |
| 검증 | 전체 Go suite와 Foundry 18개 test 통과, 실제 StatusUpdate proof와 current-root binding 확인 |
| 대표 성능 | StatusUpdate 19,682 constraints·390,463 gas·약 563 ms E2E |
| 미포함 | Batch, Claim Status, HTTP Indexer, downstream 탐색, Besu·final SRS |
| 다음 단계 | M7에서 AuditRecord와 private provenance를 이용해 문제 객체의 이력을 복원합니다. |

## 1. 실제 구현한 큰 그림

```text
Note Tree index i
  ↔ NoteStatusTree index i

Voucher Tree index j
  ↔ VoucherStatusTree index j
```

Status leaf는 `Active=0`, `Frozen=1`, `Revoked=2`입니다. 전체 StatusTree와 path는 Go Indexer가 유지하고 Contract는 `noteStatusRoot`, `voucherStatusRoot`만 저장합니다.

정상 output은 StatusTree를 갱신하지 않습니다. Status Authority가 Status를 변경할 때만 private path로 StatusUpdate proof를 생성하고 root 하나를 갱신합니다.

## 2. M5에서 달라진 점

```text
M1~M5:
  private membership + nullifier

M6:
  private membership + 같은 index의 Active proof + nullifier
```

기존 `EntryExitLedger`와 Circuit은 building-block baseline으로 남겼습니다. 별도 `ZkDPPStatusLedger`와 Status-aware verifier가 M6 이후 Main Protocol 경로입니다.

## 3. 공개·비공개값

| 동작 | 공개 | 비공개 |
|---|---|---|
| 정상 소비 | membership root, 최신 Status root, nullifier, output | `cm`·`rv`, 기존 index, 두 Merkle path, 객체 내용 |
| Status 변경 | object type, `cm`·`rv`, index, old/new Status, old/new root | Status siblings 32개 |

어떤 객체가 Frozen·Revoked됐는지는 공개됩니다. 정상 소비 proof는 그 transaction이 과거 어느 `cm`·`rv`를 사용했는지 공개하지 않습니다.

## 4. 저장 상태와 전이

| 상태 | 역할 |
|---|---|
| immutable `statusAuthority` | 상태 변경 transaction을 승인합니다. |
| `noteStatusRoot` | Note 예외 상태를 요약하는 최신 root입니다. |
| `voucherStatusRoot` | Voucher 예외 상태를 요약하는 최신 root입니다. |

허용 전이:

```text
Active → Frozen
Frozen → Active
Frozen → Revoked
```

Batch는 없으며 transaction 하나가 leaf 하나만 변경합니다. `Revoked`는 terminal입니다.

## 5. Correctness 결과

- 기존 object index와 Status index를 공유하고 별도 index를 만들지 않았습니다.
- Note·Voucher StatusTree는 같은 numeric index에서도 독립적입니다.
- 실제 StatusUpdate PLONK proof로 empty root의 index 1을 Frozen root로 변경했습니다.
- Contract가 storage의 최신 root를 verifier public input으로 제공해 과거 Active proof를 거부했습니다.
- 잘못된 Authority, objectId-index, path·root와 금지 전이가 실패했습니다.
- Frozen 상태에서 Status-aware 소비가 실패하고 Unfreeze 후 성공했습니다.
- Revoked 후 상태 변경과 소비를 허용하지 않습니다.
- 전체 Go package test 통과, Foundry 18개 test 통과입니다.

## 6. Circuit 결과

| Circuit | Status path 수 | Constraints | Public | Prove | Verify | Solidity proof |
|---|---:|---:|---:|---:|---:|---:|
| StatusUpdate | old/new 2회 | 19,682 | 6 | 418.104 ms | 2.647 ms | 1,056 B |
| StatusPrivateSpend | 1 | 26,230 | 3 | 446.632 ms | 2.622 ms | 1,056 B |
| StatusTransfer | 1 | 45,211 | 7 | 775.573 ms | 2.604 ms | 1,056 B |
| StatusProceed | 1 | 32,644 | 4 | 474.542 ms | 2.700 ms | 1,056 B |
| StatusRecall | 1 | 35,952 | 5 | 774.872 ms | 2.626 ms | 1,056 B |
| StatusMerge | 2 | 57,850 | 5 | 807.267 ms | 2.711 ms | 1,056 B |
| StatusSplit | 1 | 44,223 | 5 | 804.541 ms | 2.760 ms | 1,056 B |
| StatusProcess | 3 | 98,591 | 9 | 1,552.113 ms | 2.597 ms | 1,056 B |

기존 관계에 Active path 하나를 추가할 때 constraints는 9,823개 증가했습니다. Merge는 두 개로 19,646개, Process는 세 개로 29,469개 증가했습니다. Proof 크기와 native verifier 시간은 거의 일정하지만 Prover가 추가 Merkle path 계산을 부담합니다.

단일 실행값이므로 서로 다른 milestone의 Prove 시간 차이를 안정적인 속도 향상·저하로 일반화하지 않습니다.

## 7. Event gas

| Transaction | Receipt gas | Calldata |
|---|---:|---:|
| Exit | 402,190 | 1,188 B |
| Transfer | 3,216,888 | 1,316 B |
| Proceed | 1,515,232 | 1,220 B |
| Recall | 1,516,323 | 1,252 B |
| Merge | 1,558,175 | 1,252 B |
| Split | 2,385,480 | 1,252 B |
| Process | 2,448,134 | 1,380 B |
| Active→Frozen | 390,463 | 1,316 B |
| Frozen→Active | 390,406 | 1,316 B |
| Frozen→Revoked | 390,543 | 1,316 B |

Status-aware Event는 기존 proof에 path를 합쳤으므로 Contract가 Poseidon2 32회를 직접 실행하지 않습니다. 기존 Event 대비 gas 증가는 약 3.5K~4.6K로, 주로 public Status root가 하나 늘어난 verifier 입력 비용입니다.

`ZkDPPStatusLedger` deployment는 6,098,976 gas입니다. StatusUpdate는 Status Authority가 old/new root 계산을 proof에 넣고 Contract는 verifier 실행 후 root SSTORE 하나만 수행합니다.

## 8. Live E2E

| Transaction | Prove | E2E |
|---|---:|---:|
| Exit | 437.063 ms | 595.515 ms |
| Transfer | 782.500 ms | 1,091.054 ms |
| Proceed | 477.088 ms | 681.697 ms |
| Recall | 758.606 ms | 982.610 ms |
| Merge | 804.264 ms | 1,011.022 ms |
| Split | 797.735 ms | 1,105.078 ms |
| Process | 1,510.800 ms | 1,803.116 ms |
| Active→Frozen | 411.959 ms | 562.738 ms |
| Frozen→Active | 412.669 ms | 562.498 ms |
| Frozen→Revoked | 415.484 ms | 566.177 ms |

Status Authority는 StatusUpdate proof 생성 약 412~415 ms를 부담합니다. 일반 Event에서는 input 수에 비례한 Status path가 Participant Prover 비용에 포함됩니다.

## 9. 측정 경계와 환경

- Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1
- PLONK-KZG/BLS12-381, Poseidon2, depth 32
- Foundry·Anvil 1.7.1, Prague, chain ID 31337, block gas limit 30M
- Gas와 live E2E는 서로 다른 clean chain에서 최종 1회 측정했습니다.
- E2E는 witness, prove, ABI·estimate·sign과 submit-to-receipt를 포함합니다.
- Setup·fixture 생성 중 발견한 runner 오류와 누락을 수정한 뒤 최종 clean run만 Raw 결과로 보존했습니다.

## 10. 계획과 실제 구현의 차이

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Main Contract | 별도 `ZkDPPStatusLedger` | 동일 | 없음 |
| Status leaf | 0·1·2 | 동일 | 없음 |
| Update 검증 | StatusUpdate ZKP | 동일 | 없음 |
| Process verifier | Status-aware Policy verifier만 사용 | immutable M6 verifier와 PolicyRecord 일치 강제 | legacy verifier 우회 방지 |
| Status root 초기화 | empty root | Go golden root를 Solidity constant로 고정 | 배포 시 불필요한 추가 Status Hash 제거 |
| Batch | 미지원 | 미지원 | 없음 |

Protocol 의미 변경은 없습니다.

## 11. 재현 명령

```text
make setup-m6
make test-go
make test-contract-m6
make benchmark-m6-gas
make benchmark-m6-e2e
```

## 12. Artifact와 checksum

M6 Circuit별 development CCS·SRS·PK·VK, generated Solidity verifier와 fixed proof fixture를 생성했습니다. 재생성 가능한 key·proof는 Git 대상이 아닙니다. [`m6-generated-checksums.json`](../output/m6-generated-checksums.json)에 generated 파일과 M1~M5 raw checksum을 기록했으며 기존 결과 checksum은 모두 유지됐습니다.

## 13. 제외·한계

- Go Indexer는 in-memory Tree이며 HTTP·DB·reorg 복구가 없습니다.
- Indexer의 거짓 path는 proof 실패로 막지만 availability는 보장하지 않습니다.
- Status Authority가 live 객체만 선택하는 운영 가정은 Contract가 강제하지 않습니다.
- Status 변경 대상과 timing metadata는 공개됩니다.
- Claim Status와 downstream live 객체 탐색은 구현하지 않았습니다.
- Development SRS이며 Production setup이 아닙니다.
- 공식 성능은 단일 실행값입니다.

## 14. M7에 전달하는 기능

M7은 immutable Status Authority, 공개 `StatusChanged`, private Active proof와 두 Status root를 이어받습니다. AuditRecord·encrypted parents·`producerOf`를 추가해 문제 객체의 provenance를 복원하고, 복원 결과로 live downstream 객체를 순차 Freeze하는 orchestration을 설계해야 합니다.
