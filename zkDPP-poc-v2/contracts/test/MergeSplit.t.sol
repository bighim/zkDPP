// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {EntryExitLedger} from "../src/EntryExitLedger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {EntryVerifier} from "../src/EntryVerifier.sol";
import {PrivateSpendVerifier} from "../src/PrivateSpendVerifier.sol";
import {TransferVerifier} from "../src/TransferVerifier.sol";
import {ProceedVerifier} from "../src/ProceedVerifier.sol";
import {RecallVerifier} from "../src/RecallVerifier.sol";
import {MergeVerifier} from "../src/MergeVerifier.sol";
import {SplitVerifier} from "../src/SplitVerifier.sol";
interface VmM4{function readFile(string calldata)external view returns(string memory);function parseJsonBytes(string calldata,string calldata)external pure returns(bytes memory);function parseJsonUint(string calldata,string calldata)external pure returns(uint256);function prank(address)external;function expectRevert(bytes4)external;}
contract MergeSplitTest{
    VmM4 constant vm=VmM4(address(uint160(uint256(keccak256("hevm cheat code")))));address constant ISSUER=address(0x1001);string fixture;EntryExitLedger ledger;Poseidon2BLS12381 poseidon;
    function setUp()public{fixture=vm.readFile("test/fixtures/m4-proofs.json");poseidon=new Poseidon2BLS12381();ledger=new EntryExitLedger(address(new EntryVerifier()),address(new PrivateSpendVerifier()),address(new TransferVerifier()),address(new ProceedVerifier()),address(new RecallVerifier()),address(new MergeVerifier()),address(new SplitVerifier()),address(poseidon));ledger.setEntryIssuer(ISSUER,true);}
    function testCanonicalMergeSplit()public{_entries();ledger.merge(_proof(".Merge.proof"),_u(".Merge.NoteRoot"),_u(".Merge.NF1"),_u(".Merge.NF2"),_u(".Merge.CMOut"));ledger.split(_proof(".Split.proof"),_u(".Split.NoteRoot"),_u(".Split.NF"),_u(".Split.CMOut1"),_u(".Split.CMOut2"));require(ledger.noteLeafCount()==_u(".FinalCount"),"count");require(ledger.currentNoteRoot()==_u(".FinalRoot"),"root");require(ledger.noteNullifiers(_u(".Merge.NF1"))&&ledger.noteNullifiers(_u(".Merge.NF2"))&&ledger.noteNullifiers(_u(".Split.NF")),"nf");(uint256 root,uint256[] memory path)=ledger.getNotePath(4);require(_root(_u(".Split.CMOut2"),4,path)==root,"path");}
    function testDuplicateInputAndInvalidProofAreAtomic()public{_entries();uint256 root=ledger.currentNoteRoot();vm.expectRevert(EntryExitLedger.DuplicateInput.selector);ledger.merge(hex"deadbeef",_u(".Merge.NoteRoot"),_u(".Merge.NF1"),_u(".Merge.NF1"),_u(".Merge.CMOut"));bytes memory bad=_proof(".Merge.proof");bad[0]=bytes1(uint8(bad[0])^1);vm.expectRevert(EntryExitLedger.InvalidProof.selector);ledger.merge(bad,_u(".Merge.NoteRoot"),_u(".Merge.NF1"),_u(".Merge.NF2"),_u(".Merge.CMOut"));require(ledger.currentNoteRoot()==root&&ledger.noteLeafCount()==2,"mutated");}
    function testSplitReplayFails()public{_entries();ledger.merge(_proof(".Merge.proof"),_u(".Merge.NoteRoot"),_u(".Merge.NF1"),_u(".Merge.NF2"),_u(".Merge.CMOut"));ledger.split(_proof(".Split.proof"),_u(".Split.NoteRoot"),_u(".Split.NF"),_u(".Split.CMOut1"),_u(".Split.CMOut2"));vm.expectRevert(EntryExitLedger.SpentNullifier.selector);ledger.split(hex"deadbeef",_u(".Split.NoteRoot"),_u(".Split.NF"),999,1000);}
    function _entries()internal{for(uint256 i=0;i<2;i++){vm.prank(ISSUER);ledger.entry(_proof(string.concat(".Entries[",_s(i),"].proof")),_u(string.concat(".Entries[",_s(i),"].Commitment")));}}
    function _root(uint256 leaf,uint256 index,uint256[] memory siblings)internal view returns(uint256 current){current=leaf;for(uint256 i=0;i<siblings.length;i++){current=(index&1)==0?poseidon.compress(current,siblings[i]):poseidon.compress(siblings[i],current);index>>=1;}}
    function _proof(string memory k)internal view returns(bytes memory){return vm.parseJsonBytes(fixture,k);}function _u(string memory k)internal view returns(uint256){return vm.parseJsonUint(fixture,k);}function _s(uint256 v)internal pure returns(string memory){if(v==0)return"0";return"1";}
}
