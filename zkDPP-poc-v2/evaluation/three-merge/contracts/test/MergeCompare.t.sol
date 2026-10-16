// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;
import {MergeProofRouter} from "../src/MergeProofRouter.sol";
import {PlonkVerifier as VP} from "../src/generated/path/PlonkVerifier.sol";
import {PlonkVerifier as VE} from "../src/generated/eq/PlonkVerifier.sol";
import {PlonkVerifier as VA} from "../src/generated/add/PlonkVerifier.sol";
import {PlonkVerifier as VB} from "../src/generated/B/PlonkVerifier.sol";
import {PlonkVerifier as VC} from "../src/generated/C/PlonkVerifier.sol";
interface Vm {
 function readFile(string calldata) external returns(string memory);
 function parseJsonStringArray(string calldata,string calldata) external pure returns(string[] memory);
 function parseJsonBytes(string calldata,string calldata) external pure returns(bytes memory);
 function parseUint(string calldata) external pure returns(uint256);
 function parseBytes(string calldata) external pure returns(bytes memory);
 function toString(uint256) external pure returns(string memory);
 function writeJson(string calldata,string calldata) external;
 function projectRoot() external returns(string memory);
}
contract MergeCompareTest {
 Vm constant vm=Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
 event log_named_uint(string key,uint256 val);
 MergeProofRouter router;
 bytes[6] proofsA; bytes[6] alternateA;bytes proofB;bytes proofC;
 uint256[15] statement; uint256[15] alternateStatement;uint256[9] currentStatement;
 function setUp() public {
  router=new MergeProofRouter(address(new VP()),address(new VE()),address(new VA()),address(new VB()),address(new VC()));
  string memory data=vm.readFile(string.concat(vm.projectRoot(),"/test/fixtures/proofs.json"));
  string[] memory a=vm.parseJsonStringArray(data,".A");string[] memory alt=vm.parseJsonStringArray(data,".altA");
  for(uint256 i=0;i<6;i++){proofsA[i]=vm.parseBytes(a[i]);alternateA[i]=vm.parseBytes(alt[i]);}
  proofB=vm.parseJsonBytes(data,".B");proofC=vm.parseJsonBytes(data,".C");
  string[] memory s=vm.parseJsonStringArray(data,".canonical");string[] memory as_=vm.parseJsonStringArray(data,".altCanonical");
  for(uint256 i=0;i<15;i++){statement[i]=vm.parseUint(s[i]);alternateStatement[i]=vm.parseUint(as_[i]);}
  string[] memory cs=vm.parseJsonStringArray(data,".publicC");for(uint256 i=0;i<9;i++)currentStatement[i]=vm.parseUint(cs[i]);
 }
 function acceptsA(bytes[6] memory p,uint256[15] memory s) internal view returns(bool){
  try router.verifyA(p,s) returns(bool b){return b;}catch{return false;}
 }
 function acceptsB(bytes memory p,uint256[15] memory s) internal view returns(bool){
  try router.verifyB(p,s) returns(bool b){return b;}catch{return false;}
 }
 function acceptsC(bytes memory p,uint256[9] memory s) internal view returns(bool){
  try router.verifyC(p,s) returns(bool b){return b;}catch{return false;}
 }
 function testValidAllAndAlternative() public view {
  require(router.verifyA(proofsA,statement),"A valid");require(router.verifyB(proofB,statement),"B valid");
  require(router.verifyC(proofC,currentStatement),"C valid");require(router.verifyA(alternateA,alternateStatement),"alt valid");
 }
 function testAtomicMixEverySlot() public view {
  for(uint256 i=0;i<6;i++){bytes[6] memory p=proofsA;p[i]=alternateA[i];require(!acceptsA(p,statement),"mixed accepted");}
 }
 function testWrongPublicAB() public view {
  for(uint256 i=0;i<15;i++){uint256[15] memory s=statement;s[i]=8;require(!acceptsA(proofsA,s),"A mismatch");require(!acceptsB(proofB,s),"B mismatch");}
 }
 function testWrongPublicCAll() public view {
  for(uint256 i=0;i<9;i++){uint256[9] memory s=currentStatement;s[i]=8;require(!acceptsC(proofC,s),"C mismatch");}
 }
 function testVerifierWithoutCode() public {
  try new MergeProofRouter(address(0),address(1),address(2),address(3),address(4)) {revert("no-code accepted");} catch {}
 }
 // Each test starts from a fresh state: no preceding verifier call warms the
 // same verifier address. gasleft delta excludes test/setup and intrinsic tx gas.
 function testGasA() public {
  bytes[6] memory p=proofsA;uint256[15] memory s=statement;
  uint256 st=gasleft();bool ok=router.verifyA(p,s);uint256 gasUsed=st-gasleft();require(ok);
  uint256 abiBytes=abi.encodeCall(router.verifyA,(proofsA,statement)).length;
  emit log_named_uint("A-verification-wrapper-gas",gasUsed);emit log_named_uint("A-canonical-ABI-bytes",abiBytes);
  save("A",gasUsed,abiBytes);
 }
 function testGasB() public {
  bytes memory p=proofB;uint256[15] memory s=statement;
  uint256 st=gasleft();bool ok=router.verifyB(p,s);uint256 gasUsed=st-gasleft();require(ok);
  uint256 abiBytes=abi.encodeCall(router.verifyB,(proofB,statement)).length;
  emit log_named_uint("B-verification-wrapper-gas",gasUsed);emit log_named_uint("B-canonical-ABI-bytes",abiBytes);
  save("B",gasUsed,abiBytes);
 }
 function testGasC() public {
  bytes memory p=proofC;uint256[9] memory s=currentStatement;
  uint256 st=gasleft();bool ok=router.verifyC(p,s);uint256 gasUsed=st-gasleft();require(ok);
  uint256 abiBytes=abi.encodeCall(router.verifyC,(proofC,currentStatement)).length;
  emit log_named_uint("C-verification-wrapper-gas",gasUsed);emit log_named_uint("C-canonical-ABI-bytes",abiBytes);
  save("C",gasUsed,abiBytes);
 }
 function save(string memory arm,uint256 g,uint256 b) internal {
  string memory data=string.concat('{"arm":"',arm,'","verificationWrapperGas":',vm.toString(g),',"canonicalABIBytes":',vm.toString(b),'}');
  vm.writeJson(data,string.concat(vm.projectRoot(),"/test/fixtures/gas-",arm,".json"));
 }
}

