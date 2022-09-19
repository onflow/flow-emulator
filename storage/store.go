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

// Package storage defines the interface and implementations for interacting with
// persistent chain state.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	flowgo "github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-emulator/types"
)

const (
	globalStoreName            = "global"
	blockIndexStoreName        = "blockIndex"
	blockStoreName             = "blocks"
	collectionStoreName        = "collections"
	transactionStoreName       = "transactions"
	transactionResultStoreName = "transactionResults"
	eventStoreName             = "events"
	ledgerStoreName            = "ledger"
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
	LatestBlockHeight(ctx context.Context) (uint64, error)

	// LatestBlock returns the block with the highest block height.
	LatestBlock(ctx context.Context) (flowgo.Block, error)

	// Store stores the block. If the exactly same block is already in a storage, return successfully
	StoreBlock(ctx context.Context, block *flowgo.Block) error

	// BlockByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	BlockByID(ctx context.Context, blockID flowgo.Identifier) (*flowgo.Block, error)

	// BlockByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	BlockByHeight(ctx context.Context, height uint64) (*flowgo.Block, error)

	// CommitBlock atomically saves the execution results for a block.
	CommitBlock(
		ctx context.Context,
		block flowgo.Block,
		collections []*flowgo.LightCollection,
		transactions map[flowgo.Identifier]*flowgo.TransactionBody,
		transactionResults map[flowgo.Identifier]*types.StorableTransactionResult,
		delta delta.Delta,
		events []flowgo.Event,
	) error

	// CollectionByID gets the collection (transaction IDs only) with the given ID.
	CollectionByID(ctx context.Context, collectionID flowgo.Identifier) (flowgo.LightCollection, error)

	// TransactionByID gets the transaction with the given ID.
	TransactionByID(ctx context.Context, transactionID flowgo.Identifier) (flowgo.TransactionBody, error)

	// TransactionResultByID gets the transaction result with the given ID.
	TransactionResultByID(ctx context.Context, transactionID flowgo.Identifier) (types.StorableTransactionResult, error)

	// LedgerViewByHeight returns a view into the ledger state at a given block.
	LedgerViewByHeight(ctx context.Context, blockHeight uint64) *delta.View

	// EventsByHeight returns the events in the block at the given height, optionally filtered by type.
	EventsByHeight(ctx context.Context, blockHeight uint64, eventType string) ([]flowgo.Event, error)
}

type StorageBackendKeys interface {
	StorageKey(key string) string
	LatestBlockKey() []byte
	BlockHeightKey(height uint64) []byte
	IdentifierKey(id flowgo.Identifier) []byte
	EventKey(blockHeight uint64, txIndex, eventIndex uint32, eventType flowgo.EventType) []byte
}

type StorageBackendData interface {
	GetBytes(ctx context.Context, store string, key []byte) ([]byte, error)
	SetBytes(ctx context.Context, store string, key []byte, value []byte) error
	SetBytesWithVersion(ctx context.Context, store string, key []byte, value []byte, version uint64) error
	GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) ([]byte, error)
}

type StorageBackendKeysImpl struct {
}

func (s *StorageBackendKeysImpl) StorageKey(key string) string {
	return key
}

func (s *StorageBackendKeysImpl) LatestBlockKey() []byte {
	return []byte("latest_block_height")
}

func (s *StorageBackendKeysImpl) BlockHeightKey(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%032d", blockHeight))
}

func (s *StorageBackendKeysImpl) IdentifierKey(id flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%x", id))
}

func (s *StorageBackendKeysImpl) EventKey(blockHeight uint64, txIndex, eventIndex uint32, eventType flowgo.EventType) []byte {
	return []byte(fmt.Sprintf(
		"%032d-%032d-%032d-%s",
		blockHeight,
		txIndex,
		eventIndex,
		eventType,
	))
}

type StoreImpl struct {
	KeysBackend StorageBackendKeys
	DataBackend StorageBackendData
}

