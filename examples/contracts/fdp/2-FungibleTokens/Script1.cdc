// Script1.cdc

import FungibleToken from 0x01 

// This script reads the Vault balances of two accounts.
pub fun main() {
    // Get the accounts' public account objects
    let acct1 = getAccount(0x01)
    let acct2 = getAccount(0x02)

    // Get references to the account's receivers
    // by getting their public capability
    // and borrowing a reference from the capability
    let acct1ReceiverRef = acct1.getCapability(/public/MainReceiver)!
                            .borrow<&FungibleToken.Vault{FungibleToken.Balance}>()!
    let acct2ReceiverRef = acct2.getCapability(/public/MainReceiver)!
                            .borrow<&FungibleToken.Vault{FungibleToken.Balance}>()!

    // Use optional chaining to read and log balance fields
    log("Account 1 Balance")
	log(acct1ReceiverRef.balance)
    log("Account 2 Balance")
    log(acct2ReceiverRef.balance)
}
