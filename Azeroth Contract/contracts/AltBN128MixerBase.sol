// Copyright (c) 2015-2020 Clearmatics Technologies Ltd
// Copyright (c) 2021-2021 Zkrypto Inc.
// SPDX-License-Identifier: LGPL-3.0+

pragma solidity ^0.8.2;
pragma experimental ABIEncoderV2;

// import "./MixerBase.sol";
import "./zklayBase.sol";
import "./MiMC7.sol";

/// Partial implementation of abstract MixerBase which implements the
/// curve-specific methods to use the ALT-BN128 pairing.
abstract contract AltBN128MixerBase is ZklayBase {
    /// Constructor of the contract
    constructor(
        uint256 depth,
        uint256[] memory vk,
        uint256[] memory vkNft
    ) ZklayBase(depth, vk, vkNft) {}

    /// The Merkle tree hash functions.
    function _hash(bytes32 left, bytes32 right)
        internal
        pure
        override
        returns (bytes32)
    {
        return MiMC7._hash(left, right);
    }
}
