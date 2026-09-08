# Milestone Result Template

Result는 완료된 Milestone의 구현·검증·성능을 독립적으로 기억하기 위한 문서입니다.

## Metadata

```markdown
- 상태: 완료 | 부분 완료 | 실패
- 구현 명세: [링크]
- Raw 결과: [링크]
- 실행 횟수: Setup N회, 측정 N회
```

## 0. 30초 안에 기억 복구하기

첫 50줄 안에 다음을 표시합니다.

| 질문 | 답 |
|---|---|
| 이전 상태 | 직전 baseline에서 가능했던 것 |
| 이번 목표 | 해결하려던 문제 |
| 실제 구현 | 새로 추가한 기능 |
| 이제 가능한 것 | 실제 실행 가능한 흐름 |
| 검증 | 대표 correctness 결과 |
| 대표 성능 | 핵심 Circuit·Contract·감사 수치 |
| 미포함 | 아직 없는 기능 |
| 다음 단계 | 후속 Milestone |

## 1. 무엇이 달라졌나요?

기존 baseline, 계획, 실제 구현을 비교합니다.

## 2. 어떤 객체 전이를 구현했나요?

Event별 Circuit 확인과 Contract 상태 변경을 구분합니다.

## 3. 무엇이 공개되고 숨겨지나요?

public input·private witness·calldata·storage를 구분합니다.

## 4. 어떤 상태를 저장하나요?

Tree·mapping·AuditRecord·Claim·key 자료의 목적과 변경 Event를 기록합니다.

## 5. 계획과 실제 차이가 있나요?

실패·재시도·범위 변경을 숨기지 않습니다.

## 6. Correctness와 최종 상태

positive·negative·atomicity·일반 계산 대조와 최종 상태를 기록합니다.

## 7. 측정 경계와 환경

포함·제외 구간, 단위, 실행 횟수, 환경을 구분합니다.

## 8. 실제 성능 결과

Master Key·Circuit·deployment·gas·audit 결과를 단위별로 분리합니다.

## 9. 재현 명령

필요한 선행 Artifact와 실행 순서를 기록합니다.

## 10. Artifact·checksum

Git 포함 결과와 재생성 가능한 secret·Artifact를 구분합니다.

## 11. 실패·제외·한계

Production 보안으로 주장하지 않는 내용을 포함합니다.

## 12. 다음 Milestone 연결

이번 결과가 제공한 것과 다음 단계가 추가할 것을 구분합니다.
