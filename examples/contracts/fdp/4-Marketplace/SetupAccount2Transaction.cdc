// SetupAccount2Transaction.cdc

import FungibleToken from 0x01
import NonFungibleToken from 0x02

// This transaction adds an empty Vault to account 0x02
// and mints an NFT with id=1 that is deposited into
// the NFT collection on account 0x01.
transaction {

    // Private reference to this account's minter resource
    let minterRef: &NonFungibleToken.NFTMinter

    prepare(acct: AuthAccount) {
        // create a new vault instance with an initial balance of 30
        let vaultA <- FungibleToken.createEmptyVault()

		// Store the vault in the account storage
		acct.save(<-vaultA, to: /storage/MainVault)

        // Create a public Receiver capability to the Vault
		let ReceiverRef = acct.link<&FungibleToken.Vault{FungibleToken.Receiver, FungibleToken.Balance}>
                                    (/public/MainReceiver, target: /storage/MainVault)

        log("Created a Vault and published a reference")

        // Borrow a reference for the NFTMinter in storage
        self.minterRef = acct.borrow<&NonFungibleToken.NFTMinter>(from: /storage/NFTMinter)!
    }
    execute {
        // Get the recipient's public account object
        let recipient = getAccount(0x01)

        // Get the Collection reference for the receiver
        // getting the public capability and borrowing a reference from it
        let receiverRef = recipient.getCapability(/public/NFTReceiver)!
                                   .borrow<&{NonFungibleToken.NFTReceiver}>()!

        // Mint an NFT and deposit it into account 0x01's collection
        self.minterRef.mintNFT(recipient: receiverRef)

        log("New NFT minted for account 1")
    }
}
 
