# M1 Master Key Audit 일괄 구현 계획

## 1. 목표와 진행 방식

[M1 구현 명세](M1-master-key-audit.md)를 기준으로 일곱 단계로 구현합니다. 단계별 사용자 승인을 요청하지 않고 내부 Gate를 통과한 뒤 계속 진행하며, 전체 구현·검증·측정·Result가 끝난 뒤 한 번에 보고합니다.

```text
M1.0 v1 M9 source baseline과 v2 module 준비
M1.1 v2 객체·회계·식별자 Core
M1.2 Master Key 복구·새 Audit 암호화 Core
M1.3 Event Circuit·fixture·PLONK Artifact
M1.4 ZkDPPV2Ledger·Foundry
M1.5 Master Key Forward Trace·AuditAndFreeze
M1.6 Anvil 측정·최종 Gate·Result
```

기존 `zkDPP-poc-v1`, Conversation History와 v1 Raw·Artifact는 수정·재측정하지 않습니다. 자동 커밋하지 않습니다.

## 2. M1.0 — v1 baseline과 프로젝트 준비

- v1 기준 commit `9c5d335`의 tracked source 중 구현에 필요한 항목만 v2로 복사합니다.
- 복사 대상은 Go module·Makefile·Docker 설정·testdata·`cmd`·`features`·`internal`·Foundry 설정·Contract source·test입니다.
- v1 milestones·docs·output·Artifact·cache·generated verifier·proof fixture는 복사하지 않습니다.
- Go module과 모든 내부 import를 `github.com/bighim/zkDPP/zkDPP-poc-v2`로 변경합니다.
- v2 `.gitignore`에는 cache·Artifact·generated verifier·fixture·secret share를 포함합니다.
- v1 tracked source·Raw·Conversation History checksum을 `artifacts/development/m1/baseline.json`에 기록합니다.
- v1 M9 코드를 v2 baseline으로 가져온 직후 compile 가능한지 확인한 뒤 Protocol 변경을 시작합니다.

내부 Gate:

- v1 파일은 수정되지 않습니다.
- v2 source가 v1 module을 import하지 않습니다.
- ignored SRS·PK·VK·proof·private share가 Git 후보에 나타나지 않습니다.
- M1 변경 전 baseline checksum이 고정됩니다.

## 3. M1.1 — v2 객체·회계·식별자 Core

- `DocumentInfo`에 Unit을 추가하고 NFC UTF-8·uint32 big-endian length-prefix encoding을 `ProductName→LotID→Unit` 순서로 구현합니다.
- Note nullifier를 $H_{nf}(cm,sk_{owner})$ 순서로 바꿉니다.
- Voucher resolution secret $s_{res}=H_{res}(opening)$과 $rvnf=H_{rvnf}(rv,s_{res})$를 구현합니다.
- Note·Voucher commitment, Owner·Policy·Scope·Audit Domain과 PolicyRef 식은 v1을 유지합니다.
- Claim Core에 $h=H_{issue}(DocumentHash,issuePolicyRef,claimNonce)$, ClaimRef와 DPPClaim을 추가합니다.
- 공통 allocation에 private $\Delta e_{transport}$를 Voucher에만 더하는 checked uint64 관계를 추가합니다.
- Merge·Split은 ELIGIBLE·같은 DocumentHash를 강제하고 WASTE의 비-Exit 소비를 거부합니다.
- deadline helper를 절대 uint64 block number $D$로 바꾸고 Epoch·timestamp helper를 v2 경로에서 제거합니다.

내부 Gate:

- Unit만 바꾸면 DocumentHash가 달라집니다.
- 일반 계산의 $nf,s_{res},rvnf,h$가 고정 vector와 같습니다.
- 운송 탄소 0·양수·overflow와 부분·전량 Transfer가 명세 식과 같습니다.
- WASTE는 Exit만 허용되고 graph·Status 자료형에서는 유지됩니다.
- v1 Domain을 `:v2`로 바꾼 상수가 없습니다.

## 4. M1.2 — Master Key 복구와 Audit 암호화 Core

- `ExternalKeyPackage`, 위원별 private share와 public share 자료형을 구현합니다.
- 테스트 전용 1차 Shamir polynomial로 외부 DKG output fixture를 생성하되 Master Key와 polynomial을 영속 저장하지 않습니다.
- 서로 다른 두 share의 profile·session·member ID·scalar·public share를 확인하고 Lagrange interpolation으로 $SK_A$를 복구합니다.
- 복구 뒤 (SK_AG=PK_A)를 반드시 확인합니다.
- 기존 Jubjub Point·canonical encoding·subgroup 검사와 Poseidon2를 재사용합니다.
- Record마다 새 $r_i,R_i,Z_i,K_i$, 위치마다 $k_{i,j}$를 만드는 Native Encrypt·Decrypt를 구현합니다.
- Circuit gadget에서 $L,n$을 제거하고 고정 $PK_A$ 상수 아래의 동일한 관계를 검증합니다.
- 1-Field와 5-Field 독립 암호화 Circuit으로 Core를 먼저 검증합니다.
- private share 파일은 0600으로 만들고 secret·Master Key·평문을 로그와 Raw JSON에 출력하지 않습니다.

