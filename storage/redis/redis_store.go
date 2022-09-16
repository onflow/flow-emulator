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

package redis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	redis "github.com/go-redis/redis/v8"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	flowgo "github.com/onflow/flow-go/model/flow"
	msgpack "github.com/vmihailenco/msgpack/v5"
	"strconv"
	"sync"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/types"
)

func ledgerValueKey(registerID flowgo.RegisterID) []byte {
	return []byte(fmt.Sprintf("%s-%s", "ledgerValue", registerID.String()))
}

func ledgerChangelogKey(registerID flowgo.RegisterID) []byte {
	return []byte(fmt.Sprintf("%s-%s-%s",
		"ledgerChangeLog",
		registerID.Owner,
		registerID.Key))
}

// Store implements the Store interface with an in-memory store.
type Store struct {
	mu  sync.RWMutex
	rdb *redis.Client

	// highest block height
	blockHeight uint64
}

// New returns a new in-memory Store implementation.
func New(url string) (*Store, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(options)

	s := &Store{
		rdb: rdb,
		mu:  sync.RWMutex{},
	}
	s.blockHeight, _ = s.getInt("blockHeight", "latest")

	return s, nil
}

func (s *Store) get(store string, key any) ([]byte, error) {
	rawKey, err := msgpack.Marshal(key)
	if err != nil {
		return nil, err
	}

	val, err := s.rdb.Get(context.Background(), fmt.Sprintf("%s_%s", store, hex.EncodeToString(rawKey))).Result()

	if err != nil {
		return nil, err
	}

	rawBytes, err := hex.DecodeString(val)
	if err != nil {
		return nil, err
	}

	return rawBytes, nil
}
func (s *Store) setInt(store string, key string, value uint64) error {
	err := s.rdb.Set(context.Background(), fmt.Sprintf("%s_%s", store, key), fmt.Sprintf("%d", value), 0).Err()
	if err != nil {
		return err
	}
	return nil
}
func (s *Store) getInt(store string, key string) (uint64, error) {
	val, err := s.rdb.Get(context.Background(), fmt.Sprintf("%s_%s", store, key)).Result()
	if err != nil {
		return 0, err
	}
	result, _ := strconv.ParseUint(val, 0, 64)

	return result, nil
}

func (s *Store) set(store string, key any, value any) error {
	rawKey, err := msgpack.Marshal(key)
	if err != nil {
		return err
	}

	rawValue, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}

	err = s.rdb.Set(context.Background(), fmt.Sprintf("%s_%s", store, hex.EncodeToString(rawKey)), hex.EncodeToString(rawValue), 0).Err()
	if err != nil {
		return err
	}
	return nil
}
func (s *Store) zget(store string, key any, blockHeight uint64) ([]byte, error) {
	rawKey, err := msgpack.Marshal(key)
	if err != nil {
		return nil, err
	}

	val, err := s.rdb.ZRevRangeByScore(context.Background(), fmt.Sprintf("%s_%s", store, hex.EncodeToString(rawKey)), &redis.ZRangeBy{Max: fmt.Sprintf("%d", blockHeight), Offset: 0, Count: 1}).Result()

	if err != nil {
		return nil, err
	}

	if len(val) == 0 {
		return nil, storage.ErrNotFound
	}

	rawBytes, err := hex.DecodeString(val[0])
	if err != nil {
		return nil, err
	}

	return rawBytes, nil
}

func (s *Store) zadd(store string, key any, value []byte, blockHeight uint64) error {
	rawKey, err := msgpack.Marshal(key)
	if err != nil {
		return err
	}

	err = s.rdb.ZAdd(context.Background(), fmt.Sprintf("%s_%s", store, hex.EncodeToString(rawKey)), &redis.Z{Score: float64(blockHeight), Member: hex.EncodeToString(value)}).Err()
	if err != nil {
		return err
	}
	return nil
}

var _ storage.Store = &Store{}

func (s *Store) BlockByID(id flowgo.Identifier) (block *flowgo.Block, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blockHeight, err := s.getInt("blockIDToHeight", id.String())
	if err != nil {
		return nil, storage.ErrNotFound
	}

	rawBlock, err := s.get("blocks", blockHeight)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var aBlock flowgo.Block
	err = msgpack.Unmarshal(rawBlock, &aBlock)

	return &aBlock, err
}

func (s *Store) BlockByHeight(blockHeight uint64) (block *flowgo.Block, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawBlock, err := s.get("blocks", blockHeight)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var aBlock flowgo.Block
	err = msgpack.Unmarshal(rawBlock, &aBlock)

	return &aBlock, err
}

