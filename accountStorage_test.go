package emulator

import (
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStorageTransaction(t *testing.T) {
	t.Parallel()

	const limit = 1000

	b, err := NewBlockchain(
		WithStorageLimitEnabled(false),
		WithTransactionMaxGasLimit(limit),
	)
	require.NoError(t, err)

	accountKeys := test.AccountKeyGenerator()
	accountKey, signer := accountKeys.NewWithSigner()
	accountAddress, err := b.CreateAccount([]*flow.AccountKey{accountKey}, nil)
	assert.NoError(t, err)

	const code = `transaction {
		  prepare(signer: AuthAccount) {
			  	signer.save("storage value", to: /storage/storageTest)
 				signer.link<&String>(/public/publicTest, target: /storage/storageTest)
				signer.link<&String>(/private/privateTest, target: /storage/storageTest)
		  }
   		}
    `

	tx1 := flow.NewTransaction().
		SetScript([]byte(code)).
		SetGasLimit(limit).
		SetProposalKey(accountAddress, 0, 0).
		SetPayer(accountAddress).
		AddAuthorizer(accountAddress)

	err = tx1.SignEnvelope(accountAddress, 0, signer)
	assert.NoError(t, err)

	err = b.AddTransaction(*tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	accountStorage, err := b.GetAccountStorage(accountAddress)
	assert.NoError(t, err)

	require.NotNil(t, accountStorage.Public.Get("publicTest"))
	require.NotNil(t, accountStorage.Storage.Get("storageTest"))
	require.NotNil(t, accountStorage.Private.Get("privateTest"))
	assert.Equal(t, accountStorage.Public.Get("publicTest").String(), `Link<&String>(/storage/storageTest)`)
	assert.Equal(t, accountStorage.Storage.Get("storageTest").String(), `"storage value"`)
	assert.Equal(t, accountStorage.Private.Get("privateTest").String(), `Link<&String>(/storage/storageTest)`)
}
