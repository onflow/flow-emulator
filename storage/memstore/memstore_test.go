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
	const blockHeight = 0
	key := flowgo.RegisterID{
		Owner:      "",
		Controller: "",
		Key:        "foo",
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
			actualValue, err := view.Get("", "", "foo")

			require.NoError(t, err)
			assert.Equal(t, value.Value, actualValue)
		}()
	}

	wg.Wait()
}
