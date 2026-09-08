# 감사 암호화 코어는 무엇을 검증하나요?

**private Field 벡터를 올바르게 암호화했는지 검증합니다. 그 벡터가 실제 공급망 Event의 정보인지는 판단하지 않습니다.**

구현 기준은 [M6-B1 명세](../../../zkDPP-poc-v1/milestones/M6-B1-audit-encryption-core.md)입니다.

## 키 하나로 어떻게 벡터를 숨기나요?

Jubjub에서 $R_1=rG$, $Z=rPK$를 계산합니다. 기존 Poseidon2로 transaction 키와 위치별 마스크를 만듭니다.

$$
K=H(\mathrm{AuditKeyTag},Z_X,Z_Y,L,n)
$$

$$
k_j=H(\mathrm{AuditMaskTag},K,j),\qquad C_j=M_j+k_j\pmod p
$$

전체 암호문은 $R_1$과 암호화된 Field 배열입니다. 한 키를 사용하지만 같은 숫자를 모든 원소에 더하지 않습니다.

| 구분 | 내용 |
|---|---|
| 공개 입력 | $L,R_{1X},R_{1Y},C_0,\ldots,C_4$ — 8개 |
| private witness | 평문 다섯 Field와 난수 $r$ |
| Circuit 상수 | 위원회 PK, Jubjub·Poseidon2 설정 |
| 재사용 단위 | AssertEncrypted gadget과 Native auditcrypto 함수 |

## 어떤 경우를 거부하나요?

잘못된 암호문·공개점·문맥·평문 관계, 0 또는 범위 밖인 난수, 다른 위원회 공개키로 만든 암호문을 거부합니다. Native decoding은 canonical Field·scalar·곡선·subgroup을 확인합니다.

반면 **임의의 평문을 그에 맞게 올바르게 암호화했다면 성공합니다.** 실제 부모·출력 nullifier를 강제하려면 [Process adapter](../audit_process_3_2/README.md)가 필요합니다.

## 어떻게 실행하나요?

프로젝트 root에서 다음 순서로 실행합니다.

1. make setup-m6-b1
2. make evaluate-m6-b1
3. make test-go

첫 명령은 키·개발 SRS·Circuit keys를 준비합니다. 두 번째는 두 Circuit의 공식 proof를 한 번씩 생성합니다. 이미 결과가 있으면 중복 공식 측정을 거부합니다.

같은 gadget의 1-Field 사용은 correctness test에서만 확인합니다. 1-Field용 별도 Setup·배포는 하지 않습니다.

## 한계는 무엇인가요?

정직한 위원회와 Auditor가 검증된 기록만 처리하는 POC입니다. 저수준 복호화 함수는 임의의 잘못된 암호문을 인증하지 않습니다. 원 TDH2의 응답 증명·Gamma·R2·e/f는 없습니다. 일반적인 CCA 보안·Production secret 보관·상수 시간 실행을 주장하지 않습니다.
