# zkDPP v2 구현 수정 목록

기준 명세: [zkDPP v2 프로토콜](/Users/jongho/workspace/zkDPP-local/notes/zkdpp-v2-protocol.md)  
기준 구현: commit `9c5d3352d2f60162106a4c7841849bf9b022cbd3`의 보존 M9 source.  
상태: 수정 방향을 정리한 목록. 아래 구현·실행 검증은 아직 수행하지 않았다.

## 1. 유지할 구조와 이번 결정

- 일반 일곱 Event(Entry·Transfer·Proceed·Recall·Merge·Split·Exit)는 Event별
  고정 verifier를 유지한다. 전부 동적 PolicyRef·scope·grant 체계로 옮기지 않는다.
- Process의 등록 정책·scope grant 경로와 Issue의 기존 발급 정책 선택 경로를
  유지한다. Issue에 scope grant를 추가하거나 기존 정책 선택을 제거하지 않는다.
- 이 유지 결정은 **정책 선택과 권한 경로**에 관한 것이다. 아래에 따로 기록한
  Issue lifecycle, 문서 결합이나 nullifier 식의 수정 요구를 취소하지 않는다.
- Committee의 유효한 key-share 전달은 audit 외부 전제다. Auditor의 로컬
  복호화와 키 조각 접근, 원격 partial-response proof 부재를 수정 사유로 삼지 않는다.
- 회계 방향: Entry 초기 탄소는 유효한 명시 값을 허용하고 생략하면 0을 쓴다.
  Transfer는 운송 탄소를 추가한다. WASTE는 Exit로만 소비한다. 이 세 항목은
  아래 수정·확인 목록에 포함하며, 구현·실행 완료 상태와 구분한다.

## 2. 명시된 구현 수정 항목

| ID | 구현 수정 | 근거와 범위 | 상태 |
|---|---|---|---|
| LIFE-01 | Issue가 Note를 소비하고 공개 Claim을 생성하도록 연결; Exit는 무출력 종료 | 이미 정리한 Issue·Exit·Claim lifecycle. Claim status 제거, producer·audit terminal·DPP 검증 정렬 포함 | 구현 대기 |
| AUDIT-01 | 실제 forward 결과의 전체 frontier → Freeze → 공통 H_r 확인 연결 | 승인된 AuditAndFreeze 및 AF01–AF14 기준 | 구현 대기 |
| DOC-01 | DocumentInfo의 Unit을 canonical encoding과 hash에 포함 | 사용자 요청의 문서·식별자 항목 | 구현 대기 |
| DOC-02 | Merge·Split의 DocumentHash 보존 검사 추가 | 사용자 요청의 문서·식별자 항목 | 구현 대기 |
| ID-01 | Note/Voucher 소비 식별자와 domain을 v2 명세의 식으로 통일 | 사용자 요청의 문서·식별자 항목 | 구현 대기 |
| CLAIM-01 | 공개 Claim handle·DPPClaim·검증을 명세와 일치 | LIFE-01과 동일 작업에 연결하며 중복 구현하지 않음 | 구현 대기 |
| CARBON-01 | Entry 초기 탄소 입력 허용을 유지하고 생략 시 기본값 0 확인·정렬 | 사용자 결정. e=0 강제 회로 수정은 하지 않음 | 입력 경로·기본값 검증 대기 |
| CARBON-02 | Transfer 운송 탄소 증분을 Voucher에 추가 | 사용자 결정과 명세의 배분식 | 구현 대기 |
| WASTE-01 | WASTE Note의 비-Exit 소비 거부 | 사용자 확인 Exit-only 규칙. 정상 WASTE 생성·감사·동결은 유지 | 구현 대기 |

### CARBON-01. Entry 초기 탄소와 기본값

초기 탄소가 생략되면 client 입력 처리에서 0으로 정규화하고, 명시한 유효한
음이 아닌 값은 그대로 사용한다. 실제 사용한 값은 Note의 e 필드와 commitment,
proof witness에 결합한다. 잘못된 명시 입력을 0으로 바꾸어 통과시키지 않는다.

