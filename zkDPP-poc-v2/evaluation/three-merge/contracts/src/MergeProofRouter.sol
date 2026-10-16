// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;
interface IV { function Verify(bytes calldata p,uint256[] calldata pi) external view returns(bool); }
// Stateless proof-part wrappers. No ledger, RSA accumulator, union or primality
// processing. Shared statement is supplied once and all gadget inputs derived here.
contract MergeProofRouter {
 IV public immutable path; IV public immutable eq; IV public immutable add;
 IV public immutable combined; IV public immutable current;
 constructor(address p,address e,address a,address b,address c){
  require(p.code.length>0&&e.code.length>0&&a.code.length>0&&b.code.length>0&&c.code.length>0,"verifier code");
  path=IV(p);eq=IV(e);add=IV(a);combined=IV(b);current=IV(c);
 }
 function verifyA(bytes[6] calldata proofs,uint256[15] calldata s) external view returns(bool){
  for(uint256 i=0;i<3;i++){
   uint256[] memory pi=new uint256[](5);pi[0]=s[i];
   for(uint256 j=0;j<4;j++)pi[j+1]=s[3+4*i+j];
   if(!path.Verify(proofs[i],pi))return false;
  }
  for(uint256 i=0;i<2;i++){
   uint256[] memory pi=new uint256[](2);pi[0]=s[3+4*i+1];pi[1]=s[11];
   if(!eq.Verify(proofs[3+i],pi))return false;
  }
  uint256[] memory sum=new uint256[](3);sum[0]=s[6];sum[1]=s[10];sum[2]=s[14];
  return add.Verify(proofs[5],sum);
 }
 function verifyB(bytes calldata proof,uint256[15] calldata s) external view returns(bool){
  uint256[] memory pi=new uint256[](15);for(uint256 i=0;i<15;i++)pi[i]=s[i];
  return combined.Verify(proof,pi);
 }
 function verifyC(bytes calldata proof,uint256[9] calldata s) external view returns(bool){
  uint256[] memory pi=new uint256[](9);for(uint256 i=0;i<9;i++)pi[i]=s[i];
  return current.Verify(proof,pi);
 }
}

