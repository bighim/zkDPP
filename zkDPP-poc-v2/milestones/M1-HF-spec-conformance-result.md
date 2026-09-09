# M1-HF 결과 — 실제 proof에서 감사·동결까지 연결했습니다

- 상태: 완료
- 구현 명세: [M1-HF](M1-HF-spec-conformance.md)
- 이전 구현: commit `8560fd3`의 M1
- 측정 환경: Go 1.25.7, GOMAXPROCS=8, Foundry 1.7.1, Solidity 0.8.30, Anvil Prague, chain ID 31337, block gas limit 30M

## 무엇이 달라졌나요?

**Hotfix에서는 10개 Circuit의 실제 proof를 Ledger에 제출하고, 그 transaction에서 만들어진 감사 기록을 RPC로 읽어 추적·동결했습니다.**

- **전체 공급망 20개 Event**가 실제 Poseidon2·PLONK verifier를 통해 성공했습니다.
- Forward Tracing은 Claim 두 개와 Exit 한 개를 정상 종단으로 구분했습니다.
- Claim에서 시작한 Backward Tracing은 원자재 Entry 네 개까지 복원했습니다.
- 별도의 Entry→Split→Split 시나리오에서 C·D·E를 찾아 실제 Freeze transaction 세 건을 제출했습니다.
- Frozen·Revoked가 섞인 경우에도 추가 동결 없이 차단 완료를 확인했습니다.
- Snapshot 이후 먼저 소비된 대상은 `INCOMPLETE`, 잘못된 chain 설정은 동결 0건과 `TRACE_FAILED`로 남았습니다.

**이 결과에는 mock verifier가 사용되지 않았습니다.** 기존 M1의 mock 중심 측정과 구분할 수 있도록 새 Raw·Artifact 경로를 사용했습니다.

## 어떤 관계를 수정했나요?

활성 Hash Domain 11개를 `v2`로 변경했습니다. PolicyRef의 입력은 기존 `eventKind, authorityId, policyId, version` 순서입니다. Owner address·Note·Voucher·nullifier·Claim·암호문·Merkle root를 새 Domain으로 다시 계산했습니다.

Process는 최종 ELIGIBLE 질량이 양수인지 검사합니다. 모든 State가 0인 Process는 거부하고, 일부 input이 zero-State여도 유효한 ELIGIBLE output을 만드는 경우는 허용합니다.

Issue는 소비 Note에 결합된 DocumentHash로 Claim을 만듭니다. ProductName·LotID·Unit을 공개받은 외부 DPP verifier가 문자열에서 DocumentHash와 Claim을 재계산하며, 실제 RPC로 Claim 등록·producer·Policy·Issue 원본을 확인합니다.

Master Key는 기존에 합의한 한 단계 구조입니다. Committee share 두 개를 결합하고 public key와 일치하는지 확인합니다. 실제 DKG와 Key Rotation은 후속 단계입니다.

## 실제로 어떤 공급망을 실행했나요?

1. Aluminum A·B, Cathode, Anode를 Entry했습니다.
2. Aluminum A를 Transfer한 뒤 Recall하고, 다시 Transfer하여 Factory가 Proceed했습니다.
3. 나머지 원자재도 Factory가 전달받았습니다.
4. 같은 DocumentHash를 가진 Aluminum A·B를 Merge했습니다.
5. 세 Note를 Process하여 ELIGIBLE `(270,30,261)`과 WASTE `(30,0,0)`을 만들었습니다.
6. ELIGIBLE Note를 Split한 뒤 각각 Issue했습니다. WASTE는 Exit했습니다.

첫 Transfer의 운송 탄소 1 kgCO2e가 Recall 이후에도 유지되어 Process output에 반영됩니다. 20개 Event가 생성한 Tree root·leaf count·AuditRecord·producer·spent mapping을 일반 계산과 대조했습니다.

## Circuit·proof 비용은 어떻게 나왔나요?

| Event | 공개 입력 | Constraints | Prove | Verify |
|---|---:|---:|---:|---:|
| Entry | 4 | 18,117 | 458.36 ms | 2.70 ms |
| Transfer | 10 | 49,627 | 873.85 ms | 2.63 ms |
| Proceed | 7 | 38,563 | 908.28 ms | 2.78 ms |
| Recall | 8 | 35,987 | 852.57 ms | 2.59 ms |
| Merge | 9 | 55,749 | 883.41 ms | 2.65 ms |
| Split | 9 | 50,455 | 881.06 ms | 2.65 ms |
| Process | 15 | 90,569 | 1,682.50 ms | 2.67 ms |
| Exit | 5 | 30,043 | 492.17 ms | 2.69 ms |
| Issue Standard | 7 | 35,957 | 841.65 ms | 2.60 ms |
| Issue Strict | 7 | 35,957 | 836.25 ms | 2.73 ms |

