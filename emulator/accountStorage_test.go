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
package emulator_test

import (
	"context"
	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"testing"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAccountStorageTests(t *testing.T) (
	*emulator.Blockchain,
	*adapters.SDKAdapter,
) {
	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	return b, adapters.NewSDKAdapter(&logger, b)
}

func TestStorageTransaction(t *testing.T) {
	t.Parallel()

	b, adapter := setupAccountStorageTests(t)

	accountKeys := test.AccountKeyGenerator()
	accountKey, signer := accountKeys.NewWithSigner()

	accountAddress, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKey}, nil)
	assert.NoError(t, err)

	const code = `transaction {
		  prepare(signer: AuthAccount) {
			  	signer.save("storage value", to: /storage/storageTest)
 				signer.link<&String>(/public/publicTest, target: /storage/storageTest)
				signer.link<&String>(/private/privateTest, target: /storage/storageTest)
		  }
   		}
    `

	tx1 := flowsdk.NewTransaction().
		SetScript([]byte(code)).
		SetGasLimit(1000).
		SetProposalKey(accountAddress, 0, 0).
		SetPayer(accountAddress).
		AddAuthorizer(accountAddress)

	err = tx1.SignEnvelope(accountAddress, 0, signer)
	assert.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	accountStorage, err := b.GetAccountStorage(flowgo.Address(accountAddress))
	assert.NoError(t, err)

	assert.NotNil(t, accountStorage.Public.Get("publicTest"))
	require.NotNil(t, accountStorage.Storage.Get("storageTest"))
	assert.NotNil(t, accountStorage.Private.Get("privateTest"))
	assert.Equal(t, accountStorage.Public.Get("publicTest").String(), `PathLink<&String>(/storage/storageTest)`)
	assert.Equal(t, accountStorage.Storage.Get("storageTest").String(), `"storage value"`)
	assert.Equal(t, accountStorage.Private.Get("privateTest").String(), `PathLink<&String>(/storage/storageTest)`)
}
