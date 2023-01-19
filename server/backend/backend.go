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

package backend

import (
	"context"
	"fmt"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"

	emulator "github.com/onflow/flow-emulator"
	convert "github.com/onflow/flow-emulator/convert/sdk"
)

//sdk missing blockStatus
type BlockStatus int

const (
	// BlockStatusUnknown indicates that the block status is not known.
	BlockStatusUnknown BlockStatus = iota
	// BlockStatusFinalized is the status of a finalized block.
	BlockStatusFinalized
	// BlockStatusSealed is the status of a sealed block.
	BlockStatusSealed
)

// Backend wraps an emulated blockchain and makes it flow-go-sdk compatible
type Backend struct {
	emulator emulator.Emulator
}

// New returns a new backend.
func New(emulator emulator.Emulator) *Backend {
	return &Backend{
		emulator: emulator,
	}
}

func (b *Backend) SetEmulator(emulator emulator.Emulator) {
	b.emulator = emulator
}

func (b *Backend) Emulator() emulator.Emulator {
	return b.emulator
}

func (b *Backend) EnableAutoMine() {
	b.emulator.EnableAutoMine()
}

func (b *Backend) DisableAutoMine() {
	b.emulator.DisableAutoMine()
}

func (b *Backend) Ping(ctx context.Context) error {
	return b.emulator.Ping()
}

func (b *Backend) GetChainID(ctx context.Context) sdk.ChainID {
	return sdk.ChainID(b.emulator.GetNetworkParameters().ChainID)
}

// GetLatestBlockHeader gets the latest sealed block header.
func (b *Backend) GetLatestBlockHeader(
	_ context.Context,
	_ bool,
) (
	*sdk.BlockHeader,
	BlockStatus,
	error,
) {
	block, _, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, BlockStatusUnknown, err
	}

	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}

	// this should always return latest sealed block
	return &blockHeader, BlockStatusSealed, nil
}

// GetBlockHeaderByHeight gets a block header by height.
func (b *Backend) GetBlockHeaderByHeight(
	_ context.Context,
	height uint64,
) (
	*sdk.BlockHeader,
	BlockStatus,
	error,
) {
	block, _, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, BlockStatusUnknown, err
	}

	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return &blockHeader, BlockStatusSealed, nil
}

// GetBlockHeaderByID gets a block header by ID.
func (b *Backend) GetBlockHeaderByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*sdk.BlockHeader,
	BlockStatus,
	error,
) {
	block, _, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, BlockStatusUnknown, err
	}

	blockHeader := sdk.BlockHeader{
		ID:        sdk.Identifier(block.ID()),
		ParentID:  sdk.Identifier(block.Header.ParentID),
		Height:    block.Header.Height,
		Timestamp: block.Header.Timestamp,
	}
	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return &blockHeader, BlockStatusSealed, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Backend) GetLatestBlock(
	_ context.Context,
	_ bool,
) (
	*sdk.Block,
	BlockStatus,
	error,
) {
	flowBlock, _, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, BlockStatusUnknown, err
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
	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return &block, BlockStatusSealed, nil
}

// GetBlockByHeight gets a block by height.
func (b *Backend) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (
	*sdk.Block,
	BlockStatus,
	error,
) {
	flowBlock, _, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, BlockStatusUnknown, err
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

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return &block, BlockStatusSealed, nil
}

// GetBlockByID gets a block by ID.
func (b *Backend) GetBlockByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*sdk.Block,
	BlockStatus,
	error,
) {
	flowBlock, _, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, BlockStatusUnknown, err
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

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return &block, BlockStatusSealed, nil
}

// GetCollectionByID gets a collection by ID.
func (b *Backend) GetCollectionByID(
	_ context.Context,
	id sdk.Identifier,
) (*sdk.Collection, error) {
	col, err := b.emulator.GetCollectionByID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}

	sdkCol := convert.FlowLightCollectionToSDK(*col)

	return &sdkCol, nil
}

// SendTransaction submits a transaction to the network.
func (b *Backend) SendTransaction(ctx context.Context, tx sdk.Transaction) error {
	flowTx := convert.SDKTransactionToFlow(tx)
	return b.emulator.SendTransaction(flowTx)
}

