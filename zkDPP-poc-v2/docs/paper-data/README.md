# 현재 원고의 수치와 재집계

현재 거래·증명 측정 판본은 **2026-10-16 run01**, 몫 다항식 조각의 추가 무작위화를 적용한 실행이다. 현재 원고의 회계 범위 보완은 이 수치를 바꾸지 않았다.

- [읽기용 현재 수치표](current-results.md)
- [공식 측정행](runs/2026-10-16-run01/measurements.json)
- [원자료 해시·환경·집계 기준](runs/2026-10-16-run01/source-manifest.json)
- [이전 공개 CSV와 안내](archive/b11b6e7/README.md)

| 원고 결과 | 현재 CSV | 집계 방식 |
| --- | --- | --- |
| 거래별 비용 | events.csv | 감사 포함, 준비 제외 공식 5회 중앙값과 최소·최대 |
| 감사 추가 비용 | audit-overhead.csv | 같은 조건의 포함/미포함 중앙값 차이 |
| 가공 입출력 수 | process.csv | 형상·감사 여부마다 공식 1회 |
| 전달 방식 | transfer-comparison.csv, transfer-routes.csv | 직접 전달 5회 중앙값; SharedSecret 1회; 두 단계는 감사 포함 개별 거래 중앙값의 합 |
| 동일 기능 Merge | same-merge.csv | 구성별 3프로세스 × 공식 10회, 합친 30회 중앙값 |
| A/B/C 보조 비교 | three-merge.csv | 구성별 3프로세스 × 공식 10회, 합친 30회 중앙값 |
| 공개 자료 사전 확인 후 키 복원 | key-recovery-prevalidated.csv | 기존 실행의 준비 1회·공식 1회, 내부 복원 1,000회 평균 유지 |
| 증명 없는 추적·동결 규모 | tracing.csv, freeze.csv | 기존 실행의 공식 5회 중앙값 유지 |

프로젝트 루트 `zkDPP-poc-v2`에서 다음 명령으로 수치만 다시 집계한다. 실험이나 증명 생성은 실행하지 않는다.

```sh
node docs/src/build-paper-results.cjs --assets-only
```

새 집계기는 위 공식 측정행을 읽고 CSV와 current-results.md를 재생성한다. 과거 output/experiments의 기본 설정 측정으로 현재 CSV를 덮어쓰지 않는다. transfer-routes.csv의 두 단계 행은 개별 중앙값의 합이므로 관측된 최소·최대 칸을 비워 두었다.

키 복원·증명 없는 추적/동결은 위에서 지정한 기존 실행 CSV를 유지한다. key-recovery.csv는 전체 package 검증을 포함한 별도 범위이며 현재 원고의 사전 확인 후 복원 표에는 key-recovery-prevalidated.csv를 사용한다. 기존 원자료 출처는 provenance.json의 preservedLegacy에 보존했다. docs/paper-evaluation-results.{md,tex,pdf}와 paper-figures/paper-tables의 기존 보고서·그림은 이전 실행 자료이며 현재 수치표와 구분한다. 동일 입력·회로라도 환경·설정·실행이 달라졌으므로 전후 시간 차이를 옵션 하나의 효과로 해석하지 않는다.
