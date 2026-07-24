import path from 'path';

const __dirname = path.resolve();
import fs from 'fs-extra';
import _ from 'lodash';

const basedPath = path.join(__dirname, 'solc');
const buildPath = path.join(basedPath, 'compiled');

/**
 * basePath : project-root/build/
 * @param {string} fileName snarkPack's proof and crs data file
 * @returns {string} json string
 */
export function readVerifyinput(fileName) {
    return fs.readFileSync(path.join(__dirname, fileName + '.json'), 'utf8');
}

/**
 * basePath : project-root/build/
 * @param {string} compiledPath
 * @param {string} contractName contract Name
 * @returns {string} compiled contract json string
 */
export function readCompiledContract(compiledPath, contractName) {
    return fs.readFileSync(path.join(compiledPath, contractName + '.json'), 'utf8');
}

/**
 * basePath : project-root/build/deployed/
 * @param {string} deployedPath
 * @param {string} contractName contract Name
 * @returns {Object} deployed contract JSON
 */
export function readDeployedContract(deployedPath, contractName) {
    return JSON.parse(fs.readFileSync(path.join(deployedPath, contractName + '.json')));
}

/**
 * basePath : project-root/src/web3/keystore/
 * @param {string} keystoreName keystore Name
 * @returns {string} keystore json string
 */
export function readKeystore(keystoreName) {
    return fs.readFileSync(path.join(__dirname, 'src', 'web3', 'keystore', keystoreName + '.json'));
}

/**
 * basePath : project-root/build/
 * @param {Object} compiled contract name
 */
export function writeCompiledContract(compiled) {
    writeCompiledContractPath(compiled, buildPath);
}

/**
 * basePath : project-root/build/
 * @param {Object} compiled Compiled result in .sol files
 * @param {string} targetContractName  Target contract name
 * @param {string} contractPath path of compiled contracts
 */
export function writeCompiledContractAbiAndBytecode(compiled, targetContractName, contractPath) {
    writeCompiledContractAbiAndBytecodePath(compiled, targetContractName, buildPath, contractPath);
}

/**
 * @param {string} root
 * @param {Promise<Set<string>>} sources
 */
export function compileImports(root, sources) {
    sources[root] = {
        content: fs.readFileSync(root, 'utf8'),
    };
    const imports = getNeededImports(root);
    for (let i = 0; i < imports.length; i++) {
        compileImports(imports[i], sources);
    }
}

/**
 * basePath : project-root/build/deployed/
 * @returns {[string]}
 */
export function getDeployedFileList() {
    return getFilelist(path.join(buildPath, 'deployed'));
}

/**
 * basePath : project-root/src/web3/contracts
 * @returns {[string]}
 */
export function getSolidityFileList() {
    return getFilelist(path.join(buildPath, 'src', 'web3', 'contracts'));
}

/**
 * basePath : project-root/build/
 * @returns {[string]}
 */
export function getCompiledFileList() {
    return getFilelist(path.join(buildPath));
}

/**
 *
 * @param {string} basePath
 * @returns {[string]}
 */
export function getFilelist(basePath) {
    return fs.readdirSync(basePath, function (err, content) {
        if (err) {
            return err;
        } else {
            return content;
        }
    });
}

/**
 *
 * @param {string} path
 * @returns string
 */
export function getPragma(path) {
    let file = fs.readFileSync(path, 'utf8');
    const pragma = file.split('\n');
    console.log(pragma);
    for (let i = 0; i < pragma.length; i++) {
        if (pragma[i].trim().startsWith('pragma')) {
            return pragma[i].substring(7, pragma[i].length);
        }
    }
}

/**
 * returns All import paths in solc
 * @param {string} path
 * @returns {[string]} import paths
 */
export function getNeededImports(path) {
    const file = fs.readFileSync(path, 'utf8');
    const files = [];
    file.toString()
        .split('\n')
        .forEach(function (line, index, arr) {
            if ((index === arr.length - 1 && line === '') || !line.trim().startsWith('import')) {
                return;
            }
            const relativePath = line.substring(8, line.length - 2);
            const fullPath = buildFullPath(path, relativePath);
            files.push(fullPath);
        });
    return files;
}

/**
 * @param {string} parent parent's path
 * @param {string} path Relative path from the parent's path
 * @returns {string} Relative path from the project root directory
 */
function buildFullPath(parent, path) {
    let curDir = parent.substring(0, parent.lastIndexOf('/'));
    if (path.startsWith('./')) {
        return curDir + '/' + path.substr(2);
    }
    while (path.startsWith('../')) {
        curDir = curDir.substring(0, curDir.lastIndexOf('/'));
        path = path.substring(3);
    }
    if (curDir.length === 0) return path;
    else {
        return curDir + '/' + path;
    }
}

/**
 * write the deployed contract to the buildPath in json format
 * @param {Object} txReceipt ethereum TransactionReceipt format
 * @param {Object} compiled compiled sol file
 * @param {string} contractName contract name
 * @param {string} deployedPath path to write file
 * @returns {JSON} Deployed Contract
 */
export function writeDeployedContract(txReceipt, compiled, contractName, deployedPath) {
    let outputFile = {
        contractName: contractName,
        bytecode: '0x' + compiled.evm.bytecode.object,
        abi: compiled.abi,
        address: txReceipt.contractAddress,
        txHash: txReceipt.transactionHash,
        blockNumber: txReceipt.blockNumber,
        gasUsed: txReceipt.gasUsed,
        from: txReceipt.from,
    };
    fs.outputJsonSync(path.resolve(deployedPath, contractName + '.json'), outputFile);
    return outputFile;
}

/**
 * write the compiled contract to the buildPath in json format
 * @param {Object} compiled compiled sol file
 * @param {string} buildPath path to write file
 */
function writeCompiledContractPath(compiled, buildPath) {
    fs.ensureDirSync(buildPath);
    //logger.debug(`Compiling ${compiled.contracts}`);
    console.log(compiled);
    for (let contractFilePath in compiled.contracts) {
        for (let contractName in compiled.contracts[contractFilePath]) {
            console.log('compile: ' + contractName + '.sol');
            fs.outputJsonSync(
                path.resolve(buildPath, contractName + '.json'),
                compiled.contracts[contractFilePath][contractName],
            );
        }
    }
}

/**
 * write the compiled contract to the buildPath in json format
 * @param {Object} compiled compiled sol file
 * @param {string} targetContractName Target contract name
 * @param {string} buildPath path to write file
 * @param {string} contractPath path of compiled contracts
 */
function writeCompiledContractAbiAndBytecodePath(compiled, targetContractName, buildPath, contractPath) {
    fs.ensureDirSync(buildPath);
    const targetContract = _.get(_.get(compiled.contracts, contractPath), targetContractName);

    const abi = _.get(targetContract, 'abi');
    const bytecode = _.get(targetContract, 'evm.bytecode.object');

    console.log(`Generate file, ${path.join(buildPath, targetContractName + '/abi.json')}`);
    fs.outputJsonSync(
        path.resolve(buildPath, targetContractName + '/abi.json'),
        abi,
    );
    console.log(`Generate file, ${path.join(buildPath, targetContractName + '/bytecode')}`);
    fs.writeFileSync(
        path.resolve(buildPath, targetContractName + '/bytecode'),
        '0x' + bytecode,
        { flag: 'w' },
    );
}
