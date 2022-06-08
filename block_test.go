/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
)

func TestCommitBlock(t *testing.T) {

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Add tx1 to pending block
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	tx1Result, err := b.GetTransactionResult(tx1.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusPending, tx1Result.Status)

	tx2 := flow.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err = b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx2.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Add tx2 to pending block
	err = b.AddTransaction(*tx2)
	require.NoError(t, err)

	tx2Result, err := b.GetTransactionResult(tx2.ID())
	assert.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusPending, tx2Result.Status)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	// Execute tx2
	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	// Commit tx1 and tx2 into new block
	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err = b.GetTransactionResult(tx1.ID())
	require.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, tx1Result.Status)

	// tx2 status also becomes TransactionStatusSealed, even though it is reverted
	tx2Result, err = b.GetTransactionResult(tx2.ID())
	require.NoError(t, err)
	assert.Equal(t, flow.TransactionStatusSealed, tx2Result.Status)
	assert.Error(t, tx2Result.Error)
}

func TestBlockView(t *testing.T) {

	t.Parallel()

	const nBlocks = 3

	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	t.Run("genesis should have 0 view", func(t *testing.T) {
		block, err := b.GetBlockByHeight(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), block.Header.Height)
		assert.Equal(t, uint64(0), block.Header.View)
	})

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	// create a few blocks, each with one transaction
	for i := 0; i < nBlocks; i++ {

		tx := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Add tx to pending block
		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		// execute and commit the block
		_, _, err = b.ExecuteAndCommitBlock()
		require.NoError(t, err)
	}

	for height := uint64(1); height <= nBlocks+1; height++ {
		block, err := b.GetBlockByHeight(height)
		require.NoError(t, err)

		maxView := height * emulator.MaxViewIncrease
		t.Run(fmt.Sprintf("block %d should have view <%d", height, maxView), func(t *testing.T) {
			assert.Equal(t, height, block.Header.Height)
			assert.LessOrEqual(t, block.Header.View, maxView)
		})
	}
}
