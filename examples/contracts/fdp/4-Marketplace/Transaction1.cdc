// Transaction1.cdc

import FungibleToken from 0x01
import NonFungibleToken from 0x02
import Marketplace from 0x03

// This transaction creates a new Sale Collection object,
// lists an NFT for sale, puts it in account storage,
// and creates a public capability to the sale so that others can buy the token.
transaction {

    prepare(acct: AuthAccount) {

        // Borrow a reference to the stored Vault
        let receiver = acct.borrow<&{FungibleToken.Receiver}>(from: /storage/MainVault)!

        // Create a new Sale object, 
        // initializing it with the reference to the owner's vault
        let sale <- Marketplace.createSaleCollection(ownerVault: receiver)

        // borrow a reference to the NFTCollection in storage
        let collectionRef = acct.borrow<&NonFungibleToken.Collection>(from: /storage/NFTCollection)!
    
        // Withdraw the NFT from the collection that you want to sell
        // and move it into the transaction's context
        let token <- collectionRef.withdraw(withdrawID: 1)

        // List the token for sale by moving it into the sale object
        sale.listForSale(token: <-token, price: UFix64(10))

        // Store the sale object in the account storage 
        acct.save(<-sale, to: /storage/NFTSale)

        // Create a public capability to the sale so that others can call its methods
        acct.link<&Marketplace.SaleCollection{Marketplace.SalePublic}>(/public/NFTSale, target: /storage/NFTSale)

        log("Sale Created for account 1. Selling NFT 1 for 10 tokens")
    }
}
 
