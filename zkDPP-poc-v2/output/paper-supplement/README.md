# Process 독립 입출력 보충 측정

공식 Process는 2→1, 3→1, 4→1, 2→2, 2→3의 Audit 없음/있음 각 1회, 총 10회입니다. warm-up 0회이며 Entry 준비 Proof 26회는 별도입니다. 시간 지표만을 위한 재측정은 0회입니다.

- `process-independent.json`: 공식 10개 Measurement 집계
- `process-*-plain.json`, `process-*-audit.json`: 입력·출력 Note, 전후 root, 준비 transaction, deployment 및 개별 Measurement
- `*-transaction.json`: 실제 transaction·receipt 원문. 저장한 Proof로 재조립한 calldata와 실제 transaction data를 비교했습니다.
- `setup.json`: 새 6관계의 Constraints·domain·PK/VK/CCS hash. 기존 canonical 및 Lagrange SRS를 검증 후 재사용했습니다.
- `verification-before*.json`: 실행 source hash, 기존 baseline 예외, 보존 대상 및 환경
- `ledger-verification-*.json`: 원본 대비 등록 arity 조건 1줄만 변경했는지 검증, source·build hash. `runtimeCodeKeccak`는 immutable을 채우기 전 compiler template의 hash이며 실제 배포 runtime hash는 각 case의 `deployments[].codeHash`입니다.
- `verification-after.json`: 기존 실험 파일 4,097개와 historical baseline 일치 파일 1,182개 불변, 측정 source 불변, 10개 receipt·Proof 참조·입출력 합계 대조, 모든 plain/audit 쌍의 State·root·PolicyRef 동일 확인
- `environment-final.json`: 실제 확인한 환경 및 source/artifact 검사 결과

최초 attempt는 기존 Ledger의 registration arity allowlist에서 3→1을 거부하여 원장 준비 중 종료했습니다. 그때 공식·준비 Proof는 모두 0회였습니다. 실패 기록을 `attempts/`에 보존했습니다. 이후 기존 Ledger를 수정하지 않고 `artifacts/development/paper-supplement/contracts/src/experiments/`의 사본 두 개에 (3,1), (4,1), (2,3) 허용조건만 추가하여 10개 case 모두 같은 보충 원장 조건에서 측정했습니다.

State는 항상 3개이고 Scale=10^9, 입력 합계 (320,30,231), 출력 합계 (300,30,261), loss=6.25%, added carbon=30, waste=0%, 모두 ELIGIBLE입니다. 입력 균등분할의 잔여는 입력 1에, 출력 2개는 질량 2:1, 3개는 3:2:1로 나누고 recycled/carbon의 비례 floor 잔여는 출력 1에 넣습니다.

Event E2E는 기존 `Engine.Execute` 경계를 유지합니다. 입력 path 확보·Setup·배포·Entry 준비는 밖이며 Native/witness/Prove/transaction/receipt는 안입니다. Verify·최종 상태·calldata 대조·파일 저장은 밖입니다. E2E는 1회 보조 관측치이며 기존 5회 median과 묶지 않습니다. steps tracing을 실행하지 않았으므로 Measurement의 `SSTORECount`/`SSTOREGas`의 0은 측정값이 아닙니다. Gas는 실제 성공 receipt의 gasUsed이며 calldata byte 구성에 따른 변동도 포함합니다.

기존 baseline의 HTML 및 일부 문서 경로 예외는 branch 차이로 별도 기록했습니다. 기존 baseline/checksum을 수정하거나 통과한 것으로 표시하지 않았습니다. 기존 v1/M1/M1-HF·원자료·키는 덮어쓰지 않았습니다.

실행 entry point는 `cmd/paper_supplement`의 `preflight`, `setup`, `run`입니다. 성공 결과나 키가 이미 있으면 덮어쓰기를 거부합니다. 별도 Compose 파일은 `docker-compose.paper-supplement.yml`입니다. Supplemental Foundry out의 `Imports.sol`과 `Poseidon2BLS12381.sol`은 검증된 기존 build output으로 향하는 relative symlink이며, 새 verifier 및 Ledger만 분리 빌드합니다.
