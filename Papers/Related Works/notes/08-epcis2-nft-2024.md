# Decentralized Ledger Technology for EPCIS 2.0

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Fausto Neri da Silva Vanin, Yalew Kidane Tolcha, Rodrigo da Rosa Righi, Cristiano André da Costa, Daeyoung Kim
- 출처: IEEE Blockchain 2024
- 출판 상태: peer-reviewed conference paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.1109/Blockchain62396.2024.00018](https://doi.org/10.1109/Blockchain62396.2024.00018)
- 공식 페이지: [IEEE Xplore](https://ieeexplore.ieee.org/document/10664203)
- 로컬 PDF: [EPCIS 2.0 NFT](../../../IEEE%20Blockchain%20Conference/2024/Decentralized_Ledger_Technology_for_EPCIS_2.0_Utilizing_NFTs_for_Enhanced_Product_Traceability.pdf)

## 3. 한 문장 요약과 전체 흐름

EPCIS 2.0의 표준 product event를 NFT mint, transfer, burn, aggregation operation에 대응시키고 Hyperledger Fabric에서 capture와 traceability query를 수행한다.

## 4. 해결하려는 문제와 연구 목표

Supply-chain traceability system이 서로 다른 data model을 쓰면 조직 간 event 교환과 query가 어렵다. 논문은 EPCIS 2.0 semantics를 DLT token operation과 직접 연결하려 한다.

## 5. 저자가 주장하는 핵심 기여

- EPCIS 2.0 event와 NFT operation mapping
- ERC-998식 composition을 반영한 aggregation
- Hash mapping을 이용한 traceability module
- Product originality 검사 algorithm
- Hyperledger Fabric·Caliper 평가

## 6. 시스템과 lifecycle 범위

Object observation, aggregation, transformation, business transaction, association 등 EPCIS가 다루는 product-event lifecycle을 포함한다.

## 7. 등장 주체와 신뢰 가정

EPCIS event를 제출하는 supply-chain organization, Fabric peer, token owner와 query client가 등장한다. Event issuer와 product identifier의 physical binding을 신뢰한다.

## 8. 핵심 객체와 데이터 구조

- EPCIS event: What, When, Where, Why, How
- Product NFT
- Composition-capable token relation
- Event-to-token operation mapping
- Hash-based traceability index

## 9. 전체 동작 과정

EPCIS event를 capture하면 chaincode가 event 유형을 token operation으로 변환한다. Product creation은 mint, 이동은 transfer, 종료는 burn, 포장·구성 관계는 aggregation으로 기록한다. Query module은 hash mapping으로 관련 event를 찾는다.

## 10. 암호 기술과 사용 목적

Permissioned DLT, hash, digital signature와 NFT operation을 사용한다. ZKP 기반 attribute privacy는 중심이 아니다.

## 11. 논문이 제공하는 보장

표준화된 EPCIS event와 blockchain product operation의 대응, event record immutability, product originality query를 제공한다.

## 12. 데이터 신뢰성과 Consistency 확보 방식

Consistency는 EPCIS event schema와 chaincode mapping rule로 확보한다. Hash는 event record를 고정하고 query index를 만든다. Transformation Event의 세부 attribute accounting을 암호학적으로 증명하지는 않는다.

## 13. Privacy, 기원 추적과 CoC

기원 추적은 EPCIS event chain과 token ownership history로 제공된다. Privacy는 핵심 목표가 아니다. CoC는 standardized business event로 표현된다.

## 14. 구현과 성능 평가

Hyperledger Fabric과 Hyperledger Caliper를 사용해 event capture throughput·latency와 traceability query 시간을 측정한다. 저자는 query index가 capture overhead를 크게 늘리지 않으면서 조회 시간을 줄인다고 보고한다.

## 15. 연구 목표 안에서의 강점과 주의점

Traceability를 독자 data schema가 아니라 EPCIS 2.0의 표준 event semantics와 연결한 점이 강점이다. Blockchain에 기록되기 전 event value의 진실성은 submitter와 identifier infrastructure에 의존한다.

## 16. Matrix 분류값

- 1차 분류: DPP–Supply Chain Integrated System
- Product representation: product NFT
- Event: EPCIS 2.0 five event types
- Provenance: EPCIS event query
- Consistency: standard event mapping + hash anchoring

## 17. 중립적인 인용 문장 후보

Vanin et al.은 EPCIS 2.0 event를 NFT operation에 대응시키고, Hyperledger Fabric에서 표준 product event의 capture와 traceability query를 구현하였다.
