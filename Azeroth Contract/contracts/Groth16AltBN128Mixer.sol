// Copyright (c) 2015-2020 Clearmatics Technologies Ltd
// Copyright (c) 2021-2021 Zkrypto Inc.
// SPDX-License-Identifier: LGPL-3.0+

pragma solidity ^0.8.2;
pragma experimental ABIEncoderV2;

import "./AltBN128MixerBase.sol";
import "./Groth16AltBN128.sol";

/// Instance of AltBN128MixerBase implementing the Groth16 verifier for the
/// alt-bn128 pairing.
contract Groth16AltBN128Mixer is AltBN128MixerBase {
    constructor(
        uint256 depth,
        uint256[] memory vk,
        uint256[] memory vkNft
    ) public AltBN128MixerBase(depth, vk, vkNft) {}

    function _verifyZKProof(
        uint256[] memory proof,
        uint256[] memory inputs,
        bool isNft
    ) internal override returns (bool) {
        if (isNft) {
            return Groth16AltBN128._verify(_vkNft, proof, inputs);
        }
        return Groth16AltBN128._verify(_vk, proof, inputs);
    }
}
