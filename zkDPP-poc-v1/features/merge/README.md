# Merge

같은 owner와 같은 AssetRole의 private Note 두 개를 소비해 State 합을 가진 새 Note를 만듭니다. ProductProfile과 DocumentHash compatibility는 M4 POC에서 검사하지 않습니다.

| 공개값 | 비공개값 |
|---|---|
| `noteRoot`, `nf1`, `nf2`, `cmOut` | input Note·`cm`·path 2개, owner secret, output Note |
