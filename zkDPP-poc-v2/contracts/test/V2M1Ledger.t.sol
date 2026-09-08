// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {ZkDPPV2Ledger as L} from "../src/ZkDPPV2Ledger.sol";

interface VmV2 {
    function prank(address) external;
    function expectRevert(bytes4) external;
    function roll(uint256) external;
}

contract V2MockHasher {
    uint256 constant FIELD = 52435875175126190479447740508185965837690552500527637822603658699938581184513;
    function compress(uint256 left, uint256 right) external pure returns (uint256) {
        return uint256(keccak256(abi.encode(left,right))) % FIELD;
    }
}

contract V2MockVerifier {
    bool public result = true;
    function setResult(bool value) external { result=value; }
    function Verify(bytes calldata, uint256[] calldata) external view returns(bool) { return result; }
}

contract V2M1LedgerTest {
    VmV2 constant vm=VmV2(address(uint160(uint256(keccak256("hevm cheat code")))));
    address constant ISSUER=address(0x1111);
    address constant STATUS=address(0x2222);
    L ledger;
    V2MockVerifier verifier;

    function setUp() public {
        verifier=new V2MockVerifier();
        address[7] memory fixedVerifiers;
        for(uint256 i;i<7;i++) fixedVerifiers[i]=address(verifier);
        ledger=new L(fixedVerifiers,address(new V2MockHasher()),STATUS);
        ledger.setEntryIssuer(ISSUER,true);
    }

    function cipher(uint256 parents,uint256 outputs) internal pure returns(L.AuditCipher memory a) {
        a.r1X=11; a.r1Y=12;
        a.encryptedParents=new uint256[](parents);
        a.encryptedOutputNfs=new uint256[](outputs);
        for(uint256 i;i<parents;i++) a.encryptedParents[i]=100+i;
        for(uint256 i;i<outputs;i++) a.encryptedOutputNfs[i]=200+i;
    }

    function entry(uint256 cm) internal returns(uint256) {
        vm.prank(ISSUER);
        return ledger.entry(hex"01",cm,cipher(0,1));
    }

    function testExitHasNoSuccessorAndIsAtomic() public {
        entry(101);
        uint256 root=ledger.currentNoteRoot();
        uint256 aid=ledger.exit(hex"02",root,501,cipher(1,0));
        L.AuditRecord memory record=ledger.getAuditRecord(aid);
        require(record.eventKind==7,"exit kind");
        require(record.outputRefs.length==0,"exit output");
        require(ledger.noteSpentIn(501)==aid,"spent relation");
        uint256 beforeID=ledger.nextAuditRecordId();
        vm.expectRevert(L.AlreadySpent.selector);
        ledger.exit(hex"02",root,501,cipher(1,0));
        require(ledger.nextAuditRecordId()==beforeID,"atomic id");
    }

    function testAbsoluteBlockDeadlineAndStatus() public {
        entry(102);
        uint256 root=ledger.currentNoteRoot();
        vm.roll(100);
        vm.expectRevert(L.InvalidDeadline.selector);
        ledger.transfer(hex"03",root,601,201,202,100,cipher(1,2));
        vm.prank(STATUS); ledger.setStatus(1,601,1);
        vm.expectRevert(L.InvalidState.selector);
        ledger.transfer(hex"03",root,601,201,202,101,cipher(1,2));
        vm.prank(STATUS); ledger.setStatus(1,601,0);
        ledger.transfer(hex"03",root,601,201,202,101,cipher(1,2));
    }

    function testIssueCreatesTerminalClaim() public {
        entry(103);
        uint256 root=ledger.currentNoteRoot();
        ledger.registerPolicyAuthority(address(this));
        uint256 ref=ledger.reservePolicy(8);
        ledger.registerPolicy(ref,1,1,bytes32(uint256(1)),address(verifier));
        uint256 aid=ledger.issue(hex"04",ref,root,701,9001,cipher(1,0));
        require(ledger.claimRegistered(9001),"claim missing");
        require(ledger.producerOf(3,9001)==aid,"claim producer");
        require(ledger.noteSpentIn(701)==aid,"note spend");
        L.AuditRecord memory record=ledger.getAuditRecord(aid);
        require(record.policyRef==ref&&record.outputRefs.length==1,"claim record");
        vm.expectRevert(L.AlreadySpent.selector);
        ledger.exit(hex"05",root,701,cipher(1,0));
    }

    function testInvalidProofChangesNothing() public {
        verifier.setResult(false);
        uint256 beforeID=ledger.nextAuditRecordId();
        vm.prank(ISSUER);
        vm.expectRevert(L.InvalidProof.selector);
        ledger.entry(hex"00",999,cipher(0,1));
        require(ledger.nextAuditRecordId()==beforeID&&!ledger.commitments(999),"partial write");
    }

    function testAllEventStateTransitions() public {
        entry(1001);
        uint256 root=ledger.currentNoteRoot();
        vm.roll(10);
        ledger.transfer(hex"10",root,1101,2001,1002,20,cipher(1,2));
        uint256 voucherRoot=ledger.currentVoucherRoot();
        ledger.proceed(hex"11",voucherRoot,2101,1003,cipher(1,1));

        root=ledger.currentNoteRoot();
        ledger.merge(hex"12",root,1102,1103,1004,cipher(2,1));
        root=ledger.currentNoteRoot();
        ledger.split(hex"13",root,1104,1005,1006,cipher(1,2));

        ledger.registerPolicyAuthority(address(this));
        uint256 processRef=ledger.reservePolicy(6);
        ledger.registerPolicy(processRef,3,2,bytes32(uint256(1)),address(verifier));
        ledger.setPolicyGrant(processRef,3001,true);
        root=ledger.currentNoteRoot();
        uint256[3] memory nfs=[uint256(1105),1106,1107];
        uint256[2] memory outputs=[uint256(1007),1008];
        ledger.process(hex"14",processRef,3001,root,nfs,outputs,cipher(3,2));

        entry(1009);
        root=ledger.currentNoteRoot();
        ledger.transfer(hex"15",root,1108,2002,1010,30,cipher(1,2));
        voucherRoot=ledger.currentVoucherRoot();
        vm.roll(30);
        ledger.recall(hex"16",voucherRoot,2102,1011,30,cipher(1,1));

        require(ledger.noteSpentIn(1101)!=0&&ledger.voucherSpentIn(2101)!=0,"spend maps");
        require(ledger.producerOf(2,2001)!=0&&ledger.producerOf(1,1011)!=0,"producer maps");
        require(ledger.nextAuditRecordId()==10,"all records");
    }
}
