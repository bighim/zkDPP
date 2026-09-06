# zkDPP-poc-v1 작업 지침

이 파일은 새 대화, 컨텍스트 압축 또는 작업 인계 후 프로젝트 기준을 복구하는 시작점입니다. 상세 설계·진행 상태·결과를 이 파일에 복사하지 않습니다.

## 프로젝트 목적

zkDPP-poc-v1은 다음을 확인하는 POC입니다.

- 구현 가능한가?
- 올바른 입력과 잘못된 입력을 구분하는가?
- 핵심 기능의 성능은 어느 정도인가?

Production 고도화는 현재 milestone의 기능·correctness·측정에 필요할 때만 추가합니다.

## 읽기 순서

### Agent가 작업을 시작할 때

1. `AGENTS.md`
2. `ARCHITECTURE.md`
3. `REUSE.md`
4. `MILESTONES.md`
5. 현재 Milestone Background가 있으면 해당 문서
6. 현재 Milestone 명세
7. 직전 Milestone Result
8. 작업 대상 Feature README

미래 milestone을 시작할 때는 `milestones/FUTURE-MILESTONE-CONTEXT.md`도 읽습니다. Future Context는 구현 명세가 아니므로 현재 코드와 최신 결정을 대조한 뒤 새 명세를 작성합니다.

### 사람이 지난 작업을 기억할 때

```text
MILESTONES.md
  → 해당 Result
  → 상세 정보가 필요할 때만 Milestone 명세
  → 결정 이유가 필요할 때만 Milestone Background
```

Result 하나만으로 객체 전이, Privacy, 상태 변화, 검증과 대표 성능이 보여야 합니다.

## 문서 권한

충돌 시 다음 순서를 사용합니다.

1. 현재 구현 대상 Milestone의 동결된 명세
2. `ARCHITECTURE.md`의 stable invariant
3. `REUSE.md`의 재사용 경계
4. 완료된 Milestone Result의 실제 구현 사실
5. Future Context와 Conversation History

기존 `poc-v2`와 Conversation History는 참고 자료이지 zkDPP-poc-v1의 현재 명세가 아닙니다.

완료된 Milestone 명세·Result는 당시 구현의 기록입니다. 후속 설계가 바뀌어도 과거 결과를 새 방식으로 고쳐 쓰지 않습니다. 활성 문서에서는 **현재 구현**, **후속 단계에서 결정한 변경**, **아직 미정인 세부사항**을 구분합니다. 완료된 POC 결정을 미정으로 남기거나, 예정된 교체를 이미 구현된 것으로 표현하지 않습니다.

## YAGNI Gate

새 코드나 추상화는 다음 중 하나에 해당할 때만 추가합니다.

1. 현재 milestone 완료 조건에 필요합니다.
2. Correctness·구현 가능성 확인에 필요합니다.
3. 성능 측정·결과 기록에 필요합니다.
4. 두 개 이상의 실제 기능에서 확인된 중복을 제거합니다.

YAGNI를 이유로 공개·비공개 경계, 숫자 범위, Hash Domain, negative test와 일반 계산·Circuit 일치를 생략하지 않습니다.

## 구현 경계

- 설계 변경은 코드보다 Milestone 명세에 먼저 반영합니다.
- 기존 `poc-v2` 파일을 수정하거나 직접 import하지 않습니다.
- 고정 ZK identity는 `testdata/common/actors-v1.json`만 기준으로 사용합니다.
- 고정 secret은 로컬 재현용이며 Production·공개 네트워크에 사용하지 않습니다.
- 재생성 가능한 SRS·key·proof·build 결과는 소스와 구분합니다.
- 측정은 Milestone에서 정한 횟수·경계만 사용합니다.
- 구현·test·측정·Result가 끝나지 않으면 완료로 표시하지 않습니다.

## Milestone 문서 Gate

구현 전 [`milestones/MILESTONE-CHECKLIST.md`](milestones/MILESTONE-CHECKLIST.md)를 확인합니다. Milestone 명세는 무엇을 왜, 어떤 관계와 Interface로 구현할지 결정합니다.

