# M2 - Poseidon2/SHA-256 independent gadgets

- Status: PASS
- PLONK-KZG/BLS12-381의 MerklePath, Eq, Add, And를 profile별 독립 circuit/key/verifier로 생성했다.
- native root와 circuit root, Solidity verifier positive test가 통과했다.
- bad path/root/salt/equality/sum/overflow 및 cross-profile proof를 거부한다.
- ST 1회와 MT 1회 결과를 남겼다. ST는 논문 Table III 진단용일 뿐 공식 Event 비교가 아니다.
- 최종 Foundry Gate: 6 tests passed, 0 failed (suite 전체 1회).
