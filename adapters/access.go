/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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
	"time"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
	"github.com/onflow/flow-emulator/utils"
)

var _ access.API = &AccessAdapter{}

// AccessAdapter wraps the emulator adapters to be compatible with accessmodel.API.
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

func convertError(err error, defaultStatusCode codes.Code) error {
	if err != nil {
		switch err.(type) {
		case types.InvalidArgumentError:
			return status.Error(codes.InvalidArgument, err.Error())
		case types.NotFoundError:
			return status.Error(codes.NotFound, err.Error())
		default:
			return status.Error(defaultStatusCode, err.Error())
		}
	}
	return nil
}

func (a *AccessAdapter) Ping(_ context.Context) error {
	return convertError(a.emulator.Ping(), codes.Internal)
}

func (a *AccessAdapter) GetNetworkParameters(_ context.Context) accessmodel.NetworkParameters {
	return a.emulator.GetNetworkParameters()
}

func (a *AccessAdapter) GetLatestBlockHeader(_ context.Context, _ bool) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetLatestBlockHeader called")

	return block.ToHeader(), flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByHeight(_ context.Context, height uint64) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockHeaderByHeight called")

	return block.ToHeader(), flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockHeaderByID(_ context.Context, id flowgo.Identifier) (*flowgo.Header, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockHeaderByID called")

	return block.ToHeader(), flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetLatestBlock(_ context.Context, _ bool) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetLatestBlock called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByHeight(_ context.Context, height uint64) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockByHeight called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetBlockByID(_ context.Context, id flowgo.Identifier) (*flowgo.Block, flowgo.BlockStatus, error) {
	block, err := a.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, convertError(err, codes.Internal)
	}

	a.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Height,
		"blockID":     block.ID().String(),
	}).Msg("🎁  GetBlockByID called")

	return block, flowgo.BlockStatusSealed, nil
}

func (a *AccessAdapter) GetCollectionByID(_ context.Context, id flowgo.Identifier) (*flowgo.LightCollection, error) {
	collection, err := a.emulator.GetCollectionByID(id)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Str("colID", id.String()).
		Msg("📚  GetCollectionByID called")

	return collection, nil
}

func (a *AccessAdapter) GetFullCollectionByID(_ context.Context, id flowgo.Identifier) (*flowgo.Collection, error) {
	collection, err := a.emulator.GetFullCollectionByID(id)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Str("colID", id.String()).
		Msg("📚  GetFullCollectionByID called")

	return collection, nil

}

