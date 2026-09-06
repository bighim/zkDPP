# Exit 감사 Circuit은 무엇을 확인하나요?

**기존 Exit 규칙과 실제 부모·출력 소비값의 올바른 암호화를 하나의 proof에서 확인합니다.**

[M7 명세](../../milestones/M7-audit-tracing.md)의 해당 Event 절이 구현 기준입니다. 기존 회로는 수정하지 않고 Define을 호출합니다.

## 무엇을 공개하나요?

기존 Event 공개 입력 뒤에 Jubjub 공개점 좌표, 부모 암호문, 출력 소비값 암호문을 순서대로 둡니다. 소비 cm·rv·owner secret·path·암호화 난수·평문 출력 nf는 witness입니다. 위원회 PK는 상수입니다.

## 무엇이 실패하나요?

기존 State·Role·소유권 조건 위반, 엉뚱한 부모·출력 소비값을 암호화한 경우, 잘못된 문맥·난수·공개점·암호문은 실패합니다. 독립적인 임의 메시지 암호화 증명과는 다릅니다.

## 어떻게 실행하나요?

make setup-m7, make evaluate-m7, make test-go 순서로 준비·검증합니다. make test-contract-m7은 원장까지 확인합니다. 암호화는 B1의 정직한 위원회·원본 기록 전용 코어이며 일반적인 TDH2 보안을 그대로 주장하지 않습니다.

현재 상태·미소비 검사는 Circuit 밖의 ZkDPPAuditLedger가 nf로 확인합니다. Status path는 없습니다.

