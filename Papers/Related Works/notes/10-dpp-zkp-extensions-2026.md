# Zero-Knowledge Proof Extensions for Digital Product Passports

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Chibuzor Udokwu, Stefan Craß
- 출처: Electronics 15(4), 2026
- 출판 상태: peer-reviewed journal paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.3390/electronics15040745](https://doi.org/10.3390/electronics15040745)
- 공식 공개본: [MDPI](https://www.mdpi.com/2079-9292/15/4/745)
- 로컬 PDF: [DPP ZKP Extensions](../../DPP%20Reference/Udokwu%20%E1%84%86%E1%85%B5%E1%86%BE%20Cra%C3%9F%20-%202026%20-%20Zero-Knowledge%20Proof%20Extensions%20for%20Digital%20Product%20Passports%20in%20Sustainability%20Claims%20Reporting%20and.pdf)

## 3. 한 문장 요약과 전체 흐름

Supplier가 발급한 VC로 sustainability input data를 확인하고, proving service가 claim별 circuit을 선택해 recycled content와 같은 DPP claim을 private proof로 변환하는 일반 절차를 제시한다.

## 4. 해결하려는 문제와 연구 목표

Carbon footprint, recycled content와 material composition claim은 검증 가능해야 하지만 원재료 수량과 공급 관계는 영업 비밀일 수 있다. 논문은 DPP sustainability claim을 formal statement와 ZKP workflow로 바꾸는 방법을 정리한다.

## 5. 저자가 주장하는 핵심 기여

- Sustainability claim의 formal representation
- Input data matrix와 trust model
- Claim proof generation·verification sequence
- Example claim circuit template
- VC 기반 input validation과 scenario evaluation

## 6. 시스템과 lifecycle 범위

DPP의 sustainability reporting과 claim verification에 집중한다. 전체 제품 Event graph를 직접 관리하는 lifecycle system은 아니다.

## 7. 등장 주체와 신뢰 가정

Supplier, manufacturer, external auditor 또는 credential issuer, proving service, verifier가 등장한다. Issuer가 material quantity 등 credential claim을 올바르게 확인한다고 가정한다.

## 8. 핵심 객체와 데이터 구조

- Sustainability claim
- Claim input data matrix
- W3C Verifiable Credential
- Issuer·holder DID
- Claim-specific circuit
- ZK proof와 public verification value

## 9. 전체 동작 과정

Supplier 또는 auditor가 input data credential을 발급한다. Manufacturer가 credential을 모아 proving service에 전달한다. Service는 signature와 status를 확인하고 claim에 맞는 circuit으로 proof를 만든다. Verifier는 proof로 claim만 확인한다.

## 10. 암호 기술과 사용 목적

- VC signature와 DID resolution: input claim issuer 확인
- zkSNARK circuit: material composition 등 claim predicate 검증
- Fixed-point scaling: finite field에서 비율과 수량 계산
- Blockchain·smart contract: proof verification을 배치할 수 있는 infrastructure

## 11. 논문이 제공하는 보장

유효한 credential에서 가져온 private input이 선택한 sustainability predicate를 만족한다는 proof를 제공한다.

## 12. 데이터 신뢰성과 Consistency 확보 방식

Credential이 source authenticity를 담당하고 ZKP가 claim computation을 담당한다. Input-output product Event relation보다 claim-level consistency에 초점을 둔다.

## 13. Privacy, 기원 추적과 CoC

공급 수량과 claim 계산 input을 공개하지 않는 것이 privacy 목표다. VC issuer chain이 evidence provenance를 제공하지만 물품 CoC 전체를 별도 graph로 모델링하지는 않는다.

## 14. 구현과 성능 평가

Credential 수를 늘리며 signature verification 시간을 측정하고, material-composition example circuit의 proof generation을 평가한다. Scenario-based design evaluation과 threat analysis를 함께 제공한다.

## 15. 연구 목표 안에서의 강점과 주의점

특정 claim 하나가 아니라 claim specification, input trust, proof generation을 분리한 methodology를 제시한 점이 강점이다. Credential issuer와 proving service의 역할, credential reuse와 collusion은 deployment trust model에서 명확히 해야 한다.

## 16. Matrix 분류값

- 1차 분류: DPP–Supply Chain Integrated System
- Product representation: DPP sustainability claim과 VC
- Verification statement: claim-specific predicate
- Privacy target: claim input data
- Consistency: credential validation + claim circuit

## 17. 중립적인 인용 문장 후보

Udokwu and Craß는 DPP sustainability claim을 formal predicate로 표현하고, issuer-signed input credential을 검증한 뒤 claim-specific circuit으로 privacy-preserving proof를 생성하는 workflow를 제시하였다.
