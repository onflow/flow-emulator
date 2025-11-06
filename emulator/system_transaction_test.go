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

package emulator_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func TestGetSystemTransactions(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Commit a block to generate system transactions
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)
	require.NotNil(t, block)

	blockID := flowgo.Identifier(block.ID())

	// Get system transactions for the block
	systemTxIDs, err := b.GetSystemTransactionsForBlock(blockID)
	require.NoError(t, err)
	assert.NotEmpty(t, systemTxIDs, "block should have system transactions")

	// Test GetSystemTransaction for each system transaction
	for _, txID := range systemTxIDs {
		tx, err := adapter.GetSystemTransaction(context.Background(), txID, blockID)
		require.NoError(t, err)
		assert.NotNil(t, tx, "system transaction should exist")
		// Note: system transactions may have different IDs than expected
		// due to how they are generated, so we just verify we can retrieve them

		// Test GetSystemTransactionResult
		result, err := adapter.GetSystemTransactionResult(
			context.Background(),
			txID,
			blockID,
			entities.EventEncodingVersion_CCF_V0,
		)
		require.NoError(t, err)
		assert.NotNil(t, result, "system transaction result should exist")
		// Verify the result has the expected transaction ID
		assert.Equal(t, txID, result.TransactionID)
	}
}

func TestGetSystemTransaction_NotFound(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Commit a block
	block, _, err := b.ExecuteAndCommitBlock()
	require.NoError(t, err)

	blockID := flowgo.Identifier(block.ID())

	// Try to get a non-existent system transaction
	fakeTxID := flowgo.MakeID([32]byte{0x99})
	tx, err := adapter.GetSystemTransaction(context.Background(), fakeTxID, blockID)
	assert.Error(t, err, "should return error for non-existent system transaction")
	assert.Nil(t, tx)

	result, err := adapter.GetSystemTransactionResult(
		context.Background(),
		fakeTxID,
		blockID,
		entities.EventEncodingVersion_CCF_V0,
	)
	assert.Error(t, err, "should return error for non-existent system transaction result")
	assert.Nil(t, result)
}

func TestGetSystemTransaction_BlockNotFound(t *testing.T) {
	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewAccessAdapter(&logger, b)

	// Try to get system transactions for a non-existent block
	fakeBlockID := flowgo.MakeID([32]byte{0x88})
	fakeTxID := flowgo.MakeID([32]byte{0x99})

	tx, err := adapter.GetSystemTransaction(context.Background(), fakeTxID, fakeBlockID)
	assert.Error(t, err, "should return error for non-existent block")
	assert.Nil(t, tx)

	result, err := adapter.GetSystemTransactionResult(
		context.Background(),
		fakeTxID,
		fakeBlockID,
		entities.EventEncodingVersion_CCF_V0,
	)
	assert.Error(t, err, "should return error for non-existent block")
	assert.Nil(t, result)
}
