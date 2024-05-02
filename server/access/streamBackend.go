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

package access

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
)

type StateStreamBackend struct {
	blockchain *emulator.Blockchain

	log            zerolog.Logger
	sendTimeout    time.Duration
	responseLimit  float64
	sendBufferSize int

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

func NewStateStreamBackend(blockchain *emulator.Blockchain, log zerolog.Logger) *StateStreamBackend {
	return &StateStreamBackend{
		blockchain:       blockchain,
		log:              log,
		sendTimeout:      subscription.DefaultSendTimeout,
		responseLimit:    subscription.DefaultResponseLimit,
		sendBufferSize:   subscription.DefaultSendBufferSize,
		getExecutionData: getExecutionDataFunc(blockchain),
		getStartHeight:   getStartHeightFunc(blockchain),
	}
}

var _ state_stream.API = &StateStreamBackend{}

func (b *StateStreamBackend) SubscribeEventsFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	filter state_stream.EventFilter,
) subscription.Subscription {
	return b.SubscribeEvents(ctx, startBlockID, 0, filter)
}

func (b *StateStreamBackend) SubscribeEventsFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	filter state_stream.EventFilter,
) subscription.Subscription {
	return b.SubscribeEvents(ctx, flow.ZeroID, startHeight, filter)
}

func (b *StateStreamBackend) SubscribeEventsFromLatest(
	ctx context.Context,
	filter state_stream.EventFilter,
) subscription.Subscription {
	return b.SubscribeEvents(ctx, flow.ZeroID, 0, filter)
}

func getStartHeightFunc(blockchain *emulator.Blockchain) GetStartHeightFunc {
	return func(blockID flow.Identifier, height uint64) (uint64, error) {

		//check latest block request
		if blockID != flow.ZeroID && height == 0 {
			block, err := blockchain.GetLatestBlock()
			if err != nil {
				return 0, storage.ErrNotFound
			}
			return block.Header.Height, nil
		}

		// try with start at blockID
		if blockID != flow.ZeroID {
			block, err := blockchain.GetBlockByID(blockID)
			if err != nil {
				return 0, storage.ErrNotFound
			}
			return block.Header.Height, nil
		}

		// try with start at blockHeight
		_, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			return 0, storage.ErrNotFound
		}

		return height, nil
	}
}

func getExecutionDataFunc(blockchain *emulator.Blockchain) GetExecutionDataFunc {
	return func(_ context.Context, height uint64) (*execution_data.BlockExecutionDataEntity, error) {
		block, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			var blockNotFoundByHeightError *types.BlockNotFoundByHeightError
			if errors.As(err, &blockNotFoundByHeightError) {
				return nil, errors.Join(err, subscription.ErrBlockNotReady)
			}
			return nil, err
		}

		chunks := make([]*execution_data.ChunkExecutionData, len(block.Payload.Guarantees))

		for i, collectionGuarantee := range block.Payload.Guarantees {
			lightCollection, err := blockchain.GetCollectionByID(collectionGuarantee.CollectionID)
			if err != nil {
				return nil, err
			}

			collection := &flow.Collection{}
			var events []flow.Event
			var txResults []flow.LightTransactionResult

			for _, transactionId := range lightCollection.Transactions {
				tx, err := blockchain.GetTransaction(transactionId)
				if err != nil {
					return nil, err
				}
				collection.Transactions = append(collection.Transactions, tx)

				txResult, err := blockchain.GetTransactionResult(transactionId)
				if err != nil {
					return nil, err
				}
				events = append(events, txResult.Events...)

				lightResult := flow.LightTransactionResult{
					TransactionID: txResult.TransactionID,
					Failed:        txResult.ErrorMessage != "",
				}
				txResults = append(txResults, lightResult)
			}

			chunk := &execution_data.ChunkExecutionData{
				Collection: collection,
				Events:     events,
				//TODO: add trie updates
				TransactionResults: txResults,
			}
			chunks[i] = chunk
		}

		executionData := &execution_data.BlockExecutionData{
			BlockID:             block.ID(),
			ChunkExecutionDatas: chunks,
		}

		result := execution_data.NewBlockExecutionDataEntity(
			flow.ZeroID,
			executionData,
		)
		return result, nil
	}
}

