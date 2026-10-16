// SPDX-License-Identifier: Apache-2.0
// Code generated from gnark-crypto v0.20.1 BLS12-381 Poseidon2 parameters. DO NOT EDIT.
pragma solidity ^0.8.30;

contract Poseidon2BLS12381 {
    uint256 internal constant MODULUS = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001;

    function _pow5(uint256 value) private pure returns (uint256) {
        uint256 squared = mulmod(value, value, MODULUS);
        uint256 fourth = mulmod(squared, squared, MODULUS);
        return mulmod(fourth, value, MODULUS);
    }

    function compress(uint256 left, uint256 right) external pure returns (uint256) {
        require(left < MODULUS && right < MODULUS, "non-canonical field element");
        uint256 x0 = left;
        uint256 x1 = right;
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x30ad66715875b7f8f573a3fccb0239fe6f3835ecd52d8475ea97cf0515d8067b, MODULUS);
        x1 = addmod(x1, 0x5c3023f1195e979990cefb861e7c6dd5695b824302b4a36341231648572a331c, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x26da63e9fd39692199b152e604858e6086ac3dbe2099951fb2bbb80a6399d024, MODULUS);
        x1 = addmod(x1, 0x0349c6d329bcd5d3daf38c4d4baadda1ccaa7afc91d2610edef470c131344599, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x3460284102aa67ecf48ac45309aceeeccb057c512fc68257294987b6e21f6b99, MODULUS);
        x1 = addmod(x1, 0x4108225ffbd2d84fd7021b5ab6718b58fc276b71083d9b54a864a10000afd639, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x53f2b48552da5bfb9f29407e07620c27ee8048729a867db40af6df6a44333b34, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x27704e1a76333693596cdc706b642d73066aec48cccb13da6f76edee960bd205, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x19b27d25952730231378b8390ae5a6eb70122b6f8e9c0b9461c241c80457e959, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x46f8789105e704b75d73fb568bbb6869dcf4945fe0478d2889555968e3842323, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x16e684b282e982ae970e43db1276c4bd63be88cda96b29d1ae29347ade65f6a5, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x0c24f2fa02fe7ace1f5761ffacbfd26daf7d962a7ca5b01742b9519d2b07ec63, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x6166b462619b79e26f0ca9da544d3a58ae4e69b590482b52e2920b7995383e53, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x2b8eb86a73b9dfa65b56cd38345db38130b74df9f2312aacf52bee6a5d9a61a8, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x72e69e109aea15ef4c251d59790d527b4411d79b9fba48385b90f978a0810721, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x20c4222f8e997a4d334a93c19f0d61b74d9ea5116472974f7a6b050861952458, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x16f7969c01ad7ecd176845838acc716c294c2fb5062f8fbf41d372d10c5c4fcc, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x6e96fd5e18290eb98182bfed169d8e289a81387f735a9237b5e6521613a01f6e, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x1fdbbbe12312edc895bb61519f8d28eca7a6e0bd19d2ba7b53bee02f92e340cd, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x1def9d3c9d3445256697646d7bba088d9e32d18aaec9f3c4ea35bc594d3484d5, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x68ffdf30f948da034925398b4304a00675403102916076b3941a560994b7f475, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x28faaa69eac9f4be0d19af1bc9f07cf7b0db697fbe79933785905e576e589830, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x266769123a962f8743c328ae208c7422d2580108d20234ffc7a2a39f19f24bb0, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x18f7cea8a1cb110e28be90779966cc2047321f4e0e2b2234ec4e01e44a30f197, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x4d13ffa902e5b02a23974e0129a2cf9c675c1eafc4ecfe5462c2c5f2d93aa012, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x5f6dbddf53766bd0b171ae058514437e068b4c01d251f1a90113663ac2e2397e, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x220c7a998104a1d62c7da2350efa8dfc8803fbe42be8a91056e088518bb27e93, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x1bcfeb3170ca2c8f34b0d08da77607df297b1a9f1e85bdb33542328a1e0d1af2, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x0bf4759c30b67df1f7d842481cd432855222e3c57d056c50ee4d3300ed1dfac3, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x6347b26be65ba6fa175ea02b99b26c94dd208bb4e40ea10edd57e4aadf8b8bbf, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x4a4ce49fcfc9ca9933f2688d3b8663e5fa00421676e441e9796cefc8b08ef6f4, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x31f1b7c6160908610385626d16e6334ac28d9f33a68ed96f8abbed6ac5f5f01d, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x5ef6b4843259965bd68234e198bd12369d15653b3690429038ff802dcbb878ae, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x051f7af507ebdacee937f9fdc2b06be349a534cf2caafc4fa759cda1a47b6efa, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x4e14eddbfb582b0cfee8712543db4cff872f07298c8db6b5ccec0303a1b315d2, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x0d85b675e0c4bf8fd5837f4bf620879b3a87083fa66b7979f583186efd63ea26, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x3903d84d866ef61d6bfa81c5d8fcdab07af31f0ef0eb01df1f608331ddff446c, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x0a2c6782aa38f553c6c84550fa70884f6bb66a6e8f3999ed8cafbad1354d5a5b, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x6fb7cb0b5d690d5319b9733263dda9ed8ce1656a0a1265ae8118fb75f506d43f, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x6867f41eb71a34dea00ce871f5f631bcadbf4ed61fd741c666bc2cb99e3d11bd, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x49f2316cd73e6a1aa601f1421f7b08452ec61beb6a3df02fe8d53966ddea97f4, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x5457329c2be450225059922e37a6c04c3caf6f6b4765c4c27de9cd8eae833e5b, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x117fb863e88f008a50f758f06345e45901e323262bdf9f8aaf0ab790c99a1332, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x458d6ccb0c716dd329421608ec011251ab94c811ad6e899bac75933865d7eabf, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x1dcea749bcfee2fbeda12c37c790e707b58e10de2f23000a01d60543895840a6, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x172352d22248dcbc908cac24ed901072ed19cfad992235c1abaa9fc54e011bb8, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x46dec23da655a456bac8785c75ee251c86153dd3de0ab8f458bb8d390147c8a5, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x71aa7ac592aaddd755aec1c692af66549de386366434feedf75fadf0687d09da, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x53e54d01d3f3df674ed77e51cb2809f5ecb27e3bf429c646b069266bacad6419, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x42854f2d7dc19a539d8cdc0ed589cacacf1e4f1e6b666c5ab4a33fd2c1465fbe, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x5e4b94cb8d183e084194ffdcdd64c85565dd931baca4ae4e381af83e4f85f4d3, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x5f0f99209e028924c579fc2ab3337065b7e11b15179b775ee29438075ff7b7af, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x456f93d438b421dd85642a2236b9f23635e5f9067918b6f217e8bd62c6196158, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x12bb7020ddf0651239e2135468054a3bc6d9f2036c6b5a51983b2283a6a7320c, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x40984dc1779c6aa86643f63d32c4e6ef86fab8b55dfed98de14f1f3395ebc75c, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x02a1c6ef1e05dd1111c382e4bfeb6e44fa887b2c4be934df0fa3fe7248de0cfb, MODULUS);
        x0 = _pow5(x0);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            uint256 next0 = addmod(x0, sum, MODULUS);
            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);
            x0 = next0;
            x1 = next1;
        }
        x0 = addmod(x0, 0x47d4c4842b518b5af97b8f856ae557e9105f7c4e7036e43b35bf76abc7dbd54f, MODULUS);
        x1 = addmod(x1, 0x10392a6a40f9e59b521152d150372545f380c163885b13d910c4238a6eb0321b, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x28f4d5d4d89a5b0d6da546a734bb1aca2f2409694940ce52e4cac81e91a534b6, MODULUS);
        x1 = addmod(x1, 0x053f8faaa5978246a87081143d59fef9cb70032540fd19f729b83425f313e8cb, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        x0 = addmod(x0, 0x3d97c5422e9684be7f6f6273dfcb085b5dc723fa18bac3d113fef9a218b914b6, MODULUS);
        x1 = addmod(x1, 0x6dc0d4ce6b24d2dc26e73435cf85aa6e9c093a5776d1a50dfe99aa162c6572b4, MODULUS);
        x0 = _pow5(x0);
        x1 = _pow5(x1);
        {
            uint256 sum = addmod(x0, x1, MODULUS);
            x0 = addmod(sum, x0, MODULUS);
            x1 = addmod(sum, x1, MODULUS);
        }
        return addmod(x1, right, MODULUS);
    }
}
