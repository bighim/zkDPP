# zkDPP–CoC–DPP 공통 이해 초안

Date: 2026-08-05  
Status: 연구원 간 검토·합의를 위한 초안

## 목적

이 문서는 zkDPP, Chain of Custody(CoC), Digital Product Passport(DPP)의
관계를 같은 의미로 이해하기 위한 짧은 기준 문서다. 아직 선택하지 않은 표준이나
구현 범위를 합의된 사실처럼 기록하지 않는다.

## 가장 짧은 정리

> **DPP는 제품에 대해 무엇을 누구에게 보여줄지를 정하고, CoC는 그 정보가
> 어디에서 왔으며 공급망 과정이 정당한지를 뒷받침하고, zkDPP는 그 근거를
> 공개하지 않으면서도 정당성을 증명한다.**

## 핵심 개념

### DPP

- 특정 제품 또는 제품 모델에 연결되는 디지털 정보 객체다.
- 제품의 생산 이전부터 사용·수리·재활용까지 필요한 **선택된 정보**를 담는다.
- 어떤 정보를 담고, 누가 읽고 갱신하는지는 적용 profile에 따라 달라진다.
- EU Battery Passport는 DPP의 한 가지 규제 profile이지, 모든 DPP의 정의 자체는
  아니다.

### CoC

- 인증된 원료나 sustainability attribute가 공급망을 따라 어떻게 들어오고,
  가공되고, 전달되고, 배정됐는지를 관리하는 근거 체계다.
- 수량, material type, site, period, loss, conversion, allocation 및 중복 사용 여부
  등을 다룰 수 있다.
- CoC 자체가 DPP는 아니다. CoC가 만든 결과가 DPP sustainability claim의 근거가
  될 수 있다.

### Sustainability attribute

- 인증 원료에 연결된 재활용·지속가능성 등의 주장 가능한 속성을 뜻한다.
- ISCC PLUS Mass Balance에서는 실제 분자의 위치와 별개로, 정해진 guardrail 안에서
  인증 속성을 특정 산출물에 배정할 수 있다.
- 이 attribute credit은 외부 감축사업에서 구매하는 탄소배출권·offset과 다른
  개념이다.

### zkDPP

- 원재료와 공급망의 민감한 정보를 공개하지 않고도 CoC와 attribute allocation의
  정당성을 증명하는 계층이다.
- 그 결과를 실제 제품의 DPP claim과 hash로 연결하면 DPP-integrated system이 된다.
- DPP 연결을 구현하지 않는 경우에는 “DPP에서 활용할 수 있는 CoC 증명 기반”이라고
  표현해야 한다.

## 전체 관계

```text
승인된 기관의 원재료 credential
                ↓
비공개 CoC 상태전이
(Transfer / Merge / Split / Process)
                ↓
중복 없는 sustainability attribute 배정
                ↓
제품별 sustainability claim 생성
                ↓
선택 사항: claimHash를 실제 DPP 객체에 연결
```

## 현재 제안하는 연구 범위

현재 zkDPP의 핵심은 다음으로 정리한다.

> **DPP에 표시될 sustainability claim이 승인된 원재료 인증, 올바른 비공개 CoC
> 상태전이 및 중복 없는 attribute 배정에 근거한다는 사실을 증명하는 시스템.**

구체적으로 다음 보장을 목표로 한다.

1. 승인된 issuer만 최초 원재료 credential 또는 Entry를 발행한다.
2. 이전 상태나 attribute entitlement는 한 번만 소비된다.
3. Merge, Split, Process는 선택된 CoC policy의 수량·종류·손실·배정 규칙을 지킨다.
4. 동일한 sustainability attribute를 여러 제품 claim에 중복 사용하지 못한다.
5. DPP 연동을 구현할 경우, 증명 결과가 특정 제품의 `claimHash`와 일치한다.

## 외부 가정과 비주장 범위

zkDPP가 직접 증명하지 않는 것은 다음과 같다.

- 실제 원재료가 정말 재활용·지속가능 원료인지
- 측정 수량과 실제 물리량이 같은지
- certificate와 실제 물리 제품이 올바르게 연결됐는지
- ISCC PLUS 인증의 모든 조직·감사 요건
- EU Battery Regulation 전체 compliance
- 아직 선택하지 않은 탄소발자국 또는 재활용률 계산 방법 전체

최초 원재료의 진실성은 승인된 issuer 또는 auditor가 보장한다고 가정한다. zkDPP는
그 인증이 이후 공급망에서 변조·부풀림·중복 사용되지 않았음을 증명한다.

## ISCC PLUS와 EU Battery Passport의 위치

- **EU Battery Passport:** 연구 motivation과 적용 가능한 DPP profile을 제공한다.
- **ISCC PLUS:** 구체적인 CoC/Mass Balance policy instance 후보를 제공한다.
- **zkDPP:** 선택된 CoC 규칙을 비공개로 검증하고, 그 결과를 DPP claim에 연결할 수
  있게 한다.

ISCC PLUS 적합성이 곧 EU Battery Passport 적합성을 뜻하지 않는다. 둘을 함께
사용하려면 어떤 ISCC 결과가 어떤 Battery Passport field의 근거가 되는지 별도
mapping이 필요하다.

## 논문에서 사용할 수 있는 쉬운 표현

> 본 연구는 기업의 원재료와 공정 정보를 공개하지 않으면서도, 제품의
> sustainability claim이 신뢰할 수 있는 원재료 인증에서 시작해 올바른 공급망
> 과정을 거쳤고 다른 제품에 중복 사용되지 않았음을 증명하는 시스템을 제안한다.

DPP claim binding까지 실제로 구현한 경우에만 다음 문장을 사용할 수 있다.

> 본 연구는 그 증명 결과를 특정 제품의 DPP sustainability claim과 암호학적으로
> 연결한다.

## 아직 합의해야 할 사항

1. 어떤 실제 원재료와 공급망 시나리오를 사용할 것인가?
2. 어떤 CoC 표준과 attribution 방식의 어떤 규칙을 구현할 것인가?
3. 최종적으로 증명할 sustainability claim은 정확히 무엇인가?
4. material state와 attribute entitlement를 별도 note로 관리할 것인가?
5. DPP 연동을 claimHash adapter까지 실제 구현할 것인가?
6. 어떤 정보는 public이고 어떤 정보는 private인가?
7. ISCC·EU 규제 적합성을 어디까지 주장하고 어디부터 외부 가정으로 둘 것인가?

## 다음 단계

하나의 수치 예제를 선택하여 다음 대응표를 만든다.

```text
원재료 credential
→ private state
→ CoC rule
→ attribute budget
→ allocation proof
→ public sustainability claim
→ 선택 사항: DPP claimHash
```

이 대응표가 합의된 뒤 circuit, contract, registry와 DPP adapter 범위를 확정한다.

## 참고 자료

- [ISCC PLUS 공식 설명](https://iscc-system.org/certification/certification-schemes/iscc-plus/)
- [ISCC PLUS Mass Balance와 Attribution](https://iscc-system.org/mass-balance-and-attribution-understanding-the-difference/)
- [EU Batteries Regulation](https://eur-lex.europa.eu/eli/reg/2023/1542/oj?locale=en)

