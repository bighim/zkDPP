# Transfer Audit 유무의 항목별 Gas 대조

기존 실험 3의 공식 run 1, 첫 Transfer Proof와 선행 Entry 두 Proof를 재실행했습니다.
No-Audit receipt는 기존 대표값과 같은 **3,221,268 Gas**입니다.
기존 Audit 측정은 실험 1의 공식 run 2이며 **3,577,414 Gas**입니다.
두 값 모두 각 실험의 median에 해당하는 실제 transaction입니다.

| 동일한 분류 기준 | Audit 없음 | Audit 있음 | 차이 |
|---|---:|---:|---:|
| Tree Hash 호출 | 1,422,404 | 1,422,404 | 0 |
| Tree SLOAD·SSTORE | 1,276,500 | 1,276,500 | 0 |
| Proof 검증 호출 | 335,007 | 339,592 | 4,585 |
| AuditRecord SLOAD·SSTORE | 0 | 294,700 | 294,700 |
| 객체 등록·소비·생성 연결 SLOAD·SSTORE | 68,700 | 112,800 | 44,100 |
| calldata 등 나머지 비용 | 118,657 | 131,418 | 12,761 |
| **합계** | **3,221,268** | **3,577,414** | **356,146** |

## 보고서 재분류

AuditRecord 자체 비용 294,700에 Proof 차이 4,585, mapping 차이 44,100, 기타 차이 12,761을 합쳐 **Audit 추가 비용 356,146**으로 표시합니다.
다른 행에는 Audit 없는 실행의 값을 사용하므로 중복 집계하지 않습니다.
No-Audit에는 producerOf가 없으므로 기본 mapping 행에는 생성 연결 저장을 포함하지 않습니다. 해당 비용은 Audit 추가 비용으로 이동합니다.

앞선 44,200은 mapping **SSTORE만**의 차이였습니다.
SLOAD까지 합치면 차이는 **44,100**입니다. No-Audit의 boolean 소비 표시에는 warm SLOAD가 한 번 더 있습니다.
이로써 이전에 배분하지 못한 차액을 임의로 기타 항목에서 빼지 않고 실제 trace로 분리했습니다.

## 검증·한계

- No-Audit의 75개 SSTORE와 144개 SLOAD를 모두 실제 stack의 슬롯으로 분류했습니다. 미분류 접근이 있으면 실패합니다.
- Tree SLOAD는 131회, SSTORE는 72회로 Audit 실행과 같습니다.
- Proof 호출 1회와 Poseidon2 호출 64회를 확인했습니다.
- 하위 호출은 Ledger 호출 직전·직후 gas 차이로 포함하고 별도로 더하지 않습니다.
- 기본·calldata 비용을 포함한 합계가 실제 receipt와 일치하며 환급은 0입니다.
- 배포 runtime code hash는 기존 실험 3과 일치하고 기존 Audit 원자료 checksum은 유지됩니다.
- 두 대표 run의 State·Tree 형태는 같지만 opening·commitment·Proof는 다릅니다. 따라서 차이에는 calldata byte 차이도 포함되며, 동일 byte 입력의 순수 암호 연산 비용이라고 주장하지 않습니다.
- 이번 측정은 초기화 후 Entry 2건·Transfer 1건의 단일 재실행입니다. 새 Proof·SRS·PK/VK는 생성하지 않았습니다.
- 전용 project `zkdpp-transfer-gas-plain`, Anvil 1.7.1, Prague, 31337, gas limit 30M, port 18547을 사용한 뒤 해당 project만 종료했습니다.

`comparison.json`은 최종 항목별 비교, 나머지 JSON은 receipt·호출·opcode 원자료입니다.
`checksums.json`에 원자료·재사용 Proof·Contract·분류 코드 checksum을 보존했습니다.
재현: `measure_transfer_gas -plain`, `trace_transfer_root output/paper-supplement/transfer-gas-attribution-plain`, `compare_transfer_gas`.
기존 출력 경로가 있으면 덮어쓰기를 거부합니다.
