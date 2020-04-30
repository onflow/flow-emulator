package examples

import (
	"fmt"

	"github.com/onflow/flow-go-sdk"
)

// GenerateCreateCollectionScript Creates a script that instantiates a new
// NFT collection instance, stores the collection in memory, then stores a
// reference to the collection.
func GenerateCreateCollectionScript(tokenAddr flow.Address) []byte {
	template := `
		import NonFungibleToken, ExampleNFT from 0x%s

		transaction {
		  prepare(acct: AuthAccount) {

			let collection <- ExampleNFT.createEmptyCollection()
			
			acct.save(<-collection, to: /storage/NFTCollection)

			acct.link<&{NonFungibleToken.CollectionPublic}>(/public/NFTCollection, target: /storage/NFTCollection)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenAddr))
}

// GenerateMintNFTScript Creates a script that uses the admin resource
// to mint a new NFT and deposit it into a user's collection
func GenerateMintNFTScript(tokenAddr, receiverAddr flow.Address) []byte {
	template := `
		import NonFungibleToken, ExampleNFT from 0x%s

		transaction {
			let minter: &ExampleNFT.NFTMinter
		
			prepare(signer: AuthAccount) {
		
				self.minter = signer.borrow<&ExampleNFT.NFTMinter>(from: /storage/NFTMinter)!
			}
		
			execute {
				let recipient = getAccount(0x%s)
		
				let receiver = recipient
					.getCapability(/public/NFTCollection)!
					.borrow<&{NonFungibleToken.CollectionPublic}>()
					?? panic("Could not get receiver reference to the NFT Collection")
		
				self.minter.mintNFT(recipient: receiver)
			}
		}
	`

	return []byte(fmt.Sprintf(template, tokenAddr, receiverAddr))
}

// GenerateInspectCollectionScript creates a script that retrieves an NFT collection
// from storage and tries to borrow a reference for an NFT that it owns.
// If it owns it, it will not fail.
func GenerateInspectCollectionScript(nftCodeAddr, userAddr flow.Address, nftID int) []byte {
	template := `
		import NonFungibleToken, ExampleNFT from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let collectionRef = acct.getCapability(/public/NFTCollection)!.borrow<&{NonFungibleToken.CollectionPublic}>()
				?? panic("Could not borrow capability from public collection")
			
			let tokenRef = collectionRef.borrowNFT(id: UInt64(%d))
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, nftID))
}

// GenerateInspectCollectionLenScript creates a script that retrieves an NFT collection
// from storage and tries to borrow a reference for an NFT that it owns.
// If it owns it, it will not fail.
func GenerateInspectCollectionLenScript(nftCodeAddr, userAddr flow.Address, length int) []byte {
	template := `
		import NonFungibleToken, ExampleNFT from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let collectionRef = acct.getCapability(/public/NFTCollection)!.borrow<&{NonFungibleToken.CollectionPublic}>()
				?? panic("Could not borrow capability from public collection")
			
			if %d != collectionRef.getIDs().length {
				panic("Collection Length is not correct")
			}
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, length))
}
