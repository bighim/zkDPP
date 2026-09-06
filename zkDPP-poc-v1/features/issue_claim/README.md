# M8 Issue Claim

공개 dppCommitment의 private 원문이 하나의 Issue Policy를 만족하는지 증명합니다. 공개 입력은 issuePolicyRef와 dppCommitment이며 제품 정보와 State는 공개하지 않습니다.

Standard V1과 Strict V2는 같은 Circuit 구현을 서로 다른 compile-time 상수로 Setup합니다. Issue는 Note membership·owner·nf를 다시 검사하지 않으며 ELIGIBLE DPP만 허용합니다.
