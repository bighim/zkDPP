# 논문 문서 편집 규칙

- 사용자가 직접 수정한 해석·수치·표현은 요청 범위 밖에서 바꾸지 않습니다.
- 현재 보고서의 Figure는 본문 → `paper-modules/*.tex` 한 단계로 유지합니다.
- 현재 보고서 또는 Figure TeX를 수정한 뒤에는 `docs/`에서 `node src/format-paper-tex.cjs`를 실행합니다.
- LaTeX Workshop과 같은 `latexindent`, 공백 4개를 사용합니다. `-m`으로 문장 줄바꿈을 재구성하지 않습니다.
- PDF 빌드는 `node src/build-paper-pdf.cjs`를 사용합니다. 이 명령은 포맷 후 빌드하며 본문을 재생성하지 않습니다.
- 개별 Figure 빌드는 `node src/build-paper-module.cjs <name>`을 사용합니다. 해당 Figure를 포맷한 뒤 빌드합니다.
- 최종 PDF는 반드시 `docs/` 아래에 저장하고 렌더링을 확인합니다.
- 과거 milestone 문서·결과와 사용하지 않는 이전 Figure 파일을 일괄 포맷하지 않습니다.
- Git hook이나 Husky에 의존하지 않습니다. 외부 파일 수정은 VS Code의 format-on-save를 호출하지 않으므로 수정 뒤 CLI formatter를 실행해야 합니다.
