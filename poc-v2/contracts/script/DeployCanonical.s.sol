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

interface DeployVm {
    function envUint(string calldata name) external view returns (uint256);
    function startBroadcast(uint256 privateKey) external;
    function stopBroadcast() external;
}

contract DeployCanonical {
    DeployVm private constant vm = DeployVm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function run() external {
        vm.startBroadcast(vm.envUint("ANVIL_DEPLOYER_PK"));
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
        new ZkDPP(verifierAddresses, address(poseidon));
        vm.stopBroadcast();
    }
}
