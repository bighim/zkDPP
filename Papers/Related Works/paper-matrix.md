# Supply Chain·DPP Related Works Matrix

## 읽는 방법

- 핵심 범위는 `제품 전이 모델`, `Privacy-preserving Supply Chain Verification`, `Supply Chain + DPP` 세 유형이다.
- 순위는 논문의 최종 품질 순위가 아니라 **원문을 읽고 이해를 맞출 순서**다.
- RW-01 Sahai를 1번, RW-02 Token Recipes를 2번으로 고정했다.
- `Abstract 검토`와 `PDF 요청` 단계에서는 Problem·Aim과 잠정 유형만 기록한다.
- Scheme, 보장 범위, 성능과 최종 중요도는 원문 분석과 사용자 확인 뒤에 확정한다.
- Review·Survey와 범위 밖 연구는 핵심 순위에 포함하지 않고 하단에서 관리한다.

분석 상태는 `Abstract 검토` → `PDF 요청` → `원문 분석` → `이해 확정` 순서로 진행한다.

## 순차 분석 목록

| 순위 | ID | 논문 | 잠정 유형 | 원문 상태 | 분석 상태 |
|---:|---|---|---|---|---|
| 1 | RW-01 | Sahai 2020 | 제품 전이·Privacy | Local PDF | [원문 분석](notes/01-sahai-2020.md) |
| 2 | RW-02 | Token Recipes 2018·2020 | 제품 전이 | Local PDF | [이해 확정](notes/02-token-recipes-family.md) |
| 3 | RW-19 | Complex assembly traceability | 제품 전이 | Local PDF | [원문 분석](notes/19-complex-assembly-traceability-2021.md) |
| 4 | RW-04 | DAG-Based Tokens | 제품 전이 | Local PDF | [원문 분석](notes/04-dag-tokens-2019.md) |
| 5 | RW-08 | EPCIS 2.0 with NFTs | 제품 전이·Supply Chain + DPP | Local PDF | [원문 분석](notes/08-epcis2-nft-2024.md) |
| 6 | RW-43 | Bill of Assembly with Smart Contracts | 제품 전이 | Official Full Text | Abstract 검토 |
| 7 | RW-17 | Blockchain-based DPP design principles | Supply Chain + DPP | PDF Needed | Abstract 검토 |
| 8 | RW-10 | DPP ZKP Extensions | Supply Chain + DPP·Privacy | Local PDF | [원문 분석](notes/10-dpp-zkp-extensions-2026.md) |
| 9 | RW-35 | Human-Centric DPP | Supply Chain + DPP·Privacy | Official Full Text | Abstract 검토 |
| 10 | RW-09 | ZKVeil | Privacy verification | Local PDF | [원문 분석](notes/09-zkveil-2026.md) |
| 11 | RW-03 | PrivChain | Privacy verification | Local PDF | [원문 분석](notes/03-privchain-2022.md) |
| 12 | RW-24 | ProChain | Privacy verification | PDF Needed | Abstract 검토 |
| 13 | RW-33 | DECOUPLES | Privacy verification | PDF Needed | Abstract 검토 |
| 14 | RW-06 | TradeChain | Privacy verification | Local PDF | [원문 분석](notes/06-tradechain-2021.md) |
| 15 | RW-26 | Multi-hop information accountability | Privacy verification | PDF Needed | Abstract 검토 |
| 16 | RW-39 | Interoperable DPP information flows | Supply Chain + DPP | PDF Needed | Abstract 검토 |
| 17 | RW-40 | Batch-level DPP reference architecture | Supply Chain + DPP | Official Full Text | Abstract 검토 |
| 18 | RW-37 | Connecting Producers and Recyclers | Supply Chain + DPP | Official Full Text | Abstract 검토 |
| 19 | RW-36 | Decentralized DPP Building Blocks | Supply Chain + DPP | Official Full Text | Abstract 검토 |
| 20 | RW-42 | Complex supply-chain secure sharing | Privacy verification | PDF Needed | Abstract 검토 |
| 21 | RW-25 | TPPSUPPLY | Privacy verification | PDF Needed | Abstract 검토 |
| 22 | RW-34 | Product Source Verification with ZKP | Privacy verification | PDF Needed | Abstract 검토 |
| 23 | RW-16 | DPP with DID and VC | Supply Chain + DPP·Privacy | Local PDF | Abstract 검토 |
| 24 | RW-18 | Low-carbon hydrogen DPP | Supply Chain + DPP | Official Full Text | Abstract 검토 |
| 25 | RW-38 | DPP for recycling automation | Supply Chain + DPP | Official Full Text | Abstract 검토 |
| 26 | RW-41 | Healthcare-device lifecycle DPP | Supply Chain + DPP | PDF Needed | Abstract 검토 |
| 27 | RW-44 | Digital Twin-Based DPP | Supply Chain + DPP | Official Full Text | Abstract 검토 |

