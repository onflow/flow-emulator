package examples

import (
	"testing"

	"github.com/dapperlabs/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"
)

const (
	resourceTokenContractFile = "./contracts/fungible-token.cdc"
)

func TestTokenDeployment(t *testing.T) {
	b := NewEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(resourceTokenContractFile)
	_, err := b.CreateAccount(nil, tokenCode)
	assert.NoError(t, err)
	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestCreateToken(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode)
	assert.NoError(t, err)

	// Vault must be instantiated with a positive balance
	t.Run("Cannot create token with negative initial balance", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateTokenScript(contractAddr, -7)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			true,
		)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateTokenScript(contractAddr, 10)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			false,
		)

		result, err := b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootKey().Address, 10))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})

	t.Run("Should be able to create multiple tokens and store them in an array", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			false,
		)
	})
}

func TestInAccountTransfers(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode)
	assert.NoError(t, err)

	// then deploy the three tokens to an account
	tx := flow.NewTransaction().
		SetScript(GenerateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5)).
		SetGasLimit(20).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(
		t, b, tx,
		[]flow.Address{b.RootKey().Address},
		[]crypto.Signer{b.RootKey().Signer()},
		false,
	)

	t.Run("Should be able to withdraw tokens from a vault", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateWithdrawScript(contractAddr, 0, 3)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			false,
		)

		// Assert that the vaults balance is correct
		result, err := b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootKey().Address, 0, 7))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})

	t.Run("Should be able to withdraw and deposit tokens from one vault to another in an account", func(t *testing.T) {
		tx = flow.NewTransaction().
			SetScript(GenerateWithdrawDepositScript(contractAddr, 1, 2, 8)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			false,
		)

		// Assert that the vault's balance is correct
		result, err := b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootKey().Address, 1, 12))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		// Assert that the vault's balance is correct
		result, err = b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootKey().Address, 2, 13))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})
}

func TestExternalTransfers(t *testing.T) {
	b := NewEmulator()

	accountKeys := test.AccountKeyGenerator()

	// First, deploy the token contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode)
	assert.NoError(t, err)

	// then deploy the tokens to an account
	tx := flow.NewTransaction().
		SetScript(GenerateCreateTokenScript(contractAddr, 10)).
		SetGasLimit(20).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(
		t, b, tx,
		[]flow.Address{b.RootKey().Address},
		[]crypto.Signer{b.RootKey().Signer()},
		false,
	)

	// create a new account
	bastianAccountKey, bastianSigner := accountKeys.NewWithSigner()

	bastianAddress, err := b.CreateAccount([]*flow.AccountKey{bastianAccountKey}, nil)
	require.NoError(t, err)

	// then deploy the tokens to the new account
	tx = flow.NewTransaction().
		SetScript(GenerateCreateTokenScript(contractAddr, 10)).
		SetGasLimit(20).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(bastianAddress)

	SignAndSubmit(
		t, b, tx,
		[]flow.Address{b.RootKey().Address, bastianAddress},
		[]crypto.Signer{b.RootKey().Signer(), bastianSigner},
		false,
	)

	t.Run("Should be able to withdraw and deposit tokens from a vault", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateDepositVaultScript(contractAddr, bastianAddress, 3)).
			SetGasLimit(20).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(
			t, b, tx,
			[]flow.Address{b.RootKey().Address},
			[]crypto.Signer{b.RootKey().Signer()},
			false,
		)

		// Assert that the vaults' balances are correct
		result, err := b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootKey().Address, 7))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}

		result, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, bastianAddress, 13))
		require.NoError(t, err)
		if !assert.True(t, result.Succeeded()) {
			t.Log(result.Error.Error())
		}
	})
}
