// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {ZkDPPV2Ledger as L} from "../src/ZkDPPV2Ledger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {HFEntry} from "../src/HFEntry.sol";
import {HFTransfer} from "../src/HFTransfer.sol";
import {HFProceed} from "../src/HFProceed.sol";
import {HFRecall} from "../src/HFRecall.sol";
import {HFMerge} from "../src/HFMerge.sol";
import {HFSplit} from "../src/HFSplit.sol";
import {HFExit} from "../src/HFExit.sol";
import {HFProcess} from "../src/HFProcess.sol";
import {HFStandard} from "../src/HFStandard.sol";
import {HFStrict} from "../src/HFStrict.sol";
interface VmHF {function readFile(string calldata)external view returns(string memory);function parseJsonBytes(string calldata,string calldata)external pure returns(bytes memory);function parseJsonUint(string calldata,string calldata)external pure returns(uint256);function parseJsonUintArray(string calldata,string calldata)external pure returns(uint256[] memory);function toString(uint256)external pure returns(string memory);function expectRevert()external;function roll(uint256)external;}
contract IntegrationTest {
 VmHF constant vm=VmHF(address(uint160(uint256(keccak256("hevm cheat code")))));
 L ledger;string fixture;address processV;address standardV;address strictV;uint256 pr;uint256 sr;uint256 tr;
 function setUp()public {
 fixture=vm.readFile("../test/fixtures/v2-m1-hotfix-proofs.json");
 address[7] memory fixedV=[address(new HFEntry()),address(new HFTransfer()),address(new HFProceed()),address(new HFRecall()),address(new HFMerge()),address(new HFSplit()),address(new HFExit())];
 processV=address(new HFProcess());standardV=address(new HFStandard());strictV=address(new HFStrict());
 ledger=new L(fixedV,address(new Poseidon2BLS12381()),address(this));require(address(ledger).code.length<=24576,"EIP170");ledger.setEntryIssuer(address(this),true);ledger.registerPolicyAuthority(address(this));pr=ledger.reservePolicy(6);ledger.registerPolicy(pr,3,2,bytes32(uint256(1)),processV);sr=ledger.reservePolicy(8);ledger.registerPolicy(sr,1,1,bytes32(uint256(2)),standardV);tr=ledger.reservePolicyVersion(2);ledger.registerPolicy(tr,1,1,bytes32(uint256(3)),strictV);
 }
 function read(uint256 group,uint256 i)internal view returns(uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a){
 string memory path=string.concat(".Groups[",vm.toString(group),"].Events[",vm.toString(i),"]");k=uint8(vm.parseJsonUint(fixture,string.concat(path,".Kind")));proof=vm.parseJsonBytes(fixture,string.concat(path,".Proof"));p=vm.parseJsonUintArray(fixture,string.concat(path,".Public"));uint b=k==0?1:k==1?5:k==2?3:k==3||k==4||k==5||k==8?4:k==6?8:2;uint parents=k==0?0:k==4?2:k==6?3:1;uint future=k==7||k==8?0:k==1||k==5||k==6?2:1;a.r1X=p[b];a.r1Y=p[b+1];a.encryptedParents=new uint256[](parents);a.encryptedOutputNfs=new uint256[](future);for(uint j;j<parents;j++)a.encryptedParents[j]=p[b+2+j];for(uint j;j<future;j++)a.encryptedOutputNfs[j]=p[b+2+parents+j];
 }
 function execute(uint256 group,uint256 i)internal returns(uint256){
 (uint8 k,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=read(group,i);
 if(k==0)return ledger.entry(proof,p[0],a);if(k==1)return ledger.transfer(proof,p[0],p[1],p[2],p[3],p[4],a);if(k==2)return ledger.proceed(proof,p[0],p[1],p[2],a);if(k==3)return ledger.recall(proof,p[0],p[1],p[2],p[3],a);if(k==4)return ledger.merge(proof,p[0],p[1],p[2],p[3],a);if(k==5)return ledger.split(proof,p[0],p[1],p[2],p[3],a);if(k==7)return ledger.exit(proof,p[0],p[1],a);if(k==8)return ledger.issue(proof,p[0],p[1],p[2],p[3],a);
 ledger.setPolicyGrant(p[0],p[1],true);return ledger.process(proof,p[0],p[1],p[2],[p[3],p[4],p[5]],[p[6],p[7]],a);
 }
 function testAllRealEvents()public{for(uint i;i<20;i++)execute(0,i);require(ledger.nextAuditRecordId()==21,"records");(,,,L.AuditCipher memory unused)=read(0,0);unused; (, ,uint256[] memory p,)=read(0,17);require(ledger.claimRegistered(p[3]),"standard");vm.expectRevert();execute(0,17);}
 function testFrozenAndDeadlineAtomicity()public{for(uint i;i<4;i++)execute(0,i);(,,uint256[] memory p,)=read(0,4);uint before=ledger.nextAuditRecordId();ledger.setStatus(1,p[1],1);vm.expectRevert();execute(0,4);require(ledger.nextAuditRecordId()==before,"partial");ledger.setStatus(1,p[1],0);vm.roll(p[4]);vm.expectRevert();execute(0,4);vm.roll(p[4]-1);execute(0,4);(,,p,)=read(0,5);vm.roll(p[3]);execute(0,5);vm.expectRevert();execute(0,5);}
 function testCipherTamperRejected()public{(,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=read(0,0);a.encryptedOutputNfs[0]++;vm.expectRevert();ledger.entry(proof,p[0],a);require(ledger.nextAuditRecordId()==1,"atomic");}
 function testAuditTreeRealProofs()public{execute(1,0);execute(1,1);execute(1,2);require(ledger.noteLeafCount()==5&&ledger.nextAuditRecordId()==4,"audit counts");}
 function testWrongPolicyVerifierRejects()public{for(uint i;i<17;i++)execute(0,i);(,bytes memory proof,uint256[] memory p,)=read(0,17);require(!HFStrict(strictV).Verify(proof,p),"wrong verifier accepted");}
 function testFrozenVoucherAndExpiredRecall()public{for(uint i;i<5;i++)execute(0,i);(,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=read(0,5);ledger.setStatus(2,p[1],1);vm.expectRevert();ledger.recall(proof,p[0],p[1],p[2],p[3],a);vm.expectRevert();ledger.proceed(proof,p[0],p[1],p[2],a);ledger.setStatus(2,p[1],0);vm.roll(p[3]+1);vm.expectRevert();ledger.recall(proof,p[0],p[1],p[2],p[3],a);require(ledger.voucherSpentIn(p[1])==0,"voucher spent on failure");}
 function testProcessGrantAndWrongVerifier()public{for(uint i;i<15;i++)execute(0,i);(,bytes memory proof,uint256[] memory p,L.AuditCipher memory a)=read(0,15);vm.expectRevert();ledger.process(proof,p[0],p[1],p[2],[p[3],p[4],p[5]],[p[6],p[7]],a);ledger.setPolicyGrant(p[0],p[1],true);uint before=ledger.nextAuditRecordId();p[1]++;vm.expectRevert();ledger.process(proof,p[0],p[1],p[2],[p[3],p[4],p[5]],[p[6],p[7]],a);require(ledger.nextAuditRecordId()==before,"failed process wrote record");}
}
