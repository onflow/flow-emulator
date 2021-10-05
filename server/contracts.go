package server

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-nft/lib/go/contracts"
)

var baseContractsPath = "./server/contracts/"

func deployContracts(conf *Config, b *emulator.Blockchain) map[string]flow.Address {
	nftAddress, _ := deployContract("NonFungibleToken", contracts.NonFungibleToken(), b)
	exampleNFT, _ := deployContract("ExampleNFT", contracts.ExampleNFT(nftAddress.Hex()), b)

	nftStorefrontContract := loadContract("NFTStorefront.cdc", map[string]flow.Address{
		"FungibleToken":    flow.HexToAddress(fvm.FlowTokenAddress(b.GetChain()).Hex()),
		"NonFungibleToken": nftAddress,
	})

	nftStorefront, _ := deployContract("NFTStorefront", nftStorefrontContract, b)

	addresses := map[string]flow.Address{
		"NonFungibleToken": nftAddress,
		"ExampleNFT":       exampleNFT,
		"NFTStorefront":    nftStorefront,
	}

	return addresses
}

func loadContract(name string, replacements map[string]flow.Address) []byte {
	code := string(readFile(filepath.Join(baseContractsPath, name)))
	for name, realAddress := range replacements {
		placeholder := regexp.MustCompile(fmt.Sprintf(`"[^"\s].*/%s.cdc"`, name))
		code = placeholder.ReplaceAllString(code, fmt.Sprintf("0x%s", realAddress.String()))
	}
	return []byte(code)
}

func deployContract(name string, contract []byte, b *emulator.Blockchain) (flow.Address, error) {
	return b.CreateAccount(
		nil,
		[]templates.Contract{
			{
				Name:   name,
				Source: string(contract),
			},
		},
	)
}

func readFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}
