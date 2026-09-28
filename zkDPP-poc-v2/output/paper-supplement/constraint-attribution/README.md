# Transfer / Proceed / Recall Constraints 분리

기존 Circuit을 수정하지 않고 gnark profiler로 6개 구성을 각각 한 번 compile했습니다.
Go 1.25.7, gnark 0.15.0, BLS12-381, SCS, GOMAXPROCS=8을 사용했습니다.
SRS·Setup·Proof·Anvil·시간 측정은 수행하지 않았습니다.

## Transfer와 Proceed의 차이 — Audit 없음

| 구성요소 | Transfer | Proceed | 차이 |
|---|---:|---:|---:|
| 입력 membership | 9,825 | 9,825 | 0 |
| Note 유효성·범위 검사 | 7,140 | 3,570 | 3,570 |
| Note commitment | 4,214 | 2,107 | 2,107 |
| Voucher 유효성·범위 검사 | 3,698 | 3,698 | 0 |
| Voucher commitment | 2,709 | 2,709 | 0 |
| 비례 배분 ratio 검증 | 6,880 | 0 | 6,880 |
| Note·Voucher nullifier | 602 | 903 | -301 |
| owner address | 301 | 301 | 0 |
| 기타 관계·compiler | 276 | 10 | 266 |
| **합계** | **35,645** | **23,123** | **12,522** |

Transfer는 입력 Note와 Change Note를 검증합니다.
Proceed는 출력 Note 하나를 검증합니다.
두 ratio 호출은 재활용 질량·탄소의 비례 배분과 관련된 결과·나머지 범위 및 관계를 검증합니다.
6,880개는 이 두 호출에 귀속된 합계이며, 전체 State 검증 비용은 아닙니다.

## Audit이 있는 전체 구성

| 구성요소 | Transfer | Proceed | Recall |
|---|---:|---:|---:|
| Audit 없음 전체 | 35,645 | 23,123 | 23,252 |
| Audit 추가로 계산한 출력 소비값 | 1,505 | 602 | 602 |
| Audit scalar multiplication 두 번 | 8,611 | 8,611 | 8,611 |
| Audit 키·mask Hash | 2,399 | 1,801 | 1,801 |
| Audit 난수·암호문 등 나머지 검사 | 1,723 | 1,721 | 1,721 |
| **Audit 있음 전체** | **49,883** | **35,858** | **35,987** |

## 측정 경계와 재현

`constraints.json`과 `.pprof`는 실제 compile 결과입니다.
각 Constraints를 호출 stack에 따라 한 범주에만 귀속시켰습니다.
모든 범주의 합이 최종 CCS Constraints와 같은지 실행 중 검사했습니다.
6개 최종 합계는 기존 Event별 Audit 비교 수치와 일치했습니다.

이는 **전체 Circuit 내부의 Constraints 귀속 분석**입니다.
연산을 제거해 다시 compile하는 ablation과 같지는 않습니다.
Compiler의 공통 연산 재사용 때문에 범주별 값은 독립 gadget 비용과 다를 수 있습니다.
Gas·Prove 시간의 비중을 나타내지 않습니다.

새 디렉터리 이름으로 재현합니다. 기존 디렉터리는 덮어쓰지 않습니다.

```sh
GOCACHE="$PWD/.cache/go-build" GOTOOLCHAIN=go1.25.7 GOMAXPROCS=8 \
  go run ./cmd/profile_event_constraints .codex-build/constraints-attribution-next
```

Compile 6건은 모두 성공했습니다.
이후 선택적 `go tool pprof -list` 조회는 설치된 toolchain에 pprof 실행 도구가 없어 실패했습니다.
JSON 분류는 Go pprof parser로 이미 완료되었으며 원시 profile을 보존했습니다.
기존 결과 보고서는 수정하지 않았습니다.
