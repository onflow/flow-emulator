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
	"strings"
	"testing"

	"github.com/onflow/flow-emulator/convert"
	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	flowaccess "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestForkingAgainstTestnet exercises the forking path by wiring a remote store
func TestForkingAgainstTestnet(t *testing.T) {
	logger := zerolog.Nop()

	// Get remote latest sealed height to pin fork
	conn, err := grpc.NewClient("access.testnet.nodes.onflow.org:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial remote: %v", err)
	}
	defer conn.Close()
	remote := flowaccess.NewAccessAPIClient(conn)
	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	if err != nil {
		t.Fatalf("get remote header: %v", err)
	}
	remoteHeight := rh.Block.Height

	cfg := &Config{
		// Do not start listeners; NewEmulatorServer only configures components.
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Testnet, // will be overridden by detectRemoteChainID
		ForkURL:                   "access.testnet.nodes.onflow.org:9000",
		ForkBlockNumber:           remoteHeight,
	}

	srv := NewEmulatorServer(&logger, cfg)
	if srv == nil {
		t.Fatal("NewEmulatorServer returned nil")
	}

	if cfg.ChainID != flowgo.Testnet {
		t.Fatalf("expected ChainID to be Testnet after fork detection, got %q", cfg.ChainID)
	}

	// Create an initial local block so we have a valid reference block ID in the forked store
	if _, _, err := srv.Emulator().ExecuteAndCommitBlock(); err != nil {
		t.Fatalf("prime local block: %v", err)
	}

	// Submit a minimal transaction against the forked emulator to ensure tx processing works
	latest, err := srv.Emulator().GetLatestBlock()
	if err != nil {
		t.Fatalf("get latest block: %v", err)
	}
	// Allow emulator height to be equal to or one greater than remote (if remote advanced by one between queries)
	if latest.Height != remoteHeight+1 {
		t.Fatalf("fork height mismatch: emulator %d not in {remote, remote+1} where remote=%d", latest.Height, remoteHeight)
	}
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
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	if err := tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer); err != nil {
		t.Fatalf("sign envelope: %v", err)
	}
	if err := srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx)); err != nil {
		t.Fatalf("add tx: %v", err)
	}
	if _, results, err := srv.Emulator().ExecuteAndCommitBlock(); err != nil {
		t.Fatalf("execute block: %v", err)
	} else {
		if len(results) != 1 {
			t.Fatalf("expected 1 tx result, got %d", len(results))
		}
		r := results[0]
		if !r.Succeeded() {
			t.Fatalf("tx failed: %v", r.Error)
		}
	}

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
	if err != nil {
		t.Fatalf("get latest block for read tx: %v", err)
	}
	readTx := flowsdk.NewTransaction().
		SetScript(readTxCode).
		SetReferenceBlockID(flowsdk.Identifier(latest2.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))
	if err := readTx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer); err != nil {
		t.Fatalf("sign read envelope: %v", err)
	}
	if err := srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*readTx)); err != nil {
		t.Fatalf("add read tx: %v", err)
	}
	if _, readResults, err := srv.Emulator().ExecuteAndCommitBlock(); err != nil {
		t.Fatalf("execute read block: %v", err)
	} else {
		if len(readResults) != 1 || !readResults[0].Succeeded() {
			t.Fatalf("read tx failed: %v", readResults[0].Error)
		}
		logs := readResults[0].Logs
		found := false
		for _, l := range logs {
			if l == "true" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected log containing true, got: %v", logs)
		}
	}
}