// GetTransaction gets a transaction by ID.
func (b *Backend) GetTransaction(
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
func (b *Backend) GetTransactionResult(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.TransactionResult, error) {
	result, err := b.emulator.GetTransactionResult(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}

	events, err := convert.FlowEventsToSDK(result.Events)
	if err != nil {
		return nil, err
	}

	sdkResult := sdk.TransactionResult{
		Status:        sdk.TransactionStatus(result.Status),
		Error:         fmt.Errorf(result.ErrorMessage), //TODO: bluesign: fix this
		Events:        events,
		TransactionID: sdk.Identifier(result.TransactionID),
		BlockHeight:   result.BlockHeight,
		BlockID:       sdk.Identifier(result.BlockID),
	}
	return &sdkResult, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (b *Backend) GetAccount(
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
func (b *Backend) GetAccountAtLatestBlock(
	ctx context.Context,
	address sdk.Address,
) (*sdk.Account, error) {

	account, err := b.getAccount(address)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *Backend) getAccount(address sdk.Address) (*sdk.Account, error) {
	account, err := b.emulator.GetAccount(convert.SDKAddressToFlow(address))
	if err != nil {
		return nil, err
	}

	sdkAccount, err := convert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}
	return &sdkAccount, nil
}

func (b *Backend) GetAccountAtBlockHeight(
	ctx context.Context,
	address sdk.Address,
	height uint64,
) (*sdk.Account, error) {

	account, err := b.emulator.GetAccountAtBlockHeight(convert.SDKAddressToFlow(address), height)
	if err != nil {
		return nil, err
	}

	sdkAccount, err := convert.FlowAccountToSDK(*account)
	if err != nil {
		return nil, err
	}
	return &sdkAccount, nil

}

// ExecuteScriptAtLatestBlock executes a script at a the latest block
func (b *Backend) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	block, _, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlock(script, arguments, block.Header.Height)
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height
func (b *Backend) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	return b.executeScriptAtBlock(script, arguments, blockHeight)
}

// ExecuteScriptAtBlockID executes a script at a specific block ID
func (b *Backend) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID sdk.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	block, _, err := b.emulator.GetBlockByID(convert.SDKIdentifierToFlow(blockID))
	if err != nil {
		return nil, err
	}

	return b.executeScriptAtBlock(script, arguments, block.Header.Height)
}

// executeScriptAtBlock is a helper for executing a script at a specific block
func (b *Backend) executeScriptAtBlock(script []byte, arguments [][]byte, blockHeight uint64) ([]byte, error) {
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

func (b *Backend) GetAccountStorage(address sdk.Address) (*emulator.AccountStorage, error) {
	return b.emulator.GetAccountStorage(convert.SDKAddressToFlow(address))
}

func (b *Backend) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (b *Backend) GetExecutionResultForBlockID(_ context.Context, _ sdk.Identifier) (*sdk.ExecutionResult, error) {
	return nil, nil
}

func (b *Backend) GetTransactionResultByIndex(ctx context.Context, id sdk.Identifier, index uint32) (*sdk.TransactionResult, error) {
	result, err := b.emulator.GetTransactionResultByIndex(convert.SDKIdentifierToFlow(id), index)
	if err != nil {
		return nil, err
	}

	events, err := convert.FlowEventsToSDK(result.Events)
	if err != nil {
		return nil, err
	}

	sdkResult := sdk.TransactionResult{
		Status:        sdk.TransactionStatus(result.Status),
		Error:         fmt.Errorf(result.ErrorMessage), //TODO: bluesign: fix this
		Events:        events,
		TransactionID: sdk.Identifier(result.TransactionID),
		BlockHeight:   result.BlockHeight,
		BlockID:       sdk.Identifier(result.BlockID),
	}

	return &sdkResult, nil

}

func (b *Backend) GetTransactionsByBlockID(ctx context.Context, id sdk.Identifier) (result []*sdk.Transaction, err error) {
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

func (b *Backend) GetTransactionResultsByBlockID(ctx context.Context, id sdk.Identifier) (result []*sdk.TransactionResult, err error) {
	transactionResults, err := b.emulator.GetTransactionResultsByBlockID(convert.SDKIdentifierToFlow(id))
	if err != nil {
		return nil, err
	}

	for _, transactionResult := range transactionResults {
		events, err := convert.FlowEventsToSDK(transactionResult.Events)
		if err != nil {
			return nil, err
		}

		sdkResult := sdk.TransactionResult{
			Status:        sdk.TransactionStatus(transactionResult.Status),
			Error:         fmt.Errorf(transactionResult.ErrorMessage), //TODO: bluesign: fix this
			Events:        events,
			TransactionID: sdk.Identifier(transactionResult.TransactionID),
			BlockHeight:   transactionResult.BlockHeight,
			BlockID:       sdk.Identifier(transactionResult.BlockID),
		}
		result = append(result, &sdkResult)

	}
	return result, nil
}

func (b *Backend) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []sdk.Identifier) (result []*sdk.BlockEvents, err error) {
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

func (b *Backend) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) (result []*sdk.BlockEvents, err error) {
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
