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
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/emulator"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-emulator/storage/sqlite"
)

func TestInitialization(t *testing.T) {

	t.Parallel()

	t.Run("should inject initial state when initialized with empty store", func(t *testing.T) {

		t.Parallel()

		file, err := os.CreateTemp("", "sqlite-test")
		require.Nil(t, err)
		store, err := sqlite.New(file.Name())
		require.Nil(t, err)
		defer store.Close()

		b, _ := emulator.New(emulator.WithStore(store))
		logger := zerolog.Nop()
		adapter := adapters.NewSDKAdapter(&logger, b)

		serviceAcct, err := adapter.GetAccount(context.Background(), flowsdk.ServiceAddress(flowsdk.Emulator))
		require.NoError(t, err)

		assert.NotNil(t, serviceAcct)

		latestBlock, err := b.GetLatestBlock()
		require.NoError(t, err)

		assert.EqualValues(t, 0, latestBlock.Header.Height)
		assert.Equal(t,
			flowgo.Genesis(flowgo.Emulator).ID(),
			latestBlock.ID(),
		)
	})

	t.Run("should restore state when initialized with non-empty store", func(t *testing.T) {

		t.Parallel()

		file, err := os.CreateTemp("", "sqlite-test")
		require.Nil(t, err)
		store, err := sqlite.New(file.Name())
		require.Nil(t, err)
		defer store.Close()

		b, _ := emulator.New(emulator.WithStore(store), emulator.WithStorageLimitEnabled(false))
		logger := zerolog.Nop()
		adapter := adapters.NewSDKAdapter(&logger, b)

		contracts := []templates.Contract{
			{
				Name:   "Counting",
				Source: counterScript,
			},
		}

		counterAddress, err := adapter.CreateAccount(context.Background(), nil, contracts)
		require.NoError(t, err)

		// Submit a transaction adds some ledger state and event state
		script := fmt.Sprintf(
			`
                import 0x%s

                transaction {

                  prepare(acct: auth(Storage, Capabilities) &Account) {

                    let counter <- Counting.createCounter()
                    counter.add(1)

                    acct.storage.save(<-counter, to: /storage/counter)

                    let counterCap = acct.capabilities.storage.issue<&Counting.Counter>(/storage/counter)
                    acct.capabilities.publish(counterCap, at: /public/counter)
                  }
                }
            `,
			counterAddress,
		)

		tx := flowsdk.NewTransaction().
			SetScript([]byte(script)).
			SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(b.ServiceKey().Address, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(b.ServiceKey().Address).
			AddAuthorizer(b.ServiceKey().Address)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(b.ServiceKey().Address, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		require.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		assert.NoError(t, err)
		require.NotNil(t, block)

		minedTx, err := adapter.GetTransaction(context.Background(), tx.ID())
		require.NoError(t, err)

		minedEvents, err := adapter.GetEventsForHeightRange(context.Background(), "", block.Header.Height, block.Header.Height)
		require.NoError(t, err)

		// Create a new emulator with the same store
		b, _ = emulator.New(emulator.WithStore(store))
		adapter = adapters.NewSDKAdapter(&logger, b)

		t.Run("should be able to read blocks", func(t *testing.T) {
			latestBlock, _, err := adapter.GetLatestBlock(context.Background(), true)
			require.NoError(t, err)

			assert.Equal(t, flowsdk.Identifier(block.ID()), latestBlock.ID)

			blockByHeight, _, err := adapter.GetBlockByHeight(context.Background(), block.Header.Height)
			require.NoError(t, err)

			assert.Equal(t, flowsdk.Identifier(block.ID()), blockByHeight.ID)

			blockByHash, _, err := adapter.GetBlockByID(context.Background(), convert.FlowIdentifierToSDK(block.ID()))
			require.NoError(t, err)

			assert.Equal(t, flowsdk.Identifier(block.ID()), blockByHash.ID)
		})

		t.Run("should be able to read transactions", func(t *testing.T) {
			txByHash, err := adapter.GetTransaction(context.Background(), tx.ID())
			require.NoError(t, err)

			assert.Equal(t, minedTx, txByHash)
		})

		t.Run("should be able to read events", func(t *testing.T) {
			gotEvents, err := adapter.GetEventsForHeightRange(context.Background(), "", block.Header.Height, block.Header.Height)
			require.NoError(t, err)

			assert.Equal(t, minedEvents[0].Events, gotEvents[0].Events)
		})

		t.Run("should be able to read ledger state", func(t *testing.T) {
			readScript := fmt.Sprintf(
				`
                  import 0x%s

                  access(all) fun main(): Int {
                      return getAccount(0x%s).capabilities.borrow<&Counting.Counter>(/public/counter)?.count ?? 0
                  }
                `,
				counterAddress,
				b.ServiceKey().Address,
			)

			result, err := b.ExecuteScript([]byte(readScript), nil)
			require.NoError(t, err)

			assert.Equal(t, cadence.NewInt(1), result.Value)
		})
	})
}