func (s *Store) LatestBlock() (block flowgo.Block, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawBlock, err := s.get("blocks", s.blockHeight)
	if err != nil {
		return flowgo.Block{}, storage.ErrNotFound
	}
	var aBlock flowgo.Block
	err = msgpack.Unmarshal(rawBlock, &aBlock)

	return aBlock, err
}

func (s *Store) StoreBlock(block *flowgo.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.storeBlock(block)
}

func (s *Store) storeBlock(block *flowgo.Block) error {
	var aBlock flowgo.Block = *block
	_ = s.set("blocks", block.Header.Height, aBlock)
	_ = s.setInt("blockIDToHeight", block.ID().String(), block.Header.Height)

	if block.Header.Height > s.blockHeight {
		s.blockHeight = block.Header.Height
		_ = s.setInt("blockHeight", "latest", block.Header.Height)
	}

	return nil
}

func (s *Store) CommitBlock(
	block flowgo.Block,
	collections []*flowgo.LightCollection,
	transactions map[flowgo.Identifier]*flowgo.TransactionBody,
	transactionResults map[flowgo.Identifier]*types.StorableTransactionResult,
	delta delta.Delta,
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

func (s *Store) CollectionByID(colID flowgo.Identifier) (collection flowgo.LightCollection, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawCollection, err := s.get("collections", colID)
	if err != nil {
		return flowgo.LightCollection{}, storage.ErrNotFound
	}

	err = msgpack.Unmarshal(rawCollection, &collection)
	if err != nil {
		return flowgo.LightCollection{}, storage.ErrNotFound
	}

	return collection, nil
}

func (s *Store) insertCollection(col flowgo.LightCollection) error {
	_ = s.set("collections", col.ID(), col)
	return nil
}

func (s *Store) TransactionByID(txID flowgo.Identifier) (tx flowgo.TransactionBody, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawTx, err := s.get("transactions", txID)
	if err != nil {
		return flowgo.TransactionBody{}, storage.ErrNotFound
	}

	err = msgpack.Unmarshal(rawTx, &tx)
	if err != nil {
		return flowgo.TransactionBody{}, storage.ErrNotFound
	}

	return tx, nil
}

func (s *Store) insertTransaction(txID flowgo.Identifier, tx flowgo.TransactionBody) error {
	_ = s.set("transactions", txID, tx)
	return nil
}

func (s *Store) TransactionResultByID(txID flowgo.Identifier) (txResult types.StorableTransactionResult, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rawResult, err := s.get("transactionResults", txID)
	if err != nil {
		return types.StorableTransactionResult{}, storage.ErrNotFound
	}

	err = msgpack.Unmarshal(rawResult, &txResult)
	if err != nil {
		return types.StorableTransactionResult{}, storage.ErrNotFound
	}

	return txResult, nil
}

func (s *Store) insertTransactionResult(txID flowgo.Identifier, result types.StorableTransactionResult) error {
	_ = s.set("transactionResults", txID, result)
	return nil
}

func (s *Store) LedgerViewByHeight(blockHeight uint64) *delta.View {
	return delta.NewView(func(owner, key string) (value flowgo.RegisterValue, err error) {
		id := flowgo.RegisterID{
			Owner: owner,
			Key:   key,
		}
		value, err = s.zget("ledgerView", ledgerValueKey(id), blockHeight)

		if err != nil {
			// silence not found errors
			if errors.Is(err, storage.ErrNotFound) {
				return nil, nil
			}

			return nil, err
		}

		return value, nil
	})
}

func (s *Store) UnsafeInsertLedgerDelta(blockHeight uint64, delta delta.Delta) error {
	return s.insertLedgerDelta(blockHeight, delta)
}

func (s *Store) insertLedgerDelta(blockHeight uint64, delta delta.Delta) error {

	updatedIDs, updatedValues := delta.RegisterUpdates()
	for i, registerID := range updatedIDs {
		value := updatedValues[i]
		err := s.zadd("ledgerView", ledgerValueKey(registerID), value, blockHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allEventsRaw, err := s.get("eventsByBlockHeight", blockHeight)
	if err != nil {
		return nil, err
	}
	var allEvents []flowgo.Event
	err = msgpack.Unmarshal(allEventsRaw, &allEvents)
	if err != nil {
		return nil, err
	}

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

func (s *Store) insertEvents(blockHeight uint64, events []flowgo.Event) error {

	var allEvents []flowgo.Event
	allEventsRaw, err := s.get("eventsByBlockHeight", blockHeight)
	if err != nil {
		allEvents = make([]flowgo.Event, 0)
	}
	err = msgpack.Unmarshal(allEventsRaw, &allEvents)
	if err != nil {
		allEvents = make([]flowgo.Event, 0)
	}
	allEvents = append(allEvents, events...)

	_ = s.set("eventsByBlockHeight", blockHeight, allEvents)

	return nil
}
