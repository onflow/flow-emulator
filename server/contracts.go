package server

import (
	"fmt"
	"io/ioutil"
	"regexp"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-nft/lib/go/contracts"
)

var ftAddress = flow.HexToAddress("ee82856bf20e2aa6")
var baseContractsPath = "./server/contracts/"

// deployContracts is meant to be a place where you can load up existing contracts
// NFTStorefront is an example of how you can do this
func deployContracts(conf *Config, b *emulator.Blockchain) map[string]flow.Address {
	addresses := make(map[string]flow.Address)

	nftAddress, _ := deployContract("NonFungibleToken", contracts.NonFungibleToken(), b, addresses)
	deployContract("ExampleNFT", contracts.ExampleNFT(nftAddress.Hex()), b, addresses)

	nftStorefront := loadContract("NFTStorefront.cdc", map[string]flow.Address{
		"FungibleToken":    ftAddress,
		"NonFungibleToken": nftAddress,
	})
	deployContract("NFTStorefront", nftStorefront, b, addresses)

	return addresses
}

func loadContract(name string, replacements map[string]flow.Address) []byte {
	code := string(readFile(baseContractsPath + name))
	for name, realAddress := range replacements {
		placeholder := regexp.MustCompile(fmt.Sprintf(`"[^"\s].*/%s.cdc"`, name))
		code = placeholder.ReplaceAllString(code, "0x"+realAddress.String())
	}
	return []byte(code)
}

func deployContract(name string, contract []byte, b *emulator.Blockchain, addresses map[string]flow.Address) (flow.Address, error) {
	address, err := b.CreateAccount(
		nil,
		[]templates.Contract{
			{
				Name:   name,
				Source: string(contract),
			},
		},
	)
	addresses[name] = address
	return address, err
}

func readFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}
