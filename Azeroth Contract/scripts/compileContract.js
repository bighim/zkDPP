import solcModules from '../core/index.js';
import path from 'path';
import url from 'url';

const CONTRACT_NAME = 'Groth16AltBN128Mixer';

// Module path
const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const __contractsDir = path.join(__dirname, '../contracts');

function compilingContract() {
    const basePath = path.join(__contractsDir, CONTRACT_NAME + '.sol');
    return solcModules.compiler(basePath, CONTRACT_NAME);
}

console.log('Web3 configuration load ...');
console.log({
    contract_name: CONTRACT_NAME,
});

console.log(`Compiling smart contract [${CONTRACT_NAME}] ...`);
const compiled = compilingContract();
if (compiled.contracts !== undefined) {
    console.log('Init Smart contract Succeed !');
    process.exit(0);
} else {
    console.log('Solidity Compile Error');
    process.exit(0);
}
