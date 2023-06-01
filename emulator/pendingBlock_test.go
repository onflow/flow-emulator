package emulator_test

import (
	"context"
	"testing"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPendingBlockTests(t *testing.T) (
	*emulator.Blockchain,
	*adapters.SDKAdapter,
	*flowsdk.Transaction,
	*flowsdk.Transaction,
	*flowsdk.Transaction,
) {
	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	tx1 := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	tx2 := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber+1).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = tx2.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	invalid := flowsdk.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = invalid.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	return b, adapter, tx1, tx2, invalid
}

func TestPendingBlockBeforeExecution(t *testing.T) {

	t.Parallel()

	t.Run("EmptyPendingBlock", func(t *testing.T) {

		t.Parallel()

		b, _, _, _, _ := setupPendingBlockTests(t)

		// Execute empty pending block
		_, err := b.ExecuteBlock()
		assert.NoError(t, err)

		// Commit empty pending block
		_, err = b.CommitBlock()
		assert.NoError(t, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddDuplicateTransaction", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Add tx1 again
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &types.DuplicateTransactionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitBeforeExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Attempt to commit block before execution begins
		_, err = b.CommitBlock()
		assert.IsType(t, &types.PendingBlockCommitBeforeExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockDuringExecution(t *testing.T) {

	t.Parallel()

	t.Run("ExecuteNextTransaction", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		require.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Execute invalid script tx (reverts)
		result, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteBlock", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		require.NoError(t, err)

		// Execute all tx in pending block (tx1, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)

		// tx1 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteNextThenBlock", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add tx2 to pending block
		err = adapter.SendTransaction(context.Background(), *tx2)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Execute rest of tx in pending block (tx2, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)
		// tx2 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddTransactionMidExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to add tx2 to pending block after execution begins
		err = adapter.SendTransaction(context.Background(), *tx2)
		assert.IsType(t, &types.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitMidExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to commit block before execution finishes
		_, err = b.CommitBlock()
		assert.IsType(t, &types.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("TransactionsExhaustedDuringExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to execute nonexistent next tx (fails)
		_, err = b.ExecuteNextTransaction()
		assert.IsType(t, &types.PendingBlockTransactionsExhaustedError{}, err)

		// Attempt to execute rest of block tx (fails)
		_, err = b.ExecuteBlock()
		assert.IsType(t, &types.PendingBlockTransactionsExhaustedError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockCommit(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	t.Run("CommitBlock", func(t *testing.T) {
		tx1 := flowsdk.NewTransaction().
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
		err = adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Commit pending block
		block, err := b.CommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
	})

	t.Run("ExecuteAndCommitBlock", func(t *testing.T) {
		tx1 := flowsdk.NewTransaction().
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
		err = adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute and commit pending block
		block, results, err := b.ExecuteAndCommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
		assert.Len(t, results, 1)
	})
}
