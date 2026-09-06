# M7 — 감사 기록을 따라가 실제로 동결했나요?

- 상태: 완료
- 구현 기준: [M7 명세](M7-audit-tracing.md)
- Raw: [Circuit](../output/m7-circuit.json), [gas](../output/m7-anvil-gas.json), [감사 추적](../output/m7-audit.json), [checksum·실행 이력](../output/m7-generated-checksums.json)
- 실행 횟수: Setup·evaluate·gas 각 1회입니다. 원장 크기 개선 후 Foundry 전체 suite를, 추적 결과 보완 후 전체 Go suite와 감사 측정을 각각 2회 실행했습니다. 첫 감사 측정은 별도 보존했습니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

**이제 암호문 한 건을 복원하는 것에서 끝나지 않습니다. 실제 원장의 여러 기록을 따라 미소비 Note를 찾고, 그 Note의 사용을 막았습니다.**

| 질문 | M7 결과 |
|---|---|
| 이전 상태 | B1은 공통 암호화와 대표 Process 복원을 검증했지만 실제 원장·추적·동결에는 연결하지 않았습니다. |
| 이번 목표 | 감사 기록을 모든 대상 Event에 연결하고 부모·자손을 추적해 선정한 대상을 동결하는 것입니다. |
| 실제 구현 | 감사 Event 8개, ZkDPPAuditLedger, AuditRecord·생성/소비 mapping, 양방향 감사 프로그램, nf 상태 집행입니다. |
| 이제 가능한 것 | Entry→Split→Split에서 A의 자손 C·D·E를 찾고 Freeze합니다. D에서 부모를 따라 A의 Entry도 찾습니다. |
| 검증 | 전체 Go suite 통과, Foundry 33개 test 통과, 실제 기록 18개의 원문·Tree·mapping 일치입니다. |
| 대표 성능 | 대표 그래프 backward 약 0.975초, forward 약 2.150초입니다. Note Freeze는 약 49.5K gas입니다. |
| 미포함 | Claim·DPP, 자동 자손 동결, 영구 감사 캐시, 위원 서버, DKG·키 회전입니다. |
| 다음 단계 | M8 Claim·DPP 연결을 설계합니다. 운영용 감사 서비스나 대규모 최적화는 별도입니다. |

**감사 시간의 대부분은 원본 조회였습니다.** 아래 시간은 로컬 RPC를 포함한 작은 그래프의 단일 측정값이며, 복호화 연산만의 시간이나 대규모 공급망 성능은 아닙니다.

## 1. B1에서 무엇이 달라졌나요?

| B1 | M7 |
|---|---|
| Process 한 종류에 암호화 검사를 연결했습니다. | Entry·Exit·Transfer·Proceed·Recall·Merge·Split·Process에 연결했습니다. |
| 실험용 Contract에 공개 입력 15개를 저장했습니다. | 실제 원장에 필요한 AuditRecord 범위를 저장합니다. |
| 암호문 한 건의 원문 복원을 확인했습니다. | 생성·소비 mapping으로 여러 기록을 따라갑니다. |
| 기존 M6 StatusTree는 그대로였습니다. | 새 Main Contract는 공개 nf·rvnf로 상태를 직접 조회합니다. |

기존 M6는 보존했습니다. 현재 새 경로는 **ZkDPPAuditLedger**이며 membership Tree는 유지하고 StatusTree·Active path·StatusUpdate proof만 사용하지 않습니다.

## 2. 대표 예시에서 무슨 일이 일어났나요?

**한 Participant가 만든 Note를 두 번 Split한 뒤, 실제 원장 기록으로 연결관계를 복원했습니다.**

$$
\varnothing \xrightarrow{\mathrm{Entry}} cm_A
$$

$$
cm_A \xrightarrow{\mathrm{Split}} (cm_B,cm_C)
$$

$$
cm_B \xrightarrow{\mathrm{Split}} (cm_D,cm_E)
$$

