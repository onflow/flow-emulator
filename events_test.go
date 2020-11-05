package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go-sdk"

	emulator "github.com/onflow/flow-emulator"
)

func TestEventEmitted(t *testing.T) {
	t.Run("EmittedFromScript", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		script := []byte(`
			pub event MyEvent(x: Int, y: Int)

			pub fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		result, err := b.ExecuteScript(script, nil)
		assert.NoError(t, err)
		require.Len(t, result.Events, 1)

		actualEvent := result.Events[0]

		decodedEvent := actualEvent.Value

		location := runtime.ScriptLocation(result.ScriptID.Bytes())
		expectedType := fmt.Sprintf("%s.MyEvent", location.ID())

		// NOTE: ID is undefined for events emitted from scripts

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, cadence.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, cadence.NewInt(2), decodedEvent.Fields[1])
	})

	t.Run("EmittedFromAccount", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountContracts := []templates.Contract{
			{
				Name: "Test",
				Source: `
                    pub contract Test {
						pub event MyEvent(x: Int, y: Int)

						pub fun emitMyEvent(x: Int, y: Int) {
							emit MyEvent(x: x, y: y)
						}
					}
				`,
			},
		}

		publicKey := b.ServiceKey().AccountKey()

		address, err := b.CreateAccount(
			[]*flow.AccountKey{publicKey},
			accountContracts,
		)
		assert.NoError(t, err)

		script := []byte(fmt.Sprintf(`
			import 0x%s

			transaction {
				execute {
					Test.emitMyEvent(x: 1, y: 2)
				}
			}
		`, address.Hex()))

		tx := flow.NewTransaction().
			SetScript(script).
			SetGasLimit(emulator.MaxGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
		assert.NoError(t, err)

		err = b.AddTransaction(*tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		require.NoError(t, err)

		location := runtime.AddressContractLocation{
			AddressLocation: address.Bytes(),
			Name:            "Test",
		}
		expectedType := fmt.Sprintf("%s.Test.MyEvent", location.ID())

		events, err := b.GetEventsByHeight(block.Header.Height, expectedType)
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0]

		decodedEvent := actualEvent.Value

		expectedID := flow.Event{TransactionID: tx.ID(), EventIndex: 0}.ID()

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, cadence.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, cadence.NewInt(2), decodedEvent.Fields[1])
	})
}
