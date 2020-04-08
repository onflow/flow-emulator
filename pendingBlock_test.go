package emulator_test

import (
	"testing"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
)

func setupPendingBlockTests(t *testing.T) (
	*emulator.Blockchain,
	*flow.Transaction,
	*flow.Transaction,
	*flow.Transaction,
) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address, b.RootKey().ID).
		AddAuthorizer(b.RootKey().Address, b.RootKey().ID)

	err = tx1.SignContainer(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	require.NoError(t, err)

	tx2 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber+1).
		SetPayer(b.RootKey().Address, b.RootKey().ID).
		AddAuthorizer(b.RootKey().Address, b.RootKey().ID)

	err = tx2.SignContainer(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	require.NoError(t, err)

	invalid := flow.NewTransaction().
		SetScript([]byte("invalid script")).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address, b.RootKey().ID).
		AddAuthorizer(b.RootKey().Address, b.RootKey().ID)

	err = invalid.SignContainer(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
	require.NoError(t, err)

	return b, tx1, tx2, invalid
}

func TestPendingBlockBeforeExecution(t *testing.T) {
	t.Run("EmptyPendingBlock", func(t *testing.T) {
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
		b, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Add tx1 again
		err = b.AddTransaction(*tx)
		assert.IsType(t, &emulator.ErrDuplicateTransaction{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitBeforeExecution", func(t *testing.T) {
		b, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := b.AddTransaction(*tx)
		assert.NoError(t, err)

		// Attempt to commit block before execution begins
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.ErrPendingBlockCommitBeforeExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockDuringExecution(t *testing.T) {
	t.Run("ExecuteNextTransaction", func(t *testing.T) {
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
		assert.IsType(t, &emulator.ErrPendingBlockMidExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitMidExecution", func(t *testing.T) {
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
		assert.IsType(t, &emulator.ErrPendingBlockMidExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("TransactionsExhaustedDuringExecution", func(t *testing.T) {
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
		assert.IsType(t, &emulator.ErrPendingBlockTransactionsExhausted{}, err)

		// Attempt to execute rest of block tx (fails)
		_, err = b.ExecuteBlock()
		assert.IsType(t, &emulator.ErrPendingBlockTransactionsExhausted{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockCommit(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	t.Run("CommitBlock", func(t *testing.T) {
		tx1 := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address, b.RootKey().ID).
			AddAuthorizer(b.RootKey().Address, b.RootKey().ID)

		err = tx1.SignContainer(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Add tx1 to pending block
		err = b.AddTransaction(*tx1)
		require.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockHash := b.PendingBlockID()

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		// Commit pending block
		block, err := b.CommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockHash, block.ID())
	})

	t.Run("ExecuteAndCommitBlock", func(t *testing.T) {
		tx1 := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address, b.RootKey().ID).
			AddAuthorizer(b.RootKey().Address, b.RootKey().ID)

		err = tx1.SignContainer(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// Add tx1 to pending block
		err = b.AddTransaction(*tx1)
		assert.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockHash := b.PendingBlockID()

		// Execute and commit pending block
		block, results, err := b.ExecuteAndCommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockHash, block.ID())
		assert.Len(t, results, 1)
	})
}