각 relation의 첫 실제 Event를 대표값으로 사용했습니다. Setup은 하나의 131,075-point development universal SRS를 사용하며 최대 domain은 $2^{17}$입니다. Process의 양수 제약 추가로 기존 M1보다 constraint 한 개가 증가했습니다.

최종 proof는 전체 공급망 20건, 감사 graph 3건, 추가 correctness용 3건으로 총 26건입니다. gnark 저장 형식의 proof는 664 bytes입니다. Solidity용 proof와 전체 transaction calldata 크기는 다른 값이며 Raw에 별도로 나타납니다.

근거: [Circuit Raw](../output/m1-hotfix-circuit.json).

## 실제 온체인 비용은 어떻게 나왔나요?

아래 값은 전체 lifecycle 안의 해당 Event 영수증에서 읽은 값입니다. Tree 위치와 선행 상태가 달라지면 같은 Event의 gas도 달라집니다.

| Event | 전체 gas | calldata bytes | SSTORE opcode gas 합 |
|---|---:|---:|---:|
| 첫 Entry | 2,258,762 | 1,412 | 970,100 |
| 첫 Transfer | 3,594,530 | 1,604 | 1,402,600 |
| Recall | 1,785,185 | 1,540 | 487,700 |
| 첫 Proceed | 1,782,008 | 1,508 | 487,700 |
| Merge | 1,849,988 | 1,572 | 546,900 |
| Process | 2,930,363 | 1,764 | 829,600 |
| 제품 Split | 2,722,091 | 1,572 | 659,500 |
| Issue Standard | 678,701 | 1,508 | 255,600 |
| Issue Strict | 678,689 | 1,508 | 255,600 |
| WASTE Exit | 534,300 | 1,444 | 131,500 |

Ledger 배포는 7,718,222 gas, runtime은 20,585 bytes로 EIP-170 한도 이내입니다. 전체 gas에는 proof 검증, calldata, Tree와 감사 기록·mapping 변경이 포함됩니다. SSTORE 열은 trace의 해당 opcode 비용 합이며 gas 차이를 순수 저장비용으로 간주한 값이 아닙니다.

원자재 lifecycle과 감사 graph는 각각 독립 Ledger에 실행합니다. 각 배포의 verifier 10개·Hasher·Ledger와 권한·Policy 등록 transaction도 Raw에 보존했습니다. transaction 시간 열은 영수증과 opcode trace를 수집하는 진단 실행 구간을 포함합니다.

근거: [Anvil Raw](../output/m1-hotfix-anvil.json).

## 감사·동결에서는 무엇을 확인했나요?

RPC는 기대 chain ID·Ledger 주소·runtime code hash를 확인합니다. 상태 조회에는 block hash와 `requireCanonical=true`를 전달합니다. AuditRecord마다 transaction·성공 receipt·Event 로그·공개 입력·storage 내용을 대조합니다.

| 실행 | 결과 | RPC 호출 | 시간 |
|---|---|---:|---:|
| Aluminum A에서 Forward | Claim 2개·Exit 1개·미소비 zero Change Note 2개 | 184 | 4,731.63 ms |
| Claim에서 Backward | 원자재 Entry 4개 복원 | 148 | 4,116.41 ms |
| Entry→Split→Split Forward | C·D·E frontier | 58 | 1,837.61 ms |
| C·D·E AuditAndFreeze | 세 대상 모두 차단 | 115 | 2,357.23 ms |

C·D·E의 실제 Freeze는 각각 49,434 gas, 100-byte calldata를 사용했습니다. 공통 checkpoint 블록 67에서 모든 대상의 미소비·Frozen 상태와 receipt의 canonical 포함을 다시 확인했습니다.

같은 target 세 개 중 하나를 Revoked로 바꾼 뒤 다시 감사했을 때는 새 Freeze transaction 없이 `COMPLETE_AT_CHECKPOINT`였습니다. Snapshot 뒤 C를 먼저 Exit한 경우에는 C를 결과에서 제거하지 않고 `INCOMPLETE`로 기록했습니다. 새 자손을 자동 추적하지 않았습니다.

대상별 결과에는 초기 관찰 블록·spent·Status, Freeze 시도·확인 여부, transaction hash·receipt block·gas, 불명확 상태·오류, 최종 checkpoint 관찰이 남습니다. 일부 transaction의 결과를 알 수 없으면 최종 Status가 Frozen이어도 전체 완료로 표시하지 않습니다.

근거: [Audit Raw](../output/m1-hotfix-audit.json).

## Master Key 복구 비용은 어떻게 나왔나요?