func (b *StateStreamBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error) {
	block, err := b.blockchain.GetBlockByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block header for %s: %w", blockID, err)
	}

	executionData, err := b.getExecutionData(ctx, block.Header.Height)

	if err != nil {
		// need custom not found handler due to blob not found error
		if errors.Is(err, storage.ErrNotFound) || execution_data.IsBlobNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "could not find execution data: %v", err)
		}

		return nil, rpc.ConvertError(err, "could not get execution data", codes.Internal)
	}

	return executionData.BlockExecutionData, nil
}

func (b *StateStreamBackend) SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) subscription.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getResponse)

	go subscription.NewStreamer(b.log, b.blockchain.Broadcaster(), b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b *StateStreamBackend) getResponse(ctx context.Context, height uint64) (interface{}, error) {
	executionData, err := b.getExecutionData(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
	}

	return &backend.ExecutionDataResponse{
		Height:        height,
		ExecutionData: executionData.BlockExecutionData,
	}, nil
}

type GetExecutionDataFunc func(context.Context, uint64) (*execution_data.BlockExecutionDataEntity, error)

type GetStartHeightFunc func(flow.Identifier, uint64) (uint64, error)

func (b *StateStreamBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.EventFilter) subscription.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getEventsFactory(filter))

	go subscription.NewStreamer(b.log, b.blockchain.Broadcaster(), b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

func (b *StateStreamBackend) getEventsFactory(filter state_stream.EventFilter) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		executionData, err := b.getExecutionData(ctx, height)
		if err != nil {
			return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		b.log.Trace().
			Hex("block_id", logging.ID(executionData.BlockID)).
			Uint64("height", height).
			Msgf("sending %d events", len(events))

		return &backend.EventsResponse{
			BlockID: executionData.BlockID,
			Height:  height,
			Events:  events,
		}, nil
	}
}

func (b *StateStreamBackend) SubscribeAccountStatus(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter state_stream.AccountStatusFilter) subscription.Subscription {
	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}
	sub := subscription.NewHeightBasedSubscription(b.sendBufferSize, nextHeight, b.getAccountStatusFactory(filter))
	go subscription.NewStreamer(b.log, b.blockchain.Broadcaster(), b.sendTimeout, b.responseLimit, sub).Stream(ctx)
	return sub
}

func (b *StateStreamBackend) SubscribeAccountStatusesFromStartBlockID(
	ctx context.Context,
	startBlockID flow.Identifier,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	return b.SubscribeAccountStatus(ctx, startBlockID, 0, filter)
}

func (b *StateStreamBackend) SubscribeAccountStatusesFromStartHeight(
	ctx context.Context,
	startHeight uint64,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	return b.SubscribeAccountStatus(ctx, flow.ZeroID, startHeight, filter)
}

func (b *StateStreamBackend) SubscribeAccountStatusesFromLatestBlock(
	ctx context.Context,
	filter state_stream.AccountStatusFilter,
) subscription.Subscription {
	return b.SubscribeAccountStatus(ctx, flow.ZeroID, 0, filter)
}

func (b *StateStreamBackend) getAccountStatusFactory(filter state_stream.AccountStatusFilter) subscription.GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {

		executionData, err := b.getExecutionData(ctx, height)
		if err != nil {
			return nil, fmt.Errorf("could not get execution data for block %d: %w", height, err)
		}

		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, storage.ErrHeightNotIndexed) {
				return nil, fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
			}
			return nil, err
		}

		filteredProtocolEvents := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			filteredProtocolEvents = append(filteredProtocolEvents, filter.Filter(chunkExecutionData.Events)...)
		}

		allAccountProtocolEvents := filter.GroupCoreEventsByAccountAddress(filteredProtocolEvents, b.log)

		return &backend.AccountStatusesResponse{
			BlockID:       executionData.BlockID,
			Height:        height,
			AccountEvents: allAccountProtocolEvents,
		}, nil

	}
}

func (b *StateStreamBackend) GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}