func (a *AccessAdapter) GetTransaction(_ context.Context, id flowgo.Identifier) (*flowgo.TransactionBody, error) {
	tx, err := a.emulator.GetTransaction(id)
	if err != nil {
		return nil, convertError(err, codes.Internal)
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
	*accessmodel.TransactionResult,
	error,
) {
	result, err := a.emulator.GetTransactionResult(id)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		result.Events, err = ConvertCCFEventsToJsonEvents(result.Events)
		if err != nil {
			return nil, convertError(err, codes.Internal)
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
		return nil, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccount called")

	return account, nil
}

func (a *AccessAdapter) GetAccountAtLatestBlock(ctx context.Context, address flowgo.Address) (*flowgo.Account, error) {
	account, err := a.GetAccount(ctx, address)
	if err != nil {
		return nil, convertError(err, codes.Internal)
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
		return nil, convertError(err, codes.Internal)
	}
	return account, nil
}

func convertScriptResult(result *types.ScriptResult, err error) ([]byte, error) {
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if !result.Succeeded() {
		return nil, status.Error(codes.InvalidArgument, result.Error.Error())
	}

	valueBytes, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, convertError(err, codes.InvalidArgument)
	}

	return valueBytes, nil
}

func (a *AccessAdapter) ExecuteScriptAtLatestBlock(
	_ context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	latestBlock, err := a.emulator.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	a.logger.Debug().
		Uint64("blockHeight", latestBlock.Height).
		Msg("👤  ExecuteScriptAtLatestBlock called")

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
		return nil, convertError(err, codes.Internal)
	}

	eventCount := 0

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range events {
			events[i].Events, err = ConvertCCFEventsToJsonEvents(events[i].Events)
			eventCount = eventCount + len(events[i].Events)
			if err != nil {
				return nil, convertError(err, codes.Internal)
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
		return nil, convertError(err, codes.Internal)
	}

	eventCount := 0

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range events {
			events[i].Events, err = ConvertCCFEventsToJsonEvents(events[i].Events)
			eventCount = eventCount + len(events[i].Events)
			if err != nil {
				return nil, convertError(err, codes.Internal)
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

func (a *AccessAdapter) GetSystemTransaction(_ context.Context, _ flowgo.Identifier, _ flowgo.Identifier) (*flowgo.TransactionBody, error) {
	return nil, nil
}

func (a *AccessAdapter) GetSystemTransactionResult(_ context.Context, _ flowgo.Identifier, _ flowgo.Identifier, _ entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, nil
}

func (a *AccessAdapter) GetAccountBalanceAtLatestBlock(_ context.Context, address flowgo.Address) (uint64, error) {

	account, err := a.emulator.GetAccount(address)
	if err != nil {
		return 0, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccountBalanceAtLatestBlock called")

	return account.Balance, nil
}

func (a *AccessAdapter) GetAccountBalanceAtBlockHeight(_ context.Context, address flowgo.Address, height uint64) (uint64, error) {
	account, err := a.emulator.GetAccountAtBlockHeight(address, height)
	if err != nil {
		return 0, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Stringer("address", address).
		Uint64("height", height).
		Msg("👤  GetAccountBalanceAtBlockHeight called")

	return account.Balance, nil
}

func (a *AccessAdapter) GetAccountKeyAtLatestBlock(_ context.Context, address flowgo.Address, keyIndex uint32) (*flowgo.AccountPublicKey, error) {
	account, err := a.emulator.GetAccount(address)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	for _, key := range account.Keys {
		if key.Index == keyIndex {
			return &key, nil
		}
	}

	a.logger.Debug().
		Stringer("address", address).
		Uint32("keyIndex", keyIndex).
		Msg("👤  GetAccountKeyAtLatestBlock called")

	return nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)
}

func (a *AccessAdapter) GetAccountKeyAtBlockHeight(_ context.Context, address flowgo.Address, keyIndex uint32, height uint64) (*flowgo.AccountPublicKey, error) {
	account, err := a.emulator.GetAccountAtBlockHeight(address, height)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	for _, key := range account.Keys {
		if key.Index == keyIndex {
			return &key, nil
		}
	}

	a.logger.Debug().
		Stringer("address", address).
		Uint32("keyIndex", keyIndex).
		Uint64("height", height).
		Msg("👤  GetAccountKeyAtBlockHeight called")

	return nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)
}

func (a *AccessAdapter) GetAccountKeysAtLatestBlock(_ context.Context, address flowgo.Address) ([]flowgo.AccountPublicKey, error) {
	account, err := a.emulator.GetAccount(address)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccountKeysAtLatestBlock called")

	return account.Keys, nil
}

func (a *AccessAdapter) GetAccountKeysAtBlockHeight(_ context.Context, address flowgo.Address, height uint64) ([]flowgo.AccountPublicKey, error) {
	account, err := a.emulator.GetAccountAtBlockHeight(address, height)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	a.logger.Debug().
		Stringer("address", address).
		Uint64("height", height).
		Msg("👤  GetAccountKeysAtBlockHeight called")

	return account.Keys, nil
}

func (a *AccessAdapter) GetTransactionResultByIndex(
	_ context.Context,
	blockID flowgo.Identifier,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	results, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}
	if len(results) <= int(index) {
		return nil, convertError(&types.TransactionNotFoundError{ID: flowgo.Identifier{}}, codes.Internal)
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range results {
			results[i].Events, err = ConvertCCFEventsToJsonEvents(results[i].Events)
			if err != nil {
				return nil, convertError(err, codes.Internal)
			}
		}
	}

	return results[index], nil
}

func (a *AccessAdapter) GetTransactionsByBlockID(_ context.Context, blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	result, err := a.emulator.GetTransactionsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}
	return result, nil
}

func (a *AccessAdapter) GetTransactionResultsByBlockID(
	_ context.Context,
	blockID flowgo.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	result, err := a.emulator.GetTransactionResultsByBlockID(blockID)
	if err != nil {
		return nil, convertError(err, codes.Internal)
	}

	// Convert CCF events to JSON events, else return CCF encoded version
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for i := range result {
			result[i].Events, err = ConvertCCFEventsToJsonEvents(result[i].Events)
			if err != nil {
				return nil, convertError(err, codes.Internal)
			}
		}
	}

	return result, nil
}

func (a *AccessAdapter) SendTransaction(_ context.Context, tx *flowgo.TransactionBody) error {
	a.logger.Debug().
		Str("txID", tx.ID().String()).
		Msg(`✉️   Transaction submitted`)

	return convertError(a.emulator.SendTransaction(tx), codes.Internal)
}

func (a *AccessAdapter) GetNodeVersionInfo(
	_ context.Context,
) (
	*accessmodel.NodeVersionInfo,
	error,
) {
	return &accessmodel.NodeVersionInfo{}, nil
}

func (a *AccessAdapter) SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flowgo.Identifier, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartBlockID(ctx, startBlockID, a.getBlockResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartHeight(ctx, startHeight, a.getBlockResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlocksFromLatest(ctx context.Context, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromLatest(ctx, a.getBlockResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flowgo.Identifier, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartBlockID(ctx, startBlockID, a.getBlockHeaderResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartHeight(ctx, startHeight, a.getBlockHeaderResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromLatest(ctx, a.getBlockHeaderResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flowgo.Identifier, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartBlockID(ctx, startBlockID, a.getBlockDigestResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromStartHeight(ctx, startHeight, a.getBlockDigestResponse(blockStatus))
}

func (a *AccessAdapter) SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flowgo.BlockStatus) subscription.Subscription {
	return a.subscribeBlocksFromLatest(ctx, a.getBlockDigestResponse(blockStatus))
}

func (a *AccessAdapter) subscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flowgo.Identifier, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	block, err := a.emulator.GetBlockByID(startBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get block by ID")
	}
	
	emulatorBlockchain, ok := a.emulator.(*emulator.Blockchain)
	if !ok {
		return subscription.NewFailedSubscription(fmt.Errorf("emulator is not a Blockchain"), "invalid emulator type")
	}

	sub := subscription.NewHeightBasedSubscription(subscription.DefaultSendBufferSize, block.Height, getData)
	go subscription.NewStreamer(*a.logger, emulatorBlockchain.Broadcaster(), subscription.DefaultSendTimeout, subscription.DefaultResponseLimit, sub).Stream(ctx)
	return sub
}

func (a *AccessAdapter) subscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	emulatorBlockchain, ok := a.emulator.(*emulator.Blockchain)
	if !ok {
		return subscription.NewFailedSubscription(fmt.Errorf("emulator is not a Blockchain"), "invalid emulator type")
	}

	sub := subscription.NewHeightBasedSubscription(subscription.DefaultSendBufferSize, startHeight, getData)
	go subscription.NewStreamer(*a.logger, emulatorBlockchain.Broadcaster(), subscription.DefaultSendTimeout, subscription.DefaultResponseLimit, sub).Stream(ctx)
	return sub
}

func (a *AccessAdapter) subscribeBlocksFromLatest(ctx context.Context, getData subscription.GetDataByHeightFunc) subscription.Subscription {
	block, err := a.emulator.GetLatestBlock()
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get latest block")
	}
	
	emulatorBlockchain, ok := a.emulator.(*emulator.Blockchain)
	if !ok {
		return subscription.NewFailedSubscription(fmt.Errorf("emulator is not a Blockchain"), "invalid emulator type")
	}

	sub := subscription.NewHeightBasedSubscription(subscription.DefaultSendBufferSize, block.Height, getData)
	go subscription.NewStreamer(*a.logger, emulatorBlockchain.Broadcaster(), subscription.DefaultSendTimeout, subscription.DefaultResponseLimit, sub).Stream(ctx)
	return sub
}

func (a *AccessAdapter) getBlockResponse(blockStatus flowgo.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		block, err := a.getBlock(height, blockStatus)
		if err != nil {
			return nil, err
		}

		a.logger.Trace().
			Hex("block_id", logging.ID(block.ID())).
			Uint64("height", height).
			Msgf("sending block info")

		return block, nil
	}
}

func (a *AccessAdapter) getBlockHeaderResponse(blockStatus flowgo.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		header, err := a.getBlockHeader(height, blockStatus)
		if err != nil {
			return nil, err
		}

		a.logger.Trace().
			Hex("block_id", logging.ID(header.ID())).
			Uint64("height", height).
			Msgf("sending block header info")

		return header, nil
	}
}

func (a *AccessAdapter) getBlockDigestResponse(blockStatus flowgo.BlockStatus) subscription.GetDataByHeightFunc {
	return func(_ context.Context, height uint64) (interface{}, error) {
		header, err := a.getBlockHeader(height, blockStatus)
		if err != nil {
			return nil, err
		}

		a.logger.Trace().
			Hex("block_id", logging.ID(header.ID())).
			Uint64("height", height).
			Msgf("sending lightweight block info")

		return flowgo.NewBlockDigest(header.ID(), header.Height, time.Unix(int64(header.Timestamp), 0).UTC()), nil
	}
}

func (a *AccessAdapter) getBlock(height uint64, expectedBlockStatus flowgo.BlockStatus) (*flowgo.Block, error) {
	if err := a.validateHeight(height, expectedBlockStatus); err != nil {
		return nil, err
	}

	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block for height %d: %w", height, subscription.ErrBlockNotReady)
	}

	return block, nil
}

func (a *AccessAdapter) getBlockHeader(height uint64, expectedBlockStatus flowgo.BlockStatus) (*flowgo.Header, error) {
	if err := a.validateHeight(height, expectedBlockStatus); err != nil {
		return nil, err
	}

	block, err := a.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block header for height %d: %w", height, subscription.ErrBlockNotReady)
	}

	return block.ToHeader(), nil
}

