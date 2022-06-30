/*
 * Flow Emulator
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package memstore

import (
	"sync"
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemstore(t *testing.T) {

	t.Parallel()

	const blockHeight = 0
	key := flowgo.RegisterID{
		Owner: "",
		Key:   "foo",
	}
	value := flowgo.RegisterEntry{
		Key:   key,
		Value: []byte("bar"),
	}

	store := New()

	err := store.UnsafeInsertLedgerDelta(
		blockHeight,
		delta.Delta{
			Data: map[string]flowgo.RegisterEntry{
				key.String(): value,
			},
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			view := store.LedgerViewByHeight(blockHeight)
			actualValue, err := view.Get("", "foo")

			require.NoError(t, err)
			assert.Equal(t, value.Value, actualValue)
		}()
	}

	wg.Wait()
}

func TestMemstoreSetValueToNil(t *testing.T) {

	t.Parallel()

	store := New()
	key := flowgo.RegisterID{
		Owner: "",
		Key:   "foo",
	}
	value := flowgo.RegisterEntry{
		Key:   key,
		Value: []byte("bar"),
	}
	nilValue := flowgo.RegisterEntry{
		Key:   key,
		Value: nil,
	}

	// set initial value
	err := store.insertLedgerDelta(0,
		delta.Delta{
			Data: map[string]flowgo.RegisterEntry{
				key.String(): value,
			},
		})
	require.NoError(t, err)

	// check initial value
	register, err := store.LedgerViewByHeight(0).Get(key.Owner, key.Key)
	require.NoError(t, err)
	require.Equal(t, string(value.Value), string(register))

	// set value to nil
	err = store.insertLedgerDelta(1,
		delta.Delta{
			Data: map[string]flowgo.RegisterEntry{
				key.String(): nilValue,
			},
		})
	require.NoError(t, err)

	// check value is nil
	register, err = store.LedgerViewByHeight(1).Get(key.Owner, key.Key)
	require.NoError(t, err)
	require.Equal(t, string(nilValue.Value), string(register))
}
