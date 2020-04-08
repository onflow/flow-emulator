package examples

import (
	"testing"

	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go-sdk"
)

const (
	NFTContractFile = "./contracts/nft.cdc"
)

func TestNFTDeployment(t *testing.T) {
	b := NewEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(NFTContractFile)
	_, err := b.CreateAccount(nil, tokenCode, GetNonce())
	if !assert.NoError(t, err) {
		t.Log(err.Error())
	}
	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestCreateNFT(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(NFTContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.NoError(t, err)

	// Vault must be instantiated with a positive ID
	t.Run("Cannot create token with negative ID", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateNFTScript(contractAddr, -7)).
			SetGasLimit(10).
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateNFTScript(contractAddr, 1)).
			SetGasLimit(20).
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)
	})

	// Assert that the account's collection is correct
	result, err := b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, true))
	require.NoError(t, err)
	if !assert.True(t, result.Succeeded()) {
		t.Log(result.Error.Error())
	}

	// Assert that the account's collection doesn't contain ID 3
	result, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 3, true))
	require.NoError(t, err)
	assert.True(t, result.Reverted())
}

func TestTransferNFT(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(NFTContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.NoError(t, err)

	// then deploy a NFT to the root account
	tx := flow.NewTransaction().
		SetScript(GenerateCreateNFTScript(contractAddr, 1)).
		SetGasLimit(20).
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

	SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// Assert that the account's collection is correct
	result, err := b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, true))
	require.NoError(t, err)
	if !assert.True(t, result.Succeeded()) {
		t.Log(result.Error.Error())
	}

	// create a new account
	bastianPrivateKey := RandomPrivateKey()
	bastianPublicKey := bastianPrivateKey.ToAccountKey()
	bastianPublicKey.Weight = keys.PublicKeyWeightThreshold

	bastianAddress, err := b.CreateAccount([]flow.AccountKey{bastianPublicKey}, nil, GetNonce())

	// then deploy an NFT to another account
	tx = flow.NewTransaction().
		SetScript(GenerateCreateNFTScript(contractAddr, 2)).
		SetGasLimit(20).
		SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
		AddAuthorizer(bastianAddress, 0)

	SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)

	// transfer an NFT
	t.Run("Should be able to withdraw an NFT and deposit to another accounts collection", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateDepositScript(contractAddr, bastianAddress, 1)).
			SetGasLimit(20).
			SetPayer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID).
			AddAuthorizer(b.RootAccountAddress(), b.RootKey().ToAccountKey().ID)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		// Assert that the account's collection is correct
		result, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, bastianAddress, 1, true))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the account's collection is correct
		result, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, bastianAddress, 2, true))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the account's id keys are correct
		result, err = b.ExecuteScript(GenerateInspectKeysScript(contractAddr, bastianAddress, 2, 1))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the account's collection is correct
		result, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, false))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the account's collection is correct
		result, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 2, false))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})
}
