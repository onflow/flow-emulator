import ApprovalVoting from 0x01

// Script1.cdc
//
// This script allows anyone to read the tallied votes for each proposal
//

pub fun main() {

    // Access the public fields of the contract to log 
    // the proposal names and vote counts

    log("Number of Votes for Proposal 1:")
    log(ApprovalVoting.proposals[0])
    log(ApprovalVoting.votes[0])

    log("Number of Votes for Proposal 2:")
    log(ApprovalVoting.proposals[1])
    log(ApprovalVoting.votes[1])

}