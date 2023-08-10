//go:build !JS
// +build !JS

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

package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/glebarez/go-sqlite"
	"github.com/onflow/flow-emulator/storage"
)

const InMemory = ":memory:"

var _ storage.SnapshotProvider = &Store{}
var _ storage.Store = &Store{}
var _ storage.RollbackProvider = &Store{}

//go:embed createTables.sql
var createTablesSql string

// Store implements the Store interface
type Store struct {
	storage.DefaultStore
	db            *sql.DB
	url           string
	mu            sync.RWMutex
	snapshotNames []string
}

// New returns a new in-memory Store implementation.
func New(url string) (store *Store, err error) {

	dbUrl := url
	if dbUrl != InMemory {
		urlInfo, err := os.Stat(url)
		if err == nil && urlInfo.IsDir() {
			dbUrl = filepath.Join(url, "emulator.sqlite")
			if err != nil {
				return nil, err
			}
		}
	}

	db, err := sql.Open("sqlite", dbUrl)
	if err != nil {
		return nil, err
	}

	err = initDb(db)
	if err != nil {
		return nil, err
	}

	store = &Store{
		db:  db,
		url: url,
	}

	store.DataSetter = store
	store.DataGetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
}

func initDb(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(createTablesSql)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RollbackToBlockHeight(height uint64) error {
	if s.CurrentHeight <= height {
		return fmt.Errorf("rollback height should be less then current height")
	}

	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}

	for _, table := range []string{"ledger", "blocks", "blockIndex", "events", "transactions", "collections", "transactionResults"} {
		_, err = tx.Exec(fmt.Sprintf(`DELETE from %s where height>%d`, table, height))
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return s.DefaultStore.SetBlockHeight(height)
}

func (s *Store) Snapshots() (snapshots []string, err error) {
	if !s.SupportSnapshotsWithCurrentConfig() {
		return []string{}, fmt.Errorf("snapshot is not supported with current configuration")
	}

	if s.url == InMemory {
		return s.snapshotNames, nil
	}

	files, err := os.ReadDir(s.url)
	if err != nil {
		return snapshots, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasPrefix(file.Name(), "snapshot_") {
			snapshotName := strings.TrimPrefix(file.Name(), "snapshot_")
			snapshots = append(snapshots, snapshotName)
		}

	}
	return snapshots, nil
}
func (s *Store) LoadSnapshot(name string) error {
	if !s.SupportSnapshotsWithCurrentConfig() {
		return fmt.Errorf("snapshot is not supported with current configuration")
	}

	var dbfile string
	if s.url == InMemory {
		dbfile = fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
		db, err := sql.Open("sqlite", dbfile)
		if err != nil {
			return err
		}

		//check if existing snapshot? (it has to have at least one table, as memorydb will automatically create any db)
		result := db.QueryRow("SELECT count(name) FROM sqlite_schema WHERE type='table'")
		var count int
		err = result.Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			return fmt.Errorf("snapshot %s does not exist", name)
		}
	} else {
		dbfile = filepath.Join(s.url, fmt.Sprintf("snapshot_%s", name))
		_, err := os.Stat(dbfile)
		if os.IsNotExist(err) {
			return fmt.Errorf("snapshot %s does not exist", name)
		}
	}

	db, err := sql.Open("sqlite", dbfile)
	if err != nil {
		return err
	}

	s.db.Close()
	s.db = db

	return nil
}

func (s *Store) CreateSnapshot(name string) error {
	if !s.SupportSnapshotsWithCurrentConfig() {
		return fmt.Errorf("snapshot is not supported with current configuration")
	}

	var dbfile string
	if s.url == InMemory {
		dbfile = fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
		db, err := sql.Open("sqlite", dbfile)
		if err != nil {
			return err
		}

		//check if existing snapshot? (it has to have at least one table, as memorydb will automatically create any db)
		result := db.QueryRow("SELECT count(name) FROM sqlite_schema WHERE type='table'")
		var count int
		err = result.Scan(&count)
		if err != nil {
			return err
		}

	} else {
		dbfile = filepath.Join(s.url, fmt.Sprintf("snapshot_%s", name))
	}

	_, err := s.db.Exec(fmt.Sprintf("VACUUM main INTO '%s'", dbfile))
	if err != nil {
		return err
	}
	s.snapshotNames = append(s.snapshotNames, name)
	return nil
}

func (s *Store) SupportSnapshotsWithCurrentConfig() bool {
	if s.url == InMemory {
		return true
	}
	fileInfo, err := os.Stat(s.url)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

func (s *Store) GetBytes(ctx context.Context, store string, key []byte) ([]byte, error) {
	return s.GetBytesAtVersion(ctx, store, key, 0)
}

func (s *Store) SetBytes(ctx context.Context, store string, key []byte, value []byte) error {
	return s.SetBytesWithVersion(ctx, store, key, value, 0)
}

func (s *Store) SetBytesWithVersion(ctx context.Context, store string, key []byte, value []byte, version uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	height := s.CurrentHeight
	//global table has no height
	if store == "global" {
		height = 0
	}
	_, err := s.db.Exec(
		fmt.Sprintf(
			"INSERT INTO %s (key, version, value, height) VALUES (?, ?, ?, ?) ON CONFLICT(key, version, height) DO UPDATE SET value=excluded.value",
			store,
		),
		hex.EncodeToString(key),
		version,
		hex.EncodeToString(value),
		height,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(
		fmt.Sprintf(
			"SELECT value from %s  WHERE key = ? and version <= ? order by version desc LIMIT 1",
			store,
		),
		hex.EncodeToString(key),
		version,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		rawBytes, err := hex.DecodeString(value)
		if err != nil {
			return nil, err
		} else {
			return rawBytes, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, storage.ErrNotFound
}
func (s *Store) Close() error {
	s.db.Close()
	return nil
}
