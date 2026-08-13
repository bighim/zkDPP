# M5 - RSA-free Event contract integration

- Status: PASS
- Entry, Ship, Merge, Split, Process, Exit의 positive/negative Foundry test가 통과했다.
- 하나의 Event transaction이 필요한 독립 proof 전체와 state transition을 원자적으로 처리한다.
- proof 실패 시 partial state가 남지 않는다.
- duplicate `DocHash`, consumed input, terminal input, unregistered participant를 거부한다.
- paper mode/deployment flag는 없고 hardened semantics만 사용한다.
- 6개 Event 모두 30M block gas limit 안에 있다.
