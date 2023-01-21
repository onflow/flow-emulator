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
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/glebarez/go-sqlite"
	"github.com/onflow/flow-emulator/storage"
)

var _ storage.SnapshotProvider = &Store{}

// Store implements the Store interface
type Store struct {
	storage.DefaultStore
	db  *sql.DB
	url string
	mu  sync.RWMutex
}

func (s *Store) ListSnapshots() (snapshots []string, err error) {
	if !s.SupportSnapshotsWithCurrentConfig() {
		return []string{}, fmt.Errorf("Snapshot is not supported with memory db")
	}

	path := filepath.Dir(s.url)
	files, err := ioutil.ReadDir(path)
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

func (s *Store) JumpToSnapshot(snapshotName string, createIfNotExists bool) error {
	if !s.SupportSnapshotsWithCurrentConfig() {
		return fmt.Errorf("Snapshot is not supported with memory db")
	}
	path := filepath.Dir(s.url)
	dbfile := fmt.Sprintf("%s/snapshot_%s", path, snapshotName)

	_, err := os.Stat(dbfile)
	if !createIfNotExists && os.IsNotExist(err) {
		return fmt.Errorf("Snapshot %s does not exist", snapshotName)
	}

	if os.IsNotExist(err) {
		_, err := s.db.Exec(fmt.Sprintf("VACUUM main INTO '%s'", dbfile))
		if err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite", dbfile)
	if err != nil {
		return err
	}

	err = initdb(db)
	if err != nil {
		return err
	}

	//switch
	s.db.Close()
	s.db = db

	return nil
}

func (s *Store) SupportSnapshotsWithCurrentConfig() bool {
	return s.url != ":memory:"
}

func initdb(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS global(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS ledger(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS blocks(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS blockIndex(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS events(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS transactions(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS collections(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS transactionResults(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// New returns a new in-memory Store implementation.
func New(url string) (*Store, error) {

	db, err := sql.Open("sqlite", url)
	if err != nil {
		return nil, err
	}

	err = initdb(db)
	fmt.Println("init", err)
	if err != nil {
		return nil, err
	}

	store := &Store{
		db:  db,
		url: url,
	}

	store.DataSetter = store
	store.DataGetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}

	return store, nil
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
	_, err := s.db.Exec(fmt.Sprintf("INSERT INTO %s (key, version, value) VALUES (?, ?, ?) ON CONFLICT(key, version) DO UPDATE SET value=excluded.value", store), hex.EncodeToString(key), version, hex.EncodeToString(value))
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(fmt.Sprintf("SELECT value from %s  WHERE key = ? and version <= ? order by version desc LIMIT 1", store), hex.EncodeToString(key), version)
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
	return nil
	//s.db.Close()
}

var _ storage.Store = &Store{}
