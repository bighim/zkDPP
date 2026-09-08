// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {ZkDPPAuditLedger as L} from "../src/ZkDPPAuditLedger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {AuditProcessVerifier} from "../src/AuditProcessVerifier.sol";
import {M7EntryVerifier} from "../src/M7EntryVerifier.sol";
import {M7TransferVerifier} from "../src/M7TransferVerifier.sol";
import {M7ProceedVerifier} from "../src/M7ProceedVerifier.sol";
import {M7RecallVerifier} from "../src/M7RecallVerifier.sol";
import {M7MergeVerifier} from "../src/M7MergeVerifier.sol";
import {M7SplitVerifier} from "../src/M7SplitVerifier.sol";
import {M7ExitVerifier} from "../src/M7ExitVerifier.sol";
interface VmM7{
 function readFile(string calldata)external view returns(string memory);
 function parseJsonBytes(string calldata,string calldata)external pure returns(bytes memory);
 function parseJsonUintArray(string calldata,string calldata)external pure returns(uint256[] memory);
 function parseJsonUint(string calldata,string calldata)external pure returns(uint256);
 function prank(address)external;
 function expectRevert(bytes4)external;
 function warp(uint256)external;
 function toString(uint256)external pure returns(string memory);
}
contract M7AuditLedgerTest{
 VmM7 constant vm=VmM7(address(uint160(uint256(keccak256("hevm cheat code")))));
 address constant ISSUER=address(0x1001);
 address constant AUTHORITY=address(0x4004);
 string fixture;
 L ledger;
 Poseidon2BLS12381 hasher;
 address[8] vs;
 function setUp()public{
  fixture=vm.readFile("test/fixtures/m7-proofs.json");hasher=new Poseidon2BLS12381();
  vs=[address(new M7EntryVerifier()),address(new M7TransferVerifier()),address(new M7ProceedVerifier()),address(new M7RecallVerifier()),address(new M7MergeVerifier()),address(new M7SplitVerifier()),address(new AuditProcessVerifier()),address(new M7ExitVerifier())];
  _freshLedger();
 }
 function _freshLedger()internal{ledger=new L(vs,address(hasher),AUTHORITY);ledger.setEntryIssuer(ISSUER,true);vm.warp(60000);}
 function _path(uint g,uint i,bool extra)internal pure returns(string memory){
  return string.concat(".Groups[",vm.toString(g),"].",extra?"Extra[":"Events[",vm.toString(i),"]");
 }
 function _get(uint g,uint i,bool extra)internal view returns(uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a){
  string memory path=_path(g,i,extra);
  k=uint8(vm.parseJsonUint(fixture,string.concat(path,".Kind")));
  proof=vm.parseJsonBytes(fixture,string.concat(path,".Proof"));
  uint256[] memory inputs=vm.parseJsonUintArray(fixture,string.concat(path,".PublicInputs"));
  uint b=k==0?1:k==1?6:k==2?3:k==3?4:k==4||k==5?4:k==6?8:2;
  uint parents=k==0?0:k==4?2:k==6?3:1;
  uint outputs=k==7?0:k==1||k==5||k==6?2:1;
  p=new uint256[](b);for(uint j;j<b;j++)p[j]=inputs[j];
  a.r1X=inputs[b];a.r1Y=inputs[b+1];a.encryptedParents=new uint256[](parents);a.encryptedOutputNfs=new uint256[](outputs);
  for(uint j;j<parents;j++)a.encryptedParents[j]=inputs[b+2+j];
  for(uint j;j<outputs;j++)a.encryptedOutputNfs[j]=inputs[b+2+parents+j];
 }
 function _execute(uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)internal returns(uint){
  if(k==0){vm.prank(ISSUER);return ledger.entry(proof,p[0],a);}
  if(k==1)return ledger.transfer(proof,p[0],p[1],p[2],p[3],p[4],p[5],a);
  if(k==2)return ledger.proceed(proof,p[0],p[1],p[2],a);
  if(k==3)return ledger.recall(proof,p[0],p[1],p[2],p[3],a);
  if(k==4)return ledger.merge(proof,p[0],p[1],p[2],p[3],a);
  if(k==5)return ledger.split(proof,p[0],p[1],p[2],p[3],a);
  if(k==6){uint256[3] memory n=[p[3],p[4],p[5]];uint256[2] memory c=[p[6],p[7]];return ledger.process(proof,p[0],p[1],p[2],n,c,a);}
  return ledger.exit(proof,p[0],p[1],a);
 }
 function _run(uint g,uint i)internal returns(uint id){
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(g,i,false);
  if(k==1)vm.warp(100*600);if(k==3)vm.warp(105*600);
  id=_execute(k,proof,p,a);
  L.AuditRecord memory r=ledger.getAuditRecord(id);
  require(r.eventKind==k&&r.r1X==a.r1X&&r.r1Y==a.r1Y,"record");
  require(keccak256(abi.encode(r.encryptedParents,r.encryptedOutputNfs))==keccak256(abi.encode(a.encryptedParents,a.encryptedOutputNfs)),"cipher");
  for(uint j;j<r.outputRefs.length;j++)require(ledger.producerOf(r.outputRefs[j].objectType,r.outputRefs[j].rawId)==id,"producer");
  if(k!=0){
   uint start=k==6?3:1;uint count=k==4?2:k==6?3:1;
   for(uint j;j<count;j++){require((k==2||k==3?ledger.voucherSpentIn(p[start+j]):ledger.noteSpentIn(p[start+j]))==id,"consumer");}
  }
 }
 function _status(uint8 typ,uint256 nf,uint8 state)internal{vm.prank(AUTHORITY);ledger.setStatus(typ,nf,state);}
 function _prefix(uint g,uint n)internal{for(uint i;i<n;i++)_run(g,i);}
 function _policy()internal{
  ledger.registerPolicyAuthority(address(this));uint ref=ledger.reservePolicy(6);
  (,,uint256[] memory p,)=_get(3,3,false);require(ref==p[0],"policy ref");
  ledger.registerPolicy(ref,3,2,bytes32(uint256(1)),vs[6]);ledger.setPolicyGrant(ref,p[1],true);
 }
 function testNoteGraphAndFreezeUnfreezeRevoke()public{
  _prefix(0,3);require(ledger.noteLeafCount()==5&&ledger.nextAuditRecordId()==4,"graph");
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(0,3,false);
  uint root=ledger.currentNoteRoot();_status(1,p[1],1);
  vm.expectRevert(L.InactiveObject.selector);_execute(k,proof,p,a);
  require(ledger.nextAuditRecordId()==4&&ledger.currentNoteRoot()==root,"frozen wrote");
  _status(1,p[1],0);require(_execute(k,proof,p,a)==4,"exit record");
  require(ledger.noteLeafCount()==5&&ledger.currentNoteRoot()==root,"exit append");
  vm.expectRevert(L.SpentNullifier.selector);_execute(k,proof,p,a);
  vm.expectRevert(L.SpentNullifier.selector);_status(1,p[1],1);
  (k,proof,p,a)=_get(0,1,true);_status(1,p[1],1);_status(1,p[1],2);
  vm.expectRevert(L.InactiveObject.selector);_execute(k,proof,p,a);
  vm.expectRevert(L.InvalidStatus.selector);_status(1,p[1],0);
 }
 function testVoucherProceedRecallAndFreeze()public{
  _prefix(1,3);
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(1,3,false);
  _status(2,p[1],1);vm.expectRevert(L.InactiveObject.selector);_execute(k,proof,p,a);
  vm.expectRevert(L.InactiveObject.selector);ledger.recall(proof,p[0],p[1],p[2],100,a);
  _status(2,p[1],0);_run(1,3);
  vm.expectRevert(L.ResolvedVoucher.selector);ledger.recall(proof,p[0],p[1],p[2],100,a);
  _run(1,4);(k,proof,p,a)=_get(1,5,false);vm.warp(105*600);
  _status(2,p[1],1);vm.expectRevert(L.InactiveObject.selector);_execute(k,proof,p,a);
  _status(2,p[1],0);_run(1,5);
  vm.expectRevert(L.ResolvedVoucher.selector);ledger.proceed(proof,p[0],p[1],p[2],a);
  require(ledger.voucherLeafCount()==2&&ledger.noteLeafCount()==6,"voucher final");
 }
 function testMergeAndProcessRecord()public{
  _prefix(2,4);require(ledger.noteLeafCount()==5&&ledger.nextAuditRecordId()==5,"merge");
  _freshLedger();_policy();_prefix(3,4);
  L.AuditRecord memory r=ledger.getAuditRecord(4);
  require(r.eventKind==6&&r.encryptedParents.length==3&&r.outputRefs.length==2&&r.encryptedOutputNfs.length==2,"process shape");
  require(ledger.noteLeafCount()==5,"process count");
 }
 function testEveryNoteConsumerRejectsFrozen()public{
  uint[5] memory groups=[uint(0),1,2,0,3];uint[5] memory indices=[uint(3),2,2,1,3];
  for(uint j;j<5;j++){
   _freshLedger();uint g=groups[j];uint i=indices[j];if(g==3)_policy();_prefix(g,i);
   (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(g,i,false);
   uint s=k==6?p[3]:p[1];_status(1,s,1);uint next=ledger.nextAuditRecordId();uint root=ledger.currentNoteRoot();
   vm.expectRevert(L.InactiveObject.selector);_execute(k,proof,p,a);
   require(ledger.nextAuditRecordId()==next&&ledger.currentNoteRoot()==root,"partial frozen write");
  }
 }
 function testPermissionInvalidCipherAndAtomicity()public{
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(0,0,false);
  vm.expectRevert(L.NotEntryIssuer.selector);ledger.entry(proof,p[0],a);
  ledger.setEntryIssuer(ISSUER,false);vm.expectRevert(L.NotEntryIssuer.selector);_execute(k,proof,p,a);ledger.setEntryIssuer(ISSUER,true);
  a.encryptedOutputNfs[0]++;uint root=ledger.currentNoteRoot();
  vm.expectRevert(L.InvalidProof.selector);_execute(k,proof,p,a);
  require(ledger.nextAuditRecordId()==1&&ledger.currentNoteRoot()==root&&!ledger.commitments(p[0]),"invalid proof wrote");
  a.encryptedOutputNfs=new uint256[](0);vm.expectRevert(L.InvalidAuditShape.selector);_execute(k,proof,p,a);
  vm.expectRevert(L.UnknownRecord.selector);ledger.getAuditRecord(1);
  _run(0,0);(k,proof,p,a)=_get(0,1,false);p[3]=p[2];
  vm.expectRevert(L.DuplicateCommitment.selector);_execute(k,proof,p,a);
  require(ledger.nextAuditRecordId()==2&&ledger.noteLeafCount()==1&&!ledger.noteNullifiers(p[1]),"duplicate output partial");
 }
 function testStatusAuthorityAndInvalidTransitions()public{
  vm.expectRevert(L.NotStatusAuthority.selector);ledger.setStatus(1,123,1);
  vm.expectRevert(L.InvalidStatus.selector);_status(1,123,2);
  _status(1,123,1);require(ledger.voucherStatusByNf(123)==0,"type separation");
  vm.expectRevert(L.InvalidStatus.selector);_status(1,123,1);
  vm.expectRevert(L.InvalidObjectType.selector);_status(3,123,1);
  vm.expectRevert(L.InvalidField.selector);_status(1,type(uint256).max,1);
  _status(1,123,2);vm.expectRevert(L.InvalidStatus.selector);_status(1,123,0);
 }
 function testPolicyGrantDisableAndWrongCaller()public{
  _policy();_prefix(3,3);
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(3,3,false);
  ledger.setPolicyGrant(p[0],p[1],false);vm.expectRevert(L.MissingPolicyGrant.selector);_execute(k,proof,p,a);
  ledger.setPolicyGrant(p[0],p[1],true);_execute(k,proof,p,a);ledger.disablePolicy(p[0]);
  require(ledger.getAuditRecord(4).policyRef==p[0],"disable erased audit");
  vm.expectRevert(L.DisabledPolicy.selector);_execute(k,proof,p,a);
 }
 function testRecallEpochAndInvalidRoot()public{
  _prefix(1,5);(uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_get(1,5,false);
  vm.warp(106*600);vm.expectRevert(L.InvalidEpoch.selector);_execute(k,proof,p,a);
  p[3]=106;vm.expectRevert(L.InvalidProof.selector);_execute(k,proof,p,a);
  p[0]=0;vm.expectRevert(L.InvalidRoot.selector);_execute(k,proof,p,a);
 }
}