1. A를 Entry하고 B·C, D·E 순서로 생성했습니다. 감사 기록 3개와 Note leaf 5개가 생겼습니다.
2. **Backward:** D의 생성 기록에서 B를 복원하고, 다시 A의 Entry까지 찾았습니다.
3. **Forward:** A의 소비 nf로 첫 Split을 찾고, B의 소비 nf로 다음 Split을 찾았습니다. 미소비 leaf는 C·D·E였습니다.
4. 세 leaf의 nf를 Freeze하자 해당 Note들의 소비가 실패했습니다.
5. C는 Unfreeze 후 Exit가 성공했습니다. E는 Revoke 후 계속 소비가 거부됐고 D는 Frozen으로 남았습니다.

마지막에는 Note leaf 5개, AuditRecord 4개, 소비된 Note nf 3개(A·B·C)가 남았습니다. C를 다시 Freeze하려 하면 이미 소비됐기 때문에 거부했습니다.

감사 결과는 **방문 객체·기록 ID·복원한 연결관계·미소비 leaf·종료 경로**를 반환합니다. Backward는 방문한 생성 기록의 공개 출력도 함께 알 수 있어 A→B·C와 B→D·E의 네 연결을 반환합니다. 실제로 따라간 ancestor 객체는 D·B·A의 3개입니다.

### Voucher·Merge·Process도 연결됐나요?

| 사례 | 실제 확인 |
|---|---|
| Transfer·Proceed | Voucher의 rvnf로 Receiver Note까지 이어졌습니다. |
| Transfer·Recall | 같은 방식으로 Sender의 반환 Note까지 이어졌습니다. 전량 전달의 zero Change도 남았습니다. |
| Voucher 동결 | Frozen이면 Proceed·Recall 모두 거부됐고, 해제 후 성공한 한 분기만 rvnf를 소비했습니다. |
| Merge·Split | 두 부모의 Entry를 복원하고 합쳐진 뒤 분기한 출력을 중복 없이 따라갔습니다. |
| Process | 부모 Note 3개와 ELIGIBLE·WASTE 출력 2개의 관계를 복원했습니다. |
| Exit | 다음 출력이 없는 소비 기록을 미소비 leaf가 아니라 종료 경로로 구분했습니다. |

## 3. 누가 무엇을 검증하고 공개하나요?

| 주체 | 수행한 일 |
|---|---|
| Participant | 실제 부모·출력 소비값을 계산해 암호화하고 Event proof를 만듭니다. |
| Circuit | 기존 Event 규칙과 올바른 감사 정보가 같은 암호문에 들어 있음을 함께 확인합니다. |
| Contract | 권한·현재 소비/상태·proof를 확인하고 원장과 AuditRecord를 함께 갱신합니다. |
| 위원 두 명 | 같은 원본 기록을 독립 조회하고 partial decryption을 제공합니다. |
| Auditor | 응답을 결합하고 부모·자손을 탐색합니다. 자동으로 동결 transaction을 보내지는 않습니다. |

공개되는 값은 기존 Event 공개 입력과 공개점·암호문입니다. 실제 부모 cm·rv, 출력이 나중에 사용할 평문 nf·rvnf, owner secret·path·암호화 난수는 private witness입니다.

Entry는 승인받은 Participant가 자기 secret으로 자기 Note를 만듭니다. 관리자는 권한만 부여하며 secret을 받지 않습니다. EVM 제출자와 ZK owner를 연결하는 별도 검사는 없습니다. 실험 transaction 제출은 account 1을 사용했고, Proceed의 실제 ZK 소유권은 Receiver secret으로 검증했습니다.

## 4. AuditRecord에는 무엇을 저장했나요?

**암호문과 출력 참조는 storage에 남겼지만, public input 전체를 복사하지는 않았습니다.**

| 저장한 내용 | 용도 |
|---|---|
| eventKind·policyRef | 기록의 종류와 관련 Process Policy입니다. |
| outputRefs | 만든 cm·rv와 대응 순서입니다. |
| 공개점 좌표 2개 | 위원이 복호화할 때 필요합니다. |
| encryptedParents | 실제 부모 참조를 암호화한 부분입니다. |
| encryptedOutputNfs | outputRefs에 대응하는 소비 nf·rvnf를 암호화한 부분입니다. |

