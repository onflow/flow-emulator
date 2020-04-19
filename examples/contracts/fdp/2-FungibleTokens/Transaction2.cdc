// Transaction2.cdc

import FungibleToken from 0x01

// This transaction configures an account to store and receive tokens defined by
// the FungibleToken contract.
transaction {
	prepare(acct: AuthAccount) {
		// Create a new empty Vault object
		let vaultA <- FungibleToken.createEmptyVault()
			
		// Store the vault in the account storage
		acct.save(<-vaultA, to: /storage/MainVault)

        log("Empty Vault stored")

        // Create a public Receiver capability to the Vault
        acct.link<&FungibleToken.Vault{FungibleToken.Receiver, FungibleToken.Balance}>(
            /public/MainReceiver, 
            target: /storage/MainVault
        )

        log("References created")
	}

    post {
        // Check that the capabilities were created correctly
        getAccount(0x01).getCapability(/public/MainReceiver)!
                        .check<&FungibleToken.Vault{FungibleToken.Receiver, FungibleToken.Balance}>():  
                        "Vault Receiver Reference was not created correctly"
    }
}
 
