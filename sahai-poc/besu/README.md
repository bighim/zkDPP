# Besu QBFT network

The network uses four validators and one non-validating RPC node, chain ID 31337, a one-second
block period, and a 30,000,000 block gas limit. The Besu image is pinned to:

`hyperledger/besu@sha256:5c319f8f5f3449438c03ea7fa2c9bf24b866dc55ac98d802bb41ad793e740587`

`scripts/setup-besu.sh` materializes the generated genesis and validator keys,
then creates one identical node/account permission file for all five nodes. Generated private keys
and database directories are excluded from Git.

Local permissioning controls admission and transaction senders. The `SahaiLedger` participant role
is an independent application-layer check; neither mechanism implements Idemix anonymity.

Run `scripts/setup-besu.sh` before starting the `besu` Compose profile. The benchmark Gate records:

- four validators, one RPC peer, and BLS12-381 G1-add precompile availability;
- continued block finality while `validator4` is stopped;
- rejection of a funded account omitted from the account allowlist;
- rejection of a fifth node identity omitted from the node allowlist;
- rejection of an allowlisted but unregistered Contract caller; and
- identical fixed-proof `gasUsed` and calldata on Anvil and Besu.

The private key values `1` and `2` are test-only negative-Gate identities. Key `1` is funded but
omitted from both permission lists; key `2` is funded and account-allowlisted but is not registered
as a `SahaiLedger` participant. They are not production credentials.
