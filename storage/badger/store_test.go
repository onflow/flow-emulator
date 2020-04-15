package badger_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/test"
	model "github.com/dapperlabs/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-emulator/storage"
	"github.com/dapperlabs/flow-emulator/storage/badger"
	"github.com/dapperlabs/flow-emulator/types"
	"github.com/dapperlabs/flow-emulator/utils/unittest"
)

func TestBlocks(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	block1 := types.Block{
		Height: 1,
	}
	block2 := types.Block{
		Height: 2,
	}

	t.Run("should return error for not found", func(t *testing.T) {
		t.Run("BlockByID", func(t *testing.T) {
			_, err := store.BlockByID(test.IdentifierGenerator().New())
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})

		t.Run("BlockByHeight", func(t *testing.T) {
			_, err := store.BlockByHeight(block1.Height)
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})

		t.Run("LatestBlock", func(t *testing.T) {
			_, err := store.LatestBlock()
			if assert.Error(t, err) {
				assert.Equal(t, storage.ErrNotFound, err)
			}
		})
	})

	t.Run("should be able to insert block", func(t *testing.T) {
		err := store.InsertBlock(block1)
		assert.NoError(t, err)
	})

	// insert block 1
	err := store.InsertBlock(block1)
	assert.NoError(t, err)

	t.Run("should be able to get inserted block", func(t *testing.T) {
		t.Run("BlockByHeight", func(t *testing.T) {
			block, err := store.BlockByHeight(block1.Height)
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("BlockByID", func(t *testing.T) {
			block, err := store.BlockByID(block1.ID())
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("LatestBlock", func(t *testing.T) {
			block, err := store.LatestBlock()
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})
	})

	// insert block 2
	err = store.InsertBlock(block2)
	assert.NoError(t, err)

	t.Run("Latest block should update", func(t *testing.T) {
		block, err := store.LatestBlock()
		assert.NoError(t, err)
		assert.Equal(t, block2, block)
	})
}

func TestTransactions(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	tx := unittest.TransactionFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.TransactionByID(tx.ID())
		if assert.Error(t, err) {
			assert.Equal(t, storage.ErrNotFound, err)
		}
	})

	t.Run("should be able to insert tx", func(t *testing.T) {
		err := store.InsertTransaction(tx)
		assert.NoError(t, err)

		t.Run("should be able to get inserted tx", func(t *testing.T) {
			storedTx, err := store.TransactionByID(tx.ID())
			require.Nil(t, err)
			assert.Equal(t, tx.ID(), storedTx.ID())
		})
	})
}

func TestTransactionResults(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	ids := test.IdentifierGenerator()

	t.Run("should return error for not found", func(t *testing.T) {
		txID := ids.New()

		_, err := store.TransactionResultByID(txID)
		if assert.Error(t, err) {
			assert.Equal(t, storage.ErrNotFound, err)
		}
	})

	t.Run("should be able to insert result", func(t *testing.T) {
		txID := ids.New()
		result := unittest.TransactionResultFixture()

		err := store.InsertTransactionResult(txID, result)
		assert.NoError(t, err)

		t.Run("should be able to get inserted result", func(t *testing.T) {
			storedResult, err := store.TransactionResultByID(txID)
			require.Nil(t, err)
			assert.Equal(t, result, storedResult)
		})
	})
}

func TestLedger(t *testing.T) {
	t.Run("get/set", func(t *testing.T) {
		store, dir := setupStore(t)
		defer func() {
			require.Nil(t, store.Close())
			require.Nil(t, os.RemoveAll(dir))
		}()

		var blockHeight uint64 = 1

		ledger := make(types.LedgerDelta)
		ledger["foo"] = []byte("bar")

		t.Run("should get able to set ledger", func(t *testing.T) {
			err := store.InsertLedgerDelta(blockHeight, ledger)
			assert.NoError(t, err)
		})

		t.Run("should be to get set ledger", func(t *testing.T) {
			gotLedger := store.LedgerViewByHeight(blockHeight)
			gotRegister, err := gotLedger.Get("foo")
			assert.NoError(t, err)
			assert.Equal(t, ledger["foo"], gotRegister)
		})
	})

	t.Run("versioning", func(t *testing.T) {
		store, dir := setupStore(t)
		defer func() {
			require.Nil(t, store.Close())
			require.Nil(t, os.RemoveAll(dir))
		}()

		// Create a list of ledgers, where the ledger at index i has
		// keys (i+2)-1->(i+2)+1 set to value i-1.
		totalBlocks := 10
		var ledgers []types.LedgerDelta
		for i := 2; i < totalBlocks+2; i++ {
			ledger := make(types.LedgerDelta)
			for j := i - 1; j <= i+1; j++ {
				ledger[fmt.Sprintf("%d", j)] = []byte{byte(i - 1)}
			}
			ledgers = append(ledgers, ledger)
		}
		require.Equal(t, totalBlocks, len(ledgers))

		// Insert all the ledgers, starting with block 1.
		// This will result in a ledger state that looks like this:
		// Block 1: {1: 1, 2: 1, 3: 1}
		// Block 2: {2: 2, 3: 2, 4: 2}
		// ...
		// The combined state at block N looks like:
		// {1: 1, 2: 2, 3: 3, ..., N+1: N, N+2: N}
		for i, ledger := range ledgers {
			err := store.InsertLedgerDelta(uint64(i+1), ledger)
			require.NoError(t, err)
		}

		// View at block 1 should have keys 1, 2, 3
		t.Run("should version the first written block", func(t *testing.T) {
			gotLedger := store.LedgerViewByHeight(1)
			for i := 1; i <= 3; i++ {
				val, err := gotLedger.Get(fmt.Sprintf("%d", i))
				assert.NoError(t, err)
				assert.Equal(t, []byte{byte(1)}, val)
			}
		})

		// View at block N should have values 1->N+2
		t.Run("should version all blocks", func(t *testing.T) {
			for block := 2; block < totalBlocks; block++ {
				gotLedger := store.LedgerViewByHeight(uint64(block))
				// The keys 1->N-1 are defined in previous blocks
				for i := 1; i < block; i++ {
					val, err := gotLedger.Get(fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(i)}, val)
				}
				// The keys N->N+2 are defined in the queried block
				for i := block; i <= block+2; i++ {
					val, err := gotLedger.Get(fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(block)}, val)
				}
			}
		})
	})
}

