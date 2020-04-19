// SetupAccount1Transaction.cdc

import FungibleToken from 0x01
import NonFungibleToken from 0x02

// This transaction sets up account 0x01 for the marketplace tutorial
// by publishing a Vault reference and creating an empty NFT Collection.
transaction {
    prepare(acct: AuthAccount) {
        // Create a public Receiver capability to the Vault
        acct.link<&FungibleToken.Vault{FungibleToken.Receiver, FungibleToken.Balance}>
                 (/public/MainReceiver, target: /storage/MainVault)

        log("Created Vault references")

        // store an empty NFT Collection in account storage
        acct.save(<-NonFungibleToken.createEmptyCollection(), to: /storage/NFTCollection)

        // publish a capability to the Collection in storage
        acct.link<&{NonFungibleToken.NFTReceiver}>(/public/NFTReceiver, target: /storage/NFTCollection)

        log("Created a new empty collection and published a reference")
    }
}
