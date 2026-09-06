# Status Update

Status Authority가 private Status path 하나로 `oldRoot → newRoot` 전이가 올바름을 증명합니다. 공개 입력은 object type, 두 root, 기존 index와 두 Status이며 Path 32개는 비공개입니다. Contract는 현재 root를 직접 public input으로 넣고 성공 시 new root 하나만 저장합니다.

허용 전이는 Active→Frozen, Frozen→Active, Frozen→Revoked입니다. Batch update와 온체인 Merkle path 계산은 지원하지 않습니다.
