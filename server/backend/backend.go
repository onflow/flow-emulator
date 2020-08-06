package backend

import (
	"context"
	"fmt"

	"github.com/dapperlabs/flow-go/fvm"
	"github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"

	emulator "github.com/dapperlabs/flow-emulator"
	"github.com/dapperlabs/flow-emulator/types"
)

// Backend wraps an emulated blockchain and implements the RPC handlers
// required by the Access API.
type Backend struct {
	logger     *logrus.Logger
	blockchain emulator.BlockchainAPI
	automine   bool
}

// New returns a new backend.
func New(logger *logrus.Logger, blockchain emulator.BlockchainAPI) *Backend {
	return &Backend{
		logger:     logger,
		blockchain: blockchain,
		automine:   false,
	}
}

// SendTransaction submits a transaction to the network.
func (b *Backend) SendTransaction(ctx context.Context, tx sdk.Transaction) error {
	err := b.blockchain.AddTransaction(tx)
	if err != nil {
		switch t := err.(type) {
		case *emulator.DuplicateTransactionError:
			return status.Error(codes.InvalidArgument, err.Error())
		case *types.FlowError:
			switch t.FlowError.(type) {
			case *fvm.InvalidSignaturePublicKeyError:
				return status.Error(codes.InvalidArgument, err.Error())
			case *fvm.InvalidSignatureAccountError:
				return status.Error(codes.InvalidArgument, err.Error())
			default:
				return status.Error(codes.Internal, err.Error())
			}
		default:
			return status.Error(codes.Internal, err.Error())
		}
	} else {
		b.logger.
			WithField("txID", tx.ID().String()).
			Debug("️✉️   Transaction submitted")
	}

	if b.automine {
		b.CommitBlock()
	}

	return nil
}

// GetLatestBlockHeader gets the latest sealed block header.
func (b *Backend) GetLatestBlockHeader(ctx context.Context) (*sdk.Block, error) {
	block, err := b.blockchain.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetLatestBlockHeader called")

	return block, nil
}

// GetBlockHeaderByHeight gets a block header by height.
func (b *Backend) GetBlockHeaderByHeight(
	ctx context.Context,
	height uint64,
) (*sdk.Block, error) {
	block, err := b.blockchain.GetBlockByHeight(height)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetBlockHeaderByHeight called")

	return block, nil
}

