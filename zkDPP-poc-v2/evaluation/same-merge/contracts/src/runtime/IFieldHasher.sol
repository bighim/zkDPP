// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

interface IFieldHasher {
    function compress(uint256 left, uint256 right) external view returns (uint256);
}
