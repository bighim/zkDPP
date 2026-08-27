# zkDPP v1 완료 설계 결정

- 상태: 논의 완료 항목 정리본
- 기준일: 2026-08-24
- 범위: 기존 우선순위 목록의 선결 결정과 완료된 1–3번
- 다음 논의: 우선순위 4번 — Event별 정확한 relation과 수치 표현

## 1. 문서 목적과 경계

이 문서는 현재까지 사용자와 논의하여 확정한 설계 결정만 모은다. 아직 논의가 끝나지 않은 사항을 추측하여 규격으로 만들지 않는다.

여기서 정하는 v1은 실제 공급망 전체를 그대로 상용화하는 완성 시스템이 아니라, 다음 질문에 답하기 위한 제한된 CoC(chain of custody) 증명 프로토타입이다.

> 신뢰된 지점에서 등록된 질량, 재활용 할당 크레딧, 누적 탄소 값이 이후의 비공개 전이에서 정해진 회계 규칙에 맞게 사용되었는가?

따라서 ZK 증명은 등록된 값 이후의 디지털 회계 일관성을 보장한다. 센서 측정값, 실제 물질, 실제 공정과 등록값이 일치하는지는 보장 범위 밖이다.

## 2. 완료 상태 요약

| 구분 | 결정 | 중요도 | 상태 |
|---|---|---:|---|
| 선결 결정 | 수량을 `q_mass` 하나로 통일하는 mass-only 모델 | 5/5 | 완료 |
| 우선순위 1 | 재활용 할당 크레딧 `a_rec`의 의미와 생명주기 | 5/5 | 완료 |
| 우선순위 2 | 누적 탄소 `e`의 의미, 경계와 생명주기 | 5/5 | 완료 |
| 우선순위 3 | 최소 Note 스키마와 material 범위 | 5/5 | 완료 |

## 3. 선결 결정: mass-only 수량 모델

### 3.1 결정

v1의 유일한 수량 기준은 질량 `q_mass`이다. 기존 POC의 item-count `Quantity`와 `State.total_kg`처럼 같은 Note 안에 개수와 질량을 중복해서 두지 않는다.

개념적인 Note 상태는 다음 세 값으로 구성한다.

```text
(q_mass, a_rec, e)
```

- `q_mass`: Note가 나타내는 물량의 질량
- `a_rec`: 그 Note에 할당된 재활용 mass-balance 크레딧의 절대 질량
- `e`: 그 Note에 귀속된 추적 경계 내 누적 탄소배출량

Transfer, Split, Merge, Process와 sustainability 계산은 모두 `q_mass`를 기준으로 한다.

### 3.2 선택 이유

- 현재 연구 대상은 배터리 그 자체가 아니라 일반적인 mass-balance CoC 증명이다.
- 재활용 비율과 질량 기반 탄소 집약도는 모두 질량을 기준으로 계산할 수 있다.
- 개수와 질량을 동시에 유지하면 두 값의 일치 관계, 단위 변환, 추가 range constraint가 필요하다.
- 현재 목표에는 per-item claim이나 개별 제품 단위의 재고·리콜 증명이 필요하지 않다.

### 3.3 보장하지 않는 것

v1은 다음을 증명하지 않는다.

- 제품 또는 용기의 개수
- cell, coil, container와 같은 단위별 재고
- 개별 serial 또는 unit-level identity
- 개수 기준 claim이나 개별 단위 리콜

이 정보가 DPP 메타데이터에 존재할 수는 있지만, v1의 핵심 ZK 회계 상태에는 포함하지 않는다.

## 4. 우선순위 1 완료: `a_rec` 생명주기

### 4.1 의미

`a_rec`은 Note에 할당된 재활용 mass-balance 크레딧의 절대 질량이다. 이는 해당 Note 안에 실제 재활용 물질이 그만큼 물리적으로 들어 있다는 뜻이 아니다.

모든 live Note는 다음 local bound를 만족한다.

```text
0 <= a_rec <= q_mass
```

따라서 하나의 Note에는 그 Note의 총질량보다 많은 재활용 크레딧을 할당할 수 없다.

### 4.2 Event별 규칙

| Event | `a_rec` 규칙 |
|---|---|
| Entry | 신뢰된 issuer가 초기 `q_mass`와 `a_rec`을 등록한다. 회로는 범위와 commitment를 검사하지만 물리적 진실성과 최초 발행의 정당성은 issuer 신뢰 가정이다. |
| Transfer | 입력 Note의 크레딧을 이전 물량과 잔여 물량의 질량에 비례하여 배분한다. |
| Proceed | 후속 Note로 크레딧을 변경 없이 전달한다. |
| Recall | 복구되는 Note로 크레딧을 변경 없이 전달한다. |
| Merge | 입력 Note들의 크레딧을 합산한다. |
| Split | 출력 질량에 비례하여 크레딧을 배분하며, 출력의 합은 입력 크레딧과 같아야 한다. |
| Process | 승인된 policy가 출력 간 재배분을 정의할 수 있지만 크레딧을 새로 만들 수는 없다. |
| Exit | Note와 그 Note에 남은 크레딧을 terminal하게 소비한다. |
| Issue | Note를 한 번 소비하여 claim을 만든다. 정확한 claim 형식과 binding은 우선순위 6에서 정한다. |

