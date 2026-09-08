# Private Spend Kernel

## 목적

어떤 Note를 소비하는지는 숨기면서 Note Tree membership, 소유권과 nullifier를 함께 검증합니다.

## 공개값과 비공개값

| 공개값 | 비공개값 |
|---|---|
| `noteRoot`, `nf` | Note 전체, `sk_owner`, leaf index, Depth-32 siblings |

소비하는 `cm`, address와 leaf index는 public input이 아닙니다.

## 검증 순서

1. Note State와 AssetRole 조건을 확인합니다.
2. `sk_owner`로 address를 다시 계산합니다.
3. Note commitment를 다시 계산합니다.
4. private path로 `noteRoot` membership을 확인합니다.
5. `sk_owner`와 commitment로 public `nf`를 다시 계산합니다.

## 현재 한계

Contract의 duplicate-nullifier mapping과 Active/Frozen/Revoked 검증은 후속 milestone에서 조합합니다.
