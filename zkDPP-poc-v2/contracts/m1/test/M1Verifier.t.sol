// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {PlonkVerifier} from "../src/generated/v2-m1/entry/PlonkVerifier.sol";

interface VmVerifier {
    function readFile(string calldata) external view returns(string memory);
    function parseJsonBytes(string calldata,string calldata) external pure returns(bytes memory);
    function parseJsonUintArray(string calldata,string calldata) external pure returns(uint256[] memory);
}

contract M1RealVerifierTest {
    VmVerifier constant vm=VmVerifier(address(uint160(uint256(keccak256("hevm cheat code")))));
    function testFixedEntryProof() public {
        string memory fixture=vm.readFile("test/fixtures/v2-m1-proofs.json");
        bytes memory proof=vm.parseJsonBytes(fixture,".Proofs[0].proof");
        uint256[] memory inputs=vm.parseJsonUintArray(fixture,".Proofs[0].publicInputs");
        require(new PlonkVerifier().Verify(proof,inputs),"entry proof");
    }
}
