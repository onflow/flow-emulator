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

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
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

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetLatestBlockHeader called")

	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockHeaderByHeight called")

	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByID(_ context.Context, id flowgo.Identifier) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockHeaderByID called")

	return block.Header, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetLatestBlock(_ context.Context, _ bool) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetLatestBlock called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByHeight(_ context.Context, height uint64) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockByHeight called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByID(_ context.Context, id flowgo.Identifier) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockByID called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetCollectionByID(_ context.Context, id flowgo.Identifier) (*flowgo.LightCollection, error) {
	collection, err := a.emulator.GetCollectionByID(id)
	if err != nil {
		return nil, convertError(err)
	}

	a.logger.Debug().
		Str("colID", id.String()).
		Msg("📚  GetCollectionByID called")

	return collection, nil
}

func (a *AccessAdapter) GetTransaction(_ context.Context, id flowgo.Identifier) (*flowgo.TransactionBody, error) {
	tx, err := a.emulator.GetTransaction(id)
	if err != nil {
		return nil, convertError(err)
	}

	a.logger.Debug().
		Str("txID", id.String()).
		Msg("💵  GetTransaction called")

	return tx, nil
}

func (a *AccessAdapter) GetTransactionResult(
	_ context.Context,
	id flowgo.Identifier,
	_ flowgo.Identifier,
	_ flowgo.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (
	*access.TransactionResult,
	error,
) {
	result, err := a.emulator.GetTransactionResult(id)
	if err != nil {
		return nil, convertError(err)
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		result.Events, err = ConvertCCFEventsToJsonEvents(result.Events)
		if err != nil {
			return nil, convertError(err)
		}
	}
	a.logger.Debug().
		Str("txID", id.String()).
		Msg("📝  GetTransactionResult called")

	return result, nil
}

func (a *AccessAdapter) GetAccount(_ context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.emulator.GetAccount(address)
	if err != nil {
		return nil, convertError(err)
	}

	a.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccount called")

	return account, nil
}

func (a *AccessAdapter) GetAccountAtLatestBlock(ctx context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.GetAccount(ctx, address)
	if err != nil {
		return nil, convertError(err)
	}

	a.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccountAtLatestBlock called")

	return account, nil
}

func (a *AccessAdapter) GetAccountAtBlockHeight(
	_ context.Context,
	address flowgo.Address,
	height uint64,
) (*flowgo.Account, error) {

	a.logger.Debug().
		Stringer("address", address).
		Uint64("height", height).
		Msg("👤  GetAccountAtBlockHeight called")

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
	a.logger.Debug().Msg("👤  ExecuteScriptAtLatestBlock called")
	result, err := a.emulator.ExecuteScript(script, arguments)
	if err == nil {
		utils.PrintScriptResult(a.logger, result)
	}
	return convertScriptResult(result, err)
}

func (a *AccessAdapter) ExecuteScriptAtBlockHeight(
	_ context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	a.logger.Debug().
		Uint64("blockHeight", blockHeight).
		Msg("👤  ExecuteScriptAtBlockHeight called")

	result, err := a.emulator.ExecuteScriptAtBlockHeight(script, arguments, blockHeight)
	if err == nil {
		utils.PrintScriptResult(a.logger, result)
	}
	return convertScriptResult(result, err)
}

func (a *AccessAdapter) ExecuteScriptAtBlockID(
	_ context.Context,
	blockID flowgo.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {

	a.logger.Debug().
		Stringer("blockID", blockID).
		Msg("👤  ExecuteScriptAtBlockID called")

	result, err := a.emulator.ExecuteScriptAtBlockID(script, arguments, blockID)
	if err == nil {
		utils.PrintScriptResult(a.logger, result)
	}
	return convertScriptResult(result, err)
}

func (a *AccessAdapter) GetEventsForHeightRange(
	_ context.Context,
	eventType string,
	startHeight, endHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flowgo.BlockEvents, error) {
	events, err := a.emulator.GetEventsForHeightRange(eventType, startHeight, endHeight)
	if err != nil {
		return nil, convertError(err)
	}

	eventCount := 0

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range events {
			events[i].Events, err = ConvertCCFEventsToJsonEvents(events[i].Events)
			eventCount = eventCount + len(events[i].Events)
			if err != nil {
				return nil, convertError(err)
			}
		}
	}

	a.logger.Debug().Fields(map[string]any{
		"eventType":   eventType,
		"startHeight": startHeight,
		"endHeight":   endHeight,
		"eventCount":  eventCount,
	}).Msg("🎁  GetEventsForHeightRange called")

	return events, nil
}

func (a *AccessAdapter) GetEventsForBlockIDs(
	_ context.Context,
	eventType string,
	blockIDs []flowgo.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]flowgo.BlockEvents, error) {
	events, err := a.emulator.GetEventsForBlockIDs(eventType, blockIDs)
	if err != nil {
		return nil, convertError(err)
	}

	eventCount := 0

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range events {
			events[i].Events, err = ConvertCCFEventsToJsonEvents(events[i].Events)
			eventCount = eventCount + len(events[i].Events)
			if err != nil {
				return nil, convertError(err)
			}
		}
	}

	a.logger.Debug().Fields(map[string]any{
		"eventType":  eventType,
		"eventCount": eventCount,
	}).Msg("🎁  GetEventsForBlockIDs called")

	return events, nil
}

