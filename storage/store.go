// Package storage defines the interface and implementations for interacting with
// persistent chain state.
package storage

import (
	"github.com/dapperlabs/flow-go-sdk"

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

	// BlockByID returns the block with the given ID.
	BlockByID(flow.Identifier) (types.Block, error)

	// BlockByHeight returns the block with the given height.
	BlockByHeight(blockHeight uint64) (types.Block, error)

	// LatestBlock returns the block with the highest block height.
	LatestBlock() (types.Block, error)

	// InsertBlock inserts a block.
	InsertBlock(types.Block) error

	// CommitBlock atomically saves the execution results for a block.
	CommitBlock(
		block types.Block,
		transactions []flow.Transaction,
		delta types.LedgerDelta,
		events []flow.Event,
	) error

	// TransactionByID gets the transaction with the given ID.
	TransactionByID(flow.Identifier) (flow.Transaction, error)

	// InsertTransaction inserts a transaction.
	InsertTransaction(flow.Transaction) error

	// LedgerViewByHeight returns a view into the ledger state at a given block.
	LedgerViewByHeight(blockHeight uint64) *types.LedgerView

	// InsertLedgerDelta inserts a register delta at a given block.
	InsertLedgerDelta(blockHeight uint64, delta types.LedgerDelta) error

	// RetrieveEvents returns all events with the given type between startBlock and
	// endBlock (inclusive). If eventType is empty, returns all events in the
	// range, regardless of type.
	RetrieveEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error)

	// InsertEvents inserts events for a block.
	InsertEvents(blockHeight uint64, events []flow.Event) error
}
