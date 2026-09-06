# 실제 Process와 암호문은 어떻게 연결하나요?

**M5의 3→2 Process를 검증하고, 실제 부모와 올바른 출력 소비 nullifier가 암호문에 들어갔는지 추가로 확인합니다.**

[기존 M5 Circuit](../process_policy_3_2/circuit.go)은 수정하지 않고 Define 관계를 호출합니다. 실제 입력 commitment를 다시 얻는 작은 중복은 허용합니다.

## 무엇을 암호화하나요?

$$
(cm_A,cm_B,cm_C)\xrightarrow{\mathrm{Process}}(cm_D,cm_E)
$$

$$
\mathbf M=(cm_A,cm_B,cm_C,nf_D,nf_E)
$$

부모는 private 입력 Note에서 계산하고, 출력 nullifier는 같은 owner secret과 공개 output commitment에서 계산합니다. 별도 평문 배열을 받아 신뢰하지 않습니다.

| 구분 | 내용 |
|---|---|
| 공개 입력 앞 8개 | 기존 M5의 PolicyRef, ScopeRef, root, 입력 nf 3개, 출력 cm 2개 |
| 공개 입력 뒤 7개 | $R_1$의 좌표 2개와 암호문 Field 5개 |
| private witness | 기존 M5 witness와 암호화 난수 |
| 문맥 $L$ | AuditContextTag, EventKind 6, 기존 공개 입력 8개를 순서대로 Hash |

공개 입력은 총 15개입니다. 부모 commitment와 출력 소비 nullifier는 평문으로 공개하지 않습니다.

## 무엇이 더 검증되나요?

독립 암호화 Circuit은 엉뚱한 평문도 일관되게 암호화하면 성공할 수 있습니다. 이 adapter는 그 암호문이 실제 Event의 감사 메시지가 아니면 거부합니다. 잘못된 부모·출력 nf·배열 순서·문맥을 사용하는 paired test로 차이를 확인합니다.

기존 owner·membership·State·Role·Policy 수학도 그대로 검증합니다. Registry의 사용 권한과 실제 원장 소비 상태까지 이 Circuit 하나가 확인하는 것은 아닙니다.

## 어떻게 확인하나요?

프로젝트 root에서 make setup-m6-b1, make evaluate-m6-b1 후 make test-go와 make test-contract-m6-b1을 사용합니다.

make benchmark-m6-b1-gas는 작은 AuditEncryptionStore의 verifier·기록 비용을 측정하고, 원본 기록을 읽어 두 위원의 응답으로 평문을 복원합니다.

**이 Contract는 Main Ledger가 아닙니다.** 실제 Note 소비·Tree append·PolicyGrant·Status 집행과 그래프 탐색은 후속 단계입니다. 기존 M5 keys는 변경된 Circuit에 재사용하지 않고 별도의 개발용 Setup을 사용합니다.
