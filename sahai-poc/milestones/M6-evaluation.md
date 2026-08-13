# M6 - Anvil MT benchmark와 zkDPP-POC 비교

- Status: PASS
- Poseidon2/SHA-256 fixed-proof gas를 profile별 1회 측정했다.
- Poseidon2/SHA-256 fresh-proof MT E2E를 profile별 1회 측정했다.
- `receipt.gasUsed`와 build/witness/prove/encode/submit-to-receipt 경계를 분리했다.
- 기존 `poc-v2`는 임시 복사본에서 Poseidon2 MT gas/E2E를 1회 실행했다.
- 공식 비교 CSV는 양쪽 실제 값과 case 수를 먼저 제시하고 마지막 열에 `Sahai/zkDPP` 비율을 둔다.
- `proceed`, `recall`은 unmatched relation으로 비교에서 제외한다.
