# zkDPP-poc-v1

비공개 공급망 Protocol의 구현 가능성, correctness와 성능을 단계별로 확인하는 POC입니다.

## 무엇을 읽나요?

| 목적 | 문서 |
|---|---|
| 전체 진행 상황 | [`MILESTONES.md`](MILESTONES.md) |
| 지난 작업 기억 복구 | 해당 Milestone Result |
| 상세 설계 | 해당 Milestone 명세 |
| 설계 결정 이유 | 해당 Milestone Background |
| 전체 시스템 구조 | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| 최종 Protocol의 동작을 이해 | [`zkDPP-protocol-guide.md`](docs/zkDPP-protocol-guide.md) |
| 구현 수정에 필요한 상세 Reference | [`zkDPP-protocol-reference.md`](docs/zkDPP-protocol-reference.md) |
| AI·기계용 Protocol Context | [`zkDPP-protocol.yaml`](docs/zkDPP-protocol.yaml) |
| 재사용 가능한 기능 | [`REUSE.md`](REUSE.md) |
| M3~M9 기존 논의 | [`FUTURE-MILESTONE-CONTEXT.md`](milestones/FUTURE-MILESTONE-CONTEXT.md) |

현재 완료된 결과:

- [M1 Private Note Core Result](milestones/M1-private-note-core-result.md)
- [M2 Entry·Exit Ledger Result](milestones/M2-entry-exit-ledger-result.md)
- [M3 Transfer·Voucher Result](milestones/M3-transfer-voucher-result.md)
- [M4 Merge·Split Result](milestones/M4-merge-split-result.md)
- [M5 Process·Policy Result](milestones/M5-process-policy-result.md)
- [M6 Status 집행 Result](milestones/M6-status-enforcement-result.md)
- [M6-B1 감사 암호화 코어 Result](milestones/M6-B1-audit-encryption-core-result.md)
- [M7 감사 기록·양방향 추적·nf 기반 동결 Result](milestones/M7-audit-tracing-result.md)
- [M8 Exit·DPP·Issue Claim Result](milestones/M8-exit-dpp-issue-result.md)
- [M9 Universal SRS·최종 Anvil 통합 Result](milestones/M9-final-integration-result.md)

최근 구현 기준:

- [M6-B1 감사 암호화 코어 Background](milestones/M6-B1-audit-encryption-core-background.md)
- [M6-B1 감사 암호화 코어 구현 명세](milestones/M6-B1-audit-encryption-core.md)

M6-B1은 3→2 Process 사례로 암호화·증명·2-of-3 복원을 검증했습니다. 전체 AuditRecord 원장·그래프 탐색·동결 전환은 [후속 Context](milestones/FUTURE-MILESTONE-CONTEXT.md#m6-b1)에 남아 있습니다. 기존 M6 Main Contract는 그대로 유지합니다.

최근 Main Protocol은 [M7 감사 기록·양방향 추적·nf 기반 동결 명세](milestones/M7-audit-tracing.md)와 [Result](milestones/M7-audit-tracing-result.md)입니다. 별도 Background 없이 예시·선택 이유·구현 조건을 한 파일에 담았습니다. AuditRecord에는 암호문과 outputRefs를 보관하며, B1의 15개 공개 입력 전체 복사는 가져오지 않았습니다.

최근 Main Protocol은 [M8 구현 명세](milestones/M8-exit-dpp-issue.md)와 [Result](milestones/M8-exit-dpp-issue-result.md)입니다. Exit에서 private Note를 DPP commitment로 확정하고, 같은 DPP에 Standard·Strict Policy Claim을 붙여 독립 상태와 upstream 감사를 검증했습니다. 다음 단계는 M9 universal SRS·최종 통합입니다.

최종 통합 기준은 [M9 명세](milestones/M9-final-integration.md)와 [Result](milestones/M9-final-integration-result.md)입니다. 현재 가장 큰 Process를 지원하는 $2^{17}$ universal SRS, final Circuit 10개, ZkDPPClaimLedger와 전체 Anvil 시나리오를 재현했습니다.

## 실행 명령

```text
make test-go

make setup-m1
make evaluate-m1

make setup-m2
make test-contract-m2
make benchmark-m2-gas
make benchmark-m2-e2e

make setup-m3
make test-contract-m3
make benchmark-m3-gas
make benchmark-m3-e2e

make setup-m4
make test-contract-m4
make benchmark-m4-gas
make benchmark-m4-e2e

make setup-m5
make test-contract-m5
make benchmark-m5-gas
make benchmark-m5-e2e
make benchmark-m5-verifier-ablation

make setup-m6
make test-contract-m6
make benchmark-m6-gas
make benchmark-m6-e2e

make setup-m6-b1
make evaluate-m6-b1
make test-contract-m6-b1
make benchmark-m6-b1-gas
make benchmark-m6-b1-decrypt
make check-m6-b1

make setup-m7
make evaluate-m7
make test-contract-m7
make benchmark-m7-gas
make benchmark-m7-audit
make check-m7

make setup-m8
make evaluate-m8
make test-contract-m8
make benchmark-m8-gas
make benchmark-m8-audit
make check-m8

make setup-m9-final
make evaluate-m9
make benchmark-m9-setup
make test-contract-m9
make benchmark-m9-anvil
make benchmark-m9-audit
make check-m9
```

고정 identity는 `testdata/common/actors-v1.json`에 있으며 공개된 로컬 실험용 값입니다. 실제 참여자·공개 네트워크·Production에 사용하면 안 됩니다.

M6-B1은 공식 Raw가 이미 있으면 중복 측정을 거부합니다. 위원회 shares·개발 keys는 Git에서 제외하며, 전체 Foundry 회귀에 필요한 과거 생성물과 재현 전제는 [M6-B1 Result](milestones/M6-B1-audit-encryption-core-result.md#9-어떻게-재현하나요)에 있습니다.
