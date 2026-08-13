# M3 - Event payload와 연결 시나리오

- Status: PASS
- Entry/Ship 각 2 case, Merge/Split/Process/Exit 각 1 case인 연결 8-transaction DAG를 구현했다.
- proof count는 Entry 2, Ship 5, Merge/Split/Process 6, Exit 2다.
- Poseidon2/SHA-256, canonical/independent fixture를 MT로 각각 1회 측정했다.
- 각 output `DocHash`가 다음 `inputId`/`InDocs`에 연결되고 수량/participant 관계가 자동 검증된다.
- 독립 fixture는 component diagnostic에만 쓰며 공식 zkDPP 비율에는 포함하지 않는다.
