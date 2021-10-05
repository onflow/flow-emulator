<br />
<p align="center">
  <a href="https://docs.onflow.org/emulator/">
    <img src="./emulator-banner.svg" alt="Logo" width="410" height="auto">
  </a>

  <p align="center">
    <i>The Flow Emulator is a lightweight tool that emulates the behaviour of the real Flow network.</i>
    <br />
    <a href="https://docs.onflow.org/emulator/"><strong>Read the docs»</strong></a>
    <br />
    <br />
    <a href="https://github.com/onflow/flow-emulator/issues">Report Bug</a>
    ·
    <a href="https://github.com/onflow/flow-emulator/blob/master/CONTRIBUTING.md">Contribute</a>
  </p>
</p>
<br />
<br />


The emulator exposes a gRPC server that implements the Flow Access API, which is designed to have near feature parity with the real network API.

# Running

## Configuration
The Flow Emulator can be run in different modes and settings, all of them are described in the table bellow. 

Please note that if you will run the emulator using the Flow CLI you must use flags to pass configuration values
and if you plan to run the emulator with Docker you must use the environment variables (Env) to pass configuration values.

| Flag             | Env   | Default | Description                                                            |
| ----------------- | ------ | ----------------- | ----------------- |
| `--port`, `-p` | `FLOW_PORT` | `3569` | RPC port to listen on |
| `--http-port` | `FLOW_HTTPPORT` | `8080` | HTTP port to listen on |
| `--verbose`, `-v` | `FLOW_VERBOSE` | `false` | Enable verbose logging (useful for debugging) |
| `--log-format` | `FLOW_LOGFORMAT` | `text` | Output log format (valid values `text`, `JSON`) |
| `--block-time`, `-b` | `FLOW_BLOCKTIME` | `0` | Time between sealed blocks. Valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h` |
| `--include-helpful-contracts` | `FLOW_INCLUDEHELPFULCONTRACTS` | `false` | Deploy some helpful contracts like [NFT](https://github.com/onflow/flow-nft/blob/master/contracts/NonFungibleToken.cdc) and an [NFT Marketplace](https://github.com/onflow/nft-storefront), when the emulator starts |
| `--service-priv-key` | `FLOW_SERVICEPRIVATEKEY` | random | Private key used for the [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account) |
| `--service-pub-key` | `FLOW_SERVICEPUBLICKEY` | random | Public key used for the [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account) |
| `--service-sig-algo` | `FLOW_SERVICEKEYSIGALGO` | `ECDSA_P256` | Service account key [signature algorithm](https://docs.onflow.org/cadence/language/crypto/#signing-algorithms) |
| `--service-hash-algo` | `FLOW_SERVICEKEYHASHALGO` | `SHA3_256` | Service account key [hash algorithm](https://docs.onflow.org/cadence/language/crypto/#hashing) |
| `--init` | `FLOW_INIT` | `false` | Generate and set a new [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account) |
| `--grpc-debug` | `FLOW_GRPCDEBUG` | `false` | Enable gRPC server reflection for debugging with grpc_cli |
| `--persist` | `FLOW_PERSIST` | false | Enable persistence of the state between restarts |
| `--dbpath` | `FLOW_DBPATH` | `./flowdb` | Specify path for the database file persisting the state |
| `--simple-addresses` | `FLOW_SIMPLEADDRESSES` | `false` | Use sequential addresses starting with `0x1` |
| `--token-supply` | `FLOW_TOKENSUPPLY` | `1000000000.0` | Initial FLOW token supply |
| `--transaction-expiry` | `FLOW_TRANSACTIONEXPIRY` | `10` | [Transaction expiry](https://docs.onflow.org/flow-go-sdk/building-transactions/#reference-block), measured in blocks |
| `--storage-limit` | `FLOW_STORAGELIMITENABLED` | `true` | Enable [account storage limit](https://docs.onflow.org/cadence/language/accounts/#storage-limit) |
| `--storage-per-flow` | `FLOW_STORAGEMBPERFLOW` |  | Specify size of the storage in MB for each FLOW in account balance. Default value from the flow-go |
| `--min-account-balance` | `FLOW_MINIMUMACCOUNTBALANCE` |  | Specify minimum balance the account must have. Default value from the flow-go |
| `--transaction-fees` | `FLOW_TRANSACTIONFEESENABLED` | `false` | Enable [transaction fees](https://docs.onflow.org/flow-token/concepts/#transaction-fees) |
| `--transaction-max-gas-limit` | `FLOW_TRANSACTIONMAXGASLIMIT` | `9999` | Maximum [gas limit for transactions](https://docs.onflow.org/flow-go-sdk/building-transactions/#gas-limit) |
| `--script-gas-limit` | `FLOW_SCRIPTGASLIMIT` | `100000` | Specify gas limit for script execution |

## Running the emulator with the Flow CLI

The emulator is bundled with the [Flow CLI](https://docs.onflow.org/flow-cli), a command-line interface for working with Flow.

### Installation

Follow [these steps](https://docs.onflow.org/flow-cli/install/) to install the Flow CLI.

### Starting the server

Starting the emulator by using Flow CLI also leverages CLI configuration file `flow.json`. 
You can use the `flow.json` to specify the service account which will be reused between restarts. 
Read more about CLI configuration [here](https://docs.onflow.org/flow-cli/configuration/).

You can start the emulator with the Flow CLI:

```shell script
flow emulator --init
```

### Using the emulator in a project

You can start the emulator in your project context by running the above command
in the same directory as `flow.json`. This will configure the emulator with your
project's service account, meaning you can use it to sign and submit transactions.
Read more about the project and configuration [here](https://docs.onflow.org/flow-cli/configuration/).

## Running the emulator with Docker

Docker builds for the emulator are automatically built and pushed to
`gcr.io/flow-container-registry/emulator`, tagged by commit and semantic version. You can also [build the image locally](#building).

```bash
docker run gcr.io/flow-container-registry/emulator
```

The full list of environment variables can be found [here](#configuration). 
You can pass any environment variable by using `-e` docker flag and pass the valid value.

*Custom Configuration Example:*
```bash
docker run -e FLOW_PORT=9001 -e FLOW_VERBOSE=true -e FLOW_SERVICEPUBLICKEY=<hex-encoded key> gcr.io/flow-container-registry/emulator
```

To generate a service key, use the `keys generate` command in the Flow CLI.
```bash
flow keys generate
```

## Development

Read [contributing document](./CONTRIBUTING.md).
