<br />
<p align="center">
  <a href="https://docs.onflow.org/emulator/">
    <img src="docs/emulator-banner.svg" alt="Logo" width="410" height="auto">
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


### The Emulator

The emulator exposes a gRPC server that implements the Flow Access API, which is designed to have near feature parity with the real network API.

### The Flowser Emulator Explorer

There is also a block explorer GUI for the emulator, that will help you speed up development when using the emulator. 
- [Flowser GitHub Repository](https://github.com/onflowser/flowser)
- [Flowser Documentation](https://github.com/onflowser/flowser#-contents)

# Running

## Configuration
The Flow Emulator can be run in different modes and settings, all of them are described in the table bellow. 

Please note that if you will run the emulator using the Flow CLI you must use flags to pass configuration values
and if you plan to run the emulator with Docker you must use the environment variables (Env) to pass configuration values.

| Flag                          | Env                          | Default        | Description                                                                                                                                                                                                                                       |
|-------------------------------|------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--port`, `-p`                | `FLOW_PORT`                  | `3569`         | gRPC port to listen on                                                                                                                                                                                                                            |
| `--rest-port`                 | `FLOW_RESTPORT`              | `8888`         | REST API port to listen on                                                                                                                                                                                                                        |
| `--admin-port`                | `FLOW_ADMINPORT`             | `8080`         | Admin API port to listen on                                                                                                                                                                                                                       |
| `--verbose`, `-v`             | `FLOW_VERBOSE`               | `false`        | Enable verbose logging (useful for debugging)                                                                                                                                                                                                     |
| `--log-format`                | `FLOW_LOGFORMAT`             | `text`         | Output log format (valid values `text`, `JSON`)                                                                                                                                                                                                   |
| `--block-time`, `-b`          | `FLOW_BLOCKTIME`             | `0`            | Time between sealed blocks. Valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`                                                                                                                                                             |
| `--contracts`                 | `FLOW_WITHCONTRACTS`         | `false`        | Start with contracts like [NFT](https://github.com/onflow/flow-nft/blob/master/contracts/NonFungibleToken.cdc) and an [NFT Marketplace](https://github.com/onflow/nft-storefront), when the emulator starts |
| `--service-priv-key`          | `FLOW_SERVICEPRIVATEKEY`     | random         | Private key used for the [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account)                                                                                                                                     |
| `--service-sig-algo`          | `FLOW_SERVICEKEYSIGALGO`     | `ECDSA_P256`   | Service account key [signature algorithm](https://docs.onflow.org/cadence/language/crypto/#signing-algorithms)                                                                                                                                    |
| `--service-hash-algo`         | `FLOW_SERVICEKEYHASHALGO`    | `SHA3_256`     | Service account key [hash algorithm](https://docs.onflow.org/cadence/language/crypto/#hashing)                                                                                                                                                    |
| `--init`                      | `FLOW_INIT`                  | `false`        | Generate and set a new [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account)                                                                                                                                       |
| `--rest-debug`                | `FLOW_RESTDEBUG`             | `false`        | Enable REST API debugging output                                                                                                                                                                                                                  |
| `--grpc-debug`                | `FLOW_GRPCDEBUG`             | `false`        | Enable gRPC server reflection for debugging with grpc_cli                                                                                                                                                                                         |
| `--persist`                   | `FLOW_PERSIST`               | false          | Enable persistence of the state between restarts                                                                                                                                                                                                  |
| `--snapshot`                  | `FLOW_SNAPSHOT`              | false          | Enable snapshot support                                                                                                                                                                        |
| `--dbpath`                    | `FLOW_DBPATH`                | `./flowdb`     | Specify path for the database file persisting the state                                                                                                                                                                                           |
| `--simple-addresses`          | `FLOW_SIMPLEADDRESSES`       | `false`        | Use sequential addresses starting with `0x1`                                                                                                                                                                                                      |
| `--token-supply`              | `FLOW_TOKENSUPPLY`           | `1000000000.0` | Initial FLOW token supply                                                                                                                                                                                                                         |
| `--transaction-expiry`        | `FLOW_TRANSACTIONEXPIRY`     | `10`           | [Transaction expiry](https://docs.onflow.org/flow-go-sdk/building-transactions/#reference-block), measured in blocks                                                                                                                              |
| `--storage-limit`             | `FLOW_STORAGELIMITENABLED`   | `true`         | Enable [account storage limit](https://docs.onflow.org/cadence/language/accounts/#storage-limit)                                                                                                                                                  |
| `--storage-per-flow`          | `FLOW_STORAGEMBPERFLOW`      |                | Specify size of the storage in MB for each FLOW in account balance. Default value from the flow-go                                                                                                                                                |
| `--min-account-balance`       | `FLOW_MINIMUMACCOUNTBALANCE` |                | Specify minimum balance the account must have. Default value from the flow-go                                                                                                                                                                     |
| `--transaction-fees`          | `FLOW_TRANSACTIONFEESENABLED` | `false`        | Enable variable transaction fees and execution effort metering <br> as decribed in [Variable Transaction Fees: Execution Effort](https://github.com/onflow/flow/pull/753) FLIP                                                                    |
| `--transaction-max-gas-limit` | `FLOW_TRANSACTIONMAXGASLIMIT` | `9999`         | Maximum [gas limit for transactions](https://docs.onflow.org/flow-go-sdk/building-transactions/#gas-limit)                                                                                                                                        |
| `--script-gas-limit`          | `FLOW_SCRIPTGASLIMIT`        | `100000`       | Specify gas limit for script execution                                                                                                                                                                                                            |
| `--coverage-reporting`        | `FLOW_COVERAGEREPORTING`     | `false`        | Enable Cadence code coverage reporting                                                                                                                                                                                                      |
| `--contract-removal`          | `FLOW_CONTRACTREMOVAL`            | `true`         | Allow removal of already deployed contracts, used for updating during development                                                                                                                                                                 |
| `--skip-tx-validation` | `FLOW_SKIPTRANSACTIONVALIDATION` | `false`        | Skip verification of transaction signatures and sequence numbers                                                                                                                                                                                  |
| `--host`                      | `FLOW_HOST`                  | ` `            | Host to listen on for emulator GRPC/REST/Admin servers (default: All Interfaces)                                                                                                                                                                                            |
| `--chain-id`                  | `FLOW_CHAINID`               | `emulator`     | Chain to simulate, if 'mainnet' or 'testnet' values are used, you will be able to run transactions against that network and a local fork will be created..  Valid values are: 'emulator', 'testnet', 'mainnet'                                     |
| `--redis-url`                 | `FLOW_REDIS_URL`             | ''             | Redis-server URL for persisting redis storage backend ( `redis://[[username:]password@]host[:port][/database]` )                                                                                                                                  |
| `--start-block-height`        | `FLOW_STARTBLOCKHEIGHT`             | `0`             | Start block height to use when starting the network using 'testnet' or 'mainnet' as the chain-id    |

| `--legacy-upgrade` | `FLOW_LEGACYUPGRADE` | `false`         | Enable upgrading of legacy contracts |

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
flow emulator
```
You need to make sure the configuration `flow.json` exists, or create it beforehand using the `flow init` command.

### Using the emulator in a project

You can start the emulator in your project context by running the above command
in the same directory as `flow.json`. This will configure the emulator with your
project's service account, meaning you can use it to sign and submit transactions.
Read more about the project and configuration [here](https://docs.onflow.org/flow-cli/configuration/).

## Using Emulator in Go
You can use the emulator as a module in your Go project. To install emulator, use go get:
```
go get github.com/onflow/flow-emulator
```

After installing the emulator module you can initialize it in the code:
```go
var opts []emulator.Option
privKey, err := crypto.DecodePrivateKeyHex(crypto.ECDSA_P256, "")

opts = append(opts, emulator.WithServicePublicKey(
  privKey.PublicKey(),
  crypto.ECDSA_P256,
  crypto.SHA3_256,
))

blockchain, err := emulator.NewBlockchain(opts...)
```

You can then access all methods of the blockchain like so:
```go
account, err := blockchain.GetAccount(address) 
```

## Rolling back state to blockheight 
It is possible to roll back the emulator state to a specific block height. This
feature is extremely useful for testing purposes. You can set up an account
state, perform tests on that state, and then roll back the state to its initial
state after each test.

To roll back to a specific block height, you can utilize below HTTP request: 
``
POST http://localhost:8080/emulator/rollback

Post Data: height={block height}
```

Note: it is only possible to roll back state to a height that was previously executed by the emulator.
To roll back to a past block height when using a forked Mainnet or Testnet network, use the
`--start-block-height` flag.

## Managing emulator state
It's possible to manage emulator state by using the admin API. You can at any point 
create a new named snapshot of the state and then at any later point revert emulator 
state to that reference. 

In order to use the state management functionality you need to run the emulator with persistent state:
```bash
flow emulator --persist
```

Create a new snapshot by doing an HTTP request: 
```
POST http://localhost:8080/emulator/snapshots

Post Data: name={snapshot name}

```
*Please note the example above uses the default admin API port*

At any later point you can reload to that snapshot by executing:

```
PUT http://localhost:8080/emulator/snapshots?name={snapshot name}
```

You need to use the same value for `name` parameter.

The snapshot functionality is a great tool for testing where you can first initialize 
a base snapshot with seed values, execute the test and then revert to that initialized state.

You can list existing snapshots with:
```
GET http://localhost:8080/emulator/snapshots
```

## Cadence Code Coverage

The admin API includes endpoints for viewing and managing Cadence code coverage.

In order to use this functionality you need to run the emulator with the respective flag which enables code coverage:
```bash
flow emulator --coverage-reporting
```

To view the code coverage report, visit this URL: http://localhost:8080/emulator/codeCoverage

To flush/reset the collected code coverage report, run the following command:
```bash
curl -XPUT 'http://localhost:8080/emulator/codeCoverage/reset'
```
Note: The above command will reset the code coverage for all the locations, except for `A.f8d6e0586b0a20c7.FlowServiceAccount`, which is a system contract that is essential to the operations of Flow.

To get better reports with source file references, you can utilize the `sourceFile` pragma in the headers of your transactions and scripts.

```cadence
#sourceFile("scripts/myScript.cdc")
```

## Running the emulator with Docker

Docker builds for the emulator are automatically built and pushed to
`gcr.io/flow-container-registry/emulator`, tagged by commit and semantic version. You can also build the image locally.

```bash
docker run -p 3569:3569 -p 8080:8080 -e FLOW_HOST=0.0.0.0 gcr.io/flow-container-registry/emulator
```

The full list of environment variables can be found [here](#configuration). 
You can pass any environment variable by using `-e` docker flag and pass the valid value.

*Custom Configuration Example:*
```bash
docker run -p 3569:3569 -p 8080:8080 -e FLOW_HOST=0.0.0.0 -e FLOW_PORT=9001 -e FLOW_VERBOSE=true -e FLOW_SERVICEPRIVATEKEY=<hex-encoded key> gcr.io/flow-container-registry/emulator
```

To generate a service key, use the `keys generate` command in the Flow CLI.
```bash
flow keys generate
```

## Emulating mainnet and testnet transactions
The emulator allows you to simulate the execution of transactions as if they were 
performed on the mainnet or testnet. In order to activate this feature, 
you must specify the network name for the chain ID flag in the following manner:
```
flow emulator --chain-id mainnet
```
Please note, the actual execution on the real network may differ.

By default, the forked network will start from the latest sealed block when the emulator
is started. You can specify a different starting block height by using the `--start-block-height` flag.

You can also store all of your changes and cached registers to a persistent db using the `--persist` flag,
along with the other sqlite settings.

## Debugging
To debug any transactions sent via VSCode or Flow CLI, you can use the `debugger` pragma. 
This will cause execution to pause at the debugger for any transaction or script which includes that pragma.

```cadence 
#debugger()
```

## Development

Read [contributing document](./CONTRIBUTING.md).
