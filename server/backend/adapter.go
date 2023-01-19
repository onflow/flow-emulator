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

	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ access.API = &Adapter{}

// Adapter wraps the emulator backend to be compatible with access.API.
type Adapter struct {
	backend *Backend
	logger  *logrus.Logger
}

func convertError(err error) error {
	if err != nil {
		switch err.(type) {
		case emulator.InvalidArgumentError:
			return status.Error(codes.InvalidArgument, err.Error())
		case emulator.NotFoundError:
			return status.Error(codes.NotFound, err.Error())
		default:
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

// NewAdapter returns a new backend adapter.
func NewAdapter(logger *logrus.Logger, backend *Backend) *Adapter {
	return &Adapter{
		backend: backend,
		logger:  logger,
	}
}

func (a *Adapter) Ping(ctx context.Context) error {
	return convertError(a.backend.Emulator().Ping())
}

func (a *Adapter) GetNetworkParameters(ctx context.Context) access.NetworkParameters {
	return a.backend.Emulator().GetNetworkParameters()
}

func (a *Adapter) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, blockStatus, nil
}

func (a *Adapter) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, blockStatus, nil
}

func (a *Adapter) GetBlockHeaderByID(ctx context.Context, id flowgo.Identifier) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block.Header, blockStatus, nil
}

func (a *Adapter) GetLatestBlock(ctx context.Context, isSealed bool) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, blockStatus, nil
}

func (a *Adapter) GetBlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, blockStatus, nil
}

func (a *Adapter) GetBlockByID(ctx context.Context, id flowgo.Identifier) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, blockStatus, err := a.backend.Emulator().GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}
	return block, blockStatus, nil
}

func (a *Adapter) GetCollectionByID(ctx context.Context, id flowgo.Identifier) (*flowgo.LightCollection, error) {
	collection, err := a.backend.Emulator().GetCollectionByID(id)
	if err != nil {
		return nil, convertError(err)
	}
	return collection, nil
}

func (a *Adapter) GetTransaction(ctx context.Context, id flowgo.Identifier) (*flowgo.TransactionBody, error) {
	tx, err := a.backend.Emulator().GetTransaction(id)
	if err != nil {
		return nil, convertError(err)
	}
	return tx, nil
}

func (a *Adapter) GetTransactionResult(ctx context.Context, id flowgo.Identifier) (*access.TransactionResult, error) {
	result, err := a.backend.Emulator().GetTransactionResult(id)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *Adapter) GetAccount(ctx context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.backend.Emulator().GetAccount(address)
	if err != nil {
		return nil, convertError(err)
	}
	return account, nil
}

func (a *Adapter) GetAccountAtLatestBlock(ctx context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.GetAccount(ctx, address)
	if err != nil {
		return nil, convertError(err)
	}
	return account, nil
}

func (a *Adapter) GetAccountAtBlockHeight(
	ctx context.Context,
	address flowgo.Address,
	height uint64,
) (*flowgo.Account, error) {
	account, err := a.backend.Emulator().GetAccountAtBlockHeight(address, height)
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

func (a *Adapter) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.backend.Emulator().ExecuteScript(script, arguments))
}

func (a *Adapter) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.backend.Emulator().ExecuteScriptAtBlockHeight(script, arguments, blockHeight))
}

func (a *Adapter) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flowgo.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return convertScriptResult(a.backend.Emulator().ExecuteScriptAtBlockID(script, arguments, blockID))
}

func (a *Adapter) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]flowgo.BlockEvents, error) {
	events, err := a.backend.Emulator().GetEventsForHeightRange(eventType, startHeight, endHeight)
	if err != nil {
		return nil, convertError(err)
	}
	return events, nil
}

func (a *Adapter) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []flowgo.Identifier,
) ([]flowgo.BlockEvents, error) {
	events, err := a.backend.Emulator().GetEventsForBlockIDs(eventType, blockIDs)
	if err != nil {
		return nil, convertError(err)
	}
	return events, nil
}

func (a *Adapter) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (a *Adapter) GetExecutionResultForBlockID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *Adapter) GetExecutionResultByID(ctx context.Context, id flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *Adapter) GetTransactionResultByIndex(ctx context.Context, blockID flowgo.Identifier, index uint32) (*access.TransactionResult, error) {
	result, err := a.backend.Emulator().GetTransactionResultByIndex(blockID, index)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *Adapter) GetTransactionsByBlockID(ctx context.Context, blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	result, err := a.backend.Emulator().GetTransactionsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *Adapter) GetTransactionResultsByBlockID(ctx context.Context, blockID flowgo.Identifier) ([]*access.TransactionResult, error) {
	result, err := a.backend.Emulator().GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	return result, nil
}

func (a *Adapter) SendTransaction(ctx context.Context, tx *flowgo.TransactionBody) error {
	return convertError(a.backend.Emulator().SendTransaction(tx))
}
