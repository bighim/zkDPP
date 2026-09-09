# M1-HF 재현 실행

`cmd/m1_hotfix`는 Hotfix의 Setup·실제 proof 생성·Anvil 실행·최종 파일 검사를 연결합니다.

- `setup`: 10개 Circuit을 하나의 development universal SRS로 Setup합니다. 기존 SRS가 있으면 교체하지 않습니다.
- `evaluate`: 연결 시나리오의 26개 proof를 생성하고 저장된 키로 검증합니다. 첫 Event를 각 relation의 대표 측정으로 기록합니다.
- `anvil`: 실제 Poseidon2·verifier 10개·Ledger를 배포하고 20개 Event lifecycle과 3개 Event 감사 graph를 실행합니다.
- `audit`: 별도 clean chain에서 같은 실제 proof를 제출한 뒤 원본 transaction·receipt와 storage를 대조하고 양방향 감사·동결을 실행합니다.
- `key-recovery`: 3개 위원 조합과 1·10·100·1,000개 기록 복호화를 측정합니다.
- `finalize`: 기존 결과와의 일치를 확인한 뒤 생성물 checksum을 처음 고정합니다.
- `check`: 저장된 checksum, Circuit을 메모리에서 재compile한 CCS, public input 순서, Raw 결과를 읽기 전용으로 대조합니다.

새 측정 파일이 이미 있으면 해당 측정 명령은 실패합니다. 수정 후 재실행이 필요하면 앞선 실행 파일을 `artifacts/development/m1-hotfix/attempts`에 보존한 뒤 실행합니다. 실행 횟수와 이유를 Result에 함께 기록합니다.

기존 M1 결과는 덮어쓰지 않습니다. 기존 M1의 source는 commit `8560fd3`에서 확인할 수 있습니다.

테스트용 고정 share와 owner secret은 운영 키가 아닙니다. AuditRecord의 난수는 고정 재현 입력으로서 시나리오 사이에서도 서로 다르게 구성합니다. 실제 배포용 난수는 `auditcrypto.RandomScalar`를 사용해야 합니다.

자세한 동작과 Gate는 [M1-HF 명세](../../milestones/M1-HF-spec-conformance.md)에 있습니다.
