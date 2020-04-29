// Transaction2.cdc

import NonFungibleToken from 0x02

// This transaction allows the Minter account to mint an NFT
// and deposit it into its collection.

transaction {

    // The reference to the collection that will be receiving the NFT
    let receiverRef: &{NonFungibleToken.NFTReceiver}

    // The reference to the Minter resource stored in account storage
    let minterRef: &NonFungibleToken.NFTMinter

    prepare(acct: AuthAccount) {
        // Get the owner's collection capability and borrow a reference
        self.receiverRef = acct.getCapability(/public/NFTReceiver)!
                               .borrow<&{NonFungibleToken.NFTReceiver}>()!
        
        // Borrow a capability for the NFTMinter in storage
        self.minterRef = acct.borrow<&NonFungibleToken.NFTMinter>(from: /storage/NFTMinter)!
    }

    execute {
        // Use the minter reference to mint an NFT, which deposits
        // the NFT into the collection that is sent as a parameter.
        self.minterRef.mintNFT(recipient: self.receiverRef)

        log("NFT Minted and deposited to Account 2's Collection")
    }
}
