// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

import {Poseidon2MerklePathVerifier as MerkleVerifier} from "../src/generated/poseidon2/merklepath/PlonkVerifier.sol";
import {Poseidon2EqVerifier as EqVerifier} from "../src/generated/poseidon2/eq/PlonkVerifier.sol";
import {Poseidon2AddVerifier as AddVerifier} from "../src/generated/poseidon2/add/PlonkVerifier.sol";
import {Poseidon2AndVerifier as AndVerifier} from "../src/generated/poseidon2/and/PlonkVerifier.sol";
import {SahaiLedger} from "../src/SahaiLedger.sol";

interface VmLedger {
    function readFile(string calldata path) external view returns (string memory);
    function parseJsonBytes(string calldata json, string calldata key) external pure returns (bytes memory);
    function parseJsonBytes32(string calldata json, string calldata key) external pure returns (bytes32);
    function parseJsonBool(string calldata json, string calldata key) external pure returns (bool);
    function parseJsonString(string calldata json, string calldata key) external pure returns (string memory);
    function parseJsonUintArray(string calldata json, string calldata key) external pure returns (uint256[] memory);
    function toString(uint256 value) external pure returns (string memory);
}

contract TestableSahaiLedger is SahaiLedger {
    constructor(address merkle, address eq, address add, address andVerifier)
        SahaiLedger(1, merkle, eq, add, andVerifier) {}
    function seedForTest(AssetData calldata asset) external {
        bytes32[] memory empty = new bytes32[](0); _store(asset, empty);
    }
    function loadInputForTest(bytes32 id) external view returns (bytes32) {
        return _loadInput(id).docHash;
    }
}

contract UnauthorizedCaller {
    function callEntry(SahaiLedger ledger, SahaiLedger.AssetData calldata output, SahaiLedger.GadgetProof[] calldata proofs) external {
        ledger.entry(output, proofs);
    }
}

