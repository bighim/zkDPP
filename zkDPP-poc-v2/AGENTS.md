# zkDPP-poc-v2 작업 지침

이 파일은 새 대화·컨텍스트 압축·작업 인계 뒤 v2의 기준을 복구하는 시작점입니다.

## 프로젝트 목적

zkDPP-poc-v2는 다음을 확인하는 POC입니다.

- Master Key를 복구하면 한 key 기간의 AuditRecord를 정확히 일괄 복호화할 수 있는가?
- 변경된 Exit·Issue·Claim과 회계 규칙을 Circuit·Contract가 일관되게 강제하는가?
- Forward Tracing 결과 전체를 Freeze하고 공통 Snapshot에서 결과를 판정할 수 있는가?
- 기존 v1 M9 방식과 비교한 성능·Privacy 상충 관계는 무엇인가?

## 읽기 순서

1. `AGENTS.md`
2. `ARCHITECTURE.md`
3. `REUSE.md`
4. `MILESTONES.md`
5. 현재 Milestone Background
6. 현재 Milestone 구현 명세
7. 구현 뒤 생성된 Result
8. 작업 대상 Feature README

M2·M3을 시작할 때는 `milestones/FUTURE-MILESTONE-CONTEXT.md`를 먼저 읽습니다.

## 문서 권한

충돌 시 다음 순서를 사용합니다.

1. 사용자에게 승인받은 현재 Milestone 명세
2. `ARCHITECTURE.md`의 구현 완료 invariant
3. `REUSE.md`의 실제 재사용 검증 경계
4. 완료된 Result의 실제 구현 사실
5. Future Context
6. `../Conversation History`와 `../zkDPP-poc-v1`

현재 활성 구현 기준은 M1-HF입니다. M1 자체 명세와 충돌하는 Domain·Issue 문자열 처리·감사 결과 조건은 승인된 M1-HF 명세를 따릅니다. 과거 M1 source는 commit `8560fd3`, Raw·Result·Artifact는 기존 경로로 보존합니다.

## v1 보존 원칙

- `../zkDPP-poc-v1`은 완료된 M9 baseline입니다.
- v2를 위해 v1 코드·Circuit·Contract·Artifact·Raw JSON·Result를 수정하지 않습니다.
- v1과 v2 측정값을 비교할 때 v1 Raw를 다시 실행하거나 덮어쓰지 않습니다.
- v2 source는 새 Go module 경로를 사용하고 v1을 직접 import하지 않습니다.

## YAGNI Gate

다음에 필요한 기능만 추가합니다.

1. 현재 Milestone correctness 확인
2. 공개·비공개 경계 검증
3. 성능 측정과 결과 재현
4. 두 개 이상의 실제 기능에서 확인된 중복 제거

M1에서는 DKG·Key Rotation·post-snapshot 자손 fallback·운영 서버·Production key deletion 증명을 구현하지 않습니다.

## 문체와 명세 기준

- 큰 그림과 핵심 문장을 먼저 씁니다.
- Event·Circuit·Contract·State·public input·private witness 등 의미가 고정된 용어는 유지합니다.
- 누가 계산하고 누가 확인하며 무엇을 저장하는지 주어를 생략하지 않습니다.
- 수식은 코드 블록 밖에 두고 복잡한 수식을 표 안에 넣지 않습니다.
- 기능별 목적·입력·출력·일반 계산·Circuit pseudo code·Contract pseudo code·상태 변화·실패 조건을 같은 절에 둡니다.
- Pseudo code는 `# 핵심:`과 3~8개의 단계별 한글 주석을 사용합니다.
- 완료일과 기준일은 문서에 넣지 않습니다.

## 구현 완료 조건

- `milestones/MILESTONE-CHECKLIST.md`를 만족합니다.
- 일반 계산·Circuit·Contract·감사 원문이 일치합니다.
- positive·negative·atomicity·성능 측정을 완료합니다.
- `milestones/RESULT-TEMPLATE.md`에 따른 Result를 생성합니다.
- Result와 Raw JSON이 일치하기 전에는 완료로 표시하지 않습니다.
