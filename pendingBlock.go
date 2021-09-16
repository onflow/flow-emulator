package emulator

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	flowgo "github.com/onflow/flow-go/model/flow"
)

type IndexedTransactionResult struct {
	Transaction *fvm.TransactionProcedure
	Index       uint32
}

// MaxViewIncrease represents the largest difference in view number between
// two consecutive blocks. The minimum view increment is 1.
const MaxViewIncrease = 3

// A pendingBlock contains the pending state required to form a new block.
type pendingBlock struct {
	height    uint64
	view      uint64
	parentID  flowgo.Identifier
	timestamp time.Time
	// mapping from transaction ID to transaction
	transactions map[flowgo.Identifier]*flowgo.TransactionBody
	// list of transaction IDs in the block
	transactionIDs []flowgo.Identifier
	// mapping from transaction ID to transaction result
	transactionResults map[flowgo.Identifier]IndexedTransactionResult
	// current working ledger, updated after each transaction execution
	ledgerView *delta.View
	// events emitted during execution
	events []flowgo.Event
	// index of transaction execution
	index uint32
}

// newPendingBlock creates a new pending block sequentially after a specified block.
func newPendingBlock(prevBlock *flowgo.Block, ledgerView *delta.View) *pendingBlock {

	return &pendingBlock{
		height: prevBlock.Header.Height + 1,
		// the view increments by between 1 and MaxViewIncrease to match
		// behaviour on a real network, where views are not consecutive
		view:               prevBlock.Header.View + uint64(rand.Intn(MaxViewIncrease)+1),
		parentID:           prevBlock.ID(),
		timestamp:          time.Now().UTC(),
		transactions:       make(map[flowgo.Identifier]*flowgo.TransactionBody),
		transactionIDs:     make([]flowgo.Identifier, 0),
		transactionResults: make(map[flowgo.Identifier]IndexedTransactionResult),
		ledgerView:         ledgerView,
		events:             make([]flowgo.Event, 0),
		index:              0,
	}
}

// ID returns the ID of the pending block.
func (b *pendingBlock) ID() flowgo.Identifier {
	return b.Block().ID()
}

// Height returns the number of the pending block.
func (b *pendingBlock) Height() uint64 {
	return b.height
}

// Block returns the block information for the pending block.
func (b *pendingBlock) Block() *flowgo.Block {
	collections := b.Collections()

	guarantees := make([]*flowgo.CollectionGuarantee, len(collections))
	for i, collection := range collections {
		guarantees[i] = &flowgo.CollectionGuarantee{
			CollectionID: collection.ID(),
		}
	}

	return &flowgo.Block{
		Header: &flowgo.Header{
			Height:    b.height,
			View:      b.view,
			ParentID:  b.parentID,
			Timestamp: b.timestamp,
		},
		Payload: &flowgo.Payload{
			Guarantees: guarantees,
		},
	}
}

func (b *pendingBlock) Collections() []*flowgo.LightCollection {
	if len(b.transactionIDs) == 0 {
		return []*flowgo.LightCollection{}
	}

	transactionIDs := make([]flowgo.Identifier, len(b.transactionIDs))

	// TODO: remove once SDK models are removed
	for i, transactionID := range b.transactionIDs {
		transactionIDs[i] = flowgo.Identifier(transactionID)
	}

	collection := flowgo.LightCollection{Transactions: transactionIDs}

	return []*flowgo.LightCollection{&collection}
}

func (b *pendingBlock) Transactions() map[flowgo.Identifier]*flowgo.TransactionBody {
	return b.transactions
}

func (b *pendingBlock) TransactionResults() map[flowgo.Identifier]IndexedTransactionResult {
	return b.transactionResults
}

// LedgerDelta returns the ledger delta for the pending block.
func (b *pendingBlock) LedgerDelta() delta.Delta {
	return b.ledgerView.Delta()
}

// AddTransaction adds a transaction to the pending block.
func (b *pendingBlock) AddTransaction(tx flowgo.TransactionBody) {
	b.transactionIDs = append(b.transactionIDs, tx.ID())
	b.transactions[tx.ID()] = &tx
}

// ContainsTransaction checks if a transaction is included in the pending block.
func (b *pendingBlock) ContainsTransaction(txID flowgo.Identifier) bool {
	_, exists := b.transactions[txID]
	return exists
}

// GetTransaction retrieves a transaction in the pending block by ID.
func (b *pendingBlock) GetTransaction(txID flowgo.Identifier) *flowgo.TransactionBody {
	return b.transactions[txID]
}

// nextTransaction returns the next indexed transaction.
func (b *pendingBlock) nextTransaction() *flowgo.TransactionBody {
	txID := b.transactionIDs[b.index]
	return b.GetTransaction(txID)
}

// ExecuteNextTransaction executes the next transaction in the pending block.
//
// This function uses the provided execute function to perform the actual
// execution, then updates the pending block with the output.
func (b *pendingBlock) ExecuteNextTransaction(
	execute func(ledgerView state.View, txIndex uint32, tx *flowgo.TransactionBody) (*fvm.TransactionProcedure, error),
) (*fvm.TransactionProcedure, error) {
	tx := b.nextTransaction()

	childView := b.ledgerView.NewChild()

	// increment transaction index even if transaction reverts
	b.index++

	tp, err := execute(childView, b.index, tx)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	b.events = append(b.events, tp.Events...)

	err = b.ledgerView.MergeView(childView)
	if err != nil {
		// fail fast if fatal error occurs
		return nil, err
	}

	b.transactionResults[tx.ID()] = IndexedTransactionResult{
		Transaction: tp,
		Index:       b.index,
	}

	return tp, nil
}

// Events returns all events captured during the execution of the pending block.
func (b *pendingBlock) Events() []flowgo.Event {
	return b.events
}

// ExecutionStarted returns true if the pending block has started executing.
func (b *pendingBlock) ExecutionStarted() bool {
	return b.index > 0
}

// ExecutionComplete returns true if the pending block is fully executed.
func (b *pendingBlock) ExecutionComplete() bool {
	return b.index >= uint32(b.Size())
}

// Size returns the number of transactions in the pending block.
func (b *pendingBlock) Size() int {
	return len(b.transactionIDs)
}

// Empty returns true if the pending block is empty.
func (b *pendingBlock) Empty() bool {
	return b.Size() == 0
}
