# Token Recipes research family

> 상태: 원문 분석 및 사용자 이해 확인 완료

## 1. 논문 정보와 출판 상태

- 저자: Martin Westerkamp, Friedhelm Victor, Axel Küpper
- 대표본: *Tracing manufacturing processes using blockchain-based token compositions*, Digital Communications and Networks 6(2), 2020
- 최초 발표: *Blockchain-Based Supply Chain Traceability: Token Recipes Model Manufacturing Processes*, IEEE Cybermatics 2018
- 관리 방식: 두 논문을 하나의 research family로 집계

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- Journal DOI: [10.1016/j.dcan.2019.01.007](https://doi.org/10.1016/j.dcan.2019.01.007)
- Conference DOI: [10.1109/Cybermatics_2018.2018.00267](https://doi.org/10.1109/Cybermatics_2018.2018.00267)
- 공식 Journal full text: [ScienceDirect](https://www.sciencedirect.com/science/article/pii/S235286481830244X)
- 공식 공개본: [arXiv:1810.09843](https://arxiv.org/abs/1810.09843)ㄷ

## 3. 한 문장 요약과 전체 흐름

물리적 batch를 non-fungible token으로 나타내고, 제품별 smart contract에 정의된 recipe가 input batch token을 소비한 뒤 output batch token을 생성하게 하여 제조 provenance를 기록한다.

## 4. 해결하려는 문제와 연구 목표

단순한 transfer history만 기록하면 원재료가 제조 공정을 거쳐 다른 제품으로 바뀌는 순간 input과 output의 관계가 끊어진다. 이 논문은 제조 과정에서 어떤 batch가 소비되어 어떤 product batch가 생성되었는지 ledger에 남기는 것을 목표로 한다.

## 5. 저자가 주장하는 핵심 기여

1. 제조 공정을 token recipe로 표현한다.
2. recipe가 요구하는 input token을 소비하고 output token을 생성한다.
3. EVM smart contract prototype을 구현한다.
4. input 수에 따른 gas 증가와 scalability를 평가한다.

## 6. 시스템과 lifecycle 범위

원자재 또는 batch의 생성, 소유권 이전, 제조 input 소비, output batch 생성과 provenance query를 다룬다. 소비 이후 수리·재사용·재활용 lifecycle은 직접 모델링하지 않는다.

## 7. 등장 주체와 신뢰 가정

- Product contract creator: 제품 유형과 recipe를 배포·정의
- Manufacturer: 보유 input batch를 recipe에 넣어 output batch 생성
- Supplier·buyer: batch token 소유권 이전
- Query client: event를 따라 provenance tree 복원

권한 있는 조직이 물리적 batch와 token을 올바르게 연결한다고 가정한다. Smart contract는 온체인 token relation을 강제하지만 실제 제조가 recipe대로 수행됐는지는 직접 관찰하지 않는다.

## 8. 핵심 객체와 데이터 구조

- Product smart contract: 제품 유형별 batch와 recipe 관리
- Batch token: 하나의 물리적 batch를 나타내는 식별 가능한 token
- Recipe: output을 만들기 위해 필요한 input product와 수량
- Event log: batch 생성·소비·이전 관계 기록

표준 ERC-20은 batch를 구분하기 어렵고 ERC-721은 quantity와 제조 input consumption을 직접 표현하지 않기 때문에 별도 token 구조를 사용한다.

## 9. 전체 동작 과정

1. 제품 유형을 위한 smart contract를 배포한다.
2. 해당 contract에 제조 recipe를 정의한다.
3. 원재료 또는 기존 batch token을 생성한다.
4. Manufacturer가 recipe에 필요한 input batch를 제출한다.
5. Contract가 product type, quantity, ownership을 검사한다.
6. Input batch를 소비하고 새로운 output batch를 만든다.
7. Event log를 재귀적으로 조회해 output의 input provenance를 복원한다.

## 10. 암호 기술과 사용 목적

핵심은 ZKP가 아니라 EVM smart contract의 deterministic enforcement다. Ethereum address와 transaction signature는 caller 권한을 확인하고, ledger와 event log는 token consumption history를 고정한다.

## 11. 논문이 제공하는 보장

- 존재하는 input batch와 요구 수량 없이는 recipe 실행 불가
- 소비된 input과 생성된 output 사이의 온체인 연결
- 동일 token의 중복 소비 방지
- Event 재귀 조회를 통한 제조 provenance 복원

## 12. 데이터 신뢰성과 Consistency 확보 방식

Consistency는 smart contract가 recipe의 input product와 amount를 검사하고 input token을 소비하는 방식으로 확보한다. 이는 Hash만 저장하는 구조가 아니다. 다만 carbon, recycled content와 같은 숨겨진 다차원 attribute relation을 증명하지는 않는다.

## 13. Privacy, 기원 추적과 CoC

Privacy는 주요 목표가 아니다. Token과 Event 관계가 공개되며, provenance tree를 복원할 수 있다. CoC는 batch ownership과 ingredient-to-product composition의 기록으로 제공된다.

## 14. 구현과 성능 평가

- Solidity와 EVM 기반 prototype
- Web3 기반 provenance query 도구
- Recipe input 수가 늘어날수록 transaction gas가 선형적으로 증가
- 복잡한 제품에서도 input 수에 비례해 비용을 예측할 수 있음을 보인다.

## 15. 연구 목표 안에서의 강점과 주의점

강점은 물품의 제조를 단순 transfer가 아니라 input consumption과 output generation으로 명시한 점이다. Batch, recipe, provenance event가 구현 객체로 직접 연결되어 있다.

주의할 점은 product type별 contract와 공개 recipe가 운영·확장 비용을 만들 수 있고, 물리적 공정의 진실성은 별도 검증이 필요하다는 점이다.

## 16. Matrix 분류값

- 1차 분류: Supply Chain Scheme
- Product representation: batch-specific token
- Manufacturing transition: recipe-based consume-and-mint
- Provenance: event graph
- Privacy target: 주요 목표 아님
- Consistency: smart-contract recipe enforcement

## 17. 중립적인 인용 문장 후보

Westerkamp et al.은 물리적 batch를 식별 가능한 token으로 표현하고, recipe가 input token을 소비하여 output token을 생성하도록 함으로써 제조 과정의 ingredient-to-product provenance를 ledger에 기록하였다.
