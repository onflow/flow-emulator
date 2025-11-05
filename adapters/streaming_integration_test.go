/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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

package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/storage/memstore"
)

// setupRealBlockchain creates a real blockchain instance for integration testing
func setupRealBlockchain(t *testing.T) (*emulator.Blockchain, *AccessAdapter) {
	store := memstore.New()
	logger := zerolog.Nop()

	blockchain, err := emulator.New(
		emulator.WithStore(store),
		emulator.WithServerLogger(logger),
	)
	require.NoError(t, err)

	adapter := NewAccessAdapter(&logger, blockchain)
	return blockchain, adapter
}

func TestStreamingBlocks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Subscribe from genesis
	sub := adapter.SubscribeBlocksFromStartHeight(ctx, 0, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	
	// Check that subscription is valid
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	require.NotNil(t, ch)

	// Read the genesis block
	select {
	case data := <-ch:
		require.NotNil(t, data, "expected genesis block data")
		block, ok := data.(*flowgo.Block)
		require.True(t, ok, "expected *flowgo.Block, got %T", data)
		assert.Equal(t, uint64(0), block.Height, "expected genesis block height 0")
		t.Logf("✅ Received genesis block: height=%d, id=%s", block.Height, block.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis block")
	}

	// Commit a new block
	newBlock, _, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)
	t.Logf("✅ Committed new block: height=%d, id=%s", newBlock.Height, newBlock.ID())

	// Should receive the new block via subscription
	select {
	case data := <-ch:
		require.NotNil(t, data, "expected new block data")
		block, ok := data.(*flowgo.Block)
		require.True(t, ok, "expected *flowgo.Block, got %T", data)
		assert.Equal(t, uint64(1), block.Height, "expected block height 1")
		assert.Equal(t, newBlock.ID(), block.ID(), "block IDs should match")
		t.Logf("✅ Received new block via stream: height=%d, id=%s", block.Height, block.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new block")
	}

	t.Log("✅ Block streaming working correctly!")
}

func TestStreamingBlockHeaders_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get genesis block for reference
	genesisBlock, err := blockchain.GetBlockByHeight(0)
	require.NoError(t, err)

	// Subscribe from genesis block ID
	sub := adapter.SubscribeBlockHeadersFromStartBlockID(ctx, genesisBlock.ID(), flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	require.NotNil(t, ch)

	// Read the genesis block header
	select {
	case data := <-ch:
		require.NotNil(t, data, "expected genesis header")
		header, ok := data.(*flowgo.Header)
		require.True(t, ok, "expected *flowgo.Header, got %T", data)
		assert.Equal(t, uint64(0), header.Height)
		assert.Equal(t, genesisBlock.ID(), header.ID())
		t.Logf("✅ Received genesis header: height=%d, id=%s", header.Height, header.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis header")
	}

	t.Log("✅ Block header streaming working correctly!")
}

func TestStreamingBlockDigests_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Subscribe from latest
	sub := adapter.SubscribeBlockDigestsFromLatest(ctx, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	require.NotNil(t, ch)

	// Read the genesis digest
	select {
	case data := <-ch:
		require.NotNil(t, data, "expected genesis digest")
		digest, ok := data.(*flowgo.BlockDigest)
		require.True(t, ok, "expected *flowgo.BlockDigest, got %T", data)
		assert.Equal(t, uint64(0), digest.Height)
		t.Logf("✅ Received genesis digest: height=%d, id=%s, timestamp=%s", 
			digest.Height, digest.BlockID, digest.Timestamp)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis digest")
	}

	// Commit a block
	newBlock, _, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)

	// Should receive the digest
	select {
	case data := <-ch:
		require.NotNil(t, data, "expected new digest")
		digest, ok := data.(*flowgo.BlockDigest)
		require.True(t, ok, "expected *flowgo.BlockDigest, got %T", data)
		assert.Equal(t, uint64(1), digest.Height)
		assert.Equal(t, newBlock.ID(), digest.BlockID)
		t.Logf("✅ Received new digest via stream: height=%d, id=%s", digest.Height, digest.BlockID)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new digest")
	}

	t.Log("✅ Block digest streaming working correctly!")
}

func TestStreamingTransactionStatuses_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create and submit a transaction
	serviceKey := blockchain.ServiceKey()
	latestBlock, err := blockchain.GetLatestBlock()
	require.NoError(t, err)

	// Convert SDK address to flow address
	serviceAddress := flowgo.BytesToAddress(serviceKey.Address.Bytes())

	txBuilder := flowgo.NewTransactionBodyBuilder().
		SetScript([]byte(`
			transaction {
				prepare(signer: &Account) {
					log("Transaction executed successfully")
				}
			}
		`)).
		SetReferenceBlockID(latestBlock.ID()).
		SetProposalKey(serviceAddress, serviceKey.Index, serviceKey.SequenceNumber).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress)
	
	tx, err := txBuilder.Build()
	require.NoError(t, err)

	// Send and subscribe
	sub := adapter.SendAndSubscribeTransactionStatuses(ctx, tx, entities.EventEncodingVersion_CCF_V0)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())
	t.Logf("✅ Transaction submitted: id=%s", tx.ID())

	ch := sub.Channel()
	require.NotNil(t, ch)

	// Commit a block to execute the transaction
	go func() {
		time.Sleep(100 * time.Millisecond)
		block, results, err := blockchain.ExecuteAndCommitBlock()
		if err != nil {
			t.Logf("❌ Error committing block: %v", err)
			return
		}
		t.Logf("✅ Block committed: height=%d, transactions=%d", block.Height, len(results))
	}()

	// Collect status updates
	statusesReceived := make(map[flowgo.TransactionStatus]bool)
	timeout := time.After(5 * time.Second)

	for {
		select {
		case data := <-ch:
			if data == nil {
				continue
			}

			// Check if it's an error
			if sub.Err() != nil {
				if sub.Err() == subscription.ErrEndOfData {
					t.Log("✅ Received end of data signal (expected after sealed)")
					goto done
				}
				t.Logf("Subscription error: %v", sub.Err())
				continue
			}

			// Should be a slice of transaction results
			results, ok := data.([]*accessmodel.TransactionResult)
			if !ok {
				t.Logf("Unexpected data type: %T", data)
				continue
			}

			for _, result := range results {
				// The transaction ID should match, but during intermediate statuses
				// it might not be fully populated yet
				if result.TransactionID != flowgo.ZeroID {
					assert.Equal(t, tx.ID(), result.TransactionID)
				}
				statusesReceived[result.Status] = true
				t.Logf("✅ Received status: %s", result.Status)

				// If sealed, we're done
				if result.Status == flowgo.TransactionStatusSealed {
					goto done
				}
			}

		case <-timeout:
			t.Fatalf("timeout waiting for transaction statuses. Received: %v", statusesReceived)
		}
	}

