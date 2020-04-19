// HelloWorldResource.cdc
//
// This is a variation of the HelloWorld contract that introduces the concept of
// resources, a new form of linear type that is unique to Cadence. Resources can be
// used to create a secure model of digital ownership.
//
// Learn more about resources in this tutorial: https://docs.onflow.org/docs/hello-world

pub contract HelloWorld {

    // Declare a resource that only includes one function.
	pub resource HelloAsset {
        // A transaction can call this function to get the "Hello, World!"
        // message from the resource.
		pub fun hello(): String {
			return "Hello, World!"
		}
	}

	init() {
        // Use the create built-in function to create a new instance
        // of the HelloAsset resource
        let newHello <- create HelloAsset()

        // We can do anything in the init function, including accessing
        // the storage of the account that this contract is deployed to.
        //
        // Here we are storing the newly created HelloAsset resource
        // in the private account storage 
        // by specifying a custom path to the resource
		self.account.save(<-newHello, to: /storage/Hello)

        log("HelloAsset created and stored")
	}
}
 
