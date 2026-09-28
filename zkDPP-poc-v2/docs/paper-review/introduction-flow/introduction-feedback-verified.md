# Introduction 피드백의 근거와 표현 범위

동료에게 보낼 문안은 [전달용 문서](introduction-feedback-for-colleague.md)에 있습니다.  
이 문서는 그 문안의 근거만 정리하며, 확인한 사실과 zkDPP에 대한 제안을 구분합니다.

## 1. 정기 감사와 Event별 검증을 어떻게 설명할 것인가?

**전달할 핵심은 “제3자가 매 거래마다 감사한다”가 아니라, “외부에서 확인한 자료·규칙을 바탕으로 Event마다 계산을 검사한다”입니다.**  
실물 측정·문서 검토와 자동화된 규칙 검증은 대상이 다르므로, 검사 횟수만으로 어느 방식이 더 우수하다고 결론 내리지 않습니다.

### Split·Merge와 Entry·Process의 차이

Split·Merge도 새 Note와 출력 State를 만들지만, 입력에 적용할 합산·배분 규칙이 정해져 있습니다.  
따라서 “새 State를 만드는가”보다 **“기존 입력과 정해진 규칙만으로 확인할 수 있는 계산인가, 외부 사실이나 공정 판단이 필요한가”**로 구분하는 것이 정확합니다.

Entry의 최초 값과 실물의 일치 여부는 외부 판단입니다.  
Process 역시 출력을 임의로 정하는 것이 아니라 승인된 공정 규칙을 따라야 하며, 그 규칙의 적절성과 공정·측정 정보의 진실성은 별도 확인이 필요합니다.

현재 구현은 승인된 Entry 제출자와 등록된 Process 정책을 확인하지만, 매 실행마다 새로운 제3자 검토서나 승인을 요구하지는 않습니다.  
매 Entry·Process마다 독립된 제3자가 자료를 심사하도록 하려면, 그 부분은 **추가 운영 정책 또는 새 요구사항**으로 정의해야 합니다.

외부 확인이 필요한 곳을 Entry·Process로만 한정하는 것도 정확하지 않습니다.  
예를 들어 현재 Transfer의 운송 탄소 증분도 외부에서 제공하며, 시스템은 수치 관계를 검사하지만 실제 배출량이 맞는지 측정하지 않습니다.

### 주장할 수 있는 장점

정해진 계산 규칙에 어긋나는 거래를 다음 정기 감사까지 기다리지 않고, 원장에 수락되기 전에 검사·거부할 수 있습니다.  
이는 **검증하는 시점을 Event마다 두는 구조적 장점**이며, 실물 감사의 대체나 Greenwashing 감소 효과를 입증한 실험 결과는 아닙니다.

## 2. 지속적인 확인이 필요하다는 외부 근거

### OECD: 신규 제품과 공급 관계 변화에 맞춘 재평가

