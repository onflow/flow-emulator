// Transaction3.cdc

import HelloWorld from 0x02

// This transaction creates a new capability 
// for the HelloAsset resource in storage
// and adds it to the account's public area.
//
// Other accounts and scripts can use this capability
// to create a reference to the private object to be able to
// access its fields and call its methods.

transaction {
	prepare(account: AuthAccount) {

        // Create a public capability by linking the capability to 
        // a `target` object in account storage
        // This does not check if the link is valid or if the target exists
        // it just creates the capability.
        // The capability is created and stored at /public/Hello, and is 
        // also returned from the function.
        let capability = account.link<&HelloWorld.HelloAsset>(/public/Hello, target: /storage/Hello)

        // Use the capability's borrow method to create a new reference 
        // to the object that the capability links to
        let helloReference = capability!.borrow<&HelloWorld.HelloAsset>()

        // Call the hello function using the reference to the HelloAsset resource.
        //
        // We use the "?" symbol because the value we are accessing is an optional.
        log(helloReference?.hello())
	}
}
