# Blockchain-based application for the traceability of complex assembly structures

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Marlene Kuhn, Felix Funk, Guanlai Zhang, Jörg Franke
- 출처: Journal of Manufacturing Systems, Volume 59, 2021, pp. 617-630
- 출판 상태: peer-reviewed journal paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.1016/j.jmsy.2021.04.013](https://doi.org/10.1016/j.jmsy.2021.04.013)
- 공식 페이지: [ScienceDirect](https://www.sciencedirect.com/science/article/abs/pii/S0278612521000935)
- 로컬 PDF: [Complex Assembly Traceability](../PDFs/Blockchain-based%20application%20for%20the%20traceability%20of%20complex%20assembly%20structures.pdf)

## 3. 한 문장 요약과 전체 흐름

Batch와 개별 부품을 하나의 ERC-1155 smart contract에서 token으로 표현하고, 조립에 사용된 input token을 소각한 뒤 output token을 생성하여 복잡한 제품의 조립 관계를 graph로 추적한다.

전체 흐름은 `token 생성 → 기업·공정 간 전달 → craft를 통한 조립 → 상태 갱신 → 조립 graph 조회`로 구성된다.

## 4. 해결하려는 문제와 연구 목표

맞춤형 제품은 많은 batch와 개별 부품이 여러 공장과 기업을 거쳐 계층적으로 조립된다. 제품 구성이 자주 달라지기 때문에 제품 종류마다 별도의 smart contract를 배포하는 방식으로는 복잡한 조립 구조를 유연하게 표현하기 어렵다.

논문의 목표는 다음 정보를 하나의 분산 traceability system에서 연결하는 것이다.

- 어떤 batch와 개별 부품이 조립에 사용되었는가
- 각 부품을 어느 기업·설비·작업자가 관리했는가
- 조립 과정에서 제품의 상태가 언제 어떻게 바뀌었는가
- 최종 제품에서 하위 부품까지 어떻게 거슬러 올라갈 수 있는가

## 5. 저자가 주장하는 핵심 기여

1. 복잡한 다계층 조립 구조를 추적하는 dApp `TokenTrail`을 제안한다.
2. Fungible batch와 non-fungible unique part를 하나의 ERC-1155 기반 `Assembly Token Manager`에서 관리한다.
3. 임의의 제품 구조를 다루기 위해 제품별 조립 구조를 smart contract에 미리 고정하지 않는다.
4. Input token의 소비와 output token의 생성을 event로 연결하여 조립 graph를 복원한다.
5. 자동차 전기·전자 시스템 조립 사례로 prototype의 기능과 처리 용량을 평가한다.

## 6. 시스템과 lifecycle 범위

원재료 또는 기본 부품의 token 생성부터 기업 간 전달, 조립, 품질·공정 상태 갱신, 최종 제품의 구성 조회까지를 다룬다.

제품의 사용·수리·재활용 단계보다는 제조와 조립 단계의 traceability가 중심이다.

## 7. 등장 주체와 신뢰 가정

- Supply-chain company: 한 개의 PoA signing node 운영
- Traceability-Relevant Actor, TRA: 기계, 작업자, 품질 검사소, 물류 거점처럼 token을 보유하거나 처리하는 주체
- Controller: 특정 token을 처리할 권한을 가진 account
- Operator: 다른 account를 대신해 ERC-1155 token을 처리하도록 승인된 account
- TokenTrail user: 제품 식별자를 이용해 조립 구조와 상태 이력을 조회하는 사용자

Permissioned Ethereum network의 참여 기업은 이미 식별되어 있으며, 과반수 signing node가 공모하지 않는다고 가정한다. Controller와 operator 권한은 누가 token을 처리할 수 있는지를 제한하지만, 입력한 제조 정보가 실제 물리적 공정과 일치하는지는 별도의 현장 연계와 참여자의 정직한 기록에 의존한다.

## 8. 핵심 객체와 데이터 구조

### ERC-1155 token과 instance

- Token ID: 같은 traceability 정보를 공유하는 물품 종류 또는 batch를 식별
- Token instance: 해당 token에 속하는 하나의 물리적 부품
- Batch: supply가 여러 개인 fungible token
- Unique part: supply가 하나인 사실상의 non-fungible token
- Burned token: 조립에 소비되어 현재 balance는 없지만 이력 조회를 위해 ID가 유지되는 token

### On-chain 데이터

- `balances[tokenId][account]`: Account가 보유한 token instance 수량
- `controllers[tokenId][account]`: Account의 token 처리 권한
- `status`: 품질·승인·공정 상태를 기록하는 event
- `serialNumber`: UBID 또는 UIID를 연결하는 event
- `URI`: off-chain attribute 파일을 연결하는 event
- `craftedToken`: input token과 새 output token의 조립 관계를 기록하는 event

### Off-chain 데이터

Neo4j에는 제품·공정·설비 설명, 주문 사양과 실행 정보가 저장된다. TokenTrail은 on-chain event와 Neo4j 데이터를 함께 조회해 사용자에게 제품 구조와 이력을 보여준다.

## 9. 전체 동작 과정

### 9.1 Token 생성

`create`는 초기 supply, URI, serial number와 TRA account를 입력받는다. 새 Token ID를 만들고 지정된 TRA에 token instance를 할당한 뒤 controller, transfer, metadata와 상태 event를 기록한다.

### 9.2 Token 전달

ERC-1155의 `safeTransferFrom`과 `safeBatchTransferFrom`을 이용해 하나 또는 여러 token의 instance를 다른 TRA account로 전달한다. Balance 변화가 물품의 위치와 관리 주체 변화를 나타낸다.

### 9.3 조립

`craft`는 다음 값을 입력받는다.

- Input token ID 배열
- 각 input에서 소비할 수량 배열
- 새 output token의 초기 supply
- 조립을 수행하는 TRA
- URI와 serial number

Smart contract는 다음 조건을 검사한다.

1. Token ID 배열과 수량 배열의 길이가 같다.
2. TRA가 각 input token을 필요한 수량만큼 보유한다.
3. 호출자가 각 input token의 controller다.

조건이 맞으면 input instance를 TRA의 balance에서 소각하고 새 output token을 생성한다. 이어서 `craftedToken` event에 input ID, input 수량, TRA, output 수량과 새 Token ID를 기록한다.

### 9.4 상태 갱신

Controller는 `newStatus` 등의 함수로 품질, 승인 또는 공정 상태 event를 추가한다. 기존 event를 수정하는 대신 새로운 상태와 발생 시점을 누적한다.

### 9.5 조립 graph 조회

TokenTrail은 최종 제품의 UIID에서 Token ID를 찾은 뒤 다음 과정을 반복한다.

1. 해당 token의 상태 event를 조회한다.
2. `craftedToken` event에서 input token을 찾는다.
3. 각 input token에 같은 조회를 재귀적으로 수행한다.
4. 더 이상 input이 없는 기본 부품에 도달하면 탐색을 종료한다.

이 결과로 최종 제품부터 원재료·부품까지 이어지는 token graph와 시간순 상태 graph를 복원한다.

## 10. 암호 기술과 사용 목적

| 기술 | 사용 목적 |
|---|---|
| Ethereum transaction signature | Transaction 작성 account 인증 |
| Blockchain hash와 Merkle root | Block·transaction·state·event의 변경 탐지와 light-node 검증 |
| Proof of Authority, Clique | 식별된 기업들 사이의 permissioned consensus |
| ERC-1155 smart contract | Batch와 unique part의 balance·권한·전달·소비 관리 |

ZKP, commitment 또는 암호화된 attribute relation은 사용하지 않는다. Prototype은 높은 투명성을 목표로 하며, 민감한 데이터의 암호화는 향후 확장 가능성으로만 제시한다.

## 11. 논문이 제공하는 보장

- Batch와 unique part를 하나의 contract에서 함께 표현
- Token balance를 통한 위치·보유 관계 추적
- Controller와 operator를 통한 token 처리 권한 제한
- 조립에 소비된 input token과 생성된 output token의 연결
- Event를 이용한 제품 구성, 상태와 담당 TRA의 이력 복원
- Consortium ledger를 이용한 기록의 사후 변경 방지

## 12. 데이터 신뢰성과 Consistency 확보 방식

이 논문에서 Consistency는 주로 token의 **소비 가능성, 처리 권한과 조립 관계**에서 확보된다.

- 실제 보유 수량보다 많은 input instance를 소비할 수 없다.
- Controller가 아닌 account는 해당 token을 조립에 사용할 수 없다.
- 소비된 input ID·수량과 새 output ID·수량이 하나의 `craftedToken` event로 연결된다.
- 이후 graph traversal로 어떤 input이 어떤 output에 포함되었는지 확인할 수 있다.

반면 `craft`에는 사전에 등록된 Recipe가 없다. Contract는 material type의 호환성, input quantity와 output quantity 사이의 수율, carbon·recycled content 같은 attribute의 전이식을 검사하지 않는다. 따라서 이 논문의 핵심은 **조립 관계를 기록하고 재구성하는 것**이며, 제품 속성의 산술적 전이를 검증하는 것은 연구 목표에 포함되지 않는다.

## 13. Privacy, 기원 추적과 CoC

`craftedToken` event가 input과 output을 직접 연결하므로 제품의 구성 graph를 명시적으로 추적할 수 있다. Transfer, controller와 status event를 함께 조회하면 물품의 이동, 처리 주체와 공정 상태도 시간순으로 복원할 수 있다.

Privacy는 주요 목표가 아니다. Network 접근은 permissioned 구조로 제한되지만, 참여자에게는 제품 구성과 공정 이력이 투명하게 공유된다. Prototype에는 조립 관계를 숨기는 암호화나 unlinkability가 없다.

## 14. 구현과 성능 평가

- Ethereum permissioned network
- Clique Proof of Authority
- Geth client
- ERC-1155 기반 Assembly Token Manager
- Neo4j off-chain graph database
- TokenTrail dApp과 web interface

평가 대상은 자동차 전기·전자 시스템 15개 주문이다.

| 항목 | 값 |
|---|---:|
| 전체 부품 수 | 13,226개 |
| Blockchain transaction | 약 15,000건 |
| Event log | 약 30,000건 |
| 실험 장비 | Intel i7 2.7 GHz, RAM 7.7 GB laptop |
| 기본 block time | 5초 |
| 기본 block gas limit | 6,000,000 gas |
| 주문당 craft 함수 총합 | 평균 약 7.9 million gas |
| 주문당 transfer 함수 총합 | 평균 약 5.9 million gas |
| Token 생성까지 포함한 주문당 총합 | 약 20 million gas |

Block gas limit이 6, 10, 15, 20 million일 때 저자가 계산한 처리량은 각각 분당 약 3, 5, 7.5, 9.5개 주문이다. 이 수치는 개별 transaction의 gas가 아니라 한 제품 주문을 구성하는 여러 함수 호출의 합과 simulation 설정에서 도출된 처리량이다.

## 15. 연구 목표 안에서의 강점과 주의점

한 contract 안에서 batch와 unique part를 함께 다루고, input token의 일부 수량을 여러 상위 assembly가 나누어 사용할 수 있게 한 점이 핵심이다. 제품 종류마다 contract를 새로 배포하지 않아도 동적으로 구성되는 다계층 조립 graph를 표현할 수 있다.

주의해서 읽을 점은 smart contract가 조립의 산업적 타당성을 판단하지 않는다는 것이다. `craft` 호출 권한과 token balance는 검사하지만, 어떤 부품을 어떤 비율로 결합해야 하는지는 contract 밖의 제조 절차와 참여자에게 맡긴다. 또한 실제 평가도 제한된 자동차 test data를 이용한 prototype simulation이며 실제 공장과 자동 연동한 결과는 아니다.

## 16. Matrix 분류값

- 1차 분류: Supply Chain Scheme
- Product representation: ERC-1155 batch token과 unique-part token
- Lifecycle coverage: 생성, 전달, 조립, 상태 갱신, 구성 조회
- Supply Chain Event: Create, Transfer, Craft, Status Update, Controller Change, Query
- Manufacturing transition: 여러 input token instance 소비 후 한 개의 새 output Token ID 생성
- Provenance: `craftedToken` event 기반 assembly graph
- Privacy target: 주요 목표 아님
- Consistency: Balance·controller 검사와 input-output event link
- Trust assumption: permissioned participant와 physical-token binding
- Evaluation: Automotive E/E assembly prototype과 gas·throughput simulation

## 17. 중립적인 인용 문장 후보

Kuhn et al.은 ERC-1155를 이용하여 batch와 개별 부품을 하나의 smart contract에서 표현하고, 조립에 소비된 input token과 새 output token을 event로 연결함으로써 다계층 assembly graph를 추적하는 TokenTrail을 제시하였다.