Process의 aggregate credit relation은 다음과 같다.

```text
sum(a_rec_in) = sum(a_rec_out) + a_retired
a_retired >= 0
```

여기서 `a_retired`는 더 이상 live Note에 배분되지 않는 크레딧이다. 이것이 곧 물리적 질량 손실을 의미하는 것은 아니며, 특정 Process policy가 필요하면 더 강한 관계를 추가할 수 있다.

Transfer와 Split은 단순한 물량 분할이므로 비례 배분만 허용한다. 크레딧을 특정 출력에 집중시키는 비비례 배분은 명시적인 Process policy가 검사하는 경우에만 허용한다.

### 4.3 이 결정이 방지하는 것

- 한 Note 안에서 질량보다 많은 크레딧을 주장하는 것
- 하나의 Split에서 입력보다 많은 총 크레딧을 출력하는 것
- 여러 입력을 Merge 또는 Process한 뒤 보유 총량보다 많은 크레딧을 만드는 것
- 이미 소비한 Note를 다시 사용해 동일 크레딧을 반복 지출하는 것

마지막 공격은 Note commitment 자체의 nullifier와 one-time consumption으로 막는다. 다만 Entry 이전의 동일 물리량에 대한 중복 발행은 암호학적으로 확인하지 않고 신뢰된 issuer 가정에 둔다.

## 5. 우선순위 2 완료: 누적 탄소 `e` 생명주기

### 5.1 의미와 시스템 경계

`e`는 해당 Note에 귀속된 절대 누적 탄소배출량이며, policy가 정한 고정 단위의 scaled `kgCO2e`로 표현한다.

v1의 탄소 추적 경계는 Entry부터 현재 Note까지이다.

```text
e_entry = 0
```

따라서 Entry 이전의 원료 채취, 전처리 또는 embodied carbon은 포함하지 않는다. 이 모델은 full product carbon footprint가 아니다.

### 5.2 Event별 규칙

| Event | `e` 규칙 |
|---|---|
| Entry | `e = 0`으로 시작한다. |
| Transfer | 입력의 누적 탄소를 질량 비례로 이전 물량과 잔여 물량에 배분하고, 이전되는 물량에는 비공개 비음수 운송 배출량 `delta_e_transport`를 더한다. |
| Proceed | 누적 탄소를 변경 없이 전달한다. |
| Recall | 복구되는 Note로 누적 탄소를 변경 없이 전달한다. |
| Merge | 입력 Note들의 누적 탄소를 합산한다. |
| Split | 출력 질량에 비례하여 누적 탄소를 배분하고 총량을 보존한다. |
| Process | 입력 누적 탄소의 합에 비공개 비음수 공정 배출량 `delta_e_process`를 더한 후 출력 질량에 비례하여 배분한다. |
| Exit | 최종 Note를 terminal하게 소비한다. |
| Issue | 최종 Note를 한 번 소비하여 claim을 만든다. claim profile과 public binding은 우선순위 6에서 정한다. |

Process의 aggregate carbon relation은 다음과 같다.

```text
E_total = sum(e_in) + delta_e_process
delta_e_process >= 0
sum(e_out) = E_total
```

프로토타입에서는 Process의 출력별 탄소 배분도 출력 질량 비례 방식을 사용한다. 생산용 시스템에서 co-product allocation 등 다른 방식이 필요하면 해당 policy용 relation으로 교체할 수 있다.

### 5.3 claim에서의 해석

탄소 값은 절대량으로 상태에 저장하지만, 질량 기준 탄소 집약도 claim은 나눗셈 없이 다음과 같이 검사할 수 있다.

```text
e <= tau_carbon * q_mass
```

여기서 `tau_carbon`은 허용 탄소 집약도이다. 이 식의 정확한 고정소수점 표현, public input 구성과 Issue binding은 아직 완료되지 않은 후속 항목이다.

### 5.4 신뢰 경계

`delta_e_transport`와 `delta_e_process`는 신뢰된 운영 입력이다. ZK 회로는 입력된 값이 음수가 아니고 누적·배분 규칙에 맞게 사용됐는지는 증명하지만, 측정 장치나 외부 데이터 원천이 실제 배출량을 정확히 보고했는지는 증명하지 않는다.

## 6. 우선순위 3 완료: 최소 Note 스키마와 material 범위

### 6.1 v1 Note commitment

v1의 개념적인 Note commitment는 다음과 같다.

```text
cm = H(
  "zkDPP:Note:v1",
  DocumentHash,
  q_mass,
  a_rec,
  e,
  owner,
  opening
)
```

