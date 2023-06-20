package emulator

import (
	"fmt"

	"github.com/onflow/flow-emulator/convert"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-nft/lib/go/contracts"
	fusd "github.com/onflow/fusd/lib/go/contracts"
	nftstorefront "github.com/onflow/nft-storefront/lib/go/contracts"
)

var CommonContracts = func() []ContractDescription {
	chain := flowgo.Emulator.Chain()
	ftAddress := flowsdk.HexToAddress(fvm.FungibleTokenAddress(chain).HexWithPrefix())
	serviceAddress := flowsdk.HexToAddress(chain.ServiceAddress().HexWithPrefix())

	return []ContractDescription{
		{
			Name:        "FUSD",
			Address:     serviceAddress,
			Description: "💵  FUSD contract",
			Source:      fusd.FUSD(ftAddress.String()),
		},
		{
			Name:        "NonFungibleToken",
			Address:     serviceAddress,
			Description: "✨  NFT contract",
			Source:      contracts.NonFungibleToken(),
		},
		{
			Name:        "ViewResolver",
			Address:     serviceAddress,
			Description: "✨  Metadata views contract",
			Source:      contracts.Resolver(),
		},
		{
			Name:        "MetadataViews",
			Address:     serviceAddress,
			Description: "✨  Metadata views contract",
			Source:      contracts.MetadataViews(ftAddress, serviceAddress),
		},
		{
			Name:        "ExampleNFT",
			Address:     serviceAddress,
			Description: "✨  Example NFT contract",
			Source:      contracts.ExampleNFT(serviceAddress, serviceAddress, serviceAddress),
		},
		{
			Name:        "NFTStorefrontV2",
			Address:     serviceAddress,
			Description: "✨  NFT Storefront contract v2",
			Source:      nftstorefront.NFTStorefront(2, ftAddress.String(), serviceAddress.String()),
		},
		{
			Name:        "NFTStorefront",
			Address:     serviceAddress,
			Description: "✨  NFT Storefront contract",
			Source:      nftstorefront.NFTStorefront(1, ftAddress.String(), serviceAddress.String()),
		},
	}
}()

type ContractDescription struct {
	Name        string
	Address     flowsdk.Address
	Description string
	Source      []byte
}

func DeployContracts(b *Blockchain, deployments []ContractDescription) error {
	for _, c := range deployments {
		err := deployContract(b, c.Name, c.Source)
		if err != nil {
			return err
		}
	}

	return nil
}

func deployContract(b *Blockchain, name string, contract []byte) error {
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
		SetReferenceBlockID(flowsdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceAddress, serviceKey.Index, serviceKey.SequenceNumber).
		SetPayer(serviceAddress)

	signer, err := serviceKey.Signer()
	if err != nil {
		return err
	}

	err = tx.SignEnvelope(serviceAddress, serviceKey.Index, signer)
	if err != nil {
		return err
	}

	err = b.AddTransaction(*convert.SDKTransactionToFlow(*tx))
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
