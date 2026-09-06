# M5 Process·Policy Result

- 상태: 완료
- 구현 명세: [`M5-process-policy.md`](M5-process-policy.md)
- Background: [`M5-process-policy-background.md`](M5-process-policy-background.md)
- Raw 결과: [`m5-circuit.json`](../output/m5-circuit.json), [`m5-anvil-gas.json`](../output/m5-anvil-gas.json), [`m5-anvil-e2e.json`](../output/m5-anvil-e2e.json), [`m5-verifier-ablation.json`](../output/m5-verifier-ablation.json)
- 실행 횟수: Setup·gas·E2E·A/B ablation 각 1회

## 0. 30초 안에 기억 복구하기

| 질문 | M5 결과 |
|---|---|
| 이전 상태 | M4는 고정 Merge·Split만 있고 승인된 공정 Policy·Factory 권한이 없었습니다. |
| 이번 목표 | exact 3-to-2 Process와 Policy Authority·Record·Grant·Disable을 구현합니다. |
| 실제 구현 | PolicyRef·ScopeRef, 69,122-constraint Process, Registry, constant/storage VK verifier, Anvil runner |
| 이제 가능한 것 | 승인된 ZK Factory scope만 고정 비율 공정을 실행하고 ELIGIBLE·WASTE Note를 만들 수 있습니다. |
| 검증 | 전체 Go test와 Foundry 16개 test 통과, A/B가 같은 VK·proof를 검증 |
| 대표 성능 | Process 2,444,682 gas·1,848.457 ms; constant Verify 377,762 gas; storage Verify 473,513 gas |
| 미포함 | Product Type 현실 검증, private Policy, 비선형 공정, Status·Audit·Issue·Besu |
| 다음 단계 | M6에서 Active·Frozen·Revoked 상태 집행을 설계합니다. |

## 1. 실제 구현한 큰 그림

```text
Admin → Authority ID
Authority → Policy family/version 예약
Factory → ScopeRef
Certifier → Circuit·PK·VK
Authority → PolicyRecord·Grant
Factory → private Note 3개 Process
Contract → ELIGIBLE·WASTE Note
```

PolicyRef는 `(PROCESS,authorityId,policyId,version)`, ScopeRef는 `(sk_owner,policyRef)`에 Poseidon2로 binding됩니다. 같은 Policy version 안에서만 stable scope linking을 허용합니다.

## 2. Process 관계

Input 합 `(320,30,230)`에 질량 손실률 6.25%와 input kg당 추가 탄소 0.09375 kgCO2e를 적용했습니다.

```text
Intermediate = (300,30,260)
ELIGIBLE     = (270,30,260)
WASTE        = ( 30, 0,  0)
```

Rate·Allocation·Role은 Circuit constant입니다. Product Type·DocumentHash 관계와 근거 문서 진실성은 Circuit 밖 Certifier 책임입니다.

## 3. 공개·비공개값

| 공개값 | 비공개값 |
|---|---|
| `policyRef`, `policyScopeRef`, `noteRoot` | input Note·`cm`·path 3개, `sk_owner` |
| `nf1`, `nf2`, `nf3`, `cmEligible`, `cmWaste` | State·delta·remainder·output DocumentHash·opening |

Public input은 8개이며 consumed commitment는 포함되지 않습니다.

## 4. Policy Registry

| 상태 | 역할 |
|---|---|
| `policyAuthorities` | 등록된 EVM Policy Authority |
| `policyFamilies`·`policyReservations` | Setup 전 ID·version·PolicyRef 확정 |
| `policyRecords` | eventKind·arity·vkHash·verifier·enabled |
| `policyGrants` | ZK owner scope의 Policy 사용 권한 |

같은 PolicyRef 재등록·수정·재활성화는 불가능합니다. Grant는 revoke·regrant가 가능하고 Disable은 과거 output에 소급하지 않습니다.

## 5. Correctness

- PolicyRef Native·Circuit·Solidity 일치
- canonical Process와 다른 batch의 같은 rate 검증
- wrong scope·owner·path·delta·allocation·Role·remainder 실패
- 미등록·다른 Authority, missing Grant, disabled Policy 실패
- Registry·Process 실패 atomicity
- M2~M4 Event 회귀 없음
- Foundry 총 16개 test 통과

## 6. Circuit 결과

| Constraints | Public | Setup | Prove | Verify | Proof | Serialized VK | Optimized VK |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 69,122 | 8 | 3,588 ms | 1,843.182 ms | 2.646 ms | 664 B | 49,144 B | 38 words·1,216 B |

`vkHash`는 `4010319a62c7c2baa4af4e800ae863a196954b3ab911c94ac2f0bbe8f530cb82`입니다.

## 7. Registry·Process gas

| Transaction | Gas |
|---|---:|
| Authority 등록 | 96,302 |
| Policy 예약 | 217,247 |
| PolicyRecord 등록 | 103,485 |
| Grant | 58,454 |
| Policy disable | 34,760 |
| Process | 2,444,682 |

Process calldata는 1,380 B입니다. 최종 Note leaf는 5개이고 세 input nullifier·root가 Go oracle과 일치했습니다.

## 8. Live E2E

| Witness | Prove | Tx prepare | Submit→receipt | E2E |
|---:|---:|---:|---:|---:|
| 0.106 ms | 1,552.058 ms | 243.297 ms | 52.970 ms | 1,848.457 ms |

## 9. Verifier A/B ablation

| 항목 | Constant A | Storage B |
|---|---:|---:|
| Verifier deployment | 1,602,636 gas | 1,580,460 gas |
| Optimized VK 등록 | bytecode 포함 | 913,960 gas |
| 동일 proof Verify | 377,762 gas | 473,513 gas |

Storage B는 deployment+VK 등록이 A보다 891,784 gas 높고, 매 Verify도 95,751 gas 높았습니다. 단일 Policy·현재 38-word layout에서는 A가 등록과 반복 검증 모두 더 저렴했습니다. 이 결론은 Policy 수와 공통-code 배포 모델이 달라지면 다시 평가해야 합니다.

두 방식은 동일한 1,216 B optimized VK·vkHash·proof·public input을 사용했습니다. 49,144 B serialized Go VK 전체를 SSTORE하지 않았습니다.

## 10. 재현 명령

```text
make setup-m5
make test-go
make test-contract-m5
make benchmark-m5-gas
make benchmark-m5-e2e
make benchmark-m5-verifier-ablation
```

## 11. 제외·한계와 M6 연결

- Policy Authority와 Entry Issuer는 공개 EVM role입니다.
- 같은 Policy의 scopeRef 사용은 linkable합니다.
- WASTE 탄소·`a_rec` 0은 true-waste POC attribution입니다.
- Storage verifier는 custom-gate-free 38-word M5 layout만 지원합니다.
- Development SRS이며 Production setup이 아닙니다.
- 공식 성능은 단일 실행값입니다.

M6는 기존 Note·Voucher·향후 Claim이 Active일 때만 소비되도록 Frozen·Revoked 상태를 추가합니다.
