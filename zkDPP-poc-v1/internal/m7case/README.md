# M7 테스트 데이터는 어떻게 연결되나요?

**같은 객체를 다음 Event에서 실제로 소비하도록 root·path·State를 연결합니다.**

- Note graph: Entry A → Split B·C → Split D·E → C Exit입니다. D·E Exit proof는 동결·철회 실패 검사에만 사용합니다.
- Voucher: M3의 부분 Transfer·Proceed, 전량 Transfer·Recall을 사용합니다.
- Merge: M4의 두 Entry·Merge·Split입니다.
- Process: M5의 세 Entry·3→2 Process이며 B1의 감사 Process keys를 재사용합니다.

일반 계산으로 실제 부모와 출력 소비값을 만든 뒤 새 난수로 암호화합니다. 기존 Note·Voucher 모델이나 과거 시나리오는 수정하지 않습니다.

공식 evaluate는 Event별 첫 case를 대표 성능으로 기록합니다. 같은 Event의 나머지 연결 proof는 correctness용이며 별도 호출 횟수를 남깁니다. 생성한 proof는 Contract test·gas·감사 시나리오에서 재사용합니다.
