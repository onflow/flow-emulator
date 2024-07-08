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

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/psiemens/graceland"

	"github.com/onflow/flow-emulator/types"
)

const (
	globalStoreName            = "global"
	BlockIndexStoreName        = "blockIndex"
	BlockStoreName             = "blocks"
	CollectionStoreName        = "collections"
	TransactionStoreName       = "transactions"
	TransactionResultStoreName = "transactionResults"
	EventStoreName             = "events"
	LedgerStoreName            = "ledger"
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
	graceland.Routine
	LatestBlockHeight(ctx context.Context) (uint64, error)

	// LatestBlock returns the block with the highest block height.
	LatestBlock(ctx context.Context) (flowgo.Block, error)

	// StoreBlock stores the block in storage. If the exactly same block is already in a storage, return successfully
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
		executionSnapshot *snapshot.ExecutionSnapshot,
		events []flowgo.Event,
	) error

	// CollectionByID gets the collection (transaction IDs only) with the given ID.
	CollectionByID(ctx context.Context, collectionID flowgo.Identifier) (flowgo.LightCollection, error)

	// FullCollectionByID gets the full collection (including transaction bodies) with the given ID.
	FullCollectionByID(ctx context.Context, collectionID flowgo.Identifier) (flowgo.Collection, error)

	// TransactionByID gets the transaction with the given ID.
	TransactionByID(ctx context.Context, transactionID flowgo.Identifier) (flowgo.TransactionBody, error)

	// TransactionResultByID gets the transaction result with the given ID.
	TransactionResultByID(ctx context.Context, transactionID flowgo.Identifier) (types.StorableTransactionResult, error)

	// LedgerByHeight returns a storage snapshot into the ledger state
	// at a given block.
	LedgerByHeight(
		ctx context.Context,
		blockHeight uint64,
	) (snapshot.StorageSnapshot, error)

	// EventsByHeight returns the events in the block at the given height, optionally filtered by type.
	EventsByHeight(ctx context.Context, blockHeight uint64, eventType string) ([]flowgo.Event, error)
}

type SnapshotProvider interface {
	Snapshots() ([]string, error)
	CreateSnapshot(snapshotName string) error
	LoadSnapshot(snapshotName string) error
	SupportSnapshotsWithCurrentConfig() bool
}

type RollbackProvider interface {
	RollbackToBlockHeight(height uint64) error
}

type KeyGenerator interface {
	Storage(key string) string
	LatestBlock() []byte
	ForkedBlock() []byte
	BlockHeight(height uint64) []byte
	Identifier(id flowgo.Identifier) []byte
}

type DataGetter interface {
	GetBytes(ctx context.Context, store string, key []byte) ([]byte, error)
	GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) ([]byte, error)
}

type DataSetter interface {
	SetBytes(ctx context.Context, store string, key []byte, value []byte) error
	SetBytesWithVersion(ctx context.Context, store string, key []byte, value []byte, version uint64) error
}

type DefaultKeyGenerator struct {
}

func (s *DefaultKeyGenerator) Storage(key string) string {
	return key
}

func (s *DefaultKeyGenerator) LatestBlock() []byte {
	return []byte("latest_block_height")
}

func (s *DefaultKeyGenerator) ForkedBlock() []byte {
	return []byte("forked_block_height")
}

func (s *DefaultKeyGenerator) BlockHeight(blockHeight uint64) []byte {
	return []byte(fmt.Sprintf("%032d", blockHeight))
}

func (s *DefaultKeyGenerator) Identifier(id flowgo.Identifier) []byte {
	return []byte(fmt.Sprintf("%x", id))
}

type DefaultStore struct {
	KeyGenerator
	DataSetter
	DataGetter
	CurrentHeight uint64
}

func (s *DefaultStore) SetBlockHeight(height uint64) error {
	s.CurrentHeight = height
	return s.DataSetter.SetBytes(
		context.Background(),
		s.KeyGenerator.Storage(globalStoreName),
		s.KeyGenerator.LatestBlock(),
		mustEncodeUint64(height),
	)
}

func (s *DefaultStore) Start() error {
	return nil
}

func (s *DefaultStore) Stop() {}

func (s *DefaultStore) LatestBlockHeight(ctx context.Context) (latestBlockHeight uint64, err error) {
	latestBlockHeightEnc, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(globalStoreName),
		s.KeyGenerator.LatestBlock(),
	)
	if err != nil {
		return
	}

	err = decodeUint64(&latestBlockHeight, latestBlockHeightEnc)

	return
}

