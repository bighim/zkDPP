# Related Works 검색 기록

## 1. 현재 검색 범위

핵심 후보는 다음 세 유형으로 제한한다.

1. **제품 전이 모델**: 물리적 제품, 원자재, 부품 또는 batch를 객체로 표현하고 Transfer, Merge, Split, Process 또는 조립 전이를 구체적으로 모델링한 연구
2. **Privacy-preserving Supply Chain Verification**: 공급망의 provenance, transaction, credential 또는 제품 속성을 ZKP, commitment, credential, encryption 등으로 보호하면서 검증하는 연구
3. **Supply Chain + DPP**: 공급망에서 생성된 제품 정보가 downstream 참여자, 소비자 또는 재활용 단계까지 이어지도록 DPP를 구성하고 갱신·공유·검증하는 연구

DPP라는 용어의 사용 여부만으로 후보를 판단하지 않는다. **공급망에서 생성된 제품 정보가 downstream으로 이어지는지**를 우선 확인한다.

## 2. 선별 규칙

핵심 후보는 다음 조건을 먼저 통과해야 한다.

- 물리적 제품·원자재·부품·batch를 다룬다.
- 공급망에서 생성된 제품 정보가 downstream으로 이어진다.
- 구체적인 객체, 동작 또는 검증 절차가 존재한다.
- 정식 학술 출판 상태와 DOI 또는 공식 출판 페이지를 확인할 수 있다.

다음 연구는 핵심 후보에서 제외하고 Matrix 하단에서만 관리한다.

- Review·Survey
- IoT·센서 등을 이용해 실제 물품의 진품 여부만 확인하는 연구
- 단순 Hash anchoring 또는 일반 blockchain traceability
- 접근통제만 제공하는 연구
- Logistics-only, cyber·software supply chain
- Recall·redactability 중심 연구

## 3. 원문과 분석 상태

| 상태 | 의미 |
|---|---|
| `Abstract 검토` | 제목·초록·공식 서지정보만 확인한 상태 |
| `PDF 요청` | 다음 순서로 사용자에게 원문 확보를 요청한 상태 |
| `원문 분석` | 원문 전체를 읽고 상세 노트 초안을 작성한 상태 |
| `이해 확정` | 사용자와 논문의 구조 및 해석을 확인한 상태 |
| `제외` | 현재 핵심 범위 밖 참고 문헌 |

원문이 없는 후보는 암호 구조, 보장 범위, 성능과 최종 중요도를 확정하지 않는다. `Official Full Text`는 공식 공개본의 존재를 확인했다는 뜻이며, 순차 분석 전에 사용자가 `PDFs/`에 저장하는 절차는 동일하다.

## 4. 2026-08-12 재검색

### 검색 기간과 출처

- 중심 기간: 2026년부터 2020년까지
- 2020년 이전: 기존 Seed research family만 유지
- 출처: IEEE Xplore, ACM Digital Library, SpringerLink, ScienceDirect, Taylor & Francis, HICSS와 대학·연구기관 repository
- IEEE Blockchain 2022–2025 proceedings를 연도별 키워드 검색으로 별도 확인

### 검색 축

- `supply chain product transition merge split process blockchain`
- `manufacturing token batch assembly downstream information`
- `privacy preserving supply chain provenance product attribute verification`
- `zero knowledge proof commitment credential encryption supply chain product`
- `digital product passport supply chain lifecycle information sharing verification`
- `digital product passport blockchain downstream recycler consumer`

### References·cited-by 경로

1. Sahai 2020과 Token Recipes 계열에서 제품 전이 및 제조 relation 후보를 추적했다.
2. PrivChain, TradeChain, ZKVeil에서 privacy-preserving provenance, credential과 compliance verification 후보를 추적했다.
3. DPP fundamentals·review 논문은 핵심 후보가 아니라, Supply Chain + DPP 논문의 bibliography를 찾는 검색 도구로만 사용했다.
4. DPP ZKP Extensions의 references에서 ZKP 기반 공급망 검증 후보를 확인했다.
5. IEEE Blockchain 2022–2025 검색 결과에서는 기존 EPCIS 2.0 논문 외에 현재 조건을 통과하는 신규 research family를 확정하지 않았다.

## 5. 신규 후보 10개

아래 내용은 모두 **초록 수준의 잠정 기록**이다. 원문 분석 전에는 Scheme과 중요도를 확정하지 않는다.

