# M8 — Exit한 DPP에 Claim을 실제로 붙였나요?

- 상태: 완료
- 구현 기준: [M8 명세](M8-exit-dpp-issue.md)
- 결정 배경: [M8 Background](M8-exit-dpp-issue-background.md)
- Raw: [Circuit](../output/m8-circuit.json), [gas](../output/m8-anvil-gas.json), [Claim 감사](../output/m8-audit.json), [checksum·실행 이력](../output/m8-generated-checksums.json)
- 공식 실행 횟수: Setup·evaluate·gas는 각 1회, audit는 원본 Event 대조 보강 전후 2회입니다.

## 0. 30초 안에 무엇을 기억하면 되나요?

**M7의 terminal Exit를 확장해 공개 DPP commitment를 만들고, 같은 DPP에 제품 정보와 State를 공개하지 않는 Policy Claim 두 개를 붙였습니다.**

| 질문 | M8 결과 |
|---|---|
| 이전 상태 | M7 Exit는 Note를 소비하고 provenance 경로를 종료했지만 DPP나 Claim을 만들지 않았습니다. |
| 이번 목표 | private Note를 finalized DPP로 확정하고, 그 State가 Sustainability Policy를 만족한다는 사실을 반복해서 보여주는 것입니다. |
| 실제 구현 | DPP commitment, M8 Exit, Standard·Strict Issue Circuit, ZkDPPClaimLedger, Claim 상태와 backward 감사입니다. |
| 이제 가능한 것 | 하나의 DPP에 서로 다른 Policy Claim을 붙이고 제3자가 등록 여부·상태를 조회할 수 있습니다. |
| 검증 | 전체 Go suite 통과, Foundry 36개 test 통과, 보호 파일 307개의 checksum이 유지됐습니다. |
| 대표 성능 | Exit 36,930 constraints·618,936 gas, Issue 12,490 constraints·459,977 gas, 최종 Claim 감사 약 0.839초입니다. |
| 미포함 | DPP 소유권 이전, component DPP, Re-entry, 중간 Claim, Claim Tree와 규제 DPP 전체입니다. |
| 다음 단계 | M9에서 최종 Circuit·Contract를 universal SRS와 통합 평가 기준으로 정리합니다. |

전체 전이는 다음입니다.

$$
\mathrm{private\ Note}
\xrightarrow{\mathrm{Exit}}
\mathrm{Finalized\ DPP}(dppCommitment)
$$

$$
\mathrm{Finalized\ DPP}
\xrightarrow{\mathrm{IssuePolicy}}
\mathrm{같은\ DPP}+\mathrm{Claim}
$$

Issue는 DPP를 소비하거나 State를 바꾸지 않습니다. 같은 DPP·같은 PolicyRef Claim은 한 번만 만들 수 있고, 다른 version의 Claim은 별도로 등록할 수 있습니다.

## 1. M7에서 무엇이 달라졌나요?

| M7 | M8 실제 구현 |
|---|---|
| Exit가 Note를 소비하고 output 없이 끝납니다. | Exit가 Note와 같은 제품·State를 가진 DPP commitment를 생성합니다. |
| producerOf는 Note·Voucher를 찾습니다. | DPP object type 3과 Exit 생성 기록을 추가했습니다. |
| Process Policy만 실제로 사용했습니다. | 같은 Authority Registry로 Issue Policy V1·V2를 등록했습니다. |
| 모든 공급망 AuditRecord에는 암호문이 있었습니다. | 공개 DPP·Policy만 다루는 Issue는 암호문 없는 Record를 사용합니다. |
| nf·rvnf 상태를 집행했습니다. | DPP·Policy 조합별 Claim 상태를 추가했습니다. |

기존 ZkDPPAuditLedger와 M7 Circuit·Artifact·Raw 결과는 수정하지 않았습니다. M8은 새 ZkDPPClaimLedger를 배포하는 fresh POC입니다.

## 2. 무엇을 구현했나요?

### DPP commitment

DPP private data는 DocumentHash, AssetRole, $q_{\mathrm{mass}}$, $a_{\mathrm{rec}}$, $e$, dppOpening입니다.

$$
dppCommitment=
H(DPPTag,DocumentHash,AssetRole,q_{\mathrm{mass}},a_{\mathrm{rec}},e,dppOpening)
$$

owner address·Note opening·nf는 포함하지 않습니다. dppOpening은 hiding 값이며 현재 소유자를 증명하는 credential이 아닙니다.

### M8 Exit

