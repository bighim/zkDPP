# Split

private Note를 질량 비례 State의 Note 두 개로 나눕니다. Output 2를 내림 계산하고 output 1이 residual을 받으며 zero-State output 하나를 허용합니다.

| 공개값 | 비공개값 |
|---|---|
| `noteRoot`, `nf`, `cmOut1`, `cmOut2` | input Note·`cm`·path, owner secret, output Note 2개와 remainder |