각 항목의 의미는 다음과 같다.

- domain separator `"zkDPP:Note:v1"`: 다른 스키마의 commitment와 혼동되지 않도록 버전을 고정한다.
- `DocumentHash`: off-chain DPP 또는 업무 문서를 opaque하게 binding한다. 회로는 문서 내용을 해석하지 않는다.
- `q_mass`, `a_rec`, `e`: v1의 핵심 private accounting state이다.
- `owner`: Note를 사용할 수 있는 주체에 대한 binding이다.
- `opening`: commitment hiding과 Note 고유성에 사용하는 비밀값이다.

구체적인 serialization, hash primitive와 field encoding은 우선순위 4에서 정한다. schema 또는 policy version은 prover가 임의로 고르는 mutable witness가 아니라 domain separator와 승인된 circuit/policy 식별자를 통해 고정한다.

### 6.2 v1에서 제외하는 필드

다음 값은 v1 Note core에 넣지 않는다.

- 별도의 item-count `quantity`
- `unit`
- `materialType`
- `materialClass`
- `MergeProfile`
- `eligibilityDomain`

### 6.3 material을 제외한 결과

v1이 증명하는 재활용 속성은 다음과 같다.

> 이 Note에 일정량의 재활용 mass-balance 크레딧이 할당되어 있고, 그 할당이 정해진 회계 규칙을 위반하지 않았다.

반면 다음은 증명하지 않는다.

- 크레딧이 어떤 material에서 발생했는지
- 입력 material과 출력 product가 물질적으로 호환되는지
- 특정 material-specific claim에 사용할 자격이 있는지
- 실제 제품에 재활용 물질이 물리적으로 포함됐는지
- ISCC PLUS 등 특정 인증제도의 전체 요건을 준수했는지

즉, material type을 속여서 다른 이름의 제품에 크레딧을 붙이는 공격은 v1의 generic claim 정의 안에서는 별도 검증 대상이 아니다. 특정 material 이름이나 적격성을 claim에 포함하려는 순간에는 현재 모델만으로 충분하지 않다.

### 6.4 보존하는 확장 경로

material-specific claim이 필요해지면 새 Note schema와 policy version에서 다음을 추가할 수 있다.

1. committed `materialType` 또는 `eligibilityClass`
2. Process 입력·출력 간 material compatibility relation
3. Issue 시 claim의 material 범위와 Note type의 일치 검사

이 확장은 가능성만 남겨두며 v1 구현 범위에는 포함하지 않는다.

## 7. 통합된 v1 보장과 가정

### 7.1 ZK protocol이 보장하려는 것

- Note가 정해진 commitment schema에 따라 생성되었다.
- 소비하는 Note가 존재하며 prover에게 사용 권한이 있다.
- 같은 Note는 nullifier 때문에 두 번 소비할 수 없다.
- `q_mass`, `a_rec`, `e`가 승인된 Event/policy의 회계 relation을 만족한다.
- 공개하지 않기로 한 상태값과 전이 세부정보는 witness로 유지할 수 있다.

마지막 항목의 정확한 leakage boundary와 공개·비공개 parameter 구분은 우선순위 5에서 형식화한다.

### 7.2 외부 신뢰에 두는 것

- Entry issuer가 초기 질량과 크레딧을 정확하고 중복 없이 등록한다.
- 운영 주체가 운송·공정 탄소 입력을 정확히 제공한다.
- 실제 물질, 실제 공정과 등록된 디지털 값이 일치한다.
- 승인 기관이 올바른 policy/circuit을 등록한다.

이 구분에 따라 논문의 보장 문구는 “물리적 sustainability truth를 증명한다”가 아니라 “신뢰된 입력 이후의 비공개 회계와 claim derivation의 일관성을 증명한다”로 제한한다.

## 8. 현재 모델의 한 줄 요약

> zkDPP v1은 신뢰된 Entry에서 시작한 private Note `(q_mass, a_rec, e)`가 승인된 transition을 거치면서 질량 기반 재활용 크레딧과 Entry 이후 탄소배출량을 일관되게 회계했음을 증명하는 generic, material-agnostic CoC 프로토타입이다.

## 9. 관련 설계 아티팩트

- [Private provenance/audit 설계 노트](./zkdpp-private-provenance-audit-design.md)
- [Mass-only 결정 기록](../../.research-supervisor/events/2026-08-21/162544-auto-mass-only-settled-next-freeze-f1.md)
- [`a_rec` 생명주기 완료 기록](../../.research-supervisor/events/2026-08-23/204045-auto-credit-lifecycle-complete-at1.md)
- [`e` 생명주기 완료 기록](../../.research-supervisor/events/2026-08-23/211033-auto-carbon-lifecycle-complete-ax1.md)
- [최소 Note/material 범위 완료 기록](../../.research-supervisor/events/2026-08-24/120341-auto-generic-credit-note-scope-confirmed-m8v.md)
