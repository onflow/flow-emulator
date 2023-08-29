/*
 * Flow Emulator
 *
 * Copyright Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
		access(all) fun main() {
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

	txCode := `
		//some comment 
		#sourceFile("transaction.cdc")	
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
				access(all) contract C {
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

func TestSourceFileCommentedOutPragmaForContract(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithTransactionValidationEnabled(false),
	)
	b.EnableAutoMine()

	require.NoError(t, err)
	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	contract := `//#sourceFile("contracts/C.cdc")		
				access(all) contract C {
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

	assert.Equal(t, "A.f8d6e0586b0a20c7.C", b.GetSourceFile(common.NewAddressLocation(
		nil,
		common.Address(b.ServiceKey().Address),
		"C",
	)))
}
