# Milestone 구현 전 Checklist

Milestone 명세 하나만 읽고 구현할 수 있는지 확인합니다. 해당 없는 항목은 `해당 없음`으로 기록합니다.

## 문제·목표·범위

- [ ] 해결할 문제와 완료 후 가능한 행동이 첫 화면에 있습니다.
- [ ] 구현할 기능과 구현하지 않을 기능이 분리돼 있습니다.
- [ ] 긴 선택 이유는 같은 폴더의 Background에 보존됩니다.

## 객체·전이·신뢰

- [ ] 역할과 신뢰 주체가 정해졌습니다.
- [ ] 객체·State·단위·범위가 정해졌습니다.
- [ ] Event별 입력·출력·terminal 조건이 정해졌습니다.
- [ ] Master Key·share·삭제·DKG의 보장과 가정이 분리돼 있습니다.

## Privacy·Circuit·Contract

- [ ] public input·private witness·calldata·storage가 구분됩니다.
- [ ] 일반 계산과 Circuit 관계가 같습니다.
- [ ] Circuit·Contract·외부 신뢰의 책임이 구분됩니다.
- [ ] 각 Event 절에 전체 Circuit·Contract pseudo code가 있습니다.

## 상태·Interface·원자성

- [ ] 저장 상태와 각 mapping·Tree의 목적이 설명됩니다.
- [ ] 함수 입력·출력·호출 권한이 정해졌습니다.
- [ ] AuditRecord와 producer·spent·Claim 변경이 원자적입니다.
- [ ] AuditAndFreeze의 off-chain·on-chain 경계가 명확합니다.

## 시나리오·검증·측정

- [ ] 대표 성공 시나리오와 필수 실패 시나리오가 있습니다.
- [ ] Master Key 복구·암호문·Event 의미를 대조합니다.
- [ ] 측정 범위·단위·횟수·Raw 위치가 정해졌습니다.
- [ ] Artifact·SRS·key lifecycle과 재현 명령이 정해졌습니다.

## Colocation·완료

- [ ] 기능별 목적·입력·출력·pseudo code·상태·실패 조건이 같은 절에 있습니다.
- [ ] Background 없이도 명세로 구현할 수 있습니다.
- [ ] Result Template에 따른 Result가 완료 조건에 포함됩니다.
