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
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/access"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	emulator "github.com/onflow/flow-emulator"
	convert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/types"
)

// Backend wraps an emulated blockchain and implements the RPC handlers
// required by the Access API.
type Backend struct {
	logger   *zerolog.Logger
	emulator Emulator
	automine bool
}

// SetEmulator hotswaps emulator for state management.
func (b *Backend) SetEmulator(emulator Emulator) {
	b.emulator = emulator
}

// New returns a new backend.
func New(logger *zerolog.Logger, emulator Emulator) *Backend {
	return &Backend{
		logger:   logger,
		emulator: emulator,
		automine: false,
	}
}

func (b *Backend) Ping(ctx context.Context) error {
	return nil
}

func (b *Backend) GetNetworkParameters(ctx context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: flowgo.Emulator,
	}
}

// GetLatestBlockHeader gets the latest sealed block header.
func (b *Backend) GetLatestBlockHeader(
	_ context.Context,
	_ bool,
) (
	*flowgo.Header,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetLatestBlockHeader called")

	// this should always return latest sealed block
	return block.Header, flowgo.BlockStatusSealed, nil
}

// GetBlockHeaderByHeight gets a block header by height.
func (b *Backend) GetBlockHeaderByHeight(
	_ context.Context,
	height uint64,
) (
	*flowgo.Header,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetBlockHeaderByHeight called")

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return block.Header, flowgo.BlockStatusSealed, nil
}

// GetBlockHeaderByID gets a block header by ID.
func (b *Backend) GetBlockHeaderByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*flowgo.Header,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetBlockHeaderByID called")

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return block.Header, flowgo.BlockStatusSealed, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Backend) GetLatestBlock(
	_ context.Context,
	_ bool,
) (
	*flowgo.Block,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetLatestBlock called")

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return block, flowgo.BlockStatusSealed, nil
}

// GetBlockByHeight gets a block by height.
func (b *Backend) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (
	*flowgo.Block,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByHeight(height)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetBlockByHeight called")

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return block, flowgo.BlockStatusSealed, nil
}

