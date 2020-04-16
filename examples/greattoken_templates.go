package examples

import (
	"fmt"

	"github.com/dapperlabs/flow-go-sdk"
)

// GenerateCreateMinterScript Creates a script that instantiates
// a new GreatNFTMinter instance and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func GenerateCreateMinterScript(nftAddr flow.Address, initialID, specialMod int) []byte {
	template := `
		import GreatToken from 0x%s

		transaction {

		  prepare(acct: AuthAccount) {
			let minter <- GreatToken.createGreatNFTMinter(firstID: %d, specialMod: %d)
			acct.save(<-minter, to: /storage/GreatNFTMinter)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// GenerateMintScript Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func GenerateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		import GreatToken from 0x%s

		transaction {

		  prepare(acct: AuthAccount) {
			let minter = acct.borrow<&GreatToken.GreatNFTMinter>(from: /storage/GreatNFTMinter)!

            if let nft <- acct.load<@GreatToken.GreatNFT>(from: /storage/GreatNFT) {
                destroy nft
            }

            acct.save(<-minter.mint(), to: /storage/GreatNFT)
			acct.link<&GreatToken.GreatNFT>(/public/GreatNFT, target: /storage/GreatNFT)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr.String()))
}

// GenerateInspectNFTScript Creates a script that retrieves an NFT
// from storage and makes assertions about its properties.
// If these assertions fail, the script panics.
func GenerateInspectNFTScript(nftCodeAddr, userAddr flow.Address, expectedID int, expectedIsSpecial bool) []byte {
	template := `
		import GreatToken from 0x%s

		pub fun main() {
		  let acct = getAccount(0x%s)
		  let nft = acct.getCapability(/public/GreatNFT)!.borrow<&GreatToken.GreatNFT>()!
		  assert(
              nft.id() == %d,
              message: "incorrect id"
          )
		  assert(
              nft.isSpecial() == %t,
              message: "incorrect specialness"
          )
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, expectedID, expectedIsSpecial))
}

// GenerateGetNFTIDScript creates a script that retrieves an NFT from storage and returns its ID.
func GenerateGetNFTIDScript(nftCodeAddr, userAddr flow.Address) []byte {
	template := `
		import GreatToken from 0x%s

		pub fun main(): Int {
		  let acct = getAccount(0x%s)
		  let nft = acct.published[&GreatToken.GreatNFT] ?? panic("missing nft")
		  return nft.id()
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr))
}
