# Proceed

Receiver가 private Voucher의 membership과 수신 권한을 증명해 같은 State의 Receiver Note를 만듭니다.

| 공개값 | 비공개값 |
|---|---|
| `voucherRoot`, `rvnf`, `cmReceiver` | Voucher·`rv`·opening·path, Receiver secret, output Note |

소비 `rv`는 숨기고 public `rvnf`로 한 번만 해결합니다. Deadline은 검사하지 않습니다.
