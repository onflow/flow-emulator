package examples

import (
	"fmt"

	"github.com/onflow/flow-go-sdk"
)

// GenerateCreateTokenScript creates a script that instantiates
// a new Vault instance and stores it in storage.
// balance is an argument to the Vault constructor.
// The Vault must have been deployed already.
func GenerateCreateTokenScript(tokenAddr flow.Address) []byte {
	template := `
      import FungibleToken, FlowToken from 0x%s

      transaction {

          prepare(acct: AuthAccount) {
              let vault <- FlowToken.createEmptyVault()
              acct.save(<-vault, to: /storage/flowTokenVault)

              acct.link<&{FungibleToken.Receiver}>(/public/flowTokenReceiver, target: /storage/flowTokenVault)
              acct.link<&{FungibleToken.Balance}>(/public/flowTokenBalance, target: /storage/flowTokenVault)
          }
      }
    `
	return []byte(fmt.Sprintf(template, tokenAddr))
}

// GenerateMintTokensScript creates a script that uses the admin resource
// to mint new tokens and deposit them in a Vault
func GenerateMintTokensScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s
	
		transaction {
	
			// Vault resource that holds the tokens that are being minted
			var vault: @FlowToken.Vault
		
			prepare(signer: AuthAccount) {
		
				// Get a reference to the signer's MintAndBurn resource in storage
				let mintAndBurn = signer.borrow<&FlowToken.MintAndBurn>(from: /storage/flowTokenMintAndBurn)
					?? panic("Couldn't borrow MintAndBurn reference from storage")
		
				// Mint 10 new tokens
				self.vault <- mintAndBurn.mintTokens(amount: %d.0)
			}
		
			execute {
				// Get the recipient's public account object
				let recipient = getAccount(0x%s)
		
				// Get a reference to the recipient's Receiver
				let receiver = recipient.getCapability(/public/flowTokenReceiver)!
					.borrow<&{FungibleToken.Receiver}>()
					?? panic("Couldn't borrow receiver reference to recipient's vault")
		
				// Deposit the newly minted token in the recipient's Receiver
				receiver.deposit(from: <-self.vault)
			}
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, amount, receiverAddr))
}

// GenerateInspectVaultScript creates a script that retrieves a
// Vault from the array in storage and makes assertions about
// its balance. If these assertions fail, the script panics.
func GenerateInspectVaultScript(tokenCodeAddr, userAddr flow.Address, expectedBalance float64) []byte {
	template := `
		import FungibleToken, FlowToken from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultRef = acct.getCapability(/public/flowTokenBalance)!.borrow<&{FungibleToken.Balance}>()
				?? panic("Could not borrow Balance reference to the Vault")
			assert(
                vaultRef.balance == UFix64(%f),
                message: "incorrect balance!"
            )
		}
    `

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, expectedBalance))
}
