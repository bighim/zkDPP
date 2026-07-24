// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {IFieldHasher} from "./IFieldHasher.sol";
import {IZkVerifier} from "./IZkVerifier.sol";

contract ZkDPP {
    uint256 public constant TREE_DEPTH = 32;
    uint256 public constant EPOCH_SIZE = 600;
    uint256 public constant MAX_PROCESS_ARITY = 3;
    uint256 public constant STATE_LENGTH = 3;

    uint8 internal constant ENTRY = 0;
    uint8 internal constant TRANSFER = 1;
    uint8 internal constant PROCEED = 2;
    uint8 internal constant RECALL = 3;
    uint8 internal constant MERGE = 4;
    uint8 internal constant SPLIT = 5;
    uint8 internal constant PROCESS = 6;
    uint8 internal constant EXIT = 7;

    struct Tree {
        mapping(uint256 => bool) acceptedRoots;
        mapping(uint256 => uint256) nodes;
        uint256[TREE_DEPTH + 1] zeroes;
        uint256 leafCount;
        uint256 currentRoot;
    }

    struct User {
        bool registered;
        uint256 addressField;
        uint256 publicKey;
    }

    IZkVerifier[8] public verifiers;
    IFieldHasher public immutable fieldHasher;
    Tree private mt;
    Tree private rvmt;

    mapping(uint256 => bool) public commitments;
    mapping(uint256 => bool) public nullifiers;
    mapping(uint256 => bool) public vouchers;
    mapping(uint256 => bool) public voucherNullifiers;
    mapping(uint256 => uint256) public deadlineRV;
    mapping(address => User) public users;

    event MTAppend(uint256 indexed leaf, uint256 index, uint256 root);
    event RVMTAppend(uint256 indexed leaf, uint256 index, uint256 root);

    error InvalidProof();
    error InvalidRoot();
    error InvalidArrayLength();
    error DuplicateCommitment();
    error DuplicateVoucher();
    error DuplicateInput();
    error SpentNullifier();
    error ResolvedVoucher();
    error UnknownVoucher();
    error RecallDeadlineExpired();
    error TreeFull();
    error InvalidLeafIndex();
    error AlreadyRegistered();

    constructor(address[8] memory verifierAddresses, address fieldHasher_) {
        for (uint256 i = 0; i < 8; i++) {
            verifiers[i] = IZkVerifier(verifierAddresses[i]);
        }
        fieldHasher = IFieldHasher(fieldHasher_);
        _initTree(mt);
        _initTree(rvmt);
    }

    function registerUser(uint256 addressField, uint256 publicKey) external {
        if (users[msg.sender].registered) revert AlreadyRegistered();
        users[msg.sender] = User(true, addressField, publicKey);
    }

    function entry(bytes calldata proof, uint256 cmNew) external {
        if (commitments[cmNew]) revert DuplicateCommitment();
        _verify(ENTRY, proof, _one(cmNew));
        commitments[cmNew] = true;
        _appendMT(cmNew);
    }

    function transfer(
        bytes calldata proof,
        uint256 root,
        uint256 cmIn,
        uint256 nf,
        uint256 rv,
        uint256 cmChange,
        uint256 deltaEpoch
    ) external {
        if (!mt.acceptedRoots[root]) revert InvalidRoot();
        if (nullifiers[nf]) revert SpentNullifier();
        if (commitments[cmChange]) revert DuplicateCommitment();
        if (vouchers[rv]) revert DuplicateVoucher();
        uint256[] memory inputs = new uint256[](6);
        inputs[0] = root;
        inputs[1] = cmIn;
        inputs[2] = nf;
        inputs[3] = rv;
        inputs[4] = cmChange;
        inputs[5] = deltaEpoch;
        _verify(TRANSFER, proof, inputs);
        nullifiers[nf] = true;
        commitments[cmChange] = true;
        vouchers[rv] = true;
        _appendMT(cmChange);
        _appendRVMT(rv);
        deadlineRV[rv] = currentEpoch() + deltaEpoch;
    }

    function proceed(bytes calldata proof, uint256 root, uint256 rv, uint256 rvnf, uint256 cmRecv) external {
        _resolutionPreconditions(root, rv, rvnf, cmRecv);
        _verify(PROCEED, proof, _four(root, rv, rvnf, cmRecv));
        voucherNullifiers[rvnf] = true;
        commitments[cmRecv] = true;
        _appendMT(cmRecv);
    }

    function recall(bytes calldata proof, uint256 root, uint256 rv, uint256 rvnf, uint256 cmReturn) external {
        _resolutionPreconditions(root, rv, rvnf, cmReturn);
        if (currentEpoch() >= deadlineRV[rv]) revert RecallDeadlineExpired();
        _verify(RECALL, proof, _four(root, rv, rvnf, cmReturn));
        voucherNullifiers[rvnf] = true;
        commitments[cmReturn] = true;
        _appendMT(cmReturn);
    }

    function merge(
        bytes calldata proof,
        uint256 root1,
        uint256 root2,
        uint256 cm1,
        uint256 cm2,
        uint256 nf1,
        uint256 nf2,
        uint256 cmOut
    ) external {
        if (!mt.acceptedRoots[root1] || !mt.acceptedRoots[root2]) revert InvalidRoot();
        if (cm1 == cm2) revert DuplicateInput();
        if (nullifiers[nf1] || nullifiers[nf2]) revert SpentNullifier();
        if (commitments[cmOut]) revert DuplicateCommitment();
        uint256[] memory inputs = new uint256[](7);
        inputs[0] = root1;
        inputs[1] = root2;
        inputs[2] = cm1;
        inputs[3] = cm2;
        inputs[4] = nf1;
        inputs[5] = nf2;
        inputs[6] = cmOut;
        _verify(MERGE, proof, inputs);
        nullifiers[nf1] = true;
        nullifiers[nf2] = true;
        commitments[cmOut] = true;
        _appendMT(cmOut);
    }

    function split(bytes calldata proof, uint256 root, uint256 cmIn, uint256 nf, uint256 cmOut1, uint256 cmOut2)
        external
    {
        if (!mt.acceptedRoots[root]) revert InvalidRoot();
        if (nullifiers[nf]) revert SpentNullifier();
        uint256[] memory inputs = new uint256[](5);
        inputs[0] = root;
        inputs[1] = cmIn;
        inputs[2] = nf;
        inputs[3] = cmOut1;
        inputs[4] = cmOut2;
        _verify(SPLIT, proof, inputs);
        nullifiers[nf] = true;
        _insertOutput(cmOut1);
        _insertOutput(cmOut2);
    }

    function process(
        bytes calldata proof,
        uint256[] calldata processDelta,
        uint256[] calldata allocation,
        uint256[] calldata roots,
        uint256[] calldata cmIn,
        uint256[] calldata nfIn,
        uint256[] calldata cmOut
    ) external {
        uint256 m = cmIn.length;
        uint256 n = cmOut.length;
        if (
            m == 0 || m > MAX_PROCESS_ARITY || n == 0 || n > MAX_PROCESS_ARITY || roots.length != m || nfIn.length != m
                || processDelta.length != STATE_LENGTH || allocation.length != n * STATE_LENGTH
        ) revert InvalidArrayLength();
        for (uint256 i = 0; i < m; i++) {
            if (!mt.acceptedRoots[roots[i]]) revert InvalidRoot();
            if (nullifiers[nfIn[i]]) revert SpentNullifier();
            for (uint256 j = 0; j < i; j++) {
                if (cmIn[i] == cmIn[j]) revert DuplicateInput();
            }
        }
        uint256[] memory inputs = new uint256[](26);
        inputs[0] = m;
        inputs[1] = n;
        for (uint256 k = 0; k < STATE_LENGTH; k++) {
            inputs[2 + k] = processDelta[k];
        }
        for (uint256 j = 0; j < n; j++) {
            for (uint256 k = 0; k < STATE_LENGTH; k++) {
                inputs[5 + j * STATE_LENGTH + k] = allocation[j * STATE_LENGTH + k];
            }
        }
        for (uint256 i = 0; i < m; i++) {
            inputs[14 + i * 3] = roots[i];
            inputs[15 + i * 3] = cmIn[i];
            inputs[16 + i * 3] = nfIn[i];
        }
        for (uint256 j = 0; j < n; j++) {
            inputs[23 + j] = cmOut[j];
        }
        _verify(PROCESS, proof, inputs);
        for (uint256 i = 0; i < m; i++) {
            nullifiers[nfIn[i]] = true;
        }
        for (uint256 j = 0; j < n; j++) {
            _insertOutput(cmOut[j]);
        }
    }

    function exit(bytes calldata proof, uint256 root, uint256 cmIn, uint256 nf) external {
        if (!mt.acceptedRoots[root]) revert InvalidRoot();
        if (nullifiers[nf]) revert SpentNullifier();
        uint256[] memory inputs = new uint256[](3);
        inputs[0] = root;
        inputs[1] = cmIn;
        inputs[2] = nf;
        _verify(EXIT, proof, inputs);
        nullifiers[nf] = true;
    }

    function getMTPath(uint256 index) external view returns (uint256, uint256[] memory) {
        return _getPath(mt, index);
    }

    function getRVMTPath(uint256 index) external view returns (uint256, uint256[] memory) {
        return _getPath(rvmt, index);
    }

    function currentMTRoot() external view returns (uint256) {
        return mt.currentRoot;
    }

    function currentRVRoot() external view returns (uint256) {
        return rvmt.currentRoot;
    }

    function mtLeafCount() external view returns (uint256) {
        return mt.leafCount;
    }

    function rvLeafCount() external view returns (uint256) {
        return rvmt.leafCount;
    }

    function acceptedMTRoot(uint256 root) external view returns (bool) {
        return mt.acceptedRoots[root];
    }

    function acceptedRVRoot(uint256 root) external view returns (bool) {
        return rvmt.acceptedRoots[root];
    }

    function currentEpoch() public view returns (uint256) {
        return block.timestamp / EPOCH_SIZE;
    }

    function _resolutionPreconditions(uint256 root, uint256 rv, uint256 rvnf, uint256 cmOut) private view {
        if (!rvmt.acceptedRoots[root]) revert InvalidRoot();
        if (!vouchers[rv]) revert UnknownVoucher();
        if (voucherNullifiers[rvnf]) revert ResolvedVoucher();
        if (commitments[cmOut]) revert DuplicateCommitment();
    }

    function _insertOutput(uint256 cm) private {
        if (commitments[cm]) revert DuplicateCommitment();
        commitments[cm] = true;
        _appendMT(cm);
    }

    function _verify(uint8 eventID, bytes calldata proof, uint256[] memory inputs) private view {
        if (!verifiers[eventID].Verify(proof, inputs)) revert InvalidProof();
    }

    function _one(uint256 a) private pure returns (uint256[] memory v) {
        v = new uint256[](1);
        v[0] = a;
    }

    function _four(uint256 a, uint256 b, uint256 c, uint256 d) private pure returns (uint256[] memory v) {
        v = new uint256[](4);
        v[0] = a;
        v[1] = b;
        v[2] = c;
        v[3] = d;
    }

    function _initTree(Tree storage tree) private {
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            tree.zeroes[level + 1] = fieldHasher.compress(tree.zeroes[level], tree.zeroes[level]);
        }
        tree.currentRoot = tree.zeroes[TREE_DEPTH];
        tree.acceptedRoots[tree.currentRoot] = true;
    }

    function _appendMT(uint256 leaf) private {
        _append(mt, leaf, false);
    }

    function _appendRVMT(uint256 leaf) private {
        _append(rvmt, leaf, true);
    }

    function _append(Tree storage tree, uint256 leaf, bool isRV) private {
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
        if (isRV) emit RVMTAppend(leaf, index, current);
        else emit MTAppend(leaf, index, current);
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

    function _node(Tree storage tree, uint256 level, uint256 position) private view returns (uint256) {
        uint256 value = tree.nodes[_nodeIndex(level, position)];
        return value == 0 ? tree.zeroes[level] : value;
    }

    function _nodeIndex(uint256 level, uint256 position) private pure returns (uint256) {
        return (uint256(1) << (TREE_DEPTH - level)) - 1 + position;
    }
}
