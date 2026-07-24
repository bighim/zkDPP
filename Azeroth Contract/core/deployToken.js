import fs from 'fs';
import path from 'path';
import url from 'url';
import sendTransaction from './sendTransaction';
import Constants from './constants';


const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const compiledPath = path.join(__dirname, '../compiled');

async function deployToken(web3, tokenPresetName, tokenConstructorArgs) { // args: [tokenName, tokenSym, (nft)baseURI];
    const compiledTokenPath = path.join(compiledPath, tokenPresetName);
    const abi = JSON.parse(
        fs.readFileSync(
            path.join(compiledTokenPath, 'abi.json'),
            'utf8'
        ));
    const bytecode = fs.readFileSync(path.resolve(compiledTokenPath, 'bytecode'), 'utf8');

    const deployCall = new web3.eth.Contract(abi).deploy({
        data: bytecode,
        arguments: tokenConstructorArgs,
    });
    const receipt = await sendTransaction(web3, deployCall, Constants.DEFAULT_DEPLOY_GAS);

    return receipt.contractAddress;
}

export default deployToken;