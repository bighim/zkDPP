# Shared-secret 직접 전달 — Gas 보충 실험

**실제 Anvil transaction 1회의 Gas는 2,724,242입니다.**

| 실행 | 역할 | Gas | Constraints |
|---|---|---:|---:|
| Shared Entry | 선행 Note 등록 | 2,258,733 | 18,416 |
| Shared direct Transfer | 공식 비교값 | **2,724,242** | **49,627** |
| Receiver Exit | 소유 자격·소비값 correctness | 534,352 | 28,241 |

공식 direct Transfer는 1회, warm-up은 0회입니다. 새로운 Proof는 3개이며 Entry·Exit의 2개는 준비·correctness용입니다. 시간의 median이나 분포를 측정한 결과가 아닙니다.

## 비교하려는 동작

Note 하나를 소비하고 Receiver Note·Sender Change Note를 직접 만듭니다. Voucher는 없습니다. 부모 cm과 두 출력의 미래 nf를 감사 공개키로 암호화하고, 실제 AuditRecord와 producerOf·noteSpentIn을 저장합니다.

기존 direct-plain과 입력 State `(160,15,115)`, 전량 전달, 운송 탄소 `1`, zero Change 조건을 맞췄습니다. 값은 `10^9` scale입니다. 원래 실험의 direct official run 1과 opening offset도 맞췄습니다. 원래 2,366,228 Gas는 기존 5회 median이며 이번 값은 1회이므로 동일 반복 통계라고 표현하지 않습니다.

## deliberate Privacy 차이

이번 대안의 모든 Note는 다음 소비값을 사용합니다.

`s = H(ExperimentSharedResolutionTag, opening)`

`nf = H(NullifierTag, cm, s)`

`ExperimentSharedResolutionTag = HashToField("zkDPP:ExperimentSharedResolution:v1")`입니다. Note commitment의 필드와 Domain은 기존 것과 같습니다.

Receiver Note opening을 아는 Sender는 미래 nf를 알 수 있습니다. 따라서 Receiver가 소비할 때 공개되는 nf로 해당 소비를 식별할 수 있습니다. 그러나 spend Circuit은 별도로 `Address(secret) == Note.Address`를 강제하므로, opening을 아는 것만으로는 소비하지 못합니다. 표의 Privacy △는 이 Sender linkability만 뜻합니다. State·owner secret 전체 공개를 뜻하지 않습니다.

**이 nf 규칙은 기존 owner-secret nf와 다릅니다.** 새로운 `SharedSecretAuditLedger`에만 Entry·Direct·Exit relation으로 사용합니다. 기존 owner-secret 소비 경로나 Policy Event 경로는 `_recordPolicy`에서 거부합니다. 같은 cm을 두 종류의 nf로 소비하는 우회 경로를 허용하지 않습니다. 기존 zkDPP Circuit·Ledger·키는 수정하지 않습니다.

## 확인한 결과

- 실제 PLONK Proof를 새 verifier에서 검증했습니다.
- 실제 RPC의 Note root가 Native root와 일치했습니다.
- 실제 AuditRecord의 공개점·암호문 배열이 Proof의 공개 입력과 일치했습니다.
- 각 입력의 noteSpentIn과 출력 producerOf·등록 상태가 일치했습니다.
- 복구한 감사 Master Key로 부모 cm과 Receiver·Change nf를 복원했습니다.
- Receiver의 실제 Exit가 복원된 Receiver nf를 소비했습니다.
- 같은 Exit calldata 재호출은 거부됐습니다.
- Circuit 테스트에서 Sender secret 소비, 기존 owner-secret 방식 nf, opening 변조를 거부했습니다.

전체 기존 Tracing runner에 새 EventKind 9 decoder를 추가한 것은 아닙니다. 이 실험은 실제 저장·복호화한 부모와 미래 소비값을 실제 소비 기록에 대조하여 감사 연결을 확인합니다.

## 환경·준비·재현

GOMAXPROCS=8, GOMEMLIMIT=8GiB, Anvil 1.7.1, Prague, chain ID 31337, gas limit 30M입니다. 전용 Compose project는 `zkdpp-shared-transfer`, port는 18549이며 Anvil 자원은 2 CPU·4GiB입니다. 현재 node나 다른 project를 종료하지 않습니다.

기존 development canonical SRS·domain 32768/65536 Lagrange SRS로 **신규 3개 CCS·PK·VK만** 생성했습니다. 새 SRS를 생성하거나 기존 키를 덮어쓰지 않았습니다. Artifact 재로딩 때 checksum을 확인하고 Prove·Verify했습니다.

```sh
GOMAXPROCS=8 GOMEMLIMIT=8GiB GOCACHE="$PWD/.cache/go-build" go test ./internal/experiments -run '^TestSharedTransferOwnership$' -count=1
GOMAXPROCS=8 GOMEMLIMIT=8GiB GOCACHE="$PWD/.cache/go-build" go run ./cmd/shared_transfer -mode setup
sh contracts/shared-transfer/stage.sh
docker compose -f docker-compose.shared-transfer.yml -p zkdpp-shared-transfer run --rm foundry build
GOMAXPROCS=8 GOMEMLIMIT=8GiB GOCACHE="$PWD/.cache/go-build" go run ./cmd/shared_transfer -mode run
```

완료된 Setup·공식 결과는 덮어쓰기를 거부합니다. command는 새 환경에서의 순서이며 기존 결과가 있는 위치에서 재실행하지 않습니다.

## 원자료와 실패 이력

- `result.json`: 실제 transaction·receipt·calldata·Proof, 배포 runtime code hash, 비교 조건과 보호 checksum
- `attempt-*.json`: 시작·종료와 실제 실행 상태
- `verification.json`: 측정 뒤 source·Artifact·build metadata·기존 보호 파일 대조
- `artifacts/development/shared-transfer/`: CCS·PK·VK·generated verifier·Proof 원자료(기존 Git ignore 적용)

준비 단계에서 Docker socket sandbox 접근 1회가 거부되어 승인된 실행으로 전환했습니다. 최초 Solidity build는 외부 상대 import 경로로 실패했습니다. 동일 source의 전용 staging 사본으로 경로를 수정한 뒤 두 번째 build에서 성공했습니다. 이 실패들은 Proof 생성·공식 transaction 이전입니다. 단위 테스트 첫 실행은 통과했지만 전역 Go cache trim 권한 오류가 뒤따랐습니다. 프로젝트 내부 cache로 다시 실행하여 성공했습니다. Setup 1회·공식 direct 실행 1회이며 실패한 측정 transaction은 없습니다.
