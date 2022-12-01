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

package badger

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"
	"github.com/onflow/flow-emulator/storage"
)

// Store is an embedded storage implementation using Badger as the underlying
// persistent key-value store.
type Store struct {
	storage.DefaultStore
	config          Config
	db              *badger.DB
	dbGitRepository *git.Repository
}

var _ storage.Store = &Store{}

func badgerKey(storage string, key []byte) []byte {
	return []byte(fmt.Sprintf("%s-%x", storage, key))
}

func badgerVersionedKey(storage string, key []byte, version uint64) []byte {
	return []byte(fmt.Sprintf("%s-%x-%032d", storage, key, version))
}

func New(opts ...Opt) (*Store, error) {
	config := getBadgerConfig(opts...)
	config.BadgerOptions.BypassLockGuard = false
	db, err := badger.Open(config.BadgerOptions)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	_ = db.Sync()
	store := &Store{db: db, config: config}
	store.DataGetter = store
	store.DataSetter = store
	store.KeyGenerator = &storage.DefaultKeyGenerator{}
	if err = store.setup(); err != nil {
		return nil, err
	}
	return store, nil
}

// getTx returns a getter function bound to the input transaction that can be
// used to get values from Badger.
//
// The getter function checks for key-not-found errors and wraps them in
// storage.NotFound in order to comply with the storage.Store interface.
//
// This saves a few lines of converting a badger.Item to []byte.
func getTx(txn *badger.Txn) func([]byte) ([]byte, error) {
	return func(key []byte) ([]byte, error) {
		// Badger returns an "item" upon GETs, we need to copy the actual value
		// from the item and return it.
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil, storage.ErrNotFound
			}
			return nil, err
		}

		val := make([]byte, item.ValueSize())
		return item.ValueCopy(val)
	}
}

func (s *Store) GetBytes(ctx context.Context, store string, key []byte) (result []byte, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		result, err = getTx(txn)(badgerKey(store, key))
		if err != nil {
			return err
		}
		return nil
	})
	return
}

func (s *Store) SetBytes(ctx context.Context, store string, key []byte, value []byte) (err error) {
	err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set(badgerKey(store, key), value)
		return err
	})
	return
}

func (s *Store) SetBytesWithVersion(ctx context.Context, store string, key []byte, value []byte, version uint64) (err error) {
	err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set(badgerVersionedKey(store, key, version), value)
		return err
	})
	return
}

func (s *Store) GetBytesAtVersion(ctx context.Context, store string, key []byte, version uint64) (result []byte, err error) {

	iterOpts := badger.DefaultIteratorOptions
	iterOpts.Reverse = true
	iterOpts.Prefix = badgerKey(store, key)
	startKey := badgerVersionedKey(store, key, version)

	err = s.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(iterOpts)
		defer iter.Close()
		iter.Seek(startKey)

		if !iter.Valid() {
			err = storage.ErrNotFound
			return err
		} else {
			item := iter.Item()
			result, err = item.ValueCopy([]byte{})
			return err
		}
	})

	return
}

//
//
// git below
///

func getTag(r *git.Repository, tag string) *object.Tag {
	tags, err := r.TagObjects()
	if err != nil {
		return nil
	}
	var res *object.Tag = nil
	_ = tags.ForEach(func(t *object.Tag) error {
		if t.Name == tag {
			res = t
		}
		return nil
	})
	return res
}

