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
	fusd "github.com/onflow/fusd/lib/go/contracts"
)

var baseContractsPath = "./server/contracts/"

type DeployDescription struct {
	name        string
	address     flow.Address
	description string
}

func deployContracts(conf *Config, b *emulator.Blockchain) []DeployDescription {
	ftAddress := flow.HexToAddress(fvm.FungibleTokenAddress(b.GetChain()).Hex())
	serviceAcct := b.ServiceKey().Address

	contracts := map[string][]byte{
		"FUSD":             fusd.FUSD(ftAddress.String()),
		"NonFungibleToken": contracts.NonFungibleToken(),
		"ExampleNFT":       contracts.ExampleNFT(serviceAcct.Hex()),
		"NFTStoreFront": loadContract("NFTStorefront.cdc", map[string]flow.Address{
			"FungibleToken":    serviceAcct,
			"NonFungibleToken": serviceAcct,
		}),
	}
	for name, contract := range contracts {
		templates.AddAccountContract(serviceAcct, templates.Contract{
			Name:   name,
			Source: string(contract),
		})
	}

	addresses := []DeployDescription{
		{"FUSD", serviceAcct, "💵  FUSD contract"},
		{"NonFungibleToken", serviceAcct, "✨   NFT contract"},
		{"ExampleNFT", serviceAcct, "✨   NFT contract"},
		{"NFTStorefront", serviceAcct, "✨   NFT contract"},
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

func readFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}
