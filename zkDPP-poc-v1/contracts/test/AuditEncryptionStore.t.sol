// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {AuditEncryptionStore} from "../src/AuditEncryptionStore.sol";
import {AuditProcessVerifier} from "../src/AuditProcessVerifier.sol";
import {IZkVerifier} from "../src/IZkVerifier.sol";

interface VmB1 {
    function readFile(string calldata) external view returns (string memory);
    function parseJsonBytes(string calldata, string calldata) external pure returns (bytes memory);
    function parseJsonUintArray(string calldata, string calldata) external pure returns (uint256[] memory);
    function expectRevert(bytes4) external;
}

contract FalseAuditVerifier is IZkVerifier {
    function Verify(bytes calldata, uint256[] calldata) external pure returns (bool) { return false; }
}
contract RevertingAuditVerifier is IZkVerifier {
    function Verify(bytes calldata, uint256[] calldata) external pure returns (bool) { revert("fixture revert"); }
}

contract AuditEncryptionStoreTest {
    VmB1 private constant vm = VmB1(address(uint160(uint256(keccak256("hevm cheat code")))));
    AuditEncryptionStore private store;
    bytes private proof;
    uint256[15] private inputs;

    function setUp() public {
        string memory fixture = vm.readFile("test/fixtures/m6-b1-proofs.json");
        proof = vm.parseJsonBytes(fixture, ".proof");
        uint256[] memory values = vm.parseJsonUintArray(fixture, ".publicInputs");
        require(values.length == 15, "fixture arity");
        for (uint256 i; i < 15; ++i) inputs[i] = values[i];
        store = new AuditEncryptionStore(address(new AuditProcessVerifier()));
    }

    function testVerifyOnlyDoesNotStore() public {
        require(store.verifyOnly(proof, inputs), "proof");
        require(store.nextRecordId() == 1, "verify mutated ID");
        vm.expectRevert(AuditEncryptionStore.UnknownRecord.selector);
        store.getRecord(1);
    }
    function testStoreExactPublicInputAndDiagnosticReplay() public {
        uint256 id = store.verifyAndStore(proof, inputs);
        require(id == 1 && store.nextRecordId() == 2, "ID");
        uint256[15] memory actual = store.getRecord(id);
        for (uint256 i; i < 15; ++i) require(actual[i] == inputs[i], "record mismatch");
        // This is deliberately not a spending ledger.
        require(store.verifyAndStore(proof, inputs) == 2, "diagnostic replay");
        actual = store.getRecord(1);
        for (uint256 i; i < 15; ++i) require(actual[i] == inputs[i], "old record changed");
    }
    function testRejectInvalidProofWithoutPartialWrite() public {
        vm.expectRevert(AuditEncryptionStore.InvalidProof.selector);
        store.verifyAndStore(hex"00", inputs);
        require(store.nextRecordId() == 1, "partial ID update");
        vm.expectRevert(AuditEncryptionStore.UnknownRecord.selector);
        store.getRecord(1);
    }
    function testRejectChangedCiphertextAndPoint() public {
        uint256[15] memory changed = inputs;
        changed[10] = changed[10] + 1;
        vm.expectRevert(AuditEncryptionStore.InvalidProof.selector);
        store.verifyAndStore(proof, changed);
        changed = inputs;
        changed[8] = 0; changed[9] = 1;
        vm.expectRevert(AuditEncryptionStore.InvalidProof.selector);
        store.verifyAndStore(proof, changed);
        require(store.nextRecordId() == 1, "partial write");
    }
    function testRejectFieldOverflowAndUnknownRecord() public {
        uint256[15] memory changed = inputs;
        changed[0] = 52435875175126190479447740508185965837690552500527637822603658699938581184513;
        vm.expectRevert(AuditEncryptionStore.InvalidField.selector);
        store.verifyAndStore(proof, changed);
        vm.expectRevert(AuditEncryptionStore.UnknownRecord.selector);
        store.getRecord(0);
        vm.expectRevert(AuditEncryptionStore.UnknownRecord.selector);
        store.getRecord(42);
    }
    function testVerifierAddressValidation() public {
        vm.expectRevert(AuditEncryptionStore.InvalidVerifier.selector);
        new AuditEncryptionStore(address(0));
        vm.expectRevert(AuditEncryptionStore.InvalidVerifier.selector);
        new AuditEncryptionStore(address(0x1234));
    }
    function testFalseAndRevertAreNormalized() public {
        AuditEncryptionStore negative = new AuditEncryptionStore(address(new FalseAuditVerifier()));
        vm.expectRevert(AuditEncryptionStore.InvalidProof.selector);
        negative.verifyAndStore(proof, inputs);
        require(negative.nextRecordId() == 1, "false wrote");
        negative = new AuditEncryptionStore(address(new RevertingAuditVerifier()));
        vm.expectRevert(AuditEncryptionStore.InvalidProof.selector);
        negative.verifyAndStore(proof, inputs);
        require(negative.nextRecordId() == 1, "revert wrote");
    }
}