2-of-3의 세 조합은 모두 public key와 일치하는 Master Key를 복구했습니다. 복구 시간은 각각 0.098·0.095·0.094 ms입니다.

| 기록 수 | 기록당 Field | 복호화·원문 대조 | 할당 메모리 |
|---:|---:|---:|---:|
| 1 | 5 | 0.196 ms | 2,720 B |
| 10 | 5 | 1.806 ms | 27,120 B |
| 100 | 5 | 19.857 ms | 271,200 B |
| 1,000 | 5 | 170.144 ms | 2,712,000 B |

생성·파일 로딩은 이 복호화 측정 구간에서 제외합니다. 다섯 Field 전체를 원문과 대조했습니다. 메모리 값은 누적 allocation이며 peak RSS가 아닙니다.

v1 M9의 기록별 partial decryption은 감사에서 위원 응답 수가 증가하는 방식입니다. 보존된 v1 M9 첫 실행의 Forward에는 응답 16개, Backward에는 28개가 기록돼 있습니다. Hotfix는 두 share로 key를 복구한 뒤 기록별 위원 호출이 없습니다. 두 실행은 lifecycle과 RPC 검증 범위가 다르므로 시간 비율을 성능 향상률로 제시하지 않습니다.

근거: [Key Raw](../output/m1-hotfix-key-recovery.json), [보존된 v1 M9 Audit Raw](../../zkDPP-poc-v1/output/m9-audit.json).

## 실패와 재실행은 어떻게 남겼나요?

- Setup 1회, evaluate 2회, Anvil gas 2회, Audit 4회, Key 측정 1회입니다. 최종 파일은 수정 후 실행의 실제 값을 사용합니다.
- 초기 감사 실행 뒤 대상별 관찰 정보·정확한 RPC 수·key release 순서 검사를 보완했습니다.
- 두 독립 시나리오가 난수 카운터를 재사용하는 문제를 확인하여 범위를 분리했습니다. proof를 다시 생성하고 Anvil·Audit을 다시 실행했습니다. 이전 Hotfix 파일은 `attempts`에 보존했습니다.
- 개발 중 로컬 RPC 테스트 서버가 sandbox의 포트 제한으로 실패하여 허용된 실행 환경에서 다시 검증했습니다.
- Foundry 교차 verifier 테스트는 초기에는 revert를 기대했지만 generated verifier가 false를 반환하므로 그 동작에 맞게 수정했습니다.
- 이런 재시도를 공식 실행 한 번으로 축약하지 않습니다. CLI 실행 시작·종료·성공·오류는 각 attempt JSON에 남습니다.

## 어떤 검사까지 통과해야 최종 결과인가요?

Go 전체 suite를 cache 없이 실행해 통과했습니다. terminal·Revoked·post-snapshot 소비·receipt 오류·잘못된 chain/code hash·Process 양수 조건·암호화 및 release context를 확인했습니다. Foundry 7/7은 모든 실제 Event와 잘못된 proof·Policy·Scope·Voucher deadline·Status·원자성을 확인했습니다.

`finalize-m1-hotfix`가 결과와 Artifact를 대조한 뒤 checksum을 고정합니다. `check-m1-hotfix`는 저장된 checksum과 현재 파일을 비교하고, 메모리에서 compile한 Circuit의 CCS도 저장 Artifact와 대조합니다. manifest·Raw·proof를 다시 쓰지 않습니다. 임시 사본에서 verifier·공개 입력·Raw를 바꾼 검증은 실패하며, 검사 자체가 파일을 고치지 않는 것도 테스트합니다.

기존 M1·v1 결과와 Artifact, Conversation History의 보호 대상 465개는 별도의 baseline으로 대조합니다. Hotfix source는 기존 M1 source commit 위에 적용되며 과거 결과는 덮어쓰지 않습니다.

## 실행 방법과 경계

처음 재현할 때는 Setup → evaluate → Go·Foundry 테스트 → key·Anvil·Audit 측정 → finalize → check 순서입니다. 명령은 [Hotfix 실행 README](../internal/hotfix/README.md)와 [구현 명세](M1-HF-spec-conformance.md)에 있습니다. 각 측정 파일이 이미 있으면 해당 명령은 교체를 거부합니다.

키·난수·owner는 공개된 로컬 테스트 입력입니다. 실제 DKG·운영 키 배포·안전한 메모리 삭제·Key Rotation·자동 post-snapshot 자손 차단은 이번 검증 대상이 아닙니다. Anvil 블록 확인을 Production finality로 일반화하지 않습니다.

최종 파일 목록과 무결성 근거: [Hotfix checksums](../output/m1-hotfix-generated-checksums.json).
