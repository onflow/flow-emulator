package memstore

import (
	"fmt"
	"sync"

	vm "github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	model "github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-emulator/storage"
	"github.com/dapperlabs/flow-emulator/types"
)

// Store implements the Store interface with an in-memory store.
type Store struct {
	mu sync.RWMutex
	// block ID to block height
	blockIDToHeight map[model.Identifier]uint64
	// blocks by height
	blocks map[uint64]model.Block
	// collections by ID
	collections map[model.Identifier]model.LightCollection
	// transactions by ID
	transactions map[model.Identifier]model.TransactionBody
	// Transaction results by ID
	transactionResults map[model.Identifier]types.StorableTransactionResult
	// Ledger states by block height
	ledger map[uint64]vm.MapLedger
	// events by block height
	eventsByBlockHeight map[uint64][]model.Event
	// highest block height
	blockHeight uint64
}

// New returns a new in-memory Store implementation.
func New() *Store {
	return &Store{
		mu:                  sync.RWMutex{},
		blockIDToHeight:     make(map[model.Identifier]uint64),
		blocks:              make(map[uint64]model.Block),
		collections:         make(map[model.Identifier]model.LightCollection),
		transactions:        make(map[model.Identifier]model.TransactionBody),
		transactionResults:  make(map[model.Identifier]types.StorableTransactionResult),
		ledger:              make(map[uint64]vm.MapLedger),
		eventsByBlockHeight: make(map[uint64][]model.Event),
	}
}

var _ storage.Store = &Store{}

func (s *Store) BlockByID(id model.Identifier) (model.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockHeight := s.blockIDToHeight[id]
	block, ok := s.blocks[blockHeight]
	if !ok {
		return model.Block{}, storage.ErrNotFound
	}

	return block, nil
}

func (s *Store) BlockByHeight(blockHeight uint64) (model.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	block, ok := s.blocks[blockHeight]
	if !ok {
		return model.Block{}, storage.ErrNotFound
	}

	return block, nil
}

func (s *Store) LatestBlock() (model.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latestBlock, ok := s.blocks[s.blockHeight]
	if !ok {
		return model.Block{}, storage.ErrNotFound
	}
	return latestBlock, nil
}

func (s *Store) InsertBlock(block model.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.insertBlock(block)
}

func (s *Store) insertBlock(block model.Block) error {
	s.blocks[block.Header.Height] = block
	if block.Header.Height > s.blockHeight {
		s.blockHeight = block.Header.Height
	}

	return nil
}

func (s *Store) CommitBlock(
	block model.Block,
	collections []*model.LightCollection,
	transactions map[model.Identifier]*model.TransactionBody,
	transactionResults map[model.Identifier]*types.StorableTransactionResult,
	delta delta.Delta,
	events []model.Event,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(transactions) != len(transactionResults) {
		return fmt.Errorf(
			"transactions count (%d) does not match result count (%d)",
			len(transactions),
			len(transactionResults),
		)
	}

	err := s.insertBlock(block)
	if err != nil {
		return err
	}

	for _, col := range collections {
		err := s.insertCollection(*col)
		if err != nil {
			return err
		}
	}

	for _, tx := range transactions {
		err := s.insertTransaction(tx.ID(), *tx)
		if err != nil {
			return err
		}
	}

	for txID, result := range transactionResults {
		err := s.insertTransactionResult(txID, *result)
		if err != nil {
			return err
		}
	}

	err = s.insertLedgerDelta(block.Header.Height, delta)
	if err != nil {
		return err
	}

	err = s.insertEvents(block.Header.Height, events)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CollectionByID(colID model.Identifier) (model.LightCollection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.collections[colID]
	if !ok {
		return model.LightCollection{}, storage.ErrNotFound
	}
	return tx, nil
}

func (s *Store) insertCollection(col model.LightCollection) error {
	s.collections[col.ID()] = col
	return nil
}

func (s *Store) TransactionByID(txID model.Identifier) (model.TransactionBody, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[txID]
	if !ok {
		return model.TransactionBody{}, storage.ErrNotFound
	}
	return tx, nil
}

func (s *Store) insertTransaction(txID model.Identifier, tx model.TransactionBody) error {
	s.transactions[txID] = tx
	return nil
}

func (s *Store) TransactionResultByID(txID model.Identifier) (types.StorableTransactionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, ok := s.transactionResults[txID]
	if !ok {
		return types.StorableTransactionResult{}, storage.ErrNotFound
	}
	return result, nil
}

func (s *Store) insertTransactionResult(txID model.Identifier, result types.StorableTransactionResult) error {
	s.transactionResults[txID] = result
	return nil
}

func (s *Store) LedgerViewByHeight(blockHeight uint64) *delta.View {
	return delta.NewView(func(key model.RegisterID) (value model.RegisterValue, err error) {

		//return types.NewLedgerView(func(key string) ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		ledger, ok := s.ledger[blockHeight]
		if !ok {
			return nil, nil
		}

		return ledger[string(key)], nil
	})
}

func (s *Store) insertLedgerDelta(blockHeight uint64, delta delta.Delta) error {
	var oldLedger vm.MapLedger

	// use empty ledger if this is the genesis block
	if blockHeight == 0 {
		oldLedger = make(vm.MapLedger)
	} else {
		oldLedger = s.ledger[blockHeight-1]
	}

	newLedger := make(vm.MapLedger)

	// copy values from the previous ledger
	for keyString, value := range oldLedger {
		key := model.RegisterID(keyString)
		// do not copy deleted values
		if !delta.HasBeenDeleted(key) {
			newLedger[keyString] = value
		}
	}

	// write all updated values
	ids, values := delta.RegisterUpdates()
	for i, value := range values {
		key := ids[i]
		if value != nil {
			newLedger[string(key)] = value
		}
	}

	s.ledger[blockHeight] = newLedger

	return nil
}

func (s *Store) EventsByHeight(blockHeight uint64, eventType string) ([]model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allEvents := s.eventsByBlockHeight[blockHeight]

	events := make([]model.Event, 0)

	for _, event := range allEvents {
		if eventType == "" {
			events = append(events, event)
		} else {
			if string(event.Type) == eventType {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

func (s *Store) insertEvents(blockHeight uint64, events []model.Event) error {
	if s.eventsByBlockHeight[blockHeight] == nil {
		s.eventsByBlockHeight[blockHeight] = events
	} else {
		s.eventsByBlockHeight[blockHeight] = append(s.eventsByBlockHeight[blockHeight], events...)
	}

	return nil
}
