package emulator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	flowsdk "github.com/onflow/flow-go-sdk"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteScript(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	addTwoScript, counterAddress := DeployAndGenerateAddTwoScript(t, adapter)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	callScript := GenerateGetCounterCountScript(counterAddress, b.ServiceKey().Address)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript), nil)
	require.NoError(t, err)
	assert.Equal(t, cadence.NewInt(0), scriptResult.Value)

	// Submit tx (script adds 2)
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, txResult)

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

	t.Parallel()

	t.Run("Int", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New()
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

		t.Parallel()

		b, err := emulator.New()
		require.NoError(t, err)

		scriptWithArgs := `
			pub fun main(n: String): Int {
				log(n)
				return 0
			}
		`

		arg, err := jsoncdc.Encode(cadence.String("Hello, World"))
		require.NoError(t, err)
		scriptResult, err := b.ExecuteScript([]byte(scriptWithArgs), [][]byte{arg})
		require.NoError(t, err)
		assert.Contains(t, scriptResult.Logs, "\"Hello, World\"")
	})
}

func TestExecuteScript_FlowServiceAccountBalance(t *testing.T) {

	t.Parallel()

	b, err := emulator.New()
	require.NoError(t, err)

	code := fmt.Sprintf(
		`
          import FlowServiceAccount from %[1]s
          pub fun main(): UFix64 {
            let acct = getAccount(%[1]s)
            return FlowServiceAccount.defaultTokenBalance(acct)
          }
        `,
		b.GetChain().ServiceAddress().HexWithPrefix(),
	)

	res, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)
	require.NoError(t, res.Error)

	require.Positive(t, res.Value)
}

func TestInfiniteScript(t *testing.T) {

	t.Parallel()

	const limit = 90
	b, err := emulator.New(
		emulator.WithScriptGasLimit(limit),
	)
	require.NoError(t, err)

	const code = `
	    pub fun main() {
	        main()
	    }
	`
	result, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)

	require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
}

func TestScriptWithExceeedingComputationLimit(t *testing.T) {

	t.Parallel()

	const limit = 2000
	b, err := emulator.New(
		emulator.WithScriptGasLimit(limit),
	)
	require.NoError(t, err)

	const code = `
	    pub fun main() {
	        var s: Int256 = 1024102410241024
	        var i: Int256 = 0
	        var a: Int256 = 7
	        var b: Int256 = 5
	        var c: Int256 = 2

	        while i < 150000 {
	            s = s * a
	            s = s / b
	            s = s / c
	            i = i + 1
	        }
	    }
	`
	result, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)

	require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
}

func TestScriptWithSufficientComputationLimit(t *testing.T) {

	t.Parallel()

	const limit = 19000
	b, err := emulator.New(
		emulator.WithScriptGasLimit(limit),
	)
	require.NoError(t, err)

	const code = `
	    pub fun main() {
	        var s: Int256 = 1024102410241024
	        var i: Int256 = 0
	        var a: Int256 = 7
	        var b: Int256 = 5
	        var c: Int256 = 2

	        while i < 150000 {
	            s = s * a
	            s = s / b
	            s = s / c
	            i = i + 1
	        }
	    }
	`
	result, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)
	require.NoError(t, result.Error)
}