func (s *DefaultStore) LatestBlock(ctx context.Context) (block flowgo.Block, err error) {
	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil {
		return
	}

	encBlock, err := s.DataGetter.GetBytes(
		ctx,
		BlockStoreName,
		s.KeyGenerator.BlockHeight(latestBlockHeight),
	)
	if err != nil {
		return
	}

	err = decodeBlock(&block, encBlock)
	return
}

func (s *DefaultStore) StoreBlock(ctx context.Context, block *flowgo.Block) error {
	s.CurrentHeight = block.Header.Height

	encBlock, err := encodeBlock(*block)
	if err != nil {
		return err
	}

	latestBlockHeight, err := s.LatestBlockHeight(ctx)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	// insert the block by block height
	if err := s.DataSetter.SetBytes(
		ctx,
		s.KeyGenerator.Storage(BlockStoreName),
		s.KeyGenerator.BlockHeight(block.Header.Height),
		encBlock,
	); err != nil {
		return err
	}

	// add block ID to ID->height lookup
	if err := s.DataSetter.SetBytes(
		ctx,
		s.KeyGenerator.Storage(BlockIndexStoreName),
		s.KeyGenerator.Identifier(block.ID()),
		mustEncodeUint64(block.Header.Height),
	); err != nil {
		return err
	}

	// if this is latest block, set latest block
	if block.Header.Height >= latestBlockHeight {
		return s.DataSetter.SetBytes(
			ctx,
			s.KeyGenerator.Storage(globalStoreName),
			s.KeyGenerator.LatestBlock(),
			mustEncodeUint64(block.Header.Height),
		)
	}
	return nil
}

// ForkedBlockHeight returns the height of the block at which the chain forked from a live network.
// All blocks after this height were produced by the emulator.
func (s *DefaultStore) ForkedBlockHeight(ctx context.Context) (forkedBlockHeight uint64, err error) {
	forkedBlockHeightEnc, err := s.DataGetter.GetBytes(ctx, s.KeyGenerator.Storage(globalStoreName), s.KeyGenerator.ForkedBlock())
	if err != nil {
		return
	}
	err = decodeUint64(&forkedBlockHeight, forkedBlockHeightEnc)
	return
}

func (s *DefaultStore) StoreForkedBlockHeight(ctx context.Context, height uint64) error {
	return s.DataSetter.SetBytes(ctx, s.KeyGenerator.Storage(globalStoreName), s.KeyGenerator.ForkedBlock(), mustEncodeUint64(height))
}

func (s *DefaultStore) BlockByHeight(ctx context.Context, blockHeight uint64) (block *flowgo.Block, err error) {
	// get block by block height and decode
	encBlock, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(BlockStoreName),
		s.KeyGenerator.BlockHeight(blockHeight),
	)
	if err != nil {
		return
	}
	block = &flowgo.Block{}
	err = decodeBlock(block, encBlock)
	return
}

func (s *DefaultStore) BlockByID(ctx context.Context, blockID flowgo.Identifier) (block *flowgo.Block, err error) {
	blockHeightEnc, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(BlockIndexStoreName),
		s.KeyGenerator.Identifier(blockID),
	)
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

func (s *DefaultStore) CollectionByID(
	ctx context.Context,
	colID flowgo.Identifier,
) (
	col flowgo.LightCollection,
	err error,
) {
	encCol, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(CollectionStoreName),
		s.KeyGenerator.Identifier(colID),
	)
	if err != nil {
		return
	}

	err = decodeCollection(&col, encCol)

	return
}

func (s *DefaultStore) FullCollectionByID(
	ctx context.Context,
	colID flowgo.Identifier,
) (
	col flowgo.Collection,
	err error,
) {
	light := flowgo.LightCollection{}
	encCol, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(CollectionStoreName),
		s.KeyGenerator.Identifier(colID),
	)
	if err != nil {
		return
	}

	err = decodeCollection(&light, encCol)
	if err != nil {
		return
	}

	txs := make([]*flowgo.TransactionBody, len(light.Transactions))
	for i, txID := range light.Transactions {
		tx, err := s.TransactionByID(ctx, txID)
		if err != nil {
			return col, err
		}
		txs[i] = &tx
	}

	col = flowgo.Collection{
		Transactions: txs,
	}

	return
}

func (s *DefaultStore) InsertCollection(ctx context.Context, col flowgo.LightCollection) error {
	encCol, err := encodeCollection(col)
	if err != nil {
		return err
	}

	return s.DataSetter.SetBytes(
		ctx,
		s.KeyGenerator.Storage(CollectionStoreName),
		s.KeyGenerator.Identifier(col.ID()),
		encCol,
	)
}