부모와 출력 소비값은 **키 하나로 암호화한 벡터의 두 부분**입니다. 기록 ID는 mapping key이며 내부에 다시 저장하지 않습니다.

noteRoot·policyScopeRef·입력 nf·proof 등을 전부 복사하지 않습니다. AuditRecorded 로그로 생성 transaction을 찾아 공개 입력으로 문맥을 재계산하고, 종류·출력·암호문을 storage와 비교했습니다. **암호문을 calldata에만 둔 것은 아닙니다.**

### 생성·소비·동결 상태는 어디에 있나요?

| 상태 | 답하는 질문 |
|---|---|
| producerOf[종류][cm 또는 rv] | 어느 AuditRecord에서 생성됐나요? |
| noteSpentIn[nf] | 어느 기록에서 Note가 소비됐나요? |
| voucherSpentIn[rvnf] | 어느 기록에서 Voucher가 해결됐나요? |
| noteStatusByNf·voucherStatusByNf | 지금 사용할 수 있나요? |

소비 mapping의 값이 0이면 소비 기록 없음입니다. 별도 spent boolean은 중복 저장하지 않으며 기존 boolean 조회는 이 값으로 계산합니다.

Event가 실패하면 소비 ID·출력·Tree·AuditRecord가 함께 되돌아갑니다. Status 변경은 해당 상태만 바꾸며 과거 소비나 감사 기록을 삭제하지 않습니다.

## 5. 계획과 실제 구현이 달랐나요?

| 항목 | 실제 처리 | 영향 |
|---|---|---|
| 초기 Contract 크기 | 첫 runtime이 27,429 B로 EVM 한도를 넘었습니다. 내부 Event 식별·기록 처리를 공통화해 16,399 B로 줄였습니다. | 공개 API·검증 의미를 바꾸지 않고 실제 배포 한도를 통과했습니다. |
| 최적화 설정 | 코드 크기용 optimizer 설정도 점검했지만, 최종 구현은 기존 optimizer_runs=200·viaIR를 사용했습니다. | 제한을 풀어 배포하지 않았습니다. |
| 추적 결과 | 첫 버전은 leaf·통계만 반환했습니다. 최종 점검에서 방문 객체·기록·연결관계도 반환하도록 보완했습니다. | 전체 Go suite·감사 측정을 다시 실행했습니다. 첫 감사 Raw는 보존했습니다. |
| EVM 서명 | 로컬 Anvil의 unlocked account로 제출했습니다. | receipt gas·원본 조회 실험이며 별도의 로컬 서명 성능 측정은 아닙니다. |
| Process Setup | B1 CCS·PK·VK를 재사용했습니다. | M7의 새 Setup 비용으로 기록하지 않았습니다. |
| Foundry 회귀 | 첫 suite 뒤 원장 bytecode를 공통화했습니다. | 최종 소스 기준 전체 33개 test를 다시 실행했고 두 번 모두 통과했습니다. |

Setup·evaluate·gas는 각각 1회입니다. Foundry·전체 Go suite·감사 측정은 각각 2회이며 반복 이유를 위 표와 실행 기록에 남겼습니다. 두 감사 결과를 평균 내지 않았으며 최신 Raw는 연결관계 반환까지 포함한 두 번째 측정입니다.

## 6. 어떤 correctness를 확인했나요?

- 8개 Event의 일반 계산·Circuit 공개 입력·storage 암호문이 일치했습니다.
- 엉뚱한 부모·출력 소비값을 정상적으로 암호화해도 Event Circuit은 거부했습니다.
- 잘못된 공개점·난수·길이·selector·원본·snapshot을 정상 감사로 취급하지 않았습니다.
- 모든 Note 소비 경로가 Frozen 입력을 거부하고 Voucher의 두 해결 경로도 동결을 확인했습니다.
- 중복 소비·출력·잘못된 proof·권한·PolicyGrant와 금지 상태 전이를 거부했습니다.
- 실제 원장 기록 18개의 복호화 원문이 고정 테스트 데이터와 일치했습니다.
- 네 원장의 최종 root·leaf count·모든 현재 path를 Go 계산과 대조했습니다.
- 최종 전체 Go suite가 통과했고, Foundry는 기존 25개와 M7 8개를 합쳐 **33개 모두 통과**했습니다.

