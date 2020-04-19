// NFTv1.cdc
//
// The NonFungibleToken contract is a sample implementation of a non-fungible token (NFT) on Flow.
//
// This contract defines one of the simplest forms of NFTs using an
// integer ID and metadata field.
// 
// Learn more about non-fungible tokens in this tutorial: https://docs.onflow.org/docs/non-fungible-tokens

pub contract NonFungibleToken {

    // Declare the NFT resource type
    pub resource NFT {
        // The unique ID that differentiates each NFT
        pub let id: UInt64

        // String mapping to hold metadata
        pub var metadata: {String: String}

        // Initialize both fields in the init function
        init(initID: UInt64) {
            self.id = initID
            self.metadata = {}
        }
    }

    // Create a single new NFT and save it to account storage
	init() {
		self.account.save<@NFT>(<-create NFT(initID: 1), to: /storage/NFT1)
	}
}