func (s *DefaultStore) TransactionByID(
	ctx context.Context,
	txID flowgo.Identifier,
) (
	tx flowgo.TransactionBody,
	err error,
) {
	encTx, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(TransactionStoreName),
		s.KeyGenerator.Identifier(txID),
	)
	if err != nil {
		return
	}

	err = decodeTransaction(&tx, encTx)

	return
}

func (s *DefaultStore) InsertTransaction(ctx context.Context, tx flowgo.TransactionBody) error {
	encTx, err := encodeTransaction(tx)
	if err != nil {
		return err
	}

	return s.DataSetter.SetBytes(
		ctx,
		s.KeyGenerator.Storage(TransactionStoreName),
		s.KeyGenerator.Identifier(tx.ID()),
		encTx,
	)
}

func (s *DefaultStore) TransactionResultByID(
	ctx context.Context,
	txID flowgo.Identifier,
) (
	result types.StorableTransactionResult,
	err error,
) {
	encResult, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(TransactionResultStoreName),
		s.KeyGenerator.Identifier(txID),
	)
	if err != nil {
		return
	}

	err = decodeTransactionResult(&result, encResult)

	return
}

func (s *DefaultStore) InsertTransactionResult(
	ctx context.Context,
	txID flowgo.Identifier,
	result types.StorableTransactionResult,
) error {
	encResult, err := encodeTransactionResult(result)
	if err != nil {
		return err
	}
	return s.DataSetter.SetBytes(
		ctx,
		s.KeyGenerator.Storage(TransactionResultStoreName),
		s.KeyGenerator.Identifier(txID),
		encResult,
	)
}

func (s *DefaultStore) EventsByHeight(ctx context.Context, blockHeight uint64, eventType string) (events []flowgo.Event, err error) {
	eventsEnc, err := s.DataGetter.GetBytes(
		ctx,
		s.KeyGenerator.Storage(EventStoreName),
		s.KeyGenerator.BlockHeight(blockHeight),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
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

func (s *DefaultStore) InsertEvents(ctx context.Context, blockHeight uint64, events []flowgo.Event) error {
	//bluesign: encodes all events instead of inserting one by one
	b, err := encodeEvents(events)
	if err != nil {
		return err
	}

	err = s.DataSetter.SetBytes(ctx,
		s.KeyGenerator.Storage(EventStoreName),
		s.KeyGenerator.BlockHeight(blockHeight),
		b)

	if err != nil {
		return err
	}

	return nil
}

func (s *DefaultStore) InsertExecutionSnapshot(
	ctx context.Context,
	blockHeight uint64,
	executionSnapshot *snapshot.ExecutionSnapshot,
) error {
	for registerID, value := range executionSnapshot.WriteSet {
		err := s.DataSetter.SetBytesWithVersion(
			ctx,
			s.KeyGenerator.Storage(LedgerStoreName),
			[]byte(registerID.String()),
			value,
			blockHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *DefaultStore) CommitBlock(
	ctx context.Context,
	block flowgo.Block,
	collections []*flowgo.LightCollection,
	transactions map[flowgo.Identifier]*flowgo.TransactionBody,
	transactionResults map[flowgo.Identifier]*types.StorableTransactionResult,
	executionSnapshot *snapshot.ExecutionSnapshot,
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

	err = s.InsertExecutionSnapshot(
		ctx,
		block.Header.Height,
		executionSnapshot)
	if err != nil {
		return err
	}

	err = s.InsertEvents(ctx, block.Header.Height, events)
	if err != nil {
		return err
	}

	return nil

}

type defaultStoreSnapshot struct {
	defaultStore *DefaultStore
	ctx          context.Context
	blockHeight  uint64
}

func (snapshot defaultStoreSnapshot) Get(
	id flowgo.RegisterID,
) (
	flowgo.RegisterValue,
	error,
) {
	value, err := snapshot.defaultStore.GetBytesAtVersion(
		snapshot.ctx,
		snapshot.defaultStore.Storage(LedgerStoreName),
		[]byte(id.String()),
		snapshot.blockHeight)

	if err != nil {
		// silence not found errors
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return value, nil
}

func (s *DefaultStore) LedgerByHeight(
	ctx context.Context,
	blockHeight uint64,
) (snapshot.StorageSnapshot, error) {
	return defaultStoreSnapshot{
		defaultStore: s,
		ctx:          ctx,
		blockHeight:  blockHeight,
	}, nil
}