// TestForkingAgainstMainnet exercises the forking path with mainnet and tests the account key deduplication shim
func TestForkingAgainstMainnet(t *testing.T) {
	logger := zerolog.Nop()

	// Get remote latest sealed height to pin fork
	conn, err := grpc.NewClient("access.mainnet.nodes.onflow.org:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial remote: %v", err)
	}
	defer conn.Close()
	remote := flowaccess.NewAccessAPIClient(conn)
	rh, err := remote.GetLatestBlockHeader(context.Background(), &flowaccess.GetLatestBlockHeaderRequest{IsSealed: true})
	if err != nil {
		t.Fatalf("get remote header: %v", err)
	}
	remoteHeight := rh.Block.Height

	cfg := &Config{
		// Do not start listeners; NewEmulatorServer only configures components.
		DBPath:                    "",
		Persist:                   false,
		Snapshot:                  false,
		SkipTransactionValidation: true,
		ChainID:                   flowgo.Mainnet, // will be overridden by detectRemoteChainID
		ForkURL:                   "access.mainnet.nodes.onflow.org:9000",
		ForkBlockNumber:           remoteHeight,
	}

	srv := NewEmulatorServer(&logger, cfg)
	if srv == nil {
		t.Fatal("NewEmulatorServer returned nil")
	}

	if cfg.ChainID != flowgo.Mainnet {
		t.Fatalf("expected ChainID to be Mainnet after fork detection, got %q", cfg.ChainID)
	}

	// Create an initial local block so we have a valid reference block ID in the forked store
	if _, _, err := srv.Emulator().ExecuteAndCommitBlock(); err != nil {
		t.Fatalf("prime local block: %v", err)
	}

	// Test account key retrieval for a known mainnet account with multiple keys
	// This tests the account key deduplication shim
	testAccountScript := []byte(`
		transaction {
			prepare(acct: auth(Storage) &Account) {
				// Test getting account keys for a known mainnet account
				let account = getAccount(0xe467b9dd11fa00df)
				
				// Test accessing specific key indices
				let key0 = account.keys.get(keyIndex: 0)
				if key0 != nil {
					log("Key 0 weight: ".concat(key0!.weight.toString()))
					if key0!.isRevoked {
						log("Key 0 is revoked")
					} else {
						log("Key 0 is not revoked")
					}
				}
				
				let key1 = account.keys.get(keyIndex: 1)
				if key1 != nil {
					log("Key 1 weight: ".concat(key1!.weight.toString()))
					if key1!.isRevoked {
						log("Key 1 is revoked")
					} else {
						log("Key 1 is not revoked")
					}
				}
				
				// Test that we can access keys without errors
				log("Account key access test completed")
			}
		}
	`)

	latest, err := srv.Emulator().GetLatestBlock()
	if err != nil {
		t.Fatalf("get latest block: %v", err)
	}
	// Allow emulator height to be equal to or one greater than remote (if remote advanced by one between queries)
	if latest.Height != remoteHeight+1 {
		t.Fatalf("fork height mismatch: emulator %d not in {remote, remote+1} where remote=%d", latest.Height, remoteHeight)
	}
	sk := srv.Emulator().ServiceKey()

	tx := flowsdk.NewTransaction().
		SetScript(testAccountScript).
		SetReferenceBlockID(flowsdk.Identifier(latest.ID())).
		SetProposalKey(flowsdk.Address(sk.Address), sk.Index, sk.SequenceNumber).
		SetPayer(flowsdk.Address(sk.Address)).
		AddAuthorizer(flowsdk.Address(sk.Address))

	signer, err := sk.Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	if err := tx.SignEnvelope(flowsdk.Address(sk.Address), sk.Index, signer); err != nil {
		t.Fatalf("sign envelope: %v", err)
	}
	if err := srv.Emulator().AddTransaction(*convert.SDKTransactionToFlow(*tx)); err != nil {
		t.Fatalf("add tx: %v", err)
	}
	if _, results, err := srv.Emulator().ExecuteAndCommitBlock(); err != nil {
		t.Fatalf("execute block: %v", err)
	} else {
		if len(results) != 1 {
			t.Fatalf("expected 1 tx result, got %d", len(results))
		}
		r := results[0]
		if !r.Succeeded() {
			t.Fatalf("tx failed: %v", r.Error)
		}

		// Check that we got meaningful logs about the account keys
		logs := r.Logs
		hasKeyWeight := false
		hasCompletion := false
		for _, log := range logs {
			if strings.Contains(log, "weight:") {
				hasKeyWeight = true
			}
			if strings.Contains(log, "Account key access test completed") {
				hasCompletion = true
			}
		}

		if !hasKeyWeight {
			t.Fatalf("expected log with key weight, got: %v", logs)
		}
		if !hasCompletion {
			t.Fatalf("expected completion log, got: %v", logs)
		}

		t.Logf("Account key test successful. Logs: %v", logs)
	}
}
