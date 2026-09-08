// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {ZkDPPStatusLedger} from "../src/ZkDPPStatusLedger.sol";
import {IZkVerifier} from "../src/IZkVerifier.sol";
import {Poseidon2BLS12381} from "../src/generated/Poseidon2BLS12381.sol";
import {StatusUpdateVerifier} from "../src/StatusUpdateVerifier.sol";
import {StatusPrivateSpendVerifier} from "../src/StatusPrivateSpendVerifier.sol";

interface VmM6 {
    function readFile(string calldata) external view returns (string memory);
    function parseJsonBytes(string calldata, string calldata) external pure returns (bytes memory);
    function parseJsonUint(string calldata, string calldata) external pure returns (uint256);
    function prank(address) external;
    function expectRevert(bytes4) external;
}

contract M6AlwaysVerifier is IZkVerifier {
    function Verify(bytes calldata, uint256[] calldata) external pure returns (bool) {
        return true;
    }
}

contract M6EmptyStatusVerifier is IZkVerifier {
    uint256 immutable emptyRoot;

    constructor(uint256 root) {
        emptyRoot = root;
    }

    function Verify(bytes calldata, uint256[] calldata inputs) external view returns (bool) {
        return inputs.length > 1 && inputs[1] == emptyRoot;
    }
}

contract StatusEnforcementTest {
    VmM6 constant vm = VmM6(address(uint160(uint256(keccak256("hevm cheat code")))));
    address constant ISSUER = address(0x1001);
    address constant STATUS_AUTHORITY = address(0x5001);
    address constant OUTSIDER = address(0x6001);
    string m2;
    string m6;
    Poseidon2BLS12381 hasher;

    function setUp() public {
        m2 = vm.readFile("test/fixtures/m2-proofs.json");
        m6 = vm.readFile("test/fixtures/m6-proofs.json");
        hasher = new Poseidon2BLS12381();
    }

    function testActualStatusUpdateProofAndCurrentRootBinding() public {
        M6AlwaysVerifier always = new M6AlwaysVerifier();
        ZkDPPStatusLedger ledger = _ledger(
            address(always),
            address(new StatusPrivateSpendVerifier()),
            address(new StatusUpdateVerifier())
        );
        _seed(ledger);

        vm.prank(STATUS_AUTHORITY);
        ledger.updateStatus(
            _m6bytes(".features.status-update.proof"),
            1,
            _m2uint(".entries[1].commitment"),
            1,
            0,
            1,
            _m6uint(".features.status-update.publicInputs[2]")
        );
        require(
            ledger.noteStatusRoot() == _m6uint(".features.status-update.publicInputs[2]"),
            "status root"
        );

        vm.expectRevert(ZkDPPStatusLedger.InvalidProof.selector);
        ledger.exit(
            _m6bytes(".features.status-private-spend.proof"),
            _m6uint(".features.status-private-spend.publicInputs[0]"),
            _m6uint(".features.status-private-spend.publicInputs[2]")
        );
    }

    function testAuthorityBindingTransitionsAndAtomicity() public {
        M6AlwaysVerifier always = new M6AlwaysVerifier();
        M6EmptyStatusVerifier active = new M6EmptyStatusVerifier(_m6uint(".noteStatusRoot"));
        ZkDPPStatusLedger ledger = _ledger(address(active), address(active), address(always));
        _seed(ledger);
        uint256 cm = _m2uint(".entries[1].commitment");
        uint256 cmC = _m2uint(".entries[2].commitment");
        uint256 emptyRoot = ledger.EMPTY_STATUS_ROOT();
        uint256 frozenRoot = 111;

        vm.prank(OUTSIDER);
        vm.expectRevert(ZkDPPStatusLedger.NotStatusAuthority.selector);
        ledger.updateStatus(hex"01", 1, cm, 1, 0, 1, frozenRoot);

        vm.prank(STATUS_AUTHORITY);
        vm.expectRevert(ZkDPPStatusLedger.InvalidObjectBinding.selector);
        ledger.updateStatus(hex"01", 1, cm, 0, 0, 1, frozenRoot);

        vm.prank(STATUS_AUTHORITY);
        ledger.updateStatus(hex"01", 1, cm, 1, 0, 1, frozenRoot);
        require(ledger.noteStatusRoot() == frozenRoot, "freeze");

        uint256 currentNoteRoot = ledger.currentNoteRoot();
        vm.expectRevert(ZkDPPStatusLedger.InvalidProof.selector);
        ledger.exit(hex"01", currentNoteRoot, 9001);

        vm.prank(STATUS_AUTHORITY);
        ledger.updateStatus(hex"01", 1, cm, 1, 1, 0, emptyRoot);
        ledger.exit(hex"01", currentNoteRoot, 9001);
        require(ledger.noteNullifiers(9001), "unfreeze spend");

        vm.prank(STATUS_AUTHORITY);
        ledger.updateStatus(hex"01", 1, cmC, 2, 0, 1, 222);
        vm.prank(STATUS_AUTHORITY);
        ledger.updateStatus(hex"01", 1, cmC, 2, 1, 2, 333);
        vm.prank(STATUS_AUTHORITY);
        vm.expectRevert(ZkDPPStatusLedger.InvalidStatusTransition.selector);
        ledger.updateStatus(hex"01", 1, cmC, 2, 2, 0, 444);
        require(ledger.noteStatusRoot() == 333, "terminal atomicity");
    }

    function _ledger(address eventVerifier, address privateSpend, address updateVerifier)
        internal
        returns (ZkDPPStatusLedger)
    {
        M6AlwaysVerifier entryMock = new M6AlwaysVerifier();
        return new ZkDPPStatusLedger(
            STATUS_AUTHORITY,
            address(entryMock),
            privateSpend,
            eventVerifier,
            eventVerifier,
            eventVerifier,
            eventVerifier,
            eventVerifier,
            eventVerifier,
            updateVerifier,
            address(hasher)
        );
    }

    function _seed(ZkDPPStatusLedger ledger) internal {
        ledger.setEntryIssuer(ISSUER, true);
        for (uint256 i = 0; i < 3; i++) {
            vm.prank(ISSUER);
            ledger.entry(hex"01", _m2uint(string.concat(".entries[", _s(i), "].commitment")));
        }
    }

    function _m2uint(string memory key) internal view returns (uint256) {
        return vm.parseJsonUint(m2, key);
    }

    function _m6uint(string memory key) internal view returns (uint256) {
        return vm.parseJsonUint(m6, key);
    }

    function _m6bytes(string memory key) internal view returns (bytes memory) {
        return vm.parseJsonBytes(m6, key);
    }

    function _s(uint256 v) internal pure returns (string memory) {
        if (v == 0) return "0";
        if (v == 1) return "1";
        return "2";
    }
}
