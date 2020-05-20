package examples

import (
	"io/ioutil"
	"testing"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/cmd"
	"github.com/onflow/flow-go-sdk"

	emulator "github.com/dapperlabs/flow-emulator"
)

const DefaultGasLimit = 100

// ReadFile reads a file from the file system
func ReadFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// NewEmulator returns a emulator object for testing
func NewEmulator() *emulator.Blockchain {
	b, err := emulator.NewBlockchain()
	if err != nil {
		panic(err)
	}
	return b
}

// SignAndSubmit signs a transaction with an array of signers and adds their signatures to the transaction
// Then submits the transaction to the emulator. If the private keys don't match up with the addresses,
// the transaction will not succeed.
// shouldRevert parameter indicates whether the transaction should fail or not
// This function asserts the correct result and commits the block if it passed
func SignAndSubmit(
	t *testing.T,
	b *emulator.Blockchain,
	tx *flow.Transaction,
	signerAddresses []flow.Address,
	signers []crypto.Signer,
	shouldRevert bool,
) {
	// sign transaction with each signer
	for i := len(signerAddresses) - 1; i >= 0; i-- {
		signerAddress := signerAddresses[i]
		signer := signers[i]

		if i == 0 {
			err := tx.SignEnvelope(signerAddress, 0, signer)
			assert.NoError(t, err)
		} else {
			err := tx.SignPayload(signerAddress, 0, signer)
			assert.NoError(t, err)
		}
	}

	Submit(t, b, tx, shouldRevert)
}

// Submit submits a transaction and checks if it has succeeded or failed
func Submit(
	t *testing.T,
	b *emulator.Blockchain,
	tx *flow.Transaction,
	shouldRevert bool,
) {
	// submit the signed transaction
	err := b.AddTransaction(*tx)
	require.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	require.NoError(t, err)

	if shouldRevert {
		assert.False(t, result.Succeeded())
	} else {
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
			cmd.PrettyPrintError(result.Error, "", map[string]string{"": ""})
		}
	}

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

// setupUsersTokens sets up two accounts with an empty Vault
// and a NFT collection
func setupUsersTokens(
	t *testing.T,
	b *emulator.Blockchain,
	tokenAddr flow.Address,
	nftAddr flow.Address,
	signerAddresses []flow.Address,
	signerKeys []*flow.AccountKey,
	signers []crypto.Signer,
) {
	// add array of signers to transaction
	for i := 0; i < len(signerAddresses); i++ {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateTokenScript(tokenAddr)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(signerAddresses[i])

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, signerAddresses[i]},
			[]crypto.Signer{b.RootKey().Signer(), signers[i]},
			false,
		)

		tx = flow.NewTransaction().
			SetScript(GenerateCreateCollectionScript(nftAddr)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(signerAddresses[i])

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, signerAddresses[i]},
			[]crypto.Signer{b.RootKey().Signer(), signers[i]},
			false,
		)
	}
}