현재 [Entry 회로](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/features/entry/circuit.go:15)는
초기 e를 0으로 제한하지 않는다. 이 허용 범위는 유지한다. 앞선 e=0 강제
수정 요구는 철회하고, 실제 입력 API·client·fixture에서 기본값 의미와 범위
검사를 확인한다. 현재 회로의 양수 허용이 모든 입력 경로의 생략 기본값까지
검증한 근거는 아니다.

완료 확인: 생략 입력은 0, 명시적 0은 0, 유효한 양수는 그 값으로 Entry를
생성한다. 음수·범위 초과 입력과 commitment에 반영하지 않은 값 변경은 거부한다.
초기 탄소가 있는 Note의 이후 배분·운송·공정·Claim 계산에도 그 값이 반영되어야 한다.

### CARBON-02. Transfer 운송 탄소 추가

```text
e_C = floor(e_in * q_C / q_in)
e_T = e_in - e_C + delta_e_transport
delta_e_transport >= 0
```

Change Note는 입력 탄소 중 남는 질량의 몫을 유지한다. Voucher는 보내는
질량의 탄소 몫에 운송 증분을 더한다. 현재 [Transfer 회로](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/features/transfer/circuit.go:29)의
`e_in = e_T + e_C`를 위 관계로 수정하고 native allocation·witness·audit wrapper·
verifier·fixture를 함께 맞춘다. 운송 증분의 실제 값은 외부 입력이며 proof는
그 값의 범위와 올바른 가산·배분을 검사한다.

완료 확인: 운송 증분 0과 양수, 부분·전량 Transfer, 반올림 경계가 식과
일치한다. 양의 증분을 누락한 output, Change 쪽에 잘못 더한 output, 음수·
overflow는 거부한다. 받은 탄소 값은 Proceed/Recall을 거쳐 다음 Note에 보존된다.

### WASTE-01. Exit-only 소비 집행

Transfer·Merge·Split에 ELIGIBLE 입력 조건을 추가하고, Process·Issue 및
Voucher 생성·해결에서도 WASTE가 비-Exit 소비로 이어질 수 없는지 확인한다.
WASTE의 a_rec=e=0 규칙은 유지한다. Process의 정상 WASTE output 생성은 허용한다.

현재 [WASTE 상태 검사](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/internal/core/note/note.go:37)는
두 값의 0 조건을 갖지만 Transfer·Merge·Split의 role 보존만으로 소비 경로를
제한하지 못한다. 이 경로를 허용하던 테스트 기대값도 수정한다.

완료 확인: 다른 조건이 유효한 WASTE input의 Transfer·Merge·Split을 거부하고,
미소비·Active WASTE의 Exit는 허용한다. Frozen/Revoked WASTE의 Exit는 공통
status 규칙에 따라 거부한다. WASTE도 실제 graph·frontier와 동결 대상에
포함되며, Exit-only를 이유로 감사나 동결에서 제외하지 않는다.

### DOC-01. 문서 필드 결합

```text
DocumentInfo = (ProductName, LotID, Unit)
d = HashDocumentInfo(DocumentInfo)
```

현재 [DocumentInfo와 encoding](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/internal/core/document/document.go:12)은
ProductName과 LotID만 사용한다. Unit 필드를 자료형·JSON·문서 추출·canonical
encoding·hash·fixture에 일관되게 포함한다. 기존 문자열 정규화와 필드 길이
표현은 profile과 양립하면 재사용할 수 있다. 별도 형식을 이번 목록에서 선택하지 않는다.

완료 확인: 같은 ProductName/LotID에서 Unit만 바꾸면 d가 달라지고, 그 문서를
기존 Claim에 연결한 공개 검증이 거부되어야 한다. Prover의 문서 hash와 외부
DPP verifier의 hash는 같은 입력에서 같아야 한다. Unit은 DPP 문서 필드이며
회계 정수의 단위·scale 선택을 자동으로 대신하지 않는다.

### DOC-02. Merge·Split 문서 보존

```text
Merge: d_input1 = d_input2 = d_output
Split: d_input = d_output1 = d_output2
```

