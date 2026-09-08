# 공통 테스트 계정

이 폴더는 로컬 테스트넷의 고정 계정처럼 모든 milestone과 시나리오가 반복해서 사용하는 zkDPP 테스트 identity를 보관합니다.

## 어떤 파일이 기준인가요?

[`actors-v1.json`](actors-v1.json)이 고정 테스트 계정의 유일한 기준입니다. Go 코드나 Feature별 fixture에 같은 secret과 address를 다시 상수로 복사하지 않습니다.

| ID | `sk_owner` | `address` |
|---|---:|---:|
| `actor-1` | 11 | 26254203021904445333071000515551661317516843163551302036285354975946208841425 |
| `actor-2` | 22 | 47528018706149194494104010846260978829570693297937760136305455896841183604262 |
| `actor-3` | 33 | 49848565238528487643025532610542856695466309323924862924778441679757633499152 |
| `actor-4` | 44 | 40596730928840390204709665476034893252894760769645211207063750926071532213730 |
| `actor-5` | 55 | 49991025685611962253864790410137325962643753898623019661476626077140422570340 |

각 address는 다음 관계로 계산한 BLS12-381 scalar field 값입니다.

$$
\mathrm{address}
=
H\left(
\mathrm{OwnerTag},
\mathrm{sk}_{\mathrm{owner}}
\right)
$$

`OwnerTag` 문자열은 `zkDPP:Owner:v1`입니다.

## 왜 역할 이름을 넣지 않나요?

`actor-1`은 특정 Supplier나 Manufacturer로 고정되지 않습니다. Identity와 공급망 역할을 분리해야 같은 계정을 서로 다른 기능 POC에서 재사용할 수 있습니다.

각 시나리오는 필요한 역할만 별도로 연결합니다.

```text
EntryIssuer  → actor-1
Supplier     → actor-2
Processor    → actor-3
Manufacturer → actor-4
Auditor      → actor-5
```

이 역할 연결은 예시일 뿐이며 `actors-v1.json`에는 저장하지 않습니다.

## Loader는 무엇을 확인해야 하나요?

M1에서 공통 fixture loader를 만들 때 다음을 확인합니다.

1. `profile`, curve, Hash와 `OwnerTag`가 기대한 값과 같습니다.
2. `skOwner`와 `address`는 10진수 문자열이며 BLS12-381 scalar field 원소로 변환할 수 있습니다.
3. `skOwner`는 0이 아닙니다.
4. 모든 ID, `skOwner`와 `address`가 서로 중복되지 않습니다.
5. 각 address를 `skOwner`에서 다시 계산한 결과가 JSON 값과 같습니다.

검증에 실패하면 일부 계정만 사용하지 않고 fixture 전체 로드를 실패시킵니다.

## 보안상 주의할 점은 무엇인가요?

이 계정은 비밀이 아닙니다. 작은 고정 secret을 사용하므로 다음 용도로만 사용합니다.

- 일반 Go 계산과 회로 결과 비교
- local Anvil·Besu 실험
- 재현 가능한 scenario와 benchmark

Production, 공개 테스트넷, 실제 자산 또는 실제 참여자의 identity에 사용하면 안 됩니다. 이 값은 EVM transaction을 서명하는 Anvil 계정의 private key와도 관계가 없습니다.

Hash 관계나 Domain이 바뀌면 기존 값을 조용히 덮어쓰지 않습니다. 새 profile과 새 버전 파일을 만들고 관련 milestone 문서에서 변경 이유를 기록합니다.
