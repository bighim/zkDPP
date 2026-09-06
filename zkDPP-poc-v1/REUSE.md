# zkDPP-poc-v1 재사용 Registry

재사용은 코드 모양이 아니라 계산 관계, encoding, Domain, 공개범위와 실패 조건이 같은지로 판단합니다. 기존 프로젝트를 직접 import하지 않고 필요한 최소 구현을 현재 module에 둡니다.

이 Registry는 구현 근거를 추적하는 보조 문서입니다. Milestone 명세는 실제로 재사용하는 관계와 호출 방법을 자체적으로 설명해야 합니다.

## 상태

| 상태 | 의미 |
|---|---|
| 재사용 확정 | 실제 두 개 이상의 기능에서 같은 의미로 사용 |
| 수정 후 재사용 | 기반은 같지만 현재 의미에 맞게 변경 |
| 검토 후보 | 첫 사용만 확인했거나 다음 소비자 검증 필요 |
| 재사용하지 않음 | 현재 의미와 다르거나 범위 밖 |
| 보류 | 미래 milestone 결정 필요 |

## Registry

| 기능 | 상태 | 현재 경계 | 검증 근거 | 다음 사용 |
|---|---|---|---|---|
| Go·gnark 기반 | 재사용 확정 | Go 1.25.7, gnark 0.15.0, BLS12-381 PLONK | [M1 Result](milestones/M1-private-note-core-result.md) | 전체 |
| Poseidon2 Hash | 재사용 확정 | 용도별 Domain, Tree compression 분리 | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md) | 전체 |
| DocumentHash | 수정 후 재사용 | `(ProductName, LotID)`, NFC·길이 구분 | [M1 Result](milestones/M1-private-note-core-result.md) | 전체 Note |
| Owner address | 수정 후 재사용 | `H(OwnerTag, sk_owner)`, PublicKey 없음 | [M1 Result](milestones/M1-private-note-core-result.md) | 모든 소유 객체 |
| Note State 검증 | 재사용 확정 | uint64, `a_rec<=q_mass`, Role 조건 | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | M4~M8 |
| Note Commitment | 재사용 확정 | 동일 Field 순서·Domain | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | M4~M5 |
| Private Spend | 재사용 확정 | private `cm`·path, public root·nf | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | 모든 Note 소비 Event |
| Note nullifier | 재사용 확정 | Note Domain, `sk_owner`, private `cm` | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | M4~M8 |
| Voucher Commitment | 재사용 확정 | private payload, 생성 `rv` public, 소비 `rv` private | [M3 Result](milestones/M3-transfer-voucher-result.md) | Proceed·Recall·Audit |
| Voucher nullifier | 재사용 확정 | `H(VoucherNullifierTag,o_rv,rv)` | [M3 Result](milestones/M3-transfer-voucher-result.md) | 모든 Voucher resolution |
| Depth-32 membership | 재사용 확정 | private index·siblings | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | Note·Voucher Tree |
| On-chain Tree | 재사용 확정 | Note·Voucher가 같은 append/path helper, storage 분리 | [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | 후속 객체 Tree |
| Artifact runtime | 재사용 확정 | milestone별 CCS·SRS·PK·VK·checksum | [M1 Result](milestones/M1-private-note-core-result.md), [M2 Result](milestones/M2-entry-exit-ledger-result.md) | 전체 Circuit |
| Final universal SRS | 재사용 확정 | BLS12-381, max domain $2^{17}$, domain별 Lagrange·relation별 PK/VK | [M9 Result](milestones/M9-final-integration-result.md) | 현재 final Circuit 10개 |
| Public-input manifest | 재사용 확정 | relation별 ordered Field·domain·VK·verifier checksum | [M9 Result](milestones/M9-final-integration-result.md) | final Setup·Contract·fixture 대조 |
| Solidity verifier export | 재사용 확정 | VK별 generated verifier와 checksum | [M2 Result](milestones/M2-entry-exit-ledger-result.md) | EVM Event |
| Anvil runner | 재사용 확정 | clean chain gas와 live E2E 분리, epoch 이동 | [M2 Result](milestones/M2-entry-exit-ledger-result.md), [M3 Result](milestones/M3-transfer-voucher-result.md) | M4 이후 EVM |
| Voucher deadline | 재사용 확정 | 600초 epoch, private deadline, Recall만 strict 검사 | [M3 Result](milestones/M3-transfer-voucher-result.md) | Voucher Event |
| Mass allocation | 재사용 확정 | output 2 floor, output 1 residual, uint64 State | [M3 Result](milestones/M3-transfer-voucher-result.md), [M4 Result](milestones/M4-merge-split-result.md) | M5 Process 검토 |
| Multi-note spend | 재사용 확정 | 같은 root, private `cm`·path, input별 nf | [M4 Result](milestones/M4-merge-split-result.md) | M5 Process |
| Sequential Note output insert | 재사용 확정 | 중복 검사·append, transaction atomicity | [M3 Result](milestones/M3-transfer-voucher-result.md), [M4 Result](milestones/M4-merge-split-result.md) | M5 Process |
| Policy identity | 재사용 확정 | Authority family/version PolicyRef; Process만 owner별 ScopeRef 사용 | [M5 Result](milestones/M5-process-policy-result.md), [M8 Result](milestones/M8-exit-dpp-issue-result.md) | M9 통합 |
| Policy Registry | 재사용 확정 | immutable Record·one-way disable; Grant는 Process에만 적용 | [M5 Result](milestones/M5-process-policy-result.md), [M8 Result](milestones/M8-exit-dpp-issue-result.md) | M9 통합 |
| Optimized VK encoding | 재사용 확정 | custom-gate-free 38-word BLS12-381 PLONK VK | [M5 Result](milestones/M5-process-policy-result.md) | M9 evaluation |
| Sparse StatusTree | 재사용 확정 | M6 내부의 Note·Voucher에서 검증한 baseline | [M6 Result](milestones/M6-status-enforcement-result.md) | M6에 보존; M7 Main 경로에는 사용하지 않음 |
| Active Status gadget | 재사용 확정 | M6의 같은 private index·current Status root 검사 | [M6 Result](milestones/M6-status-enforcement-result.md) | M6에 보존; M7에서 mapping 조회로 대체 |
| StatusUpdate proof | 재사용 확정 | M6 one-leaf update·immutable Authority | [M6 Result](milestones/M6-status-enforcement-result.md) | M6에 보존; M7 경로에는 사용하지 않음 |
| Status-aware Event wrapper | 재사용 확정 | M6에서 검증한 input별 Active path 추가 | [M6 Result](milestones/M6-status-enforcement-result.md) | M6에 보존; M7 감사 Event가 대체 |
| Merge compatibility·rounding | 재사용 확정 | M4 POC의 같은 owner·Role, output 2 floor·output 1 residual; ProductProfile 미구현 | [M4 Result](milestones/M4-merge-split-result.md) | 후속 Event 통합 시 동일 의미 유지 |
| AuditRecord·추적·nf 상태 통합 | 재사용 확정 | 8개 Event·실제 원장·snapshot 양방향 추적·종류별 nf 상태 | [M7 Result](milestones/M7-audit-tracing-result.md) | M8 Claim 연결 |
| DPP commitment | 재사용 확정 | owner와 분리된 DocumentHash·Role·State·dppOpening; Exit와 Issue에서 동일 | [M8 Result](milestones/M8-exit-dpp-issue-result.md) | M9 통합 |
| Issue Claim·상태 | 검토 후보 | `(dppCommitment, issuePolicyRef)` 조합, version별 Active·Frozen·Revoked | [M8 Result](milestones/M8-exit-dpp-issue-result.md) | 후속 ownership·중간 Claim 검토 |
| 감사 암호화 코어·gadget | 재사용 확정 | 동일 Jubjub·Poseidon2 관계, 고정 위원회 PK, 길이별 Field 마스크; 독립 1/5-Field와 Process adapter에서 검증 | [M6-B1 Result](milestones/M6-B1-audit-encryption-core-result.md) | 다른 Event는 개별 평문 binding 검증 필요 |
| 2-of-3 원문 복원 | 재사용 확정 | 정직한 위원회·검증된 원본 전용, 같은 감사 실행의 메모리 캐시 | [M6-B1 Result](milestones/M6-B1-audit-encryption-core-result.md), [M7 Result](milestones/M7-audit-tracing-result.md) | M8 Claim 감사 검토 |

M6-B1의 재사용 근거는 암호화 관계와 대표 Process 연결입니다. 모든 Event·Voucher 적용이나 기존 StatusTree 대체를 완료했다는 의미는 아닙니다.

M7은 같은 암호화 코어를 8개 Event에 적용하고 StatusTree 대체까지 검증했습니다. B1의 15개 public input 저장 Contract는 진단 baseline이며 M7 AuditRecord 경계로 승격하지 않습니다.

M9에서 final SRS runtime·public-input manifest·Policy verifier routing과 전체 Anvil 시나리오를 실제 검증했습니다. 근거는 [M9 Result](milestones/M9-final-integration-result.md)이며, $2^{17}$보다 큰 Circuit이나 Production Ceremony로의 재사용을 의미하지 않습니다.

M6 행의 재사용 확정은 **그 단계 내부에서 검증한 사실**입니다. 후속 M7에서도 StatusTree를 계속 사용해야 한다는 의미는 아닙니다. B1 진단 Contract의 15개 값 저장 역시 M7에 그대로 가져올 재사용 조건이 아닙니다. M7은 암호문·outputRefs를 AuditRecord에 보관하되 검증용 public input 전체를 복사하지 않습니다.

## 재사용하지 않는 과거 방식

- `Quantity + State[3]`
- 소비 `cm`을 public input으로 두는 방식
- `sk → pk → address`
- 고정 최대 slot과 inactive padding
- 과거 Event Circuit을 그대로 가져오는 방식

## Shared Core 승격 조건

두 개 이상의 실제 기능에서 다음 조건이 모두 같아야 합니다.

1. 같은 입력으로 같은 관계를 계산합니다.
2. Field 순서·bit 범위·물리 단위가 같습니다.
3. 같은 Hash와 Domain을 사용합니다.
4. 공개값·비공개값 경계가 같습니다.
5. 같은 잘못된 입력을 거부합니다.

한 조건이라도 다르면 Feature 구현을 분리하거나 차이를 명시한 adapter를 사용합니다.

## 갱신 규칙

- Milestone 시작 전 관련 Registry 행을 확인합니다.
- 두 번째 실제 소비자가 생기면 검토 후보를 승격하거나 분리 유지합니다.
- 수식·Domain·공개범위가 바뀌면 재사용 확정도 다시 검토합니다.
- 검증 근거는 Result 링크로 기록하고 성능·Event 설명을 이 문서에 반복하지 않습니다.
