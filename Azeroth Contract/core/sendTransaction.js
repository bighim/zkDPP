import Constants from './constants';
import Keys from './keys';


export async function sendTransaction(
    web3,
    call,
    gas = Constants.DEFAULT_GAS,
    senderEthAddress = Keys.defaultAddress,
    senderEthPrivateKey = Keys.defaultPrivateKey,
) {
    const txDesc = {
        from: senderEthAddress,
        gas: gas,
    };
    const encodedABI = call.encodeABI();
    const txCount = await web3.eth.getTransactionCount(senderEthAddress, 'pending');
    const chainId = await web3.eth.getChainId();
    const rawTx = {
        ...txDesc,
        data: encodedABI,
        nonce: '0x' + txCount.toString(16),
        to: call._parent._address,
        common: {
            customChain: {
                networkId: chainId,
                chainId: chainId,
            }
        }
    };

    const signedTx = await web3.eth.accounts.signTransaction(rawTx, senderEthPrivateKey);

    return web3.eth.sendSignedTransaction(signedTx.rawTransaction);
}

export default sendTransaction;