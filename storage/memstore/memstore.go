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

package memstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/types"
)

// Store implements the Store interface with an in-memory store.
type Store struct {
	mu sync.RWMutex
	// block ID to block height
	blockIDToHeight map[flowgo.Identifier]uint64
	// blocks by height
	blocks map[uint64]flowgo.Block
	// collections by ID
	collections map[flowgo.Identifier]flowgo.LightCollection
	// transactions by ID
	transactions map[flowgo.Identifier]flowgo.TransactionBody
	// Transaction results by ID
	transactionResults map[flowgo.Identifier]types.StorableTransactionResult
	// Ledger states by block height
	ledger map[uint64]snapshot.SnapshotTree
	// events by block height
	eventsByBlockHeight map[uint64][]flowgo.Event
	// highest block height
	blockHeight uint64
}

// New returns a new in-memory Store implementation.
func New() *Store {
	return &Store{
		mu:                  sync.RWMutex{},
		blockIDToHeight:     make(map[flowgo.Identifier]uint64),
		blocks:              make(map[uint64]flowgo.Block),
		collections:         make(map[flowgo.Identifier]flowgo.LightCollection),
		transactions:        make(map[flowgo.Identifier]flowgo.TransactionBody),
		transactionResults:  make(map[flowgo.Identifier]types.StorableTransactionResult),
		ledger:              make(map[uint64]snapshot.SnapshotTree),
		eventsByBlockHeight: make(map[uint64][]flowgo.Event),
	}
}

var _ storage.Store = &Store{}

func (s *Store) Start() error {
	return nil
}

func (s *Store) Stop() {
}

func (s *Store) LatestBlockHeight(ctx context.Context) (uint64, error) {
	b, err := s.LatestBlock(ctx)
	if err != nil {
		return 0, err
	}

	return b.HeaderBody.Height, nil
}

func (s *Store) LatestBlock(_ context.Context) (flowgo.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latestBlock, ok := s.blocks[s.blockHeight]
	if !ok {
		return flowgo.Block{}, storage.ErrNotFound
	}
	return latestBlock, nil
}

func (s *Store) StoreBlock(_ context.Context, block *flowgo.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.storeBlock(block)
}

func (s *Store) storeBlock(block *flowgo.Block) error {
	s.blocks[block.HeaderBody.Height] = *block
	s.blockIDToHeight[block.ID()] = block.HeaderBody.Height

	if block.HeaderBody.Height > s.blockHeight {
		s.blockHeight = block.HeaderBody.Height
	}

	return nil
}

func (s *Store) BlockByID(_ context.Context, blockID flowgo.Identifier) (*flowgo.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockHeight, ok := s.blockIDToHeight[blockID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	block, ok := s.blocks[blockHeight]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return &block, nil

}

func (s *Store) BlockByHeight(_ context.Context, height uint64) (*flowgo.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	block, ok := s.blocks[height]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return &block, nil
}

func (s *Store) CommitBlock(
	_ context.Context,
	block flowgo.Block,
	collections []*flowgo.LightCollection,
	transactions map[flowgo.Identifier]*flowgo.TransactionBody,
	transactionResults map[flowgo.Identifier]*types.StorableTransactionResult,
	executionSnapshot *snapshot.ExecutionSnapshot,
	events []flowgo.Event,
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

	err := s.storeBlock(&block)
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

	err = s.insertExecutionSnapshot(
		block.HeaderBody.Height,
		executionSnapshot)
	if err != nil {
		return err
	}

	err = s.insertEvents(block.HeaderBody.Height, events)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CollectionByID(
	_ context.Context,
	collectionID flowgo.Identifier,
) (flowgo.LightCollection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.collections[collectionID]
	if !ok {
		return flowgo.LightCollection{}, storage.ErrNotFound
	}
	return tx, nil
}

func (s *Store) FullCollectionByID(
	_ context.Context,
	collectionID flowgo.Identifier,
) (flowgo.Collection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	light, ok := s.collections[collectionID]
	if !ok {
		return flowgo.Collection{}, storage.ErrNotFound
	}

	txs := make([]*flowgo.TransactionBody, len(light.Transactions))
	for i, txID := range light.Transactions {
		tx, ok := s.transactions[txID]
		if !ok {
			return flowgo.Collection{}, storage.ErrNotFound
		}
		txs[i] = &tx
	}

	return flowgo.Collection{
		Transactions: txs,
	}, nil
}

func (s *Store) TransactionByID(
	_ context.Context,
	transactionID flowgo.Identifier,
) (flowgo.TransactionBody, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[transactionID]
	if !ok {
		return flowgo.TransactionBody{}, storage.ErrNotFound
	}
	return tx, nil

}

func (s *Store) TransactionResultByID(
	_ context.Context,
	transactionID flowgo.Identifier,
) (types.StorableTransactionResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, ok := s.transactionResults[transactionID]
	if !ok {
		return types.StorableTransactionResult{}, storage.ErrNotFound
	}
	return result, nil

}

func (s *Store) LedgerByHeight(
	_ context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	return s.ledger[blockHeight], nil
}

func (s *Store) EventsByHeight(
	_ context.Context,
	blockHeight uint64,
	eventType string,
) ([]flowgo.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allEvents := s.eventsByBlockHeight[blockHeight]

	events := make([]flowgo.Event, 0)

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

func (s *Store) insertCollection(col flowgo.LightCollection) error {
	s.collections[col.ID()] = col
	return nil
}

func (s *Store) insertTransaction(txID flowgo.Identifier, tx flowgo.TransactionBody) error {
	s.transactions[txID] = tx
	return nil
}

func (s *Store) insertTransactionResult(txID flowgo.Identifier, result types.StorableTransactionResult) error {
	s.transactionResults[txID] = result
	return nil
}

func (s *Store) insertExecutionSnapshot(
	blockHeight uint64,
	executionSnapshot *snapshot.ExecutionSnapshot,
) error {
	oldLedger := s.ledger[blockHeight-1]

	s.ledger[blockHeight] = oldLedger.Append(executionSnapshot)

	return nil
}

func (s *Store) insertEvents(blockHeight uint64, events []flowgo.Event) error {
	if s.eventsByBlockHeight[blockHeight] == nil {
		s.eventsByBlockHeight[blockHeight] = events
	} else {
		s.eventsByBlockHeight[blockHeight] = append(s.eventsByBlockHeight[blockHeight], events...)
	}

	return nil
}
