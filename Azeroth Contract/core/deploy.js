import fs from 'fs';
import path from 'path';
import url from 'url';
import sendTransaction from './sendTransaction';
import Constants from './constants';


const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const compiledPath = path.join(__dirname, '../compiled');
const CONTRACT_NAME = 'Groth16AltBN128Mixer';
const compiledContractPath = path.join(compiledPath, CONTRACT_NAME);

async function deploy(web3) {
    const testParameter = JSON.parse(
        fs.readFileSync(
            path.join(compiledPath, 'testParameter.json'),
            'utf8',
        ));
    const abi = JSON.parse(
        fs.readFileSync(
            path.join(compiledContractPath, 'abi.json'),
            'utf8'
        ));
    const bytecode = fs.readFileSync(path.resolve(compiledContractPath, 'bytecode'), 'utf8');

    const args = [32, testParameter.vk, testParameter.vk_nft];
    const deployCall = new web3.eth.Contract(abi).deploy({
        data: bytecode,
        arguments: args,
    });
    const receipt = await sendTransaction(web3, deployCall, Constants.DEFAULT_DEPLOY_GAS);

    return receipt.contractAddress;
}

export default deploy;