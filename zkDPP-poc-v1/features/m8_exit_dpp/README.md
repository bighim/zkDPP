# M8 Exit DPP

private Note를 한 번 소비하고 같은 DocumentHash·AssetRole·State를 가진 공개 DPP commitment를 만듭니다. 공개 입력은 noteRoot, nf, dppCommitment, 감사 공개점 두 좌표와 암호화된 부모 cm입니다. DPP에는 owner address·Note opening·nf를 넣지 않습니다.

ELIGIBLE과 WASTE를 모두 Exit할 수 있습니다. 잘못된 Note path·owner·nf·DPP 원문·감사 암호문은 proof를 만들 수 없습니다.