## 1. 제품 전이 모델

제품·batch·token을 표현하고 Transfer, Merge, Split, Process 또는 제조·조립 전이를 구체적으로 모델링하는 연구다.

| ID | 논문 | 연도·출처 | 확인할 제품 전이 | 원문 상태 | 분석 상태 |
|---|---|---|---|---|---|
| RW-01 | [Enabling Privacy and Traceability in Supply Chains using Blockchain and Zero Knowledge Proofs](https://doi.org/10.1109/Blockchain50366.2020.00024) | IEEE Blockchain 2020 | Entry·Ship·Merge·Split·Process·Exit와 document relation | Local PDF | [원문 분석](notes/01-sahai-2020.md) |
| RW-02 | [Tracing manufacturing processes using blockchain-based token compositions](https://doi.org/10.1016/j.dcan.2019.01.007) | DCN 2020; 최초 발표 2018 | Recipe에 따른 input token 소비와 output token 생성 | Local PDF | [이해 확정](notes/02-token-recipes-family.md) |
| RW-04 | [Enhancing Blockchain Traceability with DAG-Based Tokens](https://doi.org/10.1109/Blockchain.2019.00036) | IEEE Blockchain 2019 | Transfer·Merge·Split·Fork와 DAG history | Local PDF | [원문 분석](notes/04-dag-tokens-2019.md) |
| RW-19 | [Blockchain-based application for the traceability of complex assembly structures](https://doi.org/10.1016/j.jmsy.2021.04.013) | Journal of Manufacturing Systems 2021 | ERC-1155 기반 part·batch 조립 전이 | Local PDF | [원문 분석](notes/19-complex-assembly-traceability-2021.md) |
| RW-43 | [New Framework for Complex Assembly Digitalization and Traceability Using Bill of Assembly and Smart Contracts](https://doi.org/10.3390/app13031884) | Applied Sciences 2023 | 부품·공정·작업 자원을 연결한 Bill of Assembly와 단계별 smart contract | Official Full Text | Abstract 검토 |

## 2. Privacy-preserving Supply Chain Verification

Supply Chain의 provenance, transaction, credential 또는 제품 속성을 숨긴 상태에서 검증하거나 제한적으로 공개하는 연구다.

| ID | 논문 | 연도·출처 | 초점 | 원문 상태 | 분석 상태 |
|---|---|---|---|---|---|
| RW-03 | [PrivChain](https://doi.org/10.1109/Blockchain55522.2022.00030) | IEEE Blockchain 2022 | 위치·거래정보를 숨긴 provenance claim | Local PDF | [원문 분석](notes/03-privchain-2022.md) |
| RW-06 | [TradeChain](https://doi.org/10.1109/TrustCom53373.2021.00155) | IEEE TrustCom 2021 | 실제 identity와 거래 이력 분리 | Local PDF | [원문 분석](notes/06-tradechain-2021.md) |
| RW-09 | [ZKVeil](https://doi.org/10.1109/TIFS.2026.3660595) | IEEE TIFS 2026 | 자격·사양·거래량을 숨긴 compliance verification | Local PDF | [원문 분석](notes/09-zkveil-2026.md) |
| RW-24 | [ProChain](https://doi.org/10.1016/j.cie.2023.109831) | Computers & Industrial Engineering 2024 | 비공개 Supply Chain traceability | PDF Needed | Abstract 검토 |
| RW-25 | [TPPSUPPLY](https://doi.org/10.1016/j.jisa.2022.103116) | JISA 2022 | 익명성과 추적성의 선택적 제공 | PDF Needed | Abstract 검토 |
| RW-26 | [Blockchain-based privacy preservation for supply chains supporting lightweight multi-hop information accountability](https://doi.org/10.1016/j.ipm.2021.102529) | IPM 2021 | 다단계 정보 공유와 accountability | PDF Needed | Abstract 검토 |
| RW-33 | [DECOUPLES](https://doi.org/10.1145/3297280.3297318) | ACM SAC 2019 | unlinkable product traceability | PDF Needed | Abstract 검토 |
| RW-34 | [Scalable Supply Chain Product Source Verification Using Zero-Knowledge Proofs](https://doi.org/10.1109/ICCCT63501.2025.11019766) | IEEE ICCCT 2025 | 원산지·제조정보를 숨긴 제품 검증 | PDF Needed | Abstract 검토 |
| RW-42 | [Enhancing transparency and traceability in complex supply chains](https://doi.org/10.1016/j.jisa.2025.104169) | JISA 2025 | 암호화된 제품 정보 공유, 다자 정확성 평가와 CP-ABE | PDF Needed | Abstract 검토 |

## 3. Supply Chain + DPP

Supply Chain에서 생성되는 제품·지속가능성 정보가 downstream 참여자 또는 lifecycle 후반까지 이어지도록 DPP를 구성·갱신·공유하는 연구다.

| ID | 논문 | 연도·출처 | Supply Chain·DPP 연결 | 원문 상태 | 분석 상태 |
|---|---|---|---|---|---|
| RW-08 | [Decentralized Ledger Technology for EPCIS 2.0](https://doi.org/10.1109/Blockchain62396.2024.00018) | IEEE Blockchain 2024 | EPCIS product Event를 NFT operation에 연결 | Local PDF | [원문 분석](notes/08-epcis2-nft-2024.md) |
| RW-10 | [Zero-Knowledge Proof Extensions for Digital Product Passports](https://doi.org/10.3390/electronics15040745) | Electronics 2026 | Supply Chain sustainability claim을 DPP로 전달·검증 | Local PDF | [원문 분석](notes/10-dpp-zkp-extensions-2026.md) |
| RW-16 | [Digital Product Passport Management with Decentralised Identifiers and Verifiable Credentials](https://doi.org/10.48550/arXiv.2410.15758) | CoRR 2024 | 공급자·제조사의 VC를 결합해 DPP 구성 | Local PDF | Abstract 검토 |
| RW-17 | [Blockchain-based digital product passport: design principles and demonstration](https://doi.org/10.1080/00207543.2025.2464161) | IJPR 2025 | 원자재 조달부터 sustainability 정보를 누적하는 DPP | PDF Needed | Abstract 검토 |
| RW-18 | [Enhancing trust in global supply chains: Conceptualizing DPPs for a low-carbon hydrogen market](https://doi.org/10.1007/s12525-024-00690-7) | Electronic Markets 2024 | 수소 Supply Chain의 생산정보를 downstream에 공유 | Official Full Text | Abstract 검토 |
| RW-35 | [Human-Centric Digital Product Passports](https://eref.uni-bayreuth.de/id/eprint/92416) | HICSS 2025 | textile 공급망 정보를 검증 가능한 DPP로 소비자에게 전달 | Official Full Text | Abstract 검토 |
| RW-36 | [Decentralized Digital Product Passport Building Blocks](https://doi.org/10.1109/ACCESS.2025.3594826) | IEEE Access 2025 | AAS DPP의 identity·traceability·permission을 분산 구조로 관리 | Official Full Text | Abstract 검토 |
| RW-37 | [Connecting Producers and Recyclers](https://doi.org/10.1016/j.procir.2024.02.026) | Procedia CIRP 2024 | 생산자 제품 정보를 재활용업체의 수명 종료 처리에 연결 | Official Full Text | Abstract 검토 |
| RW-38 | [A Blockchain-Based Digital Product Passport System Providing a Federated Learning Environment](https://doi.org/10.3390/su17062679) | Sustainability 2025 | 제조사·재활용업체 간 제품 정보와 recycling model 공유 | Official Full Text | Abstract 검토 |
| RW-39 | [The Digital Product Passport: Enabling Interoperable Information Flows Through Blockchain Consortia](https://doi.org/10.1007/978-3-031-60433-1_21) | I4CS 2024 | 조직 간 DPP 정보 흐름과 blockchain consortium 연결 | PDF Needed | Abstract 검토 |
| RW-40 | [A Reference Architecture for Digital Product Passports at Batch Level](https://doi.org/10.1007/978-3-031-59465-6_19) | RCIS 2024 | batch·component 단위 정적·동적 lifecycle data 연결 | Official Full Text | Abstract 검토 |
| RW-41 | [Blockchain-Enabled Digital Product Passports for Enhancing Security and Lifecycle Management in Healthcare Devices](https://doi.org/10.1109/CSNet64211.2024.10851725) | IEEE CSNet 2024 | 의료기기의 제조·유지보수·사용 정보를 lifecycle DPP에 연결 | PDF Needed | Abstract 검토 |
| RW-44 | [A Digital Twin-Based Digital Product Passport](https://doi.org/10.1016/j.procs.2024.09.251) | Procedia Computer Science 2024 | 식품 원산지·생산·sustainability 정보를 lifecycle DPP로 공유 | Official Full Text | Abstract 검토 |

## 현재 관심 범위 밖: Physical Data Trust와 Physical-Digital Binding

| ID | 논문 | 연도·출처 | 제외 이유 | 원문 상태 |
|---|---|---|---|---|
| RW-05 | [TrustChain](https://doi.org/10.1109/Blockchain.2019.00032) | IEEE Blockchain 2019 | IoT observation과 reputation 기반 물품·참여자 신뢰가 중심 | Local PDF |
| RW-07 | [Blockchain-Based Supply Chain System for Traceability, Regulation and Anti-Counterfeiting](https://doi.org/10.1109/Blockchain53845.2021.00022) | IEEE Blockchain 2021 | 위조 방지와 규제기관 기록이 중심 | Local PDF |
| RW-15 | [Blockchain-Based Traceability Architecture for Mapping Object-Related Supply Chain Events](https://doi.org/10.3390/s23031410) | Sensors 2023 | 물리적 object와 Event의 연결이 중심 | Official Full Text |
| RW-22 | [Supply chain traceability using blockchain](https://doi.org/10.1007/s12063-023-00359-y) | OMR 2023 | certificate 기반 일반 제품 추적성 | Official Full Text |

## 현재 관심 범위 밖: 일반 System·Architecture·Traceability

| ID | 논문 | 연도·출처 | 제외 이유 | 원문 상태 |
|---|---|---|---|---|
| RW-13 | [Blockchain in Supply Chain](https://doi.org/10.1007/978-3-031-07535-3_17) | Handbook on Blockchain 2022 | 일반 설계 chapter | Local PDF |
| RW-14 | [DPP Implementation Based on Multi-Blockchain](https://doi.org/10.3390/app14114874) | Applied Sciences 2024 | 일반 DPP infrastructure와 DID provider | Official Full Text |
| RW-20 | [Tracking and tracing in manufacturing supply chains](https://doi.org/10.1016/j.procir.2022.10.069) | Procedia CIRP 2022 | 일반 tracking·tracing framework | Official Full Text |
| RW-21 | [Blockchain-enabled supply chain traceability – How wide? How deep?](https://doi.org/10.1016/j.ijpe.2023.108963) | IJPE 2023 | 추적 범위와 기록 granularity framework | Official Full Text |
| RW-23 | [Privacy preserving transparent supply chain management through Hyperledger Fabric](https://doi.org/10.1016/j.bcra.2022.100072) | BCRA 2022 | Fabric channel·private data 기반 접근통제가 중심 | Official Full Text |
| RW-27 | [Digital product passports for a circular economy](https://doi.org/10.1016/j.spc.2023.02.021) | SPC 2023 | DPP data needs 조사 | PDF Needed |
| RW-28 | [A proposed universal definition of a DPP Ecosystem](https://doi.org/10.1016/j.jclepro.2022.135538) | JCP 2023 | DPP ecosystem 정의 | PDF Needed |
| RW-29 | [Stop Guessing in the Dark](https://doi.org/10.3390/systems11030123) | Systems 2023 | DPP System 요구사항 도출 | Official Full Text |
| RW-30 | [Implementing a DPP to Support the Open-Source Hardware Community](https://doi.org/10.1007/978-3-658-44114-2_8) | Springer 2024 | 일반 DPP 구현 사례 | PDF Needed |
| RW-32 | [Blockchain-based framework for supply chain collaboration](https://doi.org/10.1080/00207543.2022.2039413) | IJPR 2022 | 자원 공유와 일반 supply-chain collaboration | PDF Needed |

## Review 혹은 Survey 논문

| ID | 논문 | 연도·출처 | 활용 목적 | 원문 상태 |
|---|---|---|---|---|
| RW-11 | [Digital product passports](https://doi.org/10.1007/s12525-026-00877-0) | Electronic Markets 2026 | DPP 정의·구성요소와 references 탐색 | Local PDF |
| RW-12 | [A Comprehensive Review of Digital Product Passports in Sustainable Manufacturing](https://doi.org/10.1007/s43615-026-00927-x) | Circular Economy and Sustainability 2026 | DPP lifecycle·architecture 연구 지형과 references 탐색 | Local PDF |
| RW-31 | [Trustworthy Digital Product Passports based on Distributed Ledgers](https://doi.org/10.1016/j.procs.2026.02.101) | Procedia Computer Science 2026 | DLT 기반 DPP references 탐색 | Official Full Text |