권한 있는 Authority가 임의 nf를 제출했을 때 그것이 실제 감사 대상에 대응하는지 증명하는 기능은 없습니다. 정직한 Authority가 대상을 선정한다는 가정입니다. 비권한 호출과 이미 소비된 대상은 Contract가 거부합니다.

## 7. 비용은 무엇을 포함하나요?

| 관점 | 포함 | 포함하지 않음 |
|---|---|---|
| Participant | 실제 암호화·witness·Prove·Native Verify·메모리·proof 크기 | 운영용 네트워크와 전체 사용자 E2E |
| Contract | 실제 membership Tree·AuditRecord·조회 mapping·상태 변경 | 별도 StatusTree나 StatusUpdate proof |
| 감사 | 로컬 RPC 원본 조회·검사, 위원 연산, 결합·복원, 탐색·메모리 캐시 | 원장 준비·proof 생성·외부 위원 네트워크 |

환경은 Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, macOS arm64, GOMAXPROCS=8입니다. 권한 있는 환경 조회에서 CPU는 Apple M1 Pro이며, sandbox 안의 일부 Raw는 arm64로만 기록했습니다.

Foundry·Anvil 1.7.1, Solidity 0.8.30, Prague, chain ID 31337, 30M gas limit입니다. Gas와 감사는 별도 clean chain에서 수행하고 zkdpp-m7 컨테이너만 종료했습니다.

## 8. 실제 성능은 어느 정도인가요?

### Participant의 증명 비용은 얼마인가요?

**암호화 자체는 약 0.3 ms이고, Event 전체를 증명하는 비용이 더 큽니다.**

| Event | Constraints | 공개 입력 | Prove ms | Native Verify ms |
|---|---:|---:|---:|---:|
| Entry | 19,020 | 4 | 441.307 | 2.754 |
| Transfer | 53,836 | 11 | 868.060 | 2.618 |
| Proceed | 39,766 | 7 | 839.708 | 2.631 |
| Recall | 43,375 | 8 | 856.954 | 2.680 |
| Merge | 57,551 | 9 | 859.141 | 2.612 |
| Split | 52,246 | 9 | 849.443 | 2.643 |
| Process | 93,578 | 15 | 1,654.467 | 2.769 |
| Exit | 31,247 | 5 | 476.159 | 2.694 |

각 행은 대표 case 한 번입니다. 연결·실패 검사까지 총 20개의 proof를 생성했습니다: Entry 8, Transfer 2, Proceed 1, Recall 1, Merge 1, Split 3, Process 1, Exit 3입니다.

모든 proof는 binary 664 B, Solidity 형식 1,056 B입니다. 기존 membership path는 Event의 입력 수만큼 포함되지만 Status path는 없습니다. 상세 Setup·SRS·key 크기와 Prove 할당량·heap snapshot은 [Circuit Raw](../output/m7-circuit.json)에 있습니다. 할당량은 peak RSS가 아닙니다.

### 원장에 기록하는 gas는 얼마인가요?

**아래는 proof 검증만이 아니라 실제 Tree·감사 기록·mapping 변경을 포함한 비용입니다.**

| 실행한 case | Receipt gas | SSTORE 횟수 |
|---|---:|---:|
| 첫 Entry — A | 2,258,105 | 49 |
| 후속 Entry — Voucher 시나리오 두 번째 입력 | 1,691,779 | 49 |
| Split — A→B·C | 2,724,580 | 93 |
| Split — B→D·E | 2,748,434 | 93 |
| 부분 Transfer — 첫 Voucher append | 3,579,007 | 93 |
| 전량 Transfer — 두 번째 Voucher append | 3,029,697 | 93 |
| Proceed | 1,783,564 | 51 |
| Recall | 1,784,371 | 51 |
| Merge | 1,853,348 | 53 |
| Process | 2,884,967 | 97 |
| Exit — C | 533,631 | 9 |

