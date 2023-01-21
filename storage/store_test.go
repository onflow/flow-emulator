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

package storage_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/onflow/flow-go-sdk/test"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	convert "github.com/onflow/flow-emulator/convert/sdk"
	"github.com/onflow/flow-emulator/storage"
	"github.com/onflow/flow-emulator/storage/sqlite"
	"github.com/onflow/flow-emulator/utils/unittest"
)

func TestBlocks(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	block1 := &flowgo.Block{
		Header: &flowgo.Header{
			Height: 1,
		},
	}
	block2 := &flowgo.Block{
		Header: &flowgo.Header{
			Height: 2,
		},
	}

	t.Run("should return error for not found", func(t *testing.T) {
		t.Run("BlockByID", func(t *testing.T) {
			freshId := test.IdentifierGenerator().New()
			_, err := store.BlockByID(context.Background(), flowgo.Identifier(freshId))
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})

		t.Run("BlockByHeight", func(t *testing.T) {
			_, err := store.BlockByHeight(context.Background(), block1.Header.Height)
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})

		t.Run("LatestBlock", func(t *testing.T) {
			_, err := store.LatestBlock(context.Background())
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})
	})

	t.Run("should be able to insert block", func(t *testing.T) {
		err := store.StoreBlock(context.Background(), block1)
		assert.NoError(t, err)
	})

	// insert block 1
	err := store.StoreBlock(context.Background(), block1)
	assert.NoError(t, err)

	t.Run("should be able to get inserted block", func(t *testing.T) {
		t.Run("BlockByHeight", func(t *testing.T) {
			block, err := store.BlockByHeight(context.Background(), block1.Header.Height)
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("BlockByID", func(t *testing.T) {
			block, err := store.BlockByID(context.Background(), block1.ID())
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("LatestBlock", func(t *testing.T) {
			block, err := store.LatestBlock(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, *block1, block)
		})
	})

	// insert block 2
	err = store.StoreBlock(context.Background(), block2)
	assert.NoError(t, err)

	t.Run("Latest block should update", func(t *testing.T) {
		block, err := store.LatestBlock(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, *block2, block)
	})
}

func TestCollections(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	ids := test.IdentifierGenerator()

	// collection with 3 transactions
	col := flowgo.LightCollection{
		Transactions: []flowgo.Identifier{
			flowgo.Identifier(ids.New()),
			flowgo.Identifier(ids.New()),
			flowgo.Identifier(ids.New()),
		},
	}

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.CollectionByID(context.Background(), col.ID())
		if assert.Error(t, err) {
			assert.Equal(t, storage.ErrNotFound, err)
		}
	})

	t.Run("should be able to insert collection", func(t *testing.T) {
		err := store.InsertCollection(context.Background(), col)
		assert.NoError(t, err)

		t.Run("should be able to get inserted collection", func(t *testing.T) {
			storedCol, err := store.CollectionByID(context.Background(), col.ID())
			require.NoError(t, err)
			assert.Equal(t, col, storedCol)
		})
	})
}

func TestTransactions(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	tx := unittest.TransactionFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.TransactionByID(context.Background(), tx.ID())
		if assert.Error(t, err) {
			assert.Equal(t, storage.ErrNotFound, err)
		}
	})

	t.Run("should be able to insert tx", func(t *testing.T) {
		err := store.InsertTransaction(context.Background(), tx)
		assert.NoError(t, err)

		t.Run("should be able to get inserted tx", func(t *testing.T) {
			storedTx, err := store.TransactionByID(context.Background(), tx.ID())
			require.NoError(t, err)
			assert.Equal(t, tx.ID(), storedTx.ID())
		})
	})
}

func TestTransactionResults(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	ids := test.IdentifierGenerator()

	result := unittest.StorableTransactionResultFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		txID := flowgo.Identifier(ids.New())

		_, err := store.TransactionResultByID(context.Background(), txID)
		if assert.Error(t, err) {
			assert.Equal(t, storage.ErrNotFound, err)
		}
	})

	t.Run("should be able to insert result", func(t *testing.T) {
		txID := flowgo.Identifier(ids.New())

		err := store.InsertTransactionResult(context.Background(), txID, result)
		assert.NoError(t, err)

		t.Run("should be able to get inserted result", func(t *testing.T) {
			storedResult, err := store.TransactionResultByID(context.Background(), txID)
			require.NoError(t, err)
			assert.Equal(t, result, storedResult)
		})
	})
}

