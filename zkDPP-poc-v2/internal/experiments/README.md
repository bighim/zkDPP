# 8개 실험 실행

기존 M1-HF를 변경하지 않고 실험용 회로·원장·RPC runner를 사용합니다. 현재 source의 기능 범위와 측정 계획은 [실험 문서](../../docs/experiment-priorities-and-criteria.md)를 참고합니다.

## 새 환경에서 실행

1. `make prepare-experiments`: 기존 파일 checksum과 실험 설정을 기록합니다.
2. `make setup-experiments`: 41개 relation의 compile·공통 development SRS·PK/VK·verifier를 생성합니다.
3. `docker compose -f docker-compose.experiments.yml -p zkdpp-experiments run --rm foundry build`: 실험 Contract를 compile합니다.
4. `make benchmark-experiment EXP=1`: 실제 Proof로 기본 시나리오를 warm-up 1회, 공식 5회 실행합니다.
5. `go run ./cmd/experiments -mode fixtures`: 성공한 warm-up의 Proof를 Foundry에서 재사용합니다.
6. `make test-contract-experiments`와 `go test -count=1 ./...`: correctness·회귀를 확인합니다.
7. `make benchmark-experiments`: 이미 완료된 실험 1은 검증 후 재사용하고 나머지를 순차 실행합니다.
8. `make check-experiments`, `make report-experiments`: 읽기 전용 대조 후 같은 내용의 Markdown·LaTeX를 생성합니다.

실험용 Anvil은 port 18547, Compose project `zkdpp-experiments`를 사용합니다. 포트가 사용 중이면 다른 프로세스를 종료하지 않고 실패합니다. 각 실행이 시작한 project만 종료합니다. 생성물·개발용 키는 `artifacts/development/experiments/`, Raw는 `output/experiments/`입니다.

## 측정과 재실행

- Event E2E는 입력 path를 준비한 뒤 Native 계산·witness·Prove·transaction 구성·서명·receipt까지입니다. 로컬 Verify·상태 대조·trace·파일 저장은 제외합니다.
- 전달 경로에서는 첫 입력 path는 사전 확보하고, 중간 Voucher 정보·path 생성은 포함합니다. 사후 진단은 마지막 receipt 뒤로 미룹니다.
- DPPClaim 검증은 실제 RPC 조회와 Hash·원본 대조이며 PLONK Verify와 별도입니다.
- 감사 원본 cache는 각 감사 실행마다 새로 만듭니다. 검증한 동일 기록만 재사용하며 영구 cache를 만들지 않습니다.
- 복호화·키 복구는 workload를 1,000회 처리한 묶음을 한 run으로 측정합니다. workload 크기, 묶음 수, 공식 5회를 혼동하지 않습니다.
- `-smoke`는 실험 1~6의 개발용 단일 warm-up입니다. 결과는 attempts에 저장하며 공식 결과로 사용하지 않습니다.
- 성공한 결과 파일은 덮어쓰지 않습니다. 실패 이력은 `attempts/*-start.json`, `*-finish.json`에 남깁니다. 수정 후 재측정이 필요하면 이전 결과를 별도 attempt로 보존해야 합니다.
- Setup은 기존 SRS가 있으면 재생성을 거부합니다. 이전 키와 새 source가 섞이지 않도록 compile·manifest·Proof 대조를 수행합니다.

## 재사용·한계

감사 runtime은 M1-HF source를 실험 경로로 분리한 뒤 가변 Process arity와 실행 단위 cache를 추가했습니다. 기존 package는 수정하지 않습니다. No-Audit Ledger는 소비 boolean·Status·Policy·Claim 등록을 유지하고 감사 배열·연결 mapping을 제외합니다.

실험 Policy family 101·102는 기존 family와 구분하기 위한 예약 번호입니다. 선행 빈 family 예약은 원장 준비로 기록하고 Event E2E와 분리합니다. 로컬 Anvil의 공개 테스트 키와 고정 Committee fixture는 운영 키가 아닙니다.

최종 보고서는 Raw 대조가 끝난 경우에만 생성합니다. 미완료 결과를 과거 수치로 채우거나 개발용 실행을 공식 5회로 표시하지 않습니다.
