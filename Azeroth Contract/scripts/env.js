import dotenv from 'dotenv';

dotenv.config({
    path: process.env.ENVFILE || '.env'
});
