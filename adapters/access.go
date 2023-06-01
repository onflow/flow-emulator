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
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"
)

var _ access.API = &AccessAdapter{}

// AccessAdapter wraps the emulator adapters to be compatible with access.API.
type AccessAdapter struct {
	logger   *zerolog.Logger
	emulator emulator.Emulator
}

// NewAccessAdapter returns a new AccessAdapter.
func NewAccessAdapter(logger *zerolog.Logger, emulator emulator.Emulator) *AccessAdapter {
	return &AccessAdapter{
		logger:   logger,
		emulator: emulator,
	}
}

func convertError(err error) error {
	if err != nil {
		switch err.(type) {
		case types.InvalidArgumentError:
			return status.Error(codes.InvalidArgument, err.Error())
		case types.NotFoundError:
			return status.Error(codes.NotFound, err.Error())
		default:
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (a *AccessAdapter) Ping(_ context.Context) error {
	return convertError(a.emulator.Ping())
}

func (a *AccessAdapter) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return a.emulator.GetNetworkParameters()
}

func (a *AccessAdapter) GetLatestBlockHeader(_ context.Context, _ bool) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByID(_ context.Context, id flowgo.Identifier) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetLatestBlock(_ context.Context, _ bool) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByHeight(_ context.Context, height uint64) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByID(_ context.Context, id flowgo.Identifier) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetCollectionByID(_ context.Context, id flowgo.Identifier) (*flowgo.LightCollection, error) {
	collection, err := a.emulator.GetCollectionByID(id)
	if err != nil {
		return nil, convertError(err)
	}
	return collection, nil
}

func (a *AccessAdapter) GetTransaction(_ context.Context, id flowgo.Identifier) (*flowgo.TransactionBody, error) {
	tx, err := a.emulator.GetTransaction(id)
	if err != nil {
		return nil, convertError(err)
	}
	return tx, nil
}

func (a *AccessAdapter) GetTransactionResult(
	_ context.Context,
	id flowgo.Identifier,
	_ flowgo.Identifier,
	_ flowgo.Identifier,
) (
	*access.TransactionResult,
	error,
) {
	result, err := a.emulator.GetTransactionResult(id)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *AccessAdapter) GetAccount(_ context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.emulator.GetAccount(address)
	if err != nil {
		return nil, convertError(err)
	}
	return account, nil
}

func (a *AccessAdapter) GetAccountAtLatestBlock(ctx context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.GetAccount(ctx, address)
	if err != nil {
		return nil, convertError(err)
	}
	return account, nil
}

func (a *AccessAdapter) GetAccountAtBlockHeight(
	_ context.Context,
	address flowgo.Address,
	height uint64,
) (*flowgo.Account, error) {
	account, err := a.emulator.GetAccountAtBlockHeight(address, height)
	if err != nil {
		return nil, convertError(err)
	}
	return account, nil
}

func convertScriptResult(result *types.ScriptResult, err error) ([]byte, error) {
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if !result.Succeeded() {
		return nil, result.Error
	}

	valueBytes, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return valueBytes, nil
}

func (a *AccessAdapter) ExecuteScriptAtLatestBlock(
	_ context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.emulator.ExecuteScript(script, arguments))
}

func (a *AccessAdapter) ExecuteScriptAtBlockHeight(
	_ context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.emulator.ExecuteScriptAtBlockHeight(script, arguments, blockHeight))
}

func (a *AccessAdapter) ExecuteScriptAtBlockID(
	_ context.Context,
	blockID flowgo.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.emulator.ExecuteScriptAtBlockID(script, arguments, blockID))
}

func (a *AccessAdapter) GetEventsForHeightRange(
	_ context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]flowgo.BlockEvents, error) {
	events, err := a.emulator.GetEventsForHeightRange(eventType, startHeight, endHeight)
	if err != nil {
		return nil, convertError(err)
	}
	return events, nil
}

func (a *AccessAdapter) GetEventsForBlockIDs(
	_ context.Context,
	eventType string,
	blockIDs []flowgo.Identifier,
) ([]flowgo.BlockEvents, error) {
	events, err := a.emulator.GetEventsForBlockIDs(eventType, blockIDs)
	if err != nil {
		return nil, convertError(err)
	}
	return events, nil
}

func (a *AccessAdapter) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (a *AccessAdapter) GetExecutionResultForBlockID(_ context.Context, _ flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetExecutionResultByID(_ context.Context, _ flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetTransactionResultByIndex(_ context.Context, blockID flowgo.Identifier, index uint32) (*access.TransactionResult, error) {
	results, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	if len(results) < int(index) {
		return nil, convertError(&types.TransactionNotFoundError{ID: flowgo.Identifier{}})
	}
	return results[index], nil
}

func (a *AccessAdapter) GetTransactionsByBlockID(_ context.Context, blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	result, err := a.emulator.GetTransactionsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *AccessAdapter) GetTransactionResultsByBlockID(_ context.Context, blockID flowgo.Identifier) ([]*access.TransactionResult, error) {
	result, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *AccessAdapter) SendTransaction(_ context.Context, tx *flowgo.TransactionBody) error {
	return convertError(a.emulator.SendTransaction(tx))
}

func (a *AccessAdapter) GetNodeVersionInfo(
	_ context.Context,
) (
	*access.NodeVersionInfo,
	error,
) {
	return nil, fmt.Errorf("not supported")
}
