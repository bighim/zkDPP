import fs from 'fs';
import path from 'path';
import url from 'url';

const __dirname = path.dirname(url.fileURLToPath(import.meta.url));
const keys = JSON.parse(
    fs.readFileSync(
        path.join(__dirname, '../keys.json'),
        'utf-8'
    ));

export const addresses = Object.values(keys.addresses);
export const privateKeys = Object.values(keys.private_keys);

export const defaultAddress = addresses[0];
export const defaultPrivateKey = privateKeys[0];

const Keys = {
    addresses,
    privateKeys,
    defaultAddress,
    defaultPrivateKey,
};

export default Keys;