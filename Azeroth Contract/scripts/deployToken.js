import Web3 from 'web3';
import solcModules from '../core/index.js';

async function deployToken(web3, tokenPresetName, tokenConstructorArgs) {
    return solcModules.deployToken(web3, tokenPresetName, tokenConstructorArgs);
}

const web3 = new Web3(process.argv[2]);
const args = process.argv.slice(4);

deployToken(web3, process.argv[3], args).then((r) => {
    if (r) {
        process.stdout.write(r);
        process.exit(0);
    } else {
        process.exit(-1);
    }
});