func (s *StoreImpl) LatestBlockHeight(ctx context.Context) (latestBlockHeight uint64, err error) {
	latestBlockHeightEnc, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(globalStoreName), s.KeysBackend.LatestBlockKey())
	if err != nil {
		return
	}
	err = decodeUint64(&latestBlockHeight, latestBlockHeightEnc)
	return
}

func (s *StoreImpl) LatestBlock(ctx context.Context) (block flowgo.Block, err error) {
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return
	}
	encBlock, err := s.DataBackend.GetBytes(ctx, blockStoreName, s.KeysBackend.BlockHeightKey(latestBlockHeight))
	if err != nil {
		return
	}
	err = decodeBlock(&block, encBlock)
	return
}

func (s *StoreImpl) StoreBlock(ctx context.Context, block *flowgo.Block) error {

	encBlock, err := encodeBlock(*block)
	if err != nil {
		return err
	}
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	// insert the block by block height
	if err := s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(blockStoreName), s.KeysBackend.BlockHeightKey(block.Header.Height), encBlock); err != nil {
		return err
	}
	// add block ID to ID->height lookup
	if err := s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(blockIndexStoreName), s.KeysBackend.IdentifierKey(block.ID()), encodeUint64(block.Header.Height)); err != nil {
		return err
	}
	// if this is latest block, set latest block
	if block.Header.Height >= latestBlockHeight {
		return s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(globalStoreName), s.KeysBackend.LatestBlockKey(), encodeUint64(block.Header.Height))
	}
	return nil
}

func (s *StoreImpl) BlockByHeight(ctx context.Context, blockHeight uint64) (block *flowgo.Block, err error) {
	// get block by block height and decode
	encBlock, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(blockStoreName), s.KeysBackend.BlockHeightKey(blockHeight))
	if err != nil {
		return
	}
	block = &flowgo.Block{}
	err = decodeBlock(block, encBlock)
	return
}

func (s *StoreImpl) BlockByID(ctx context.Context, blockID flowgo.Identifier) (block *flowgo.Block, err error) {
	blockHeightEnc, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(blockIndexStoreName), s.KeysBackend.IdentifierKey(blockID))
	if err != nil {
		return
	}
	var blockHeight uint64
	err = decodeUint64(&blockHeight, blockHeightEnc)
	if err != nil {
		return
	}
	return s.BlockByHeight(ctx, blockHeight)
}

func (s *StoreImpl) CollectionByID(ctx context.Context, colID flowgo.Identifier) (col flowgo.LightCollection, err error) {
	encCol, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(collectionStoreName), s.KeysBackend.IdentifierKey(colID))
	if err != nil {
		return
	}
	err = decodeCollection(&col, encCol)
	return
}

func (s *StoreImpl) InsertCollection(ctx context.Context, col flowgo.LightCollection) error {
	encCol, err := encodeCollection(col)
	if err != nil {
		return err
	}
	return s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(collectionStoreName), s.KeysBackend.IdentifierKey(col.ID()), encCol)
}

func (s *StoreImpl) TransactionByID(ctx context.Context, txID flowgo.Identifier) (tx flowgo.TransactionBody, err error) {
	encTx, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(transactionStoreName), s.KeysBackend.IdentifierKey(txID))
	if err != nil {
		return
	}
	err = decodeTransaction(&tx, encTx)
	return
}

func (s *StoreImpl) InsertTransaction(ctx context.Context, tx flowgo.TransactionBody) error {
	encTx, err := encodeTransaction(tx)
	if err != nil {
		return err
	}
	return s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(transactionStoreName), s.KeysBackend.IdentifierKey(tx.ID()), encTx)
}

func (s *StoreImpl) TransactionResultByID(ctx context.Context, txID flowgo.Identifier) (result types.StorableTransactionResult, err error) {
	encResult, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(transactionResultStoreName), s.KeysBackend.IdentifierKey(txID))
	if err != nil {
		return
	}
	err = decodeTransactionResult(&result, encResult)
	return
}