[Merge 회로](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/features/merge/circuit.go:25)와
[Split 회로](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/features/split/circuit.go:25)에
위 equality를 추가하고 witness 생성·상위 audit wrapper·시나리오를 같은
규칙에 맞춘다. 고정 verifier 경로는 유지하되 수정한 circuit에 대응하는 VK와
verifier artifact는 함께 갱신해야 한다.

완료 확인: 다른 조건은 유효한 상태에서 Merge의 서로 다른 입력 문서 또는
변조 출력 문서, Split의 변조 출력 문서만으로 거부되는지 확인한다. 같은 문서의
정상 Merge·Split은 허용한다. Process의 문서 변경 허용 범위를 제한하지 않는다.

### ID-01. 소비 식별자와 암호문·상태 키 정렬

```text
nf = H("zkDPP:Nullifier:v2", cm, ownerSecret)
s_res = H("zkDPP:VoucherResolutionSecret:v2", opening)
rvnf = H("zkDPP:VoucherNullifier:v2", rv, s_res)
```

현재 [Note nullifier](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/internal/core/note/note.go:78),
[Voucher nullifier](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/internal/core/voucher/voucher.go:53),
[회로 gadget](/Users/jongho/workspace/zkDPP-local/outputs/zkdpp-v2-m9-conformance-2026-09-07/source/zkDPP-poc-v1/internal/circuitutil/gadgets.go:98)을
동시에 맞춘다. Domain 상수, encoding, witness 생성, output 미래 식별자 암호화,
소비 public input, audit 결과, typed status key와 fixture가 같은 식을 사용해야 한다.
Note/Voucher commitment와 ObjectRef의 v2 domain도 같은 encoding 점검에 포함한다.

완료 확인: 생성 때 암호화한 미래 식별자가 실제 소비 식별자와 일치하고,
Proceed와 Recall이 같은 rvnf를 사용하며, 복호화한 key로 실제 frontier의
status를 변경해 후속 소비를 차단할 수 있어야 한다. 같은 raw 값의 Note/Voucher
key는 타입별로 구분한다. 이전 식과 새 식을 한 실행의 fixture나 원장 상태에 섞지 않는다.

### CLAIM-01. 공개 Claim과 검증

```text
h = H("zkDPP:Issue:v2", d, issuePolicyRef, claimNonce)
DPPClaim = (issuePolicyRef, h, claimNonce)
```

Issue의 Note 소비, 실제 parent 암호화, Claim 등록과 producer index를 원자적으로
연결한다. DPP에서 재계산한 h와 등록 결과를 확인하고, Claim의 별도 status는
두지 않는다. Policy/VK 선택은 기존 Issue 등록 경로를 사용하며 scope grant를
추가하지 않는다.

완료 확인: d·policy·nonce 변조, 미등록 handle, 동일 Note의 재발급을 거부하고
정상 Claim에서 backward/forward 경로가 명세의 Note·Issue·Claim 관계와 일치한다.
이 검증은 [기존 lifecycle 차이](/Users/jongho/workspace/zkDPP-local/notes/zkdpp-v2-implementation-gap-analysis.md)의
해결과 함께 수행한다.

## 3. 구현 의존성과 검증

Execution Architecture: 현재 주 에이전트가 사용자 결정과 수정 범위를 정리하고
문서의 연결을 검증한다. 동시성 1, 추가 worker 0이며 결정적 도구는 원문 hash·
링크·변경 범위 확인에만 사용한다. 이번 종료 기준은 수정 목록의 구체화이며
프로토콜 코드 구현 착수가 아니다.

실제 구현 시에는 문서·식별자 공통 함수와 회로를 정렬하고 대응 verifier·fixture를
갱신한 뒤 lifecycle·audit와 결합한다. 단순 문자열 교체 완료를 적합성 근거로
사용하지 않는다. Gate를 닫는 실행 근거는 별도로 확보해야 한다.

[AuditAndFreeze 구현 방향](/Users/jongho/workspace/zkDPP-local/notes/zkdpp-v2-audit-freeze-implementation-direction.md)은
전체 대상·동결·공통 결과 확인의 상세 기준이다. 위 세 회계 결정은 반영했으며,
deadline·구체 scale 등 별도 profile 항목을 이 결정에 섞어 자동 확정하지 않는다.
