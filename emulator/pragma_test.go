package emulator_test

import (
	"context"
	"encoding/hex"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSourceFilePragmaForScript(t *testing.T) {

	t.Parallel()

	b, err := emulator.New()
	require.NoError(t, err)

	scriptCode := `#sourceFile("script.cdc")			
		pub fun main() {
			log("42")
		}`

	scriptResult, err := b.ExecuteScript([]byte(scriptCode), [][]byte{})
	require.NoError(t, err)
	require.NoError(t, scriptResult.Error)

	assert.Equal(t, "script.cdc", b.GetSourceFile(common.NewScriptLocation(nil, scriptResult.ScriptID.Bytes())))
}

func TestSourceFilePragmaForTransaction(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithTransactionValidationEnabled(false),
	)
	b.EnableAutoMine()

	require.NoError(t, err)
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	txCode := `#sourceFile("transaction.cdc")	
		transaction{
		}
	`
	tx := flowsdk.NewTransaction().
		SetScript([]byte(txCode)).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber+1).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	txID, _ := hex.DecodeString(tx.ID().String())

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	assert.Equal(t, "transaction.cdc", b.GetSourceFile(common.NewTransactionLocation(nil, txID)))
}

func TestSourceFilePragmaForContract(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithTransactionValidationEnabled(false),
	)
	b.EnableAutoMine()

	require.NoError(t, err)
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	contract := `#sourceFile("contracts/C.cdc")		
				pub contract C {
				}`

	tx := templates.AddAccountContract(
		b.ServiceKey().Address,
		templates.Contract{
			Name:   "C",
			Source: contract,
		})

	tx.SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber+1).
		SetPayer(b.ServiceKey().Address)

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	assert.Equal(t, "contracts/C.cdc", b.GetSourceFile(common.NewAddressLocation(
		nil,
		common.Address(b.ServiceKey().Address),
		"C",
	)))
}