done:
	// Should have received sealed status
	assert.True(t, statusesReceived[flowgo.TransactionStatusSealed], 
		"should receive sealed status. Got: %v", statusesReceived)
	
	t.Logf("✅ Transaction status streaming working correctly! Statuses received: %v", statusesReceived)
}

func TestStreamingMultipleBlocks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Subscribe from genesis
	sub := adapter.SubscribeBlocksFromStartHeight(ctx, 0, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	blocksReceived := 0
	expectedBlocks := 5

	// Read genesis
	select {
	case data := <-ch:
		block := data.(*flowgo.Block)
		assert.Equal(t, uint64(0), block.Height)
		blocksReceived++
		t.Logf("✅ Block %d: height=%d", blocksReceived, block.Height)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis block")
	}

	// Commit multiple blocks
	for i := 0; i < expectedBlocks-1; i++ {
		block, _, err := blockchain.ExecuteAndCommitBlock()
		require.NoError(t, err)
		t.Logf("✅ Committed block: height=%d", block.Height)

		// Should receive via stream
		select {
		case data := <-ch:
			receivedBlock := data.(*flowgo.Block)
			assert.Equal(t, uint64(i+1), receivedBlock.Height)
			blocksReceived++
			t.Logf("✅ Block %d: height=%d", blocksReceived, receivedBlock.Height)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for block %d", i+1)
		}
	}

	assert.Equal(t, expectedBlocks, blocksReceived, "should receive all blocks")
	t.Logf("✅ Multiple blocks streaming working correctly! Received %d blocks", blocksReceived)
}

