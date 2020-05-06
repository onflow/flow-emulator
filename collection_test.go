package emulator_test

import (
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
)

func TestCollections(t *testing.T) {
	t.Run("Empty block", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		// block should not contain any collections
		assert.Empty(t, block.CollectionGuarantees)
	})

	t.Run("Non-empty block", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

		tx1 := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx1.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		tx2 := flow.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		err = tx2.SignEnvelope(b.RootKey().Address, b.RootKey().ID, b.RootKey().Signer())
		assert.NoError(t, err)

		// generate a list of transactions
		transactions := []*flow.Transaction{tx1, tx2}

		// add all transactions to block
		for _, tx := range transactions {
			err = b.AddTransaction(*tx)
			require.NoError(t, err)
		}

		block, _, err := b.ExecuteAndCommitBlock()
		require.NoError(t, err)

		// block should contain at least one collection
		assert.NotEmpty(t, block.CollectionGuarantees)

		i := 0
		for _, guarantee := range block.CollectionGuarantees {
			collection, err := b.GetCollection(guarantee.CollectionID)
			require.NoError(t, err)

			for _, txID := range collection.TransactionIDs {
				assert.Equal(t, transactions[i].ID(), txID)
				i++
			}
		}
	})
}
