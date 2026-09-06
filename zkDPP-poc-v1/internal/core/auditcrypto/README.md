# auditcrypto는 무엇을 제공하나요?

**Event를 모르는 Field 벡터 암호화·2-of-3 복호화 코어입니다.** Event adapter가 실제 감사 평문과 문맥을 구성합니다.

| 함수 | 역할 |
|---|---|
| TrustedSetup | 안전한 난수로 공개키와 세 위원의 shares를 만듭니다. |
| Encrypt | PK·문맥·평문·새 난수에서 공개점과 암호문 벡터를 만듭니다. |
| PartialDecrypt | 위원 share로 기록의 공개점에 대한 결과를 만듭니다. |
| CombineAndDecrypt | 서로 다른 두 위원의 결과로 같은 키와 마스크를 복원합니다. |
| EnsureCommittee / LoadCommittee | 기존 키를 검증해 재사용하며 private files를 분리합니다. |

메시지는 BLS12-381 scalar Field, 암호화는 그 위의 Jubjub prime-order subgroup입니다. 외부 점은 압축하지 않은 X,Y의 canonical 32-byte big-endian 두 값입니다.

Setup은 master secret을 반환하거나 저장하지 않습니다. 위원 파일은 mode 0600, committee 폴더는 0700입니다. 불완전하거나 불일치하는 Setup은 조용히 덮어쓰지 않습니다. Go 메모리의 완전한 삭제나 Production 보호는 보장하지 않습니다.

공통 Circuit 관계는 [AssertEncrypted](../../circuitutil/audit_encryption.go)에 있습니다. 구체 사용은 [독립 feature](../../../features/audit_encryption/README.md)와 [Process feature](../../../features/audit_process_3_2/README.md)입니다.

**이 코어는 인증 복호화 서비스가 아닙니다.** 정직한 위원회·Auditor가 승인된 온체인 원본만 다룬다는 POC 가정입니다. 형식이 올바른 거짓 응답이나 변조된 암호문은 다른 평문을 만들 수 있습니다. 거부 책임은 Event proof와 기록 원본 확인에 있습니다.
