# M1 결과 — Master Key 감사와 v2 Event가 어떻게 동작하나요?

## 30초 결과

M1에서는 **외부 DKG 결과를 흉내 낸 2-of-3 share 중 두 개로 Master Key를 복구하고, 그 key로 여러 AuditRecord를 연속 복호화한 뒤, Forward Tracing으로 찾은 미소비 Note를 모두 Freeze하는 흐름**을 구현했습니다.

동시에 공급망 관계를 다음처럼 바꿨습니다.

- **Exit:** Note를 소비하고 후속 객체를 만들지 않습니다.
- **Issue:** ELIGIBLE Note를 소비하고 terminal Claim $h$를 만듭니다.
- **Transfer:** Voucher의 탄소에 비공개 운송 탄소를 더합니다.
- **Deadline:** timestamp나 Epoch 대신 절대 `block.number` $D$를 사용합니다.
- **Status:** Note와 Voucher만 nullifier 기준으로 Active·Frozen·Revoked 상태를 가집니다.

10개 Circuit은 하나의 development universal SRS를 사용했습니다. 가장 큰 Process가 $2^{17}$ domain을 요구했고, 저장한 PK·VK를 다시 읽어 10개 proof를 모두 검증했습니다.

## 무엇을 실제로 구현했나요?

### 1. 객체와 회계 관계

- DocumentHash는 ProductName·LotID·Unit을 정규화한 뒤 함께 결합합니다.
- Note nullifier는 $nf=H_{nf}(cm,sk_{owner})$ 순서로 계산합니다.
- Voucher는 opening에서 $s_{res}$를 만들고, $rv$와 $s_{res}$로 $rvnf$를 계산합니다.
- Transfer의 $delta\_e\_transport$는 Voucher 탄소에만 더하며 uint64 overflow를 거부합니다.
- Merge·Split은 ELIGIBLE Note와 같은 DocumentHash만 허용합니다.
- WASTE는 Exit 이외의 소비 Circuit에서 거부합니다.

### 2. Master Key 감사 암호화

각 AuditRecord는 같은 공개키 $PK_A$를 사용하지만, **매 기록마다 새로운 난수와 key를 사용합니다.** 또한 한 기록 안에서도 Field 위치마다 서로 다른 mask를 파생합니다.

위원 두 명이 share를 공개하면 Status Authority가 $SK_A$를 복구합니다. 복구한 key로 $SK_A G=PK_A$를 확인한 뒤, 해당 key 기간의 AuditRecord를 복호화합니다. Master Key와 share 값을 Raw JSON에는 저장하지 않았습니다.

### 3. Event Circuit 10개

| 관계 | 공개 입력 | Constraints | Prove | Verify |
|---|---:|---:|---:|---:|
| Entry | 4 | 18,117 | 529.54 ms | 2.70 ms |
| Transfer | 10 | 49,627 | 869.74 ms | 3.00 ms |
| Proceed | 7 | 38,563 | 872.92 ms | 2.67 ms |
| Recall | 8 | 35,987 | 840.69 ms | 2.62 ms |
| Merge | 9 | 55,749 | 861.11 ms | 2.59 ms |
| Split | 9 | 50,455 | 871.50 ms | 2.63 ms |
| Process | 15 | 90,568 | 1,649.80 ms | 2.66 ms |
| Exit | 5 | 30,043 | 490.38 ms | 2.61 ms |
| Issue Standard | 7 | 35,957 | 835.23 ms | 2.61 ms |
| Issue Strict | 7 | 35,957 | 833.70 ms | 2.80 ms |

모든 proof 크기는 664 bytes였습니다. 이 값은 GOMAXPROCS=8인 로컬 단일 측정이며 평균이나 운영 환경 성능으로 해석하지 않습니다.

### 4. ZkDPPV2Ledger

Contract는 다음 상태를 함께 갱신합니다.

- Note·Voucher append-only Tree
- `noteSpentIn`·`voucherSpentIn`
- 객체의 생성 AuditRecord를 찾는 `producerOf`
- 부모와 미래 소비 nullifier를 담는 AuditRecord
- terminal Claim의 `claimRegistered[h]`
- Note·Voucher의 nullifier 기반 Status
- Process·Issue verifier를 선택하는 PolicyRecord

Event가 실패하면 Tree·spent mapping·producer·AuditRecord가 함께 되돌아갑니다. Exit는 output이 없는 AuditRecord를 만들고, Issue는 Claim output 하나를 기록합니다. DPP commitment·Claim Status·Claim Tree는 v2 Contract에 없습니다.

Foundry에서는 Contract 상태 전이 5개와 실제 generated Entry verifier 1개, 총 6개 테스트가 통과했습니다. 상태 전이 테스트는 9개 Event API, absolute block deadline, Status, Exit·Issue, 중복 소비와 실패 원자성을 확인했습니다.

## Master Key 복구와 감사 결과는 어땠나요?

### Master Key 복구

| 위원 조합 | 복구·검사 시간 | $PK_A$ 일치 |
|---|---:|---|
| 1 + 2 | 0.186 ms | 성공 |
| 1 + 3 | 0.180 ms | 성공 |
| 2 + 3 | 0.179 ms | 성공 |

### 여러 기록 복호화

각 기록에 Field 5개가 있을 때의 순차 복호화 결과입니다.

| AuditRecord 수 | 전체 복호화 시간 | 원문 일치 |
|---:|---:|---|
| 1 | 0.179 ms | 성공 |
| 10 | 2.569 ms | 성공 |
| 100 | 20.113 ms | 성공 |
| 1,000 | 169.051 ms | 성공 |

