// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {Poseidon2MerklePathVerifier as PoseidonMerkle} from "../src/generated/poseidon2/merklepath/PlonkVerifier.sol";
import {Poseidon2EqVerifier as PoseidonEq} from "../src/generated/poseidon2/eq/PlonkVerifier.sol";
import {Poseidon2AddVerifier as PoseidonAdd} from "../src/generated/poseidon2/add/PlonkVerifier.sol";
import {Poseidon2AndVerifier as PoseidonAnd} from "../src/generated/poseidon2/and/PlonkVerifier.sol";
import {SHA256MerklePathVerifier as SHAMerkle} from "../src/generated/sha256/merklepath/PlonkVerifier.sol";
import {SHA256EqVerifier as SHAEq} from "../src/generated/sha256/eq/PlonkVerifier.sol";
import {SHA256AddVerifier as SHAAdd} from "../src/generated/sha256/add/PlonkVerifier.sol";
import {SHA256AndVerifier as SHAAnd} from "../src/generated/sha256/and/PlonkVerifier.sol";

interface VmGadget {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonUintArray(string calldata json, string calldata key) external pure returns (uint256[] memory);
}
interface IGadgetVerifier { function Verify(bytes calldata proof, uint256[] calldata inputs) external view returns (bool); }

contract GadgetVerifierTest {
    VmGadget private constant vm = VmGadget(address(uint160(uint256(keccak256("hevm cheat code")))));
    string private poseidonFixture;
    string private shaFixture;

    function setUp() public {
        poseidonFixture = vm.readFile("test/fixtures/poseidon2-gadget-proofs.json");
        shaFixture = vm.readFile("test/fixtures/sha256-gadget-proofs.json");
    }

    function testPoseidon2AndSHA256Gadgets() public {
        _accept(IGadgetVerifier(address(new PoseidonMerkle())), poseidonFixture, "merklepath");
        _accept(IGadgetVerifier(address(new PoseidonEq())), poseidonFixture, "eq");
        _accept(IGadgetVerifier(address(new PoseidonAdd())), poseidonFixture, "add");
        _accept(IGadgetVerifier(address(new PoseidonAnd())), poseidonFixture, "and");
        _accept(IGadgetVerifier(address(new SHAMerkle())), shaFixture, "merklepath");
        _accept(IGadgetVerifier(address(new SHAEq())), shaFixture, "eq");
        _accept(IGadgetVerifier(address(new SHAAdd())), shaFixture, "add");
        _accept(IGadgetVerifier(address(new SHAAnd())), shaFixture, "and");
    }

    function testCrossProfileProofsRejected() public {
        _reject(IGadgetVerifier(address(new SHAEq())), poseidonFixture, "eq");
        _reject(IGadgetVerifier(address(new PoseidonEq())), shaFixture, "eq");
    }

    function _accept(IGadgetVerifier verifier, string memory fixture, string memory name) private view {
        (bytes memory proof, uint256[] memory inputs) = _read(fixture, name);
        require(verifier.Verify(proof, inputs), string.concat(name, " proof rejected"));
    }
    function _reject(IGadgetVerifier verifier, string memory fixture, string memory name) private view {
        (bytes memory proof, uint256[] memory inputs) = _read(fixture, name);
        try verifier.Verify(proof, inputs) returns (bool valid) { require(!valid, "cross-profile proof accepted"); } catch {}
    }
    function _read(string memory fixture, string memory name) private view returns (bytes memory, uint256[] memory) {
        string memory base = string.concat(".gadgets.", name);
        return (vm.parseJsonBytes(fixture, string.concat(base, ".proof")), vm.parseJsonUintArray(fixture, string.concat(base, ".publicInputs")));
    }
}
