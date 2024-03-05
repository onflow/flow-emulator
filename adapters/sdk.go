/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package adapters

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-emulator/convert"
	emulator "github.com/onflow/flow-emulator/emulator"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SDKAdapter wraps an emulated emulator and implements the RPC handlers
// required by the Access API.
type SDKAdapter struct {
	logger   *zerolog.Logger
	emulator emulator.Emulator
}

func (b *SDKAdapter) EnableAutoMine() {
	b.emulator.EnableAutoMine()
}
func (b *SDKAdapter) DisableAutoMine() {
	b.emulator.DisableAutoMine()
}

func (b *SDKAdapter) Emulator() emulator.Emulator {
	return b.emulator
}

// NewSDKAdapter returns a new SDKAdapter.
func NewSDKAdapter(logger *zerolog.Logger, emulator emulator.Emulator) *SDKAdapter {
	return &SDKAdapter{
		logger:   logger,
		emulator: emulator,
	}
}

func (b *SDKAdapter) Ping(ctx context.Context) error {
	return b.emulator.Ping()
}

func (b *SDKAdapter) GetChainID(ctx context.Context) sdk.ChainID {
	return sdk.ChainID(b.emulator.GetNetworkParameters().ChainID)
}

// GetLatestBlockHeader gets the latest sealed block header.
func (b *SDKAdapter) GetLatestBlockHeader(
	_ context.Context,
	_ bool,
) (
	*sdk.BlockHeader,
	sdk.BlockStatus,
	error,
) {
	block, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, sdk.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}
	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}
	return &blockHeader, sdk.BlockStatusSealed, nil
}

// GetBlockHeaderByHeight gets a block header by height.
func (b *SDKAdapter) GetBlockHeaderByHeight(
	_ context.Context,
	height uint64,
) (
	*sdk.BlockHeader,
	sdk.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, sdk.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}
	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}
	return &blockHeader, sdk.BlockStatusSealed, nil
}

// GetBlockHeaderByID gets a block header by ID.
func (b *SDKAdapter) GetBlockHeaderByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*sdk.BlockHeader,
	sdk.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, sdk.BlockStatusUnknown, err
	}
	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}
	return &blockHeader, sdk.BlockStatusSealed, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *SDKAdapter) GetLatestBlock(
	_ context.Context,
	_ bool,
) (
	*sdk.Block,
	sdk.BlockStatus,
	error,
) {
	flowBlock, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, sdk.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}
	block := sdk.Block{
		BlockHeader: sdk.BlockHeader{
			ID:        sdk.Identifier(flowBlock.ID()),
			ParentID:  sdk.Identifier(flowBlock.Header.ParentID),
			Height:    flowBlock.Header.Height,
			Timestamp: flowBlock.Header.Timestamp,
		},
		BlockPayload: sdk.BlockPayload{},
	}
	return &block, sdk.BlockStatusSealed, nil
}

// GetBlockByHeight gets a block by height.
func (b *SDKAdapter) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (
	*sdk.Block,
	sdk.BlockStatus,
	error,
) {
	flowBlock, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, sdk.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}
	block := sdk.Block{
		BlockHeader: sdk.BlockHeader{
			ID:        sdk.Identifier(flowBlock.ID()),
			ParentID:  sdk.Identifier(flowBlock.Header.ParentID),
			Height:    flowBlock.Header.Height,
			Timestamp: flowBlock.Header.Timestamp,
		},
		BlockPayload: sdk.BlockPayload{},
	}
	return &block, sdk.BlockStatusSealed, nil
}

// GetBlockByID gets a block by ID.
func (b *SDKAdapter) GetBlockByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*sdk.Block,
	sdk.BlockStatus,
	error,
) {
	flowBlock, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, sdk.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}
	block := sdk.Block{
		BlockHeader: sdk.BlockHeader{
			ID:        sdk.Identifier(flowBlock.ID()),
			ParentID:  sdk.Identifier(flowBlock.Header.ParentID),
			Height:    flowBlock.Header.Height,
			Timestamp: flowBlock.Header.Timestamp,
		},
		BlockPayload: sdk.BlockPayload{},
	}
	return &block, sdk.BlockStatusSealed, nil
}

