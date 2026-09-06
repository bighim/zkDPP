# Recall

Sender가 private Voucher의 membership·회수 권한과 `currentEpoch < deadlineEpoch`을 증명해 같은 State의 Sender Note를 만듭니다.

| 공개값 | 비공개값 |
|---|---|
| `voucherRoot`, `rvnf`, `cmReturn`, `currentEpoch` | Voucher·`rv`·opening·private deadline·path, Sender secret, output Note |

소비 `rv`와 deadline은 private witness입니다. Deadline과 같거나 지난 Recall은 실패합니다.
