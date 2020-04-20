package examples

import (
	"fmt"

	"github.com/onflow/flow-go-sdk"
)

// GenerateCreateSaleScript creates a cadence transaction that creates a Sale collection
// and stores in in the callers account published
func GenerateCreateSaleScript(tokenAddr flow.Address, marketAddr flow.Address) []byte {
	// TODO: store capability instead of reference in market sale collection!

	template := `
		import FungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: AuthAccount) {
				let ownerVault = acct.getCapability(/public/receiver)!.borrow<&{FungibleToken.Receiver}>() !

				let collection <- Market.createSaleCollection(ownerVault: ownerVault)
				
				acct.save(<-collection, to: /storage/saleCollection)
				
				acct.link<&Market.SaleCollection>(/public/saleCollection, target: /storage/saleCollection)
			}
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, marketAddr))
}

// GenerateStartSaleScript creates a cadence transaction that starts a sale by depositing
// an NFT into the Sale Collection with an associated price
func GenerateStartSaleScript(nftAddr flow.Address, marketAddr flow.Address, id, price int) []byte {
	template := `
		import NonFungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: AuthAccount) {
				let nftCollection = acct.borrow<&NonFungibleToken.Collection>(from: /storage/collection)!
                let token <- nftCollection.withdraw(withdrawID: %d)

				let saleCollection = acct.getCapability(/public/saleCollection)!.borrow<&Market.SaleCollection>()!
				saleCollection.listForSale(token: <-token, price: %d)
			}
		}`
	return []byte(fmt.Sprintf(template, nftAddr, marketAddr, id, price))
}

// GenerateBuySaleScript creates a cadence transaction that makes a purchase of
// an existing sale
func GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, userAddr flow.Address, id, amount int) []byte {
	template := `
		import FungibleToken from 0x%s
		import NonFungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: AuthAccount) {
				let seller = getAccount(0x%s)

				let collection = acct.getCapability(/public/collection)!.borrow<&NonFungibleToken.Collection>()!
				let provider = acct.borrow<&{FungibleToken.Provider}>(from: /storage/vault)!
				
				let tokens <- provider.withdraw(amount: %d)

				let saleCollection = seller.getCapability(/public/saleCollection)!.borrow<&Market.SaleCollection>()!
			
				saleCollection.purchase(tokenID: %d, recipient: collection, buyTokens: <-tokens)

			}
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, nftAddr, marketAddr, userAddr, amount, id))
}