// GetBlockByID gets a block by ID.
func (b *Backend) GetBlockByID(
	_ context.Context,
	id sdk.Identifier,
) (
	*flowgo.Block,
	flowgo.BlockStatus,
	error,
) {
	block, err := b.emulator.GetBlockByID(id)
	if err != nil {
		return nil, flowgo.BlockStatusUnknown, status.Error(codes.Internal, err.Error())
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msg("🎁  GetBlockByID called")

	// As we don't fork the chain in emulator, and finalize and seal at the same time, this can only be Sealed
	return block, flowgo.BlockStatusSealed, nil
}

// GetCollectionByID gets a collection by ID.
func (b *Backend) GetCollectionByID(
	_ context.Context,
	id sdk.Identifier,
) (*sdk.Collection, error) {
	col, err := b.emulator.GetCollection(id)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.Debug().
		Str("colID", id.Hex()).
		Msgf("📚  GetCollectionByID called")

	return col, nil
}

// SendTransaction submits a transaction to the network.
func (b *Backend) SendTransaction(ctx context.Context, tx sdk.Transaction) error {
	err := b.emulator.AddTransaction(tx)
	if err != nil {
		switch t := err.(type) {
		case *emulator.DuplicateTransactionError:
			return status.Error(codes.InvalidArgument, err.Error())
		case *types.FlowError:
			switch t.FlowError.Code() {
			case fvmerrors.ErrCodeAccountAuthorizationError,
				fvmerrors.ErrCodeInvalidEnvelopeSignatureError,
				fvmerrors.ErrCodeInvalidPayloadSignatureError,
				fvmerrors.ErrCodeInvalidProposalSignatureError,
				fvmerrors.ErrCodeAccountPublicKeyNotFoundError,
				fvmerrors.ErrCodeInvalidProposalSeqNumberError,
				fvmerrors.ErrCodeInvalidAddressError:

				return status.Error(codes.InvalidArgument, err.Error())

			default:
				if fvmerrors.IsAccountNotFoundError(err) {
					return status.Error(codes.InvalidArgument, err.Error())
				}

				return status.Error(codes.Internal, err.Error())
			}
		default:
			return status.Error(codes.Internal, err.Error())
		}
	} else {
		b.logger.Debug().
			Str("txID", tx.ID().String()).
			Msg(`✉️   Transaction submitted`) //" was messing up vim syntax highlighting
	}

	if b.automine {
		b.CommitBlock()
	}

	return nil
}

// GetTransaction gets a transaction by ID.
func (b *Backend) GetTransaction(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Transaction, error) {
	tx, err := b.emulator.GetTransaction(id)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.Debug().
		Str("txID", id.String()).
		Msg("💵  GetTransaction called")

	return tx, nil
}

// GetTransactionResult gets a transaction by ID.
func (b *Backend) GetTransactionResult(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.TransactionResult, error) {
	result, err := b.emulator.GetTransactionResult(id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.Debug().
		Str("txID", id.String()).
		Msg("📝  GetTransactionResult called")

	return result, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (b *Backend) GetAccount(
	ctx context.Context,
	address sdk.Address,
) (*sdk.Account, error) {
	b.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccount called")

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
	b.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccountAtLatestBlock called")

	account, err := b.getAccount(address)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *Backend) getAccount(address sdk.Address) (*sdk.Account, error) {
	account, err := b.emulator.GetAccount(address)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return account, nil
}

func (b *Backend) GetAccountAtBlockHeight(
	ctx context.Context,
	address sdk.Address,
	height uint64,
) (*sdk.Account, error) {
	b.logger.Debug().
		Stringer("address", address).
		Uint64("height", height).
		Msg("👤  GetAccountAtBlockHeight called")

	account, err := b.emulator.GetAccountAtBlock(address, height)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return account, nil
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block
func (b *Backend) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	b.logger.Debug().Msg("👤  ExecuteScriptAtLatestBlock called")

	block, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
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
	b.logger.Debug().
		Uint64("blockHeight", blockHeight).
		Msg("👤  ExecuteScriptAtBlockHeight called")

	return b.executeScriptAtBlock(script, arguments, blockHeight)
}

// ExecuteScriptAtBlockID executes a script at a specific block ID
func (b *Backend) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID sdk.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	b.logger.Debug().
		Stringer("blockID", blockID).
		Msg("👤  ExecuteScriptAtBlockID called")

	block, err := b.emulator.GetBlockByID(blockID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return b.executeScriptAtBlock(script, arguments, block.Header.Height)
}

// GetEventsForHeightRange returns events matching a query.
func (b *Backend) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]flowgo.BlockEvents, error) {

	err := validateEventType(eventType)
	if err != nil {
		return nil, err
	}

	latestBlock, err := b.emulator.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// if end height is not set, use latest block height
	// if end height is higher than latest, use latest
	if endHeight == 0 || endHeight > latestBlock.Header.Height {
		endHeight = latestBlock.Header.Height
	}

	// check for invalid queries
	if startHeight > endHeight {
		return nil, status.Error(codes.InvalidArgument, "invalid query: start block must be <= end block")
	}

	results := make([]flowgo.BlockEvents, 0)
	eventCount := 0

	for height := startHeight; height <= endHeight; height++ {
		block, err := b.emulator.GetBlockByHeight(height)
		if err != nil {
			switch err.(type) {
			case emulator.NotFoundError:
				return nil, status.Error(codes.NotFound, err.Error())
			default:
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		events, err := b.emulator.GetEventsByHeight(height, eventType)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		flowEvents, err := convert.SDKEventsToFlow(events)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		result := flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         flowEvents,
		}

		results = append(results, result)
		eventCount += len(events)
	}

	b.logger.Debug().Fields(map[string]any{
		"eventType":   eventType,
		"startHeight": startHeight,
		"endHeight":   endHeight,
		"eventCount":  eventCount,
	}).Msg("🎁  GetEventsForHeightRange called")

	return results, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (b *Backend) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []sdk.Identifier,
) ([]flowgo.BlockEvents, error) {

	err := validateEventType(eventType)
	if err != nil {
		return nil, err
	}

	results := make([]flowgo.BlockEvents, 0)
	eventCount := 0

	for _, blockID := range blockIDs {
		block, err := b.emulator.GetBlockByID(blockID)
		if err != nil {
			switch err.(type) {
			case emulator.NotFoundError:
				return nil, status.Error(codes.NotFound, err.Error())
			default:
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		events, err := b.emulator.GetEventsByHeight(block.Header.Height, eventType)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		flowEvents, err := convert.SDKEventsToFlow(events)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		result := flowgo.BlockEvents{
			BlockID:        block.ID(),
			BlockHeight:    block.Header.Height,
			BlockTimestamp: block.Header.Timestamp,
			Events:         flowEvents,
		}

		results = append(results, result)
		eventCount += len(events)
	}

	b.logger.Debug().Fields(map[string]any{
		"eventType":  eventType,
		"eventCount": eventCount,
	}).Msg("🎁  GetEventsForBlockIDs called")

	return results, nil
}

func validateEventType(eventType string) error {
	if len(strings.TrimSpace(eventType)) == 0 {
		return status.Error(codes.InvalidArgument, "invalid query: eventType must not be empty")
	}
	return nil
}

// CommitBlock executes the current pending transactions and commits the results in a new block.
func (b *Backend) CommitBlock() {
	block, results, err := b.emulator.ExecuteAndCommitBlock()
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to commit block")
		return
	}

	for _, result := range results {
		printTransactionResult(b.logger, result)
	}

	blockID := block.ID()

	b.logger.Debug().Fields(map[string]any{
		"blockHeight": block.Header.Height,
		"blockID":     hex.EncodeToString(blockID[:]),
	}).Msgf("📦  Block #%d committed", block.Header.Height)
}

// executeScriptAtBlock is a helper for executing a script at a specific block
func (b *Backend) executeScriptAtBlock(script []byte, arguments [][]byte, blockHeight uint64) ([]byte, error) {
	result, err := b.emulator.ExecuteScriptAtBlock(script, arguments, blockHeight)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	printScriptResult(b.logger, result)

	if !result.Succeeded() {
		return nil, result.Error
	}

	valueBytes, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return valueBytes, nil
}

func (b *Backend) GetAccountStorage(address sdk.Address) (*emulator.AccountStorage, error) {
	b.logger.Debug().
		Stringer("address", address).
		Msg("👤  GetAccountStorage called")

	return b.emulator.GetAccountStorage(address)
}

func (b *Backend) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	panic("implement me")
}

func (b *Backend) GetExecutionResultForBlockID(_ context.Context, _ flowgo.Identifier) (*flowgo.ExecutionResult, error) {
	return nil, nil
}

// EnableAutoMine enables the automine flag.
func (b *Backend) EnableAutoMine() {
	b.automine = true
}

// DisableAutoMine disables the automine flag.
func (b *Backend) DisableAutoMine() {
	b.automine = false
}

func (b *Backend) GetTransactionResultByIndex(context.Context, flowgo.Identifier, uint32) (*access.TransactionResult, error) {
	// TODO: implement
	panic("GetTransactionResultByIndex not implemented")
}

func (b *Backend) GetTransactionsByBlockID(ctx context.Context, id flowgo.Identifier) ([]*flowgo.TransactionBody, error) {
	// TODO: implement
	panic("GetTransactionsByBlockID not implemented")
}

func (b *Backend) GetTransactionResultsByBlockID(ctx context.Context, id flowgo.Identifier) ([]*access.TransactionResult, error) {
	// TODO: implement
	panic("GetTransactionResultsByBlockID not implemented")
}

func printTransactionResult(logger *zerolog.Logger, result *types.TransactionResult) {
	if result.Succeeded() {
		logger.Info().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("⭐  Transaction executed")
	} else {
		logger.Warn().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("❗  Transaction reverted")
	}

	for _, log := range result.Logs {
		logger.Info().Msgf(
			"%s %s",
			logPrefix("LOG", result.TransactionID, aurora.BlueFg),
			log,
		)
	}

	for _, event := range result.Events {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("EVT", result.TransactionID, aurora.GreenFg),
			event,
		)
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", result.TransactionID, aurora.RedFg),
			result.Error.Error(),
		)

		if result.Debug != nil {
			logger.Debug().Fields(result.Debug.Meta).Msgf("%s %s", "❗  Transaction Signature Error", result.Debug.Message)
		}
	}
}

func printScriptResult(logger *zerolog.Logger, result *types.ScriptResult) {
	if result.Succeeded() {
		logger.Info().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("⭐  Script executed")
	} else {
		logger.Warn().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Msg("❗  Script reverted")
	}

	for _, log := range result.Logs {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("LOG", result.ScriptID, aurora.BlueFg),
			log,
		)
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", result.ScriptID, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func logPrefix(prefix string, id sdk.Identifier, color aurora.Color) string {
	prefix = aurora.Colorize(prefix, color|aurora.BoldFm).String()
	shortID := fmt.Sprintf("[%s]", id.String()[:6])
	shortID = aurora.Colorize(shortID, aurora.FaintFm).String()
	return fmt.Sprintf("%s %s", prefix, shortID)
}
