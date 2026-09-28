# Transfer Gas 분리 측정

**동일한 기존 Proof를 재실행한 실제 receipt: 3,577,414 Gas.**
기존 실험 1의 공식 run 2, `exp1-lifecycle-03` Proof를 재사용했습니다.
선행 Entry 두 건도 같은 run의 Proof로 재실행했습니다.
모든 배포 runtime code hash는 기존 실험과 일치합니다.
새 Proof·SRS·PK·VK는 생성하지 않았고 기존 결과를 덮어쓰지 않았습니다.

| 항목 | Gas |
|---|---:|
| Merkle Tree Poseidon2 호출 | 1,422,404 |
| Merkle Tree 저장 | 1,001,400 |
| Proof verifier 호출 | 339,592 |
| AuditRecord 저장 | 279,900 |
| 객체 등록·소비·생성 연결 저장 | 104,200 |
| 기타 Ledger 실행 | 386,206 |
| transaction 기본·calldata 비용 | 43,712 |
| **합계** | **3,577,414** |

## 분류 기준

- 호출 비용: Ledger의 STATICCALL 직전부터 반환 직후까지 gas 차이. 호출 opcode 부대 비용과 모든 하위 실행을 포함합니다.
- Poseidon2: 두 출력의 Tree 갱신에 64회 호출합니다. Proof 내부 연산과 겹치지 않습니다.
- Tree 저장: 72회 SSTORE. 경로 노드·root·leafCount·허용 root 기록입니다.
- AuditRecord 저장: 16회 SSTORE. 기록 본문·배열·nextAuditRecordId 갱신입니다.
- 객체 기록 저장: 5회 SSTORE. commitments·voucherCommitments·noteSpentIn·producerOf입니다.
- 기타 Ledger 실행: ABI 처리, 검사, SLOAD, 루프, 이벤트 로그 등 위 범주에 속하지 않은 실행입니다. Tree의 루프 제어 등도 포함되므로 Hash·저장 소계가 전체 Tree 함수 비용과 같지는 않습니다.
- 기본·calldata: 21,000 + zero byte당 4 + nonzero byte당 16입니다. 이 transaction은 계산 결과와 receipt가 일치하며 추가 floor 조정·환급이 없습니다.

Gas 잔량의 차이로 Ledger 최상위 실행을 분리하므로 하위 호출을 이중 집계하지 않습니다.
93개 SSTORE를 모두 분류했고 미분류 저장은 0개입니다.
저장 슬롯은 실제 Solidity 선언 순서와 mapping·배열 배치 규칙으로 도출했습니다.
도출한 키를 관측된 모든 SSTORE의 stack과 대조했습니다.
고정 verifier 7 slots, Note Tree slot 7, Voucher Tree slot 44, AuditRecord mapping slot 91입니다.

**AuditRecord 저장 279,900 Gas는 Audit 유무의 전체 차이 356,146 Gas와 다릅니다.**
Audit은 저장 외에 calldata·public input 검증·다른 mapping 저장 등도 바꾸기 때문입니다.
이 결과는 한 transaction의 분해이며 다른 Tree 위치·출력 수에 그대로 적용하지 않습니다.

## 실행과 원자료

- Anvil 1.7.1 / Prague / chain ID 31337 / gas limit 30M / port 18547
- 전용 Compose project `zkdpp-transfer-gas-attribution`, 완료 후 이 project만 종료
- `replay.json`: 기존 측정 참조, 실제 receipt·calldata·배포·선행 실행
- `calls.json`: callTracer 결과
- `opcodes.json`: 전체 opcode trace, stack·memory·storage 제외
- `root-opcodes.json`: stack을 포함한 동일 transaction의 Ledger 단계 trace
- `breakdown.json`: 항목 합계와 각 SSTORE의 slot·Gas 분류
- `checksums.json`: 원자료·기존 Proof·Contract Artifact·source SHA-256

동작 재실행은 1회입니다. Trace 조회는 저장 없이 1회, 저장 stack 포함 1회 및 callTracer 1회입니다.
측정 시간이나 E2E는 분석 대상이 아닙니다.

보조 storage-layout 추출 시도는 세 번 실패했습니다: offline solc 부재, 잘못된 source 경로, Foundry StripPrefixError panic.
이 실패들은 transaction 재실행 횟수에 포함되지 않습니다.
최적화된 source map이 SSTORE를 충분히 구분하지 못해 최종 분류는 검증한 slot 기준을 사용했습니다.

재현 코드: `cmd/measure_transfer_gas`, `cmd/trace_transfer_root`, `cmd/analyze_transfer_gas`.
원자료 출력 경로가 이미 있으면 재실행을 거부합니다.
