// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {ZkDPP} from "../src/ZkDPP.sol";
import {IZkVerifier} from "../src/IZkVerifier.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {PlonkVerifier as EntryVerifier} from "../src/generated/entry/PlonkVerifier.sol";
import {PlonkVerifier as TransferVerifier} from "../src/generated/transfer/PlonkVerifier.sol";
import {PlonkVerifier as ProceedVerifier} from "../src/generated/proceed/PlonkVerifier.sol";
import {PlonkVerifier as RecallVerifier} from "../src/generated/recall/PlonkVerifier.sol";
import {PlonkVerifier as MergeVerifier} from "../src/generated/merge/PlonkVerifier.sol";
import {PlonkVerifier as SplitVerifier} from "../src/generated/split/PlonkVerifier.sol";
import {PlonkVerifier as ProcessVerifier} from "../src/generated/process/PlonkVerifier.sol";
import {PlonkVerifier as ExitVerifier} from "../src/generated/exit/PlonkVerifier.sol";

interface Vm {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonUintArray(string calldata json, string calldata key) external pure returns (uint256[] memory);
    function toString(uint256 value) external pure returns (string memory);
    function expectRevert(bytes4 selector) external;
    function warp(uint256 timestamp) external;
}

contract MockVerifier is IZkVerifier {
    bool private immutable result;

    constructor(bool result_) {
        result = result_;
    }

    function Verify(bytes calldata, uint256[] calldata) external view returns (bool) {
        return result;
    }
}