func (a *AccessAdapter) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (a *AccessAdapter) GetProtocolStateSnapshotByBlockID(_ context.Context, _ flowgo.Identifier) ([]byte, error) {
	return nil, nil
}

func (a *AccessAdapter) GetProtocolStateSnapshotByHeight(_ context.Context, _ uint64) ([]byte, error) {
	return nil, nil
}

func (a *AccessAdapter) GetExecutionResultForBlockID(_ context.Context, _ flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetExecutionResultByID(_ context.Context, _ flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetSystemTransaction(_ context.Context, _ flowgo.Identifier) (*flowgo.TransactionBody, error) {
	return nil, nil
}

func (a *AccessAdapter) GetSystemTransactionResult(_ context.Context, _ flowgo.Identifier, _ entities.EventEncodingVersion) (*access.TransactionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetTransactionResultByIndex(
	_ context.Context,
	blockID flowgo.Identifier,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	results, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}
	if len(results) <= int(index) {
		return nil, convertError(&types.TransactionNotFoundError{ID: flowgo.Identifier{}})
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range results {
			results[i].Events, err = ConvertCCFEventsToJsonEvents(results[i].Events)
			if err != nil {
				return nil, convertError(err)
			}
		}
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

func (a *AccessAdapter) GetTransactionResultsByBlockID(
	_ context.Context,
	blockID flowgo.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*access.TransactionResult, error) {
	result, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err)
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range result {
			result[i].Events, err = ConvertCCFEventsToJsonEvents(result[i].Events)
			if err != nil {
				return nil, convertError(err)
			}
		}
	}

	return result, nil
}

func (a *AccessAdapter) SendTransaction(_ context.Context, tx *flowgo.TransactionBody) error {
	a.logger.Debug().
		Str("txID", tx.ID().String()).
		Msg(`✉️   Transaction submitted`)

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

func (a *AccessAdapter) SubscribeBlocks(_ context.Context, _ flowgo.Identifier, _ uint64, _ flowgo.BlockStatus) subscription.Subscription {
	return nil
}

func (a *AccessAdapter) SubscribeBlockHeaders(_ context.Context, _ flowgo.Identifier, _ uint64, _ flowgo.BlockStatus) subscription.Subscription {
	return nil
}

func (a *AccessAdapter) SubscribeBlockDigests(_ context.Context, _ flowgo.Identifier, _ uint64, _ flowgo.BlockStatus) subscription.Subscription {
	return nil
}

func ConvertCCFEventsToJsonEvents(events []flowgo.Event) ([]flowgo.Event, error) {
	converted := make([]flowgo.Event, 0, len(events))

	for _, event := range events {
		evt, err := convert.CcfEventToJsonEvent(event)
		if err != nil {
			return nil, err
		}
		converted = append(converted, *evt)
	}

	return converted, nil
}