func (s *StoreImpl) InsertTransactionResult(ctx context.Context, txID flowgo.Identifier, result types.StorableTransactionResult) error {
	encResult, err := encodeTransactionResult(result)
	if err != nil {
		return err
	}
	return s.DataBackend.SetBytes(ctx, s.KeysBackend.StorageKey(transactionResultStoreName), s.KeysBackend.IdentifierKey(txID), encResult)
}

func (s *StoreImpl) EventsByHeight(ctx context.Context, blockHeight uint64, eventType string) (events []flowgo.Event, err error) {
	eventsEnc, err := s.DataBackend.GetBytes(ctx, s.KeysBackend.StorageKey(eventStoreName), s.KeysBackend.BlockHeightKey(blockHeight))
	if err != nil {
		if err == ErrNotFound {
			return []flowgo.Event{}, nil
		}
		return
	}
	var blockEvents []flowgo.Event
	err = decodeEvents(&blockEvents, eventsEnc)
	if err != nil {
		return
	}
	for _, event := range blockEvents {
		if eventType != "" && event.Type != flowgo.EventType(eventType) {
			continue
		}
		events = append(events, event)
	}
	return
}

func (s *StoreImpl) InsertEvents(ctx context.Context, blockHeight uint64, events []flowgo.Event) error {
	//bluesign: encodes all events instead of inserting one by one
	b, err := encodeEvents(events)
	if err != nil {
		return err
	}

	err = s.DataBackend.SetBytes(ctx,
		s.KeysBackend.StorageKey(eventStoreName),
		s.KeysBackend.BlockHeightKey(blockHeight),
		b)

	if err != nil {
		return err
	}

	return nil
}

func (s *StoreImpl) InsertLedgerDelta(ctx context.Context, blockHeight uint64, delta delta.Delta) error {
	updatedIDs, updatedValues := delta.RegisterUpdates()
	for i, registerID := range updatedIDs {
		value := updatedValues[i]
		err := s.DataBackend.SetBytesWithVersion(ctx, s.KeysBackend.StorageKey(ledgerStoreName), []byte(registerID.String()), value, blockHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StoreImpl) CommitBlock(
	ctx context.Context,
	block flowgo.Block,
	collections []*flowgo.LightCollection,
	transactions map[flowgo.Identifier]*flowgo.TransactionBody,
	transactionResults map[flowgo.Identifier]*types.StorableTransactionResult,
	delta delta.Delta,
	events []flowgo.Event,
) error {

	if len(transactions) != len(transactionResults) {
		return fmt.Errorf(
			"transactions count (%d) does not match result count (%d)",
			len(transactions),
			len(transactionResults),
		)
	}

	err := s.StoreBlock(ctx, &block)
	if err != nil {
		return err
	}

	for _, col := range collections {
		err := s.InsertCollection(ctx, *col)
		if err != nil {
			return err
		}
	}

	for _, tx := range transactions {
		err := s.InsertTransaction(ctx, *tx)
		if err != nil {
			return err
		}
	}

	for txID, result := range transactionResults {
		err := s.InsertTransactionResult(ctx, txID, *result)
		if err != nil {
			return err
		}
	}

	err = s.InsertLedgerDelta(ctx, block.Header.Height, delta)
	if err != nil {
		return err
	}

	err = s.InsertEvents(ctx, block.Header.Height, events)
	if err != nil {
		return err
	}

	return nil

}

func (s *StoreImpl) LedgerViewByHeight(ctx context.Context, blockHeight uint64) *delta.View {
	return delta.NewView(func(owner, key string) (value flowgo.RegisterValue, err error) {
		id := flowgo.RegisterID{
			Owner: owner,
			Key:   key,
		}

		value, err = s.DataBackend.GetBytesAtVersion(ctx, s.KeysBackend.StorageKey(ledgerStoreName), []byte(id.String()), blockHeight)

		if err != nil {
			// silence not found errors
			if errors.Is(err, ErrNotFound) {
				return nil, nil
			}

			return nil, err
		}

		return value, nil
	})
}
