// Transaction1.cdc

import NonFungibleToken from 0x01

// This transaction checks if an NFT exists in the storage of the given account
// by trying to borrow from it
transaction {
    prepare(acct: AuthAccount) {
        if acct.borrow<&NonFungibleToken.NFT>(from: /storage/NFT1) != nil {
            log("The token exists!")
        } else {
            log("No token found!")
        }
    }
}
 