func setTag(r *git.Repository, tag string, tagger *object.Signature) (bool, error) {
	if getTag(r, tag) != nil {
		return false, nil
	}
	h, err := r.Head()
	if err != nil {
		return false, err
	}
	_, err = r.CreateTag(tag, h.Hash(), &git.CreateTagOptions{
		Tagger:  tagger,
		Message: tag,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
func defaultSignature(name, email string) *object.Signature {
	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

//prevents git commits when emulator running
//ignoring error here but it is not critical to operation
func (s *Store) lockGit() {
	lockPath := fmt.Sprintf("%s/.git/index.lock", s.config.DBPath)
	_ = ioutil.WriteFile(lockPath, []byte("emulatorLock"), 0755)
}

//ignoring error here but it is not critical to operation
func (s *Store) unlockGit() {
	lockPath := fmt.Sprintf("%s/.git/index.lock", s.config.DBPath)
	_ = os.Remove(lockPath)
}

func (s *Store) JumpToContext(context string) error {

	if !s.config.Snapshot {
		return fmt.Errorf("Snapshot option is not enabled")
	}

	s.unlockGit()
	defer s.lockGit()
	err := s.db.Close()
	if err != nil {
		return err
	}

	err = s.newCommit(fmt.Sprintf("Context switching to: %s", context))
	if err != nil {
		return err
	}
	w, err := s.dbGitRepository.Worktree()
	if err != nil {
		return err
	}
	branch := fmt.Sprintf("refs/heads/%s", context)
	b := plumbing.ReferenceName(branch)

	// checkout branch ( first branch name is actually context name )
	err = w.Checkout(&git.CheckoutOptions{Create: false, Force: true, Branch: b})

	if err != nil {
		// branch doesn't exist, it means we need to create it ( first branch is named after context )
		err := w.Checkout(&git.CheckoutOptions{Create: true, Force: true, Branch: b})
		if err != nil {
			return err
		}

		//after we create a tag pointing to start of this context
		created, err := setTag(s.dbGitRepository, context, defaultSignature("Emulator", "emulator@onflow.org"))
		if err != nil && !created {
			return err
		}

		s.config.Logger.Infof("Created a new state snapshot with the name '%s'", context)

	} else {

		//create new branch
		uuidWithHyphen := uuid.New()
		newBranchUuid := strings.Replace(uuidWithHyphen.String(), "-", "", -1)

		err := w.Checkout(&git.CheckoutOptions{Create: true, Force: true, Branch: plumbing.NewBranchReferenceName(newBranchUuid)})
		if err != nil {
			return err
		}

		//we have new branch but we don't need to create a tag, we just need to reset to tag
		tag := getTag(s.dbGitRepository, context)
		if tag != nil && !tag.Hash.IsZero() {
			commit, _ := tag.Commit()
			_ = w.Reset(&git.ResetOptions{
				Mode:   git.HardReset,
				Commit: commit.Hash,
			})
		}
		s.config.Logger.Infof("Switched to snapshot with name '%s'", context)

	}

	s.db, err = badger.Open(s.config.BadgerOptions)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}

	return nil

}

func (s *Store) newCommit(message string) error {
	s.unlockGit()
	defer s.lockGit()
	err := s.Sync()
	if err != nil {
		return err
	}

	w, err := s.dbGitRepository.Worktree()
	if err != nil {
		return err
	}

	err = filepath.Walk(s.config.DBPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if info.Name() == "KEYREGISTRY" || info.Name() == "MANIFEST" || info.Name() == "LOCK" {
			_, adderr := w.Add(path[strings.LastIndex(path, "/")+1:])
			return adderr
		}

		if filepath.Ext(path) == ".vlog" || filepath.Ext(path) == ".sst" {
			_, adderr := w.Add(path[strings.LastIndex(path, "/")+1:])
			return adderr
		}
		return nil
	})

	if err != nil {
		return err
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Flow Emulator",
			Email: "emulator@onflow.org",
			When:  time.Now(),
		},
	})

	if err != nil {
		return err
	}
	return nil
}

func (s *Store) openRepository(directory string) (*git.Repository, error) {
	dbgit, err := git.PlainOpen(directory)
	if err == nil {
		return dbgit, err
	}
	if err == git.ErrRepositoryNotExists {
		result, err := git.PlainInit(directory, false)
		if err == nil {
			return result, err
		}
		return nil, err
	}
	return nil, err
}

// setups git, setup sets up in-memory indexes and prepares the store for use.
func (s *Store) setup() error {

	if s.config.Snapshot {
		dbgit, err := s.openRepository(s.config.DBPath)
		s.dbGitRepository = dbgit
		if err != nil {
			return err
		}

		w, _ := dbgit.Worktree()
		r, _ := dbgit.Head()
		if r != nil {
			_ = w.Reset(&git.ResetOptions{
				Mode:   git.HardReset,
				Commit: r.Hash(),
			})
		}
		err = s.newCommit("Emulator Started New Session")
		if err != nil {
			return err
		}
		s.lockGit()
	}

	return nil
}

// Close closes the underlying Badger database. It is necessary to close
// a Store before exiting to ensure all writes are persisted to disk.
func (s *Store) Close() error {
	err := s.db.Close()
	if err != nil {
		return err
	}

	if s.config.Snapshot {
		defer s.unlockGit()
		return s.newCommit("Emulator Ended Session")
	}

	return nil
}

// Sync syncs database content to disk.
func (s *Store) Sync() error {
	if s.config.InMemory {
		return nil
	}
	return s.db.Sync()
}

func (s *Store) RunValueLogGC(discardRatio float64) error {
	if s.config.InMemory {
		return nil
	}
	err := s.db.RunValueLogGC(discardRatio)

	// ignore ErrNoRewrite, which occurs when GC results in no cleanup
	if err != nil && !errors.Is(err, badger.ErrNoRewrite) {
		return err
	}

	return nil
}
