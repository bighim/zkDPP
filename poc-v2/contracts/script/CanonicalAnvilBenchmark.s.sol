// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {ZkDPP} from "../src/ZkDPP.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {PlonkVerifier as EntryVerifier} from "../src/generated/entry/PlonkVerifier.sol";
import {PlonkVerifier as TransferVerifier} from "../src/generated/transfer/PlonkVerifier.sol";
import {PlonkVerifier as ProceedVerifier} from "../src/generated/proceed/PlonkVerifier.sol";
import {PlonkVerifier as RecallVerifier} from "../src/generated/recall/PlonkVerifier.sol";
import {PlonkVerifier as MergeVerifier} from "../src/generated/merge/PlonkVerifier.sol";
import {PlonkVerifier as SplitVerifier} from "../src/generated/split/PlonkVerifier.sol";
import {PlonkVerifier as ProcessVerifier} from "../src/generated/process/PlonkVerifier.sol";
import {PlonkVerifier as ExitVerifier} from "../src/generated/exit/PlonkVerifier.sol";

interface ScriptVm {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonUintArray(string calldata json, string calldata key) external pure returns (uint256[] memory);
    function envUint(string calldata name) external view returns (uint256);
    function toString(uint256 value) external pure returns (string memory);
    function startBroadcast(uint256 privateKey) external;
    function stopBroadcast() external;
}

contract CanonicalAnvilBenchmark {
    ScriptVm private constant vm = ScriptVm(address(uint160(uint256(keccak256("hevm cheat code")))));

    string private fixture;
    ZkDPP private dpp;
    uint256 private aluminumSupplierKey;
    uint256 private cathodeSupplierKey;
    uint256 private anodeSupplierKey;
    uint256 private foilManufacturerKey;
    uint256 private cellManufacturerKey;

    function run() external {
        fixture = vm.readFile("test/fixtures/canonical-proofs.json");
        uint256 deployerKey = vm.envUint("ANVIL_DEPLOYER_PK");
        aluminumSupplierKey = vm.envUint("ANVIL_ALUMINUM_SUPPLIER_PK");
        cathodeSupplierKey = vm.envUint("ANVIL_CATHODE_SUPPLIER_PK");
        anodeSupplierKey = vm.envUint("ANVIL_ANODE_SUPPLIER_PK");
        foilManufacturerKey = vm.envUint("ANVIL_FOIL_MANUFACTURER_PK");
        cellManufacturerKey = vm.envUint("ANVIL_CELL_MANUFACTURER_PK");

        vm.startBroadcast(deployerKey);
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
        vm.stopBroadcast();

        _as(aluminumSupplierKey);
        _entry(0);
        _entry(1);
        vm.stopBroadcast();
        _as(cathodeSupplierKey);
        _entry(2);
        vm.stopBroadcast();
        _as(anodeSupplierKey);
        _entry(3);
        vm.stopBroadcast();

        _as(aluminumSupplierKey);
        _transfer(0);
        _recall(0);
        _transfer(1);
        vm.stopBroadcast();
        _as(foilManufacturerKey);
        _proceed(0);
        vm.stopBroadcast();
        _as(aluminumSupplierKey);
        _transfer(2);
        vm.stopBroadcast();
        _as(foilManufacturerKey);
        _proceed(1);
        _process(0);
        _process(1);
        _transfer(3);
        vm.stopBroadcast();

        _as(cellManufacturerKey);
        _proceed(2);
        vm.stopBroadcast();
        _as(foilManufacturerKey);
        _transfer(4);
        vm.stopBroadcast();
        _as(cellManufacturerKey);
        _proceed(3);
        _merge(0);
        vm.stopBroadcast();

        _as(cathodeSupplierKey);
        _transfer(5);
        vm.stopBroadcast();
        _as(cellManufacturerKey);
        _proceed(4);
        vm.stopBroadcast();
        _as(anodeSupplierKey);
        _transfer(6);
        vm.stopBroadcast();
        _as(cellManufacturerKey);
        _proceed(5);
        _process(2);
        _split(0);
        _exit(0);
        _exit(1);
        vm.stopBroadcast();

        require(dpp.mtLeafCount() == 28, "MT count");
        require(dpp.rvLeafCount() == 7, "rvMT count");
    }

    function _as(uint256 privateKey) private { vm.startBroadcast(privateKey); }
    function _entry(uint256 index) private { uint256[] memory p = _inputs("entry", index); dpp.entry(_proof("entry", index), p[0]); }
    function _transfer(uint256 index) private { uint256[] memory p = _inputs("transfer", index); dpp.transfer(_proof("transfer", index), p[0], p[1], p[2], p[3], p[4], p[5]); }
    function _proceed(uint256 index) private { uint256[] memory p = _inputs("proceed", index); dpp.proceed(_proof("proceed", index), p[0], p[1], p[2], p[3]); }
    function _recall(uint256 index) private { uint256[] memory p = _inputs("recall", index); dpp.recall(_proof("recall", index), p[0], p[1], p[2], p[3]); }
    function _merge(uint256 index) private { uint256[] memory p = _inputs("merge", index); dpp.merge(_proof("merge", index), p[0], p[1], p[2], p[3], p[4], p[5], p[6]); }
    function _split(uint256 index) private { uint256[] memory p = _inputs("split", index); dpp.split(_proof("split", index), p[0], p[1], p[2], p[3], p[4]); }
    function _exit(uint256 index) private { uint256[] memory p = _inputs("exit", index); dpp.exit(_proof("exit", index), p[0], p[1], p[2]); }

    function _process(uint256 index) private {
        uint256[] memory p = _inputs("process", index);
        uint256 m = p[0];
        uint256 n = p[1];
        uint256[] memory delta = new uint256[](3);
        uint256[] memory allocation = new uint256[](n * 3);
        uint256[] memory roots = new uint256[](m);
        uint256[] memory cmIn = new uint256[](m);
        uint256[] memory nfIn = new uint256[](m);
        uint256[] memory cmOut = new uint256[](n);
        for (uint256 k = 0; k < 3; k++) delta[k] = p[2 + k];
        for (uint256 i = 0; i < n * 3; i++) allocation[i] = p[5 + i];
        for (uint256 i = 0; i < m; i++) {
            roots[i] = p[14 + i * 3];
            cmIn[i] = p[15 + i * 3];
            nfIn[i] = p[16 + i * 3];
        }
        for (uint256 i = 0; i < n; i++) cmOut[i] = p[23 + i];
        dpp.process(_proof("process", index), delta, allocation, roots, cmIn, nfIn, cmOut);
    }

    function _proof(string memory relation, uint256 index) private view returns (bytes memory) {
        return vm.parseJsonBytes(fixture, string.concat(".relations.", relation, "[", vm.toString(index), "].proof"));
    }
    function _inputs(string memory relation, uint256 index) private view returns (uint256[] memory) {
        return vm.parseJsonUintArray(fixture, string.concat(".relations.", relation, "[", vm.toString(index), "].publicInputs"));
    }
}
