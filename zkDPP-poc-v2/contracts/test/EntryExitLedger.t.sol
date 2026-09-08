// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {EntryExitLedger} from "../src/EntryExitLedger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {EntryVerifier} from "../src/EntryVerifier.sol";
import {PrivateSpendVerifier} from "../src/PrivateSpendVerifier.sol";

interface Vm {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonUint(string calldata json, string calldata key) external pure returns (uint256);
    function parseJsonUintArray(string calldata json, string calldata key) external pure returns (uint256[] memory);
    function toString(uint256 value) external pure returns (string memory);
    function prank(address sender) external;
    function expectRevert(bytes4 selector) external;
}

contract EntryExitLedgerTest {
    Vm internal constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    address internal constant ISSUER_A = address(0x1001);
    address internal constant ISSUER_B = address(0x1002);
    address internal constant ISSUER_C = address(0x1003);
    address internal constant UNAUTHORIZED = address(0x1004);

    string internal fixture;
    EntryExitLedger internal ledger;

    function setUp() public {
        fixture = vm.readFile("test/fixtures/m2-proofs.json");
        Poseidon2BLS12381 poseidon = new Poseidon2BLS12381();
        ledger = new EntryExitLedger(
            address(new EntryVerifier()),
            address(new PrivateSpendVerifier()),
            address(new PrivateSpendVerifier()),
            address(new PrivateSpendVerifier()),
            address(new PrivateSpendVerifier()),
            address(new PrivateSpendVerifier()),
            address(new PrivateSpendVerifier()),
            address(poseidon)
        );
        ledger.setEntryIssuer(ISSUER_A, true);
        ledger.setEntryIssuer(ISSUER_B, true);
        ledger.setEntryIssuer(ISSUER_C, true);
    }

    function testCanonicalEntryExit() public {
        _entry(0, ISSUER_A);
        _entry(1, ISSUER_B);
        _entry(2, ISSUER_C);

        require(ledger.noteLeafCount() == 3, "leaf count");
        require(ledger.currentNoteRoot() == vm.parseJsonUint(fixture, ".tree.finalRoot"), "root");
        (uint256 pathRoot, uint256[] memory siblings) = ledger.getNotePath(0);
        uint256[] memory expected = vm.parseJsonUintArray(fixture, ".tree.exitSiblings");
        require(pathRoot == ledger.currentNoteRoot(), "path root");
        require(siblings.length == 32 && expected.length == 32, "path length");
        for (uint256 i = 0; i < 32; i++) require(siblings[i] == expected[i], "path sibling");

        uint256 rootBefore = ledger.currentNoteRoot();
        uint256 countBefore = ledger.noteLeafCount();
        ledger.exit(_proof(".exit.proof"), _uint(".exit.root"), _uint(".exit.nullifier"));
        require(ledger.noteNullifiers(_uint(".exit.nullifier")), "exit nf");
        require(ledger.currentNoteRoot() == rootBefore, "exit root changed");
        require(ledger.noteLeafCount() == countBefore, "exit count changed");
    }

    function testOnlyAdminUpdatesEntryIssuer() public {
        vm.prank(UNAUTHORIZED);
        vm.expectRevert(EntryExitLedger.NotAdmin.selector);
        ledger.setEntryIssuer(UNAUTHORIZED, true);

        vm.expectRevert(EntryExitLedger.InvalidAccount.selector);
        ledger.setEntryIssuer(address(0), true);
    }

    function testUnauthorizedAndRevokedEntryFail() public {
        vm.prank(UNAUTHORIZED);
        vm.expectRevert(EntryExitLedger.NotEntryIssuer.selector);
        ledger.entry(_entryProof(0), _entryCM(0));

        ledger.setEntryIssuer(ISSUER_A, false);
        vm.prank(ISSUER_A);
        vm.expectRevert(EntryExitLedger.NotEntryIssuer.selector);
        ledger.entry(_entryProof(0), _entryCM(0));
    }

    function testDuplicateCommitmentFailsBeforeProof() public {
        _entry(0, ISSUER_A);
        vm.prank(ISSUER_A);
        vm.expectRevert(EntryExitLedger.DuplicateCommitment.selector);
        ledger.entry(hex"deadbeef", _entryCM(0));
    }

    function testInvalidEntryProofIsAtomic() public {
        bytes memory proof = _entryProof(0);
        proof[0] = bytes1(uint8(proof[0]) ^ 1);
        uint256 rootBefore = ledger.currentNoteRoot();
        vm.prank(ISSUER_A);
        vm.expectRevert(EntryExitLedger.InvalidProof.selector);
        ledger.entry(proof, _entryCM(0));
        require(!ledger.commitments(_entryCM(0)), "commitment changed");
        require(ledger.currentNoteRoot() == rootBefore, "root changed");
        require(ledger.noteLeafCount() == 0, "count changed");
    }

    function testInvalidRootAndDuplicateNullifierFail() public {
        vm.expectRevert(EntryExitLedger.InvalidRoot.selector);
        ledger.exit(_proof(".exit.proof"), 999, _uint(".exit.nullifier"));

        _entry(0, ISSUER_A);
        _entry(1, ISSUER_B);
        _entry(2, ISSUER_C);
        uint256 rootBefore = ledger.currentNoteRoot();
        uint256 countBefore = ledger.noteLeafCount();
        ledger.exit(_proof(".exit.proof"), _uint(".exit.root"), _uint(".exit.nullifier"));
        vm.expectRevert(EntryExitLedger.SpentNullifier.selector);
        ledger.exit(hex"deadbeef", _uint(".exit.root"), _uint(".exit.nullifier"));
        require(ledger.currentNoteRoot() == rootBefore, "duplicate root changed");
        require(ledger.noteLeafCount() == countBefore, "duplicate count changed");
    }

    function testInvalidExitProofDoesNotSpend() public {
        _entry(0, ISSUER_A);
        _entry(1, ISSUER_B);
        _entry(2, ISSUER_C);
        bytes memory proof = _proof(".exit.proof");
        proof[0] = bytes1(uint8(proof[0]) ^ 1);
        uint256 nf = _uint(".exit.nullifier");
        vm.expectRevert(EntryExitLedger.InvalidProof.selector);
        ledger.exit(proof, _uint(".exit.root"), nf);
        require(!ledger.noteNullifiers(nf), "nf changed");
    }

    function _entry(uint256 index, address issuer) internal {
        vm.prank(issuer);
        ledger.entry(_entryProof(index), _entryCM(index));
    }

    function _entryProof(uint256 index) internal view returns (bytes memory) {
        return _proof(string.concat(".entries[", vm.toString(index), "].proof"));
    }

    function _entryCM(uint256 index) internal view returns (uint256) {
        return _uint(string.concat(".entries[", vm.toString(index), "].commitment"));
    }

    function _proof(string memory key) internal view returns (bytes memory) {
        return vm.parseJsonBytes(fixture, key);
    }

    function _uint(string memory key) internal view returns (uint256) {
        return vm.parseJsonUint(fixture, key);
    }
}
