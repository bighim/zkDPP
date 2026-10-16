# 논문 초안의 데이터와 그림

기존 `output/experiments/`의 실험 1·2·3·4·8과 Process 보충 결과를 사용합니다.
Process는 `output/paper-supplement/process-independent.json`의 조건별 공식 1회 값입니다.
다른 항목의 기존 5회 median과 통계를 혼합하지 않습니다.

- 원본 출처와 SHA-256: `provenance.json`
- 표·그래프 집계값: 이 폴더의 CSV
- 그래프 원본: `../paper-figures/*.tex` — PGFPlots
- 본문: `../paper-evaluation-results.md`, `../paper-evaluation-results.tex`

## 다시 생성하기

프로젝트 루트 `zkDPP-poc-v2`에서 실행합니다.

```sh
node docs/src/build-paper-results.cjs --assets-only
mkdir -p .codex-build/paper-report
cd docs
xelatex -interaction=nonstopmode -halt-on-error paper-evaluation-results.tex
```

이 명령은 실험을 다시 실행하지 않습니다.
Raw를 검증하고 집계한 뒤 CSV와 그래프 TeX를 생성합니다.
`--assets-only`는 직접 편집한 본문 Markdown·TeX를 덮어쓰지 않습니다.
Markdown의 PNG는 각 `*-standalone.tex`를 XeLaTeX로 렌더링한 PDF에서 생성합니다.

위 명령 다음에 `docs` 폴더에서 실행합니다.

```sh
cd paper-figures
for name in audit-overhead tracing-time freeze-cost process-input-sweep process-output-sweep key-recovery; do
  xelatex -interaction=nonstopmode -halt-on-error \
    -output-directory=../../.codex-build/paper-report "${name}-standalone.tex" || exit 1
  pdftoppm -scale-to 1800 -png -singlefile \
    "../../.codex-build/paper-report/${name}-standalone.pdf" "$name" || exit 1
done
```

Node.js, XeLaTeX(kotex·PGFPlots·standalone), Poppler의 pdftoppm이 필요합니다.

## 해석 기준

- 대표값: Process는 공식 1회 관측값, 나머지는 warm-up을 제외한 공식 5회의 median
- 오차 막대는 표시하지 않습니다. min–max는 CSV에 보존합니다.
- Tracing·Freeze 그림은 Chain Split 한 계열만 표시합니다. 다른 구조의 Raw는 보존합니다.
- 키 복구: 각 run의 1,000회 묶음 시간을 1,000으로 나눈 복구 1회 시간
- Freeze Gas: 해당 run에서 제출한 여러 transaction의 합계
- Process: 출력 1개 고정·입력 2/3/4개, 입력 2개 고정·출력 1/2/3개를 독립 비교
- 현재 Process 그림은 `process-input-sweep`와 `process-output-sweep`입니다. 각 그림에 Constraints와 Gas를 나란히 표시합니다. 이전 Process 그림은 본문에서 사용하지 않습니다.
- 같은 그래프에서 연결된 선: 관측값의 연결이며 회귀식이나 외삽이 아님
