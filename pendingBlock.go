package emulator

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	model "github.com/dapperlabs/flow-go/model/flow"
	"github.com/onflow/flow-go-sdk"
)

type IndexedTransactionResult struct {
	TransactionResult *virtualmachine.TransactionResult
	Index             uint32
}

// A pendingBlock contains the pending state required to form a new block.
type pendingBlock struct {
	height   uint64
	parentID model.Identifier
	// mapping from transaction ID to transaction
	transactions map[model.Identifier]*model.TransactionBody
	// list of transaction IDs in the block
	transactionIDs []model.Identifier
	// mapping from transaction ID to transaction result
	transactionResults map[model.Identifier]IndexedTransactionResult
	// current working ledger, updated after each transaction execution
	ledgerView *delta.View
	// events emitted during execution
	events []model.Event
	// index of transaction execution
	index int
}

type executionResult struct {
	Transaction flow.Transaction
	Result      *virtualmachine.TransactionResult
}

// newPendingBlock creates a new pending block sequentially after a specified block.
func newPendingBlock(prevBlock *model.Block, ledgerView *delta.View) *pendingBlock {

	return &pendingBlock{
		height:             prevBlock.Header.Height + 1,
		parentID:           prevBlock.ID(),
		transactions:       make(map[model.Identifier]*model.TransactionBody),
		transactionIDs:     make([]model.Identifier, 0),
		transactionResults: make(map[model.Identifier]IndexedTransactionResult),
		ledgerView:         ledgerView,
		events:             make([]model.Event, 0),
		index:              0,
	}
}

// ID returns the ID of the pending block.
func (b *pendingBlock) ID() model.Identifier {
	return b.Block().ID()
}

// Height returns the number of the pending block.
func (b *pendingBlock) Height() uint64 {
	return b.height
}

// Block returns the block information for the pending block.
func (b *pendingBlock) Block() *model.Block {
	collections := b.Collections()

	guarantees := make([]*model.CollectionGuarantee, len(collections))
	for i, collection := range collections {
		guarantees[i] = &model.CollectionGuarantee{
			CollectionID: collection.ID(),
		}
	}

	return &model.Block{
		Header: &model.Header{
			Height:   b.height,
			ParentID: b.parentID,
		},
		Payload: &model.Payload{
			Guarantees: guarantees,
		},
	}
}

func (b *pendingBlock) Collections() []*model.LightCollection {
	if len(b.transactionIDs) == 0 {
		return []*model.LightCollection{}
	}

	transactionIDs := make([]model.Identifier, len(b.transactionIDs))

	// TODO: remove once SDK models are removed
	for i, transactionID := range b.transactionIDs {
		transactionIDs[i] = model.Identifier(transactionID)
	}

	collection := model.LightCollection{Transactions: transactionIDs}

	return []*model.LightCollection{&collection}
}

func (b *pendingBlock) Transactions() map[model.Identifier]*model.TransactionBody {
	return b.transactions
}

func (b *pendingBlock) TransactionResults() map[model.Identifier]IndexedTransactionResult {
	return b.transactionResults
}

// LedgerDelta returns the ledger delta for the pending block.
func (b *pendingBlock) LedgerDelta() delta.Delta {
	return b.ledgerView.Delta()
}

// AddTransaction adds a transaction to the pending block.
func (b *pendingBlock) AddTransaction(tx model.TransactionBody) {

	fmt.Printf("adding pending block tx %s\n", tx.ID())

	b.transactionIDs = append(b.transactionIDs, tx.ID())
	b.transactions[tx.ID()] = &tx
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *pendingBlock) ContainsTransaction(txID model.Identifier) bool {
	_, exists := b.transactions[txID]
	return exists
}

// GetTransaction retrieves a transaction in the pending block by ID.
func (b *pendingBlock) GetTransaction(txID model.Identifier) *model.TransactionBody {
	return b.transactions[txID]
}

// nextTransaction returns the next indexed transaction.
func (b *pendingBlock) nextTransaction() *model.TransactionBody {
	txID := b.transactionIDs[b.index]
	return b.GetTransaction(txID)
}

// ExecuteNextTransaction executes the next transaction in the pending block.
//
// This function uses the provided execute function to perform the actual
// execution, then updates the pending block with the output.
func (b *pendingBlock) ExecuteNextTransaction(
	execute func(ledgerView *delta.View, tx *model.TransactionBody) (*virtualmachine.TransactionResult, error),
) (*virtualmachine.TransactionResult, error) {
	tx := b.nextTransaction()

	childView := b.ledgerView.NewChild()

	result, err := execute(childView, tx)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	// increment transaction index even if transaction reverts
	b.index++

	convertedEvents, err := virtualmachine.ConvertEvents(uint32(b.index), result)

	if result.Succeeded() {
		b.events = append(b.events, convertedEvents...)
		b.ledgerView.MergeView(childView)
	}

	b.transactionResults[tx.ID()] = IndexedTransactionResult{
		TransactionResult: result,
		Index:             uint32(b.index),
	}

	return result, nil
}

// Events returns all events captured during the execution of the pending block.
func (b *pendingBlock) Events() []model.Event {
	return b.events
}

// ExecutionStarted returns true if the pending block has started executing.
func (b *pendingBlock) ExecutionStarted() bool {
	return b.index > 0
}

// ExecutionComplete returns true if the pending block is fully executed.
func (b *pendingBlock) ExecutionComplete() bool {
	return b.index >= b.Size()
}

// Size returns the number of transactions in the pending block.
func (b *pendingBlock) Size() int {
	return len(b.transactionIDs)
}

// Empty returns true if the pending block is empty.
func (b *pendingBlock) Empty() bool {
	return b.Size() == 0
}
