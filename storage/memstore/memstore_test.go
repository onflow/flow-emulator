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

package memstore

import (
	"context"
	"sync"
	"testing"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemstore(t *testing.T) {

	t.Parallel()

	const blockHeight = 0
	key := flow.NewRegisterID("", "foo")
	value := []byte("bar")
	store := New()

	err := store.insertExecutionSnapshot(
		blockHeight,
		&state.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: value,
			},
		},
	)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			snapshot := store.LedgerByHeight(
				context.Background(),
				blockHeight)
			actualValue, err := snapshot.Get(key)

			require.NoError(t, err)
			assert.Equal(t, value, actualValue)
		}()
	}

	wg.Wait()
}

func TestMemstoreSetValueToNil(t *testing.T) {

	t.Parallel()

	store := New()
	key := flow.NewRegisterID("", "foo")
	value := []byte("bar")
	var nilByte []byte
	nilValue := nilByte

	// set initial value
	err := store.insertExecutionSnapshot(
		0,
		&state.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: value,
			},
		})
	require.NoError(t, err)

	// check initial value
	register, err := store.LedgerByHeight(context.Background(), 0).Get(flow.NewRegisterID(key.Owner, key.Key))
	require.NoError(t, err)
	require.Equal(t, string(value), string(register))

	// set value to nil
	err = store.insertExecutionSnapshot(
		1,
		&state.ExecutionSnapshot{
			WriteSet: map[flowgo.RegisterID]flowgo.RegisterValue{
				key: nilValue,
			},
		})
	require.NoError(t, err)

	// check value is nil
	register, err = store.LedgerByHeight(context.Background(), 1).Get(flow.NewRegisterID(key.Owner, key.Key))
	require.NoError(t, err)
	require.Equal(t, string(nilValue), string(register))
}
