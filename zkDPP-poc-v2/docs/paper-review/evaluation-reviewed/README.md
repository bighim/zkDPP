# Evaluation 검토 반영본

`paper-evaluation-revised.pdf`의 내용을 편집 가능한 TeX로 재구성하고, 보존된 실험 근거를 반영했습니다.
페이지 배치를 그대로 복제한 전사본은 아닙니다.
원본은 `original.pdf`에 보존했습니다.

## 편집과 빌드

`main.tex`가 `01.Methods.tex`부터 `11.Change-Log.tex`까지 직접 불러옵니다.
각 파일의 표와 그래프도 TeX로 편집할 수 있습니다.
원본 대비 변경 사항은 PDF 마지막에 있습니다.

```sh
node build.cjs
```

위 명령은 해당 폴더에서 실행합니다.
latexindent로 공백 4개 포맷을 적용한 뒤 XeLaTeX로 두 번 빌드합니다.
최종 PDF는 이 폴더의 `main.pdf`입니다.

## 근거와 범위

- 수치의 근거 경로는 `10.Sources.tex`에 있습니다.
- 생성 당시 원자료 checksum은 `data-provenance.json`에 있습니다.
- 기존 5회 median, 보충 실험 1회, 키 복구 묶음 평균을 구분했습니다.
- 키 복구에서 n 변화 비교는 제외했지만, n=10 고정 조건은 남겼습니다.
- 새로운 실험은 실행하지 않았습니다.
- 관련 연구 논의는 비교 조건에 관한 설명이며, 외부 시스템을 재실행한 결과가 아닙니다.

`prepare.cjs`는 최초 변환용 생성기입니다.
수정한 TeX를 보존하기 위해 기존 파일을 덮어쓰지 않습니다.
이후 문서 수정은 TeX에 직접 반영하고 `build.cjs`로 빌드합니다.
