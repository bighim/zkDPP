// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {EntryExitLedger} from "../src/EntryExitLedger.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {EntryVerifier} from "../src/EntryVerifier.sol";
import {PrivateSpendVerifier} from "../src/PrivateSpendVerifier.sol";
import {TransferVerifier} from "../src/TransferVerifier.sol";
import {ProceedVerifier} from "../src/ProceedVerifier.sol";
import {RecallVerifier} from "../src/RecallVerifier.sol";

interface VmM3 {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonUint(string calldata json, string calldata key) external pure returns (uint256);
    function prank(address sender) external;
    function expectRevert(bytes4 selector) external;
    function warp(uint256 timestamp) external;
}

contract TransferVoucherTest {
    VmM3 internal constant vm = VmM3(address(uint160(uint256(keccak256("hevm cheat code")))));
    address internal constant ISSUER = address(0x1001);
    string internal fixture;
    EntryExitLedger internal ledger;
    Poseidon2BLS12381 internal poseidon;

    function setUp() public {
        fixture = vm.readFile("test/fixtures/m3-proofs.json");
        poseidon = new Poseidon2BLS12381();
        ledger = new EntryExitLedger(
            address(new EntryVerifier()),
            address(new PrivateSpendVerifier()),
            address(new TransferVerifier()),
            address(new ProceedVerifier()),
            address(new RecallVerifier()),
            address(new RecallVerifier()),
            address(new RecallVerifier()),
            address(poseidon)
        );
        ledger.setEntryIssuer(ISSUER, true);
        vm.warp(60000);
    }

    function testCanonicalTransferProceedRecall() public {
        _entries();
        _transfer(".partialTransfer");
        _proceed();
        _transfer(".fullTransfer");
        vm.warp(105 * 600);
        _recall();

        require(ledger.noteLeafCount() == _uint(".tree.finalNoteCount"), "note count");
        require(ledger.voucherLeafCount() == _uint(".tree.finalVoucherCount"), "voucher count");
        require(ledger.currentNoteRoot() == _uint(".tree.finalNoteRoot"), "note root");
        require(ledger.currentVoucherRoot() == _uint(".tree.finalVoucherRoot"), "voucher root");
        require(ledger.noteNullifiers(_uint(".partialTransfer.nullifier")), "partial nf");
        require(ledger.noteNullifiers(_uint(".fullTransfer.nullifier")), "full nf");
        require(ledger.voucherNullifiers(_uint(".proceed.voucherNullifier")), "proceed rvnf");
        require(ledger.voucherNullifiers(_uint(".recall.voucherNullifier")), "recall rvnf");

        (uint256 noteRoot, uint256[] memory notePath) = ledger.getNotePath(1);
        require(_root(_uint(".entries[1].commitment"), 1, notePath) == noteRoot, "note path");
        (uint256 voucherRoot, uint256[] memory voucherPath) = ledger.getVoucherPath(1);
        require(_root(_uint(".fullTransfer.voucherCommitment"), 1, voucherPath) == voucherRoot, "voucher path");
    }

    function testProceedAfterDeadline() public {
        _entries();
        _transfer(".partialTransfer");
        vm.warp(107 * 600);
        _proceed();
        require(ledger.voucherNullifiers(_uint(".proceed.voucherNullifier")), "not resolved");
    }

    function testResolutionIsSingleUse() public {
        _entries();
        _transfer(".partialTransfer");
        _proceed();
        vm.warp(105 * 600);
        vm.expectRevert(EntryExitLedger.ResolvedVoucher.selector);
        ledger.recall(hex"deadbeef", _uint(".proceed.voucherRoot"), _uint(".proceed.voucherNullifier"), 999, 105);
    }

    function testEpochAndInvalidProofAreAtomic() public {
        _entries();
        uint256 noteRoot = ledger.currentNoteRoot();
        uint256 voucherRoot = ledger.currentVoucherRoot();
        vm.expectRevert(EntryExitLedger.InvalidEpoch.selector);
        ledger.transfer(
            _proof(".partialTransfer.proof"), _uint(".partialTransfer.noteRoot"),
            _uint(".partialTransfer.nullifier"), _uint(".partialTransfer.voucherCommitment"),
            _uint(".partialTransfer.changeCommitment"), 99, _uint(".partialTransfer.deltaEpoch")
        );
        require(ledger.currentNoteRoot() == noteRoot && ledger.currentVoucherRoot() == voucherRoot, "epoch mutated");

        bytes memory bad = _proof(".partialTransfer.proof");
        bad[0] = bytes1(uint8(bad[0]) ^ 1);
        vm.expectRevert(EntryExitLedger.InvalidProof.selector);
        ledger.transfer(
            bad, _uint(".partialTransfer.noteRoot"), _uint(".partialTransfer.nullifier"),
            _uint(".partialTransfer.voucherCommitment"), _uint(".partialTransfer.changeCommitment"),
            _uint(".partialTransfer.transferEpoch"), _uint(".partialTransfer.deltaEpoch")
        );
        require(ledger.currentNoteRoot() == noteRoot && ledger.currentVoucherRoot() == voucherRoot, "proof mutated");
        require(ledger.noteLeafCount() == 2 && ledger.voucherLeafCount() == 0, "count mutated");
    }

    function _entries() internal {
        vm.prank(ISSUER);
        ledger.entry(_proof(".entries[0].proof"), _uint(".entries[0].commitment"));
        vm.prank(ISSUER);
        ledger.entry(_proof(".entries[1].proof"), _uint(".entries[1].commitment"));
    }

    function _transfer(string memory base) internal {
        ledger.transfer(
            _proof(string.concat(base, ".proof")), _uint(string.concat(base, ".noteRoot")),
            _uint(string.concat(base, ".nullifier")), _uint(string.concat(base, ".voucherCommitment")),
            _uint(string.concat(base, ".changeCommitment")), _uint(string.concat(base, ".transferEpoch")),
            _uint(string.concat(base, ".deltaEpoch"))
        );
    }

    function _proceed() internal {
        ledger.proceed(
            _proof(".proceed.proof"), _uint(".proceed.voucherRoot"),
            _uint(".proceed.voucherNullifier"), _uint(".proceed.outputCommitment")
        );
    }

    function _recall() internal {
        ledger.recall(
            _proof(".recall.proof"), _uint(".recall.voucherRoot"),
            _uint(".recall.voucherNullifier"), _uint(".recall.outputCommitment"),
            _uint(".recall.currentEpoch")
        );
    }

    function _root(uint256 leaf, uint256 index, uint256[] memory siblings) internal view returns (uint256 current) {
        current = leaf;
        for (uint256 level = 0; level < siblings.length; level++) {
            current = index & 1 == 0
                ? poseidon.compress(current, siblings[level])
                : poseidon.compress(siblings[level], current);
            index >>= 1;
        }
    }

    function _proof(string memory key) internal view returns (bytes memory) { return vm.parseJsonBytes(fixture, key); }
    function _uint(string memory key) internal view returns (uint256) { return vm.parseJsonUint(fixture, key); }
}
