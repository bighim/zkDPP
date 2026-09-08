# zkDPP-poc-v2 Architecture

현재 v2 M1은 구현을 완료했습니다. 이 문서는 **M1에서 검증한 invariant와 v1 보존 원칙**을 기록합니다.

## 전체 구조

```text
Participant
  → private Note·Voucher와 Audit 평문 준비
  → Event Circuit proof 생성

Main Contract
  → root·spent·Status·Policy·proof 확인
  → Tree·AuditRecord·producer·spent·Claim 상태 갱신

Committee
  → 외부 DKG로 2-of-3 share를 분배했다고 가정

Status Authority = Auditor
  → 두 share로 Master Key 복구
  → AuditRecord 복호화·graph 추적
  → frontier Freeze transaction 제출
  → 공통 결과 Snapshot 확인
```

## M1 설계 경계

- Note와 Voucher는 분리된 depth-32 append-only Merkle Tree를 사용합니다.
- 공개 commitment·private 소비·nullifier 기반 Status 집행은 v1 M9 구조를 유지합니다.
- Exit는 Note를 소비하고 output 없이 종료합니다.
- Issue는 ELIGIBLE Note를 소비하고 terminal ClaimRef를 생성합니다.
- DPP는 외부 Product Passport이며 온체인 DPP 객체를 만들지 않습니다.
- Claim은 등록 여부만 가지며 Tree·nullifier·Status가 없습니다.
- AuditRecord는 실제 부모와 output 미래 소비값을 암호화해 저장합니다.
- Master Key 복구·Forward Tracing·AuditAndFreeze는 오프체인입니다.
- Contract에는 복호화 key·plaintext·graph 전체를 제출하지 않습니다.
- Freeze는 Note·Voucher의 공개 (nf)·(rvnf)를 이용한 개별 transaction입니다.

## 구현으로 확인한 경계

- 10개 Event relation은 같은 131,075-point development universal SRS를 사용합니다.
- Process와 Issue는 PolicyRecord의 verifierRef로 선택하고, 나머지 7개 Event verifier는 constructor에서 고정합니다.
- ZkDPPV2Ledger는 Exit의 무출력 기록과 Issue의 terminal Claim을 원자적으로 저장합니다.
- AuditAndFreeze는 Contract 함수 하나가 아니라 Master Key 복구·오프체인 추적·개별 Status transaction·checkpoint 확인의 순서입니다.
- 감사 알고리즘은 in-memory 단위 테스트와 실제 Anvil Snapshot RPC 실행을 모두 검증했습니다.
- RPC 감사는 전체 블록을 복사하지 않습니다. Snapshot block을 지정해 `producerOf`, AuditRecord, `noteSpentIn`·`voucherSpentIn`, Status를 필요한 경로만 조회합니다.
- 운영용 장기 실행 server와 영구 cache는 M1 stable invariant가 아닙니다.

## 후속 변경 경계

- M2는 고정 share fixture를 Jubjub DKG output으로 교체합니다.
- M3은 M1의 Circuit 상수 public key를 key ID·registry 기반으로 변경하며 Circuit·Artifact 재생성을 허용합니다.
- post-snapshot 자손 fallback은 설계 가능성만 보존하고 현재 구현 invariant로 두지 않습니다.