// GetCollectionByID gets a collection by ID.
func (b *SDKAdapter) GetCollectionByID(
	_ context.Context,
	id sdk.Identifier,
) (*sdk.Collection, error) {
	flowCollection, err := b.emulator.GetCollectionByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}
	collection := convert.FlowLightCollectionToSDK(*flowCollection)
	return &collection, nil
}

func (b *SDKAdapter) SendTransaction(ctx context.Context, tx sdk.Transaction) error {
	flowTx := convert.SDKTransactionToFlow(tx)
	return b.emulator.SendTransaction(flowTx)
}

// GetTransaction gets a transaction by ID.
func (b *SDKAdapter) GetTransaction(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Transaction, error) {
	tx, err := b.emulator.GetTransaction(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}
	sdkTx := convert.FlowTransactionToSDK(*tx)
	return &sdkTx, nil
}

// GetTransactionResult gets a transaction by ID.
func (b *SDKAdapter) GetTransactionResult(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.TransactionResult, error) {
	flowResult, err := b.emulator.GetTransactionResult(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}
	return convert.FlowTransactionResultToSDK(flowResult)
}

// GetAccount returns an account by address at the latest sealed block.
func (b *SDKAdapter) GetAccount(
	ctx context.Context,
	address sdk.Address,
) (*sdk.Account, error) {
	account, err := b.getAccount(address)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetAccountAtLatestBlock returns an account by address at the latest sealed block.
func (b *SDKAdapter) GetAccountAtLatestBlock(
	ctx context.Context,
	address sdk.Address,
) (*sdk.Account, error) {
	account, err := b.getAccount(address)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (b *SDKAdapter) getAccount(address sdk.Address) (*sdk.Account, error) {
	account, err := b.emulator.GetAccount(convert.SDKAddressToFlow(address))
	if err != nil {
		return nil, err
	}
	return convert.FlowAccountToSDK(*account)
}

func (b *SDKAdapter) GetAccountAtBlockHeight(
	ctx context.Context,
	address sdk.Address,
	height uint64,
) (*sdk.Account, error) {
	account, err := b.emulator.GetAccountAtBlockHeight(convert.SDKAddressToFlow(address), height)
	if err != nil {
		return nil, err
	}
	return convert.FlowAccountToSDK(*account)
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block
func (b *SDKAdapter) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	block, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return b.executeScriptAtBlock(script, arguments, block.Header.Height)
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height
func (b *SDKAdapter) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return b.executeScriptAtBlock(script, arguments, blockHeight)
}

// ExecuteScriptAtBlockID executes a script at a specific block ID
func (b *SDKAdapter) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID sdk.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	block, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(blockID))
	if err != nil {
		return nil, err
	}
	return b.executeScriptAtBlock(script, arguments, block.Header.Height)
}

// executeScriptAtBlock is a helper for executing a script at a specific block
func (b *SDKAdapter) executeScriptAtBlock(script []byte, arguments [][]byte, blockHeight uint64) ([]byte, error) {
	result, err := b.emulator.ExecuteScriptAtBlockHeight(script, arguments, blockHeight)
	if err != nil {
		return nil, err
	}
	if !result.Succeeded() {
		return nil, result.Error
	}
	valueBytes, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, err
	}
	return valueBytes, nil
}

func (b *SDKAdapter) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (a *SDKAdapter) GetProtocolStateSnapshotByBlockID(_ context.Context, _ flowgo.Identifier) ([]byte, error) {
	return nil, nil
}

func (a *SDKAdapter) GetProtocolStateSnapshotByHeight(_ context.Context, _ uint64) ([]byte, error) {
	return nil, nil
}

func (b *SDKAdapter) GetExecutionResultForBlockID(_ context.Context, _ sdk.Identifier) (*sdk.ExecutionResult, error) {
	return nil, nil
}

func (b *SDKAdapter) GetSystemTransaction(_ context.Context, _ flowgo.Identifier) (*flowgo.TransactionBody, error) {
	return nil, nil
}

func (b *SDKAdapter) GetSystemTransactionResult(_ context.Context, _ flowgo.Identifier, _ entities.EventEncodingVersion) (*access.TransactionResult, error) {
	return nil, nil
}

func (b *SDKAdapter) GetTransactionsByBlockID(ctx context.Context, id sdk.Identifier) ([]*sdk.Transaction, error) {
	result := []*sdk.Transaction{}
	transactions, err := b.emulator.GetTransactionsByBlockID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}
	for _, transaction := range transactions {
		sdkTransaction := convert.FlowTransactionToSDK(*transaction)
		result = append(result, &sdkTransaction)

	}
	return result, nil
}

