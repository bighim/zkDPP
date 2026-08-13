# M8 - 비교, 보고서, 재현 Gate

- Status: PROVISIONAL (result-format Gate PASS, protocol-conformance correction pending)
- PASS: RSA-free/Poseidon2 MT 중심의 한국어 Evaluation Report 생성.
- PASS: scenario, 용어, measurement boundary와 모든 표의 해석 설명 포함.
- PASS: 원 논문 비교를 마지막 section으로 이동하고 RSA Table II/IV를 제거됨으로 표시.
- PASS: XeLaTeX compile 및 전체 페이지 rendering에서 잘림/겹침/overfull 없음.
- PASS: `make validate-results`가 JSON -> CSV -> TeX, 비율 재계산, MT 1회 metadata, backend parity, active RSA-free artifact를 대조한다.
- BLOCKER: 현재 Merkle `LeafIndex`가 private witness이므로 Sahai의 public schema-position
  binding을 충족하지 않는다. Public input으로 변경하고 contract에서 leaf 0을
  강제한 후 circuit/key/verifier와 전체 benchmark를 재생성해야 M8을 최종 PASS로 바꾼다.
