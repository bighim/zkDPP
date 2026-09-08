# zkDPP-poc-v2 Milestones

이 문서는 v2 진행 상태와 명세·Background·Result의 Index입니다.

## 상태

| 상태 | 의미 |
|---|---|
| 미정 | Future Context만 있고 구현 명세가 없습니다. |
| 설계 중 | 구현 전 관계·Interface·Gate를 검토하고 있습니다. |
| 구현 준비 완료 | 명세 검토와 사용자 승인이 끝났습니다. |
| 구현 중 | 코드·Circuit·Contract·test를 작성하고 있습니다. |
| 검증 중 | correctness·측정·Result를 확인하고 있습니다. |
| 완료 | 구현·test·측정·Result가 모두 완료됐습니다. |

## 진행 상황

| 단계 | 상태 | 한 줄 목표 | Background·Context | 명세 | Result |
|---|---|---|---|---|---|
| M0 | 완료 | v1 M9 보존과 v2 문서 체계 분리 | — | — | — |
| M1 | 완료 | Master Key 기반 v2 Event·AuditAndFreeze 통합 | [Background](milestones/M1-master-key-audit-background.md) | [명세](milestones/M1-master-key-audit.md) | [Result](milestones/M1-master-key-audit-result.md) |
| M2 | 미정 | Jubjub 2-of-3 Committee DKG | [Future Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m2) | — | — |
| M3 | 미정 | Audit Key Rotation | [Future Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m3) | — | — |

## 현재 경계

M1은 외부 DKG가 분배했다고 가정한 고정 share를 사용합니다. Event 실행이 끝난 닫힌 시나리오에서 Master Key를 복구하므로, 운영 중 Active key를 공개해도 안전하다고 주장하지 않습니다.

M1은 [일괄 구현 계획](milestones/M1-IMPLEMENTATION-PLAN.md)에 따라 구현·측정·Result 대조를 완료했습니다. 실제 DKG와 key rotation은 각각 M2·M3의 별도 범위입니다.

M2·M3은 Future Context만으로 구현하지 않습니다. 해당 단계 시작 전에 별도 Background와 자체 완결형 구현 명세를 작성합니다.
