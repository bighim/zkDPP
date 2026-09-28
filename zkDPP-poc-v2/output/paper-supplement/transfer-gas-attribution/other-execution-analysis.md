# 나머지 Contract 실행 비용 분석

기존 transaction의 저장된 root trace만 분석했습니다. 새 transaction·Proof 생성은 없습니다.
기존 분석에서 제외했던 SSTORE와 STATICCALL을 다시 제외한 합계가 정확히 386,206 Gas인지 검사했습니다.

| opcode 범주 | Gas |
|---|---:|
| SLOAD | 302,700 |
| PUSH·DUP·SWAP·POP | 34,489 |
| JUMP·JUMPI·JUMPDEST | 22,648 |
| 산술·비교·bit 연산 | 11,577 |
| KECCAK256 | 6,234 |
| Event LOG | 4,774 |
| 메모리·데이터 복사 | 3,379 |
| calldata 읽기·실행 환경 조회 등 | 405 |
| **합계** | **386,206** |

## SLOAD의 대상

| 대상 | 횟수 | Gas |
|---|---:|---:|
| Tree 형제 노드 | 64 | 134,400 |
| 빈 형제 위치의 기본 노드 | 63 | 132,300 |
| Tree leafCount | 2 | 4,200 |
| 새 root의 허용 목록 슬롯 | 2 | 4,200 |
| **Tree 갱신 관련 읽기 소계** | **131** | **275,100** |
| 입력 root 허용 여부 | 1 | 2,100 |
| 입력 소비 여부 | 1 | 2,100 |
| 입력 Status | 1 | 2,100 |
| 출력 등록 여부 | 4 | 4,400 |
| verifier 주소 | 1 | 2,100 |
| Audit 기록 counter·헤더·outputRefs 슬롯 | 8 | 14,800 |
| **SLOAD 합계** | **147** | **302,700** |

두 append의 64개 level에서 형제 노드를 읽습니다.
그중 63개 형제가 비어 있어 zeroes[level]을 storage에서 추가로 읽었습니다.
새 root 허용 목록·AuditRecord outputRefs 같은 일부 읽기는 작은 타입 저장 전 슬롯 값을 보존하는 read-modify-write에 해당합니다.
SLOAD는 144회 cold(2,100 Gas), 3회 warm(100 Gas)입니다.

원래 Tree Hash·저장 소계 2,423,804 Gas에 위 275,100 Gas를 더하면 2,698,904 Gas입니다.
이 값도 모든 Tree 함수 실행 비용은 아닙니다. 루프·mapping 주소 계산·Event 로그 등은 아직 함수별로 귀속하지 않았습니다.
따라서 전체 Tree 비용이 정확히 2,698,904라고 주장하지 않습니다.

`other-execution-breakdown.json`에 모든 opcode와 147개 읽기 slot의 분류를 보존했습니다.
미분류 SLOAD가 있으면 분석기가 실패하도록 했습니다.
분석기 첫 두 실행은 Audit outputRefs와 새 root 허용 목록 슬롯을 아직 분류하지 않아 실패했습니다.
해당 슬롯을 확인해 추가한 뒤 모든 읽기가 분류되었고 합계 검증을 통과했습니다.
재현 코드는 `cmd/analyze_transfer_gas_reads`입니다. 기존 출력이 있으면 덮어쓰기를 거부합니다.

결과 보고서의 표는 이 단계에서 변경하지 않았습니다.
