package server

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-go-sdk"
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

func deployContracts(conf *Config, b *emulator.Blockchain) ([]DeployDescription, error) {
	ftAddress := flow.HexToAddress(fvm.FungibleTokenAddress(b.GetChain()).Hex())
	serviceAddress := b.ServiceKey().Address

	nftContract := loadContract("NFTStorefront.cdc", map[string]flow.Address{
		"FungibleToken":    ftAddress,
		"NonFungibleToken": serviceAddress,
	})

	toDeploy := []struct {
		name        string
		description string
		source      []byte
	}{
		{"FUSD", "💵  FUSD contract", fusd.FUSD(ftAddress.String())},
		{"NonFungibleToken", "✨   NFT contract", contracts.NonFungibleToken()},
		{"ExampleNFT", "✨   NFT contract", contracts.ExampleNFT(serviceAddress.Hex())},
		{"NFTStorefront", "✨   NFT contract", nftContract},
	}

	for _, c := range toDeploy {
		err := b.DeployContract(c.name, c.source)
		if err != nil {
			return []DeployDescription{}, err
		}
	}

	serviceAcct, err := b.GetAccount(serviceAddress)
	if err != nil {
		return []DeployDescription{}, err
	}

	addresses := []DeployDescription{}
	for _, c := range toDeploy {
		if _, err := serviceAcct.Contracts[c.name]; err {
			addresses = append(addresses, DeployDescription{c.name, serviceAddress, c.description})
		}
	}

	return addresses, nil
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
