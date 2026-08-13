// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

interface IPlonkVerifier {
    function Verify(bytes calldata proof, uint256[] calldata publicInputs) external view returns (bool);
}

/// @notice RSA-free DocHash-ID adaptation of the Sahai supply-chain events.
/// @dev Every input is consumed exactly once. Permissioning is authorization, not anonymous authentication.
contract SahaiLedger {
    error Unauthorized();
    error UnknownDocument();
    error DuplicateDocument();
    error ConsumedDocument();
    error TerminalDocument();
    error InvalidProof();
    error InvalidTransition();

    struct AssetData {
        bytes32 documentType;
        bytes32 docHash;
        bool terminal;
    }

    struct DocumentAsset {
        bytes32 documentType;
        bytes32 docHash;
        bytes32[] inDocs;
        bool terminal;
        bool consumed;
        bool exists;
    }

    struct GadgetProof {
        bytes proof;
        uint256[] publicInputs;
    }

    event ParticipantRegistered(address indexed participant);
    event DocumentTransition(bytes32 indexed eventType, bytes32[] inputIds, bytes32[] outputIds);

    address public immutable administrator;
    uint8 public immutable digestWidth;
    IPlonkVerifier public immutable merklePathVerifier;
    IPlonkVerifier public immutable eqVerifier;
    IPlonkVerifier public immutable addVerifier;
    IPlonkVerifier public immutable andVerifier;
    mapping(address => bool) public participants;
    mapping(bytes32 => DocumentAsset) internal documents;

    modifier onlyAdministrator() { if (msg.sender != administrator) revert Unauthorized(); _; }
    modifier onlyParticipant() { if (!participants[msg.sender]) revert Unauthorized(); _; }

    constructor(uint8 digestWidth_, address merkle_, address eq_, address add_, address and_) {
        if (digestWidth_ != 1 && digestWidth_ != 2) revert InvalidTransition();
        administrator = msg.sender;
        digestWidth = digestWidth_;
        merklePathVerifier = IPlonkVerifier(merkle_);
        eqVerifier = IPlonkVerifier(eq_);
        addVerifier = IPlonkVerifier(add_);
        andVerifier = IPlonkVerifier(and_);
        participants[msg.sender] = true;
        emit ParticipantRegistered(msg.sender);
    }

    function registerParticipant(address participant) external onlyAdministrator {
        participants[participant] = true;
        emit ParticipantRegistered(participant);
    }

    function entry(AssetData calldata output, GadgetProof[] calldata proofs) external onlyParticipant {
        if (output.terminal) revert InvalidTransition();
        _verifyEntry(output, proofs);
        bytes32[] memory inputs = new bytes32[](0);
        bytes32[] memory outputs = _single(_store(output, inputs));
        emit DocumentTransition("Entry", inputs, outputs);
    }

    function ship(bytes32 inputId, AssetData calldata output, GadgetProof[] calldata proofs) external onlyParticipant {
        DocumentAsset storage input = _loadInput(inputId);
        if (output.terminal) revert InvalidTransition();
        _verifyShip(input, output, proofs);
        bytes32[] memory inputs = _single(inputId);
        _consume(input);
        bytes32[] memory outputs = _single(_store(output, inputs));
        emit DocumentTransition("Ship", inputs, outputs);
    }

    function merge(bytes32[2] calldata inputIds, AssetData calldata output, GadgetProof[] calldata proofs) external onlyParticipant {
        _combine("Merge", inputIds, output, proofs);
    }

    function process(bytes32[2] calldata inputIds, AssetData calldata output, GadgetProof[] calldata proofs) external onlyParticipant {
        _combine("Process", inputIds, output, proofs);
    }

    function split(bytes32 inputId, AssetData[2] calldata outputsData, GadgetProof[] calldata proofs) external onlyParticipant {
        DocumentAsset storage input = _loadInput(inputId);
        if (outputsData[0].terminal || outputsData[1].terminal || outputsData[0].docHash == outputsData[1].docHash) revert InvalidTransition();
        _verifySplit(input, outputsData, proofs);
        bytes32[] memory inputs = _single(inputId);
        _consume(input);
        bytes32[] memory outputs = new bytes32[](2);
        outputs[0] = _store(outputsData[0], inputs);
        outputs[1] = _store(outputsData[1], inputs);
        emit DocumentTransition("Split", inputs, outputs);
    }

    function exit(bytes32 inputId, AssetData calldata output, GadgetProof[] calldata proofs) external onlyParticipant {
        DocumentAsset storage input = _loadInput(inputId);
        if (!output.terminal) revert InvalidTransition();
        _verifyExit(output, proofs);
        bytes32[] memory inputs = _single(inputId);
        _consume(input);
        bytes32[] memory outputs = _single(_store(output, inputs));
        emit DocumentTransition("Exit", inputs, outputs);
    }

    function getDocument(bytes32 id) external view returns (DocumentAsset memory) { return documents[id]; }
    function isConsumed(bytes32 id) external view returns (bool) { return documents[id].consumed; }

    function _combine(bytes32 eventType, bytes32[2] calldata inputIds, AssetData calldata output, GadgetProof[] calldata proofs) private {
        if (inputIds[0] == inputIds[1] || output.terminal) revert InvalidTransition();
        DocumentAsset storage first = _loadInput(inputIds[0]);
        DocumentAsset storage second = _loadInput(inputIds[1]);
        _verifyCombine(first, second, output, proofs);
        bytes32[] memory inputs = new bytes32[](2);
        inputs[0] = inputIds[0]; inputs[1] = inputIds[1];
        _consume(first); _consume(second);
        bytes32[] memory outputs = _single(_store(output, inputs));
        emit DocumentTransition(eventType, inputs, outputs);
    }

    function _loadInput(bytes32 id) internal view returns (DocumentAsset storage input) {
        input = documents[id];
        if (!input.exists) revert UnknownDocument();
        if (input.terminal) revert TerminalDocument();
        if (input.consumed) revert ConsumedDocument();
    }

    function _consume(DocumentAsset storage input) internal { input.consumed = true; }

    function _store(AssetData calldata output, bytes32[] memory inputs) internal returns (bytes32 id) {
        id = output.docHash;
        if (id == bytes32(0) || documents[id].exists) revert DuplicateDocument();
        DocumentAsset storage target = documents[id];
        target.documentType = output.documentType;
        target.docHash = id;
        target.inDocs = inputs;
        target.terminal = output.terminal;
        target.exists = true;
    }

    function _verifyEntry(AssetData calldata output, GadgetProof[] calldata proofs) private view {
        if (proofs.length != 2) revert InvalidProof(); _path(proofs[0], output.docHash); _eq(proofs[1], proofs[0], 0, proofs[0], 1);
    }
    function _verifyShip(DocumentAsset storage input, AssetData calldata output, GadgetProof[] calldata proofs) private view {
        if (proofs.length != 5) revert InvalidProof(); _path(proofs[0], input.docHash); _path(proofs[1], output.docHash);
        _eq(proofs[2], proofs[0], 1, proofs[1], 0); _eq(proofs[3], proofs[0], 2, proofs[1], 2); _eq(proofs[4], proofs[0], 3, proofs[1], 3);
    }
    function _verifyCombine(DocumentAsset storage first, DocumentAsset storage second, AssetData calldata output, GadgetProof[] calldata proofs) private view {
        if (proofs.length != 6) revert InvalidProof(); _path(proofs[0], first.docHash); _path(proofs[1], second.docHash); _path(proofs[2], output.docHash);
        _eq(proofs[3], proofs[0], 1, proofs[2], 0); _eq(proofs[4], proofs[1], 1, proofs[2], 0); _add(proofs[5], proofs[0], 3, proofs[1], 3, proofs[2], 3);
    }
    function _verifySplit(DocumentAsset storage input, AssetData[2] calldata outputsData, GadgetProof[] calldata proofs) private view {
        if (proofs.length != 6) revert InvalidProof(); _path(proofs[0], input.docHash); _path(proofs[1], outputsData[0].docHash); _path(proofs[2], outputsData[1].docHash);
        _eq(proofs[3], proofs[0], 1, proofs[1], 0); _eq(proofs[4], proofs[0], 1, proofs[2], 0); _add(proofs[5], proofs[1], 3, proofs[2], 3, proofs[0], 3);
    }
    function _verifyExit(AssetData calldata output, GadgetProof[] calldata proofs) private view {
        if (proofs.length != 2) revert InvalidProof(); _path(proofs[0], output.docHash); _eq(proofs[1], proofs[0], 0, proofs[0], 1);
    }

    function _path(GadgetProof calldata candidate, bytes32 root) private view {
        uint256 width = digestWidth;
        if (candidate.publicInputs.length != width * 5 || !_rootEqual(candidate.publicInputs, root)) revert InvalidProof();
        if (!merklePathVerifier.Verify(candidate.proof, candidate.publicInputs)) revert InvalidProof();
    }
    function _eq(GadgetProof calldata candidate, GadgetProof calldata left, uint256 li, GadgetProof calldata right, uint256 ri) private view {
        uint256 width = digestWidth; if (candidate.publicInputs.length != width * 2) revert InvalidProof();
        if (!_same(candidate.publicInputs, 0, left.publicInputs, width + li * width, width) || !_same(candidate.publicInputs, width, right.publicInputs, width + ri * width, width)) revert InvalidProof();
        if (!eqVerifier.Verify(candidate.proof, candidate.publicInputs)) revert InvalidProof();
    }
    function _add(GadgetProof calldata candidate, GadgetProof calldata x, uint256 xi, GadgetProof calldata y, uint256 yi, GadgetProof calldata z, uint256 zi) private view {
        uint256 width = digestWidth; if (candidate.publicInputs.length != width * 3) revert InvalidProof();
        if (!_same(candidate.publicInputs,0,x.publicInputs,width+xi*width,width) || !_same(candidate.publicInputs,width,y.publicInputs,width+yi*width,width) || !_same(candidate.publicInputs,width*2,z.publicInputs,width+zi*width,width)) revert InvalidProof();
        if (!addVerifier.Verify(candidate.proof, candidate.publicInputs)) revert InvalidProof();
    }
    function _rootEqual(uint256[] calldata values, bytes32 root) private view returns (bool) {
        if (digestWidth == 1) return values[0] == uint256(root);
        return values[0] == (uint256(root) >> 128)
            && values[1] == (uint256(root) & type(uint128).max);
    }
    function _same(uint256[] calldata a,uint256 ao,uint256[] calldata b,uint256 bo,uint256 width) private pure returns(bool) {
        for (uint256 i=0;i<width;i++) if(a[ao+i]!=b[bo+i]) return false; return true;
    }
    function _single(bytes32 value) private pure returns (bytes32[] memory values) { values = new bytes32[](1); values[0] = value; }
}