// GetBlockHeaderByID gets a block header by ID.
func (b *Backend) GetBlockHeaderByID(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Block, error) {
	block, err := b.blockchain.GetBlockByID(id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetBlockHeaderByID called")

	return block, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Backend) GetLatestBlock(ctx context.Context) (*sdk.Block, error) {
	block, err := b.blockchain.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetLatestBlock called")

	return block, nil
}

// GetBlockByHeight gets a block by height.
func (b *Backend) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (*sdk.Block, error) {
	block, err := b.blockchain.GetBlockByHeight(height)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetBlockByHeight called")

	return block, nil
}

// GetBlockByHeight gets a block by ID.
func (b *Backend) GetBlockByID(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Block, error) {
	block, err := b.blockchain.GetBlockByID(id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debug("🎁  GetBlockByID called")

	return block, nil
}

// GetCollectionByID gets a collection by ID.
func (b *Backend) GetCollectionByID(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Collection, error) {
	col, err := b.blockchain.GetCollection(id)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.
		WithField("colID", id.Hex()).
		Debugf("📚  GetCollectionByID called")

	return col, nil
}

// GetTransaction gets a transaction by ID.
func (b *Backend) GetTransaction(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.Transaction, error) {
	tx, err := b.blockchain.GetTransaction(id)
	if err != nil {
		switch err.(type) {
		case emulator.NotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.
		WithField("txID", id.String()).
		Debugf("💵  GetTransaction called")

	return tx, nil
}

// GetTransactionResult gets a transaction by ID.
func (b *Backend) GetTransactionResult(
	ctx context.Context,
	id sdk.Identifier,
) (*sdk.TransactionResult, error) {
	result, err := b.blockchain.GetTransactionResult(id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.
		WithField("txID", id.String()).
		Debugf("📝  GetTransactionResult called")

	return result, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (b *Backend) GetAccount(
	ctx context.Context,
	address sdk.Address,
) (*sdk.Account, error) {
	b.logger.
		WithField("address", address).
		Debugf("👤  GetAccount called")

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
	b.logger.
		WithField("address", address).
		Debugf("👤  GetAccountAtLatestBlock called")

	account, err := b.getAccount(address)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *Backend) getAccount(address sdk.Address) (*sdk.Account, error) {
	account, err := b.blockchain.GetAccount(address)
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
	panic("implement me")
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block
func (b *Backend) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	b.logger.Debugf("👤  ExecuteScriptAtLatestBlock called")

	block, err := b.blockchain.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return b.executeScriptAtBlock(script, arguments, block.Height)
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height
func (b *Backend) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	blockHeight uint64,
) ([]byte, error) {
	b.logger.
		WithField("blockHeight", blockHeight).
		Debugf("👤  ExecuteScriptAtBlockHeight called")

	return b.executeScriptAtBlock(script, arguments, blockHeight)
}

// ExecuteScriptAtBlockID executes a script at a specific block ID
func (b *Backend) ExecuteScriptAtBlockID(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	blockID sdk.Identifier,
) ([]byte, error) {
	b.logger.
		WithField("blockID", blockID).
		Debugf("👤  ExecuteScriptAtBlockID called")

	block, err := b.blockchain.GetBlockByID(blockID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return b.executeScriptAtBlock(script, arguments, block.Height)
}

type BlockEvents struct {
	Block  *sdk.Block
	Events []sdk.Event
}

// GetEventsForHeightRange returns events matching a query.
func (b *Backend) GetEventsForHeightRange(
	ctx context.Context,
	eventType string,
	startHeight, endHeight uint64,
) ([]BlockEvents, error) {
	latestBlock, err := b.blockchain.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// if end height is not set, use latest block height
	// if end height is higher than latest, use latest
	if endHeight == 0 || endHeight > latestBlock.Height {
		endHeight = latestBlock.Height
	}

	// check for invalid queries
	if startHeight > endHeight {
		return nil, status.Error(codes.InvalidArgument, "invalid query: start block must be <= end block")
	}

	results := make([]BlockEvents, 0)
	eventCount := 0

	for height := startHeight; height <= endHeight; height++ {
		block, err := b.blockchain.GetBlockByHeight(height)
		if err != nil {
			switch err.(type) {
			case emulator.NotFoundError:
				return nil, status.Error(codes.NotFound, err.Error())
			default:
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		events, err := b.blockchain.GetEventsByHeight(height, eventType)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		result := BlockEvents{
			Block:  block,
			Events: events,
		}

		results = append(results, result)
		eventCount += len(events)
	}

	b.logger.WithFields(logrus.Fields{
		"eventType":   eventType,
		"startHeight": startHeight,
		"endHeight":   endHeight,
		"eventCount":  eventCount,
	}).Debugf("🎁  GetEventsForHeightRange called")

	return results, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (b *Backend) GetEventsForBlockIDs(
	ctx context.Context,
	eventType string,
	blockIDs []sdk.Identifier,
) ([]BlockEvents, error) {
	results := make([]BlockEvents, 0)
	eventCount := 0

	for _, blockID := range blockIDs {
		block, err := b.blockchain.GetBlockByID(blockID)
		if err != nil {
			switch err.(type) {
			case emulator.NotFoundError:
				return nil, status.Error(codes.NotFound, err.Error())
			default:
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		events, err := b.blockchain.GetEventsByHeight(block.Height, eventType)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		result := BlockEvents{
			Block:  block,
			Events: events,
		}

		results = append(results, result)
		eventCount += len(events)
	}

	b.logger.WithFields(logrus.Fields{
		"eventType":  eventType,
		"eventCount": eventCount,
	}).Debugf("🎁  GetEventsForBlockIDs called")

	return results, nil
}

// CommitBlock executes the current pending transactions and commits the results in a new block.
func (b *Backend) CommitBlock() {
	block, results, err := b.blockchain.ExecuteAndCommitBlock()
	if err != nil {
		b.logger.WithError(err).Error("Failed to commit block")
		return
	}

	for _, result := range results {
		printTransactionResult(b.logger, result)
	}

	b.logger.WithFields(logrus.Fields{
		"blockHeight": block.Height,
		"blockID":     block.ID.Hex(),
	}).Debugf("📦  Block #%d committed", block.Height)
}

// executeScriptAtBlock is a helper for executing a script at a specific block
func (b *Backend) executeScriptAtBlock(script []byte, arguments [][]byte, blockHeight uint64) ([]byte, error) {
	result, err := b.blockchain.ExecuteScriptAtBlock(script, arguments, blockHeight)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	printScriptResult(b.logger, result)

	valueBytes, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return valueBytes, nil
}

// EnableAutoMine enables the automine flag.
func (b *Backend) EnableAutoMine() {
	b.automine = true
}

// DisableAutoMine disables the automine flag.
func (b *Backend) DisableAutoMine() {
	b.automine = false
}

func printTransactionResult(logger *logrus.Logger, result *types.TransactionResult) {
	if result.Succeeded() {
		logger.
			WithField("txID", result.TransactionID.String()).
			Info("⭐  Transaction executed")
	} else {
		logger.
			WithField("txID", result.TransactionID.String()).
			Warn("❗  Transaction reverted")
	}

	for _, log := range result.Logs {
		logger.Debugf(
			"%s %s",
			logPrefix("LOG", result.TransactionID, aurora.BlueFg),
			log,
		)
	}

	for _, event := range result.Events {
		logger.Debugf(
			"%s %s",
			logPrefix("EVT", result.TransactionID, aurora.GreenFg),
			event,
		)
	}

	if !result.Succeeded() {
		logger.Warnf(
			"%s %s",
			logPrefix("ERR", result.TransactionID, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func printScriptResult(logger *logrus.Logger, result *types.ScriptResult) {
	if result.Succeeded() {
		logger.
			WithField("scriptID", result.ScriptID.String()).
			Info("⭐  Script executed")
	} else {
		logger.
			WithField("scriptID", result.ScriptID.String()).
			Warn("❗  Script reverted")
	}

	for _, log := range result.Logs {
		logger.Debugf(
			"%s %s",
			logPrefix("LOG", result.ScriptID, aurora.BlueFg),
			log,
		)
	}

	if !result.Succeeded() {
		logger.Warnf(
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
