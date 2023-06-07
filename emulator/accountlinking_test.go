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
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"

	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"
)

func TestAccountLinking(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := adapters.NewSDKAdapter(&logger, b)

	script := []byte(`
		#allowAccountLinking
		transaction {
			prepare(acct: AuthAccount) {
				acct.linkAccount(/private/foo)
			}
		}
	`)

	serviceAccountAddress := b.ServiceKey().Address

	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		AddAuthorizer(serviceAccountAddress).
		SetProposalKey(
			serviceAccountAddress,
			b.ServiceKey().Index,
			b.ServiceKey().SequenceNumber,
		).
		SetPayer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	block, err := b.CommitBlock()
	require.NoError(t, err)

	expectedType := "flow.AccountLinked"

	events, err := adapter.GetEventsForBlockIDs(
		context.Background(),
		expectedType,
		[]flowsdk.Identifier{
			flowsdk.Identifier(block.Header.ID()),
		},
	)
	require.NoError(t, err)
	require.Len(t, events, 1)

	actualEvent := events[0].Events[0]
	decodedEvent := actualEvent.Value
	expectedID := flowsdk.Event{
		TransactionID: tx.ID(),
		EventIndex:    0,
	}.ID()

	assert.Equal(t, expectedType, actualEvent.Type)
	assert.Equal(t, expectedID, actualEvent.ID())
	expectedPath, err := cadence.NewPath(common.PathDomainPrivate, "foo")
	require.NoError(t, err)
	require.Len(t, decodedEvent.Fields, 2)
	assert.Equal(t, expectedPath, decodedEvent.Fields[0])
	assert.Equal(t, cadence.NewAddress(serviceAccountAddress), decodedEvent.Fields[1])
}
