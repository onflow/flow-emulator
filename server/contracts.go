package server

import (
	"embed"
	"fmt"
	"path/filepath"
	"regexp"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-nft/lib/go/contracts"
	fusd "github.com/onflow/fusd/lib/go/contracts"
)

var (
	//go:embed contracts
	emContracts embed.FS
)

type DeployDescription struct {
	name        string
	address     flow.Address
	description string
}

func deployContracts(b *emulator.Blockchain) ([]DeployDescription, error) {
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
		err := deployContract(b, c.name, c.source)
		if err != nil {
			return []DeployDescription{}, err
		}
	}

	serviceAcct, err := b.GetAccount(serviceAddress)
	if err != nil {
		return []DeployDescription{}, err
	}

	addresses := make([]DeployDescription, 0)
	for _, c := range toDeploy {
		if _, err := serviceAcct.Contracts[c.name]; err {
			addresses = append(addresses, DeployDescription{c.name, serviceAddress, c.description})
		}
	}

	return addresses, nil
}

func deployContract(b *emulator.Blockchain, name string, contract []byte) error {

	serviceKey := b.ServiceKey()
	serviceAddress := serviceKey.Address

	if serviceKey.PrivateKey == nil {
		return fmt.Errorf("not able to deploy contracts without set private key")
	}

	latestBlock, err := b.GetLatestBlock()
	if err != nil {
		return err
	}

	tx := templates.AddAccountContract(serviceAddress, templates.Contract{
		Name:   name,
		Source: string(contract),
	})

	tx.SetGasLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(flow.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, serviceKey.Index, serviceKey.SequenceNumber).
		SetPayer(serviceAddress)

	err = tx.SignEnvelope(serviceAddress, serviceKey.Index, serviceKey.Signer())
	if err != nil {
		return err
	}

	err = b.AddTransaction(*tx)
	if err != nil {
		return err
	}

	_, results, err := b.ExecuteAndCommitBlock()
	if err != nil {
		return err
	}

	lastResult := results[len(results)-1]
	if !lastResult.Succeeded() {
		return lastResult.Error
	}

	return nil
}

func loadContract(name string, replacements map[string]flow.Address) []byte {
	contractFile, _ := emContracts.ReadFile(filepath.Join("contracts", name))
	code := string(contractFile)

	for name, realAddress := range replacements {
		placeholder := regexp.MustCompile(fmt.Sprintf(`"[^"\s].*/%s.cdc"`, name))
		code = placeholder.ReplaceAllString(code, fmt.Sprintf("0x%s", realAddress.String()))
	}
	return []byte(code)
}
