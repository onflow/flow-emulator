package emulator_test

import (
	"testing"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
)

func TestCommitBlock(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(10).
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

	err = tx1.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Add tx1 to pending block
	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	// TODO: fix once GetTransactionStatus is implemented
	// tx, err := b.GetTransaction(tx1.ID())
	// assert.NoError(t, err)
	// assert.Equal(t, flow.TransactionPending, tx.Status)

	tx2 := flow.NewTransaction().
		SetScript([]byte("invalid script")).
		SetGasLimit(10).
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

	err = tx2.SignContainer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID, b.RootKey().Signer())
	assert.NoError(t, err)

	// Add tx2 to pending block
	err = b.AddTransaction(*tx2)
	assert.NoError(t, err)

	// TODO: fix once GetTransactionStatus is implemented
	// tx, err = b.GetTransaction(tx2.ID())
	// assert.NoError(t, err)
	// assert.Equal(t, flow.TransactionPending, tx.Status)

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

	// TODO: fix once GetTransactionStatus is implemented
	// tx1 status becomes TransactionSealed
	// tx, err = b.GetTransaction(tx1.ID())
	// require.NoError(t, err)
	// assert.Equal(t, flow.TransactionSealed, tx.Status)

	// TODO: fix once GetTransactionStatus is implemented
	// tx2 status stays TransactionReverted
	// tx, err = b.GetTransaction(tx2.ID())
	// require.NoError(t, err)
	// assert.Equal(t, flow.TransactionReverted, tx.Status)
}
