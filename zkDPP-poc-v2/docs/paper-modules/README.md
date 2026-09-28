# 논문 Figure 모듈

본문 `paper-evaluation-results.tex`는 설명과 모듈 호출만 관리합니다.
표·그래프의 내용과 배치는 해당 Figure 파일 하나에서 수정합니다.

```text
paper-evaluation-results.tex
  └─ paper-modules/<name>.tex     배치·제목·그래프·표를 모두 포함
```

모듈은 편집 가능한 LaTeX이며, 이미지로 평탄화하지 않습니다.
자동 float은 사용하지 않아 본문에서 지정한 위치를 유지합니다.
독립적으로 출력하거나 다른 논문의 figure 환경 안에 input할 수 있습니다.

| 모듈 | 내용 |
|---|---|
| environment | 실험 환경 |
| event-costs | Event별 기본 비용 |
| merkle-counts | Verify·Update 횟수 표 두 개 |
| transfer-constraints-breakdown | Transfer Constraints 구성 |
| transfer-gas-breakdown | 동일 Proof 재실행 trace로 분리한 Transfer Gas 구성 |
| transfer-tradeoff | 직접 전달·Voucher 비교 |
| audit-overhead | Audit 그래프와 수치 표 |
| transfer-audit-breakdown | Transfer Audit 추가 Constraints·Gas 실측 분해 |
| tracing-freeze | Tracing·Freeze 병렬 그래프와 각각의 수치 표·Note 1개 동결 Gas |
| process-input-sweep | 입력 증가 그래프와 수치 표 |
| process-output-sweep | 출력 증가 그래프와 수치 표 |
| key-recovery | Master Key 복구 그래프와 수치 표 |

`key-recovery`의 현재 값은 `output/paper-supplement/prepared-recovery/result.json`과
`docs/paper-data/key-recovery-prevalidated.csv`를 사용합니다.
사전 Committee 검증을 제외한 k개 share 검증·복구 실험이며 공식 1회(내부 1,000회 평균)입니다.
이전 `key-recovery.csv`의 전체 n명 검사 결과와 혼합하지 않습니다.

## 편집·빌드

Formatter는 VS Code LaTeX Workshop과 같은 `latexindent`(공백 4개)입니다.
외부 도구의 파일 수정에는 VS Code 저장 hook이 실행되지 않으므로 빌드 명령에 formatter를 연결했습니다.

```sh
node src/format-paper-tex.cjs         # 현재 보고서와 Figure 포맷
node src/format-paper-tex.cjs --check # 수정 없이 포맷 확인
node src/build-paper-pdf.cjs         # 포맷 후 전체 PDF 빌드
```

- 수치·열·행·축·선·범례·배치·제목: `paper-modules/`의 해당 파일
- 해석 문장·절 순서: 본문 TeX

`docs/`에서 개별 Figure를 빌드합니다.

```sh
node src/build-paper-module.cjs merkle-counts
```

PDF는 `docs/paper-modules/exports/merkle-counts.pdf`에 생성됩니다.
이 출력은 현재 본문과 같은 글꼴·페이지 폭을 쓰는 단독 미리보기입니다.
전체 문서는 기존 XeLaTeX 명령으로 빌드하고 최종 PDF를 `docs/`에 보관합니다.

기존 `build-paper-results.cjs` 전체 실행은 본문을 덮어쓸 수 있으므로 수동 편집 뒤 사용하지 않습니다.
`paper-figures/`와 `paper-tables/`는 과거 생성물로 보존하며 현재 본문에서는 참조하지 않습니다.
기존 수치 생성기를 실행해도 현재 Figure 모듈에는 자동 반영되지 않습니다.
Figure 모듈 안에는 다른 TeX 파일을 불러오는 input을 두지 않습니다.
Markdown은 읽기용 문서이며 표준 Markdown은 LaTeX input을 지원하지 않습니다.
Figure 배치의 기준은 본문 TeX와 이 모듈입니다.
