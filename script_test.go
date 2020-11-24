package emulator_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
)

func TestExecuteScript(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, counterAddress := deployAndGenerateAddTwoScript(t, b)

	tx := flow.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().Signer())
	assert.NoError(t, err)

	callScript := generateGetCounterCountScript(counterAddress, b.ServiceKey().Address)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript), nil)
	require.NoError(t, err)
	assert.Equal(t, cadence.NewInt(0), scriptResult.Value)

	// Submit tx (script adds 2)
	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assertTransactionSucceeded(t, txResult)

	t.Run("BeforeCommit", func(t *testing.T) {
		t.Skip("TODO: fix stored ledger")

		// Sample call (value is still 0)
		result, err := b.ExecuteScript([]byte(callScript), nil)
		require.NoError(t, err)
		assert.Equal(t, cadence.NewInt(0), result.Value)
	})

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	t.Run("AfterCommit", func(t *testing.T) {
		// Sample call (value is 2)
		result, err := b.ExecuteScript([]byte(callScript), nil)
		require.NoError(t, err)
		assert.Equal(t, cadence.NewInt(2), result.Value)
	})
}

func TestExecuteScript_WithArguments(t *testing.T) {
	t.Run("Int", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		scriptWithArgs := `
			pub fun main(n: Int): Int {
				return n
			}
		`

		arg, err := jsoncdc.Encode(cadence.NewInt(10))
		require.NoError(t, err)

		scriptResult, err := b.ExecuteScript([]byte(scriptWithArgs), [][]byte{arg})
		require.NoError(t, err)

		assert.Equal(t, cadence.NewInt(10), scriptResult.Value)
	})
	t.Run("String", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		scriptWithArgs := `
			pub fun main(n: String): Int {
				log(n)
				return 0
			}
		`

		arg, err := jsoncdc.Encode(cadence.NewString("Hello, World"))
		require.NoError(t, err)
		scriptResult, err := b.ExecuteScript([]byte(scriptWithArgs), [][]byte{arg})
		require.NoError(t, err)
		assert.Contains(t, scriptResult.Logs, "\"Hello, World\"")
	})
}

func TestExecuteScriptAtBlockHeight(t *testing.T) {
	// TODO
	// Test that scripts can be executed at different block heights
}
