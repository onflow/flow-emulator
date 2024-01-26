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
	"fmt"
	"testing"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
)

func TestEventEmitted(t *testing.T) {

	t.Parallel()

	t.Run("EmittedFromScript", func(t *testing.T) {

		t.Parallel()

		// Emitting events in scripts is not supported

		b, err := emulator.New()
		require.NoError(t, err)

		script := []byte(`
			access(all) event MyEvent(x: Int, y: Int)

			access(all) fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		result, err := b.ExecuteScript(script, nil)
		assert.NoError(t, err)
		require.NoError(t, result.Error)
		require.Empty(t, result.Events)
	})

	t.Run("EmittedFromAccount", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New(
			emulator.WithStorageLimitEnabled(false),
		)
		require.NoError(t, err)

		logger := zerolog.Nop()
		adapter := adapters.NewSDKAdapter(&logger, b)

		accountContracts := []templates.Contract{
			{
				Name: "Test",
				Source: `
                    access(all) contract Test {
						access(all) event MyEvent(x: Int, y: Int)

						access(all) fun emitMyEvent(x: Int, y: Int) {
							emit MyEvent(x: x, y: y)
						}
					}
				`,
			},
		}

		publicKey := b.ServiceKey().AccountKey()

		address, err := adapter.CreateAccount(
			context.Background(),
			[]*flowsdk.AccountKey{publicKey},
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

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		require.NoError(t, err)

		addr, _ := common.BytesToAddress(address.Bytes())
		location := common.AddressLocation{
			Address: addr,
			Name:    "Test",
		}
		expectedType := location.TypeID(nil, "Test.MyEvent")

		events, err := adapter.GetEventsForHeightRange(context.Background(), string(expectedType), block.Header.Height, block.Header.Height)
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0].Events[0]
		decodedEvent := actualEvent.Value
		decodedEventType := decodedEvent.Type().(*cadence.EventType)
		expectedID := flowsdk.Event{TransactionID: tx.ID(), EventIndex: 0}.ID()

		assert.Equal(t, string(expectedType), actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, "x", decodedEventType.Fields[0].Identifier)
		assert.Equal(t, cadence.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, "y", decodedEventType.Fields[1].Identifier)
		assert.Equal(t, cadence.NewInt(2), decodedEvent.Fields[1])

		events, err = adapter.GetEventsForBlockIDs(context.Background(), string(expectedType), []flowsdk.Identifier{flowsdk.Identifier(block.Header.ID())})
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent = events[0].Events[0]
		decodedEvent = actualEvent.Value
		decodedEventType = decodedEvent.Type().(*cadence.EventType)
		expectedID = flowsdk.Event{TransactionID: tx.ID(), EventIndex: 0}.ID()

		assert.Equal(t, string(expectedType), actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, "x", decodedEventType.Fields[0].Identifier)
		assert.Equal(t, cadence.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, "y", decodedEventType.Fields[1].Identifier)
		assert.Equal(t, cadence.NewInt(2), decodedEvent.Fields[1])

	})
}
