import Web3 from 'web3';
import fs from 'fs';
import path from 'path';
import url from 'url';
import Keys from '../core/keys';
import sendTransaction from '../core/sendTransaction';


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
const privateKeys = Keys.privateKeys;

/**
 * Approve 'amount' ERC20 tokens to 'spender' 
 * @param {Object} web3
 * @param {string} tokenAddress 
 * @param {string} spender
 * @param {string} amount
 * @param {string | number} amount
 * @returns {Promise<boolean>} status of transaction
 */
async function approveFt(web3, tokenAddress, spender, amount) {
    let success = true;

    for (let i = 0; i < addresses.length; i++) {
        const approveCall = new web3.eth.Contract(erc20Abi, tokenAddress).methods.approve(spender, amount);
        const receipt = await sendTransaction(
            web3,
            approveCall,
            undefined,
            addresses[i],
            privateKeys[i]
        );
        success &&= receipt.status;
    }
    return success;
}

/**
 * Approve all ERC721 tokens to 'operator' 
 * @param {Object} web3
 * @param {string} tokenAddress 
 * @param {string} operator
 * @returns {Promise<boolean>} status of transaction
 */
async function approveNft(web3, tokenAddress, operator) {
    let success = true;

    for (let i = 0; i < addresses.length; i++) {
        const approveCall = new web3.eth.Contract(erc721Abi, tokenAddress).methods.setApprovalForAll(operator, true);
        const receipt = await sendTransaction(
            web3,
            approveCall,
            undefined,
            addresses[i],
            privateKeys[i]
        );
        success &&= receipt.status;
    }
    return success;
}

const web3 = new Web3(process.argv[2]);
const args = process.argv.slice(3);
if (args.length === 3) {
    approveFt(web3, ...args).then(r => {
        if (r) {
            process.exit(0);
        } else {
            process.exit(-1);
        }
    });
}
else if (args.length === 2) {
    approveNft(web3, ...args).then(r => {
        if (r) {
            process.exit(0);
        } else {
            process.exit(-1);
        }
    });
}