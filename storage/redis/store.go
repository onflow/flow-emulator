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
	"fmt"

	"github.com/go-redis/redis/v8"

	"github.com/onflow/flow-emulator/storage"
)

// Store implements the Store interface
type Store struct {
	storage.DefaultStore
	options *redis.Options
	rdb     *redis.Client
}

// New returns a new in-memory Store implementation.
func New(url string) (*Store, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	store := &Store{
		options: options,
		rdb:     redis.NewClient(options),
	}
	store.DataSetter = store
	store.DataGetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

func storeKey(store string, key []byte) string {
	return fmt.Sprintf("%s_%s", store, hex.EncodeToString(key))
}

func (s *Store) GetBytes(ctx context.Context, store string, key []byte) ([]byte, error) {
	val, err := s.rdb.Get(ctx, storeKey(store, key)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	rawBytes, err := hex.DecodeString(val)
	if err != nil {
		return nil, err
	}
	return rawBytes, nil
}

func (s *Store) SetBytes(ctx context.Context, store string, key []byte, value []byte) error {
	err := s.rdb.Set(ctx, storeKey(store, key), hex.EncodeToString(value), 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) SetBytesWithVersion(ctx context.Context, store string, key []byte, value []byte, version uint64) error {
	err := s.rdb.ZAdd(ctx,
		storeKey(store, key),
		&redis.Z{
			Score:  float64(version),
			Member: hex.EncodeToString(value),
		},
	).Err()
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) ([]byte, error) {
	val, err := s.rdb.ZRevRangeByScore(ctx,
		storeKey(store, key),
		&redis.ZRangeBy{
			Max:    fmt.Sprintf("%d", version),
			Offset: 0,
			Count:  1,
		},
	).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, storage.ErrNotFound
		}
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

var _ storage.Store = &Store{}