내부 Gate:

- 위원 조합 `{1,2}`, `{1,3}`, `{2,3}`이 같은 public key에 대응하는 key를 복구합니다.
- 한 share·중복 ID·다른 session·변조 scalar·public share를 거부합니다.
- 같은 Record의 위치별 mask와 서로 다른 Record의 $K_i$가 다릅니다.
- Field 0·(p-1)·wraparound 이후 원문을 정확히 복원합니다.
- $L,n$이 Core·Circuit·witness·public input에 없습니다.

## 5. M1.3 — Event Circuit·fixture·Artifact

- Entry·Transfer·Proceed·Recall·Merge·Split·Process·Exit·Issue Standard·Issue Strict의 최종 Circuit 10개를 구성합니다.
- $PK_A$는 모든 Circuit definition의 상수이며 package·Circuit manifest checksum을 상호 대조합니다.
- Transfer public input은 $D$ 하나를 사용하고 Recall은 public $D$를 private Voucher와 연결합니다.
- Exit는 5개 public input과 output 없는 AuditRecord shape를 사용합니다.
- Issue 두 Circuit은 7개 public input으로 Note 소비·Policy threshold·Claim $h$·부모 암호화를 한 proof에서 검증합니다.
- 모든 Event의 실제 private parent와 output 미래 소비값에서 평문을 만들고 임의 배열을 신뢰하지 않습니다.
- 전체 lifecycle fixture와 Entry→Split→Split AuditAndFreeze fixture를 별도로 구성합니다.
- 10개 Circuit을 compile한 뒤 최대 domain을 확인하고 하나의 새 development universal SRS에서 CCS·PK·VK를 생성합니다.
- Solidity verifier 10개와 fixed proof fixture를 v2 전용 generated 경로에 생성합니다.

내부 Gate:

- public input 수는 Entry 4, Transfer 10, Proceed 7, Recall 8, Merge 9, Split 9, Process 15, Exit 5, Issue 7·7입니다.
- Native·Circuit의 commitment·nullifier·Claim·암호문이 같습니다.
- 잘못된 Unit·DocumentHash·owner·path·State·운송 탄소·$D$·Claim·암호문은 실패합니다.
- 저장한 CCS·PK·VK를 다시 읽어 proof를 검증합니다.
- v1 Artifact·Raw checksum이 변하지 않습니다.

## 6. M1.4 — ZkDPPV2Ledger와 Foundry

- v1 final Contract를 복사한 새 `ZkDPPV2Ledger`에서만 변경합니다.
- constructor에는 고정 Event verifier 7개, Poseidon2 hasher와 immutable Status Authority를 둡니다.
- Process·Issue verifier는 기존 PolicyRecord의 `verifierRef`로 선택합니다.
- Exit를 `exit(proof,noteRoot,nf,auditCipher)`로 바꾸고 successor 없는 AuditRecord를 기록합니다.
- Issue를 `issue(proof,issuePolicyRef,noteRoot,nf,h,auditCipher)`로 바꾸고 Note 소비·`claimRegistered[h]`·Claim producer·AuditRecord를 원자적으로 기록합니다.
- DPP commitment·DPP ObjectRef·claimRecordOf·claimStatus·setClaimStatus를 v2 Contract에 넣지 않습니다.
- Transfer는 `block.number<D`, Recall은 `block.number<=D`를 확인하며 Proceed는 deadline을 검사하지 않습니다.
- Note·Voucher Status mapping과 개별 `setStatus`를 유지합니다.
- WASTE 비-Exit 거부는 Circuit이 강제하고 Contract는 정확한 verifier를 선택합니다.

Foundry Gate:

- 9개 Event의 positive 흐름과 변경된 AuditRecord shape가 일치합니다.
- 같은 Note 재Issue, Issue 후 Exit, Exit 후 Issue가 중복 소비로 실패합니다.
- WASTE Exit는 성공하고 다른 소비 Event는 실패합니다.
- Transfer (block.number=D) 실패, Recall 같은 블록 성공, (D+1) 실패를 확인합니다.
- wrong Policy·root·Status·proof·output·Claim과 모든 부분 상태 변경을 거부합니다.
- Claim status와 DPP commitment ABI·storage가 없습니다.
- runtime bytecode가 EIP-170 제한 이내입니다.

## 7. M1.5 — DPP 검증·Master Key 감사·AuditAndFreeze