[OECD Due Diligence Guidance for Responsible Business Conduct (2018)](https://www.oecd.org/content/dam/oecd/en/publications/reports/2018/02/oecd-due-diligence-guidance-for-responsible-business-conduct_c669bd57/15f5f4b3-en.pdf)의 **인쇄면 17쪽 “Due diligence is dynamic”**은 실사를 지속적이고 변화에 대응하는 과정으로 설명합니다.  
**인쇄면 25–26쪽, 제II부 2.1(f–g)·2.2(g)**는 정기적인 검토와 신규 제품·투입물·사업 관계 등 중요한 변화에 따른 재평가를 제시합니다.

이 자료는 **연간 확인에만 의존하지 않고 변화에 맞춰 점검할 필요성**을 뒷받침합니다.  
다만 모든 거래의 제3자 현장 감사나 ZK 검증을 요구하는 문서는 아니며, Event별 검증은 이 문제에 대한 zkDPP의 접근입니다.

### ISCC: 연간 감사 외에도 추가 점검

[ISCC Integrity Programme (2025-02-14)](https://iscc-system.org/iscc-integrity-programme-for-a-more-robust-certification-system/)의 On-site Audits는 인증기업에 대한 연간 정기 감사와 추가 Integrity Assessment를 설명합니다.  
따라서 연간 감사는 **특정 인증제도의 사례**로만 사용하고, 모든 업체가 연 1회만 확인받는다고 일반화하지 않습니다.

[ISCC 공식 Surveillance Audit 안내](https://contact.iscc-system.org/support/solutions/articles/103000349207-what-is-a-surveillance-audit-and-when-must-a-surveillance-audit-be-conducted-)는 EU·CORSIA의 특정 고위험 공급망에서 최초 인증 후 6개월, 일부 수집·거래 조건에서는 3개월의 추가 감사를 설명합니다.  
ISCC PLUS의 새 추가 감사 규칙은 2027-01-01 의무 적용이며 이전 v3.4.2와 다르므로, 제도·버전·조건을 생략해서 인용하지 않습니다.

이 근거는 위험에 따라 확인 시점이 달라진다는 점을 보여줍니다.  
“정기 감사 사이에는 아무 검증도 없다”거나 “감사 횟수를 늘리면 무조건 안전하다”는 결론은 도출하지 않습니다.

### Textile Exchange: 기업 인증과 실제 출하 제품 확인의 구분

[Textile Exchange 공식 FAQ](https://hub.textileexchange.org/home/site-wide-content/standards-faqs)의 Scope Certificates·Transaction Certificates 설명을 확인했습니다.  
Scope Certificate는 기업의 인증 범위를 나타내며, Transaction Certificate는 실제 인증 제품과 출하 정보를 확인하는 문서입니다.

이는 **기업이 인증받았다는 사실과 개별 거래 제품의 확인을 구분하는 실무 사례**입니다.  
조건에 따라 여러 출하를 하나의 Transaction Certificate에 포함할 수 있으므로, “Event 한 번마다 별도 인증서 하나”라고 해석하지 않습니다.

### ISCC PLUS: 입력·출력과 생산 데이터를 계속 관리해야 하는 이유

[ISCC PLUS Mass Balance Guidance, 제4절, PDF 13–14쪽](https://iscc-system.org/wp-content/uploads/2026/02/Mass-Balance-Guidance-document.pdf)은 입력·출력·손실·생산·배분 데이터를 수집·기록하도록 설명합니다.  
출력의 특성을 계산하는 변환·소비 계수도 실제 운영 데이터에 근거해야 한다고 명시합니다.

이 자료는 초기 인증만이 아니라 생산 과정의 자료와 계산을 관리해야 한다는 설명에 사용할 수 있습니다.  
해당 문서의 mass balance 규칙이 zkDPP와 같거나, zkDPP가 ISCC 인증 요건 전체를 충족한다는 뜻은 아닙니다.

## 3. 나머지 피드백의 근거

### Greenwashing과 Claim의 근거

[European Commission — Green claims](https://environment.ec.europa.eu/topics/circular-economy-topics/green-claims_en)는 근거가 부족하거나 소비자에게 잘못된 인상을 주는 환경 주장 문제를 설명합니다.  
재활용 플라스틱 함량과 제품 관련 CO₂ 배출 감소를 환경 주장 사례로 제시합니다.

이는 Claim을 뒷받침하는 자료를 확인해야 한다는 배경입니다.  
모든 기업이 고의로 수치를 속인다거나 zkDPP가 모든 허위 측정을 방지한다는 근거는 아닙니다.

### Recall이 필요하다고 보았던 이유

[Research Context and Design Evolution — Revert의 의미와 필요성](</Users/bighim/zkDPP-worktrees/zkDPP/Research Context and Design Evolution.tex:163>)에는 미지급·잘못된 수신자·잘못 입력한 탄소량·재활용률·수량이 도입 동기로 기록되어 있습니다.  
[RecallVoucher의 한계](</Users/bighim/zkDPP-worktrees/zkDPP/Research Context and Design Evolution.tex:241>)에서는 먼저 인수가 확정되면 판매자의 Recall 권한이 사라진다고 설명합니다.

이 내용은 회수 가능한 이전을 생각하게 된 **역사적 동기**입니다.  
현재 Recall은 미해결 이전을 기한 조건 안에서 되돌리는 기능이며, 대금 지급·실물 수령을 자동 확인하거나 이미 진행된 하류 거래를 취소하는 기능은 아닙니다.

### Traceability와 Privacy를 함께 고려할 이유

[Sahai 등의 논문 소개](https://research.ibm.com/publications/enabling-privacy-and-traceability-in-supply-chains-using-blockchain-and-zero-knowledge-proofs)와 논문 Introduction의 인쇄면 134–135쪽을 확인했습니다.  
논문은 출처·오염 관련 추적의 필요성을 설명하고, 경쟁자가 같은 공급망에 있으므로 민감한 사업정보 공개가 부담된다는 점을 제시합니다.

포장·분할·병합·가공까지 고려해야 한다는 설명도 함께 참고할 수 있습니다.  
이는 비공개 추적의 필요성을 뒷받침하지만, Privacy와 Traceability가 원리적으로 양립 불가능하다거나 zkDPP가 이를 최초로 해결했다는 근거는 아닙니다.

## 4. 내부 설계와 구현에서 확인한 범위

### 외부 확인과 규칙 검증의 역할

[Research Context and Design Evolution — 핵심 가정](</Users/bighim/zkDPP-worktrees/zkDPP/Research Context and Design Evolution.tex:340>)은 최초 물품·정보 확인 또는 인증 업체에 대한 Entry 권한 부여와, Entry 이후 State 변화의 검증을 구분합니다.  
[Process 설명](</Users/bighim/zkDPP-worktrees/zkDPP/Research Context and Design Evolution.tex:596>)의 공정별 검증키 등록은 공정 규칙의 승인에 관한 내용이며, 매 생산 건마다 별도 제3자 현장 심사가 있다는 근거로 읽지 않습니다.

[Product·State·Process 설계의 역할 구분](</Users/bighim/zkDPP-worktrees/zkDPP/Conversation History/Previous Conversation/zkDPP-Product-State-Process-설계.md:248>)은 제3자가 공정·배분 규칙의 근거를 검토하고, Circuit은 승인된 수치 관계를 검사한다고 설명합니다.  
[v2 Protocol의 Entry 설명](</Users/bighim/zkDPP-worktrees/zkDPP/Conversation History/zkdpp-v2-protocol.md:642>)은 실물 수치의 진실성과 동일 물량의 중복 등록을 ZK가 알아내는 것은 아니라고 명시합니다.

### 현재 실험 구현

[Entry](/Users/bighim/zkDPP-worktrees/zkDPP/zkDPP-poc-v2/contracts/src/experiments/ExperimentalAuditLedger.sol:119)는 제출 권한을 검사합니다.  
[Process](/Users/bighim/zkDPP-worktrees/zkDPP/zkDPP-poc-v2/contracts/src/experiments/ExperimentalAuditLedger.sol:162)는 이미 등록된 정책과 사용 권한을 확인하므로, 정책 승인과 매 Event의 자동 검증을 구분해야 합니다.

[Split·Merge 등의 Circuit](/Users/bighim/zkDPP-worktrees/zkDPP/zkDPP-poc-v2/internal/experiments/circuit.go:276)은 합산·배분·범위 관계를 검사합니다.  
[Transfer의 운송 탄소 처리](/Users/bighim/zkDPP-worktrees/zkDPP/zkDPP-poc-v2/internal/experiments/circuit.go:341)는 외부 증분을 반영하므로, 외부 사실 확인이 Entry·Process에만 필요하다고 단정할 수 없습니다.

### 문서에서 사용할 표현

- **사용할 표현:** 외부에서 초기 자료와 공정 규칙을 확인하고, 시스템은 승인된 규칙을 Event마다 검증해 정기 감사를 보완한다.
- **피할 표현:** 제3자가 Entry·Process를 매번 직접 감사하므로 기존 연 1회 감사보다 우수하다.
- **구분할 내용:** 매 Event의 규칙 검증은 현재 기능이며, 매 생산 건에 대한 새 제3자 승인은 별도로 정해야 할 운영 방식이다.

이번 확인은 연구 배경과 표현 범위를 정리하기 위한 것입니다.  
실제 부정행위 감소나 감사 비용 절감을 측정하지 않았으며, 외부 자료를 zkDPP의 성능 우위나 보안 증명의 근거로 사용하지 않았습니다.