같은 SSTORE 횟수라도 새 slot인지 이미 쓰던 slot인지에 따라 gas가 다릅니다. 첫 Entry·Voucher append를 후속 실행과 구분해야 합니다.

SSTORE에는 다음이 포함됩니다.

- **Tree:** leaf·중간 node·root·count·accepted root입니다.
- **감사 기록:** 종류·Policy·공개점·두 암호문 배열·outputRefs와 배열 길이입니다.
- **조회·소비:** commitment 등록·producerOf·spentIn·다음 기록 ID입니다.

이 분류는 저장 내용의 설명이며 각 항목의 격리된 gas 실험은 아닙니다. Raw의 SSTORE opcode 합은 전체 trace에서 측정한 값입니다. 이를 receipt gas 전체나 순수 암호문 저장비라고 부르지 않습니다.

| 상태 변경 사례 | Receipt gas | SSTORE |
|---|---:|---:|
| Note Active→Frozen | 49,474 | 1회 |
| Note Frozen→Active | 27,617 | 1회 |
| Note Frozen→Revoked | 32,455 | 1회 |

각 상태 변경은 proof 없이 Authority 권한과 현재 소비·상태를 확인합니다. 같은 상태 전이라도 calldata 내용 등에 따라 소폭 달라집니다.

새 Ledger 배포는 **6,656,941 gas**, runtime은 **16,399 B**입니다. Poseidon2 배포는 2,282,769 gas, 각 verifier 배포는 약 1.602M gas입니다. 원장 4개는 독립 시나리오 구성용이며 실제 Protocol이 반드시 원장 4개를 요구하는 것은 아닙니다.

### 부모·자손 추적은 얼마나 걸렸나요?

**대표 그래프에서는 복호화 계산보다 검증된 원본을 읽는 비용이 대부분이었습니다.**

| 방향 | 전체 ms | 원본 조회·검사 ms | 위원 1 ms | 위원 2 ms | 결합·복원 ms | 나머지 탐색 ms |
|---|---:|---:|---:|---:|---:|---:|
| D→B→A Backward | 974.988 | 966.826 | 1.183 | 1.314 | 3.117 | 2.549 |
| A→C·D·E Forward | 2,149.579 | 2,137.037 | 1.876 | 1.838 | 4.374 | 4.455 |

| 방향 | 방문 객체 | 고유 기록 | 실제 복호화 | 위원 응답 | 캐시 재사용 | RPC 요청 |
|---|---:|---:|---:|---:|---:|---:|
| Backward | 3 | 3 | 2 | 4 | 0 | 46 |
| Forward | 5 | 3 | 3 | 6 | 2 | 81 |

Backward는 Entry에 부모가 없으므로 그 기록을 추가 복호화하지 않았습니다. Forward는 C·E가 같은 생성 기록을 다시 만나므로 메모리의 원문을 재사용했습니다.

각 위원이 원본을 독립 조회하고, Auditor도 기록·로그·transaction·receipt·block을 확인했습니다. 이 POC는 원본 조회를 합치거나 영구 캐시로 최적화하지 않았습니다. 조회 시간에는 RPC 대기뿐 아니라 decoding·일치 검사가 포함됩니다.

보조 감사도 같은 방식으로 검증했습니다.

| 시나리오 | Forward ms | Backward ms | 확인한 내용 |
|---|---:|---:|---|
| Voucher | 1,889.981 | 1,218.824 | Proceed의 Receiver 출력과 Recall의 Sender 반환 경로 |
| Merge·Split | 2,312.684 | 1,550.293 | 합쳐진 뒤 분기한 출력과 두 Entry 부모 |
| Process | 1,758.602 | 1,316.376 | 두 출력과 세 Entry 부모 |

