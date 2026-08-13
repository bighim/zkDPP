# Enhancing Blockchain Traceability with DAG-Based Tokens

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Hiroki Watanabe, Tatsuro Ishida, Shigenori Ohashi, Shigeru Fujimura, Atsushi Nakadaira, Kota Hidaka, Jay Kishigami
- 출처: IEEE Blockchain 2019
- 출판 상태: peer-reviewed conference paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.1109/Blockchain.2019.00036](https://doi.org/10.1109/Blockchain.2019.00036)
- 공식 페이지: [IEEE Xplore](https://ieeexplore.ieee.org/document/8946198)
- 로컬 PDF: [DAG-Based Tokens](../../../IEEE%20Blockchain%20Conference/2019/Enhancing_Blockchain_Traceability_with_DAG-Based_Tokens.pdf)

## 3. 한 문장 요약과 전체 흐름

Token의 이전 상태와 새 상태를 DAG edge로 직접 연결하여 transfer뿐 아니라 merge, split, fork가 포함된 이력을 전체 blockchain scan 없이 조회한다.

## 4. 해결하려는 문제와 연구 목표

일반 token interface는 안전한 transfer에는 유용하지만 과거 이력을 빠르게 찾는 구조를 제공하지 않는다. Blockchain explorer에 의존하지 않으면 모든 block을 검색해야 한다. 논문은 token 자체에 history relation을 넣어 trustless query를 빠르게 만드는 것을 목표로 한다.

## 5. 저자가 주장하는 핵심 기여

1. Token state history를 DAG로 표현한다.
2. Merge, Split, branch를 history model에 포함한다.
3. Ethereum prototype과 history retrieval 실험을 제공한다.

## 6. 시스템과 lifecycle 범위

제품을 나타내는 token의 생성 이후 유통·병합·분할 이력을 다룬다. 제품 attribute의 의미론적 변화는 중심 대상이 아니다.

## 7. 등장 주체와 신뢰 가정

Token owner가 state transition을 제출하고, smart contract가 relation을 기록하며, query client가 DAG를 따라간다. 물리적 제품과 token의 결합은 별도 식별 수단에 의존한다.

## 8. 핵심 객체와 데이터 구조

- Token state
- 이전 token state를 가리키는 parent reference
- Merge·Split에서 여러 parent 또는 child를 갖는 DAG
- History query index

## 9. 전체 동작 과정

1. Product token을 생성한다.
2. Transfer 또는 변환 시 새 state를 만든다.
3. 새 state가 이전 state를 parent로 참조한다.
4. Merge·Split은 다대일 또는 일대다 edge로 기록한다.
5. Query client가 latest state에서 parent edge를 따라 history를 복원한다.

## 10. 암호 기술과 사용 목적

Blockchain transaction signature와 smart contract state가 token transition을 인증한다. 별도의 ZKP로 private attribute를 검증하는 논문은 아니다.

## 11. 논문이 제공하는 보장

- Token history의 tamper-evident 기록
- Merge·Split을 포함한 relation 표현
- 중앙화된 explorer 없이 history traversal

## 12. 데이터 신뢰성과 Consistency 확보 방식

Consistency는 token state의 parent-child reference와 smart contract validation으로 확보한다. Product attribute의 수량 보존이나 hidden relation은 검증하지 않는다.

## 13. Privacy, 기원 추적과 CoC

Privacy는 주요 목표가 아니다. 공개된 DAG가 token provenance와 CoC를 직접 제공한다.

## 14. 구현과 성능 평가

Ethereum mainnet의 1,030 token history를 naive scan으로 찾는 데 약 57분이 걸린 사례를 기준으로 삼고, 제안한 DAG query가 같은 규모의 history를 수초 안에 탐색함을 보인다.

## 15. 연구 목표 안에서의 강점과 주의점

History retrieval을 token data model의 문제로 명확히 분리한 점이 강점이다. 공개 DAG이므로 거래 관계를 숨기는 용도에는 적합하지 않으며, 이 점은 논문의 목표 범위와 함께 읽어야 한다.

## 16. Matrix 분류값

- 1차 분류: Supply Chain Scheme
- Product representation: DAG-based token state
- Event: Transfer, Merge, Split, Fork
- Provenance: parent-child history
- Privacy target: 주요 목표 아님
- Consistency: smart-contract state link

## 17. 중립적인 인용 문장 후보

Watanabe et al.은 token state 사이의 relation을 DAG로 기록하여 merge와 split이 포함된 제품 이력을 전체 blockchain scan 없이 탐색하는 token design을 제시하였다.
