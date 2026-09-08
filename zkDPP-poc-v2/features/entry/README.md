# Entry Circuit

## 목적

신뢰된 EntryIssuer가 공급망 시작점의 ELIGIBLE Note를 만들었음을 증명합니다.

| 공개값 | 비공개값 |
|---|---|
| `cm` | DocumentHash, AssetRole, State, address, opening |

## 검증 조건

- `q_mass`, `a_rec`, `e`는 uint64입니다.
- `q_mass > 0`입니다.
- `a_rec <= q_mass`입니다.
- AssetRole은 ELIGIBLE입니다.
- 초기 `e`는 0 이상 uint64이며 0보다 클 수 있습니다.
- 비공개 Note로 계산한 commitment가 공개 `cm`과 같습니다.

실제 물품과 초기 State의 진실성은 EntryIssuer의 외부 확인에 의존합니다.
