package examples

import (
	"fmt"

	"github.com/onflow/flow-go-sdk"
)

// GenerateCreateTokenScript creates a script that instantiates
// a new Vault instance and stores it in memory.
// balance is an argument to the Vault constructor.
// The Vault must have been deployed already.
func GenerateCreateTokenScript(tokenAddr flow.Address, initialBalance int) []byte {
	template := `
      import FungibleToken, FlowToken from 0x%s

      transaction {

          prepare(acct: AuthAccount) {
              let vault <- FlowToken.createVault(initialBalance: %d)
              acct.save(<-vault, to: /storage/vault)

              acct.link<&{FungibleToken.Receiver}>(/public/receiver, target: /storage/vault)
              acct.link<&{FungibleToken.Balance}>(/public/balance, target: /storage/vault)
          }
      }
    `
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance))
}

// GenerateCreateThreeTokensArrayScript creates a script
// that creates three new vault instances, stores them
// in an array of vaults, and then stores the array
// to the storage of the signer's account
func GenerateCreateThreeTokensArrayScript(tokenAddr flow.Address, initialBalance int, bal2 int, bal3 int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		transaction {

		  prepare(acct: AuthAccount) {
			let vaultA <- FlowToken.createVault(initialBalance: %d)
    		let vaultB <- FlowToken.createVault(initialBalance: %d)
			let vaultC <- FlowToken.createVault(initialBalance: %d)
			
			var vaults <- [<-vaultA, <-vaultB]

			vaults.append(<-vaultC)

            acct.save(<-vaults, to: /storage/vaults) 

            acct.link<&[FlowToken.Vault]>(/public/vaults, target: /storage/vaults)
		  }
		}
	`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance, bal2, bal3))
}

// GenerateWithdrawScript creates a script that withdraws
// tokens from a vault and destroys the tokens
func GenerateWithdrawScript(tokenCodeAddr flow.Address, vaultNumber int, withdrawAmount int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		transaction {
		  prepare(acct: AuthAccount) {
			let vaults <- acct.load<@[FlowToken.Vault]>(from: /storage/vaults)!
			
			let withdrawVault <- vaults[%d].withdraw(amount: %d)

			acct.save(<-vaults, to: /storage/vaults) 

			destroy withdrawVault
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, vaultNumber, withdrawAmount))
}

// GenerateWithdrawDepositScript creates a script
// that withdraws tokens from a vault and deposits
// them to another vault
func GenerateWithdrawDepositScript(tokenCodeAddr flow.Address, withdrawVaultNumber int, depositVaultNumber int, withdrawAmount int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		transaction {
		  prepare(acct: AuthAccount) {
			let vaults <- acct.load<@[FlowToken.Vault]>(from: /storage/vaults)!
			
			let withdrawVault <- vaults[%d].withdraw(amount: %d)

			vaults[%d].deposit(from: <-withdrawVault)

			acct.save(<-vaults, to: /storage/vaults)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, withdrawVaultNumber, withdrawAmount, depositVaultNumber))
}

// GenerateDepositVaultScript creates a script that withdraws an tokens from an account
// and deposits it to another account's vault
func GenerateDepositVaultScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		transaction {
		  prepare(acct: AuthAccount) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.borrow<&{FungibleToken.Provider}>(from: /storage/vault)!
			let receiverRef = recipient.getCapability(/public/receiver)!.borrow<&{FungibleToken.Receiver}>()! 

			let tokens <- providerRef.withdraw(amount: %d)

			receiverRef.deposit(from: <-tokens)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInspectVaultScript creates a script that retrieves a
// Vault from the array in storage and makes assertions about
// its balance. If these assertions fail, the script panics.
func GenerateInspectVaultScript(tokenCodeAddr, userAddr flow.Address, expectedBalance int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultRef = acct.getCapability(/public/balance)!.borrow<&{FungibleToken.Balance}>()!
			assert(
                vaultRef.balance == UInt64(%d),
                message: "incorrect balance!"
            )
		}
    `

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, expectedBalance))
}

// GenerateInspectVaultArrayScript creates a script that retrieves a
// Vault from the array in storage and makes assertions about
// its balance. If these assertions fail, the script panics.
func GenerateInspectVaultArrayScript(tokenCodeAddr, userAddr flow.Address, vaultNumber int, expectedBalance int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaults = acct.getCapability(/public/vaults)!.borrow<&[FlowToken.Vault]>()!
			assert(
                vaults[%d].balance == UInt64(%d),
                message: "incorrect Balance!"
            )
        }
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, vaultNumber, expectedBalance))
}
