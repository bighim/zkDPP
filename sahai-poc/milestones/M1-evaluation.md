# M1 - RSA 제거 및 DocHash-ID model

- Status: PASS
- active model/ABI/fixture에서 `DocPrime`, certificate, accumulator, membership/non-membership, RPoKE를 제거했다.
- `DocHash`가 salted document commitment와 ledger ID를 겸한다.
- `InDocs` direct provenance DAG는 유지한다.
- 모든 input은 한 번만 소비되며 duplicate output, consumed input, terminal input을 거부한다.
- accumulator 기반 upstream set query와 contamination proof가 제외된다는 한계를 문서화했다.
