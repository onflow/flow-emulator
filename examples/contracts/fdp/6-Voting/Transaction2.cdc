import ApprovalVoting from 0x01

// Transaction2.cdc
//
// This transaction allows the administrator of the Voting contract
// to create a new ballot and store it in a voter's account
// The voter and the administrator have to both sign the transaction
// so it can access their storage

transaction {
    prepare(admin: AuthAccount, voter: AuthAccount) {

        // borrow a reference to the admin Resource
        let adminRef = admin.borrow<&ApprovalVoting.Administrator>(from: /storage/VotingAdmin)!
        
        // create a new Ballot by calling the issueBallot
        // function of the admin Reference
        let ballot <- adminRef.issueBallot()

        // store that ballot in the voter's account storage
        voter.save(<-ballot, to: /storage/Ballot)

        log("Ballot transferred to voter")
    }
}