func TestInsertEvents(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	t.Run("should be able to insert events", func(t *testing.T) {
		events := []flow.Event{unittest.EventFixture(func(e *flow.Event) {})}
		var blockHeight uint64 = 1

		err := store.InsertEvents(blockHeight, events)
		assert.NoError(t, err)

		t.Run("should be able to get inserted events", func(t *testing.T) {
			gotEvents, err := store.EventsByHeight(blockHeight, "")
			assert.NoError(t, err)
			assert.Equal(t, events, gotEvents)
		})
	})
}
func TestEventsByHeight(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	var (
		nonEmptyBlockHeight    uint64 = 1
		emptyBlockHeight       uint64 = 2
		nonExistentBlockHeight uint64 = 3

		allEvents = make([]flow.Event, 10)
		eventsA   = make([]flow.Event, 0, 5)
		eventsB   = make([]flow.Event, 0, 5)
	)

	for i, _ := range allEvents {
		event := unittest.EventFixture()
		event.TransactionIndex = i
		event.EventIndex = i * 2

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

	err := store.InsertEvents(nonEmptyBlockHeight, allEvents)
	assert.NoError(t, err)

	err = store.InsertEvents(emptyBlockHeight, nil)
	assert.NoError(t, err)

	t.Run("should be able to query by block", func(t *testing.T) {
		t.Run("non-empty block", func(t *testing.T) {
			events, err := store.EventsByHeight(nonEmptyBlockHeight, "")
			assert.NoError(t, err)
			assert.Equal(t, allEvents, events)
		})

		t.Run("empty block", func(t *testing.T) {
			events, err := store.EventsByHeight(emptyBlockHeight, "")
			assert.NoError(t, err)
			assert.Empty(t, events)
		})

		t.Run("non-existent block", func(t *testing.T) {
			events, err := store.EventsByHeight(nonExistentBlockHeight, "")
			assert.NoError(t, err)
			assert.Empty(t, events)
		})
	})

	t.Run("should be able to query by event type", func(t *testing.T) {
		t.Run("type=A, block=1", func(t *testing.T) {
			// should be one event type=1 in block 1
			events, err := store.EventsByHeight(nonEmptyBlockHeight, "A")
			assert.NoError(t, err)
			assert.Equal(t, eventsA, events)
		})

		t.Run("type=B, block=1", func(t *testing.T) {
			// should be 0 type=2 events here
			events, err := store.EventsByHeight(nonEmptyBlockHeight, "B")
			assert.NoError(t, err)
			assert.Equal(t, eventsB, events)
		})
	})
}

func TestPersistence(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	block := types.Block{Height: 1}
	tx := unittest.TransactionFixture()
	events := []flow.Event{unittest.EventFixture(func(e *flow.Event) {})}

	ledger := make(types.LedgerDelta)
	ledger["foo"] = []byte("bar")

	// insert some stuff to to the store
	err := store.InsertBlock(block)
	assert.NoError(t, err)
	err = store.InsertTransaction(tx)
	assert.NoError(t, err)
	err = store.InsertEvents(block.Height, events)
	assert.NoError(t, err)
	err = store.InsertLedgerDelta(block.Height, ledger)

	// close the store
	err = store.Close()
	assert.NoError(t, err)

	// create a new store with the same database directory
	store, err = badger.New(badger.WithPath(dir))
	require.Nil(t, err)

	// should be able to retrieve what we stored
	gotBlock, err := store.LatestBlock()
	assert.NoError(t, err)
	assert.Equal(t, block, gotBlock)

	gotTx, err := store.TransactionByID(tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, tx.ID(), gotTx.ID())

	gotEvents, err := store.EventsByHeight(block.Height, "")
	assert.NoError(t, err)
	assert.Equal(t, events, gotEvents)

	gotLedger := store.LedgerViewByHeight(block.Height)
	gotRegister, err := gotLedger.Get("foo")
	assert.NoError(t, err)
	assert.Equal(t, ledger["foo"], gotRegister)
}

func benchmarkInsertLedgerDelta(b *testing.B, nKeys int) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ledger := make(types.LedgerDelta)
	for i := 0; i < nKeys; i++ {
		ledger[fmt.Sprintf("%d", i)] = []byte{byte(i)}
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := store.InsertLedgerDelta(1, ledger); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertLedgerDelta1(b *testing.B)    { benchmarkInsertLedgerDelta(b, 1) }
func BenchmarkInsertLedgerDelta10(b *testing.B)   { benchmarkInsertLedgerDelta(b, 10) }
func BenchmarkInsertLedgerDelta100(b *testing.B)  { benchmarkInsertLedgerDelta(b, 100) }
func BenchmarkInsertLedgerDelta1000(b *testing.B) { benchmarkInsertLedgerDelta(b, 1000) }

func BenchmarkBlockDiskUsage(b *testing.B) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ids := test.IdentifierGenerator()

	b.StartTimer()
	var lastDBSize int64
	for i := 0; i < b.N; i++ {
		block := types.Block{
			Height:   uint64(i),
			ParentID: ids.New(),
			Guarantees: []*model.CollectionGuarantee{
				{
					CollectionID: model.Identifier(ids.New()),
				},
			},
		}
		if err := store.InsertBlock(block); err != nil {
			b.Fatal(err)
		}

		if err := store.Sync(); err != nil {
			b.Fatal(err)
		}

		size, err := dirSize(dir)
		if err != nil {
			b.Fatal(err)
		}

		dbSizeIncrease := size - lastDBSize
		b.ReportMetric(float64(dbSizeIncrease), "db_size_increase_bytes/op")
		lastDBSize = size
	}
}

func BenchmarkLedgerDiskUsage(b *testing.B) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.StartTimer()
	var lastDBSize int64
	for i := 0; i < b.N; i++ {
		ledger := make(types.LedgerDelta)
		for j := 0; j < 100; j++ {
			ledger[fmt.Sprintf("%d-%d", i, j)] = []byte{byte(i), byte(j)}
		}
		if err := store.InsertLedgerDelta(uint64(i), ledger); err != nil {
			b.Fatal(err)
		}
		if err := store.Sync(); err != nil {
			b.Fatal(err)
		}

		size, err := dirSize(dir)
		if err != nil {
			b.Fatal(err)
		}

		dbSizeIncrease := size - lastDBSize
		b.ReportMetric(float64(dbSizeIncrease), "db_size_increase_bytes/op")
		lastDBSize = size
	}
}

// setupStore creates a temporary directory for the Badger and creates a
// badger.Store instance. The caller is responsible for closing the store
// and deleting the temporary directory.
func setupStore(t *testing.T) (*badger.Store, string) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)

	store, err := badger.New(badger.WithPath(dir))
	require.Nil(t, err)

	return store, dir
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
