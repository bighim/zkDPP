# zkDPP-poc-v2

zkDPP-poc-v2는 완료된 `zkDPP-poc-v1` M9을 보존하면서, Master Key 기반 감사와 변경된 Exit·Issue·Claim lifecycle을 검증하는 후속 POC입니다.

M1은 **완료** 상태입니다. 10개 Event Circuit, Master Key 복구, v2 Ledger, 외부 DPP Claim 검증과 AuditAndFreeze POC를 구현했으며 실제 결과는 [M1 Result](milestones/M1-master-key-audit-result.md)에 정리했습니다.

## 무엇을 읽나요?

| 목적 | 문서 |
|---|---|
| 전체 진행 상황 | [`MILESTONES.md`](MILESTONES.md) |
| M1 선택 이유 | [`M1-master-key-audit-background.md`](milestones/M1-master-key-audit-background.md) |
| M1 구현 기준 | [`M1-master-key-audit.md`](milestones/M1-master-key-audit.md) |
| M1 구현 순서 | [`M1-IMPLEMENTATION-PLAN.md`](milestones/M1-IMPLEMENTATION-PLAN.md) |
| M1 실제 결과 | [`M1-master-key-audit-result.md`](milestones/M1-master-key-audit-result.md) |
| M2·M3 후속 결정 | [`FUTURE-MILESTONE-CONTEXT.md`](milestones/FUTURE-MILESTONE-CONTEXT.md) |
| 설계·구현 경계 | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| v1 재사용 예정 범위 | [`REUSE.md`](REUSE.md) |

## M1은 무엇을 검증하나요?

M1은 다음 흐름을 닫힌 POC 시나리오에서 검증했습니다.

```text
외부 DKG 결과로 가정한 2-of-3 share
  → Status Authority가 Master Key 복구
  → 해당 key로 AuditRecord 전체 복호화
  → Forward Tracing
  → 전체 미소비 frontier Freeze
  → 공통 블록에서 차단 결과 확인
```

동시에 v1 M9의 객체 전이를 다음처럼 변경합니다.

```text
Exit:  Note → 없음
Issue: ELIGIBLE Note → ClaimRef(h)
```

M1은 실제 DKG, Audit Key Rotation과 post-snapshot 경합 자손 추적을 구현하지 않았습니다. 각각 M2·M3 또는 후속 검토 범위입니다.

## 재현 명령

- `make test-go`
- `make test-contract-m1`
- `make setup-m1`
- `make evaluate-m1`
- `make benchmark-m1`
- `make check-m1`

SRS·PK/VK·proof·위원 share는 개발용 생성물이며 Git 대상에서 제외됩니다.

## 무엇을 보존하나요?

- `../zkDPP-poc-v1`의 M1~M9 코드·문서·Raw 결과·Artifact 의미
- `../Conversation History`의 원본 대화와 동료 전달 문서
- v1의 Note·Voucher commitment, Owner·Policy·Audit Domain과 PolicyRef

v2 구현은 새 폴더에서만 진행하며 v1 source를 직접 import하지 않습니다.
