# Blockchain-Based Supply Chain System for Traceability, Regulation and Anti-Counterfeiting

> 상태: 원문 분석 초안 완료 · 사용자 검토 대기

## 1. 논문 정보와 출판 상태

- 저자: Wang Fat Lau, Dennis Y. W. Liu, Man Ho Au
- 출처: IEEE Blockchain 2021
- 출판 상태: peer-reviewed conference paper

## 2. DOI, 공식 페이지, 공식 공개본과 로컬 PDF

- DOI: [10.1109/Blockchain53845.2021.00022](https://doi.org/10.1109/Blockchain53845.2021.00022)
- 공식 페이지: [IEEE Xplore](https://ieeexplore.ieee.org/document/9680497)
- 로컬 PDF: [IEEE 2021 PDF](../../../IEEE%20Blockchain%20Conference/2021/Blockchain-Based_Supply_Chain_System_for_Traceability_Regulation_and_Anti-Counterfeiting.pdf)

## 3. 한 문장 요약과 전체 흐름

UID가 항상 존재하지 않는 제조·의약 공급망에서도 batch, 포장, 제조·물류 기록을 DAG 형태로 연결하고 규제기관과 소비자가 다단계 origin을 조회할 수 있게 한다.

## 4. 해결하려는 문제와 연구 목표

현실의 의약 공급망에서는 모든 단위에 UID가 붙지 않으며 포장과 물류 흐름이 복잡하다. 기존 단순 선형 추적은 multi-hop relation과 규제 절차를 충분히 표현하기 어렵다.

## 5. 저자가 주장하는 핵심 기여

- UID가 없는 물품을 포함하는 traceability model
- DAG-shaped multi-hop supply flow
- 규제 기록과 anti-counterfeiting query
- Multi-layer blockchain architecture와 효율적 tracing algorithm

## 6. 시스템과 lifecycle 범위

제조, 포장, 물류, 도매, 규제 확인과 소비자 origin query를 다룬다.

## 7. 등장 주체와 신뢰 가정

Manufacturer, wholesaler, logistics provider, regulator, consumer와 permissioned blockchain operator가 등장한다. 등록 제조사와 regulator의 서명·승인을 신뢰한다.

## 8. 핵심 객체와 데이터 구조

- Product·batch·package record
- Manufacturing log
- Logistics record
- Regulatory approval
- DAG edge와 multi-hop trace index

## 9. 전체 동작 과정

물품과 제조·포장 기록을 등록하고, 이동 때마다 이전 record를 참조한다. 규제기관은 제조와 포장 변경 기록을 확인하며, 소비자는 final record에서 origin을 역추적한다.

## 10. 암호 기술과 사용 목적

Digital signature, hash-linked ledger, smart contract와 multi-layer blockchain을 사용한다. Privacy-preserving ZKP가 중심인 Scheme은 아니다.

## 11. 논문이 제공하는 보장

Authorized actor가 제출한 제조·물류 기록의 변경 방지, 다단계 origin tracing, counterfeit 검사에 필요한 record continuity를 제공한다.

## 12. 데이터 신뢰성과 Consistency 확보 방식

Signature와 규제 승인, ledger reference가 기록의 출처와 순서를 고정한다. 물품 quantity나 hidden attribute의 algebraic relation을 증명하지는 않는다.

## 13. Privacy, 기원 추적과 CoC

기원 추적과 규제 투명성이 중심이며 privacy는 주요 목표가 아니다. CoC는 DAG record와 actor signature로 표현된다.

## 14. 구현과 성능 평가

Multi-layer architecture와 logarithmic tracing algorithm의 비용을 분석하고 prototype에서 origin query와 record operation의 효율을 평가한다.

## 15. 연구 목표 안에서의 강점과 주의점

UID가 없는 현실 조건을 문제 설정에 포함하고 규제 흐름을 traceability 구조와 연결한 점이 강점이다. 기록의 진실성은 authorized submitter와 regulator의 업무 절차에 의존한다.

## 16. Matrix 분류값

- 1차 분류: Supply Chain Scheme
- Product representation: product·batch·package record
- Provenance: DAG multi-hop tracing
- CoC: signature와 regulated record
- Privacy target: 주요 목표 아님

## 17. 중립적인 인용 문장 후보

Lau et al.은 UID가 없는 물품을 포함하는 제조·물류 공급망을 DAG record로 구성하고, 규제 기록과 다단계 origin query를 지원하는 blockchain architecture를 제시하였다.
