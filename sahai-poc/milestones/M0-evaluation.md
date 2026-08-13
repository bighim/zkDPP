# M0 - RSA-free conformance specification

- Status: PASS
- `[Paper]`, `[Adaptation]`, `[Experiment]`를 분리했다.
- 6개 Event의 input/output, predicate, proof count를 manifest에 고정했다.
- 32 attributes, 8 leaves, 3 siblings 구조와 Poseidon2/SHA-256 encoding을 명시했다.
- Exit은 terminal output을 만들고 `Sender == Recipient`를 Path 1 + Eq 1로 검증한다.
- 공식 Event 비교는 MT-only, hardened-only, 조합당 1회로 고정했다.
