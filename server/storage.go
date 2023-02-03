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

package server

import (
	"github.com/psiemens/graceland"

	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/memstore"
	"github.com/onflow/flow-emulator/storage/redis"
	"github.com/onflow/flow-emulator/storage/sqlite"
)

type Storage interface {
	graceland.Routine
	Store() storage.Store
}

type RedisStorage struct {
	store *redis.Store
}

func NewRedisStorage(url string) (*RedisStorage, error) {
	rdb, err := redis.New(url)
	if err != nil {
		return nil, err
	}
	return &RedisStorage{store: rdb}, nil
}

func (s *RedisStorage) Start() error {
	return nil
}

func (s *RedisStorage) Stop() {}

func (s *RedisStorage) Store() storage.Store {
	return s.store
}

type MemoryStorage struct {
	store *memstore.Store
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{store: memstore.New()}
}

func (s *MemoryStorage) Start() error {
	return nil
}

func (s *MemoryStorage) Stop() {}

func (s *MemoryStorage) Store() storage.Store {
	return s.store
}

type SqliteStorage struct {
	store *sqlite.Store
}

func NewSqliteStorage(url string) (*SqliteStorage, error) {
	db, err := sqlite.New(url)
	if err != nil {
		return nil, err
	}
	return &SqliteStorage{store: db}, nil
}

func (s *SqliteStorage) Start() error {
	return nil
}

func (s *SqliteStorage) Stop() {}

func (s *SqliteStorage) Store() storage.Store {
	return s.store
}