- 외부 DPP의 ProductName·LotID·Unit과 DPPClaim으로 $h$를 재계산하는 `VerifyDPPClaim` client를 구현합니다.
- Claim 등록·producer·Issue AuditRecord·Policy manifest를 하나의 Snapshot에서 확인합니다.
- v1 RPC source의 `(blockNumber,blockHash)` Snapshot·원본 transaction·receipt·code hash 검사를 재사용합니다.
- TraceForward는 Master Key로 Record를 복호화하며 Claim과 Exit를 서로 다른 terminal로 반환합니다.
- AuditAndFreeze는 Trace 성공 뒤 전체 typed frontier를 고정하고 Active target을 개별 Status transaction으로 Freeze합니다.
- target별 already Frozen·Revoked·consumed·query failure·submission failure를 보존합니다.
- 모든 시도 후 하나의 $H_r$에서 전체 target을 다시 조회해 네 outcome을 반환합니다.
- 감사 세션 종료 뒤 Master Key와 전달받은 share copy를 best-effort zeroize합니다.
- post-snapshot consumer fallback은 호출하거나 구현하지 않습니다.

내부 Gate:

- Claim→Issue→upstream Entry backward tracing이 성공합니다.
- Entry A→Split→Split의 C·D·E 전체 frontier가 정확합니다.
- Trace 실패 시 Status transaction이 0개입니다.
- C·D·E Freeze 뒤 같은 $H_r$에서 `COMPLETE_AT_CHECKPOINT`입니다.
- target 하나를 Snapshot 뒤 소비하면 `INCOMPLETE`이며 새 자손을 따라가지 않습니다.
- 일부 실패를 제거하거나 전체 완료로 바꾸지 않습니다.

## 8. M1.6 — 측정·최종 대조·Result

- Go 전체 suite와 Foundry 전체 suite는 코드 안정화 뒤 각각 최종 실행합니다.
- 고정 proof gas와 live audit는 서로 다른 clean Anvil chain에서 실행합니다.
- Anvil은 Compose project `zkdpp-v2-m1`, RPC port 18546, chain ID 31337, Prague, 30M block gas limit을 사용합니다.
- Go proof와 복호화 workload는 GOMAXPROCS=8로 순차 실행합니다.
- Master Key 복구는 세 두-위원 조합을 측정합니다.
- 독립 Record 1·10·100·1,000개의 복호화 시간·할당 메모리·원문 일치를 측정합니다.
- Event별 Circuit·proof·deployment·transaction gas와 AuditAndFreeze RPC·Freeze·최종 확인을 측정합니다.
- v1 M9 Raw는 읽기만 하고 Record별 partial-decryption 방식과 비교합니다.
- 공식 case는 각각 한 번 실행하고 평균으로 주장하지 않습니다.

Raw 결과:

```text
output/m1-circuit.json
output/m1-key-recovery.json
output/m1-anvil.json
output/m1-audit.json
output/m1-generated-checksums.json
```

최종 Gate:

- 일반 계산·Circuit·Contract·복호화 원문·graph·Status 결과가 일치합니다.
- public input manifest·ABI·fixture·verifier 순서가 일치합니다.
- v1 M9·Conversation History 보호 checksum이 유지됩니다.
- secret share·Master Key·SRS·PK·VK·proof가 Git 대상에 없습니다.
- `M1-master-key-audit-result.md`를 Result Template에 맞춰 생성합니다.
- 전체 Gate와 Result 대조 후에만 명세를 `구현 기준 동결`, MILESTONES를 `완료`로 변경합니다.

## 9. 실행 Interface

```text
make setup-m1
make evaluate-m1
make test-go
make test-contract-m1
make benchmark-m1-key-recovery
make benchmark-m1-decrypt
make benchmark-m1-anvil
make benchmark-m1-audit
make benchmark-m1
make check-m1
```

Aggregate benchmark와 개별 benchmark를 중복 실행하지 않습니다. 실패와 재실행은 attempt metadata에 이유·횟수를 기록합니다.

## 10. 확정 가정

- 외부 DKG가 올바른 Jubjub 2-of-3 share를 배포했다고 가정합니다.
- M1 fixture는 DKG correctness의 근거가 아닙니다.
- M1 key는 모든 Event가 끝난 닫힌 시나리오에서만 release합니다.
- Status Authority와 Auditor는 동일하고 올바른 감사·집행·삭제를 신뢰합니다.
- $PK_A$는 Circuit 상수이며 M3 Rotation에서 Circuit·Artifact를 다시 생성합니다.
- post-snapshot partial-decryption fallback은 Background에만 남기고 구현하지 않습니다.
- Production DKG·Rotation·network·key deletion 증명·운영 DPP integration은 범위 밖입니다.
