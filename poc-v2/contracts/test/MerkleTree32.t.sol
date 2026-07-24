// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {MerkleTree32} from "../src/MerkleTree32.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";

contract MerkleTree32Test {
    Poseidon2BLS12381 internal poseidon;
    MerkleTree32 internal tree;

    function setUp() public {
        poseidon = new Poseidon2BLS12381();
        tree = new MerkleTree32(address(poseidon));
    }

    function testReferenceVectorsAndPath() public {
        require(
            poseidon.compress(0, 0) == 0x0440d4ee8370b0cd0098bbc48e4b57cce27b1dbdc977f6c1b45c00064e29e374, "compress00"
        );
        require(
            poseidon.compress(1, 2) == 0x23927976918cacff4850f47a852748216d73f6f4582093316723fc4481b6fb5f, "compress12"
        );
        tree.append(11);
        tree.append(22);
        tree.append(33);
        (uint256 root, uint256[] memory siblings) = tree.getPath(1);
        uint256 current = 22;
        uint256 index = 1;
        for (uint256 level = 0; level < siblings.length; level++) {
            current = index & 1 == 0
                ? poseidon.compress(current, siblings[level])
                : poseidon.compress(siblings[level], current);
            index >>= 1;
        }
        require(current == root, "path");
        require(tree.acceptedRoots(root), "accepted root");
    }
}