M8 Exit는 Note membership·owner·nf를 확인한 뒤 Note와 DPP private data의 DocumentHash·Role·State가 같은지 검증했습니다. 실제 부모 cm 하나도 기존 감사 공개키로 암호화했습니다.

| 공개값 | 비공개값 |
|---|---|
| noteRoot, nf, dppCommitment, $R_{1X}$, $R_{1Y}$, encryptedParentCM | Note·cm·owner secret·path, DPP private data, 암호화 난수 |

Exit AuditRecord는 DPP outputRef와 암호화된 부모 cm을 저장합니다. DPP는 소비되지 않으므로 Tree·nullifier·encryptedOutputNfs를 만들지 않았습니다.

### Issue Claim

하나의 Issue Circuit 구현을 다음 두 compile-time Policy 설정으로 Setup했습니다.

| Policy | 최소 재활용률 | 최대 탄소집약도 |
|---|---:|---:|
| Standard V1 | 10% | 1.00 kgCO2e/kg |
| Strict V2 | 11% | 0.97 kgCO2e/kg |

Public input은 issuePolicyRef와 dppCommitment 두 개입니다. DocumentHash·Role·State·dppOpening은 private witness입니다.

M5 Process output $(270,30,260)$은 재활용률 약 11.11%, 탄소집약도 약 0.963이므로 두 Policy를 모두 통과했습니다.

Issue에는 PolicyGrant·policyScopeRef가 없습니다. proof 제출 EVM account도 DPP owner로 해석하지 않습니다.

## 3. Contract에는 무엇이 저장됐나요?

| 상태 | 답하는 질문 |
|---|---|
| producerOf[DPP][dppCommitment] | 이 DPP를 어느 Exit 기록이 만들었나요? |
| claimRecordOf[dppCommitment][issuePolicyRef] | 이 Claim을 어느 Issue 기록이 만들었나요? |
| claimStatus[dppCommitment][issuePolicyRef] | Claim이 Active·Frozen·Revoked 중 무엇인가요? |

Claim 식별값은 dppCommitment와 issuePolicyRef의 조합입니다. 별도 Claim Hash·nonce·Tree를 만들지 않았습니다.

Issue AuditRecord는 eventKind=ISSUE와 policyRef만 저장합니다. outputRefs·공개점·encryptedParents·encryptedOutputNfs는 비어 있습니다. 공개 dppCommitment는 ClaimIssued Event와 transaction calldata에서 읽으므로 Record에 다시 저장하지 않았습니다.

Claim 상태 전이는 다음만 허용했습니다.

```text
Active → Frozen
Frozen → Active
Frozen → Revoked
```

한 Policy version의 상태 변경은 같은 DPP의 다른 Claim에 영향을 주지 않았습니다. verifyClaim은 registered를 별도로 반환하여 미등록 Claim의 기본 status 0을 Active Claim으로 오인하지 않게 했습니다.

## 4. 대표 실행에서는 무슨 일이 있었나요?

1. M7의 Entry 3건과 감사 Process proof를 재사용했습니다.
2. Process가 ELIGIBLE Note와 WASTE Note를 만들었습니다.
3. ELIGIBLE Note를 M8 Exit하여 dppCommitment를 등록했습니다.
4. Standard Claim을 Issue했습니다.
5. 같은 Standard Claim의 두 번째 Issue는 실패했습니다.
6. Strict Claim을 별도로 Issue했습니다.
7. Standard Claim은 Freeze 후 Active로 되돌렸습니다.
8. Strict Claim은 Freeze 후 Revoke했습니다.
9. verifyClaim이 Standard=Active, Strict=Revoked를 반환했습니다.

최종 상태는 Note leaf 5개, AuditRecord 7개, DPP 하나와 Claim 두 개입니다. WASTE도 Exit proof가 성공했지만 Standard·Strict Issue proof는 만들 수 없음을 Circuit test로 확인했습니다.

## 5. Claim에서 원자재까지 추적했나요?

**Standard Claim에서 시작해 Issue→Exit→Process→Entry 3건으로 이동했습니다.**

1. claimRecordOf로 Issue 기록을 찾았습니다.
2. Issue의 Event·transaction·빈 암호문 Record를 대조했습니다.
3. producerOf[DPP]로 Exit 기록을 찾았습니다.
4. 위원 두 명이 Exit의 encryptedParentCM을 복호화했습니다.
5. 복원한 cm에서 기존 M7 backward tracer를 실행했습니다.
6. Process 부모 Note 3개와 각각의 Entry를 찾았습니다.

