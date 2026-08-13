// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;
import {SahaiLedger} from "./SahaiLedger.sol";

/// @notice Benchmark-only input seeding adapter; never deployed as the protocol contract.
contract BenchmarkSahaiLedger is SahaiLedger {
    uint256 public benchmarkStorage;
    constructor(uint8 width,address merkle,address eq,address add,address andVerifier) SahaiLedger(width,merkle,eq,add,andVerifier) {}
    function seedBenchmarkInput(AssetData calldata asset) external onlyAdministrator { bytes32[] memory empty = new bytes32[](0); _store(asset, empty); }
    function benchmarkNoop() external onlyAdministrator {}
    function benchmarkStorageWrite(uint256 value) external onlyAdministrator { benchmarkStorage = value; }
}
