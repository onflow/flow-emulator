/*
 * Flow Emulator
 *
 * Copyright 2019-2024 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	flowaccess "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/utils"
)

func getFreePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCP address")
	return tcpAddr.Port
}

// TestForkingAgainstTestnet exercises the forking path by wiring a remote store
func TestForkingAgainstTestnet(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	const target = "access.testnet.nodes.onflow.org:9000"

	// Get remote latest sealed height to pin fork with automatic retry
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()
	remote := flowaccess.NewAccessAPIClient(conn)

	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	remoteHeight := rh.Block.Height - 10 // Use a buffer to avoid edge cases

	cfg := &Config{
		// Do not start listeners; NewEmulatorServer only configures components.
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet, // will be overridden by detectRemoteChainID
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	srv := NewEmulatorServer(&logger, cfg)
	require.NotNil(t, srv)
	require.Equal(t, flowgo.Testnet, cfg.ChainID)

	// Submit a minimal transaction against the forked emulator to ensure tx processing works
	latest, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)

	// Allow emulator height to be equal to or one greater than remote (if remote advanced by one between queries)
	require.Equal(t, remoteHeight+1, latest.Height)

	sk := srv.Emulator().ServiceKey()

	// Write an Int into account storage and publish a capability to read it later
	writeScript := []byte(`
        transaction {
            prepare(acct: auth(Storage, Capabilities) &Account) {
                acct.storage.save<Int>(42, to: /storage/foo)
				let cap = acct.capabilities.storage.issue<&Int>(/storage/foo)
				acct.capabilities.publish(cap, at: /public/foo)
            }
        }
    `)
	tx := flowsdk.NewTransaction().
		SetScript(writeScript).
		SetReferenceBlockID(flowsdk.Identifier(latest.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx))
	require.NoError(t, err)

	_, results, err := srv.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.Len(t, results, 1)
	r := results[0]
	require.True(t, r.Succeeded(), "Write transaction should succeed")
	require.NoError(t, r.Error)

	// Read back in a second transaction using the same authorizer and assert via logs
	readTxCode := []byte(`
		transaction {
			prepare(acct: auth(Storage) &Account) {
				let ok = acct.storage.borrow<&Int>(from: /storage/foo) != nil
				log(ok)
			}
		}
	`)
	latest2, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)
	readTx := flowsdk.NewTransaction().
		SetScript(readTxCode).
		SetReferenceBlockID(flowsdk.Identifier(latest2.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	err = readTx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*readTx))
	require.NoError(t, err)

	_, readResults, err := srv.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	require.Len(t, readResults, 1)
	require.True(t, readResults[0].Succeeded(), "Read transaction should succeed")
	require.NoError(t, readResults[0].Error)

	logs := readResults[0].Logs
	found := false
	for _, l := range logs {
		if l == "true" {
			found = true
			break
		}
	}
	require.True(t, found, "Should have successfully read stored value")
}

// TestForkingAgainstMainnet exercises the forking path with mainnet
func TestForkingAgainstMainnet(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	const target = "access.mainnet.nodes.onflow.org:9000"

	// Get remote latest sealed height to pin fork with automatic retry
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()
	remote := flowaccess.NewAccessAPIClient(conn)

	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	remoteHeight := rh.Block.Height - 10 // Use a buffer to avoid edge cases

	cfg := &Config{
		// Do not start listeners; NewEmulatorServer only configures components.
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Mainnet, // will be overridden by detectRemoteChainID
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	srv := NewEmulatorServer(&logger, cfg)
	require.NotNil(t, srv)
	require.Equal(t, flowgo.Mainnet, cfg.ChainID)

	// Test account key retrieval for a known mainnet account with multiple keys
	// This tests the account key deduplication shim by executing a script that accesses
	// keys and ensures no errors occur (successful execution proves the shim works)
	testAccountScript := []byte(`
		access(all) fun main(): Bool {
			// Test getting account keys for a known mainnet account
			let account = getAccount(0xe467b9dd11fa00df)
			
			// Test accessing specific key indices
			let key0 = account.keys.get(keyIndex: 0)
			if key0 == nil {
				return false
			}
			// Access weight and revoked status to test parsing
			// (successful access without errors proves the shim works)
			if key0!.weight < 0.0 || key0!.isRevoked == key0!.isRevoked {
				// Just access the properties, don't actually test values
			}
			
			let key1 = account.keys.get(keyIndex: 1)
			if key1 == nil {
				return false
			}
			// Access weight and revoked status to test parsing
			if key1!.weight < 0.0 || key1!.isRevoked == key1!.isRevoked {
				// Just access the properties, don't actually test values
			}
			
			return true
		}
	`)

	latest, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)
	// Allow emulator height to be equal to or one greater than remote (if remote advanced by one between queries)
	require.Equal(t, remoteHeight+1, latest.Height)
	// Execute the script to test account key retrieval
	scriptResult, err := srv.Emulator().ExecuteScript(testAccountScript, nil)
	require.NoError(t, err)
	require.True(t, scriptResult.Succeeded(), "Account key retrieval script should succeed")
	require.NoError(t, scriptResult.Error)

	// Check that the script returned true (all verifications passed)
	require.Equal(t, cadence.Bool(true), scriptResult.Value)
}

// TestForkingWithEVMInteraction exercises EVM functionality in a forked environment
func TestForkingWithEVMInteraction(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	const target = "access.mainnet.nodes.onflow.org:9000"

	// Get remote latest sealed height to pin fork with automatic retry
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()
	remote := flowaccess.NewAccessAPIClient(conn)

	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	remoteHeight := rh.Block.Height - 10 // Use a buffer to avoid edge cases

	cfg := &Config{
		// Do not start listeners; NewEmulatorServer only configures components.
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Mainnet,
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	srv := NewEmulatorServer(&logger, cfg)
	require.NotNil(t, srv)
	require.Equal(t, flowgo.Mainnet, cfg.ChainID)

	latest, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)
	require.Equal(t, remoteHeight+1, latest.Height)

	// Execute a transaction that calls EVM.encodeABI to verify EVM works in forked environment
	sk := srv.Emulator().ServiceKey()

	evmTxCode := []byte(`
		import EVM from 0xe467b9dd11fa00df

		transaction {
			prepare(acct: auth(Storage) &Account) {
				let encoded = EVM.encodeABI([])
			}
		}
	`)

	latestBlock, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)

	tx := flowsdk.NewTransaction().
		SetScript(evmTxCode).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx))
	require.NoError(t, err)

	_, results, err := srv.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.Len(t, results, 1)

	txResult := results[0]
	require.True(t, txResult.Succeeded(), "EVM transaction should succeed in forked environment")
	require.NoError(t, txResult.Error)
}

// TestForkWithPersist verifies that forking with --persist persists local overlay changes
// (new transactions) to SQLite and can resume from the persisted state.
// Note: Forked registers are cached separately in SQLite (.flow-fork-cache/)
func TestForkWithPersist(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	const target = "access.testnet.nodes.onflow.org:9000"

	// Get remote latest sealed height to pin fork
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()
	remote := flowaccess.NewAccessAPIClient(conn)

	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	remoteHeight := rh.Block.Height - 10

	// Create temp directory for DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &Config{
		DBPath:                    dbPath,
		Persist:                   true, // Enable persist mode
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet,
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	// First run: write to storage
	srv := NewEmulatorServer(&logger, cfg)
	require.NotNil(t, srv)

	latest, err := srv.Emulator().GetLatestBlock()
	require.NoError(t, err)
	require.Equal(t, remoteHeight+1, latest.Height)

	sk := srv.Emulator().ServiceKey()

	// Write an Int into account storage
	writeScript := []byte(`
        transaction {
            prepare(acct: auth(Storage) &Account) {
                acct.storage.save<Int>(42, to: /storage/persistTest)
            }
        }
    `)
	tx := flowsdk.NewTransaction().
		SetScript(writeScript).
		SetReferenceBlockID(flowsdk.Identifier(latest.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx))
	require.NoError(t, err)

	_, results, err := srv.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Succeeded(), "Persist write transaction should succeed")

	// Stop emulator (simulates restart)
	srv.Stop()

	// Verify SQLite file exists and has data
	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "SQLite DB should have data")

	// Second run: restart with same DB and read from cache
	cfg2 := &Config{
		DBPath:                    dbPath, // Same DB!
		Persist:                   true,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet,
		ForkHost:                  target,
		ForkHeight:                remoteHeight,
	}

	srv2 := NewEmulatorServer(&logger, cfg2)
	require.NotNil(t, srv2)

	latest2, err := srv2.Emulator().GetLatestBlock()
	require.NoError(t, err)

	sk2 := srv2.Emulator().ServiceKey()

	// Read the value we wrote in the previous run
	readScript := []byte(`
		transaction {
			prepare(acct: auth(Storage) &Account) {
				let value = acct.storage.borrow<&Int>(from: /storage/persistTest)
				log(value != nil && *value == 42)
			}
		}
	`)
	readTx := flowsdk.NewTransaction().
		SetScript(readScript).
		SetReferenceBlockID(flowsdk.Identifier(latest2.ID())).
		SetProposalKey(flowsdk.Address(sk2.Address), sk2.Index, sk2.SequenceNumber).
		SetPayer(flowsdk.Address(sk2.Address)).
		AddAuthorizer(flowsdk.Address(sk2.Address))

	signer2, err := sk2.Signer()
	require.NoError(t, err)

	err = readTx.SignEnvelope(flowsdk.Address(sk2.Address), sk2.Index, signer2)
	require.NoError(t, err)

	err = srv2.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*readTx))
	require.NoError(t, err)

	_, readResults, err := srv2.Emulator().ExecuteAndCommitBlock()
	require.NoError(t, err)

	require.Len(t, readResults, 1)
	require.True(t, readResults[0].Succeeded(), "Persist read transaction should succeed")

	logs := readResults[0].Logs
	found := false
	for _, l := range logs {
		if l == "true" {
			found = true
			break
		}
	}
	require.True(t, found, "Should have found stored value from SQLite cache, logs: %v", logs)
}

// TestForkCacheCreation verifies that the SQLite fork cache directory is created
// with the correct name (network-blockID format) when forking from a local emulator
func TestForkCacheCreation(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()

	// Create unique cache directory for this test to avoid conflicts
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, ".flow-fork-cache")

	// Start upstream emulator
	upstreamPort := getFreePort(t)
	upstreamCfg := &Config{
		GRPCPort:                  upstreamPort,
		RESTPort:                  getFreePort(t),
		AdminPort:                 getFreePort(t),
		Host:                      "127.0.0.1",
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Emulator,
	}

	upstream := NewEmulatorServer(&logger, upstreamCfg)
	require.NotNil(t, upstream)

	// Start upstream server
	go func() {
		upstream.Start()
	}()
	t.Cleanup(func() { upstream.Stop() })

	// Wait for upstream to be ready
	time.Sleep(500 * time.Millisecond)

	// Execute a transaction on upstream to create some state
	upstreamEmulator := upstream.Emulator()
	sk := upstreamEmulator.ServiceKey()
	latestBlock, err := upstreamEmulator.GetLatestBlock()
	require.NoError(t, err)

	writeScript := []byte(`
		transaction {
			prepare(acct: auth(Storage) &Account) {
				acct.storage.save<String>("test_value", to: /storage/testCache)
			}
		}
	`)
	tx := flowsdk.NewTransaction().
		SetScript(writeScript).
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer)
	require.NoError(t, err)

	err = upstreamEmulator.AddTransaction(*convert.SDKTransactionToFlow(*tx))
	require.NoError(t, err)

	_, results, err := upstreamEmulator.ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Succeeded(), "Upstream write transaction should succeed")

	// Get the block we'll fork from
	forkBlock, err := upstreamEmulator.GetLatestBlock()
	require.NoError(t, err)
	forkHeight := forkBlock.Height

	upstreamTarget := fmt.Sprintf("127.0.0.1:%d", upstreamPort)

	// Connect to upstream via gRPC
	conn, err := grpc.NewClient(
		upstreamTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		utils.DefaultGRPCRetryInterceptor(),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()
	remote := flowaccess.NewAccessAPIClient(conn)

	// Verify we can reach upstream
	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, rh.Block.Height, forkHeight)

	// Fork from upstream
	forkCfg := &Config{
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Emulator,
		ForkHost:                  upstreamTarget,
		ForkHeight:                forkHeight,
		ForkCacheDir:              cacheDir,
	}

	fork := NewEmulatorServer(&logger, forkCfg)
	require.NotNil(t, fork, "Fork should succeed")

	// Verify forked state
	forkLatestBlock, err := fork.Emulator().GetLatestBlock()
	require.NoError(t, err)
	require.Equal(t, forkHeight+1, forkLatestBlock.Height)

	// Verify cache directory was created with correct name format: flow-emulator-{blockID}
	blockID := forkBlock.ID().String()
	truncatedID := blockID
	if len(blockID) > 12 {
		truncatedID = blockID[:12]
	}
	expectedCacheName := "flow-emulator-" + truncatedID

	// Find the cache directory
	expectedCachePath := filepath.Join(cacheDir, expectedCacheName)
	cacheInfo, err := os.Stat(expectedCachePath)
	require.NoError(t, err, "Cache directory %s should exist", expectedCacheName)
	require.True(t, cacheInfo.IsDir(), "Cache path should be a directory")

	// Verify cache contains SQLite files
	entries, err := os.ReadDir(expectedCachePath)
	require.NoError(t, err)
	require.Greater(t, len(entries), 0, "Cache directory should contain SQLite files")

	// Verify we can read the forked state (confirms fork works)
	readScript := []byte(`
		access(all) fun main(): String? {
			let acct = getAuthAccount<auth(Storage) &Account>(0xf8d6e0586b0a20c7)
			let ref = acct.storage.borrow<&String>(from: /storage/testCache)
			if ref == nil {
				return nil
			}
			return *ref!
		}
	`)
	result, err := fork.Emulator().ExecuteScript(readScript, nil)
	require.NoError(t, err)
	require.True(t, result.Succeeded(), "Read script should succeed")
	require.NotNil(t, result.Value, "Should have read value from forked state")

	fork.Stop()
}
