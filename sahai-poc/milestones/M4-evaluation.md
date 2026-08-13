# M4 - Solidity verifier와 profile binding

- Status: PASS
- Poseidon2와 SHA-256의 4개 verifier를 고유 contract name으로 생성했다.
- 같은 profile의 golden proof는 Go/Solidity에서 통과하고 다른 profile 조합은 실패한다.
- 256-bit SHA digest는 128-bit limb 2개, Poseidon2 digest는 BLS12-381 field element 1개로 공개한다.
- RSA arithmetic, Pocklington, MODEXP, RPoKE contract는 공식 구현과 ABI에 존재하지 않는다.
