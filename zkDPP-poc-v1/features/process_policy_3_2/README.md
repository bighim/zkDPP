# Process Policy 3-to-2

같은 ZK owner의 ELIGIBLE Note 3개를 고정 Policy rate·allocation으로 ELIGIBLE Note와 WASTE Note로 변환합니다.

| 공개값 | 비공개값 |
|---|---|
| `policyRef`, `policyScopeRef`, `noteRoot`, `nf[3]`, `cmOut[2]` | input Note·`cm`·path, owner secret, State·delta·remainder, output Note |

Policy는 질량 손실률 6.25%, input kg당 추가 탄소 0.09375 kgCO2e, output 질량 90:10과 탄소·재활용 attribution의 ELIGIBLE 100% 귀속을 고정합니다.