| ID | 연도 | 잠정 유형 | 논문과 공식 링크 | 초록에서 확인한 Problem·Aim | 원문 상태 |
|---|---:|---|---|---|---|
| RW-35 | 2025 | Supply Chain + DPP·Privacy | [Human-Centric Digital Product Passports](https://eref.uni-bayreuth.de/id/eprint/92416) | 민감한 공급망 참여자·공정 정보를 보호하면서 소비자에게 검증 가능한 제품 정보를 전달하는 textile DPP prototype | Official Full Text |
| RW-36 | 2025 | Supply Chain + DPP | [Decentralized Digital Product Passport Building Blocks](https://doi.org/10.1109/ACCESS.2025.3594826) | AAS 기반 DPP의 identity, traceability, access permission과 데이터 관리 책임을 분산 구조로 구성 | Official Full Text |
| RW-37 | 2024 | Supply Chain + DPP | [Connecting Producers and Recyclers](https://doi.org/10.1016/j.procir.2024.02.026) | 생산자가 보유한 제품 정보를 수명 종료 단계의 재활용업체가 활용할 수 있도록 DPP와 AAS 기반 정보 교환을 구현 | Official Full Text |
| RW-38 | 2025 | Supply Chain + DPP | [A Blockchain-Based DPP System Providing a Federated Learning Environment](https://doi.org/10.3390/su17062679) | 제조사와 재활용업체가 DPP를 통해 제품·폐기물 정보와 재활용 자동화 model을 연계 | Official Full Text |
| RW-39 | 2024 | Supply Chain + DPP | [The DPP: Enabling Interoperable Information Flows Through Blockchain Consortia](https://doi.org/10.1007/978-3-031-60433-1_21) | 조직 간 DPP 정보 흐름을 blockchain consortium과 public interface로 연결하는 구조와 governance를 설계 | PDF Needed |
| RW-40 | 2024 | Supply Chain + DPP | [A Reference Architecture for DPPs at Batch Level](https://doi.org/10.1007/978-3-031-59465-6_19) | 제조 공급망의 batch·component 단위 정적·동적 데이터를 연결하는 DPP reference architecture와 proof of concept | Official Full Text |
| RW-41 | 2024 | Supply Chain + DPP | [Blockchain-Enabled DPPs for Healthcare Devices](https://doi.org/10.1109/CSNet64211.2024.10851725) | 의료기기의 제조, 유지보수, software update와 사용 이력을 lifecycle DPP로 기록·추적 | PDF Needed |
| RW-42 | 2025 | Privacy-preserving Supply Chain Verification | [Enhancing transparency and traceability in complex supply chains](https://doi.org/10.1016/j.jisa.2025.104169) | 공급망 전 단계의 제품 정보를 암호화해 공유하고, 다자 평가와 CP-ABE로 정보 정확성·접근 범위를 관리 | PDF Needed |
| RW-43 | 2023 | 제품 전이 모델 | [New Framework for Complex Assembly Digitalization and Traceability Using Bill of Assembly and Smart Contracts](https://doi.org/10.3390/app13031884) | 부품·공정·작업 자원을 Bill of Assembly로 연결하고 assembly 단계별 검증·문서화를 smart contract로 자동화 | Official Full Text |
| RW-44 | 2024 | Supply Chain + DPP | [A Digital Twin-Based Digital Product Passport](https://doi.org/10.1016/j.procs.2024.09.251) | 식품의 원산지, 생산 방식과 sustainability 정보를 이해관계자에게 공유하는 Digital Twin 기반 DPP architecture와 prototype | Official Full Text |

## 6. 기존 후보 재분류

- RW-23은 Hyperledger Fabric channel·private data를 이용한 접근통제가 중심이므로 핵심 후보에서 제외했다.
- TrustChain, anti-counterfeiting, object-event mapping 연구는 physical-digital binding 참고 문헌으로 유지한다.
- 일반 DPP 요구사항·정의·architecture와 Review·Survey는 핵심 순위에서 제외하고 하단 참고 목록으로 유지한다.
- 기존에 작성된 상세 노트는 삭제하지 않으며, 사용자 확인 전에는 `원문 분석` 초안으로 취급한다.

### 재검색 중 핵심 후보에서 제외한 대표 결과

| 논문·검색 결과 | 제외 이유 |
|---|---|
| *Digital Product Passports for Advancing the Circular Economy: A Research Agenda* | Review·research agenda |
| *Digital Product Passport as a Digital Twin?* | DPP와 Digital Twin의 개념·설계요소 분석이 중심 |
| *GrAC: Graph-Based Anonymous Credentials* | Supply Chain 제품 정보가 아닌 일반 identity·credential 연구 |
| *Managed Document Solution Logistics Based on Blockchain* | 문서 logistics와 literature review 중심 |
| *Integrating blockchain with digital product passports for managing reverse supply chain* | 이번 범위에서 제외한 reverse supply chain 중심 |
| ZKP를 가능성 수준에서만 제안한 Supply Chain 논문 | 구체적인 검증 statement와 Scheme을 확인할 수 없음 |

BT-CSRS는 ZKP 기반 seafood traceability를 주장하지만, 초록만으로 제품 정보의 전이 relation과 ZKP statement를 확인하기 어려워 최초 10편 이후의 보류 후보로 기록한다.

## 7. 현재 원문 분석

RW-01과 RW-02를 각각 분석 순위 1번과 2번으로 고정했으며, RW-19를 세 번째로 분석한다.

- **분석 순위 3 / RW-19**
- Marlene Kuhn, Felix Funk, Guanlai Zhang, Jörg Franke
- *Blockchain-based application for the traceability of complex assembly structures*
- Journal of Manufacturing Systems, 2021
- DOI: [10.1016/j.jmsy.2021.04.013](https://doi.org/10.1016/j.jmsy.2021.04.013)
- 원문 수령일: 2026-08-12
- 분석 상태: 원문 분석 초안 완료, 사용자 검토 대기
- 분석 이유: part·batch를 ERC-1155 token으로 표현하고 조립 과정의 input-output 구성을 기록하므로, 현재 후보 중 제품 전이 모델과 가장 직접적으로 연결된다.
- 상세 노트: [RW-19 원문 분석](notes/19-complex-assembly-traceability-2021.md)

사용자와 RW-19의 이해를 맞춘 뒤 분석 상태를 `이해 확정`으로 변경하고 다음 순위로 이동한다.
