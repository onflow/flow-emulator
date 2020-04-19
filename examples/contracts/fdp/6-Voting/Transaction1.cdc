import ApprovalVoting from 0x01

// Transaction1.cdc
//
// This transaction allows the administrator of the Voting contract
// to create new proposals for voting and save them to the smart contract

transaction {
    prepare(admin: AuthAccount) {
        
        // borrow a reference to the admin Resource
        let adminRef = admin.borrow<&ApprovalVoting.Administrator>(from: /storage/VotingAdmin)!
        
        // Call the initializeProposals function
        // to create the proposals array as an array of strings
        adminRef.initializeProposals(
            ["Longer Shot Clock", "Trampolines instead of hardwood floors"]
        )

        log("Proposals Initialized!")
    }

    post {
        ApprovalVoting.proposals.length == 2
    }

}