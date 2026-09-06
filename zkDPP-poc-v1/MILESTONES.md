# zkDPP-poc-v1 Milestones

이 문서는 전체 진행 상황과 Context·명세·Result의 Index입니다.

## 읽기 안내

| 목적 | 읽기 순서 |
|---|---|
| 지난 작업 기억 복구 | 이 표 → 해당 Result |
| 상세 동작 확인 | Result → Milestone 명세 |
| 다음 기능 설계 | 직전 Result → Future Context → 새 명세 |

## 상태

| 상태 | 의미 |
|---|---|
| 미정 | Context만 있고 구현 명세가 없음 |
| 설계 중 | 구현 전 결정을 논의 중 |
| 구현 준비 완료 | Checklist와 명세 검토 완료 |
| 구현 중 | 코드·test 작성 중 |
| 검증 중 | 구현 후 correctness·측정 확인 중 |
| 완료 | 구현·test·측정·Result 완료 |

## 진행 상황

| 단계 | 상태 | 한 줄 기능 | Context | 명세 | Result |
|---|---|---|---|---|---|
| M0 | 완료 | 운영·문서 기준 | — | — | — |
| M1 | 완료 | Private Note Core | — | [명세](milestones/M1-private-note-core.md) | [Result](milestones/M1-private-note-core-result.md) |
| M2 | 완료 | Entry·Exit Ledger | — | [명세](milestones/M2-entry-exit-ledger.md) | [Result](milestones/M2-entry-exit-ledger-result.md) |
| M3 | 완료 | Transfer·Voucher | [Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m3) | [명세](milestones/M3-transfer-voucher.md) | [Result](milestones/M3-transfer-voucher-result.md) |
| M4 | 완료 | Merge·Split | [Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m4) | [명세](milestones/M4-merge-split.md) | [Result](milestones/M4-merge-split-result.md) |
| M5 | 완료 | Process·Policy | [Background](milestones/M5-process-policy-background.md) | [명세](milestones/M5-process-policy.md) | [Result](milestones/M5-process-policy-result.md) |
| M6 | 완료 | Active·Frozen·Revoked | [Background](milestones/M6-status-enforcement-background.md) | [명세](milestones/M6-status-enforcement.md) | [Result](milestones/M6-status-enforcement-result.md) |
| M6-B1 | 완료 | 재사용 감사 암호화 코어 검증 | [Background](milestones/M6-B1-audit-encryption-core-background.md) | [명세](milestones/M6-B1-audit-encryption-core.md) | [Result](milestones/M6-B1-audit-encryption-core-result.md) |
| M7 | 완료 | 감사 기록·추적·nf 기반 동결 통합 | [Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m7) | [명세](milestones/M7-audit-tracing.md) | [Result](milestones/M7-audit-tracing-result.md) |
| M8 | 완료 | Exit·DPP·Issue Claim | [Background](milestones/M8-exit-dpp-issue-background.md) | [명세](milestones/M8-exit-dpp-issue.md) | [Result](milestones/M8-exit-dpp-issue-result.md) |
| M9 | 완료 | Universal SRS·최종 Anvil 통합 | [Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m9) | [명세](milestones/M9-final-integration.md) | [Result](milestones/M9-final-integration-result.md) |

M6-B1은 암호화 코어·대표 Process 연결·2-of-3 복원·진단 측정을 완료했습니다. Main Contract 교체는 아니며 기존 M6의 완료 상태와 결과는 유지합니다. 전체 원장·탐색·동결 통합은 [후속 Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m6-b1)로 구분합니다.

M7은 AuditRecord·8개 감사 Event·양방향 추적·nf 상태 집행의 구현·검증·측정을 완료했습니다. public input 전체를 복사하지 않고 암호문·outputRefs·필요한 메타데이터만 기록합니다. 상세 동작과 결과는 단일 명세와 Result에서 확인하며 별도 Background는 없습니다.

M8은 DPP commitment·M8 Exit·두 Issue Policy·Claim 상태·upstream 감사를 구현하고 검증했습니다. 큰 그림과 실제 성능은 [Result](milestones/M8-exit-dpp-issue-result.md), 선택 이유와 상세 관계는 [Background](milestones/M8-exit-dpp-issue-background.md)와 [구현 명세](milestones/M8-exit-dpp-issue.md)에서 확인합니다.

M9는 최종 Circuit 10개를 $2^{17}$ universal SRS로 Setup하고, Process·Issue verifier의 PolicyRecord routing과 전체 Protocol Anvil 시나리오를 구현·검증했습니다. 실제 SRS·Circuit·gas·감사 결과는 [M9 Result](milestones/M9-final-integration-result.md)에서 확인합니다.

## 구현 시작 Gate

1. Future Context와 현재 코드·Architecture를 대조합니다.
2. [`MILESTONE-CHECKLIST.md`](milestones/MILESTONE-CHECKLIST.md)를 확인합니다.
3. 별도 Milestone 명세를 작성하고 `구현 준비 완료`로 합의합니다.

Future Context만으로 코드를 구현하지 않습니다.

## 완료 Gate

1. 명세의 positive·negative·atomicity Gate를 통과합니다.
2. 측정 경계와 횟수를 지킵니다.
3. [`RESULT-TEMPLATE.md`](milestones/RESULT-TEMPLATE.md)에 맞춘 Result를 생성합니다.
4. Result와 Raw JSON을 대조합니다.

Result가 없으면 milestone을 완료로 표시하지 않습니다.
