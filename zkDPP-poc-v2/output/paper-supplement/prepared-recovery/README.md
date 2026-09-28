# 사전 검증 후 k개 share만 사용하는 Master Key 복구

기존 실험 8은 매 복구마다 n명 전체의 public share와 전체 digest를 다시 검사했습니다.
이 결과를 복구 자체에 필요한 n 증가 비용으로 해석하면 안 됩니다.
기존 source `internal/experiments/crypto.go`와 Raw `output/experiments/experiment-8.json`은 그대로 보존했습니다.

## 변경한 측정 경계

- 타이머 밖: 신뢰할 수 있는 출처에서 받은 Committee 공개 정보를 복사·검증해 보관합니다.
- 타이머 안: 받은 k개 share의 ID·session·digest·범위·public share 일치 검증, Lagrange 보간, 복구 공개키 대조입니다.
- 검증된 Committee 객체를 통해서만 복구합니다. 원본 slice를 수정해도 검증된 복사본은 변하지 않습니다.
- 공개점·digest 검사는 출처 인증을 대신하지 않습니다. 공개 정보를 신뢰할 수 있게 확보하는 절차는 실험 외부 가정입니다.
- 각 복구는 k개의 public share만 직접 인덱스로 조회합니다. n명 순회와 전체 digest 재계산은 하지 않습니다.
- scalar best-effort zeroize가 내부 반복에 포함됩니다. 암호학적 완전 삭제를 주장하지 않습니다.

## 실행 조건

Go 1.25.7, GOMAXPROCS=8, 기존 gnark-crypto/Jubjub 구현.
각 case에서 warm-up 1묶음 후 공식 1묶음을 실행했습니다. 한 묶음은 1,000회입니다.
그래프 값은 공식 묶음 전체 wall-clock을 1,000으로 나눈 평균 시간입니다. 5회 median이 아닙니다.
2-of-10은 두 그래프에서 같은 측정값을 사용합니다.
n 비교에서는 같은 공개키·동일한 두 secret share·동일한 위원 ID(1,2)를 유지합니다.
k 비교에서는 각 threshold에 맞는 유효한 Shamir 다항식으로 share를 생성합니다.
키 생성·DKG·통신·승인 대기는 측정하지 않았습니다.

## 검증

`go test ./internal/experiments -run '^TestPreparedRecovery$' -count=1` 통과.
서로 다른 위원 조합의 동일 복구, 기존 함수와 결과 일치, 부족한 share·중복 ID·잘못된 session/digest·변조 share·범위 밖 share·위원 ID 거부를 검사했습니다.
잘못된 public point는 준비 단계에서 거부했습니다.
source public-share slice 변조가 검증된 복사본에 영향을 주지 않음을 확인했습니다.

## 해석

구현상 share 검증은 k회, 보간은 k에 대한 이중 반복, 최종 공개키 검사는 1회입니다.
n은 위원 ID 범위 확인과 배열 인덱스 조회에 사용되며, 타이머 안에 n회 순회는 없습니다.
따라서 k가 고정되면 n 전체 검증에 따른 연산량 증가는 제거됩니다.
실측값은 실행 환경·측정 변동을 포함하며, 공식 1회만으로 차이가 0이라는 통계적 결론을 내리지 않습니다.
현재 n 증가 순서로 실행했으므로 실행 순서 효과와 n 자체의 영향을 wall-clock 값만으로 분리하지 못합니다.

## 재현

```sh
GOMAXPROCS=8 GOCACHE="$PWD/.cache/go-build" GOTOOLCHAIN=go1.25.7 \
  go test ./internal/experiments -run '^TestPreparedRecovery$' -count=1
GOMAXPROCS=8 GOCACHE="$PWD/.cache/go-build" GOTOOLCHAIN=go1.25.7 \
  go run ./cmd/benchmark_prepared_recovery
```

출력 디렉터리가 있으면 재실행을 거부합니다. 실패·재시도 없이 공식 실행을 완료했습니다.
`result.json`에 준비 비용·warm-up·공식 측정·단계별 시간과 명시적 경계를 보존합니다.
