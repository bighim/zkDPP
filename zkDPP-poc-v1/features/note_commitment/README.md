# Note Commitment

## 목적

Note의 비공개 내용을 공개 commitment `cm` 하나로 묶습니다. 이 기능은 Tree를 갱신하거나 Note를 소비하지 않습니다.

## 공개값과 비공개값

| 공개값 | 비공개값 |
|---|---|
| `cm` | DocumentHash, AssetRole, State, address, opening |

## 검증 관계

- State 세 값은 `uint64`입니다.
- `a_rec <= q_mass`입니다.
- AssetRole은 ELIGIBLE 또는 WASTE입니다.
- WASTE는 `a_rec=0`, `e=0`입니다.
- 비공개 Note로 계산한 commitment가 public `cm`과 같습니다.

## 현재 한계

Entry·Exit, Tree update, nullifier 중복과 Contract는 M1 범위가 아닙니다.
