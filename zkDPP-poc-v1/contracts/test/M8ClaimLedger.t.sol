// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {ZkDPPClaimLedger as L} from "../src/ZkDPPClaimLedger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {AuditProcessVerifier} from "../src/AuditProcessVerifier.sol";
import {M7EntryVerifier} from "../src/M7EntryVerifier.sol";
import {M7TransferVerifier} from "../src/M7TransferVerifier.sol";
import {M7ProceedVerifier} from "../src/M7ProceedVerifier.sol";
import {M7RecallVerifier} from "../src/M7RecallVerifier.sol";
import {M7MergeVerifier} from "../src/M7MergeVerifier.sol";
import {M7SplitVerifier} from "../src/M7SplitVerifier.sol";
import {M8ExitVerifier} from "../src/M8ExitVerifier.sol";
import {IssueStandardV1Verifier} from "../src/IssueStandardV1Verifier.sol";
import {IssueStrictV2Verifier} from "../src/IssueStrictV2Verifier.sol";

interface VmM8 {
 function readFile(string calldata) external view returns(string memory);
 function parseJsonBytes(string calldata,string calldata) external pure returns(bytes memory);
 function parseJsonUintArray(string calldata,string calldata) external pure returns(uint256[] memory);
 function prank(address) external;
 function expectRevert(bytes4) external;
}

