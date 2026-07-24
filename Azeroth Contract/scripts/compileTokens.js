import solcModules from '../core/index.js';
import path from 'path';
import url from 'url';

const ERC20_PRESET = 'ERC20PresetMinterPauser';
const ERC721_PRESET = 'ERC721PresetMinterPauserAutoId';

// Module path
const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const __contractsDir = path.join(__dirname, '../../node_modules/@openzeppelin/contracts');
const __ERC20Dir = path.join(__contractsDir, 'token/ERC20/presets/');
const __ERC721Dir = path.join(__contractsDir, 'token/ERC721/presets/');

function compilingTokens() {
    const basePathERC20 = path.join(__ERC20Dir, ERC20_PRESET + '.sol');
    const basePathERC721 = path.join(__ERC721Dir, ERC721_PRESET + '.sol');
    return {
        erc20: solcModules.compiler(basePathERC20, ERC20_PRESET),
        erc721: solcModules.compiler(basePathERC721, ERC721_PRESET),
    };
}

console.log('Web3 configuration load ...');
console.log({
    erc20_name: ERC20_PRESET,
    erc721_name: ERC721_PRESET,
});

console.log(`Compiling smart contract [${ERC20_PRESET}, ${ERC721_PRESET}] ...`);
const { erc20, erc721 } = compilingTokens();
if (erc20.contracts !== undefined &&
    erc721.contracts !== undefined) {
    console.log('Init Token contracts Succeed !');
    process.exit(0);
} else {
    console.log('Solidity Compile Error');
    process.exit(0);
}
