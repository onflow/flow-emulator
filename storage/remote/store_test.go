//go:build integration

/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
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

package remote

import (
	"testing"

	flowsdk "github.com/onflow/flow-go-sdk"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emulator "github.com/onflow/flow-emulator"
)

func Test_SimulatedMainnetTransaction(t *testing.T) {
	t.Parallel()
	remoteStore, err := New(WithChainID(flowgo.Mainnet))
	require.NoError(t, err)

	b, err := emulator.NewBlockchain(
		emulator.WithStore(remoteStore),
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionValidationEnabled(false),
		emulator.WithChainID(flowgo.Mainnet),
	)
	require.NoError(t, err)

	script := []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.echo()
			}
		}
	`)
	addr := flowsdk.HexToAddress("0x9799f28ff0453528")
	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	txRes, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	//tx1Result, err := b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.NoError(t, txRes.Error)
	assert.Len(t, txRes.Events, 1)
	assert.Equal(t, txRes.Events[0].String(), "A.9799f28ff0453528.Ping.PingEmitted: 0x953f6f26d61710cb0e140bfde1022483b9ef410ddd181bac287d9968c84f4778")
	assert.Equal(t, txRes.Events[0].Value.String(), `A.9799f28ff0453528.Ping.PingEmitted(sound: "ping ping ping")`)
}

func Test_SimulatedMainnetTransactionWithChanges(t *testing.T) {
	t.Parallel()
	remoteStore, err := New(WithChainID(flowgo.Mainnet))
	require.NoError(t, err)

	b, err := emulator.NewBlockchain(
		emulator.WithStore(remoteStore),
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionValidationEnabled(false),
		emulator.WithChainID(flowgo.Mainnet),
	)
	require.NoError(t, err)

	script := []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.sound = "pong pong pong"
			}
		}
	`)
	addr := flowsdk.HexToAddress("0x9799f28ff0453528")
	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	txRes, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.NoError(t, txRes.Error)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	script = []byte(`
		import Ping from 0x9799f28ff0453528
		
		transaction {
			execute {
				Ping.echo()
			}
		}
	`)
	tx = flowsdk.NewTransaction().
		SetScript(script).
		SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(addr, 0, 0).
		SetPayer(addr)

	err = b.AddTransaction(*tx)
	assert.NoError(t, err)

	txRes, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	//tx1Result, err := b.GetTransactionResult(tx.ID())
	assert.NoError(t, err)
	assert.NoError(t, txRes.Error)
	assert.Len(t, txRes.Events, 1)
	assert.Equal(t, txRes.Events[0].String(), "A.9799f28ff0453528.Ping.PingEmitted: 0x953f6f26d61710cb0e140bfde1022483b9ef410ddd181bac287d9968c84f4778")
	assert.Equal(t, txRes.Events[0].Value.String(), `A.9799f28ff0453528.Ping.PingEmitted(sound: "pong pong pong")`)
}