Milestone 명세는 해당 단계의 자체 완결형 구현 명세여야 합니다. 구현 담당자가 `ARCHITECTURE.md`, `REUSE.md` 또는 Conversation History를 오가지 않아도 다음 내용을 알 수 있어야 합니다.

- 사용하는 객체·State·단위·Domain
- 공개값·비공개값
- Native 계산과 Circuit 전체 검증 순서
- Contract 함수·저장 상태·원자적 변경
- Canonical fixture·폴더·명령·Artifact lifecycle
- Positive·negative·측정·완료 Gate

공통 문서와 일부 중복되더라도 구현 이해에 필요한 내용은 Milestone 명세에 반복합니다. 문서 길이를 줄이기 위해 실제 pseudo code·Interface·실패 조건을 외부 문서나 부록으로 분리하지 않습니다. 같은 기능의 목적, 입력·출력, pseudo code, 상태 변화, 실패 조건과 측정은 같은 절에 둡니다.

대안 비교와 의사결정 과정이 긴 Milestone은 같은 `milestones/` 폴더에 Background 문서를 둘 수 있습니다. Background는 왜 선택했는지와 폐기·보류 대안을 보존하지만 구현 명세를 대신하지 않습니다. 명세에는 각 결정의 짧은 이유와 Background anchor를 함께 둡니다.

완료 시 [`milestones/RESULT-TEMPLATE.md`](milestones/RESULT-TEMPLATE.md)를 사용합니다. Result에는 반드시 다음을 포함합니다.

- 첫 50줄의 30초 기억 복구 표
- 이전 milestone에서 달라진 점
- 실제 구현한 객체 전이·공개범위·저장 상태
- 계획과 실제 차이
- Correctness·측정 경계·실제 수치
- 실패·비범위·다음 milestone

Result 수치는 Raw JSON과 대조합니다. Result가 없으면 milestone을 완료로 표시하지 않습니다.

## 문체

- 사람이 자연스럽게 읽는 한국어를 사용합니다.
- 제목은 자연스러운 질문형을 우선하고 본문은 `~입니다/~합니다`로 작성합니다. 한 문장 핵심 → 같은 대표 예시 → 필요한 관계식·구현 조건 순서로 설명합니다.
- 코드 식별자와 정확한 암호학 용어만 영어를 유지합니다.
- 설명에서는 Native를 일반 계산, fixture를 고정 테스트 데이터, canonical scenario를 대표 실행 예시, Gate를 성공·실패 확인 조건으로 풀어 씁니다. 정확한 코드 이름은 유지합니다.
- Participant·Circuit·Contract·위원·Auditor 중 누가 무엇을 계산·검증·저장하는지 주어를 명시합니다. `binding 검증`처럼 압축하지 않고 실제로 어떤 값과 어떤 값이 같아야 하는지 설명합니다.
- 공개 입력·transaction calldata·Event 로그·Contract storage는 구분합니다. 공개된 값을 모두 storage에 복사해야 한다고 가정하지 않으며, 각 저장값의 필요성과 실험용 중복 여부를 설명합니다.
- 상태·역할·공개범위는 짧은 표를 우선합니다.
- 설명은 문장·표·목록을 사용하고, code block은 의사 코드·정확한 명령·구조에 한정합니다. 수식은 code block 밖에 두며 cm·nf의 첨자는 LaTeX로 표현합니다.
- 상세 수식·수도 코드는 사용하는 절 또는 부록에 둡니다.
- 실제 구현에 직접 필요한 pseudo code는 해당 기능 절에 두고, 부록에는 배경·파라미터 표·긴 생성 파일 목록만 둡니다.
- Pseudo code 첫 부분에는 `# 핵심:` 한 줄을 두고, 이후에는 `# 1. 입력 검증`, `# 2. 핵심 계산`, `# 3. 상태 반영`처럼 논리 단계별 한글 주석을 사용합니다.
- 모든 줄을 설명하는 주석은 피하고, 독자가 흐름을 끊지 않도록 단계 묶음에만 주석을 답니다.
