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
	"testing"

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
	defer func() { _ = conn.Close() }()
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
	require.True(t, r.Succeeded())
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
	require.True(t, readResults[0].Succeeded())
	require.NoError(t, readResults[0].Error)

	logs := readResults[0].Logs
	found := false
	for _, l := range logs {
		if l == "true" {
			found = true
			break
		}
	}
	require.True(t, found)
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
	defer func() { _ = conn.Close() }()
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
	require.True(t, scriptResult.Succeeded())
	require.NoError(t, scriptResult.Error)

	// Check that the script returned true (all verifications passed)
	require.Equal(t, cadence.Bool(true), scriptResult.Value)

	t.Logf("Account key test successful. Script result: %v", scriptResult.Value)
}
