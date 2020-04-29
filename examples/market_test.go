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
	tokenAccountKey, tokenSigner := accountKeys.NewWithSigner()
	tokenAddr, err := b.CreateAccount([]*flow.AccountKey{tokenAccountKey}, tokenCode)
	assert.Nil(t, err)
	_, err = b.CommitBlock()
	require.NoError(t, err)

	nftCode := ReadFile(NFTContractFile)
	nftAccountKey, nftSigner := accountKeys.NewWithSigner()
	nftAddr, err := b.CreateAccount([]*flow.AccountKey{nftAccountKey}, nftCode)
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

		tx := flow.NewTransaction().
			SetScript(GenerateMintTokensScript(tokenAddr, joshAddress, 30)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(tokenAddr)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, tokenAddr},
			[]crypto.Signer{b.RootKey().Signer(), tokenSigner},
			false,
		)

		tx = flow.NewTransaction().
			SetScript(GenerateMintNFTScript(nftAddr, bastianAddress)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(nftAddr)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address, nftAddr},
			[]crypto.Signer{b.RootKey().Signer(), nftSigner},
			false,
		)
	})

	t.Run("Can create sale collection", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateSaleScript(tokenAddr, marketAddr, 0.15)).
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

		result, err := b.ExecuteScript(GenerateInspectSaleLenScript(marketAddr, bastianAddress, 0))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

	})

	t.Run("Can put an NFT up for sale", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateStartSaleScript(nftAddr, marketAddr, 0, 10)).
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

		// Assert that the account's collection is correct
		result, err := b.ExecuteScript(GenerateInspectSaleScript(marketAddr, bastianAddress, 0, 10))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectSaleLenScript(marketAddr, bastianAddress, 1))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})

	t.Run("Cannot buy an NFT for less than the sale price", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 0, 9)).
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
			SetScript(GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 0, 10)).
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

		result, err := b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, bastianAddress, 8.5))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, joshAddress, 20))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, marketAddr, 1.5))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the accounts' collections are correct
		result, err = b.ExecuteScript(GenerateInspectCollectionLenScript(nftAddr, bastianAddress, 0))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, joshAddress, 0))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})

}
