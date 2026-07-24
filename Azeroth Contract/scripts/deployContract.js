import Web3 from 'web3';
import solcModules from '../core/index.js';

async function deploy(web3) {
    return solcModules.deploy(web3);
}

const web3 = new Web3(process.argv[2]);

deploy(web3).then((r) => {
    if (r) {
        process.stdout.write(r);
        process.exit(0);
    } else {
        process.exit(-1);
    }
});