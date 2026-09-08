// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

interface IZkVerifier {
    function Verify(bytes calldata proof, uint256[] calldata publicInputs) external view returns (bool);
}
