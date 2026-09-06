# Transfer

Private Note를 한 번 소비해 public Voucher commitment와 Change Note commitment를 만듭니다. 전달 질량은 private이고 `a_rec`, `e`는 질량 비례로 배분하며 residual은 Voucher가 받습니다.

| 공개값 | 비공개값 |
|---|---|
| `noteRoot`, `nf`, `rvNew`, `cmChange`, `transferEpoch`, `deltaEpoch` | input Note·`cm`·path·Sender secret, Receiver address, 두 output과 remainder |

ELIGIBLE·WASTE를 모두 허용하고 DocumentHash·AssetRole을 유지합니다. 전량 Transfer도 zero-State Change Note를 생성합니다. 운송 탄소는 포함하지 않습니다.