### AuditAndFreeze

대표 graph는 $A \rightarrow (B,C)$, $B \rightarrow (D,E)$입니다.

1. 블록 번호와 블록 Hash로 감사 Snapshot을 고정했습니다.
2. A에서 Forward Tracing하여 미소비 frontier C·D·E를 찾았습니다.
3. 세 nullifier를 개별 Freeze했습니다.
4. 하나의 결과 checkpoint에서 세 대상이 모두 미소비·Frozen인지 다시 확인했습니다.

먼저 같은 관계를 메모리 자료구조로 재현한 단위 테스트에서 `COMPLETE_AT_CHECKPOINT`를 확인했습니다. 5개 객체와 3개 고유 AuditRecord를 방문하고 3번 복호화했으며 전체 로컬 계산은 0.787 ms였습니다.

그다음 같은 graph를 실제 Anvil Contract에 기록하고 다음 순서로 RPC end-to-end를 실행했습니다.

1. Snapshot 블록 7의 block number와 block hash를 고정했습니다.
2. 모든 블록을 복사하지 않고 `producerOf`, AuditRecord와 `noteSpentIn`을 Snapshot block에서 필요한 경로만 조회했습니다.
3. Master Key로 세 AuditRecord를 복호화하고 C·D·E를 frontier로 확정했습니다.
4. 실제 `setStatus` transaction 3건을 제출했습니다. 각 transaction은 49,074 gas와 100-byte calldata를 사용했습니다.
5. 블록 10을 결과 checkpoint로 고정하고 세 대상이 모두 미소비·Frozen인지 다시 조회했습니다.

RPC 결과도 `COMPLETE_AT_CHECKPOINT`였고 전체 호출 시간은 61.677 ms였습니다. Trace가 실패하면 Freeze transaction이 0개인 조건과 Snapshot 이후 대상이 먼저 소비되면 `INCOMPLETE`가 되는 조건도 별도 테스트했습니다.

## Anvil에서는 무엇을 측정했나요?

| 항목 | Gas |
|---|---:|
| ZkDPPV2Ledger 배포 | 6,309,250 |
| Entry verifier 배포 | 1,601,640 |
| Entry verifier 단독 검증 | 372,058 |
| EntryIssuer 등록 | 45,612 |
| mock verifier 기반 Entry·AuditRecord 저장 | 1,214,511 |
| 실제 Entry proof 검증·AuditRecord 저장 | 1,561,818 |
| Note Freeze | 49,074 |

Ledger runtime bytecode는 20,585 bytes로 EIP-170의 24,576-byte 제한 이내였습니다. Docker daemon이 처음에는 실행 중이지 않아 첫 시도가 실패했고, Docker Desktop을 시작한 뒤 두 번째 시도에서 측정했습니다.

모든 Event의 Anvil gas를 측정하지는 않았습니다. 실행하지 않은 Event 비용을 위 값으로 추정하거나 보간하지 않았습니다.

## 어떤 실패를 확인했나요?

- 한 share 또는 중복 위원 ID로 Master Key 복구 실패
- share와 public share가 맞지 않을 때 복구 실패
- 잘못된 위치 mask·감사 암호문 거부
- Unit이 다른 외부 DPP Claim 검증 실패
- Transfer에서 `block.number == D`일 때 실패
- Recall에서 `block.number == D`는 성공하고 이후에는 실패하도록 Contract 조건 고정
- 이미 소비한 Note의 Exit·Issue 재시도 실패
- 잘못된 proof 뒤 AuditRecord ID와 commitment 불변
- Trace 실패 뒤 Freeze 호출 0개

## 현재 한계는 무엇인가요?

- M1 share는 실제 DKG 결과가 아니라 외부 DKG output을 흉내 낸 고정 fixture입니다.
- Master Key 삭제는 best-effort이며 증명 가능한 삭제가 아닙니다.
- AuditAndFreeze는 Anvil Snapshot RPC까지 연결했습니다. 다만 post-snapshot에서 새 자손이 생기면 M1은 자동으로 따라가지 않고 `INCOMPLETE`로 끝냅니다.
- RPC 대표 graph의 Event transaction은 graph·storage·감사 연결을 분리 측정하기 위해 mock verifier를 사용했습니다. 실제 generated proof의 EVM 검증은 별도 Entry 경로에서, 10개 proof 검증은 저장된 PK·VK를 사용한 Go 경로에서 확인했습니다.
- Contract correctness 전체는 mock verifier로, 실제 온체인 proof 경계는 Entry generated verifier로 확인했습니다. 10개 proof 자체는 모두 Go에서 저장된 PK·VK로 검증했습니다.
- v1 회귀용으로 복사한 legacy package에는 기존 $L,n$ API가 남아 있지만, v2 M1의 암호화 함수·gadget·10개 public input에는 사용되지 않습니다.
- SRS는 개발용입니다. Production Ceremony 결과가 아닙니다.

## 다음 단계

- M2: Jubjub 2-of-3 dealerless DKG와 성능 측정
- M3: Audit key registry와 Key Rotation

Raw 결과는 [Circuit](../output/m1-circuit.json), [Master Key](../output/m1-key-recovery.json), [Anvil](../output/m1-anvil.json), [Audit](../output/m1-audit.json), [checksums](../output/m1-generated-checksums.json)에 있습니다.
