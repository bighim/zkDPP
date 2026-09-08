// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {IFieldHasher} from "./IFieldHasher.sol";
import {IZkVerifier} from "./IZkVerifier.sol";

contract EntryExitLedger {
    uint256 public constant TREE_DEPTH = 32;
    uint256 public constant EPOCH_SIZE = 600;
    uint8 public constant PROCESS = 6;
    uint8 public constant ISSUE = 8;
    uint256 public constant POLICY_REF_TAG = 51780253709110118800953810799850621917828020824397005216617999075326973004822;

    struct Tree {
        mapping(uint256 => bool) acceptedRoots;
        mapping(uint256 => uint256) nodes;
        uint256[TREE_DEPTH + 1] zeroes;
        uint256 leafCount;
        uint256 currentRoot;
    }

    struct PolicyAuthority { address account; bool enabled; }
    struct PolicyFamily { uint8 eventKind; bool exists; }
    struct PolicyReservation { uint64 authorityId; uint64 policyId; uint64 version; uint8 eventKind; bool exists; }
    struct PolicyRecord { uint64 authorityId; uint64 policyId; uint64 version; uint8 eventKind; uint8 inputArity; uint8 outputArity; bytes32 vkHash; address verifierRef; bool enabled; }

    address public immutable admin;
    IZkVerifier public immutable entryVerifier;
    IZkVerifier public immutable privateSpendVerifier;
    IZkVerifier public immutable transferVerifier;
    IZkVerifier public immutable proceedVerifier;
    IZkVerifier public immutable recallVerifier;
    IZkVerifier public immutable mergeVerifier;
    IZkVerifier public immutable splitVerifier;
    IFieldHasher public immutable fieldHasher;

    mapping(address => bool) public entryIssuers;
    mapping(uint256 => bool) public commitments;
    mapping(uint256 => bool) public noteNullifiers;
    mapping(uint256 => bool) public voucherCommitments;
    mapping(uint256 => bool) public voucherNullifiers;
    uint64 public nextAuthorityId = 1;
    mapping(address => uint64) public authorityIdOf;
    mapping(uint64 => PolicyAuthority) public policyAuthorities;
    mapping(uint64 => uint64) public nextPolicyId;
    mapping(uint64 => mapping(uint64 => uint64)) public nextPolicyVersion;
    mapping(uint64 => mapping(uint64 => PolicyFamily)) public policyFamilies;
    mapping(uint256 => PolicyReservation) public policyReservations;
    mapping(uint256 => PolicyRecord) public policyRecords;
    mapping(uint256 => mapping(uint256 => bool)) public policyGrants;
    Tree private noteTree;
    Tree private voucherTree;

    event EntryIssuerUpdated(address indexed account, bool allowed);
    event NoteAppended(uint256 indexed cm, uint256 index, uint256 root);
    event NoteExited(uint256 indexed nf);
    event VoucherAppended(uint256 indexed rv, uint256 index, uint256 root);
    event PolicyAuthorityRegistered(uint64 indexed authorityId, address indexed account);
    event PolicyReserved(uint256 indexed policyRef, uint64 indexed authorityId, uint64 policyId, uint64 version, uint8 eventKind);
    event PolicyRegistered(uint256 indexed policyRef, address indexed verifierRef, bytes32 vkHash);
    event PolicyGrantUpdated(uint256 indexed policyRef, uint256 indexed policyScopeRef, bool allowed);
    event PolicyDisabled(uint256 indexed policyRef);

    error NotAdmin();
    error InvalidAccount();
    error NotEntryIssuer();
    error DuplicateCommitment();
    error DuplicateVoucher();
    error DuplicateInput();
    error InvalidRoot();
    error SpentNullifier();
    error ResolvedVoucher();
    error InvalidEpoch();
    error InvalidProof();
    error TreeFull();
    error InvalidLeafIndex();
    error NotPolicyAuthority();
    error DuplicateAuthority();
    error InvalidEventKind();
    error UnknownPolicyFamily();
    error PreviousVersionNotRegistered();
    error UnknownPolicyReservation();
    error DuplicatePolicy();
    error InvalidPolicyRecord();
    error UnknownPolicy();
    error DisabledPolicy();
    error MissingPolicyGrant();

    constructor(
        address entryVerifier_,
        address privateSpendVerifier_,
        address transferVerifier_,
        address proceedVerifier_,
        address recallVerifier_,
        address mergeVerifier_,
        address splitVerifier_,
        address fieldHasher_
    ) {
        admin = msg.sender;
        entryVerifier = IZkVerifier(entryVerifier_);
        privateSpendVerifier = IZkVerifier(privateSpendVerifier_);
        transferVerifier = IZkVerifier(transferVerifier_);
        proceedVerifier = IZkVerifier(proceedVerifier_);
        recallVerifier = IZkVerifier(recallVerifier_);
        mergeVerifier = IZkVerifier(mergeVerifier_);
        splitVerifier = IZkVerifier(splitVerifier_);
        fieldHasher = IFieldHasher(fieldHasher_);
        _initTree(noteTree);
        _initTree(voucherTree);
    }

    function setEntryIssuer(address account, bool allowed) external {
        if (msg.sender != admin) revert NotAdmin();
        if (account == address(0)) revert InvalidAccount();
        entryIssuers[account] = allowed;
        emit EntryIssuerUpdated(account, allowed);
    }

    function registerPolicyAuthority(address account) external returns (uint64 authorityId) {
        if (msg.sender != admin) revert NotAdmin();
        if (account == address(0)) revert InvalidAccount();
        if (authorityIdOf[account] != 0) revert DuplicateAuthority();
        authorityId = nextAuthorityId++;
        authorityIdOf[account] = authorityId;
        policyAuthorities[authorityId] = PolicyAuthority(account, true);
        nextPolicyId[authorityId] = 1;
        emit PolicyAuthorityRegistered(authorityId, account);
    }

    function reservePolicy(uint8 eventKind) external returns (uint256 policyRef) {
        uint64 authorityId = _authority(msg.sender);
        if (eventKind != PROCESS && eventKind != ISSUE) revert InvalidEventKind();
        uint64 policyId = nextPolicyId[authorityId]++;
        uint64 version = 1;
        policyFamilies[authorityId][policyId] = PolicyFamily(eventKind, true);
        nextPolicyVersion[authorityId][policyId] = 2;
        policyRef = _computePolicyRef(eventKind, authorityId, policyId, version);
        policyReservations[policyRef] = PolicyReservation(authorityId, policyId, version, eventKind, true);
        emit PolicyReserved(policyRef, authorityId, policyId, version, eventKind);
    }

    function reservePolicyVersion(uint64 policyId) external returns (uint256 policyRef) {
        uint64 authorityId = _authority(msg.sender);
        PolicyFamily memory family = policyFamilies[authorityId][policyId];
        if (!family.exists) revert UnknownPolicyFamily();
        uint64 version = nextPolicyVersion[authorityId][policyId];
        uint256 previousRef = _computePolicyRef(family.eventKind, authorityId, policyId, version - 1);
        if (policyRecords[previousRef].authorityId == 0) revert PreviousVersionNotRegistered();
        nextPolicyVersion[authorityId][policyId] = version + 1;
        policyRef = _computePolicyRef(family.eventKind, authorityId, policyId, version);
        policyReservations[policyRef] = PolicyReservation(authorityId, policyId, version, family.eventKind, true);
        emit PolicyReserved(policyRef, authorityId, policyId, version, family.eventKind);
    }

    function registerPolicy(uint256 policyRef, uint8 inputArity, uint8 outputArity, bytes32 vkHash, address verifierRef) external {
        uint64 authorityId = _authority(msg.sender);
        PolicyReservation memory reservation = policyReservations[policyRef];
        if (!reservation.exists || reservation.authorityId != authorityId) revert UnknownPolicyReservation();
        if (policyRecords[policyRef].authorityId != 0) revert DuplicatePolicy();
        if (inputArity == 0 || outputArity == 0 || vkHash == bytes32(0) || verifierRef.code.length == 0) revert InvalidPolicyRecord();
        policyRecords[policyRef] = PolicyRecord(authorityId, reservation.policyId, reservation.version, reservation.eventKind, inputArity, outputArity, vkHash, verifierRef, true);
        emit PolicyRegistered(policyRef, verifierRef, vkHash);
    }

    function setPolicyGrant(uint256 policyRef, uint256 policyScopeRef, bool allowed) external {
        PolicyRecord memory record = policyRecords[policyRef];
        if (record.authorityId == 0) revert UnknownPolicy();
        if (_authority(msg.sender) != record.authorityId) revert NotPolicyAuthority();
        if (policyScopeRef == 0) revert InvalidPolicyRecord();
        policyGrants[policyRef][policyScopeRef] = allowed;
        emit PolicyGrantUpdated(policyRef, policyScopeRef, allowed);
    }

    function disablePolicy(uint256 policyRef) external {
        PolicyRecord storage record = policyRecords[policyRef];
        if (record.authorityId == 0) revert UnknownPolicy();
        if (_authority(msg.sender) != record.authorityId) revert NotPolicyAuthority();
        if (!record.enabled) revert DisabledPolicy();
        record.enabled = false;
        emit PolicyDisabled(policyRef);
    }

    function computePolicyRef(uint8 eventKind, uint64 authorityId, uint64 policyId, uint64 version) external view returns (uint256) {
        return _computePolicyRef(eventKind, authorityId, policyId, version);
    }

    function entry(bytes calldata proof, uint256 cm) external {
        if (!entryIssuers[msg.sender]) revert NotEntryIssuer();
        if (commitments[cm]) revert DuplicateCommitment();
        _verify(entryVerifier, proof, _one(cm));
        commitments[cm] = true;
        _append(noteTree, cm, false);
    }

    function exit(bytes calldata proof, uint256 noteRoot, uint256 nf) external {
        if (!noteTree.acceptedRoots[noteRoot]) revert InvalidRoot();
        if (noteNullifiers[nf]) revert SpentNullifier();
        _verify(privateSpendVerifier, proof, _two(noteRoot, nf));
        noteNullifiers[nf] = true;
        emit NoteExited(nf);
    }

    function transfer(
        bytes calldata proof,
        uint256 noteRoot,
        uint256 nf,
        uint256 rvNew,
        uint256 cmChange,
        uint256 transferEpoch,
        uint256 deltaEpoch
    ) external {
        if (!noteTree.acceptedRoots[noteRoot]) revert InvalidRoot();
        if (noteNullifiers[nf]) revert SpentNullifier();
        if (commitments[cmChange]) revert DuplicateCommitment();
        if (voucherCommitments[rvNew]) revert DuplicateVoucher();
        if (transferEpoch != currentEpoch()) revert InvalidEpoch();
        uint256[] memory inputs = new uint256[](6);
        inputs[0] = noteRoot;
        inputs[1] = nf;
        inputs[2] = rvNew;
        inputs[3] = cmChange;
        inputs[4] = transferEpoch;
        inputs[5] = deltaEpoch;
        _verify(transferVerifier, proof, inputs);
        noteNullifiers[nf] = true;
        commitments[cmChange] = true;
        voucherCommitments[rvNew] = true;
        _append(noteTree, cmChange, false);
        _append(voucherTree, rvNew, true);
    }

    function proceed(bytes calldata proof, uint256 voucherRoot, uint256 rvnf, uint256 cmReceiver) external {
        _resolutionPreconditions(voucherRoot, rvnf, cmReceiver);
        _verify(proceedVerifier, proof, _three(voucherRoot, rvnf, cmReceiver));
        voucherNullifiers[rvnf] = true;
        commitments[cmReceiver] = true;
        _append(noteTree, cmReceiver, false);
    }

    function recall(bytes calldata proof, uint256 voucherRoot, uint256 rvnf, uint256 cmReturn, uint256 epoch)
        external
    {
        _resolutionPreconditions(voucherRoot, rvnf, cmReturn);
        if (epoch != currentEpoch()) revert InvalidEpoch();
        _verify(recallVerifier, proof, _four(voucherRoot, rvnf, cmReturn, epoch));
        voucherNullifiers[rvnf] = true;
        commitments[cmReturn] = true;
        _append(noteTree, cmReturn, false);
    }

    function merge(bytes calldata proof, uint256 noteRoot, uint256 nf1, uint256 nf2, uint256 cmOut) external {
        if (!noteTree.acceptedRoots[noteRoot]) revert InvalidRoot();
        if (nf1 == nf2) revert DuplicateInput();
        if (noteNullifiers[nf1] || noteNullifiers[nf2]) revert SpentNullifier();
        if (commitments[cmOut]) revert DuplicateCommitment();
        _verify(mergeVerifier, proof, _four(noteRoot, nf1, nf2, cmOut));
        noteNullifiers[nf1] = true;
        noteNullifiers[nf2] = true;
        commitments[cmOut] = true;
        _append(noteTree, cmOut, false);
    }

    function split(bytes calldata proof, uint256 noteRoot, uint256 nf, uint256 cmOut1, uint256 cmOut2) external {
        if (!noteTree.acceptedRoots[noteRoot]) revert InvalidRoot();
        if (noteNullifiers[nf]) revert SpentNullifier();
        if (cmOut1 == cmOut2) revert DuplicateCommitment();
        if (commitments[cmOut1] || commitments[cmOut2]) revert DuplicateCommitment();
        _verify(splitVerifier, proof, _four(noteRoot, nf, cmOut1, cmOut2));
        noteNullifiers[nf] = true;
        commitments[cmOut1] = true;
        _append(noteTree, cmOut1, false);
        if (commitments[cmOut2]) revert DuplicateCommitment();
        commitments[cmOut2] = true;
        _append(noteTree, cmOut2, false);
    }

    function process(bytes calldata proof, uint256 policyRef, uint256 policyScopeRef, uint256 noteRoot, uint256[3] calldata nf, uint256[2] calldata cmOut) external {
        PolicyRecord memory record = policyRecords[policyRef];
        if (record.authorityId == 0) revert UnknownPolicy();
        if (!record.enabled) revert DisabledPolicy();
        if (record.eventKind != PROCESS || record.inputArity != 3 || record.outputArity != 2) revert InvalidPolicyRecord();
        if (!policyGrants[policyRef][policyScopeRef]) revert MissingPolicyGrant();
        if (!noteTree.acceptedRoots[noteRoot]) revert InvalidRoot();
        for (uint256 i = 0; i < 3; i++) {
            if (noteNullifiers[nf[i]]) revert SpentNullifier();
            for (uint256 j = 0; j < i; j++) if (nf[i] == nf[j]) revert DuplicateInput();
        }
        if (cmOut[0] == cmOut[1] || commitments[cmOut[0]] || commitments[cmOut[1]]) revert DuplicateCommitment();
        uint256[] memory inputs = new uint256[](8);
        inputs[0] = policyRef; inputs[1] = policyScopeRef; inputs[2] = noteRoot;
        inputs[3] = nf[0]; inputs[4] = nf[1]; inputs[5] = nf[2]; inputs[6] = cmOut[0]; inputs[7] = cmOut[1];
        _verify(IZkVerifier(record.verifierRef), proof, inputs);
        for (uint256 i = 0; i < 3; i++) noteNullifiers[nf[i]] = true;
        for (uint256 i = 0; i < 2; i++) { commitments[cmOut[i]] = true; _append(noteTree, cmOut[i], false); }
    }

    function currentEpoch() public view returns (uint256) { return block.timestamp / EPOCH_SIZE; }
    function acceptedNoteRoot(uint256 root) external view returns (bool) { return noteTree.acceptedRoots[root]; }
    function acceptedVoucherRoot(uint256 root) external view returns (bool) { return voucherTree.acceptedRoots[root]; }
    function currentNoteRoot() external view returns (uint256) { return noteTree.currentRoot; }
    function currentVoucherRoot() external view returns (uint256) { return voucherTree.currentRoot; }
    function noteLeafCount() external view returns (uint256) { return noteTree.leafCount; }
    function voucherLeafCount() external view returns (uint256) { return voucherTree.leafCount; }
    function getNotePath(uint256 index) external view returns (uint256 root, uint256[] memory siblings) { return _getPath(noteTree, index); }
    function getVoucherPath(uint256 index) external view returns (uint256 root, uint256[] memory siblings) { return _getPath(voucherTree, index); }

    function _resolutionPreconditions(uint256 root, uint256 rvnf, uint256 cmOut) private view {
        if (!voucherTree.acceptedRoots[root]) revert InvalidRoot();
        if (voucherNullifiers[rvnf]) revert ResolvedVoucher();
        if (commitments[cmOut]) revert DuplicateCommitment();
    }

    function _authority(address account) private view returns (uint64 authorityId) {
        authorityId = authorityIdOf[account];
        if (authorityId == 0 || !policyAuthorities[authorityId].enabled) revert NotPolicyAuthority();
    }

    function _computePolicyRef(uint8 eventKind, uint64 authorityId, uint64 policyId, uint64 version) private view returns (uint256 state) {
        state = fieldHasher.compress(0, POLICY_REF_TAG);
        state = fieldHasher.compress(state, eventKind);
        state = fieldHasher.compress(state, authorityId);
        state = fieldHasher.compress(state, policyId);
        state = fieldHasher.compress(state, version);
    }

    function _initTree(Tree storage tree) private {
        for (uint256 level = 0; level < TREE_DEPTH; level++) tree.zeroes[level + 1] = fieldHasher.compress(tree.zeroes[level], tree.zeroes[level]);
        tree.currentRoot = tree.zeroes[TREE_DEPTH];
        tree.acceptedRoots[tree.currentRoot] = true;
    }

    function _append(Tree storage tree, uint256 leaf, bool isVoucher) private {
        if (tree.leafCount >= (uint256(1) << TREE_DEPTH)) revert TreeFull();
        uint256 index = tree.leafCount;
        uint256 position = index;
        uint256 current = leaf;
        tree.nodes[_nodeIndex(0, position)] = current;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            uint256 left;
            uint256 right;
            if (position & 1 == 0) {
                left = current;
                right = _node(tree, level, position + 1);
            } else {
                left = _node(tree, level, position - 1);
                right = current;
            }
            current = fieldHasher.compress(left, right);
            position >>= 1;
            tree.nodes[_nodeIndex(level + 1, position)] = current;
        }
        tree.leafCount = index + 1;
        tree.currentRoot = current;
        tree.acceptedRoots[current] = true;
        if (isVoucher) emit VoucherAppended(leaf, index, current);
        else emit NoteAppended(leaf, index, current);
    }

    function _getPath(Tree storage tree, uint256 index) private view returns (uint256 root, uint256[] memory siblings) {
        if (index >= tree.leafCount) revert InvalidLeafIndex();
        siblings = new uint256[](TREE_DEPTH);
        uint256 position = index;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            siblings[level] = _node(tree, level, position ^ 1);
            position >>= 1;
        }
        return (tree.currentRoot, siblings);
    }

    function _verify(IZkVerifier verifier, bytes calldata proof, uint256[] memory inputs) private view {
        try verifier.Verify(proof, inputs) returns (bool valid) { if (!valid) revert InvalidProof(); }
        catch { revert InvalidProof(); }
    }

    function _node(Tree storage tree, uint256 level, uint256 position) private view returns (uint256) {
        uint256 value = tree.nodes[_nodeIndex(level, position)];
        return value == 0 ? tree.zeroes[level] : value;
    }

    function _nodeIndex(uint256 level, uint256 position) private pure returns (uint256) { return (uint256(1) << (TREE_DEPTH - level)) - 1 + position; }
    function _one(uint256 a) private pure returns (uint256[] memory v) { v = new uint256[](1); v[0] = a; }
    function _two(uint256 a, uint256 b) private pure returns (uint256[] memory v) { v = new uint256[](2); v[0] = a; v[1] = b; }
    function _three(uint256 a, uint256 b, uint256 c) private pure returns (uint256[] memory v) { v = new uint256[](3); v[0] = a; v[1] = b; v[2] = c; }
    function _four(uint256 a, uint256 b, uint256 c, uint256 d) private pure returns (uint256[] memory v) { v = new uint256[](4); v[0] = a; v[1] = b; v[2] = c; v[3] = d; }
}
