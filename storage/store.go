// Package storage defines the interface and implementations for interacting with
// persistent chain state.
package storage

import (
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	model "github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-emulator/types"
)

// Store defines the storage layer for persistent chain state.
//
// This includes finalized blocks and transactions, and the resultant register
// states and emitted events. It does not include pending state, such as pending
// transactions and register states.
//
// Implementations must distinguish between not found errors and errors with
// the underlying storage by returning an instance of store.ErrNotFound if a
// resource cannot be found.
//
// Implementations must be safe for use by multiple goroutines.
type Store interface {

	// LatestBlock returns the block with the highest block height.
	LatestBlock() (model.Block, error)

	// BlockByID returns the block with the given ID.
	BlockByID(model.Identifier) (model.Block, error)

	// BlockByHeight returns the block with the given height.
	BlockByHeight(blockHeight uint64) (model.Block, error)

	// CommitBlock atomically saves the execution results for a block.
	CommitBlock(
		block model.Block,
		collections []*model.LightCollection,
		transactions map[model.Identifier]*model.TransactionBody,
		transactionResults map[model.Identifier]*types.StorableTransactionResult,
		delta delta.Delta,
		events []model.Event,
	) error

	// CollectionByID gets the collection (transaction IDs only) with the given ID.
	CollectionByID(model.Identifier) (model.LightCollection, error)

	// TransactionByID gets the transaction with the given ID.
	TransactionByID(model.Identifier) (model.TransactionBody, error)

	// TransactionResultByID gets the transaction result with the given ID.
	TransactionResultByID(model.Identifier) (types.StorableTransactionResult, error)

	// LedgerViewByHeight returns a view into the ledger state at a given block.
	LedgerViewByHeight(blockHeight uint64) *delta.View

	// EventsByHeight returns the events in the block at the given height, optionally filtered by type.
	EventsByHeight(blockHeight uint64, eventType string) ([]model.Event, error)
}
