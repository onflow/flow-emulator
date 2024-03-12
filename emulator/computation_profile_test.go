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
	"testing"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
)

func TestComputationProfileForScript(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithComputationProfiling(true),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	addTwoScript, counterAddress := DeployAndGenerateAddTwoScript(t, adapter)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Submit tx (script adds 2)
	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	AssertTransactionSucceeded(t, txResult)

	callScript := GenerateGetCounterCountScript(counterAddress, b.ServiceKey().Address)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript), nil)
	require.NoError(t, err)
	assert.Equal(t, cadence.NewInt(0), scriptResult.Value)

	computationProfile := b.ComputationProfile()
	require.NotNil(t, computationProfile)
	require.Len(t, computationProfile.Scripts, 1)

	scriptProfile := computationProfile.Scripts[0]
	assert.Equal(t, scriptResult.ScriptID.String(), scriptProfile.ID)
	assert.Equal(t, uint64(2), scriptProfile.ComputationUsed)

	expectedIntensities := map[string]uint{
		"FunctionInvocation":     2,
		"GetAccountContractCode": 1,
		"GetCode":                1,
		"GetOrLoadProgram":       3,
		"GetValue":               221,
		"ResolveLocation":        1,
		"Statement":              1,
	}
	assert.Equal(t, expectedIntensities, scriptProfile.Intensities)
}

func TestComputationProfileForTransaction(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithComputationProfiling(true),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(b.ServiceKey().Address).
		AddAuthorizer(b.ServiceKey().Address)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Submit tx (script adds 2)
	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	AssertTransactionSucceeded(t, txResult)

	computationProfile := b.ComputationProfile()
	require.NotNil(t, computationProfile)
	// The 1st transaction creates a new account and deploys the Counting contract.
	// The 2nd transaction interacts with the Counting contract.
	require.Len(t, computationProfile.Transactions, 2)

	txProfile := computationProfile.Transactions[1]
	assert.Equal(t, tx.ID().String(), txProfile.ID)
	assert.Equal(t, uint64(57), txProfile.ComputationUsed)

	expectedIntensities := map[string]uint{
		"AllocateStorageIndex":   2,
		"CreateCompositeValue":   2,
		"CreateDictionaryValue":  1,
		"EmitEvent":              73,
		"EncodeEvent":            1,
		"EncodeValue":            2336,
		"FunctionInvocation":     9,
		"GenerateAccountLocalID": 1,
		"GenerateUUID":           1,
		"GetAccountContractCode": 1,
		"GetCode":                1,
		"GetOrLoadProgram":       3,
		"GetValue":               2249,
		"ResolveLocation":        1,
		"SetValue":               2473,
		"Statement":              11,
		"TransferCompositeValue": 3,
	}
	assert.Equal(t, expectedIntensities, txProfile.Intensities)
}
