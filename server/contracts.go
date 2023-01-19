package server

import (
	"fmt"

	emulator "github.com/onflow/flow-emulator"
	sdkconvert "github.com/onflow/flow-emulator/convert/sdk"
	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/fvm"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-nft/lib/go/contracts"
	fusd "github.com/onflow/fusd/lib/go/contracts"
	nftstorefront "github.com/onflow/nft-storefront/lib/go/contracts"
)

type DeployDescription struct {
	name        string
	address     flowgo.Address
	description string
}

func deployContracts(b *emulator.Blockchain) ([]DeployDescription, error) {
	ftAddress := flowsdk.HexToAddress(fvm.FungibleTokenAddress(b.GetChain()).Hex())
	serviceAddress := flowsdk.HexToAddress(b.ServiceKey().Address.Hex())

	toDeploy := []struct {
		name        string
		description string
		source      []byte
	}{
		{
			name:        "FUSD",
			description: "💵  FUSD contract",
			source:      fusd.FUSD(ftAddress.String()),
		},
		{
			name:        "NonFungibleToken",
			description: "✨  NFT contract",
			source:      contracts.NonFungibleToken(),
		},
		{
			name:        "MetadataViews",
			description: "✨  Metadata views contract",
			source:      contracts.MetadataViews(ftAddress, serviceAddress),
		},
		{
			name:        "ExampleNFT",
			description: "✨  Example NFT contract",
			source:      contracts.ExampleNFT(serviceAddress, serviceAddress),
		},
		{
			name:        "NFTStorefrontV2",
			description: "✨   NFT Storefront contract v2",
			source:      nftstorefront.NFTStorefront(2, ftAddress.String(), serviceAddress.String()),
		},
		{
			name:        "NFTStorefront",
			description: "✨   NFT Storefront contract",
			source:      nftstorefront.NFTStorefront(1, ftAddress.String(), serviceAddress.String()),
		},
	}

	for _, c := range toDeploy {
		err := deployContract(b, c.name, c.source)
		if err != nil {
			return nil, err
		}
	}

	serviceAcct, err := b.GetAccount(b.ServiceKey().Address)
	if err != nil {
		return nil, err
	}

	deployDescriptions := make([]DeployDescription, 0)
	for _, c := range toDeploy {
		_, ok := serviceAcct.Contracts[c.name]
		if !ok {
			continue
		}
		deployDescriptions = append(
			deployDescriptions,
			DeployDescription{
				name:        c.name,
				address:     b.ServiceKey().Address,
				description: c.description,
			},
		)
	}

	return deployDescriptions, nil
}

func deployContract(b *emulator.Blockchain, name string, contract []byte) error {

	serviceKey := b.ServiceKey()
	serviceAddress := flowsdk.HexToAddress(b.ServiceKey().Address.Hex())

	if serviceKey.PrivateKey == nil {
		return fmt.Errorf("not able to deploy contracts without set private key")
	}

	latestBlock, _, err := b.GetLatestBlock()
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

	err = b.AddTransaction(*sdkconvert.SDKTransactionToFlow(*tx))
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
