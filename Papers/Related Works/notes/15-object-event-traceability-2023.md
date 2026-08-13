# Blockchain-Based Traceability Architecture for Mapping Object-Related Supply Chain Events

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Fabian Dietrich, Louis Louw, Daniel Palm
- 출처: Sensors 23(3), 2023
- 출판 상태: peer-reviewed open-access journal paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.3390/s23031410](https://doi.org/10.3390/s23031410)
- 공식 페이지: [MDPI](https://www.mdpi.com/1424-8220/23/3/1410)
- 공식 PDF: [MDPI static PDF](https://mdpi-res.com/d_attachment/sensors/sensors-23-01410/article_deploy/sensors-23-01410-v2.pdf)
- 로컬 저장소 PDF: 없음

## 3. 한 문장 요약과 전체 흐름

Globally unique token으로 physical·digital object를 나타내고, blueprint와 token container를 이용해 EPCIS의 creation, observation, aggregation, disaggregation, transformation과 transaction event를 Ethereum dApp에 기록한다.

## 4. 해결하려는 문제와 연구 목표

Supply network의 object는 이동뿐 아니라 조립, 분해, 묶음과 해체를 반복한다. 기존 token standard만으로는 object type, aggregation history, governance와 모든 EPCIS Event를 일관되게 표현하기 어렵다.

## 5. 저자가 주장하는 핵심 기여

- DApp 내부 governance concept
- 같은 type의 여러 NFT를 만들 수 있는 blueprint-based token
- EPCIS object-related Event 전체 mapping
- Aggregation·disaggregation을 위한 token container
- Operational prototype와 architecture evaluation

## 6. 시스템과 lifecycle 범위

Manufacturing과 logistics에서 object 생성, 관찰, 이동, 조립·포장, 해체와 transformation을 다룬다.

## 7. 등장 주체와 신뢰 가정

Governance authority, registered supply-chain party, object creator, token owner와 query user가 등장한다. Governance function이 참여자 등록·제거와 operation right를 관리한다.

## 8. 핵심 객체와 데이터 구조

- Globally unique object token
- Token blueprint
- Token memory
- Aggregated token을 보관하는 token container
- Party memory와 governance set
- EPCIS-aligned event record

## 9. 전체 동작 과정

Governance authority가 party와 권한을 등록한다. Blueprint가 object type별 token mint 조건을 정의한다. Supply-chain Event가 발생하면 token을 이동·관찰·aggregate·disaggregate·transform하고 event history를 ledger에 남긴다.

## 10. 암호 기술과 사용 목적

Ethereum transaction signature, smart contract, immutable ledger와 NFT-style identifier를 사용한다. Hidden attribute proof는 핵심 기술이 아니다.

## 11. 논문이 제공하는 보장

Registered party만 허용된 Event를 실행하고, token identifier와 aggregation history를 유지하며 EPCIS object event를 일관된 contract operation으로 기록한다.

## 12. 데이터 신뢰성과 Consistency 확보 방식

Blueprint와 smart-contract precondition이 object type과 operation을 강제한다. Token container는 aggregation 중 원래 identifier를 보존해 disaggregation 시 복원한다. 실제 물리 Event의 진실성은 authorized party에게 의존한다.

## 13. Privacy, 기원 추적과 CoC

공개 token history를 통한 end-to-end traceability가 중심이며 privacy는 주요 목표가 아니다. CoC는 ownership, aggregation과 transformation Event의 연속으로 표현된다.

## 14. 구현과 성능 평가

Truffle Suite, Solidity, local Ganache, Web3JS, MetaMask와 React로 prototype을 구현한다. 기능 완전성과 architecture requirement 충족 여부를 평가하고, 복잡한 contract dependency가 Ethereum transaction·deployment limit에 가까워지는 dApp complexity 문제를 보고한다.

## 15. 연구 목표 안에서의 강점과 주의점

Object master data, Event, governance와 aggregation을 하나의 architecture로 연결한 점이 강점이다. 기능 범위가 넓어질수록 contract complexity와 public-chain scalability가 빠르게 커진다는 구현상 주의점이 있다.

## 16. Matrix 분류값

- 1차 분류: Supply Chain Scheme
- Product representation: blueprint-based unique token
- Event: EPCIS object-related events
- Manufacturing transition: aggregation, disaggregation, transformation
- Privacy target: 주요 목표 아님
- Consistency: governance + smart-contract operation rule

## 17. 중립적인 인용 문장 후보

Dietrich et al.은 blueprint-based token과 token container를 이용해 unique object의 aggregation·disaggregation을 포함한 EPCIS event를 Ethereum smart contract operation으로 모델링하였다.
