# zkDPP 논문 장별 TeX 전사본

원본: `zkdpp-v0.2-260916.pdf` (18쪽).
원본 파일의 복사본은 `source.pdf`, SHA-256은 `conversion-manifest.json`에 있습니다.

## 편집 구조

```text
main.tex
├─ 00.Abstract.tex
├─ 01.Introduction.tex
├─ 02.Related-Work.tex
├─ 03.System-and-Trust-Model.tex
├─ 04.Objects-and-Ledger-State.tex
├─ 05.Transitions-and-Accounting.tex
├─ 06.Public-DPP-Verification.tex
├─ 07.Tracing-and-Freezing.tex
├─ 08.Security-Requirements.tex
├─ 09.Security-Proofs.tex
├─ 10.Model-Validation.tex
├─ 11.Scope-and-Limitations.tex
├─ 12.Conclusion.tex
└─ 13.References.tex
```

각 장은 하위 TeX 파일 없이 자체 내용을 포함합니다.
1.1, 1.2 등의 하위 절은 해당 장 파일 안에서 편집합니다.
본문·수식 27개·표 5개는 TeX이며, 그림 1은 원본 PDF 영역을 잘라 넣은 벡터 이미지입니다.
그림의 내부 도형을 수정하려면 추후 TikZ 등으로 별도 재작성해야 합니다.
참고문헌은 원문 인용 번호를 유지한 `13.References.tex`입니다. 확인되지 않은 BibTeX 필드나 DOI를 만들어 넣지 않았습니다.

## 빌드

이 문서는 Unicode 수식을 사용하므로 **XeLaTeX**로 빌드합니다.
`main.tex`의 `% !TEX program = xelatex`와 각 장의 `% !TEX root = main.tex`로 LaTeX Workshop에 엔진과 root를 지정했습니다.
프로젝트의 `.latexmkrc`도 기본 엔진을 XeLaTeX로 선택합니다.
명시적으로 `latexmk -pdf`를 사용하면 pdfLaTeX를 강제할 수 있으므로, CLI에서는 `latexmk -xelatex main.tex` 또는 아래 명령을 사용합니다.

```sh
node build.cjs
```

장별 파일을 latexindent(공백 4개)로 포맷한 뒤 XeLaTeX로 두 번 빌드합니다.
에디터와 스크립트는 모두 저장소의 `.latex-build/zkDPP-poc-v2/docs/paper-review/zkdpp-v0.2-260916/`에 빌드합니다.
LaTeX Workshop 미리보기는 이 PDF를 사용합니다. `build.cjs`는 같은 PDF를 이 폴더의 `main.pdf`에도 복사합니다.
한글 본문은 Nanum Myeongjo, 수식은 Latin Modern Math를 사용합니다.

## 복원 범위와 검토 기준

- 저자의 원본 TeX를 받은 것이 아니라 PDF로부터 재구성한 **검토용 전사본**입니다.
- 원본의 두 단을 읽기 순서에 맞춰 복원하고 1단 편집으로 변경했습니다. 원본과 페이지 수는 다릅니다.
- 본문 파일의 `% Source PDF page ...` 주석으로 원본 위치를 찾을 수 있습니다.
- 수식 번호와 표 번호, 문헌의 원문 인용 번호를 유지했습니다.
- 정책·키 구조·용어를 현재 POC에 맞춰 변경하지 않았습니다. 불일치는 검토 대상이지 전사 중 수정할 내용이 아닙니다.
- PDF의 줄 끝에서 나뉜 단어, 인라인 수식의 첨자·기호는 원본과 대조하며 윤문해야 합니다. 빌드 성공이 문장·보안 증명의 검증 완료를 뜻하지 않습니다.
- 번호가 붙은 수식은 직접 옮겼습니다. 해석을 변경하는 제안은 원문 보존본과 분리해서 논의합니다.

앞서 만든 `../introduction-review.md`는 서론의 대화용 검토 메모입니다.
`../convert_pdf_chapters.py`는 최초 전사 기록용 도구입니다. **장별 수동 편집 이후 다시 실행하지 마세요.**
