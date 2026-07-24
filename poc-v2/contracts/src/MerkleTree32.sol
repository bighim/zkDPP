// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {IFieldHasher} from "./IFieldHasher.sol";

contract MerkleTree32 {
    uint256 public constant TREE_DEPTH = 32;
    IFieldHasher public immutable fieldHasher;

    mapping(uint256 => bool) public acceptedRoots;
    mapping(uint256 => uint256) private treeNodes;
    uint256[TREE_DEPTH + 1] private zeroes;
    uint256 public leafCount;
    uint256 public currentRoot;

    event LeafAppend(uint256 indexed leaf, uint256 index, uint256 root);

    error TreeFull();
    error InvalidLeafIndex();

    constructor(address fieldHasher_) {
        fieldHasher = IFieldHasher(fieldHasher_);
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            zeroes[level + 1] = fieldHasher.compress(zeroes[level], zeroes[level]);
        }
        currentRoot = zeroes[TREE_DEPTH];
        acceptedRoots[currentRoot] = true;
    }

    function append(uint256 leaf) external returns (uint256 index, uint256 root) {
        return _append(leaf);
    }

    function getPath(uint256 index) external view returns (uint256 root, uint256[] memory siblings) {
        if (index >= leafCount) revert InvalidLeafIndex();
        siblings = new uint256[](TREE_DEPTH);
        uint256 position = index;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            siblings[level] = _node(level, position ^ 1);
            position >>= 1;
        }
        return (currentRoot, siblings);
    }

    function _append(uint256 leaf) internal returns (uint256 index, uint256 root) {
        if (leafCount >= (uint256(1) << TREE_DEPTH)) revert TreeFull();
        index = leafCount;
        uint256 position = index;
        uint256 current = leaf;
        treeNodes[_nodeIndex(0, position)] = current;
        for (uint256 level = 0; level < TREE_DEPTH; level++) {
            uint256 left;
            uint256 right;
            if (position & 1 == 0) {
                left = current;
                right = _node(level, position + 1);
            } else {
                left = _node(level, position - 1);
                right = current;
            }
            current = fieldHasher.compress(left, right);
            position >>= 1;
            treeNodes[_nodeIndex(level + 1, position)] = current;
        }
        leafCount = index + 1;
        currentRoot = current;
        acceptedRoots[current] = true;
        emit LeafAppend(leaf, index, current);
        return (index, current);
    }

    function _node(uint256 level, uint256 position) private view returns (uint256) {
        uint256 value = treeNodes[_nodeIndex(level, position)];
        return value == 0 ? zeroes[level] : value;
    }

    function _nodeIndex(uint256 level, uint256 position) private pure returns (uint256) {
        return (uint256(1) << (TREE_DEPTH - level)) - 1 + position;
    }
}