contract M8ClaimLedgerTest {
 VmM8 constant vm=VmM8(address(uint160(uint256(keccak256("hevm cheat code")))));
 address constant ISSUER=address(0x1001);address constant STATUS_AUTHORITY=address(0x4004);
 L ledger;address[8] vs;IssueStandardV1Verifier standard;IssueStrictV2Verifier strict;
 string m7;string m8;

 function setUp() public {
  m7=vm.readFile("test/fixtures/m7-proofs.json");m8=vm.readFile("test/fixtures/m8-proofs.json");
  Poseidon2BLS12381 hasher=new Poseidon2BLS12381();standard=new IssueStandardV1Verifier();strict=new IssueStrictV2Verifier();
  vs=[address(new M7EntryVerifier()),address(new M7TransferVerifier()),address(new M7ProceedVerifier()),address(new M7RecallVerifier()),address(new M7MergeVerifier()),address(new M7SplitVerifier()),address(new AuditProcessVerifier()),address(new M8ExitVerifier())];
  ledger=new L(vs,address(hasher),STATUS_AUTHORITY);ledger.setEntryIssuer(ISSUER,true);
  require(address(ledger).code.length<=24576,"EIP-170");
 }

 function _m7(uint i) internal view returns(uint8 kind,bytes memory proof,uint256[] memory p,L.AuditCipher memory a){
  string memory base=string.concat(".Groups[3].Events[",_u(i),"]");
  proof=vm.parseJsonBytes(m7,string.concat(base,".Proof"));uint256[] memory all=vm.parseJsonUintArray(m7,string.concat(base,".PublicInputs"));
  kind=i<3?0:6;uint b=kind==0?1:8;uint parents=kind==0?0:3;uint outputs=kind==0?1:2;
  p=new uint256[](b);for(uint j;j<b;j++)p[j]=all[j];a.r1X=all[b];a.r1Y=all[b+1];a.encryptedParents=new uint256[](parents);a.encryptedOutputNfs=new uint256[](outputs);
  for(uint j;j<parents;j++)a.encryptedParents[j]=all[b+2+j];for(uint j;j<outputs;j++)a.encryptedOutputNfs[j]=all[b+2+parents+j];
 }
 function _m8(string memory name) internal view returns(bytes memory proof,uint256[] memory p){proof=vm.parseJsonBytes(m8,string.concat(".",name,".Proof"));p=vm.parseJsonUintArray(m8,string.concat(".",name,".PublicInputs"));}
 function _u(uint x) internal pure returns(string memory){if(x==0)return "0";if(x==1)return "1";if(x==2)return "2";return "3";}

 function _policyAndProcess() internal {
  ledger.registerPolicyAuthority(address(this));uint ref=ledger.reservePolicy(6);
  (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=_m7(3);k;
  require(ref==p[0],"process ref");ledger.registerPolicy(ref,3,2,bytes32(uint256(1)),vs[6]);ledger.setPolicyGrant(ref,p[1],true);
  for(uint i;i<3;i++){(,proof,p,a)=_m7(i);vm.prank(ISSUER);ledger.entry(proof,p[0],a);}
  (,proof,p,a)=_m7(3);uint256[3] memory nfs=[p[3],p[4],p[5]];uint256[2] memory cms=[p[6],p[7]];ledger.process(proof,p[0],p[1],p[2],nfs,cms,a);
 }
 function _issuePolicies() internal returns(uint standardRef,uint strictRef){
  standardRef=ledger.reservePolicy(8);(,uint256[] memory sp)=_m8("Standard");require(standardRef==sp[0],"standard ref");ledger.registerPolicy(standardRef,1,1,bytes32(uint256(2)),address(standard));
  strictRef=ledger.reservePolicyVersion(2);(,uint256[] memory tp)=_m8("Strict");require(strictRef==tp[0],"strict ref");ledger.registerPolicy(strictRef,1,1,bytes32(uint256(3)),address(strict));
 }
 function _exit() internal returns(uint dpp,uint aid){(bytes memory proof,uint256[] memory p)=_m8("EligibleExit");L.AuditCipher memory a;a.r1X=p[3];a.r1Y=p[4];a.encryptedParents=new uint256[](1);a.encryptedParents[0]=p[5];a.encryptedOutputNfs=new uint256[](0);dpp=p[2];aid=ledger.exit(proof,p[0],p[1],p[2],a);}

 function testCanonicalExitIssueAndIndependentStatus() public {
  _policyAndProcess();(uint sr,uint tr)=_issuePolicies();(uint dpp,uint exitID)=_exit();require(ledger.producerOf(3,dpp)==exitID,"DPP producer");
  (bytes memory proof,uint256[] memory p)=_m8("Standard");uint sid=ledger.issue(proof,p[0],p[1]);
  vm.expectRevert(L.DuplicateClaim.selector);ledger.issue(proof,p[0],p[1]);
  (proof,p)=_m8("Strict");uint tid=ledger.issue(proof,p[0],p[1]);
  (bool ok,uint8 state,uint id)=ledger.verifyClaim(dpp,sr);require(ok&&state==0&&id==sid,"standard claim");
  (ok,state,id)=ledger.verifyClaim(dpp,tr);require(ok&&state==0&&id==tid,"strict claim");
  vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,sr,1);vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,sr,0);
  vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,tr,1);vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,tr,2);
  (,state,)=ledger.verifyClaim(dpp,sr);require(state==0,"standard independent");(,state,)=ledger.verifyClaim(dpp,tr);require(state==2,"strict revoked");
  L.AuditRecord memory issueRecord=ledger.getAuditRecord(sid);require(issueRecord.eventKind==8&&issueRecord.policyRef==sr&&issueRecord.outputRefs.length==0&&issueRecord.r1X==0&&issueRecord.r1Y==0&&issueRecord.encryptedParents.length==0&&issueRecord.encryptedOutputNfs.length==0,"Issue record shape");
 }

 function testExitAndIssueFailuresAreAtomic() public {
  _policyAndProcess();(uint sr,)=_issuePolicies();(bytes memory proof,uint256[] memory p)=_m8("EligibleExit");L.AuditCipher memory a;a.r1X=p[3];a.r1Y=p[4];a.encryptedParents=new uint256[](1);a.encryptedParents[0]=p[5]+1;a.encryptedOutputNfs=new uint256[](0);uint beforeID=ledger.nextAuditRecordId();
  vm.expectRevert(L.InvalidProof.selector);ledger.exit(proof,p[0],p[1],p[2],a);require(ledger.nextAuditRecordId()==beforeID&&ledger.noteSpentIn(p[1])==0&&ledger.producerOf(3,p[2])==0,"exit atomicity");
  a.encryptedParents[0]=p[5];vm.prank(STATUS_AUTHORITY);ledger.setStatus(1,p[1],1);vm.expectRevert(L.InactiveObject.selector);ledger.exit(proof,p[0],p[1],p[2],a);vm.prank(STATUS_AUTHORITY);ledger.setStatus(1,p[1],0);uint duplicateDPP=p[2];ledger.exit(proof,p[0],p[1],p[2],a);
  (proof,p)=_m8("WasteExit");a.r1X=p[3];a.r1Y=p[4];a.encryptedParents[0]=p[5];vm.expectRevert(L.DuplicateDPP.selector);ledger.exit(proof,p[0],p[1],duplicateDPP,a);
  (proof,p)=_m8("Standard");p[1]++;beforeID=ledger.nextAuditRecordId();vm.expectRevert(L.UnknownDPP.selector);ledger.issue(proof,sr,p[1]);require(ledger.nextAuditRecordId()==beforeID,"issue atomicity");
 }

 function testPolicyAndClaimStatusGuards() public {
  _policyAndProcess();(uint sr,uint tr)=_issuePolicies();(uint dpp,)=_exit();
  vm.expectRevert(L.InvalidIssueGrant.selector);ledger.setPolicyGrant(sr,123,true);
  vm.expectRevert(L.NotStatusAuthority.selector);ledger.setClaimStatus(dpp,sr,1);vm.prank(STATUS_AUTHORITY);vm.expectRevert(L.UnknownClaim.selector);ledger.setClaimStatus(dpp,sr,1);
  (bytes memory proof,uint256[] memory p)=_m8("Standard");ledger.issue(proof,p[0],p[1]);ledger.disablePolicy(tr);(proof,p)=_m8("Strict");vm.expectRevert(L.DisabledPolicy.selector);ledger.issue(proof,p[0],p[1]);
  (bool registered,uint8 state,uint id)=ledger.verifyClaim(dpp,tr);require(!registered&&state==0&&id==0,"unregistered default");
  vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,sr,1);vm.prank(STATUS_AUTHORITY);ledger.setClaimStatus(dpp,sr,2);vm.prank(STATUS_AUTHORITY);vm.expectRevert(L.InvalidStatus.selector);ledger.setClaimStatus(dpp,sr,0);
 }
}
