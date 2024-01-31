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
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
)

func TestCoverageReport(t *testing.T) {

	t.Parallel()

	coverageReport := runtime.NewCoverageReport()
	b, err := emulator.New(
		emulator.WithCoverageReport(coverageReport),
	)
	require.NoError(t, err)

	coverageReport.Reset()
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

	callScript := GenerateGetCounterCountScript(counterAddress, b.ServiceKey().Address)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript), nil)
	require.NoError(t, err)
	assert.Equal(t, cadence.NewInt(0), scriptResult.Value)

	// Submit tx (script adds 2)
	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	AssertTransactionSucceeded(t, txResult)

	address, err := common.HexToAddress(counterAddress.Hex())
	require.NoError(t, err)
	location := common.AddressLocation{
		Address: address,
		Name:    "Counting",
	}
	coverage := coverageReport.Coverage[location]

	assert.Equal(t, []int{}, coverage.MissedLines())
	assert.Equal(t, 4, coverage.Statements)
	assert.Equal(t, "100.0%", coverage.Percentage())
	assert.EqualValues(
		t,
		map[int]int{
			11: 1, 15: 1, 16: 1, 21: 1,
		},
		coverage.LineHits,
	)
}