func (b *SDKAdapter) GetTransactionResultsByBlockID(ctx context.Context, id sdk.Identifier) ([]*sdk.TransactionResult, error) {
	result := []*sdk.TransactionResult{}
	transactionResults, err := b.emulator.GetTransactionResultsByBlockID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}
	for _, transactionResult := range transactionResults {
		sdkResult, err := convert.FlowTransactionResultToSDK(transactionResult)
		if err != nil {
			return nil, err
		}
		result = append(result, sdkResult)
	}
	return result, nil
}

func (b *SDKAdapter) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []sdk.Identifier) ([]*sdk.BlockEvents, error) {
	result := []*sdk.BlockEvents{}
	flowBlockEvents, err := b.emulator.GetEventsForBlockIDs(eventType, convert.SDKIdentifiersToFlow(blockIDs))
	if err != nil {
		return nil, err
	}

	for _, flowBlockEvent := range flowBlockEvents {
		sdkEvents, err := convert.FlowEventsToSDK(flowBlockEvent.Events)
		if err != nil {
			return nil, err
		}

		sdkBlockEvents := &sdk.BlockEvents{
			BlockID:        sdk.Identifier(flowBlockEvent.BlockID),
			Height:         flowBlockEvent.BlockHeight,
			BlockTimestamp: flowBlockEvent.BlockTimestamp,
			Events:         sdkEvents,
		}

		result = append(result, sdkBlockEvents)

	}

	return result, nil
}

func (b *SDKAdapter) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) ([]*sdk.BlockEvents, error) {
	result := []*sdk.BlockEvents{}

	flowBlockEvents, err := b.emulator.GetEventsForHeightRange(eventType, startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	for _, flowBlockEvent := range flowBlockEvents {
		sdkEvents, err := convert.FlowEventsToSDK(flowBlockEvent.Events)

		if err != nil {
			return nil, err
		}

		sdkBlockEvents := &sdk.BlockEvents{
			BlockID:        sdk.Identifier(flowBlockEvent.BlockID),
			Height:         flowBlockEvent.BlockHeight,
			BlockTimestamp: flowBlockEvent.BlockTimestamp,
			Events:         sdkEvents,
		}

		result = append(result, sdkBlockEvents)

	}

	return result, nil
}

// CreateAccount submits a transaction to create a new account with the given
// account keys and contracts. The transaction is paid by the service account.
func (b *SDKAdapter) CreateAccount(ctx context.Context, publicKeys []*sdk.AccountKey, contracts []templates.Contract) (sdk.Address, error) {

	serviceKey := b.emulator.ServiceKey()
	latestBlock, err := b.emulator.GetLatestBlock()

	if err != nil {
		return sdk.Address{}, err
	}

	if publicKeys == nil {
		publicKeys = []*sdk.AccountKey{}
	}
	tx, err := templates.CreateAccount(publicKeys, contracts, serviceKey.Address)
	if err != nil {
		return sdk.Address{}, err
	}

	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(sdk.Identifier(latestBlock.ID())).
		SetProposalKey(serviceKey.Address, serviceKey.Index, serviceKey.SequenceNumber).
		SetPayer(serviceKey.Address)

	signer, err := serviceKey.Signer()
	if err != nil {
		return sdk.Address{}, err
	}

	err = tx.SignEnvelope(serviceKey.Address, serviceKey.Index, signer)
	if err != nil {
		return sdk.Address{}, err
	}

	err = b.SendTransaction(ctx, *tx)
	if err != nil {
		return sdk.Address{}, err
	}

	_, results, err := b.emulator.ExecuteAndCommitBlock()
	if err != nil {
		return sdk.Address{}, err
	}
	lastResult := results[len(results)-1]

	_, err = b.emulator.CommitBlock()
	if err != nil {
		return sdk.Address{}, err
	}

	if !lastResult.Succeeded() {
		return sdk.Address{}, lastResult.Error
	}

	var address sdk.Address

	for _, event := range lastResult.Events {
		if event.Type == sdk.EventAccountCreated {
			address = sdk.Address(event.Value.Fields[0].(cadence.Address))
			break
		}
	}

	if address == (sdk.Address{}) {
		return sdk.Address{}, fmt.Errorf("failed to find AccountCreated event")
	}

	return address, nil
}
