# Future Milestone Context

이 문서는 M1 이후의 합의·방향·미정·폐기사항을 보존합니다. 구현 명세가 아니며 이 문서만 보고 코드를 작성하지 않습니다.

## M2

### Jubjub 2-of-3 DKG

#### [합의]

- Committee 3명이 dealer 없이 공동 public key를 생성합니다.
- Threshold는 2-of-3입니다.
- 실제 구현 Curve는 Jubjub이고 현재 zkDPP의 Field·Point encoding을 사용합니다.
- 각 Committee member는 자기 private share만 보관합니다.
- Auditor는 DKG 과정에서 Master Key를 알지 못합니다.
- 로컬 3-party Protocol로 correctness·round·message·byte·시간·메모리를 측정합니다.
- M1의 고정 ExternalKeyPackage를 실제 DKG output으로 교체합니다.

#### [참고]

- Coinbase cb-mpc의 EC-DKG와 TDH2 `dkg_ac`는 Protocol 구조와 실패 검증의 참고 자료입니다.
- 참고 범위는 party·session ID, access structure, public/private share, public-key 합의, partial-decryption quorum과 잘못된 입력 거부입니다.
- v1 M9도 Coinbase TDH2를 참고했지만 Jubjub·Poseidon2 Core를 별도로 구현했습니다.

#### [미정]

- 악의적인 share에 대한 complaint·제외·재시도 절차
- transport 인증과 Committee 프로세스 통신
- private share의 암호화 저장·backup·복구
- DKG session timeout·중단·재개
- 승인된 감사에서 Master Key를 안전하게 release하는 Interface

#### [폐기]

- P-256·secp256k1 채택
- Coinbase C++ runtime 직접 연결
- Coinbase TDH2를 그대로 사용했다고 주장
- Auditor가 Master Key를 생성하고 Committee share로 분배
- M1 fixture를 DKG 구현 완료의 근거로 사용

## M3

### Audit Key Rotation

#### [합의]

- Key Rotation은 이전 Master Key를 공개하기 전에 완료합니다.
- 새 DKG key를 먼저 등록·활성화하고 이전 key를 Retired로 바꿉니다.
- key 상태는 `Pending → Active → Retired`이며 Retired key를 재활성화하지 않습니다.
- 정확히 하나의 key만 신규 AuditRecord에 Active입니다.
- 전환 블록 번호와 블록 Hash를 Snapshot으로 고정합니다.
- AuditRecord는 자신이 사용한 `auditKeyId`를 공개적으로 기록합니다.
- 이전 key가 유출돼도 새 key 기간의 Record를 복호화할 수 없어야 합니다.

#### [방향]

- 온체인 Audit Key Registry를 추가합니다.
- Committee DKG package에는 key ID·Jubjub public key·session·public-share 또는 transcript checksum이 포함됩니다.
- Committee quorum이 승인한 key package만 Status Authority가 제출할 수 있게 합니다.
- M1이 선택한 public-key Circuit 상수는 M3에서 public input·registry binding으로 변경합니다.
- 이 변경으로 감사 Event Circuit·CCS·PK·VK·proof·verifier를 다시 생성합니다.

#### [미정]

- Committee quorum signature와 on-chain encoding
- Pending key의 배포·활성화 시점
- DKG 또는 activation 실패 시 기존 key 유지 규칙
- 과거 private share의 재감사 보관 기간
- Master Key·Committee share release 자료의 폐기 절차
- 여러 key 기간을 지나는 graph 감사의 key 요청 순서

#### [보류]

- post-snapshot에서 먼저 소비된 leaf는 Active key의 해당 AuditRecord만 기존 M9 방식으로 partial decrypt하여 자손을 따라갈 수 있습니다.
- 이 fallback은 Key Rotation을 leaf 하나 때문에 반복하지 않는 장점이 있지만, 반복 경합·Committee interaction·새 Snapshot 의미를 별도로 정해야 합니다.
- M1에서는 가능성만 Background에 기록하고 구현·측정하지 않습니다.

#### [폐기]

- Auditor의 “감사 종료” Message만으로 Rotation 시작
- 이전 key를 먼저 공개하고 나중에 새 DKG 실행
- Auditor가 자신이 아는 key를 임의 등록
- 하나의 Audit key를 전체 배포 기간 동안 영구 사용
