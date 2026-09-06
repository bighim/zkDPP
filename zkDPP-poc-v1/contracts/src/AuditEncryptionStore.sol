// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {IZkVerifier} from "./IZkVerifier.sol";

/// @notice M6-B1 diagnostic storage, NOT a supply-chain ledger.
/// There is intentionally no nullifier, accepted-root, grant or status state.
contract AuditEncryptionStore {
    uint256 private constant FIELD_MODULUS =
        52435875175126190479447740508185965837690552500527637822603658699938581184513;
    IZkVerifier public immutable processVerifier;
    uint256 public nextRecordId = 1;
    mapping(uint256 => uint256[15]) private records;

    error InvalidVerifier();
    error InvalidField();
    error InvalidProof();
    error UnknownRecord();
    event RecordStored(uint256 indexed id);

    constructor(address verifier) {
        if (verifier == address(0) || verifier.code.length == 0) revert InvalidVerifier();
        processVerifier = IZkVerifier(verifier);
    }

    function verifyOnly(bytes calldata proof, uint256[15] calldata inputs) external view returns (bool) {
        _verify(proof, inputs);
        return true;
    }

    function verifyAndStore(bytes calldata proof, uint256[15] calldata inputs)
        external returns (uint256 id)
    {
        _verify(proof, inputs);
        id = nextRecordId;
        records[id] = inputs;
        nextRecordId = id + 1;
        emit RecordStored(id);
    }

    function getRecord(uint256 id) external view returns (uint256[15] memory) {
        if (id == 0 || id >= nextRecordId) revert UnknownRecord();
        return records[id];
    }

    function _verify(bytes calldata proof, uint256[15] calldata inputs) private view {
        uint256[] memory publicInputs = new uint256[](15);
        for (uint256 i; i < 15; ++i) {
            if (inputs[i] >= FIELD_MODULUS) revert InvalidField();
            publicInputs[i] = inputs[i];
        }
        try processVerifier.Verify(proof, publicInputs) returns (bool valid) {
            if (!valid) revert InvalidProof();
        } catch {
            revert InvalidProof();
        }
    }
}
