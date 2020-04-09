package emulator

import (
	"github.com/dapperlabs/flow-go-sdk"

	"github.com/dapperlabs/flow-emulator/types"
)

// A pendingBlock contains the pending state required to form a new block.
type pendingBlock struct {
	// block information (Height, ParentID, TransactionIDs)
	block *types.Block
	// mapping from transaction ID to transaction
	transactions map[flow.Identifier]*flow.Transaction
	// mapping from transaction ID to transaction result
	transactionResults map[flow.Identifier]TransactionResult
	// current working ledger, updated after each transaction execution
	ledgerView *types.LedgerView
	// events emitted during execution
	events []flow.Event
	// index of transaction execution
	index int
}

type executionResult struct {
	Transaction flow.Transaction
	Result      TransactionResult
}

// newPendingBlock creates a new pending block sequentially after a specified block.
func newPendingBlock(prevBlock types.Block, ledgerView *types.LedgerView) *pendingBlock {
	transactions := make(map[flow.Identifier]*flow.Transaction)
	transactionResults := make(map[flow.Identifier]TransactionResult)

	transactionIDs := make([]flow.Identifier, 0)

	block := &types.Block{
		Height:         prevBlock.Height + 1,
		ParentID:       prevBlock.ID(),
		TransactionIDs: transactionIDs,
	}

	return &pendingBlock{
		block:              block,
		transactions:       transactions,
		transactionResults: transactionResults,
		ledgerView:         ledgerView,
		events:             make([]flow.Event, 0),
		index:              0,
	}
}

// ID returns the ID of the pending block.
func (b *pendingBlock) ID() flow.Identifier {
	return b.block.ID()
}

// Height returns the number of the pending block.
func (b *pendingBlock) Height() uint64 {
	return b.block.Height
}

// Block returns the block information for the pending block.
func (b *pendingBlock) Block() types.Block {
	return *b.block
}

// LedgerDelta returns the ledger delta for the pending block.
func (b *pendingBlock) LedgerDelta() types.LedgerDelta {
	return b.ledgerView.Delta()
}

// AddTransaction adds a transaction to the pending block.
func (b *pendingBlock) AddTransaction(tx flow.Transaction) {
	b.block.TransactionIDs = append(b.block.TransactionIDs, tx.ID())
	b.transactions[tx.ID()] = &tx
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *pendingBlock) ContainsTransaction(txID flow.Identifier) bool {
	_, exists := b.transactions[txID]
	return exists
}

// GetTransaction retrieves a transaction in the pending block by ID.
func (b *pendingBlock) GetTransaction(txID flow.Identifier) *flow.Transaction {
	return b.transactions[txID]
}

// nextTransaction returns the next indexed transaction.
func (b *pendingBlock) nextTransaction() *flow.Transaction {
	txID := b.block.TransactionIDs[b.index]
	return b.GetTransaction(txID)
}

// ExecutionResults returns the transaction execution results from the pending block.
func (b *pendingBlock) ExecutionResults() []executionResult {
	blockResults := make([]executionResult, len(b.block.TransactionIDs))

	for i, txID := range b.block.TransactionIDs {
		transaction := b.transactions[txID]
		result := b.transactionResults[txID]

		blockResults[i] = executionResult{
			Transaction: *transaction,
			Result:      result,
		}
	}

	return blockResults
}

// ExecuteNextTransaction executes the next transaction in the pending block.
//
// This function uses the provided execute function to perform the actual
// execution, then updates the pending block with the output.
func (b *pendingBlock) ExecuteNextTransaction(
	execute func(ledgerView *types.LedgerView, tx flow.Transaction) (TransactionResult, error),
) (TransactionResult, error) {
	tx := b.nextTransaction()

	result, err := execute(b.ledgerView, *tx)
	if err != nil {
		// fail fast if fatal error occurs
		return TransactionResult{}, err
	}

	// increment transaction index even if transaction reverts
	b.index++

	if result.Succeeded() {
		b.events = append(b.events, result.Events...)
	}

	b.transactionResults[tx.ID()] = result

	return result, nil
}

// Events returns all events captured during the execution of the pending block.
func (b *pendingBlock) Events() []flow.Event {
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
	return len(b.block.TransactionIDs)
}

// Empty returns true if the pending block is empty.
func (b *pendingBlock) Empty() bool {
	return b.Size() == 0
}
