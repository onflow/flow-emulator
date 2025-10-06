/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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

	"github.com/onflow/cadence/common"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
)

func TestComputationReportingForScript(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithComputationReporting(true),
		// computation weights shifted by meter.MeterExecutionInternalPrecisionBytes
		// so that they correspond directly to computation used.
		emulator.WithExecutionEffortWeights(meter.ExecutionEffortWeights{
			common.ComputationKindFunctionInvocation:          2 << meter.MeterExecutionInternalPrecisionBytes,
			environment.ComputationKindGetCode:                3 << meter.MeterExecutionInternalPrecisionBytes,
			environment.ComputationKindGetAccountContractCode: 5 << meter.MeterExecutionInternalPrecisionBytes,
			environment.ComputationKindResolveLocation:        7 << meter.MeterExecutionInternalPrecisionBytes,
			common.ComputationKindStatement:                   11 << meter.MeterExecutionInternalPrecisionBytes,
		}),
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

	computationReport := b.ComputationReport()
	require.NotNil(t, computationReport)
	require.Len(t, computationReport.Scripts, 1)

	scriptProfile := computationReport.Scripts[scriptResult.ScriptID.String()]
	assert.GreaterOrEqual(t, scriptProfile.ComputationUsed, uint64(2*2+3+5+7+11))

	expectedIntensities := map[string]uint64{
		"FunctionInvocation":     2,
		"GetAccountContractCode": 1,
		"GetCode":                1,
		"ResolveLocation":        1,
		"Statement":              1,
	}
	for kind, intensity := range expectedIntensities {
		assert.GreaterOrEqual(t, scriptProfile.Intensities[kind], intensity)
	}
}

func TestComputationReportingForTransaction(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithComputationReporting(true),
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

	computationReport := b.ComputationReport()
	require.NotNil(t, computationReport)
	// The 1st transaction creates a new account and deploys the Counting contract.
	// The 2nd transaction interacts with the Counting contract.
	require.Len(t, computationReport.Transactions, 2)

	txProfile := computationReport.Transactions[txResult.TransactionID.String()]
	assert.GreaterOrEqual(t, txProfile.ComputationUsed, uint64(1))

	expectedIntensities := map[string]uint64{
		"CreateCompositeValue":   1,
		"CreateDictionaryValue":  1,
		"EmitEvent":              73,
		"EncodeEvent":            1,
		"FunctionInvocation":     9,
		"GenerateAccountLocalID": 1,
		"GenerateUUID":           1,
		"GetAccountContractCode": 1,
		"GetCode":                1,
		"ResolveLocation":        1,
		"Statement":              11,
		"TransferCompositeValue": 3,
	}
	for kind, intensity := range expectedIntensities {
		assert.GreaterOrEqualf(t, txProfile.Intensities[kind], intensity, "expected intensity for kind %s to be greater or equal to %d", kind, intensity)
	}
}
