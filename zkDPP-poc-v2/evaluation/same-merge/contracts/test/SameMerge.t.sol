// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;
import {SameMergeRouter,SplitMergeAdapter} from "../src/SameMergeRouter.sol";
import {PlonkVerifier as U} from "../src/generated/U/PlonkVerifier.sol";
import {PlonkVerifier as T} from "../src/generated/T/PlonkVerifier.sol";
import {PlonkVerifier as A} from "../src/generated/A/PlonkVerifier.sol";
import {ZkDPPV2Ledger as L} from "../src/runtime/ZkDPPV2Ledger.sol";
import {Poseidon2BLS12381} from "../src/runtime/Poseidon2BLS12381.sol";
interface Vm {
 function readFile(string calldata) external returns(string memory);
 function parseJsonStringArray(string calldata,string calldata) external pure returns(string[] memory);
 function parseJsonBytes(string calldata,string calldata) external pure returns(bytes memory);
 function parseUint(string calldata) external pure returns(uint256);
 function parseBytes(string calldata) external pure returns(bytes memory);
 function projectRoot() external returns(string memory);
 function expectRevert(bytes4) external;
}
// Trusted initial-state bootstrap only; actual Merge uses U or the real D adapter.
contract BootstrapVerifier {function Verify(bytes calldata,uint256[] calldata) external pure returns(bool){return true;}}
contract SameMergeTest {
 Vm constant vm=Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
 SameMergeRouter router;U u;SplitMergeAdapter adapter;bytes pu;bytes altU;bytes[2] pd;bytes[2] altD;
 uint256[9] s;uint256[9] altS;uint256[2] parents;
 function setUp() public {
  u=new U();router=new SameMergeRouter(address(u),address(new T()),address(new A()));adapter=new SplitMergeAdapter(router);
  string memory data=vm.readFile(string.concat(vm.projectRoot(),"/test/fixtures/proofs.json"));
  pu=vm.parseJsonBytes(data,".U");altU=vm.parseJsonBytes(data,".altU");
  string[] memory d=vm.parseJsonStringArray(data,".D");string[] memory ad=vm.parseJsonStringArray(data,".altD");for(uint256 i;i<2;i++){pd[i]=vm.parseBytes(d[i]);altD[i]=vm.parseBytes(ad[i]);}
  string[] memory cs=vm.parseJsonStringArray(data,".canonical");string[] memory acs=vm.parseJsonStringArray(data,".altCanonical");for(uint256 i;i<9;i++){s[i]=vm.parseUint(cs[i]);altS[i]=vm.parseUint(acs[i]);}
  string[] memory ps=vm.parseJsonStringArray(data,".altParents");for(uint256 i;i<2;i++)parents[i]=vm.parseUint(ps[i]);
 }
 function acceptsU(bytes memory p,uint256[9] memory st) internal view returns(bool){try router.verifyU(p,st) returns(bool b){return b;}catch{return false;}}
 function acceptsD(bytes[2] memory p,uint256[9] memory st) internal view returns(bool){try router.verifyD(p,st) returns(bool b){return b;}catch{return false;}}
 function testValidAndAlternative() public view {require(router.verifyU(pu,s)&&router.verifyD(pd,s),"valid");require(router.verifyU(altU,altS)&&router.verifyD(altD,altS),"alt");}
 function testMixEachModule() public view {for(uint256 i;i<2;i++){bytes[2] memory p=pd;p[i]=altD[i];require(!acceptsD(p,s),"mix accepted");}}
 function testAllPublicFields() public view {for(uint256 i;i<9;i++){uint256[9] memory st=s;st[i]=8;require(!acceptsU(pu,st)&&!acceptsD(pd,st),"changed public");}}
 function testNoCodeVerifier() public {try new SameMergeRouter(address(0),address(u),address(adapter)){revert("no code accepted");}catch{}}
 function ledger(bool split) internal returns(L l){
  BootstrapVerifier init=new BootstrapVerifier();address[7] memory vs;for(uint256 i;i<7;i++)vs[i]=address(init);vs[4]=split?address(adapter):address(u);
  l=new L(vs,address(new Poseidon2BLS12381()),address(this));l.setEntryIssuer(address(this),true);
  L.AuditCipher memory a;a.encryptedParents=new uint256[](0);a.encryptedOutputNfs=new uint256[](1);a.r1X=1;a.r1Y=1;
  l.entry(hex"01",parents[0],a);l.entry(hex"01",parents[1],a);require(l.currentNoteRoot()==altS[0],"actual initial tree");
 }
 function cipher(uint256[9] memory st) internal pure returns(L.AuditCipher memory a){a.r1X=st[4];a.r1Y=st[5];a.encryptedParents=new uint256[](2);a.encryptedParents[0]=st[6];a.encryptedParents[1]=st[7];a.encryptedOutputNfs=new uint256[](1);a.encryptedOutputNfs[0]=st[8];}
 function execute(L l,bytes memory p,uint256[9] memory st) internal returns(uint256){return l.merge(p,st[0],st[1],st[2],st[3],cipher(st));}
 function proof(bool split) internal view returns(bytes memory){return split?abi.encode(altD):altU;}
 function unchanged(L l,uint256 root,uint256 count,uint256 next) internal view {
  require(l.currentNoteRoot()==root&&l.noteLeafCount()==count&&l.nextAuditRecordId()==next,"partial state");require(l.noteSpentIn(altS[1])==0&&l.noteSpentIn(altS[2])==0&&!l.commitments(altS[3]),"partial index");
 }
 function testSameAtomicRuntimeSuccessAndReplay() public {
  L lu=ledger(false);L ld=ledger(true);uint256 idU=execute(lu,proof(false),altS);uint256 idD=execute(ld,proof(true),altS);
  require(idU==3&&idD==3&&lu.noteLeafCount()==3&&ld.noteLeafCount()==3,"one merge");require(lu.currentNoteRoot()==ld.currentNoteRoot(),"same final tree");
  for(uint256 i=1;i<=2;i++)require(lu.noteSpentIn(altS[i])==3&&ld.noteSpentIn(altS[i])==3,"spends");
  require(lu.producerOf(1,altS[3])==3&&ld.producerOf(1,altS[3])==3,"output");
  L.AuditRecord memory ru=lu.getAuditRecord(3);L.AuditRecord memory rd=ld.getAuditRecord(3);require(keccak256(abi.encode(ru))==keccak256(abi.encode(rd)),"same record");
  vm.expectRevert(L.AlreadySpent.selector);execute(lu,proof(false),altS);vm.expectRevert(L.AlreadySpent.selector);execute(ld,proof(true),altS);
 }
 function testInvalidSecondProofAtomic() public {
  L l=ledger(true);uint256 root=l.currentNoteRoot();bytes[2] memory p=altD;p[1]=pd[1];vm.expectRevert(L.InvalidProof.selector);execute(l,abi.encode(p),altS);unchanged(l,root,2,3);
 }
 function testInvalidFirstProofAtomic() public {
  L l=ledger(true);uint256 root=l.currentNoteRoot();bytes[2] memory p=altD;p[0]=pd[0];vm.expectRevert(L.InvalidProof.selector);execute(l,abi.encode(p),altS);unchanged(l,root,2,3);
 }
 function testRuntimeFrozenParity() public {for(uint256 i;i<2;i++){bool split=i==1;L l=ledger(split);uint256 root=l.currentNoteRoot();l.setStatus(1,altS[1],1);vm.expectRevert(L.InvalidState.selector);execute(l,proof(split),altS);unchanged(l,root,2,3);}}
 function testRuntimeRootAndDuplicateInputParity() public {for(uint256 i;i<2;i++){bool split=i==1;L l=ledger(split);uint256 root=l.currentNoteRoot();uint256[9] memory st=altS;st[0]=8;vm.expectRevert(L.InvalidRoot.selector);execute(l,proof(split),st);st=altS;st[2]=st[1];vm.expectRevert(L.InvalidShape.selector);execute(l,proof(split),st);unchanged(l,root,2,3);}}
 function testRuntimeFreshOutputParity() public {for(uint256 i;i<2;i++){bool split=i==1;L l=ledger(split);L.AuditCipher memory a;a.encryptedParents=new uint256[](0);a.encryptedOutputNfs=new uint256[](1);l.entry(hex"01",altS[3],a);uint256 next=l.nextAuditRecordId();vm.expectRevert(L.DuplicateObject.selector);execute(l,proof(split),altS);require(l.noteSpentIn(altS[1])==0&&l.noteLeafCount()==3&&l.nextAuditRecordId()==next,"fresh atomic");}}
}
