// Copyright (c) 2015-2020 Clearmatics Technologies Ltd
// Copyright (c) 2021-2021 Zkrypto Inc.
// SPDX-License-Identifier: LGPL-3.0+

pragma solidity ^0.8.2;

import './Tokens.sol';
import './BaseMerkleTree.sol';
import './Ownable.sol';

/// MixerBase implements the functions shared across all Mixers (regardless
/// which zkSNARK is used)
abstract contract ZklayBase is
    BaseMerkleTree,
    ERC223ReceivingContract,
    Ownable
{
    struct ENA {
        uint256 r;
        uint256 ct;
    }

    struct AddressMap {
        uint256 Addr;
        uint256 PkOwn;
        uint256 PkEnc;
    }

    // The roots of the different updated trees
    mapping(uint256 => bool) private _roots;

    // The public list of nullifiers (prevents double spend)
    mapping(uint256 => bool) private _nullifiers;

    // The auditor's public key
    uint256 private _APK;

    // The Most recent root of the Merkle tree
    uint256 private _rootTop;

    // The public list of user address
    mapping(uint256 => bool) private _addrList;

    // List of mapping addresses corresponding to user EOA
    mapping(address => AddressMap) private _addressMap;

    // The public list of users' encrypted accounts
    mapping(address => mapping(uint256 => ENA)) private _ENA;

    // Structure of the verification key and proofs is opaque, determined by
    // zk-snark verification library.
    uint256[] internal _vk;
    uint256[] internal _vkNft;

    // Registration status of fungible token address
    mapping(address => bool) private _tokens;

    // The number of inputs for a zk-SNARK proof
    uint256 internal constant _NUM_INPUTS = 20;

    // The unit used for public values (ether in and out), in Wei.
    // Must match the python wrappers.
    uint64 private constant _PUBLIC_UNIT_VALUE_WEI = 1;

    // The unit used for public values (ether in and out), in Ether.
    // Must match the python wrappers.
    uint64 private constant _PUBLIC_UNIT_VALUE_ETHER =
        _PUBLIC_UNIT_VALUE_WEI * (10**18);

    event LogZkTransfer(
        uint256 nullifier,
        uint256 com,
        uint256[6] ct,
        uint256 index,
        address tokenAddress
    );

    event LogZkTransferNft(
        uint256 nullifier,
        uint256 com,
        uint256[6] ct,
        uint256 index
    );

    event LogUserRegister(uint256 addr, uint256 pkOwn, uint256 pkEnc);

    event LogNFT(address tokenAddress, uint88 tokenId);

    /// Constructor
    constructor(
        uint256 depth,
        uint256[] memory vk,
        uint256[] memory vkNft
    ) BaseMerkleTree(depth) {
        uint256 initialRoot = uint256(_nodes[0]);
        _roots[initialRoot] = true;
        _rootTop = initialRoot;
        _vk = vk;
        _vkNft = vkNft;
        _tokens[address(0)] = true; // register 'Ethereum'
    }

    modifier verifyInputs(uint256 root, uint256 nullifier) {
        // 1. Check the auditor key.
        require(_APK != 0, 'APK does not exist');

        // 2. Check the root and the nullifiers.
        require(_roots[root], 'This root is not valid');

        require(
            !_nullifiers[nullifier],
            'This nullifier has already been used'
        );
        _;
    }

    modifier registeredToken(address tokenAddress) {
        require(_tokens[tokenAddress], 'Token does not exist');
        _;
    }

    function isNullified(uint256 nf) public view returns (bool) {
        return _nullifiers[nf];
    }

    function getCiphertext(address tokenAddress, uint256 addr)
        public
        view
        registeredToken(tokenAddress)
        returns (uint256, uint256)
    {
        require(_addrList[addr], 'The user does not exist');

        return (_ENA[tokenAddress][addr].ct, _ENA[tokenAddress][addr].r);
    }

    function getAPK() public view returns (uint256) {
        require(_APK != uint256(0), 'APK does not exist');

        return _APK;
    }

    function getUserPublicKeys(address eoa)
        public
        view
        returns (
            uint256,
            uint256,
            uint256
        )
    {
        return (
            _addressMap[eoa].Addr,
            _addressMap[eoa].PkOwn,
            _addressMap[eoa].PkEnc
        );
    }

    function getRootTop() public view returns (uint256) {
        return _rootTop;
    }

    function getMerklePath(uint256 index)
        public
        view
        returns (uint256[] memory)
    {
        bytes32[] memory merklePathBytes = _computeMerklePath(index);
        uint256[] memory merklePath = new uint256[](_DEPTH);

        //TODO: need conversion?
        for (uint256 i = 0; i < merklePathBytes.length; i++) {
            merklePath[i] = uint256(merklePathBytes[i]);
        }

        return merklePath;
    }

    function registerToken(address tokenAddress) public onlyOwner {
        require(tokenAddress != address(0), 'Cannot register address of zero');
        require(!_tokens[tokenAddress], 'Token already exists');
        _tokens[tokenAddress] = true;
    }

    function registerUser(
        uint256 addr,
        uint256 pkOwn,
        uint256 pkEnc
    ) public {
        require(!_addrList[addr], 'User already exist');
        _addrList[addr] = true;

        _addressMap[msg.sender].Addr = addr;
        _addressMap[msg.sender].PkOwn = pkOwn;
        _addressMap[msg.sender].PkEnc = pkEnc;

        emit LogUserRegister(addr, pkOwn, pkEnc);
    }

    function registerAuditor(uint256 apk) public onlyOwner {
        require(_APK == uint256(0), 'APK already exists');
        _APK = apk;
    }

    function zkTransfer(
        uint256[] memory proof,
        uint256[] memory inputs,
        address toEoA,
        address tokenAddress
    )
        public
        payable
        registeredToken(tokenAddress)
        verifyInputs(inputs[0], inputs[1])
    {
        uint256 addr = inputs[2];
        // Check the user address is in the list.
        require(
            _addrList[addr],
            'Invalid User: The user isn`t in the user list'
        );

        ENA memory ena = _ENA[tokenAddress][addr];
        uint256[] memory states = new uint256[](3);
        states[0] = _APK;
        states[1] = ena.r;
        states[2] = ena.ct;

        require(
            _verifyZKProof(
                proof,
                _assembleZKInputsWithStates(states, inputs),
                false
            ),
            'Invalid proof: Unable to verify the proof correctly'
        );

        // Compute a new merkle root, Update root_list.
        // rt' <- add_and_update(commit_list, c')
        // root_list.append(rt')
        _insert(bytes32(inputs[5]));
        uint256 new_merkle_root = uint256(_recomputeRoot(1));
        _addRoot(new_merkle_root);

        // Update a nullifier list by appending a new nf.
        // nf_list.append(nf)
        _nullifiers[inputs[1]] = true;

        uint256[6] memory cipherText = [
            inputs[10],
            inputs[11],
            inputs[12],
            inputs[13],
            inputs[14],
            inputs[15]
        ];
        emit LogZkTransfer(
            inputs[1],
            inputs[5],
            cipherText,
            BaseMerkleTree._numLeaves,
            tokenAddress
        );

        _processPublicValues([inputs[8], inputs[9]], toEoA, tokenAddress);

        // 10. Update a ciphertext of ENA as follows.
        // ENA[addr] <- ct'
        _ENA[tokenAddress][addr] = ENA(inputs[6], inputs[7]);
    }

    function zkTransferNft(
        uint256[] memory proof,
        uint256[] memory inputs, // 0~13
        address toEoA
    ) public payable verifyInputs(inputs[0], inputs[1]) {
        uint256[] memory states = new uint256[](1);
        states[0] = _APK;

        require(
            _verifyZKProof(
                proof,
                _assembleZKInputsWithStates(states, inputs),
                true
            ),
            'Invalid proof: Unable to verify the proof correctly'
        );

        // Compute a new merkle root, Update root_list.
        // rt' <- add_and_update(commit_list, c')
        // root_list.append(rt')
        _insert(bytes32(inputs[5]));
        uint256 new_merkle_root = uint256(_recomputeRoot(1));
        _addRoot(new_merkle_root);

        // Update a nullifier list by appending a new nf.
        // nf_list.append(nf)
        _nullifiers[inputs[1]] = true;

        uint256[6] memory cipherText = [
            inputs[8],
            inputs[9],
            inputs[10],
            inputs[11],
            inputs[12],
            inputs[13]
        ];
        emit LogZkTransferNft(
            inputs[1],
            inputs[5],
            cipherText,
            BaseMerkleTree._numLeaves
        );

        _processPublicNFT([inputs[6], inputs[7]], toEoA);
    }

    function _assembleZKInputsWithStates(
        uint256[] memory states,
        uint256[] memory inputs
    ) private pure returns (uint256[] memory) {
        // Define statement including APK and constant 'one' which generated from Jsnark.
        uint256 statementLength = 1 + inputs.length + states.length;

        uint256[] memory statements = new uint256[](statementLength);
        statements[0] = 1;

        for (uint256 i = 0; i < states.length; i++) {
            statements[i + 1] = states[i];
        }
        for (uint256 i = 0; i < inputs.length; i++) {
            statements[i + 1 + states.length] = inputs[i];
        }

        return statements;
    }

    // Implementations must implement the verification algorithm of the
    // selected SNARK.
    function _verifyZKProof(
        uint256[] memory proof,
        uint256[] memory inputs,
        bool isNft
    ) internal virtual returns (bool);

    function _addRoot(uint256 rt) private {
        _roots[rt] = true;
        _rootTop = rt;
    }

    function _processPublicValues(
        uint256[2] memory inputs,
        address EOA,
        address tokenAddress
    ) private {
        uint256 vpubIn = inputs[0];
        uint256 vpubOut = inputs[1];

        // If vpubIn is > 0, we need to make sure that right amount is paid
        if (vpubIn > 0) {
            if (tokenAddress != address(0)) {
                ERC20 erc20Token = ERC20(tokenAddress);
                erc20Token.transferFrom(msg.sender, address(this), vpubIn);
                if (msg.value > 0) {
                    (bool success, ) = msg.sender.call{value: msg.value}('');
                    require(success, 'vpubIn return transfer failed');
                }
            } else {
                vpubIn *= _PUBLIC_UNIT_VALUE_WEI;
                require(
                    msg.value == vpubIn,
                    'Wrong msg.value: Value paid is not correct'
                );
            }
        } else {
            // If vpubIn = 0, return incoming Ether to the caller
            if (msg.value > 0) {
                (bool success, ) = msg.sender.call{value: msg.value}('');
                require(success, 'vpubIn return transfer failed');
            }
        }

        // If vpubOut > 0 then we do a withdraw. We retrieve the
        // msg.sender and send him the appropriate value If proof is valid
        if (vpubOut > 0) {
            if (tokenAddress != address(0)) {
                ERC20 erc20Token = ERC20(tokenAddress);
                erc20Token.transfer(EOA, vpubOut);
            } else {
                vpubOut *= _PUBLIC_UNIT_VALUE_WEI;
                payable(EOA).transfer(vpubOut);
            }
        }
    }

    function _processPublicNFT(uint256[2] memory inputs, address EOA) private {
        uint256 nftPubIn = inputs[0];
        uint256 nftPubOut = inputs[1];

        if (nftPubIn != 0) {
            (address tokenAddress, uint88 tokenId) = deserializeTokenId(
                nftPubIn
            );
            ERC721 token = ERC721(tokenAddress);
            emit LogNFT(tokenAddress, tokenId);
            token.transferFrom(msg.sender, address(this), tokenId);
        }

        if (nftPubOut != 0) {
            (address tokenAddress, uint88 tokenId) = deserializeTokenId(
                nftPubOut
            );
            ERC721 token = ERC721(tokenAddress);
            emit LogNFT(tokenAddress, tokenId);
            token.transferFrom(address(this), EOA, tokenId);
        }
    }

    function deserializeTokenId(uint256 serializedId)
        private
        pure
        returns (address, uint88)
    {
        address tokenAddress = address(uint160(serializedId >> 88));
        uint88 tokenId = uint88(serializedId);

        return (tokenAddress, tokenId);
    }
}