contract SahaiLedgerTest {
    VmLedger private constant vm = VmLedger(address(uint160(uint256(keccak256("hevm cheat code")))));
    string private fixture;
    address private merkle;
    address private eq;
    address private add;
    address private andVerifier;

    function setUp() public {
        fixture = vm.readFile("../testdata/generated/canonical-poseidon2-st-payloads.json");
        merkle = address(new MerkleVerifier()); eq = address(new EqVerifier());
        add = address(new AddVerifier()); andVerifier = address(new AndVerifier());
    }

    function testConnectedCanonicalScenarioAndProvenance() public {
        TestableSahaiLedger ledger = _ledger();
        string[8] memory cases = ["entry_a", "ship_a", "split_a", "merge_a", "entry_b", "ship_b", "process_product", "exit_product"];
        for (uint256 i = 0; i < cases.length; i++) {
            _execute(ledger, cases[i], false);
            _assertOutputsAndInDocs(ledger, cases[i]);
        }
        require(ledger.isConsumed(_inputId("exit_product", 0)), "Exit input was not consumed");
        require(_asset("exit_product", "outputs", 0).terminal, "Exit output not terminal");
    }

    function testDuplicateAndConsumedInputsRejected() public {
        TestableSahaiLedger ledger = _ledger();
        _execute(ledger, "entry_a", false);
        try ledger.entry(_asset("entry_a", "outputs", 0), _proofs("entry_a", 2)) { revert("duplicate accepted"); } catch {}
        _execute(ledger, "ship_a", false);
        try ledger.ship(_inputId("ship_a", 0), _asset("ship_a", "outputs", 0), _proofs("ship_a", 5)) { revert("consumed input accepted"); } catch {}
        require(ledger.isConsumed(_inputId("ship_a", 0)), "input not consumed");
    }

    function testInvalidSplitProofRevertsAtomically() public {
        TestableSahaiLedger ledger = _ledger(); _execute(ledger, "entry_a", false); _execute(ledger, "ship_a", false);
        bytes32 inputId = _inputId("split_a", 0);
        _execute(ledger, "split_a", true);
        require(!ledger.isConsumed(inputId), "reverted input consumed");
        require(!ledger.getDocument(_outputId("split_a", 0)).exists, "partial output 0");
        require(!ledger.getDocument(_outputId("split_a", 1)).exists, "partial output 1");
    }

    function testTerminalInputAndUnauthorizedCallerRejected() public {
        TestableSahaiLedger ledger = _ledger();
        SahaiLedger.AssetData memory terminal = _asset("exit_product", "outputs", 0);
        ledger.seedForTest(terminal);
        try ledger.loadInputForTest(terminal.docHash) { revert("terminal input accepted"); } catch {}
        UnauthorizedCaller caller = new UnauthorizedCaller();
        try caller.callEntry(ledger, _asset("entry_a", "outputs", 0), _proofs("entry_a", 2)) { revert("unauthorized caller accepted"); } catch {}
    }

    function _execute(TestableSahaiLedger ledger, string memory name, bool corrupt) private {
        string memory eventName = _string(name, ".event");
        uint256 count = _proofCount(eventName);
        SahaiLedger.GadgetProof[] memory proofs = _proofs(name, count);
        if (corrupt) proofs[0].proof[0] = bytes1(uint8(proofs[0].proof[0]) ^ 1);
        bytes32 eventHash = keccak256(bytes(eventName));
        if (eventHash == keccak256("Entry")) {
            if (corrupt) { try ledger.entry(_asset(name,"outputs",0),proofs) { revert("bad Entry accepted"); } catch {} }
            else ledger.entry(_asset(name,"outputs",0),proofs);
        } else if (eventHash == keccak256("Ship")) {
            if (corrupt) { try ledger.ship(_inputId(name,0),_asset(name,"outputs",0),proofs) { revert("bad Ship accepted"); } catch {} }
            else ledger.ship(_inputId(name,0),_asset(name,"outputs",0),proofs);
        } else if (eventHash == keccak256("Split")) {
            SahaiLedger.AssetData[2] memory outputs = [_asset(name,"outputs",0),_asset(name,"outputs",1)];
            if (corrupt) { try ledger.split(_inputId(name,0),outputs,proofs) { revert("bad Split accepted"); } catch {} }
            else ledger.split(_inputId(name,0),outputs,proofs);
        } else if (eventHash == keccak256("Merge") || eventHash == keccak256("Process")) {
            bytes32[2] memory inputs = [_inputId(name,0),_inputId(name,1)];
            if (eventHash == keccak256("Merge")) ledger.merge(inputs,_asset(name,"outputs",0),proofs);
            else ledger.process(inputs,_asset(name,"outputs",0),proofs);
        } else if (eventHash == keccak256("Exit")) {
            ledger.exit(_inputId(name,0),_asset(name,"outputs",0),proofs);
        } else revert("unknown event");
    }

    function _assertOutputsAndInDocs(TestableSahaiLedger ledger, string memory name) private view {
        string memory eventName = _string(name, ".event"); uint256 outputCount = keccak256(bytes(eventName)) == keccak256("Split") ? 2 : 1;
        uint256 inputCount = keccak256(bytes(eventName)) == keccak256("Entry") ? 0 : (keccak256(bytes(eventName)) == keccak256("Merge") || keccak256(bytes(eventName)) == keccak256("Process") ? 2 : 1);
        for (uint256 o=0;o<outputCount;o++) { SahaiLedger.DocumentAsset memory stored=ledger.getDocument(_outputId(name,o));require(stored.exists,"missing output");require(stored.inDocs.length==inputCount,"InDocs count");for(uint256 i=0;i<inputCount;i++)require(stored.inDocs[i]==_inputId(name,i),"InDocs link"); }
    }
    function _ledger() private returns (TestableSahaiLedger) { return new TestableSahaiLedger(merkle,eq,add,andVerifier); }
    function _asset(string memory name,string memory direction,uint256 index) private view returns(SahaiLedger.AssetData memory a){string memory base=string.concat(".cases.",name,".",direction,"[",vm.toString(index),"]");a.documentType=keccak256(bytes(vm.parseJsonString(fixture,string.concat(base,".type"))));a.docHash=vm.parseJsonBytes32(fixture,string.concat(base,".docHash"));a.terminal=vm.parseJsonBool(fixture,string.concat(base,".terminal"));}
    function _proofs(string memory name,uint256 count) private view returns(SahaiLedger.GadgetProof[] memory p){p=new SahaiLedger.GadgetProof[](count);for(uint256 i=0;i<count;i++){string memory base=string.concat(".cases.",name,".proofs[",vm.toString(i),"]");p[i]=SahaiLedger.GadgetProof(vm.parseJsonBytes(fixture,string.concat(base,".proof")),vm.parseJsonUintArray(fixture,string.concat(base,".publicInputs")));}}
    function _inputId(string memory name,uint256 index) private view returns(bytes32){return vm.parseJsonBytes32(fixture,string.concat(".cases.",name,".inputs[",vm.toString(index),"].docHash"));}
    function _outputId(string memory name,uint256 index) private view returns(bytes32){return vm.parseJsonBytes32(fixture,string.concat(".cases.",name,".outputs[",vm.toString(index),"].docHash"));}
    function _string(string memory name,string memory suffix) private view returns(string memory){return vm.parseJsonString(fixture,string.concat(".cases.",name,suffix));}
    function _proofCount(string memory eventName) private pure returns(uint256){bytes32 h=keccak256(bytes(eventName));if(h==keccak256("Entry")||h==keccak256("Exit"))return 2;if(h==keccak256("Ship"))return 5;return 6;}
}