func TestLedger(t *testing.T) {

	t.Parallel()

	t.Run("get/set", func(t *testing.T) {

		t.Parallel()

		store, dir := setupStore(t)
		defer func() {
			require.NoError(t, store.Close())
			require.NoError(t, os.RemoveAll(dir))
		}()

		var blockHeight uint64 = 1

		const owner = ""
		const key = "foo"
		expected := []byte("bar")

		d := delta.NewDelta()
		d.Set(owner, key, expected)

		t.Run("should get able to set ledger", func(t *testing.T) {
			err := store.InsertLedgerDelta(context.Background(), blockHeight, d)
			assert.NoError(t, err)
		})

		t.Run("should be to get set ledger", func(t *testing.T) {
			gotLedger := store.LedgerViewByHeight(context.Background(), blockHeight)
			actual, err := gotLedger.Get(owner, key)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	})

	t.Run("versioning", func(t *testing.T) {

		t.Parallel()

		store, dir := setupStore(t)
		defer func() {
			require.NoError(t, store.Close())
			require.NoError(t, os.RemoveAll(dir))
		}()

		const owner = ""

		// Create a list of ledgers, where the ledger at index i has
		// keys (i+2)-1->(i+2)+1 set to value i-1.
		totalBlocks := 10
		var deltas []delta.Delta
		for i := 2; i < totalBlocks+2; i++ {
			d := delta.NewDelta()
			for j := i - 1; j <= i+1; j++ {
				key := fmt.Sprintf("%d", j)
				d.Set(owner, key, []byte{byte(i - 1)})
			}
			deltas = append(deltas, d)
		}
		require.Equal(t, totalBlocks, len(deltas))

		// Insert all the ledgers, starting with block 1.
		// This will result in a ledger state that looks like this:
		// Block 1: {1: 1, 2: 1, 3: 1}
		// Block 2: {2: 2, 3: 2, 4: 2}
		// ...
		// The combined state at block N looks like:
		// {1: 1, 2: 2, 3: 3, ..., N+1: N, N+2: N}
		for i, ledger := range deltas {
			err := store.InsertLedgerDelta(context.Background(), uint64(i+1), ledger)
			require.NoError(t, err)
		}

		// View at block 1 should have keys 1, 2, 3
		t.Run("should version the first written block", func(t *testing.T) {
			gotLedger := store.LedgerViewByHeight(context.Background(), 1)
			for i := 1; i <= 3; i++ {
				val, err := gotLedger.Get(owner, fmt.Sprintf("%d", i))
				assert.NoError(t, err)
				assert.Equal(t, []byte{byte(1)}, val)
			}
		})

		// View at block N should have values 1->N+2
		t.Run("should version all blocks", func(t *testing.T) {
			for block := 2; block < totalBlocks; block++ {
				gotLedger := store.LedgerViewByHeight(context.Background(), uint64(block))
				// The keys 1->N-1 are defined in previous blocks
				for i := 1; i < block; i++ {
					val, err := gotLedger.Get(owner, fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(i)}, val)
				}
				// The keys N->N+2 are defined in the queried block
				for i := block; i <= block+2; i++ {
					val, err := gotLedger.Get(owner, fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(block)}, val)
				}
			}
		})
	})
}

func TestInsertEvents(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	events := test.EventGenerator()

	t.Run("should be able to insert events", func(t *testing.T) {
		event, _ := convert.SDKEventToFlow(events.New())
		events := []flowgo.Event{event}

		var blockHeight uint64 = 1

		err := store.InsertEvents(context.Background(), blockHeight, events)
		assert.NoError(t, err)

		t.Run("should be able to get inserted events", func(t *testing.T) {
			gotEvents, err := store.EventsByHeight(context.Background(), blockHeight, "")
			assert.NoError(t, err)
			assert.Equal(t, events, gotEvents)
		})
	})
}
func TestEventsByHeight(t *testing.T) {

	t.Parallel()

	store, dir := setupStore(t)
	defer func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}()

	events := test.EventGenerator()

	var (
		nonEmptyBlockHeight    uint64 = 1
		emptyBlockHeight       uint64 = 2
		nonExistentBlockHeight uint64 = 3

		allEvents = make([]flowgo.Event, 10)
		eventsA   = make([]flowgo.Event, 0, 5)
		eventsB   = make([]flowgo.Event, 0, 5)
	)

	for i := range allEvents {
		event, _ := convert.SDKEventToFlow(events.New())

		event.TransactionIndex = uint32(i)
		event.EventIndex = uint32(i * 2)

		// interleave events of both types
		if i%2 == 0 {
			event.Type = "A"
			eventsA = append(eventsA, event)
		} else {
			event.Type = "B"
			eventsB = append(eventsB, event)
		}

		allEvents[i] = event
	}

	err := store.InsertEvents(context.Background(), nonEmptyBlockHeight, allEvents)
	assert.NoError(t, err)

	err = store.InsertEvents(context.Background(), emptyBlockHeight, nil)
	assert.NoError(t, err)

	t.Run("should be able to query by block", func(t *testing.T) {
		t.Run("non-empty block", func(t *testing.T) {
			events, err := store.EventsByHeight(context.Background(), nonEmptyBlockHeight, "")
			assert.NoError(t, err)
			assert.Equal(t, allEvents, events)
		})

		t.Run("empty block", func(t *testing.T) {
			events, err := store.EventsByHeight(context.Background(), emptyBlockHeight, "")
			assert.NoError(t, err)
			assert.Empty(t, events)
		})

		t.Run("non-existent block", func(t *testing.T) {
			events, err := store.EventsByHeight(context.Background(), nonExistentBlockHeight, "")
			assert.NoError(t, err)
			assert.Empty(t, events)
		})
	})

	t.Run("should be able to query by event type", func(t *testing.T) {
		t.Run("type=A, block=1", func(t *testing.T) {
			// should be one event type=1 in block 1
			events, err := store.EventsByHeight(context.Background(), nonEmptyBlockHeight, "A")
			assert.NoError(t, err)
			assert.Equal(t, eventsA, events)
		})

		t.Run("type=B, block=1", func(t *testing.T) {
			// should be 0 type=2 events here
			events, err := store.EventsByHeight(context.Background(), nonEmptyBlockHeight, "B")
			assert.NoError(t, err)
			assert.Equal(t, eventsB, events)
		})
	})
}

// setupStore creates a temporary directory for the Badger and creates a
// badger.Store instance. The caller is responsible for closing the store
// and deleting the temporary directory.
func setupStore(t *testing.T) (*sqlite.Store, string) {
	file, err := ioutil.TempFile("", "sqlite-test")
	require.NoError(t, err)

	store, err := sqlite.New(file.Name())
	require.NoError(t, err)

	return store, file.Name()
}

// Returns the size of a directory and all contents
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
