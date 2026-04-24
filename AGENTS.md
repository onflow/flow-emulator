# AGENTS.md

This file guides AI coding agents working in the flow-emulator repository.
Keep it current when commands, ports, or admin endpoints change.

## Overview

flow-emulator is a Go implementation of a local Flow blockchain node that
implements the Flow Access API (gRPC + REST) for development and testing.
It is bundled by `flow-cli` as `flow emulator` and is also consumable as a
Go module (`github.com/onflow/flow-emulator`). Entry point:
`cmd/emulator/main.go`, which wires `cmd/emulator/start/start.go` (a Cobra
`start` subcommand).

## Build and Test Commands

All 12 targets are defined in the root `Makefile`.

- `make run` — run the emulator server (`go run ./cmd/emulator`).
- `make test` — run all Go tests with `-coverprofile=cover.out`.
- `make coverage` — requires `COVER=true`; produces `index.html` and
  `coverage.zip` via `gocov` / `gocov-html` / `gozip`.
- `make generate` / `make generate-mocks` — regenerate mocks via `mockgen`
  into `emulator/mocks/`, `storage/mocks/`, `internal/mocks/`.
- `make install-tools` — installs `mockgen`, `gocov`, `gocov-html`, `gozip`
  into `$GOPATH/bin`. Required before `make generate`.
- `make install-linter` — installs `golangci-lint` v2.4.0.
- `make lint` / `make fix-lint` — run `golangci-lint run -v ./...` (config
  in `.golangci.yml`: errcheck, govet, ineffassign, misspell, staticcheck).
- `make check-headers` — runs `./check-headers.sh` to verify Apache-2.0
  license headers on source files.
- `make check-tidy` — runs `make generate` + `go mod tidy` + `git diff
  --exit-code`; fails if generated files or `go.sum` are stale.
- `make ci` — full CI bundle: `install-tools test check-tidy test coverage
  check-headers`.

Go toolchain: Go 1.25.1 (`go.mod`).

## Architecture

- `cmd/emulator/` — CLI binary. `main.go` supplies a default service-key
  generator; `start/start.go` defines the Cobra command, `Config` struct,
  and all CLI flags (env prefix `FLOW`).
- `emulator/` — core in-memory blockchain: `blockchain.go`, `blocks.go`,
  `pendingBlock.go`, `contracts.go`, `clock.go`, `scheduled_transactions.go`,
  `computation_report.go`, `pragma.go`. 21 `*_test.go` files live here.
- `server/` — networked surfaces. `server.go` composes `server/access/`
  (gRPC + REST Access API), `server/debugger/` (Debug Adapter Protocol),
  `server/utils/` (admin HTTP + emulator control API), `server/liveness/`.
- `storage/` — pluggable backends: `memstore/`, `sqlite/`, `redis/`,
  `remote/` (fork from live node), `checkpoint/` (load from a flow-go
  checkpoint), `util/`.
- `adapters/` — bridges `emulator.Emulator` to the flow-go `access.API`
  and flow-go-sdk interfaces.
- `convert/` — conversions between flow-go, flow-go-sdk, and Cadence
  types (`flow.go`, `vm.go`, `emu.go`).
- `types/`, `utils/`, `internal/` — supporting packages; `internal/mocks/`
  and `emulator/mocks/` are generated, do not hand-edit.

Top-level `emulator.go` re-exports `emulator.New(opts ...Option)` for
library consumers.

## Admin HTTP Server

The admin server (default port `8080`, configurable via `--admin-port` /
`FLOW_ADMINPORT`) is defined in `server/utils/admin.go` and mounts:

- `/metrics` — Prometheus metrics.
- `/live` — liveness probe.
- `/` — gRPC-Web wrapper around the Access API gRPC server.
- `/emulator/…` — emulator control API, routed in
  `server/utils/emulator.go`:
  - `/emulator/newBlock` — commit a block (any method).
  - `POST /emulator/rollback?height=<n>` — roll back to block height.
  - `GET  /emulator/snapshots` — list snapshots.
  - `POST /emulator/snapshots?name=<n>` — create snapshot.
  - `PUT  /emulator/snapshots/{name}` — jump to snapshot.
  - `GET  /emulator/logs/{id}` — transaction logs by tx ID (hex).
  - `/emulator/config` — service-key info (any method).
  - `GET  /emulator/codeCoverage` — Cadence coverage report.
  - `PUT  /emulator/codeCoverage/reset` — reset coverage.
  - `GET  /emulator/computationProfile` — pprof profile download.
  - `PUT  /emulator/computationProfile/reset` — reset profile.
  - `GET  /emulator/computationReport` — computation report.
  - `GET  /emulator/allContracts` — zip of all deployed contracts.

## Default Ports

Defined in `cmd/emulator/start/start.go`:

- `3569` — gRPC Access API (`--port` / `-p`).
- `8888` — REST Access API (`--rest-port`).
- `8080` — Admin HTTP (`--admin-port`).
- `2345` — Debug Adapter Protocol (`--debugger-port`).

## Conventions and Gotchas

- **Env vars come from the Go struct field name, not the flag name.**
  `initConfig` in `start.go` uses `sconfig.New(&conf).FromEnvironment("FLOW")
  .BindFlags(...)`, which derives env names from the `Config` struct field
  (uppercased, with the `FLOW_` prefix) and binds flags separately. Examples:
  field `ServicePrivateKey` (flag `--service-priv-key`) →
  `FLOW_SERVICEPRIVATEKEY`; field `AdminPort` (flag `--admin-port`) →
  `FLOW_ADMINPORT`; field `RestPort` (flag `--rest-port`) → `FLOW_RESTPORT`.
  Do not infer env names by mangling the flag; read the struct field.
- **Deprecated flags are hidden but still parsed** (`start.go`
  lines 293–300): `--rpc-host` → `--fork-host`; `--start-block-height`
  → `--fork-height`; `--transaction-max-gas-limit` →
  `--transaction-max-compute-limit`; `--script-gas-limit` →
  `--script-compute-limit`. Do not reuse these names.
- **Fork mode**: `--fork-height` requires `--fork-host`; otherwise the
  server exits with an error (`start.go` line 185).
- **Chain IDs**: `--chain-id` accepts only `emulator`, `testnet`,
  `mainnet` (`getSDKChainID` in `start.go`). In fork mode, the chain ID
  is re-detected from the remote node via `server.DetectRemoteChainID`.
- **Run `make generate` before `make test`** when editing interfaces
  that have mocks: `emulator.Emulator`, `storage.Store`,
  `internal.AccessAPIClient`, `internal.ExecutionDataAPIClient`. CI
  enforces this via `make check-tidy`.
- **License headers required**: `check-headers.sh` (run via
  `make check-headers`) fails the build if the Apache-2.0 header is
  missing. New `.go` files must include it (copy from any existing file).
- **Linter config is deliberately minimal** (`.golangci.yml`): only
  errcheck, govet, ineffassign, misspell, staticcheck. Do not add new
  linters without discussion.
- **Commit style** (`CONTRIBUTING.md`): imperative mood, first line
  ≤72 chars.

## Files Not to Modify

- `emulator/mocks/emulator.go`, `storage/mocks/store.go`,
  `internal/mocks/access.go`, `internal/mocks/executiondata.go` —
  generated by `make generate-mocks`.
- `go.sum` — managed by `go mod tidy`.
- `emulator/templates/systemChunkTransactionTemplate.cdc` — template
  embedded into the runtime; edit with care.
