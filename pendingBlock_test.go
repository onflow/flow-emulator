package emulator_test

import (
	"testing"

	"github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
)

func setupPendingBlockTests(t *testing.T) (
	*emulator.Blockchain,
	*flow.Transaction,
	*flow.Transaction,
	*flow.Transaction,
) {
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

	tx2 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber+1).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx2.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	invalid := flow.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = invalid.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	return b, tx1, tx2, invalid
}

func TestPendingBlockBeforeExecution(t *testing.T) {

	t.Parallel()

	t.Run("EmptyPendingBlock", func(t *testing.T) {

		t.Parallel()

		b, _, _, _ := setupPendingBlockTests(t)

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

		b, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Add tx1 again
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.DuplicateTransactionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitBeforeExecution", func(t *testing.T) {

		t.Parallel()

		b, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Attempt to commit block before execution begins
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.PendingBlockCommitBeforeExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockDuringExecution(t *testing.T) {

	t.Parallel()

	t.Run("ExecuteNextTransaction", func(t *testing.T) {

		t.Parallel()

		b, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(*invalid)
		require.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Execute invalid script tx (reverts)
		result, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteBlock", func(t *testing.T) {

		t.Parallel()

		b, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(*invalid)
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

		b, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		assert.NoError(t, err)

		// Add tx2 to pending block
		err = b.AddTransaction(*tx2)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(*invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

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

		b, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = b.AddTransaction(*invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Attempt to add tx2 to pending block after execution begins
		err = b.AddTransaction(*tx2)
		assert.IsType(t, &emulator.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitMidExecution", func(t *testing.T) {

		t.Parallel()

		b, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = b.AddTransaction(*invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Attempt to commit block before execution finishes
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("TransactionsExhaustedDuringExecution", func(t *testing.T) {

		t.Parallel()

		b, tx1, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx1)
		assert.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Attempt to execute nonexistent next tx (fails)
		_, err = b.ExecuteNextTransaction()
		assert.IsType(t, &emulator.PendingBlockTransactionsExhaustedError{}, err)

		// Attempt to execute rest of block tx (fails)
		_, err = b.ExecuteBlock()
		assert.IsType(t, &emulator.PendingBlockTransactionsExhaustedError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockCommit(t *testing.T) {

	t.Parallel()

	b, err := emulator.NewBlockchain(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	t.Run("CommitBlock", func(t *testing.T) {
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
		require.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Commit pending block
		block, err := b.CommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
	})

	t.Run("ExecuteAndCommitBlock", func(t *testing.T) {
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

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute and commit pending block
		block, results, err := b.ExecuteAndCommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
		assert.Len(t, results, 1)
	})
}
