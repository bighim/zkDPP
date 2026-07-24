import Web3 from 'web3';
import fs from 'fs';
import path from 'path';
import url from 'url';
import sendTransaction from '../core/sendTransaction';
import Constants from '../core/constants';


const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const compiledPath = path.join(__dirname, '../compiled');
const CONTRACT_NAME = 'Groth16AltBN128Mixer';
const compiledFilePath = path.join(compiledPath, CONTRACT_NAME);

const testParameter = JSON.parse(
    fs.readFileSync(
        path.join(compiledPath, 'testParameter.json'),
        'utf8',
    ));
const abi = JSON.parse(
    fs.readFileSync(
        path.join(compiledFilePath, 'abi.json'),
        'utf8'
    ));

async function registerAuditor(web3, address) {
    const registerAuditorCall = new web3.eth.Contract(abi, address).methods.registerAuditor(testParameter.apk);
    const receipt = await sendTransaction(web3, registerAuditorCall, Constants.DEFAULT_REGISTER_GAS);

    return receipt.status;
}

const web3 = new Web3(process.argv[2]);
registerAuditor(web3, process.argv[3]).then(r => {
    if (r) {
        process.exit(0);
    } else {
        process.exit(-1);
    }
});