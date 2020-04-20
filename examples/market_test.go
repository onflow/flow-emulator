package examples

import (
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	MarketContractFile = "./contracts/market.cdc"
)

func TestMarketDeployment(t *testing.T) {
	b := NewEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(resourceTokenContractFile)
	_, err := b.CreateAccount(nil, tokenCode)
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	_, err = b.CommitBlock()
	require.NoError(t, err)

	nftCode := ReadFile(NFTContractFile)
	_, err = b.CreateAccount(nil, nftCode)
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	_, err = b.CommitBlock()
	require.NoError(t, err)

	// Should be able to deploy a contract as a new account with no keys.
	marketCode := ReadFile(MarketContractFile)
	_, err = b.CreateAccount(nil, marketCode)
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	_, err = b.CommitBlock()
	require.NoError(t, err)
}

func TestCreateSale(t *testing.T) {
	b := NewEmulator()

	accountKeys := test.AccountKeyGenerator()

	// first deploy the FT, NFT, and market code
	tokenCode := ReadFile(resourceTokenContractFile)
	tokenAddr, err := b.CreateAccount(nil, tokenCode)
	assert.Nil(t, err)
	_, err = b.CommitBlock()
	require.NoError(t, err)

	nftCode := ReadFile(NFTContractFile)
	nftAddr, err := b.CreateAccount(nil, nftCode)
	assert.Nil(t, err)
	_, err = b.CommitBlock()
	require.NoError(t, err)

	marketCode := ReadFile(MarketContractFile)
	marketAddr, err := b.CreateAccount(nil, marketCode)
	assert.Nil(t, err)
	_, err = b.CommitBlock()
	require.NoError(t, err)

	// create two new accounts
	bastianAccountKey, bastianSigner := accountKeys.NewWithSigner()
	bastianAddress, err := b.CreateAccount([]*flow.AccountKey{bastianAccountKey}, nil)

	joshAccountKey, joshSigner := accountKeys.NewWithSigner()
	joshAddress, err := b.CreateAccount([]*flow.AccountKey{joshAccountKey}, nil)

	t.Run("Should be able to create FTs and NFT collections in each accounts storage", func(t *testing.T) {
		// create Fungible tokens and NFTs in each accounts storage and store references
		setupUsersTokens(
			t, b, tokenAddr, nftAddr,
			[]flow.Address{bastianAddress, joshAddress},
			[]*flow.AccountKey{bastianAccountKey, joshAccountKey},
			[]crypto.Signer{bastianSigner, joshSigner},
		)
	})

	t.Run("Can create sale collection", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateSaleScript(tokenAddr, marketAddr)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(bastianAddress)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, bastianAddress},
			[]crypto.Signer{b.RootKey().Signer(), bastianSigner},
			false,
		)
	})

	t.Run("Can put an NFT up for sale", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateStartSaleScript(nftAddr, marketAddr, 1, 10)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(bastianAddress)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, bastianAddress},
			[]crypto.Signer{b.RootKey().Signer(), bastianSigner},
			false,
		)
	})

	t.Run("Cannot buy an NFT for less than the sale price", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 1, 9)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(joshAddress)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, joshAddress},
			[]crypto.Signer{b.RootKey().Signer(), joshSigner},
			true,
		)
	})

	t.Run("Cannot buy an NFT that is not for sale", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 2, 10)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(joshAddress)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, joshAddress},
			[]crypto.Signer{b.RootKey().Signer(), joshSigner},
			true,
		)
	})

	t.Run("Can buy an NFT that is for sale", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 1, 10)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(joshAddress)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, joshAddress},
			[]crypto.Signer{b.RootKey().Signer(), joshSigner},
			false,
		)

		result, err := b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, bastianAddress, 40))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, joshAddress, 20))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the accounts' collections are correct
		result, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, bastianAddress, 1, false))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, bastianAddress, 2, false))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, joshAddress, 1, true))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, joshAddress, 2, true))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})

}
