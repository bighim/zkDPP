import Web3 from 'web3';
import fs from 'fs';
import path from 'path';
import url from 'url';
import Keys from '../core/keys';
import sendTransaction from '../core/sendTransaction';
import Constants from '../core/constants';


const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const compiledPath = path.join(__dirname, '../compiled');
const ERC20_PRESET = 'ERC20PresetMinterPauser';
const ERC721_PRESET = 'ERC721PresetMinterPauserAutoId';
const compiledErc20Path = path.join(compiledPath, ERC20_PRESET);
const compiledErc721Path = path.join(compiledPath, ERC721_PRESET);

const erc20Abi = JSON.parse(
    fs.readFileSync(
        path.join(compiledErc20Path, 'abi.json'),
        'utf8'
    ));

const erc721Abi = JSON.parse(
    fs.readFileSync(
        path.join(compiledErc721Path, 'abi.json'),
        'utf8'
    ));

const addresses = Keys.addresses;

/**
 * Mint 'amount' ERC20 tokens to all accounts. 
 * @param {Object} web3
 * @param {string} tokenAddress 
 * @param {string | number} amount
 * @returns {Promise<boolean>} status of transaction
 */
async function mintFt(web3, tokenAddress, amount) {
    let success = true;

    for (let i = 0; i < addresses.length; i++) {
        const mintCall = new web3.eth.Contract(erc20Abi, tokenAddress).methods.mint(addresses[i], amount);
        const receipt = await sendTransaction(web3, mintCall, Constants.DEFAULT_MINT_GAS);
        success &&= receipt.status;
    }
    return success;
}

/**
 * Mint a ERC721 token to all accounts. 
 * @param {Object} web3
 * @param {string} tokenAddress 
 * @returns {Promise<boolean>} status of transaction
 */
async function mintNft(web3, tokenAddress) {
    let success = true;

    for (let i = 0; i < addresses.length; i++) {
        const mintCall = new web3.eth.Contract(erc721Abi, tokenAddress).methods.mint(addresses[i]);
        const receipt = await sendTransaction(web3, mintCall, Constants.DEFAULT_MINT_GAS);
        success &&= receipt.status;
    }
    return success;
}

const web3 = new Web3(process.argv[2]);
const args = process.argv.slice(3);
if (args.length === 2) {
    mintFt(web3, ...args).then(r => {
        if (r) {
            process.exit(0);
        } else {
            process.exit(-1);
        }
    });
}
else if (args.length === 1) {
    mintNft(web3, ...args).then(r => {
        if (r) {
            process.exit(0);
        } else {
            process.exit(-1);
        }
    });
}