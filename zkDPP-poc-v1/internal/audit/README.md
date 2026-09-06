# 감사 프로그램은 무엇을 복원하나요?

**AuditRecord 원본을 확인하고, 위원 두 명의 응답으로 부모·출력 소비값을 복원해 연결관계와 미소비 leaf를 반환합니다.**

[M7 명세](../../milestones/M7-audit-tracing.md)의 5장이 기준입니다.

## 어떤 경계로 나뉘나요?

| 구성 | 역할 |
|---|---|
| model.go | Event별 길이·순서·ObjectRef·AuditRecord·공개 입력입니다. |
| rpc.go | 고정 snapshot의 storage와 직접 호출 transaction 원본을 대조합니다. |
| trace.go | 원문을 실행 중 메모리에 재사용하며 BFS로 부모·자손을 찾습니다. |

부모와 출력 소비값은 키 하나의 암호문입니다. outputRefs와 같은 순서로 출력 nf·rvnf를 읽습니다. 기록 ID와 transaction hash는 다릅니다.

## 누가 어떤 정보를 보나요?

위원은 같은 기록을 독립 조회해 partial decryption만 제공합니다. Auditor는 복원 원문·연결관계·대상 nf를 볼 수 있습니다. CLI 결과는 승인된 감사용이며 일반 공개 성능 JSON과 구분합니다.

같은 기록의 원문은 한 실행 안에서만 재사용합니다. 영구 저장·서버·자동 동결은 없습니다. 복호화 코어의 정직한 위원회·원본 전용 가정을 유지합니다.

## 실패하면 어떻게 하나요?

원본 없음·잘못된 receipt·암호문 불일치·snapshot 변경·지원하지 않는 내부 호출은 명시적 오류입니다. 조회 실패를 미소비로 해석하지 않습니다. Exit는 종료 경로이며 새로운 leaf가 아닙니다.

## 어떻게 실행하나요?

프로젝트 root에서 go run ./cmd/audit_m7에 -manifest, -block, -direction, -type, -id를 전달합니다. -id는 canonical 32-byte hex cm·rv입니다. RPC 기본값은 http://127.0.0.1:18545입니다.

배포 manifest의 Ledger·chain·code checksum·위원회 설정과 실제 snapshot이 일치해야 합니다. CLI는 동결 transaction을 제출하지 않습니다.