| 항목 | 실제 값 |
|---|---:|
| 전체 시간 | 838.693 ms |
| 고유 기록 | 6개 |
| 복호화 | 2회 |
| 위원 응답 | 4개 |
| RPC 요청 | 58회 |
| Exit 이후 기존 upstream tracing | 817.550 ms |

Issue 기록은 암호문이 없으므로 복호화하지 않았습니다. 복호화 2회는 Exit 1회와 Process 1회입니다. 세 Entry 기록은 부모가 없어 추가 복호화가 필요하지 않았습니다.

이 값은 로컬 RPC에서 작은 그래프를 한 번 실행한 결과입니다. 대규모 DPP 감사 성능으로 일반화하지 않습니다.

## 6. Correctness는 무엇을 확인했나요?

- DPP 일반 계산과 Exit·Issue Circuit commitment가 일치했습니다.
- Exit 공개 입력 6개와 Issue 공개 입력 2개의 순서가 고정됐습니다.
- ELIGIBLE·WASTE Exit가 성공했습니다.
- Note와 DPP의 DocumentHash·Role·State 불일치, 잘못된 opening·commitment·암호문이 실패했습니다.
- Standard·Strict의 정확한 경계는 성공하고 재활용률 부족·탄소 초과는 실패했습니다.
- uint64 최대값과 $10^9$의 94-bit 곱셈 범위를 확인했습니다.
- WASTE·질량 0 DPP의 Issue가 실패했습니다.
- 미등록 DPP·중복 Claim·disabled Policy·잘못된 권한·금지 상태 전이를 거부했습니다.
- 실패한 Exit·Issue 뒤에 nf·producerOf·Claim·AuditRecord가 일부만 기록되지 않았습니다.
- 전체 Go suite가 통과했습니다.
- Foundry는 기존 suite와 M8 3개를 합쳐 36개 test가 모두 통과했습니다.
- ZkDPPClaimLedger runtime은 18,360 B로 EIP-170 한도 24,576 B 이하입니다.

## 7. Circuit 비용은 얼마인가요?

| Circuit | Constraints | 공개 입력 | Prove ms | Native Verify ms | Solidity proof |
|---|---:|---:|---:|---:|---:|
| M8 Exit | 36,930 | 6 | 866.843 | 2.704 | 1,056 B |
| Standard V1 Issue | 12,490 | 2 | 265.055 | 2.688 | 1,056 B |
| Strict V2 Issue | 12,490 | 2 | 263.045 | 2.690 | 1,056 B |

모든 binary proof는 664 B입니다. M8 Exit의 일반 암호화 계산은 0.127 ms였습니다.

Standard·Strict serialized VK는 각각 49,144 B이고 optimized on-chain VK 표현은 각각 1,216 B입니다. 실제 공식 경로는 Policy별 constant verifier이며 PolicyRecord의 vkHash는 이 canonical optimized VK encoding의 SHA-256입니다.

Strict Setup의 SRS 시간이 0 ms로 기록된 것은 같은 크기의 Standard SRS가 실행 중 memory cache에 있었기 때문입니다. final universal SRS를 사용했다는 의미가 아닙니다.

## 8. 온체인 비용은 얼마인가요?

| 실행 | Receipt gas | SSTORE | calldata |
|---|---:|---:|---:|
| ZkDPPClaimLedger 배포 | 7,080,526 | 70 | 19,885 B |
| M8 Exit | 618,936 | 11 | 1,476 B |
| Standard Issue | 459,977 | 4 | 1,188 B |
| 중복 Standard Issue 거부 | 66,570 | 0 | 1,188 B |
| Strict Issue | 459,977 | 4 | 1,188 B |
| Claim Active→Frozen | 50,142 | 1 | 100 B |
| Claim Frozen→Active | 28,285 | 1 | 100 B |
| Claim Frozen→Revoked | 33,123 | 1 | 100 B |
| verifyClaim transaction | 27,393 | 0 | 68 B |

verifyClaim의 최종 eth_call latency는 0.577 ms였습니다. eth_call은 실제 gas를 지불하지 않으므로 transaction gas와 구분합니다.

SSTORE 수는 전체 transaction trace의 opcode 횟수입니다. Exit 11회나 Issue 4회를 순수 암호문 저장 비용으로 해석하지 않습니다. verifier 한 개의 배포 gas는 약 1.60M이었고 전체 deployment·Policy 등록 내역은 gas Raw에 있습니다.