func (a *AccessAdapter) validateHeight(height uint64, expectedBlockStatus flowgo.BlockStatus) error {
	// In the emulator, all blocks are immediately sealed, so we only need to check
	// if the block at the requested height exists
	latestBlock, err := a.emulator.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("could not get latest block: %w", err)
	}

	if height > latestBlock.Height {
		return fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
	}

	return nil
}

func (a *AccessAdapter) SubscribeTransactionStatuses(ctx context.Context, txID flowgo.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	// Subscribe from the latest sealed (in emulator's case, latest) block
	latestBlock, err := a.emulator.GetLatestBlock()
	if err != nil {
		return subscription.NewFailedSubscription(err, "failed to lookup latest block")
	}
	
	return a.createTransactionSubscription(ctx, txID, latestBlock.ID(), flowgo.ZeroID, requiredEventEncodingVersion)
}

func (a *AccessAdapter) SubscribeTransactionStatusesFromStartBlockID(ctx context.Context, txID flowgo.Identifier, startBlockID flowgo.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	return a.createTransactionSubscription(ctx, txID, startBlockID, flowgo.ZeroID, requiredEventEncodingVersion)
}

func (a *AccessAdapter) SubscribeTransactionStatusesFromStartHeight(ctx context.Context, txID flowgo.Identifier, startHeight uint64, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	block, err := a.emulator.GetBlockByHeight(startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "failed to get start block")
	}
	
	return a.createTransactionSubscription(ctx, txID, block.ID(), flowgo.ZeroID, requiredEventEncodingVersion)
}

