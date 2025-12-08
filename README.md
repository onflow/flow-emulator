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
<br />
<br />

### The Emulator

The emulator exposes a gRPC server that implements the Flow Access API, which is designed to have near feature parity
with the real network API.

### The Flowser Emulator Explorer

There is also a block explorer GUI for the emulator, that will help you speed up development when using the emulator.

- [Flowser GitHub Repository](https://github.com/onflowser/flowser)
- [Flowser Documentation](https://github.com/onflowser/flowser#-contents)

# Running

## Configuration

The Flow Emulator can be run in different modes and settings, all of them are described in the table below.

Please note that if you will run the emulator using the Flow CLI you must use flags to pass configuration values
and if you plan to run the emulator with Docker you must use the environment variables (Env) to pass configuration
values.

| Flag                              | Env                                  | Default        | Description                                                                                                                                                                                                   |
|-----------------------------------|--------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--port`, `-p`                    | `FLOW_PORT`                          | `3569`         | gRPC port to listen on                                                                                                                                                                                        |
| `--rest-port`                     | `FLOW_RESTPORT`                      | `8888`         | REST API port to listen on                                                                                                                                                                                    |
| `--admin-port`                    | `FLOW_ADMINPORT`                     | `8080`         | Admin API port to listen on                                                                                                                                                                                   |
| `--verbose`, `-v`                 | `FLOW_VERBOSE`                       | `false`        | Enable verbose logging (useful for debugging)                                                                                                                                                                 |
| `--log-format`                    | `FLOW_LOGFORMAT`                     | `text`         | Output log format (valid values `text`, `JSON`)                                                                                                                                                               |
| `--block-time`, `-b`              | `FLOW_BLOCKTIME`                     | `0`            | Time between sealed blocks. Valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`                                                                                                                         |
| `--contracts`                     | `FLOW_WITHCONTRACTS`                 | `false`        | Start with contracts like [ExampleNFT](https://github.com/onflow/flow-nft/blob/master/contracts/NonFungibleToken.cdc) when the emulator starts                                                                |
| `--service-priv-key`              | `FLOW_SERVICEPRIVATEKEY`             | random         | Private key used for the [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account)                                                                                                 |
| `--service-sig-algo`              | `FLOW_SERVICEKEYSIGALGO`             | `ECDSA_P256`   | Service account key [signature algorithm](https://docs.onflow.org/cadence/language/crypto/#signing-algorithms)                                                                                                |
| `--service-hash-algo`             | `FLOW_SERVICEKEYHASHALGO`            | `SHA3_256`     | Service account key [hash algorithm](https://docs.onflow.org/cadence/language/crypto/#hashing)                                                                                                                |
| `--init`                          | `FLOW_INIT`                          | `false`        | Generate and set a new [service account](https://docs.onflow.org/flow-token/concepts/#flow-service-account)                                                                                                   |
| `--rest-debug`                    | `FLOW_RESTDEBUG`                     | `false`        | Enable REST API debugging output                                                                                                                                                                              |
| `--grpc-debug`                    | `FLOW_GRPCDEBUG`                     | `false`        | Enable gRPC server reflection for debugging with grpc_cli                                                                                                                                                     |
| `--persist`                       | `FLOW_PERSIST`                       | false          | Enable persistence of the state between restarts                                                                                                                                                              |
| `--snapshot`                      | `FLOW_SNAPSHOT`                      | false          | Enable snapshot support                                                                                                                                                                                       |
| `--dbpath`                        | `FLOW_DBPATH`                        | `./flowdb`     | Specify path for the database file persisting the state                                                                                                                                                       |
| `--simple-addresses`              | `FLOW_SIMPLEADDRESSES`               | `false`        | Use sequential addresses starting with `0x1`                                                                                                                                                                  |
| `--token-supply`                  | `FLOW_TOKENSUPPLY`                   | `1000000000.0` | Initial FLOW token supply                                                                                                                                                                                     |
| `--transaction-expiry`            | `FLOW_TRANSACTIONEXPIRY`             | `10`           | [Transaction expiry](https://docs.onflow.org/flow-go-sdk/building-transactions/#reference-block), measured in blocks                                                                                          |
| `--storage-limit`                 | `FLOW_STORAGELIMITENABLED`           | `true`         | Enable [account storage limit](https://docs.onflow.org/cadence/language/accounts/#storage-limit)                                                                                                              |
| `--storage-per-flow`              | `FLOW_STORAGEMBPERFLOW`              |                | Specify size of the storage in MB for each FLOW in account balance. Default value from the flow-go                                                                                                            |
| `--min-account-balance`           | `FLOW_MINIMUMACCOUNTBALANCE`         |                | Specify minimum balance the account must have. Default value from the flow-go                                                                                                                                 |
| `--transaction-fees`              | `FLOW_TRANSACTIONFEESENABLED`        | `false`        | Enable variable transaction fees and execution effort metering <br> as described in [Variable Transaction Fees: Execution Effort](https://github.com/onflow/flow/pull/753) FLIP                               |
| `--transaction-max-compute-limit` | `FLOW_TRANSACTIONMAXCOMPUTELIMIT`    | `9999`         | Maximum [compute limit for transactions](https://docs.onflow.org/flow-go-sdk/building-transactions/#gas-limit)                                                                                                |
| `--script-compute-limit`          | `FLOW_SCRIPTCOMPUTELIMIT`            | `100000`       | Specify compute limit for script execution                                                                                                                                                                    |
| ~~`--transaction-max-gas-limit`~~ | ~~`FLOW_TRANSACTIONMAXGASLIMIT`~~    | `9999`         | **Deprecated:** Use `--transaction-max-compute-limit` instead                                                                                                                                                 |
| ~~`--script-gas-limit`~~          | ~~`FLOW_SCRIPTGASLIMIT`~~            | `100000`       | **Deprecated:** Use `--script-compute-limit` instead                                                                                                                                                          |
| `--coverage-reporting`            | `FLOW_COVERAGEREPORTING`             | `false`        | Enable Cadence code coverage reporting                                                                                                                                                                        |
| `--contract-removal`              | `FLOW_CONTRACTREMOVAL`               | `true`         | Allow removal of already deployed contracts, used for updating during development                                                                                                                             |
| `--skip-tx-validation`            | `FLOW_SKIPTRANSACTIONVALIDATION`     | `false`        | Skip verification of transaction signatures and sequence numbers                                                                                                                                              |
| `--host`                          | `FLOW_HOST`                          | ` `            | Host to listen on for emulator GRPC/REST/Admin servers (default: All Interfaces)                                                                                                                              |
| `--chain-id`                      | `FLOW_CHAINID`                       | `emulator`     | Chain to simulate, if 'mainnet' or 'testnet' values are used, you will be able to run transactions against that network and a local fork will be created.  Valid values are: 'emulator', 'testnet', 'mainnet' |
| `--redis-url`                     | `FLOW_REDIS_URL`                     | ''             | Redis-server URL for persisting redis storage backend ( `redis://[[username:]password@]host[:port][/database]` )                                                                                              |
| `--fork-host`                     | `FLOW_FORK_HOST`                     | ''             | gRPC access node address (`host:port`) to fork from                                                                                                                                                           |
| `--fork-height`                   | `FLOW_FORK_HEIGHT`                   | `0`            | Block height to pin the fork (defaults to latest sealed)                                                                                                                                                      |
| `--legacy-upgrade`                | `FLOW_LEGACYUPGRADE`                 | `false`        | Enable upgrading of legacy contracts                                                                                                                                                                          |
| `--computation-reporting`         | `FLOW_COMPUTATIONREPORTING`          | `false`        | Enable computation reporting for Cadence programs                                                                                                                                                             |
| `--computation-profiling`         | `FLOW_COMPUTATIONPROFILING`          | `false`        | Enable computation profiling for Cadence programs                                                                                                                                                             |
| `--checkpoint-dir`                | `FLOW_CHECKPOINTDIR`                 | ''             | Checkpoint directory to load the emulator state from, if starting the emulator from a checkpoint                                                                                                              |
| `--state-hash`                    | `FLOW_STATEHASH`                     | ''             | State hash of the checkpoint, if starting the emulator from a checkpoint                                                                                                                                      |
| `--num-accounts`                  | `FLOW_NUMACCOUNTS`                   | `0`            | Precreate and fund this many accounts at startup (mints 1000.0 FLOW to each)                                                                                                                                 |
    
## Running the emulator with the Flow CLI

The emulator is bundled with the [Flow CLI](https://docs.onflow.org/flow-cli), a command-line interface for working with
Flow.

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
To pin the starting block height when using a fork, use the `--fork-height` flag.

## Managing emulator state
It's possible to manage emulator state by using the admin API. You can at any point
create a new named snapshot of the state and then at any later point revert emulator
state to that reference.

In order to use the state management functionality you need to run the emulator with persistent state:
```sh
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

```sh
flow emulator --coverage-reporting
```

To view the code coverage report, visit this URL: http://localhost:8080/emulator/codeCoverage

To flush/reset the collected code coverage report, run the following command:

```sh
curl -XPUT 'http://localhost:8080/emulator/codeCoverage/reset'
```

Note: The above command will reset the code coverage for all the locations, except
for `A.f8d6e0586b0a20c7.FlowServiceAccount`, which is a system contract that is essential to the operations of Flow.

To get better reports with source file references, you can utilize the `sourceFile` pragma in the headers of your
transactions and scripts.

```cadence
#sourceFile("scripts/myScript.cdc")
```

## Cadence Computation Reporting

The admin API includes an endpoint for viewing the computation reports for Cadence programs.

In order to use this functionality you need to run the emulator with the respective flag which enables computation
reporting:

```sh
flow emulator --computation-reporting
```

To view the computation report, visit this URL: http://localhost:8080/emulator/computationReport.

The computation report can be reset by sending a PUT request to the following URL:
http://localhost:8080/emulator/computationReport/reset.

## Cadence Computation Profiling

The admin API includes an endpoint for viewing the computation profile for Cadence programs.

In order to use this functionality you need to run the emulator with the respective flag which enables computation
profiling:

```sh
flow emulator --computation-profiling
```

To view the computation report, visit this URL: http://localhost:8080/emulator/computationProfile.

This downloads a pprof file that can be analyzed using https://github.com/google/pprof.

To view the profile in a web browser, run the following command:

```sh
pprof -http=:8081 profile.pprof
```

Then open your web browser and navigate to `http://localhost:8081`.

To view the source code of the functions, first [download all deployed contracts](#downloading-all-deployed-contracts),
extract the ZIP file, and place the `contracts` folder in the same directory where you run the pprof command.

Then run the pprof command with the `-source_path` flag:

```sh
pprof -source_path=contracts -http=:8081 profile.pprof
```

The computation profile can be reset by sending a PUT request to the following URL:
http://localhost:8080/emulator/computationProfile/reset.

## Downloading all deployed contracts

To download all deployed contracts as a ZIP file, visit this URL: http://localhost:8080/emulator/allContracts

## Running the emulator with Docker

Docker builds for the emulator are automatically built and pushed to
`gcr.io/flow-container-registry/emulator`, tagged by commit and semantic version. You can also build the image locally.

```sh
docker run -p 3569:3569 -p 8080:8080 -e FLOW_HOST=0.0.0.0 gcr.io/flow-container-registry/emulator
```

The full list of environment variables can be found [here](#configuration).
You can pass any environment variable by using `-e` docker flag and pass the valid value.

*Custom Configuration Example:*

```sh
docker run -p 3569:3569 -p 8080:8080 \
    -e FLOW_HOST=0.0.0.0 \
    -e FLOW_PORT=9001 \
    -e FLOW_VERBOSE=true \
    -e FLOW_SERVICEPRIVATEKEY=<hex-encoded key> \
    gcr.io/flow-container-registry/emulator
```

To generate a service key, use the `keys generate` command in the Flow CLI.

```bash
flow keys generate
```

## Emulating mainnet and testnet transactions

The emulator allows you to simulate the execution of transactions as if
performed on the Mainnet or Testnet. To activate this feature,
you must specify the network name for the chain ID flag and the RPC host
to connect to.

```sh
flow emulator --fork-host access.mainnet.nodes.onflow.org:9000
flow emulator --fork-host access.mainnet.nodes.onflow.org:9000 --fork-height 12345
```

Please note, that the actual execution on the real network may differ depending on the exact state when the transaction
is executed.

By default, the forked network will start from the latest sealed block when the emulator is started.
You can specify a different starting block height by using the `--fork-height` flag.

You can also store all of your changes and cached registers to a persistent db by using the `--persist` flag,
along with the other SQLite settings.

To submit transactions as a different account, you can use the `--skip-tx-validation` flag to disable the transaction
signature
verification. Then submit transactions from any account using any valid private key.

## Debugging

To debug any transactions sent via VSCode or Flow CLI, you can use the `debugger` pragma.
This will cause execution to pause at the debugger for any transaction or script which includes that pragma.

```cadence
#debugger()
```

## Development

Read [contributing document](./CONTRIBUTING.md).
