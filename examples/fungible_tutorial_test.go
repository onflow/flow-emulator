package examples

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/stretchr/testify/assert"
)

const (
	fungibleTokenTutorialContractFile = "./contracts/fungible-token-tutorial.cdc"
)

func TestFungibleTokenTutorialContractDeployment(t *testing.T) {
	b := NewEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(fungibleTokenTutorialContractFile)
	_, err := b.CreateAccount(nil, tokenCode)
	assert.NoError(t, err)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestFungibleTokenTutorialContractCreation(t *testing.T) {
	b := NewEmulator()

	// First, *update* the contract
	tokenCode := ReadFile(fungibleTokenTutorialContractFile)

	updateTokenScript := templates.UpdateAccountCode(tokenCode)

	tx := flow.NewTransaction().
		SetScript(updateTokenScript).
		SetGasLimit(10).
		SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
		SetPayer(b.RootKey().Address).
		AddAuthorizer(b.RootKey().Address)

	SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey().PrivateKey}, []flow.Address{b.RootAccountAddress()}, false)

	t.Run("Set up account 1", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript([]byte(
				fmt.Sprintf(
					`
                      import FungibleToken from 0x%s

                      transaction {
                          prepare(acct: AuthAccount) {
                              // create a public capability to access the vault as a receiver
                              acct.link<&{FungibleToken.Receiver}>(/public/receiver, target: /storage/vault)
                          }
                      }
	               `,
					b.RootAccountAddress().Short(),
				),
			)).
			SetGasLimit(10).
			SetProposalKey(b.RootKey().Address, b.RootKey().ID, b.RootKey().SequenceNumber).
			SetPayer(b.RootKey().Address).
			AddAuthorizer(b.RootKey().Address)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey().PrivateKey}, []flow.Address{b.RootAccountAddress()}, false)
	})

	var account2Address flow.Address

	t.Run("Create account 2", func(t *testing.T) {
		var err error
		publicKey := b.RootKey().AccountKey()
		publicKeys := []flow.AccountKey{publicKey}
		account2Address, err = b.CreateAccount(publicKeys, nil)
		assert.NoError(t, err)
	})

	t.Run("Set up account 2", func(t *testing.T) {
		tx := flow.NewTransaction().
			SetScript([]byte(
				fmt.Sprintf(
					`
                      // NOTE: using different import address to ensure user can use different formats
                      import FungibleToken from 0x00%s

                      transaction {

                          prepare(acct: AuthAccount) {
                              // create a new vault instance
                              let vault <- FungibleToken.createEmptyVault()

                              // store it in the account storage
                              acct.save(<-vault, to: /storage/vault)

                              // create a public capability to access the vault as a receiver
                              acct.link<&{FungibleToken.Receiver}>(/public/receiver, target: /storage/vault)
                          }
                      }
                    `,
					b.RootAccountAddress().Short(),
				),
			)).
			SetGasLimit(10).
			SetProposalKey(account2Address, 0, 0).
			SetPayer(account2Address).
			AddAuthorizer(account2Address)

		SignAndSubmit(t, b, tx, []flow.AccountPrivateKey{b.RootKey().PrivateKey}, []flow.Address{account2Address}, false)
	})
}