C의 Exit 이후 A에서 다시 조회한 경우에는 미소비 leaf 2개와 종료 경로 1개를 반환했습니다. 이 별도 case의 전체 시간은 1,680.556 ms입니다.

모든 숫자는 해당 case의 단일 측정입니다. 다른 실행 간 시간 차이를 안정적인 성능 비율로 주장하지 않으며 B1의 오프라인 복호화 처리량과 직접 비교하지 않습니다.

## 9. 어떻게 다시 실행하나요?

기존 B1 위원회 설정·Process Artifact와 과거 Foundry 회귀용 생성물을 준비한 실험 사본에서 다음 순서입니다.

1. make setup-m7 — 새 7개 회로의 개발 keys와 verifier를 준비합니다.
2. make evaluate-m7 — 대표 성능과 연결용 proof를 생성합니다.
3. GOFLAGS=-count=1 GOMAXPROCS=8 make test-go
4. make test-contract-m7
5. make benchmark-m7-gas
6. make benchmark-m7-audit
7. make check-m7

Aggregate인 make benchmark-m7과 개별 benchmark를 중복 실행하지 않습니다. 공식 Raw가 있으면 덮어쓰기를 거부합니다. 새 실험은 기존 결과·Artifact를 보존한 별도 사본과 실행 전 baseline.json이 필요합니다.

독립 감사는 go run ./cmd/audit_m7에 RPC·배포 manifest·snapshot block·방향·시작 종류·cm/rv를 전달합니다. 명령은 승인된 감사 결과를 반환할 뿐 자동 동결하거나 원문 캐시를 파일로 저장하지 않습니다.

## 10. Artifact와 보존 결과는 어디에 있나요?

artifacts/development/m7에는 새 회로 keys·binding manifest·연결 proof·배포 manifest·테스트 결과·실행 이력이 있습니다. 위원회 shares와 Process keys는 B1 것을 다시 생성하지 않고 사용했습니다.

기존 코드·Artifact·Raw·과거 명세를 포함한 **338개 보호 파일**의 checksum이 변하지 않았습니다. 네 Raw JSON은 output에 보존하며 key·proof·generated verifier·fixture는 Git에서 제외합니다.

첫 감사 측정은 runs/audit-01-superseded.json, 보완 이유는 같은 폴더의 설명 JSON에 보존했습니다. Setup·evaluate·gas는 첫 시도에 성공했고 감사 측정은 두 결과를 구분해 남겼습니다. [checksum JSON](../output/m7-generated-checksums.json)이 실제 파일과 시도 이력을 연결합니다.

## 11. 어떤 한계가 있나요?

- 위원회·Auditor·Setup 생성자를 신뢰하며 검증된 원본만 복호화합니다.
- 암호문은 원 TDH2 그대로가 아니고 악의적인 위원 응답 증명은 없습니다.
- 감사 프로그램은 직접 Ledger를 호출한 transaction만 지원합니다. 내부 multicall·과거 상태를 제공하지 않는 RPC·reorg 복구는 지원하지 않고 오류로 보고합니다.
- Snapshot의 leaf가 동결 전에 소비될 수 있습니다. 현재 소비를 다시 확인해 거부하며 자동 자손 재추적은 하지 않습니다.
- 공개된 동결 nf와 이후 소비는 연결할 수 있습니다. Auditor가 이미 복원한 값을 잊도록 강제하지 않습니다.
- Production secret 관리·DKG·키 회전·최종 universal SRS가 아닙니다.
- 작은 로컬 그래프의 기능·정확성·비용 확인이며 대규모 감사 서비스나 네트워크 분산 성능 결과가 아닙니다.

## 12. 다음 단계는 무엇을 이어받나요?

**실제 원장에서 쓰는 감사 기록·생성/소비 조회·양방향 추적·nf 상태 집행이 준비됐습니다.**

M8에서는 Claim·DPP를 이 구조와 연결하는 별도 명세를 작성합니다. M7이 Claim 구현이나 운영용 감사 플랫폼까지 완료한 것은 아닙니다.