## 9. 계획과 실제 구현이 달랐나요?

| 항목 | 계획 | 실제 | 영향 |
|---|---|---|---|
| Issue Circuit | Standard·Strict 별도 Artifact | 하나의 구현을 두 Policy constant로 Setup | 관계와 VK 분리는 계획과 동일합니다. |
| M7 감사 코드 | 확장 또는 adapter | 기존 M7 tracer를 수정하지 않고 M8 Claim adapter 추가 | M7 결과를 보존하면서 DPP→Note 연결만 추가했습니다. |
| Exit 기록 | DPP output·암호화된 부모 | 별도 Exit 저장 경로로 구현 | DPP에는 future nf가 없다는 명세를 그대로 반영했습니다. |
| Protocol 의미 | 명세 기준 | 변경 없음 | 없음 |

개발 중 첫 Solidity compile은 Claim mapping 선언 누락으로 실패했고 수정했습니다. 첫 targeted Foundry 실행은 중복 DPP보다 spent nf가 먼저 검사되는 실제 순서와 test 예상이 달라 1개가 실패했습니다. 다른 미소비 Note로 중복 DPP를 검사하도록 test를 고친 뒤 targeted suite가 통과했습니다. 최종 전체 Foundry suite는 한 번 실행해 36개 모두 통과했습니다.

첫 audit 측정 뒤 ClaimIssued·DPPFinalized Event 로그도 storage·transaction과 직접 대조하도록 감사 adapter를 보강했습니다. 첫 Raw는 `artifacts/development/m8/runs/audit-01-before-event-log-check.json`에 보존하고 audit를 한 번 다시 실행했습니다. 본문 수치는 두 번째 최종 결과입니다.

전체 Go suite도 감사 adapter 보강 전후 각각 한 번 실행해 총 2회 통과했습니다. Setup·evaluate·gas는 다시 실행하지 않았습니다.

## 10. Artifact와 checksum은 어디에 있나요?

개발 Artifact는 다음 세 폴더에 있습니다.

```text
artifacts/development/m8/exit-dpp
artifacts/development/m8/issue-standard-v1
artifacts/development/m8/issue-strict-v2
```

각 폴더에는 CCS·canonical/Lagrange SRS·PK·VK·manifest가 있습니다. Issue 폴더에는 optimized-vk.json도 있습니다. 기존 M7 fixed proof는 복사하지 않고 checksum으로 참조했습니다.

`m8-generated-checksums.json`은 M8 생성물 63개와 보호 파일 307개를 대조합니다. 보호 대상은 모두 unchanged입니다. 이 Artifact는 개발·correctness 확인용이며 final Policy key가 아닙니다.

## 11. 어떻게 다시 실행하나요?

새 결과를 생성할 별도 실험 사본에서 다음 순서로 실행합니다.

```text
make setup-m8
make evaluate-m8
GOFLAGS=-count=1 GOMAXPROCS=8 make test-go
make test-contract-m8
make benchmark-m8-gas
make benchmark-m8-audit
make check-m8
```

`make benchmark-m8`은 gas와 audit를 함께 실행하는 aggregate입니다. 개별 명령과 중복 실행하지 않습니다. 공식 Raw가 이미 존재하면 덮어쓰기를 거부합니다.

환경은 Go 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1, macOS arm64, GOMAXPROCS=8입니다. EVM은 Foundry·Anvil 1.7.1, Solidity 0.8.30, Prague, chain ID 31337, block gas limit 30M입니다.

## 12. 무엇은 아직 보장하지 않나요?

- dppOpening을 안다고 현재 DPP owner임을 증명하지 않습니다.
- 과거 Owner가 private 원문 복사본을 보관하지 못하게 강제하지 않습니다.
- DPP ownership transfer·component DPP·Re-entry를 구현하지 않았습니다.
- 공급망 중간 Claim·Claim Tree·Claim nullifier·Claim 소비를 만들지 않았습니다.
- 제품 원문·QR·NFC·규제 DPP 전체 데이터 모델을 검증하지 않습니다.
- DKG·위원회 key rotation·악의적 위원 응답 증명을 구현하지 않았습니다.
- 개발용 SRS를 final universal SRS로 해석하지 않습니다.

M8의 결과는 **Exit된 private Sustainability State와 공개 DPP commitment를 연결하고, 같은 DPP에 여러 Policy Claim을 독립적으로 붙여 조회·상태 관리·upstream 감사를 할 수 있음**을 보여줍니다.
