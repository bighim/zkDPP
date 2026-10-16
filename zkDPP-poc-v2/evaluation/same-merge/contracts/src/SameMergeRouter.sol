// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;
interface IV { function Verify(bytes calldata proof,uint256[] calldata pi) external view returns(bool); }
// Single canonical statement; one atomic conjunction; no state update in this
// measured wrapper. Runtime checks remain in the unchanged production ledger.
contract SameMergeRouter {
 IV public immutable unified;IV public immutable transaction;IV public immutable audit;
 constructor(address u,address t,address a){require(u.code.length>0&&t.code.length>0&&a.code.length>0,"verifier code");unified=IV(u);transaction=IV(t);audit=IV(a);}
 function verifyU(bytes calldata proof,uint256[9] calldata s) external view returns(bool){
  uint256[] memory pi=new uint256[](9);for(uint256 i;i<9;i++)pi[i]=s[i];return unified.Verify(proof,pi);
 }
 function verifyD(bytes[2] calldata proofs,uint256[9] calldata s) external view returns(bool){
  uint256[] memory txpi=new uint256[](4);for(uint256 i;i<4;i++)txpi[i]=s[i];
  if(!transaction.Verify(proofs[0],txpi))return false;
  uint256[] memory api=new uint256[](8);for(uint256 i;i<8;i++)api[i]=s[i+1];return audit.Verify(proofs[1],api);
 }
}
// Adapter lets the unchanged current ledger verify the proof bundle once under
// its existing nine-scalar Merge interface. No new ledger entry or state flow.
contract SplitMergeAdapter {
 SameMergeRouter public immutable router;
 constructor(SameMergeRouter r){router=r;}
 function Verify(bytes calldata proof,uint256[] calldata pi) external view returns(bool){
  if(pi.length!=9)return false;bytes[2] memory ps=abi.decode(proof,(bytes[2]));uint256[9] memory s;
  for(uint256 i;i<9;i++)s[i]=pi[i];return router.verifyD(ps,s);
 }
}