contract ZkDPPTest {
    event log_named_uint(string key, uint256 value);

    Vm internal constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));
    string internal fixture;
    ZkDPP internal dpp;

    function setUp() public {
        fixture = vm.readFile("test/fixtures/canonical-proofs.json");
        Poseidon2BLS12381 poseidon = new Poseidon2BLS12381();
        address[8] memory verifierAddresses;
        verifierAddresses[0] = address(new EntryVerifier());
        verifierAddresses[1] = address(new TransferVerifier());
        verifierAddresses[2] = address(new ProceedVerifier());
        verifierAddresses[3] = address(new RecallVerifier());
        verifierAddresses[4] = address(new MergeVerifier());
        verifierAddresses[5] = address(new SplitVerifier());
        verifierAddresses[6] = address(new ProcessVerifier());
        verifierAddresses[7] = address(new ExitVerifier());
        dpp = new ZkDPP(verifierAddresses, address(poseidon));
        vm.warp(100 * 600);
    }

    function testCanonicalScenarioAllEightEvents() public {
        for (uint256 i = 0; i < 4; i++) _entry(i);
        _transfer(0);
        _recall(0);
        _transfer(1);
        _proceed(0);
        _transfer(2);
        _proceed(1);
        _process(0);
        _process(1);
        _transfer(3);
        _proceed(2);
        _transfer(4);
        _proceed(3);
        _merge(0);
        _transfer(5);
        _proceed(4);
        _transfer(6);
        _proceed(5);
        _process(2);
        _split(0);
        _exit(0);
        _exit(1);
        require(dpp.mtLeafCount() == 28, "MT count");
        require(dpp.rvLeafCount() == 7, "rvMT count");
        uint256[] memory processPI = _inputs("process", 2);
        for (uint256 i = 0; i < 3; i++) {
            require(dpp.nullifiers(processPI[16 + i * 3]), "process nf");
            require(dpp.commitments(processPI[23 + i]), "process output");
        }
        require(dpp.nullifiers(_inputs("exit", 0)[2]), "cell exit nf");
        require(dpp.nullifiers(_inputs("exit", 1)[2]), "waste exit nf");
    }

    function testRecallFailsAtDeadline() public {
        for (uint256 i = 0; i < 4; i++) _entry(i);
        _transfer(0);
        vm.warp(103 * 600);
        uint256[] memory p = _inputs("recall", 0);
        vm.expectRevert(ZkDPP.RecallDeadlineExpired.selector);
        dpp.recall(_proof("recall", 0), p[0], p[1], p[2], p[3]);
    }

    function testSequentialDuplicateOutputRevertsAtomically() public {
        Poseidon2BLS12381 poseidon = new Poseidon2BLS12381();
        MockVerifier verifier = new MockVerifier(true);
        address[8] memory addresses;
        for (uint256 i = 0; i < 8; i++) {
            addresses[i] = address(verifier);
        }
        ZkDPP target = new ZkDPP(addresses, address(poseidon));
        uint256[] memory delta = new uint256[](3);
        uint256[] memory allocation = new uint256[](6);
        uint256[] memory roots = new uint256[](1);
        roots[0] = target.currentMTRoot();
        uint256[] memory cmIn = _array(11);
        uint256[] memory nfIn = _array(21);
        uint256[] memory cmOut = new uint256[](2);
        cmOut[0] = 31;
        cmOut[1] = 31;
        vm.expectRevert(ZkDPP.DuplicateCommitment.selector);
        target.process(hex"01", delta, allocation, roots, cmIn, nfIn, cmOut);
        require(target.mtLeafCount() == 0, "atomic MT");
        require(!target.commitments(31), "atomic commitment");
        require(!target.nullifiers(21), "atomic nf");
    }

    function testPreconditionsRunBeforeProof() public {
        Poseidon2BLS12381 poseidon = new Poseidon2BLS12381();
        MockVerifier verifier = new MockVerifier(true);
        address[8] memory addresses;
        for (uint256 i = 0; i < 8; i++) {
            addresses[i] = address(verifier);
        }
        ZkDPP target = new ZkDPP(addresses, address(poseidon));
        target.entry(hex"", 99);
        vm.expectRevert(ZkDPP.DuplicateCommitment.selector);
        target.entry(hex"deadbeef", 99);
    }

    function _entry(uint256 index) internal {
        uint256[] memory p = _inputs("entry", index);
        dpp.entry(_proof("entry", index), p[0]);
    }

    function _merge(uint256 index) internal {
        uint256[] memory p = _inputs("merge", index);
        dpp.merge(_proof("merge", index), p[0], p[1], p[2], p[3], p[4], p[5], p[6]);
    }

    function _split(uint256 index) internal {
        uint256[] memory p = _inputs("split", index);
        dpp.split(_proof("split", index), p[0], p[1], p[2], p[3], p[4]);
    }

    function _transfer(uint256 index) internal {
        uint256[] memory p = _inputs("transfer", index);
        dpp.transfer(_proof("transfer", index), p[0], p[1], p[2], p[3], p[4], p[5]);
    }

    function _proceed(uint256 index) internal {
        uint256[] memory p = _inputs("proceed", index);
        dpp.proceed(_proof("proceed", index), p[0], p[1], p[2], p[3]);
    }

    function _recall(uint256 index) internal {
        uint256[] memory p = _inputs("recall", index);
        dpp.recall(_proof("recall", index), p[0], p[1], p[2], p[3]);
    }

    function _process(uint256 index) internal {
        uint256[] memory p = _inputs("process", index);
        uint256 m = p[0];
        uint256 n = p[1];
        uint256[] memory delta = new uint256[](3);
        for (uint256 k = 0; k < 3; k++) {
            delta[k] = p[2 + k];
        }
        uint256[] memory allocation = new uint256[](n * 3);
        for (uint256 i = 0; i < n * 3; i++) {
            allocation[i] = p[5 + i];
        }
        uint256[] memory roots = new uint256[](m);
        uint256[] memory cmIn = new uint256[](m);
        uint256[] memory nfIn = new uint256[](m);
        uint256[] memory cmOut = new uint256[](n);
        for (uint256 i = 0; i < m; i++) {
            roots[i] = p[14 + i * 3];
            cmIn[i] = p[15 + i * 3];
            nfIn[i] = p[16 + i * 3];
        }
        for (uint256 i = 0; i < n; i++) {
            cmOut[i] = p[23 + i];
        }
        dpp.process(_proof("process", index), delta, allocation, roots, cmIn, nfIn, cmOut);
    }

    function _exit(uint256 index) internal {
        uint256[] memory p = _inputs("exit", index);
        dpp.exit(_proof("exit", index), p[0], p[1], p[2]);
    }

    function _proof(string memory relation, uint256 index) internal view returns (bytes memory) {
        return vm.parseJsonBytes(fixture, string.concat(".relations.", relation, "[", vm.toString(index), "].proof"));
    }

    function _inputs(string memory relation, uint256 index) internal view returns (uint256[] memory) {
        return vm.parseJsonUintArray(
            fixture, string.concat(".relations.", relation, "[", vm.toString(index), "].publicInputs")
        );
    }

    function _array(uint256 value) internal pure returns (uint256[] memory result) {
        result = new uint256[](1);
        result[0] = value;
    }
}
