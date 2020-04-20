package examples

import (
	"testing"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"
)

const (
	greatTokenContractFile = "./contracts/great-token.cdc"
)

func TestDeployment(t *testing.T) {
	b := NewEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	nftCode := ReadFile(greatTokenContractFile)
	_, err := b.CreateAccount(nil, nftCode)
	assert.NoError(t, err)
	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestCreateMinter(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	nftCode := ReadFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode)
	assert.NoError(t, err)

	// GreatNFTMinter must be instantiated with initialID > 0 and
	// specialMod > 1
	t.Run("Cannot create minter with negative initial ID", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateMinterScript(contractAddr, -1, 2)).
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

	t.Run("Cannot create minter with special mod < 2", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateMinterScript(contractAddr, 1, 1)).
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

	t.Run("Should be able to create minter", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript(GenerateCreateMinterScript(contractAddr, 1, 2)).
			SetGasLimit(10).
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

func TestMinting(t *testing.T) {
	b := NewEmulator()

	// First, deploy the contract
	nftCode := ReadFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode)
	assert.NoError(t, err)

	// Next, instantiate the minter
	createMinterTx := flow.NewTransaction().
		SetScript(GenerateCreateMinterScript(contractAddr, 1, 2)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(
		t, b, createMinterTx,
		[]flow.Address{b.RootKey().Address},
		[]crypto.Signer{b.RootKey().Signer()},
		false,
	)

	// Mint the first NFT
	mintTx1 := flow.NewTransaction().
		SetScript(GenerateMintScript(contractAddr)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(
		t, b, mintTx1,
		[]flow.Address{b.RootKey().Address},
		[]crypto.Signer{b.RootKey().Signer()},
		false,
	)

	// Assert that ID/specialness are correct
	result, err := b.ExecuteScript(GenerateInspectNFTScript(contractAddr, b.RootKey().Address, 1, false))
	require.NoError(t, err)
	if !assert.True(t, result.Succeeded()) {
		t.Log(result.Error.Error())
	}

	// Mint a second NF
	mintTx2 := flow.NewTransaction().
		SetScript(GenerateMintScript(contractAddr)).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(
		t, b, mintTx2,
		[]flow.Address{b.RootKey().Address},
		[]crypto.Signer{b.RootKey().Signer()},
		false,
	)

	// Assert that ID/specialness are correct
	result, err = b.ExecuteScript(GenerateInspectNFTScript(contractAddr, b.RootKey().Address, 2, true))
	require.NoError(t, err)
	if !assert.True(t, result.Succeeded()) {
		t.Log(result.Error.Error())
	}
}
