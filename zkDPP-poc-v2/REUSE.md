# zkDPP-poc-v2 Reuse

M1 Result를 기준으로 아래 항목은 **실제 재사용 또는 변경 검증을 완료했습니다.** 범용 재사용을 뜻하지 않으며 M1의 10개 relation과 로컬 감사 POC 경계에 한정합니다.

## v1 M9에서 재사용할 예정인 것

| 항목 | 검증한 경계 | 변경된 부분 |
|---|---|---|
| Note·Voucher model | Field 순서·State·commitment | nullifier 입력 순서, WASTE 소비 경계 |
| Merkle Tree | depth 32 append·accepted roots·private path | 의미 변경 없음 |
| Policy Registry | Authority·version·immutable verifier·disable | Issue input·Claim lifecycle 변경 |
| Process | 3-to-2 State 계산·ScopeRef·Grant | Audit 암호화 Core 교체 |
| Status | Note·Voucher의 Active·Frozen·Revoked mapping | Claim Status 제거 |
| AuditRecord | outputRefs·부모·미래 소비값·producer·spent | Master Key 암호화, Exit·Issue shape 변경 |
| Audit RPC | block number·block hash Snapshot 조회 | Master Key 복호화와 AuditAndFreeze 결과 추가 |
| PLONK·Jubjub·Poseidon2 | Circuit·proof·Field·Point 기반 | (L,n) 제거, public key 상수 교체 |

## 직접 재사용하지 않는 것

- v1 generated verifier·PK·VK·proof
- v1 M9 Main Contract state
- v1 DPP commitment·Claim status
- v1 Event별 Audit ciphertext
- Coinbase P-256·secp256k1 TDH2 runtime

## M1에서 새로 재사용 가능한 항목

- `internal/core/auditcrypto`: 한 공개키와 per-record·per-position mask를 사용하는 Field 암호화, Master Key 복구·복호화
- `internal/v2circuit`: 실제 Event의 부모와 미래 소비값을 암호문에 결합하는 공통 gadget
- `internal/v2audit`: Master Key 기반 Forward Trace, Snapshot RPC source와 전체 frontier의 AuditAndFreeze 순서
- `internal/v2dpp`: 외부 DPP의 DocumentInfo·DPPClaim과 온체인 terminal Claim을 Snapshot에서 대조하는 client 경계
- `internal/v2run`: 하나의 development universal SRS에서 10개 PLONK relation을 Setup·재로딩·검증하는 경로

구현 뒤에는 실제 코드·test·Raw Result 링크를 추가해 재사용 경계를 확정합니다.