func (a *AccessAdapter) SubscribeTransactionStatusesFromLatest(ctx context.Context, txID flowgo.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	latestBlock, err := a.emulator.GetLatestBlock()
	if err != nil {
		return subscription.NewFailedSubscription(err, "failed to lookup latest block")
	}
	
	return a.createTransactionSubscription(ctx, txID, latestBlock.ID(), flowgo.ZeroID, requiredEventEncodingVersion)
}

func (a *AccessAdapter) SendAndSubscribeTransactionStatuses(ctx context.Context, tx *flowgo.TransactionBody, requiredEventEncodingVersion entities.EventEncodingVersion) subscription.Subscription {
	if err := a.emulator.SendTransaction(tx); err != nil {
		a.logger.Debug().Err(err).Str("tx_id", tx.ID().String()).Msg("failed to send transaction")
		return subscription.NewFailedSubscription(err, "failed to send transaction")
	}

	return a.createTransactionSubscription(ctx, tx.ID(), tx.ReferenceBlockID, tx.ReferenceBlockID, requiredEventEncodingVersion)
}

func (a *AccessAdapter) createTransactionSubscription(
	ctx context.Context,
	txID flowgo.Identifier,
	startBlockID flowgo.Identifier,
	referenceBlockID flowgo.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	startBlock, err := a.emulator.GetBlockByID(startBlockID)
	if err != nil {
		a.logger.Debug().Err(err).Str("block_id", startBlockID.String()).Msg("failed to get start block")
		return subscription.NewFailedSubscription(err, "failed to get start block")
	}

	emulatorBlockchain, ok := a.emulator.(*emulator.Blockchain)
	if !ok {
		return subscription.NewFailedSubscription(fmt.Errorf("emulator is not a Blockchain"), "invalid emulator type")
	}

	sub := subscription.NewHeightBasedSubscription(
		subscription.DefaultSendBufferSize,
		startBlock.Height,
		a.getTransactionStatusResponse(txID, startBlock.Height, requiredEventEncodingVersion),
	)

	go subscription.NewStreamer(*a.logger, emulatorBlockchain.Broadcaster(), subscription.DefaultSendTimeout, subscription.DefaultResponseLimit, sub).Stream(ctx)

	return sub
}

