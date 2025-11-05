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

	sub := adapter.SubscribeBlocksFromStartHeight(ctx, 0, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	require.NotNil(t, ch)

	select {
	case data := <-ch:
		block, ok := data.(*flowgo.Block)
		require.True(t, ok, "expected *flowgo.Block, got %T", data)
		assert.Equal(t, uint64(0), block.Height)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis block")
	}

	newBlock, _, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)

	select {
	case data := <-ch:
		block, ok := data.(*flowgo.Block)
		require.True(t, ok, "expected *flowgo.Block, got %T", data)
		assert.Equal(t, uint64(1), block.Height)
		assert.Equal(t, newBlock.ID(), block.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new block")
	}
}

func TestStreamingBlockHeaders_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	genesisBlock, err := blockchain.GetBlockByHeight(0)
	require.NoError(t, err)

	sub := adapter.SubscribeBlockHeadersFromStartBlockID(ctx, genesisBlock.ID(), flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()

	select {
	case data := <-ch:
		header, ok := data.(*flowgo.Header)
		require.True(t, ok, "expected *flowgo.Header, got %T", data)
		assert.Equal(t, uint64(0), header.Height)
		assert.Equal(t, genesisBlock.ID(), header.ID())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis header")
	}
}

func TestStreamingBlockDigests_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub := adapter.SubscribeBlockDigestsFromLatest(ctx, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()

	select {
	case data := <-ch:
		digest, ok := data.(*flowgo.BlockDigest)
		require.True(t, ok, "expected *flowgo.BlockDigest, got %T", data)
		assert.Equal(t, uint64(0), digest.Height)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis digest")
	}

	newBlock, _, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)

	select {
	case data := <-ch:
		digest, ok := data.(*flowgo.BlockDigest)
		require.True(t, ok, "expected *flowgo.BlockDigest, got %T", data)
		assert.Equal(t, uint64(1), digest.Height)
		assert.Equal(t, newBlock.ID(), digest.BlockID)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new digest")
	}
}

func TestStreamingTransactionStatuses_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serviceKey := blockchain.ServiceKey()
	latestBlock, err := blockchain.GetLatestBlock()
	require.NoError(t, err)

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

	sub := adapter.SendAndSubscribeTransactionStatuses(ctx, tx, entities.EventEncodingVersion_CCF_V0)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _, _ = blockchain.ExecuteAndCommitBlock()
	}()

	statusesReceived := make(map[flowgo.TransactionStatus]bool)
	timeout := time.After(5 * time.Second)

	for {
		select {
		case data := <-ch:
			if data == nil {
				continue
			}

			if sub.Err() != nil {
				if sub.Err() == subscription.ErrEndOfData {
					goto done
				}
				continue
			}

			results, ok := data.([]*accessmodel.TransactionResult)
			if !ok {
				continue
			}

			for _, result := range results {
				if result.TransactionID != flowgo.ZeroID {
					assert.Equal(t, tx.ID(), result.TransactionID)
				}
				statusesReceived[result.Status] = true

				if result.Status == flowgo.TransactionStatusSealed {
					goto done
				}
			}

		case <-timeout:
			t.Fatalf("timeout waiting for transaction statuses. Received: %v", statusesReceived)
		}
	}

done:
	assert.True(t, statusesReceived[flowgo.TransactionStatusSealed],
		"should receive sealed status. Got: %v", statusesReceived)
}

func TestStreamingMultipleBlocks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	blockchain, adapter := setupRealBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sub := adapter.SubscribeBlocksFromStartHeight(ctx, 0, flowgo.BlockStatusSealed)
	require.NotNil(t, sub)
	require.NoError(t, sub.Err())

	ch := sub.Channel()
	blocksReceived := 0
	expectedBlocks := 5

	select {
	case data := <-ch:
		block := data.(*flowgo.Block)
		assert.Equal(t, uint64(0), block.Height)
		blocksReceived++
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for genesis block")
	}

	for i := 0; i < expectedBlocks-1; i++ {
		_, _, err := blockchain.ExecuteAndCommitBlock()
		require.NoError(t, err)

		select {
		case data := <-ch:
			receivedBlock := data.(*flowgo.Block)
			assert.Equal(t, uint64(i+1), receivedBlock.Height)
			blocksReceived++
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for block %d", i+1)
		}
	}

	assert.Equal(t, expectedBlocks, blocksReceived)
}
