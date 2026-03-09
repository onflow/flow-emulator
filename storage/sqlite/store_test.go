/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
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
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	file, err := os.CreateTemp("", "test.sqlite")
	require.NoError(t, err)

	store, err := New(file.Name())
	require.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err)

	_, err = New("/invalidLocation")
	assert.NotContains(
		t,
		err.Error(),
		"unable to open database file: out of memory",
		"should not attempt to open the database file if the location is invalid",
	)
	assert.ErrorContains(
		t,
		err,
		"no such file or directory",
		"should return an error indicating the location is invalid",
	)
}

// TestConcurrentStoreAccess opens multiple Store instances against the same
// on-disk SQLite file (simulating separate processes) and performs concurrent
// reads and writes.  Without WAL mode + busy_timeout this would produce
// "database is locked" errors.
func TestConcurrentStoreAccess(t *testing.T) {
	dir := t.TempDir()

	const numStores = 4
	const numOps = 50
	const storeName = "ledger"

	stores := make([]*Store, numStores)
	for i := range stores {
		s, err := New(dir, WithMultiProcessAccess())
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		stores[i] = s
	}

	ctx := context.Background()

	// Concurrent writes from all stores, then concurrent reads.
	var wg sync.WaitGroup
	for i, s := range stores {
		wg.Add(1)
		go func(idx int, store *Store) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := []byte(fmt.Sprintf("key-%d-%d", idx, j))
				value := []byte(fmt.Sprintf("value-%d-%d", idx, j))
				err := store.SetBytes(ctx, storeName, key, value)
				assert.NoError(t, err, "store %d write %d failed", idx, j)
			}
		}(i, s)
	}
	wg.Wait()

	// Concurrent reads: every store should see every key.
	for i, s := range stores {
		wg.Add(1)
		go func(idx int, store *Store) {
			defer wg.Done()
			for si := 0; si < numStores; si++ {
				for j := 0; j < numOps; j++ {
					key := []byte(fmt.Sprintf("key-%d-%d", si, j))
					expected := []byte(fmt.Sprintf("value-%d-%d", si, j))
					val, err := store.GetBytes(ctx, storeName, key)
					assert.NoError(t, err, "store %d read key-%d-%d failed", idx, si, j)
					assert.Equal(t, expected, val)
				}
			}
		}(i, s)
	}
	wg.Wait()
}
