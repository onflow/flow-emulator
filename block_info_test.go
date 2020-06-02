package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/dapperlabs/flow-emulator"
	sdkconvert "github.com/dapperlabs/flow-emulator/convert/sdk"
)

func TestBlockInfo(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	block1, err := b.CommitBlock()
	require.NoError(t, err)

	block2, err := b.CommitBlock()
	require.NoError(t, err)

	t.Run("works as transaction", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript([]byte(`
				transaction {
					execute {
						let block = getCurrentBlock()
						log(block)

						let lastBlock = getBlock(at: block.height - UInt64(1))
						log(lastBlock)
					}
				}
			`)).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().ID, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assertTransactionSucceeded(t, result)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Height+1,
			b.PendingBlockID(), float64(b.PendingBlockTimestamp().Unix())), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Height,
			sdkconvert.SDKIdentifierToFlow(block2.ID), float64(block2.Timestamp.Unix())), result.Logs[1])
	})

	t.Run("works as script", func(t *testing.T) {
		script := []byte(`
			pub fun main() {
				let block = getCurrentBlock()
				log(block)

				let lastBlock = getBlock(at: block.height - UInt64(1))
				log(lastBlock)
			}
		`)

		result, err := b.ExecuteScript(script)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Height,
			sdkconvert.SDKIdentifierToFlow(block2.ID), float64(block2.Timestamp.Unix())), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block1.Height,
			sdkconvert.SDKIdentifierToFlow(block1.ID), float64(block1.Timestamp.Unix())), result.Logs[1])
	})
}