func (a *AccessAdapter) getTransactionStatusResponse(
	txID flowgo.Identifier,
	startHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.GetDataByHeightFunc {
	lastStatus := flowgo.TransactionStatusUnknown
	
	return func(ctx context.Context, height uint64) (interface{}, error) {
		// Check if block is ready
		if err := a.validateHeight(height, flowgo.BlockStatusSealed); err != nil {
			return nil, err
		}

		// If we've already reached a final status, stop sending updates
		if lastStatus == flowgo.TransactionStatusSealed || lastStatus == flowgo.TransactionStatusExpired {
			return nil, fmt.Errorf("transaction final status %s already reported: %w", lastStatus.String(), subscription.ErrEndOfData)
		}

		// Try to get the transaction result
		txResult, err := a.emulator.GetTransactionResult(txID)
		if err != nil {
			// If transaction is not found and we've exceeded expiry, mark as expired
			if height-startHeight >= 600 { // Default transaction expiry
				lastStatus = flowgo.TransactionStatusExpired
				return []*accessmodel.TransactionResult{{
					Status:        flowgo.TransactionStatusExpired,
					TransactionID: txID,
				}}, nil
			}
			
			// Otherwise, transaction is still pending/unknown
			if lastStatus == flowgo.TransactionStatusUnknown {
				return nil, nil // Don't send duplicate unknown status
			}
			return nil, nil
		}

		// Convert events based on encoding version
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			txResult.Events, err = ConvertCCFEventsToJsonEvents(txResult.Events)
			if err != nil {
				return nil, fmt.Errorf("failed to convert events: %w", err)
			}
		}

		// Determine the current status
		currentStatus := flowgo.TransactionStatusSealed // In emulator, all executed transactions are sealed

		// Generate status updates if status changed
		if currentStatus != lastStatus {
			results := a.generateTransactionStatusUpdates(txResult, lastStatus, currentStatus)
			lastStatus = currentStatus
			return results, nil
		}

		return nil, nil
	}
}

func (a *AccessAdapter) generateTransactionStatusUpdates(
	txResult *accessmodel.TransactionResult,
	prevStatus flowgo.TransactionStatus,
	currentStatus flowgo.TransactionStatus,
) []*accessmodel.TransactionResult {
	if prevStatus == currentStatus {
		return nil
	}

	var results []*accessmodel.TransactionResult

	// Generate intermediate status updates if we skipped any
	// Possible progression: Unknown(0) -> Pending(1) -> Finalized(2) -> Executed(3) -> Sealed(4)
	for status := prevStatus + 1; status <= currentStatus; status++ {
		result := &accessmodel.TransactionResult{
			Status:        status,
			TransactionID: txResult.TransactionID,
		}
		
		// Add block info for finalized and later statuses
		if status >= flowgo.TransactionStatusFinalized {
			result.BlockID = txResult.BlockID
			result.BlockHeight = txResult.BlockHeight
			result.CollectionID = txResult.CollectionID
		}
		
		// Add execution details for executed and sealed statuses
		if status >= flowgo.TransactionStatusExecuted {
			result.Events = txResult.Events
			result.ErrorMessage = txResult.ErrorMessage
		}

		results = append(results, result)
	}

	return results
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

func (a *AccessAdapter) GetScheduledTransaction(_ context.Context, _ uint64) (*flowgo.TransactionBody, error) {
	return nil, nil
}
func (a *AccessAdapter) GetScheduledTransactionResult(_ context.Context, _ uint64, _ entities.EventEncodingVersion) (*accessmodel.TransactionResult, error) {
	return nil, nil
}
